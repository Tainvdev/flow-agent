package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/batchexecute"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

type testRunner struct{ calls atomic.Int32 }

func (r *testRunner) Run(_ context.Context, job store.Job) (any, error) {
	r.calls.Add(1)
	return map[string]string{"job": job.ID, "kind": job.Kind}, nil
}

type unavailableAccountRunner struct{}

func (unavailableAccountRunner) Run(context.Context, store.Job) (any, error) {
	return nil, &pool.NoWorkerError{Reason: "no account is ready"}
}

type wakeSignalRunner struct{ started chan struct{} }

func (r *wakeSignalRunner) Run(context.Context, store.Job) (any, error) {
	close(r.started)
	return map[string]string{"status": "succeeded"}, nil
}

type accountAwareQueueRunner struct {
	firstAccountJob  chan struct{}
	secondAccountJob chan struct{}
	otherAccountJob  chan struct{}
	releaseAccountA  chan struct{}
}

func (r *accountAwareQueueRunner) Run(ctx context.Context, job store.Job) (any, error) {
	switch job.ID {
	case "01-a-running":
		close(r.firstAccountJob)
	case "02-a-waiting":
		close(r.secondAccountJob)
	case "03-b-ready":
		close(r.otherAccountJob)
		return map[string]string{"account_id": "account-b"}, nil
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.releaseAccountA:
		return map[string]string{"account_id": "account-a"}, nil
	}
}

type resumeTestRunner struct {
	runCalls    atomic.Int32
	resumeCalls atomic.Int32
}

type preparingRunner struct{ started chan struct{} }

type drainRunner struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

type acceptedDrainRunner struct{ started chan struct{} }

type submittingDrainRunner struct{ started chan struct{} }

func (r *acceptedDrainRunner) Run(context.Context, store.Job) (any, error) {
	return nil, fmt.Errorf("accepted drain runner requires progress reporting")
}

