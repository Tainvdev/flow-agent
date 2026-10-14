package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

// This joins durable accepted-media recovery with account isolation: while the
// recovered account-A job is blocked on its runtime gate (standing in for an
// account-local refresh/rotation), account B must still complete a new job.
type refreshResumeIntegrationRunner struct {
	manager       *AccountManager
	resumeStarted chan struct{}
	resumeDone    chan struct{}
	bDone         chan struct{}
}

func (r *refreshResumeIntegrationRunner) Run(context.Context, store.Job) (any, error) {
	return nil, fmt.Errorf("Run must not be used for this integration runner")
}

func (r *refreshResumeIntegrationRunner) RunWithProgress(ctx context.Context, job store.Job, _ AttemptReporter) (any, error) {
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, err
	}
	runtime, err := r.manager.acquireScheduledRuntime(ctx, payload.AccountID)
	if err != nil {
		return nil, err
	}
	r.manager.release(runtime)
	close(r.bDone)
	return map[string]any{"account_id": payload.AccountID, "status": "succeeded"}, nil
}

func (r *refreshResumeIntegrationRunner) ResumeWithProgress(ctx context.Context, job store.Job, attempt store.JobAttempt, _ AttemptReporter) (any, error) {
	close(r.resumeStarted)
	runtime, err := r.manager.acquireScheduledRuntime(ctx, attempt.AccountID)
	if err != nil {
		return nil, err
	}
	r.manager.release(runtime)
	close(r.resumeDone)
	return map[string]any{"job_id": job.ID, "account_id": attempt.AccountID, "media_ids": attempt.UpstreamIDs, "status": "succeeded"}, nil
}

func TestRestartedAcceptedMediaWaitsForAccountRefreshWhileOtherAccountServes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jobs.db")
	st, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, store.Job{ID: "accepted-a", Kind: "video", Payload: `{"account_id":"account-a","prompt":"resume"}`}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CreateJob(ctx, store.Job{ID: "new-b", Kind: "image", Payload: `{"account_id":"account-b","prompt":"new work"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "crashed-worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "accepted-a", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "accepted-a", 1, "rendering", "account-a", "project-a", "revision-a", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobAttemptUpstreamIDs(ctx, "accepted-a", 1, []string{"accepted-media-a"}); err != nil {
		t.Fatal(err)
	}
	admin, err := sql.Open("sqlite", st.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='accepted-a'`, time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := admin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen as a fresh service process, then install isolated account runtimes.
	st, err = store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	manager, err := NewAccountManager(st, bridge.NewBridge(nil, nil, t.TempDir()), 2)
	if err != nil {
		t.Fatal(err)
	}
	manager.strictFresh = false
	manager.initOnce.Do(func() { close(manager.initialized) })
	for _, id := range []string{"account-a", "account-b"} {
		gate := make(chan struct{}, 1)
		manager.accountGates[id] = gate
		manager.accounts[id] = &accountRuntime{id: id, gate: gate}
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		manager.scheduleWorkers[id] = worker
		manager.scheduler.Register(worker)
	}
	// Occupy A's stable gate while its cookie-refresh/rotation is in progress.
	manager.accountGates["account-a"] <- struct{}{}
	runner := &refreshResumeIntegrationRunner{
		manager: manager, resumeStarted: make(chan struct{}), resumeDone: make(chan struct{}), bDone: make(chan struct{}),
	}
	jobs, err := NewJobs(st, runner, "test-token", 2)
	if err != nil {
		t.Fatal(err)
	}
	serviceCtx, cancel := context.WithCancel(ctx)
	serviceStopped := make(chan struct{})
	go func() {
		jobs.Start(serviceCtx)
		close(serviceStopped)
	}()
	defer func() {
		cancel()
		select {
		case <-serviceStopped:
		case <-time.After(3 * time.Second):
			t.Error("job workers did not stop")
		}
	}()

	select {
	case <-runner.resumeStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("accepted media was not resumed after restart")
	}
	select {
	case <-runner.bDone:
	case <-time.After(3 * time.Second):
		t.Fatal("account B did not serve work while account A was refreshing")
	}
	resumeDeadline := time.Now().Add(3 * time.Second)
	var bJob *store.Job
	for time.Now().Before(resumeDeadline) {
		bJob, err = st.JobByID(ctx, "new-b")
		if err == nil && bJob.Status == "succeeded" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || bJob.Status != "succeeded" {
		t.Fatalf("account B job=(%+v,%v), want succeeded before releasing account A", bJob, err)
	}
	select {
	case <-runner.resumeDone:
		t.Fatal("account A resumed while its refresh gate was still held")
	default:
	}
	<-manager.accountGates["account-a"]
	select {
	case <-runner.resumeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("accepted-media resume did not continue after account A gate was released")
	}
	resumeDeadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(resumeDeadline) {
		job, err := st.JobByID(ctx, "accepted-a")
		if err == nil && job.Status == "succeeded" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, err := st.JobByID(ctx, "accepted-a")
	t.Fatalf("accepted-media job=(%+v,%v), want succeeded", job, err)
}