func (r *acceptedDrainRunner) RunWithProgress(ctx context.Context, _ store.Job, progress AttemptReporter) (any, error) {
	if err := progress.SetPhase("preparing", "binding account"); err != nil {
		return nil, err
	}
	if err := progress.BindAccount("account-a", "project-a", "revision-a"); err != nil {
		return nil, err
	}
	if err := progress.SetPhase("submitting", "submit accepted"); err != nil {
		return nil, err
	}
	if err := progress.SetUpstreamIDs([]string{"media-a"}); err != nil {
		return nil, err
	}
	if err := progress.SetPhase("rendering", "waiting for media"); err != nil {
		return nil, err
	}
	close(r.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r *submittingDrainRunner) Run(context.Context, store.Job) (any, error) {
	return nil, fmt.Errorf("submitting drain runner requires progress reporting")
}

func (r *submittingDrainRunner) RunWithProgress(ctx context.Context, _ store.Job, progress AttemptReporter) (any, error) {
	if err := progress.SetPhase("preparing", "binding account"); err != nil {
		return nil, err
	}
	if err := progress.BindAccount("account-a", "project-a", "revision-a"); err != nil {
		return nil, err
	}
	if err := progress.SetPhase("submitting", "submit outcome not yet persisted"); err != nil {
		return nil, err
	}
	close(r.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r *drainRunner) Run(ctx context.Context, job store.Job) (any, error) {
	r.once.Do(func() { close(r.started) })
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.release:
		return map[string]string{"job": job.ID}, nil
	}
}

type deadlineRetryRunner struct {
	calls          atomic.Int32
	markSubmitting bool
}

func (r *preparingRunner) Run(context.Context, store.Job) (any, error) { return nil, nil }

func (r *preparingRunner) RunWithProgress(ctx context.Context, _ store.Job, progress AttemptReporter) (any, error) {
	if err := progress.SetPhase("preparing", "binding account"); err != nil {
		return nil, err
	}
	close(r.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (r *deadlineRetryRunner) Run(context.Context, store.Job) (any, error) {
	return nil, fmt.Errorf("Run must not be called for a progress runner")
}

func (r *deadlineRetryRunner) RunWithProgress(_ context.Context, _ store.Job, progress AttemptReporter) (any, error) {
	call := r.calls.Add(1)
	if call == 1 {
		if r.markSubmitting {
			if err := progress.SetPhase("submitting", "request may reach upstream"); err != nil {
				return nil, err
			}
		}
		return nil, context.DeadlineExceeded
	}
	return map[string]string{"status": "succeeded"}, nil
}

type ambiguousSubmitError struct{}

func (ambiguousSubmitError) Error() string           { return "submit transport outcome unknown" }
func (ambiguousSubmitError) SubmissionUnknown() bool { return true }

type ambiguousRunner struct{ calls atomic.Int32 }

func (r *ambiguousRunner) Run(context.Context, store.Job) (any, error) {
	r.calls.Add(1)
	return nil, ambiguousSubmitError{}
}

type refusalRunner struct{ calls atomic.Int32 }

type httpThrottleRetryRunner struct{ calls atomic.Int32 }

type acceptedThenTimeoutRunner struct {
	runCalls    atomic.Int32
	resumeCalls atomic.Int32
}

func (r *httpThrottleRetryRunner) Run(_ context.Context, job store.Job) (any, error) {
	if r.calls.Add(1) == 1 {
		return nil, &batchexecute.HTTPThrottleError{RPCID: batchexecute.RPCIDGenerate, Status: 429, Body: "rate limited"}
	}
	return map[string]string{"job": job.ID, "status": "succeeded"}, nil
}

func (r *acceptedThenTimeoutRunner) Run(context.Context, store.Job) (any, error) {
	return nil, fmt.Errorf("Run must not be used by a progress runner")
}

func (r *acceptedThenTimeoutRunner) RunWithProgress(_ context.Context, _ store.Job, progress AttemptReporter) (any, error) {
	r.runCalls.Add(1)
	if err := progress.BindAccount("account-a", "project-a", "revision-a"); err != nil {
		return nil, err
	}
	if err := progress.SetPhase("submitting", "generation request accepted"); err != nil {
		return nil, err
	}
	if err := progress.SetUpstreamIDs([]string{"accepted-media-a"}); err != nil {
		return nil, err
	}
	if err := progress.SetPhase("rendering", "waiting for accepted media"); err != nil {
		return nil, err
	}
	return nil, context.DeadlineExceeded
}

func (r *acceptedThenTimeoutRunner) ResumeWithProgress(_ context.Context, _ store.Job, attempt store.JobAttempt, _ AttemptReporter) (any, error) {
	r.resumeCalls.Add(1)
	if len(attempt.UpstreamIDs) != 1 || attempt.UpstreamIDs[0] != "accepted-media-a" {
		return nil, fmt.Errorf("unexpected resume ids: %v", attempt.UpstreamIDs)
	}
	return map[string]string{"status": "ready"}, nil
}

func (r *refusalRunner) Run(context.Context, store.Job) (any, error) {
	r.calls.Add(1)
	return nil, &batchexecute.RejectedError{Reason: "PUBLIC_ERROR_MODEL_ACCESS_DENIED", Frames: 1}
}

type expansionTestRunner struct{ testRunner }

func (r *expansionTestRunner) Run(_ context.Context, job store.Job) (any, error) {
	r.calls.Add(1)
	var payload generationPayload
	_ = json.Unmarshal([]byte(job.Payload), &payload)
	return map[string]any{"job_id": job.ID, "account_id": payload.AccountID, "status": "succeeded"}, nil
}

func (r *expansionTestRunner) ExpandForQueue(_ context.Context, job store.Job) ([]store.Job, bool, error) {
	var payload generationPayload
	_ = json.Unmarshal([]byte(job.Payload), &payload)
	if job.Kind != "image" || !payload.AllAccounts {
		return nil, false, nil
	}
	return []store.Job{{ID: "expanded-1", Kind: "image", Payload: `{"account_id":"acct-1","count":1}`}, {ID: "expanded-2", Kind: "image", Payload: `{"account_id":"acct-1","count":1}`}}, true, nil
}

func (r *resumeTestRunner) Run(context.Context, store.Job) (any, error) {
	r.runCalls.Add(1)
	return nil, nil
}

func (r *resumeTestRunner) ResumeWithProgress(_ context.Context, _ store.Job, attempt store.JobAttempt, _ AttemptReporter) (any, error) {
	r.resumeCalls.Add(1)
	if len(attempt.UpstreamIDs) != 1 || attempt.UpstreamIDs[0] != "accepted-1" {
		return nil, fmt.Errorf("unexpected resume ids: %v", attempt.UpstreamIDs)
	}
	return map[string]any{"media_ids": attempt.UpstreamIDs, "status": "ready"}, nil
}

func newTestJobs(t *testing.T) (*Jobs, *testRunner) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	runner := &testRunner{}
	jobs, err := NewJobs(st, runner, "test-token", 2)
	if err != nil {
		t.Fatal(err)
	}
	return jobs, runner
}

func TestSetMaxAttemptsValidatesTotalExecutionBudget(t *testing.T) {
	jobs, _ := newTestJobs(t)
	for _, value := range []int{1, 3, 20} {
		if err := jobs.SetMaxAttempts(value); err != nil {
			t.Errorf("SetMaxAttempts(%d): %v", value, err)
		}
	}
	for _, value := range []int{0, -1, 21} {
		if err := jobs.SetMaxAttempts(value); err == nil {
			t.Errorf("SetMaxAttempts(%d) should reject invalid budget", value)
		}
	}
}

func TestSetRetryBudgetValidatesNamedClass(t *testing.T) {
	jobs, _ := newTestJobs(t)
	for _, value := range []int{1, 3, 20} {
		if err := jobs.SetRetryBudget("resume_media", value); err != nil {
			t.Errorf("SetRetryBudget(%d): %v", value, err)
		}
	}
	for _, tc := range []struct {
		class string
		limit int
	}{{"", 1}, {"default", 1}, {"resume_media", 0}, {"resume_media", 21}} {
		if err := jobs.SetRetryBudget(tc.class, tc.limit); err == nil {
			t.Errorf("SetRetryBudget(%q, %d) should reject invalid budget", tc.class, tc.limit)
		}
	}
}

func TestAccountUnavailableErrorCodeSurvivesJobStatusAPI(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "unavailable.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	jobs, err := NewJobs(st, unavailableAccountRunner{}, "test-token", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.CreateJob(context.Background(), store.Job{
		ID: "unavailable-job", Kind: "image", Payload: `{"prompt":"x"}`,
	}); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(context.Background(), "test-worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=(%+v,%v), want job", claimed, err)
	}
	jobs.execute(context.Background(), claimed.LeaseOwner, claimed)

	req := httptest.NewRequest(http.MethodGet, "/internal/v1/jobs/unavailable-job", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	var status map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode job status %q: %v", res.Body.String(), err)
	}
	if res.Code != http.StatusOK || status["status"] != "failed" || status["error_code"] != float64(http.StatusServiceUnavailable) {
		t.Fatalf("job status response=(%d,%v), want failed with error_code 503", res.Code, status)
	}
}

func TestQueueWorkersRunOtherAccountWhilePinnedAccountIsBusy(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "account-aware-queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	runner := &accountAwareQueueRunner{
		firstAccountJob: make(chan struct{}), secondAccountJob: make(chan struct{}),
		otherAccountJob: make(chan struct{}), releaseAccountA: make(chan struct{}),
	}
	jobs, err := NewJobs(st, runner, "test-token", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []store.Job{
		{ID: "01-a-running", Kind: "image", Payload: `{"prompt":"a1","account_id":"account-a"}`},
		{ID: "02-a-waiting", Kind: "image", Payload: `{"prompt":"a2","account_id":"account-a"}`},
		{ID: "03-b-ready", Kind: "image", Payload: `{"prompt":"b1","account_id":"account-b"}`},
	} {
		if _, _, err := st.CreateJob(context.Background(), job); err != nil {
			t.Fatalf("create %s: %v", job.ID, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-runner.releaseAccountA:
		default:
			close(runner.releaseAccountA)
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("queue workers did not stop")
		}
	}()

	select {
	case <-runner.firstAccountJob:
	case <-time.After(time.Second):
		t.Fatal("first account-a job did not start")
	}
	select {
	case <-runner.otherAccountJob:
	case <-time.After(time.Second):
		t.Fatal("account-b job was blocked behind queued work for busy account-a")
	}
	select {
	case <-runner.secondAccountJob:
		t.Fatal("second account-a job started while account-a was still occupied")
	case <-time.After(100 * time.Millisecond):
	}

	close(runner.releaseAccountA)
	select {
	case <-runner.secondAccountJob:
	case <-time.After(time.Second):
		t.Fatal("deferred account-a job did not run after its account became free")
	}
}

func TestInstanceLockHasOneOwnerAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "flow.db.service.lock")
	first, err := AcquireInstanceLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireInstanceLock(path); err == nil {
		t.Fatal("second owner acquired the same lock")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireInstanceLock(path)
	if err != nil {
		t.Fatalf("lock was not released: %v", err)
	}
	_ = second.Close()
}

func TestJobsAPIRequiresToken(t *testing.T) {
	jobs, _ := newTestJobs(t)
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":"x"}}`))
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want unauthorized", res.Code)
	}
}

func TestJobsAPIAcceptsImageCountAsOneQueuedJob(t *testing.T) {
	jobs, _ := newTestJobs(t)
	jobs.runner = &expansionTestRunner{}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":"x","count":2}}`))
	req.Header.Set("Authorization", "Bearer test-token")
	req.Header.Set("Idempotency-Key", "expanded-request")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("create status=%d body=%s", res.Code, res.Body.String())
	}
	var accepted struct {
		ID     string `json:"job_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	job, err := jobs.store.JobByID(context.Background(), accepted.ID)
	children, childrenErr := jobs.store.ChildJobs(context.Background(), accepted.ID)
	if err != nil || childrenErr != nil || accepted.Status != "queued" || job.Status != "queued" || len(children) != 0 {
		t.Fatalf("accepted job=%+v response=%+v children=%+v errors=(%v,%v)", job, accepted, children, err, childrenErr)
	}
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil || payload.Count != 2 {
		t.Fatalf("queued payload=%s count=%d err=%v, want count preserved as 2", job.Payload, payload.Count, err)
	}
}

func TestExpandedParentCompletesAfterBoundedChildJobs(t *testing.T) {
	jobs, _ := newTestJobs(t)
	runner := &expansionTestRunner{}
	jobs.runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("workers did not stop")
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":"x","all_accounts":true}}`))
	req.Header.Set("Authorization", "Bearer test-token")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("submit status=%d body=%s", res.Code, res.Body.String())
	}
	var accepted struct {
		ID string `json:"job_id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil || accepted.ID == "" {
		t.Fatalf("accepted=%q err=%v", res.Body.String(), err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.store.JobByID(context.Background(), accepted.ID)
		if err == nil && job.Status == "succeeded" {
			var result struct {
				Results []struct {
					AccountID string `json:"account_id"`
				} `json:"results"`
			}
			if err := json.Unmarshal([]byte(job.Result), &result); err != nil || len(result.Results) != 2 || result.Results[0].AccountID == "" || result.Results[1].AccountID == "" {
				t.Fatalf("parent result=%s err=%v", job.Result, err)
			}
			if runner.calls.Load() != 2 {
				t.Fatalf("child executions=%d want 2", runner.calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	parent, _ := jobs.store.JobByID(context.Background(), accepted.ID)
	children, _ := jobs.store.ChildJobs(context.Background(), accepted.ID)
	t.Fatalf("parent job did not finish after its children: parent=%+v children=%+v calls=%d", parent, children, runner.calls.Load())
}

func TestAggregateImageCountChildrenReturnsPartialResults(t *testing.T) {
	parent := store.Job{ID: "count-parent", Kind: "image", Payload: `{"count":2}`}
	children := []store.Job{
		{ID: "child-1", Kind: "image", Payload: `{"account_id":"acct-1"}`, Status: "succeeded", Result: `{"job_id":"child-1","media":[{"media_id":"m1"}],"files":[{"path":"f1"}],"elapsed_seconds":1}`},
		{ID: "child-2", Kind: "image", Payload: `{"account_id":"acct-1"}`, Status: "failed", Error: "upstream error"},
	}
	value, status, message := aggregateChildren(parent, children)
	result := value.(map[string]any)
	if status != "partial" || result["status"] != "partial" || message == "" {
		t.Fatalf("aggregate status=%q message=%q result=%v", status, message, result)
	}
	if len(result["media"].([]any)) != 1 || len(result["files"].([]any)) != 1 || len(result["child_job_ids"].([]any)) != 2 {
		t.Fatalf("aggregate lost successful child output: %v", result)
	}
}

func TestAggregateAllAccountsGroupsImageCountPerAccount(t *testing.T) {
	parent := store.Job{ID: "parent", Kind: "image", Payload: `{"all_accounts":true,"count":2}`}
	children := []store.Job{
		{ID: "a-1", Kind: "image", Payload: `{"account_id":"a"}`, Status: "succeeded", Result: `{"job_id":"a1","media":[{"media_id":"a-media-1"}],"files":[],"elapsed_seconds":1}`},
		{ID: "a-2", Kind: "image", Payload: `{"account_id":"a"}`, Status: "failed", Error: "one image failed"},
		{ID: "b-1", Kind: "image", Payload: `{"account_id":"b"}`, Status: "succeeded", Result: `{"job_id":"b1","media":[{"media_id":"b-media-1"}],"files":[],"elapsed_seconds":1}`},
		{ID: "b-2", Kind: "image", Payload: `{"account_id":"b"}`, Status: "succeeded", Result: `{"job_id":"b2","media":[{"media_id":"b-media-2"}],"files":[],"elapsed_seconds":1}`},
	}
	value, status, _ := aggregateChildren(parent, children)
	response := value.(map[string]any)
	results := response["results"].([]map[string]any)
	if status != "partial" || len(results) != 2 {
		t.Fatalf("aggregate status=%s results=%v", status, results)
	}
	accountA := results[0]["result"].(map[string]any)
	accountB := results[1]["result"].(map[string]any)
	if accountA["account_id"] != "a" || accountA["status"] != "partial" || len(accountA["media"].([]any)) != 1 {
		t.Fatalf("account a aggregate=%v", accountA)
	}
	if accountB["account_id"] != "b" || accountB["status"] != "succeeded" || len(accountB["media"].([]any)) != 2 {
		t.Fatalf("account b aggregate=%v", accountB)
	}
}

func TestExpiredAcceptedJobRunsResumePathWithoutResubmitting(t *testing.T) {
	for _, phase := range []string{"rendering", "downloading", "download_complete"} {
		t.Run(phase, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "jobs.db")
			st, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			runner := &resumeTestRunner{}
			jobs, err := NewJobs(st, runner, "test-token", 2)
			if err != nil {
				t.Fatal(err)
			}
			jobs.runner = runner
			ctx := context.Background()
			if _, _, err := st.CreateJob(ctx, store.Job{ID: "resume-me", Kind: "video", Payload: `{"prompt":"x"}`}); err != nil {
				t.Fatal(err)
			}
			if _, err := st.ClaimNextJob(ctx, "dead-worker", time.Minute); err != nil {
				t.Fatal(err)
			}
			if err := st.StartJobAttempt(ctx, "resume-me", 1); err != nil {
				t.Fatal(err)
			}
			if err := st.UpdateJobAttempt(ctx, "resume-me", 1, phase, "acct-1", "project-1", "rev-1", ""); err != nil {
				t.Fatal(err)
			}
			if err := st.SetJobAttemptUpstreamIDs(ctx, "resume-me", 1, []string{"accepted-1"}); err != nil {
				t.Fatal(err)
			}
			admin, err := sql.Open("sqlite", st.DatabasePath())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := admin.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='resume-me'`, time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano)); err != nil {
				_ = admin.Close()
				t.Fatal(err)
			}
			if err := admin.Close(); err != nil {
				t.Fatal(err)
			}
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}

			// Reopen the durable database as a fresh service process would. The new
			// worker must resume accepted media instead of submitting it again.
			restartedStore, err := store.Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer restartedStore.Close()
			restartedJobs, err := NewJobs(restartedStore, runner, "test-token", 2)
			if err != nil {
				t.Fatal(err)
			}
			claimed, err := restartedStore.ClaimNextJob(ctx, "resume-worker", time.Minute)
			if err != nil || claimed == nil || claimed.ResumeAttempt == nil {
				t.Fatalf("claim=(%+v,%v), want resume attempt", claimed, err)
			}
			if claimed.ResumeAttempt.Phase != phase {
				t.Fatalf("recovered phase=%q, want %q", claimed.ResumeAttempt.Phase, phase)
			}
			restartedJobs.execute(ctx, claimed.LeaseOwner, claimed)
			got, err := restartedStore.JobByID(ctx, "resume-me")
			if err != nil || got.Status != "succeeded" {
				t.Fatalf("resumed job=(%+v,%v)", got, err)
			}
			if runner.runCalls.Load() != 0 || runner.resumeCalls.Load() != 1 {
				t.Fatalf("run calls=%d, resume calls=%d; want no generation replay and one resume", runner.runCalls.Load(), runner.resumeCalls.Load())
			}
		})
	}
}

func TestBoundAttemptStaysPreparingUntilExplicitSubmitPhase(t *testing.T) {
	jobs, _ := newTestJobs(t)
	ctx := context.Background()
	if _, _, err := jobs.store.CreateJob(ctx, store.Job{ID: "preflight-job", Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
		t.Fatal(err)
	}
	claimed, err := jobs.store.ClaimNextJob(ctx, "preflight-worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=(%+v,%v)", claimed, err)
	}
	if err := jobs.store.StartJobAttempt(ctx, claimed.ID, 1); err != nil {
		t.Fatal(err)
	}
	progress := &jobAttemptProgress{store: jobs.store, jobID: claimed.ID, attemptNo: 1}
	if err := progress.BindAccount("account-a", "project-a", "revision-1"); err != nil {
		t.Fatal(err)
	}
	attempts, err := jobs.store.JobAttempts(ctx, claimed.ID)
	if err != nil || len(attempts) != 1 || attempts[0].Phase != "preparing" || attempts[0].AccountID != "account-a" {
		t.Fatalf("bound attempt=(%+v,%v), want preparing with bound account", attempts, err)
	}
	if err := progress.SetPhase("submitting", "sending generation RPC"); err != nil {
		t.Fatal(err)
	}
	attempts, err = jobs.store.JobAttempts(ctx, claimed.ID)
	if err != nil || len(attempts) != 1 || attempts[0].Phase != "submitting" {
		t.Fatalf("submitted attempt=(%+v,%v), want submitting", attempts, err)
	}
}

func TestDeadlineRetriesOnlyBeforeDurableSubmitBoundary(t *testing.T) {
	for _, tc := range []struct {
		name            string
		markSubmitting  bool
		preSubmitBudget int
		wantStatus      string
		wantCalls       int32
	}{
		{name: "preparing", wantStatus: "succeeded", wantCalls: 2},
		{name: "submitting", markSubmitting: true, wantStatus: "failed", wantCalls: 1},
		{name: "preparing_budget_one", preSubmitBudget: 1, wantStatus: "failed", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			jobs, _ := newTestJobs(t)
			if tc.preSubmitBudget != 0 {
				if err := jobs.SetRetryBudget(retryClassPreSubmit, tc.preSubmitBudget); err != nil {
					t.Fatal(err)
				}
			}
			runner := &deadlineRetryRunner{markSubmitting: tc.markSubmitting}
			jobs.runner = runner
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() { jobs.Start(ctx); close(done) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Error("job workers did not stop")
				}
			})
			if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{
				ID: "deadline-" + tc.name, Kind: "video", Payload: `{"prompt":"x"}`,
			}); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				got, err := jobs.store.JobByID(context.Background(), "deadline-"+tc.name)
				if err == nil && (got.Status == "succeeded" || got.Status == "failed") {
					if got.Status != tc.wantStatus {
						t.Fatalf("job status=%s, want %s (job=%+v)", got.Status, tc.wantStatus, got)
					}
					if got.Attempts != int(tc.wantCalls) || runner.calls.Load() != tc.wantCalls {
						t.Fatalf("attempts=%d runner calls=%d, want %d", got.Attempts, runner.calls.Load(), tc.wantCalls)
					}
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
			got, _ := jobs.store.JobByID(context.Background(), "deadline-"+tc.name)
			t.Fatalf("job did not reach terminal status: %+v", got)
		})
	}
}

func TestHTTPGenerationThrottleUsesBoundedAccountThrottleBudget(t *testing.T) {
	jobs, _ := newTestJobs(t)
	if err := jobs.SetRetryBudget("account_throttle", 2); err != nil {
		t.Fatal(err)
	}
	runner := &httpThrottleRetryRunner{}
	jobs.runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("job workers did not stop")
		}
	})
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{
		ID: "http-throttle-retry", Kind: "image", Payload: `{"prompt":"x"}`,
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := jobs.store.JobByID(context.Background(), "http-throttle-retry")
		if err == nil && (got.Status == "succeeded" || got.Status == "failed") {
			if got.Status != "succeeded" || got.Attempts != 2 || runner.calls.Load() != 2 {
				t.Fatalf("job=(%+v,%v), calls=%d; want success after bounded second attempt", got, err, runner.calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := jobs.store.JobByID(context.Background(), "http-throttle-retry")
	t.Fatalf("job=(%+v,%v), calls=%d; did not settle", got, err, runner.calls.Load())
}

func TestAcceptedMediaFailureUsesResumePathInsteadOfResubmitting(t *testing.T) {
	jobs, _ := newTestJobs(t)
	runner := &acceptedThenTimeoutRunner{}
	jobs.runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("job workers did not stop")
		}
	})
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{
		ID: "accepted-timeout", Kind: "video", Payload: `{"prompt":"x"}`,
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, err := jobs.store.JobByID(context.Background(), "accepted-timeout")
		if err == nil && (got.Status == "succeeded" || got.Status == "failed") {
			if got.Status != "succeeded" || got.Attempts != 2 || runner.runCalls.Load() != 1 || runner.resumeCalls.Load() != 1 {
				t.Fatalf("job=(%+v,%v) run=%d resume=%d; want one submit followed by one resume", got, err, runner.runCalls.Load(), runner.resumeCalls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := jobs.store.JobByID(context.Background(), "accepted-timeout")
	t.Fatalf("job=(%+v,%v) run=%d resume=%d; did not settle", got, err, runner.runCalls.Load(), runner.resumeCalls.Load())
}

func TestJobsCanBeRecoveredByIdempotencyKey(t *testing.T) {
	jobs, _ := newTestJobs(t)
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{
		ID: "job-recover-key", Kind: "image", Payload: `{"prompt":"x"}`, IdempotencyKey: "client-retry-key",
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/jobs/by-key?key=client-retry-key", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil || res.Code != http.StatusOK || body["job_id"] != "job-recover-key" {
		t.Fatalf("lookup response=(%d,%s), err=%v", res.Code, res.Body.String(), err)
	}

	missing := httptest.NewRequest(http.MethodGet, "/internal/v1/jobs/by-key?key=not-found", nil)
	missing.Header.Set("Authorization", "Bearer test-token")
	missingRes := httptest.NewRecorder()
	jobs.ServeHTTP(missingRes, missing)
	if missingRes.Code != http.StatusNotFound {
		t.Fatalf("missing lookup status=%d, want 404", missingRes.Code)
	}
}

func TestJobsAPICancelsQueuedJob(t *testing.T) {
	jobs, _ := newTestJobs(t)
	create := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":"x"}}`))
	create.Header.Set("Authorization", "Bearer test-token")
	created := httptest.NewRecorder()
	jobs.ServeHTTP(created, create)
	var accepted struct {
		ID string `json:"job_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &accepted); err != nil || accepted.ID == "" {
		t.Fatalf("create response %q: %v", created.Body.String(), err)
	}

	cancel := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs/"+accepted.ID+"/cancel", nil)
	cancel.Header.Set("Authorization", "Bearer test-token")
	cancelled := httptest.NewRecorder()
	jobs.ServeHTTP(cancelled, cancel)
	if cancelled.Code != http.StatusNoContent {
		t.Fatalf("cancel status=%d body=%s", cancelled.Code, cancelled.Body.String())
	}
	job, err := jobs.store.JobByID(context.Background(), accepted.ID)
	if err != nil || job.Status != "cancelled" {
		t.Fatalf("job after cancellation = (%+v, %v)", job, err)
	}
}

func TestJobEventsAPIStreamsDurableSequenceAndResumes(t *testing.T) {
	jobs, _ := newTestJobs(t)
	ctx := context.Background()
	if _, _, err := jobs.store.CreateJob(ctx, store.Job{
		ID: "stream-job", Kind: "image", Payload: `{"prompt":"private prompt"}`,
	}); err != nil {
		t.Fatal(err)
	}
	createdEvents, err := jobs.store.JobEventsAfter(ctx, "stream-job", 0, 128)
	if err != nil || len(createdEvents) != 1 || createdEvents[0].Type != "status" {
		t.Fatalf("created job events=(%+v,%v), want one queued status event", createdEvents, err)
	}
	claimed, err := jobs.store.ClaimNextJob(ctx, "stream-worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=(%+v,%v)", claimed, err)
	}
	if err := jobs.store.StartJobAttempt(ctx, claimed.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := jobs.store.UpdateJobAttempt(ctx, claimed.ID, 1, "submitting", "account-a", "project-a", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if err := jobs.store.FinishJob(ctx, claimed.ID, claimed.LeaseOwner, "succeeded", `{"ok":true}`, ""); err != nil {
		t.Fatal(err)
	}

	stream := func(lastID string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/jobs/stream-job/events", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		if lastID != "" {
			req.Header.Set("Last-Event-ID", lastID)
		}
		res := httptest.NewRecorder()
		jobs.ServeHTTP(res, req)
		return res
	}
	first := stream("")
	if first.Code != http.StatusOK || first.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("SSE response=(%d,%v) body=%q", first.Code, first.Header(), first.Body.String())
	}
	pattern := regexp.MustCompile(`(?m)^id: ([0-9]+)\nevent: ([a-z_]+)\ndata: (.+)$`)
	matches := pattern.FindAllStringSubmatch(first.Body.String(), -1)
	if len(matches) < 5 {
		t.Fatalf("SSE events=%d body=%q, want queued/running/attempt/submitting/succeeded", len(matches), first.Body.String())
	}
	ids := make([]int64, len(matches))
	for i, match := range matches {
		ids[i], err = strconv.ParseInt(match[1], 10, 64)
		if err != nil || ids[i] <= 0 || (i > 0 && ids[i] <= ids[i-1]) {
			t.Fatalf("event IDs are not strictly increasing: %v (parse error %v)", ids, err)
		}
	}
	if !strings.Contains(first.Body.String(), `"status":"queued"`) ||
		!strings.Contains(first.Body.String(), `"status":"running"`) ||
		!strings.Contains(first.Body.String(), `"phase":"preparing"`) ||
		!strings.Contains(first.Body.String(), `"phase":"submitting"`) ||
		!strings.Contains(first.Body.String(), `"status":"succeeded"`) {
		t.Fatalf("SSE stream missing durable lifecycle events: %s", first.Body.String())
	}
	if strings.Contains(first.Body.String(), "private prompt") {
		t.Fatal("event stream exposed the job payload")
	}

	cursor := strconv.FormatInt(ids[1], 10)
	resumed := stream(cursor)
	if resumed.Code != http.StatusOK || strings.Contains(resumed.Body.String(), "\nid: "+strconv.FormatInt(ids[0], 10)+"\n") {
		t.Fatalf("resumed stream did not honor Last-Event-ID %s: (%d,%q)", cursor, resumed.Code, resumed.Body.String())
	}
	resumedEvents := pattern.FindAllStringSubmatch(resumed.Body.String(), -1)
	if len(resumedEvents) == 0 {
		t.Fatalf("resumed stream returned no events: %q", resumed.Body.String())
	}
	for _, match := range resumedEvents {
		id, parseErr := strconv.ParseInt(match[1], 10, 64)
		if parseErr != nil || id <= ids[1] {
			t.Fatalf("resumed stream replayed cursor or older event: %q", match[0])
		}
	}
	if final := stream(strconv.FormatInt(ids[len(ids)-1], 10)); pattern.MatchString(final.Body.String()) {
		t.Fatalf("stream after final cursor replayed an event: %q", final.Body.String())
	}
	invalid := stream("-1")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("negative Last-Event-ID status=%d, want 400", invalid.Code)
	}
}

func TestAcceptedJobSurvivesSubmitRequestDisconnect(t *testing.T) {
	jobs, _ := newTestJobs(t)
	requestCtx, disconnect := context.WithCancel(context.Background())
	defer disconnect()
	create := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":"x"}}`)).WithContext(requestCtx)
	create.Header.Set("Authorization", "Bearer test-token")
	created := httptest.NewRecorder()
	jobs.ServeHTTP(created, create)
	if created.Code != http.StatusAccepted {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var accepted struct {
		ID string `json:"job_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &accepted); err != nil || accepted.ID == "" {
		t.Fatalf("create response %q: %v", created.Body.String(), err)
	}

	// Model the client dropping its connection after receiving the durable
	// acknowledgement. Request cancellation must not be treated as job cancel.
	disconnect()
	job, err := jobs.store.JobByID(context.Background(), accepted.ID)
	if err != nil || job.Status != "queued" {
		t.Fatalf("accepted job after submit disconnect=(%+v,%v), want queued", job, err)
	}
}

func TestGracefulShutdownRequeuesJobStillPreparing(t *testing.T) {
	jobs, _ := newTestJobs(t)
	runner := &preparingRunner{started: make(chan struct{})}
	jobs.runner = runner
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{ID: "job-shutdown-prep", Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("job did not enter preparing")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("service workers did not stop")
	}
	job, err := jobs.store.JobByID(context.Background(), "job-shutdown-prep")
	if err != nil || job.Status != "queued" {
		t.Fatalf("job after shutdown=(%+v,%v), want queued", job, err)
	}
	attempts, err := jobs.store.JobAttempts(context.Background(), "job-shutdown-prep")
	if err != nil || len(attempts) != 1 || attempts[0].Phase != "interrupted_before_submit" {
		t.Fatalf("attempts after shutdown=(%+v,%v), want interrupted_before_submit", attempts, err)
	}
}

func TestStopClaimingDrainsActiveJobsAndLeavesQueuedJobsForRecovery(t *testing.T) {
	jobs, _ := newTestJobs(t)
	jobs.workers = 1
	runner := &drainRunner{started: make(chan struct{}), release: make(chan struct{})}
	jobs.runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	for _, id := range []string{"drain-active", "drain-queued"} {
		if _, _, err := jobs.store.CreateJob(ctx, store.Job{ID: id, Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
			cancel()
			t.Fatal(err)
		}
	}
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("first job did not start")
	}
	jobs.StopClaiming()
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	err := jobs.Drain(drainCtx)
	drainCancel()
	if err != context.DeadlineExceeded {
		cancel()
		t.Fatalf("drain before active job release=%v, want deadline", err)
	}
	close(runner.release)
	if err := jobs.Drain(context.Background()); err != nil {
		cancel()
		t.Fatalf("drain after active job completion: %v", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("service workers did not stop after drain")
	}
	active, activeErr := jobs.store.JobByID(context.Background(), "drain-active")
	queued, queuedErr := jobs.store.JobByID(context.Background(), "drain-queued")
	if activeErr != nil || active.Status != "succeeded" {
		t.Fatalf("active job after drain=(%+v,%v), want succeeded", active, activeErr)
	}
	if queuedErr != nil || queued.Status != "queued" {
		t.Fatalf("unclaimed job after drain=(%+v,%v), want queued for restart", queued, queuedErr)
	}
}

func TestShutdownPreservesAcceptedMediaForResume(t *testing.T) {
	jobs, _ := newTestJobs(t)
	jobs.workers = 1
	runner := &acceptedDrainRunner{started: make(chan struct{})}
	jobs.runner = runner
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{ID: "shutdown-accepted", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("accepted job did not reach rendering")
	}
	jobs.StopClaiming()
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	if err := jobs.Drain(drainCtx); err != context.DeadlineExceeded {
		drainCancel()
		cancel()
		t.Fatalf("drain active accepted work=%v, want deadline", err)
	}
	drainCancel()
	cancel() // App cancels execution after its bounded drain expires.
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("service worker did not stop after drain deadline")
	}
	job, err := jobs.store.JobByID(context.Background(), "shutdown-accepted")
	if err != nil || job.Status != "queued" {
		t.Fatalf("job after shutdown=(%+v,%v), want queued for resume", job, err)
	}
	resumed, err := jobs.store.ClaimNextJob(context.Background(), "restart-worker", time.Minute)
	if err != nil || resumed == nil || resumed.ResumeAttempt == nil || resumed.ResumeAttempt.UpstreamIDs[0] != "media-a" {
		t.Fatalf("recovered accepted media=(%+v,%v), want persisted resume metadata", resumed, err)
	}
}

func TestShutdownDoesNotReplaySubmissionWithUnknownAcceptance(t *testing.T) {
	jobs, _ := newTestJobs(t)
	jobs.workers = 1
	runner := &submittingDrainRunner{started: make(chan struct{})}
	jobs.runner = runner
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{ID: "shutdown-unknown", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	select {
	case <-runner.started:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("job did not reach ambiguous submit phase")
	}
	jobs.StopClaiming()
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("service worker did not stop after shutdown")
	}
	job, err := jobs.store.JobByID(context.Background(), "shutdown-unknown")
	if err != nil || job.Status != "submission_unknown" || job.Attempts != 1 {
		t.Fatalf("job after ambiguous shutdown=(%+v,%v), want one terminal submission_unknown attempt", job, err)
	}
	if claimed, err := jobs.store.ClaimNextJob(context.Background(), "restart-worker", time.Minute); err != nil || claimed != nil {
		t.Fatalf("restart claim=(%+v,%v), ambiguous submission must not replay", claimed, err)
	}
}

func TestAmbiguousSubmissionIsPersistedWithoutRetry(t *testing.T) {
	jobs, _ := newTestJobs(t)
	runner := &ambiguousRunner{}
	jobs.runner = runner
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{ID: "job-unknown", Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("service workers did not stop")
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobs.store.JobByID(context.Background(), "job-unknown")
		if err == nil && job.Status == "submission_unknown" {
			if runner.calls.Load() != 1 {
				t.Fatalf("runner calls=%d, want exactly one", runner.calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, err := jobs.store.JobByID(context.Background(), "job-unknown")
	t.Fatalf("job did not reach submission_unknown: (%+v,%v)", job, err)
}

func TestModelPolicyRefusalIsTerminalWithoutRetry(t *testing.T) {
	jobs, _ := newTestJobs(t)
	runner := &refusalRunner{}
	jobs.runner = runner
	if _, _, err := jobs.store.CreateJob(context.Background(), store.Job{
		ID: "job-policy-refusal", Kind: "image", Payload: `{"prompt":"disallowed"}`,
	}); err != nil {
		t.Fatal(err)
	}
	claimed, err := jobs.store.ClaimNextJob(context.Background(), "test-worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=(%+v,%v), want job", claimed, err)
	}
	jobs.execute(context.Background(), claimed.LeaseOwner, claimed)

	got, err := jobs.store.JobByID(context.Background(), "job-policy-refusal")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.Attempts != 1 || runner.calls.Load() != 1 {
		t.Fatalf("policy refusal job=(status=%q attempts=%d runner_calls=%d), want failed once without retry",
			got.Status, got.Attempts, runner.calls.Load())
	}
	if claimed, err := jobs.store.ClaimNextJob(context.Background(), "retry-worker", time.Minute); err != nil || claimed != nil {
		t.Fatalf("claim after policy refusal=(%+v,%v), want no automatic replay", claimed, err)
	}
}

func TestJobsAPIIsIdempotentAndRunsPersistedJob(t *testing.T) {
	jobs, runner := newTestJobs(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("job workers did not stop")
		}
	})

	create := func(payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Idempotency-Key", "request-1")
		res := httptest.NewRecorder()
		jobs.ServeHTTP(res, req)
		return res
	}
	first := create(`{"kind":"image","payload":{"prompt":"secret prompt"}}`)
	if first.Code != http.StatusAccepted {
		t.Fatalf("first submit status = %d: %s", first.Code, first.Body.String())
	}
	var accepted struct {
		ID string `json:"job_id"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &accepted); err != nil || accepted.ID == "" {
		t.Fatalf("submit response = %q, err=%v", first.Body.String(), err)
	}
	second := create(`{"kind":"image","payload":{"prompt":"secret prompt"}}`)
	if second.Code != http.StatusOK {
		t.Fatalf("idempotent submit status = %d: %s", second.Code, second.Body.String())
	}
	different := create(`{"kind":"image","payload":{"prompt":"different"}}`)
	if different.Code != http.StatusConflict {
		t.Fatalf("different payload status = %d, want conflict", different.Code)
	}
	unsupportedVersion := create(`{"kind":"image","payload":{"prompt":"secret prompt"},"payload_version":2}`)
	if unsupportedVersion.Code != http.StatusBadRequest {
		t.Fatalf("unsupported payload version status = %d, want bad request", unsupportedVersion.Code)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/jobs/"+accepted.ID, nil)
		req.Header.Set("Authorization", "Bearer test-token")
		res := httptest.NewRecorder()
		jobs.ServeHTTP(res, req)
		if res.Code == http.StatusOK && strings.Contains(res.Body.String(), `"status":"succeeded"`) {
			if strings.Contains(res.Body.String(), "secret prompt") {
				t.Fatal("job status leaked its stored request payload")
			}
			var view struct {
				PayloadVersion int `json:"payload_version"`
				History        []struct {
					Phase     string `json:"phase"`
					AccountID string `json:"account_id"`
				} `json:"attempt_history"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &view); err != nil || len(view.History) != 1 || view.History[0].Phase != "succeeded" {
				t.Fatalf("job attempt history = (%+v,%v), response=%s", view.History, err, res.Body.String())
			}
			if view.PayloadVersion != store.CurrentJobPayloadVersion {
				t.Fatalf("job payload version = %d, want %d", view.PayloadVersion, store.CurrentJobPayloadVersion)
			}
			if runner.calls.Load() != 1 {
				t.Fatalf("runner calls = %d, want one", runner.calls.Load())
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not reach succeeded status")
}

func TestAcceptedJobWakesIdleWorkerWithoutWaitingForPollInterval(t *testing.T) {
	previousPoll := queuePoll
	queuePoll = 5 * time.Second
	defer func() { queuePoll = previousPoll }()

	jobs, _ := newTestJobs(t)
	runner := &wakeSignalRunner{started: make(chan struct{})}
	jobs.runner = runner
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("queue workers did not stop")
		}
	}()
	time.Sleep(30 * time.Millisecond) // let workers enter their long idle wait

	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(
		`{"kind":"image","payload":{"prompt":"wake me"}}`))
	req.Header.Set("Authorization", "Bearer test-token")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("submit status=%d body=%s, want 202", res.Code, res.Body.String())
	}
	select {
	case <-runner.started:
	case <-time.After(time.Second):
		t.Fatal("accepted job waited for the five-second queue poll instead of waking a worker")
	}
}
