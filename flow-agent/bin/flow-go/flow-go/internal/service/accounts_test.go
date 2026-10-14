package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/batchexecute"
	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/config"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/engine"
	"github.com/kodelyx/flow-go/flow-go/internal/httpx"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

type schedulerStressRunner struct {
	manager    *AccountManager
	mu         sync.Mutex
	served     map[string]int
	active     map[string]int
	maxAccount map[string]int
	global     atomic.Int32
	maxGlobal  atomic.Int32
}

type authBurstServiceRunner struct {
	manager      *AccountManager
	accountBJobs atomic.Int32
}

func (r *authBurstServiceRunner) Run(ctx context.Context, job store.Job) (any, error) {
	var payload generationPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, err
	}
	if payload.AccountID == "account-b" {
		runtime, err := r.manager.acquireScheduledRuntime(ctx, payload.AccountID)
		if err != nil {
			return nil, err
		}
		defer r.manager.release(runtime)
		r.accountBJobs.Add(1)
		return map[string]any{"account_id": runtime.id, "status": "succeeded"}, nil
	}
	return r.manager.Run(ctx, job)
}

func (r *schedulerStressRunner) Run(ctx context.Context, _ store.Job) (any, error) {
	worker, err := r.manager.acquireScheduled(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	defer r.manager.scheduler.Release(worker, 0, nil)
	r.mu.Lock()
	r.served[worker.ID]++
	r.active[worker.ID]++
	if r.active[worker.ID] > r.maxAccount[worker.ID] {
		r.maxAccount[worker.ID] = r.active[worker.ID]
	}
	r.mu.Unlock()
	active := r.global.Add(1)
	for old := r.maxGlobal.Load(); active > old && !r.maxGlobal.CompareAndSwap(old, active); old = r.maxGlobal.Load() {
	}
	select {
	case <-ctx.Done():
		r.global.Add(-1)
		r.mu.Lock()
		r.active[worker.ID]--
		r.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(3 * time.Millisecond):
	}
	r.global.Add(-1)
	r.mu.Lock()
	r.active[worker.ID]--
	r.mu.Unlock()
	return map[string]string{"account_id": worker.ID}, nil
}

func TestOneHundredConcurrentJobsShareFiveAccountsSafely(t *testing.T) {
	manager := newSchedulerManager(t, 20)
	accountIDs := []string{"account-a", "account-b", "account-c", "account-d", "account-e"}
	for _, id := range accountIDs {
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		manager.scheduleWorkers[id] = worker
		manager.scheduler.Register(worker)
	}
	runner := &schedulerStressRunner{
		manager: manager, served: make(map[string]int), active: make(map[string]int),
		maxAccount: make(map[string]int),
	}
	jobs, err := NewJobs(manager.store, runner, "stress-token", 20)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("stress workers did not stop")
		}
	})

	const count = 100
	jobIDs := make([]string, count)
	var submitWG sync.WaitGroup
	for i := 0; i < count; i++ {
		submitWG.Add(1)
		go func(i int) {
			defer submitWG.Done()
			body := fmt.Sprintf(`{"kind":"image","payload":{"prompt":"load-%d"}}`, i)
			req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(body))
			req.Header.Set("Authorization", "Bearer stress-token")
			req.Header.Set("Idempotency-Key", fmt.Sprintf("stress-%d", i))
			res := httptest.NewRecorder()
			jobs.ServeHTTP(res, req)
			if res.Code != http.StatusAccepted {
				t.Errorf("submit %d status=%d body=%s", i, res.Code, res.Body.String())
				return
			}
			var accepted struct {
				JobID string `json:"job_id"`
			}
			if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil || accepted.JobID == "" {
				t.Errorf("submit %d acknowledgement=%q err=%v", i, res.Body.String(), err)
				return
			}
			jobIDs[i] = accepted.JobID
		}(i)
	}
	submitWG.Wait()
	for i, id := range jobIDs {
		if id == "" {
			t.Fatalf("job %d was not accepted", i)
		}
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		terminal := 0
		for _, id := range jobIDs {
			job, err := manager.store.JobByID(context.Background(), id)
			if err == nil && (job.Status == "succeeded" || job.Status == "failed" || job.Status == "submission_unknown") {
				if job.Status != "succeeded" {
					t.Fatalf("job %s ended %s: %s", id, job.Status, job.Error)
				}
				terminal++
			}
		}
		if terminal == count {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, id := range jobIDs {
		job, err := manager.store.JobByID(context.Background(), id)
		if err != nil || job.Status != "succeeded" {
			t.Fatalf("job %s final status=(%v,%v), want succeeded", id, job, err)
		}
	}
	if got := runner.maxGlobal.Load(); got < 2 {
		t.Fatalf("maximum cross-account concurrency=%d, want more than one", got)
	}
	runner.mu.Lock()
	defer runner.mu.Unlock()
	for _, id := range accountIDs {
		if runner.served[id] == 0 {
			t.Errorf("account %s served no jobs", id)
		}
		if runner.maxAccount[id] > 1 {
			t.Errorf("account %s ran %d jobs concurrently", id, runner.maxAccount[id])
		}
	}
}

func newSchedulerManager(t *testing.T, workers int) *AccountManager {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "scheduler.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	br := bridge.NewBridge(nil, nil, t.TempDir())
	manager, err := NewAccountManager(st, br, workers)
	if err != nil {
		t.Fatal(err)
	}
	manager.strictFresh = false
	manager.initOnce.Do(func() { close(manager.initialized) })
	for _, id := range []string{"account-a", "account-b"} {
		gate := make(chan struct{}, 1)
		manager.accountGates[id] = gate
		manager.accounts[id] = &accountRuntime{id: id, gate: gate}
	}
	return manager
}

func TestKnownReferenceMediaCannotCrossAccounts(t *testing.T) {
	m := newSchedulerManager(t, 1)
	rowID, err := m.store.RecordGeneration(store.Generation{JobID: "owner-job", AccountID: "account-a", Kind: "image"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.store.CreateJob(context.Background(), store.Job{ID: "owner-job", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if err := m.store.StartJobAttempt(context.Background(), "owner-job", 1); err != nil {
		t.Fatal(err)
	}
	if err := m.store.UpdateJobAttempt(context.Background(), "owner-job", 1, "succeeded", "account-a", "project-a", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := m.store.RecordMedia(store.Media{GenerationID: &rowID, MediaID: "account-a-media", Kind: "image"}); err != nil {
		t.Fatal(err)
	}
	otherAccount, err := engine.New(m.store, m.bridge.AccountView("account-b"), engine.Options{AccountID: "account-b"})
	if err != nil {
		t.Fatal(err)
	}
	err = m.validateReferenceOwnership(context.Background(), otherAccount, generationPayload{References: []string{"account-a-media"}})
	if err == nil || !strings.Contains(err.Error(), `belongs to account "account-a"`) {
		t.Fatalf("cross-account reference error=%v, want ownership refusal", err)
	}
	autoPinned := generationPayload{References: []string{"account-a-media"}}
	if err := m.bindReferenceAccount(context.Background(), &autoPinned); err != nil || autoPinned.AccountID != "account-a" {
		t.Fatalf("known media affinity=(%q,%v), want account-a", autoPinned.AccountID, err)
	}
	if err := m.validateReferenceOwnership(context.Background(), otherAccount, generationPayload{References: []string{"external-media"}}); err != nil {
		t.Fatalf("unknown external media should remain caller-addressable: %v", err)
	}
	if err := m.ValidateJobPayload(context.Background(), store.Job{Kind: "video", Payload: `{"prompt":"x","references":["external-media"]}`}); err == nil || !strings.Contains(err.Error(), "explicit account_id") {
		t.Fatalf("unowned reference validation error=%v, want explicit account requirement", err)
	}
	if err := m.ValidateJobPayload(context.Background(), store.Job{Kind: "video", Payload: `{"prompt":"x","references":["external-media"],"account_id":"account-b"}`}); err != nil {
		t.Fatalf("explicitly pinned external reference rejected: %v", err)
	}
}

func TestMediaLimitsAreSharedManagerResources(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "media-limits.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	manager, err := NewAccountManagerWithMediaLimits(st, bridge.NewBridge(nil, nil, t.TempDir()), 4, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if cap(manager.global) != 4 || cap(manager.mediaPollGate) != 3 || cap(manager.mediaDownloadGate) != 2 {
		t.Fatalf("manager limits: submit=%d poll=%d download=%d, want 4/3/2",
			cap(manager.global), cap(manager.mediaPollGate), cap(manager.mediaDownloadGate))
	}
	if _, err := NewAccountManagerWithMediaLimits(st, manager.bridge, 4, 0, 2); err == nil {
		t.Fatal("zero poll limit should be rejected")
	}
}

func TestReadyAccountsHonorsFreshnessAndCircuitState(t *testing.T) {
	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		worker := pool.NewWorker(id, nil)
		m.scheduleWorkers[id] = worker
		m.accounts[id].schedulerWorker = worker
		m.scheduler.Register(worker)
	}
	m.scheduleWorkers["account-b"].RestoreCooldown(time.Now().Add(time.Minute))
	if got := m.ReadyAccounts(); got != 1 {
		t.Fatalf("ready accounts with one open circuit=%d, want 1", got)
	}
	m.strictFresh = true
	if got := m.ReadyAccounts(); got != 0 {
		t.Fatalf("ready accounts with disconnected strict-fresh sessions=%d, want 0", got)
	}
	if !m.Initialized() {
		t.Fatal("scheduler test manager should represent a completed initial scan")
	}
}

func TestPinnedAccountWaitsForInitialRegistryScan(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "startup.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	manager, err := NewAccountManager(st, bridge.NewBridge(nil, nil, t.TempDir()), 2)
	if err != nil {
		t.Fatal(err)
	}
	// A different account has already registered while the pinned target is
	// still being bootstrapped. Registry size alone must not turn this into a
	// premature "unknown account" result.
	manager.scheduler.Register(pool.NewWorker("account-b", nil))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, acquireErr := manager.acquireScheduled(ctx, "account-a", 0)
		result <- acquireErr
	}()
	select {
	case err := <-result:
		t.Fatalf("pinned request returned before initial scan completed: %v", err)
	case <-time.After(25 * time.Millisecond):
	}

	manager.initOnce.Do(func() { close(manager.initialized) })
	select {
	case err := <-result:
		if err == nil || !strings.Contains(err.Error(), "account-a") {
			t.Fatalf("pinned account result = %v, want unavailable account-a after scan", err)
		}
	case <-ctx.Done():
		t.Fatal("pinned request did not resume after initial scan completed")
	}
}

func TestAccountManagerValidatesGenerationPayloadBeforeQueueing(t *testing.T) {
	m := newSchedulerManager(t, 1)
	invalid := []store.Job{
		{Kind: "image", Payload: `{"prompt":"  "}`},
		{Kind: "image", Payload: `{"prompt":"x","count":5}`},
		{Kind: "image", Payload: `{"prompt":"x","aspect":"panorama"}`},
		{Kind: "video", Payload: `{"prompt":"x","duration":"seven"}`},
		{Kind: "video", Payload: `{"prompt":"x","references":["asset"],"start_image":"frame"}`},
		{Kind: "upload-video", Payload: `{}`},
		{Kind: "upload-video", Payload: `{"file_path":"missing-upload.mp4"}`},
	}
	for _, job := range invalid {
		if err := m.ValidateJobPayload(context.Background(), job); err == nil {
			t.Errorf("invalid %s payload %s passed validation", job.Kind, job.Payload)
		}
	}
	directoryPayload, err := json.Marshal(map[string]string{"file_path": t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateJobPayload(context.Background(), store.Job{Kind: "upload-video", Payload: string(directoryPayload)}); err == nil {
		t.Fatal("upload-video accepted a directory as its file")
	}
	uploadPath := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(uploadPath, []byte("video"), 0600); err != nil {
		t.Fatal(err)
	}
	uploadPayload, err := json.Marshal(map[string]string{"file_path": uploadPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []store.Job{
		{Kind: "image", Payload: `{"prompt":"x","count":4,"aspect":"1:1"}`},
		{Kind: "video", Payload: `{"prompt":"x","duration":"8s","aspect":"9:16"}`},
		{Kind: "video", Payload: `{"prompt":"x","duration":"6","references":["asset"],"account_id":"account-a"}`},
		{Kind: "upload-video", Payload: string(uploadPayload)},
	} {
		if err := m.ValidateJobPayload(context.Background(), job); err != nil {
			t.Errorf("valid %s payload %s rejected: %v", job.Kind, job.Payload, err)
		}
	}
}

func TestInvalidGenerationPayloadIsRejectedBeforeDurableAcceptance(t *testing.T) {
	m := newSchedulerManager(t, 1)
	jobs, err := NewJobs(m.store, m, "validation-token", 1)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(`{"kind":"image","payload":{"prompt":" "}}`))
	req.Header.Set("Authorization", "Bearer validation-token")
	res := httptest.NewRecorder()
	jobs.ServeHTTP(res, req)
	if res.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid request status=%d body=%s, want 422", res.Code, res.Body.String())
	}
	stats, err := m.store.JobQueueStats(context.Background())
	if err != nil || stats.Outstanding != 0 {
		t.Fatalf("invalid request consumed queue capacity: stats=%+v err=%v", stats, err)
	}
}

func TestAccountSchedulerSerializesPerAccountButKeepsAccountsIndependent(t *testing.T) {
	m := newSchedulerManager(t, 2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first, err := m.acquire(ctx, "account-a")
	if err != nil {
		t.Fatal(err)
	}
	secondAcquire := make(chan *accountRuntime, 1)
	secondErr := make(chan error, 1)
	go func() { runtime, err := m.acquire(ctx, "account-a"); secondAcquire <- runtime; secondErr <- err }()
	select {
	case <-secondAcquire:
		t.Fatal("a second request acquired a busy account runtime")
	case <-time.After(30 * time.Millisecond):
	}
	other, err := m.acquire(ctx, "account-b")
	if err != nil || other.id != "account-b" {
		t.Fatalf("other account acquire=(%v,%v)", other, err)
	}
	m.release(other)
	m.release(first)
	select {
	case runtime := <-secondAcquire:
		if err := <-secondErr; err != nil || runtime.id != "account-a" {
			t.Fatalf("waiting account acquire=(%v,%v)", runtime, err)
		}
		m.release(runtime)
	case <-ctx.Done():
		t.Fatal("waiting request did not acquire the released account")
	}
}

func TestSharedAccountSchedulerRunsAcrossAccountsAndHonorsPins(t *testing.T) {
	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		worker.SetCredits(100, "G1_TIER1")
		m.scheduleWorkers[id] = worker
		m.scheduler.Register(worker)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	first, err := m.acquireScheduled(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.acquireScheduled(ctx, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatalf("concurrent jobs selected account %q twice despite per-account concurrency 1", first.ID)
	}
	m.scheduler.Release(second, 0, nil)

	pinnedResult := make(chan *pool.Worker, 1)
	pinnedError := make(chan error, 1)
	go func() {
		worker, err := m.acquireScheduled(ctx, first.ID, 10)
		pinnedResult <- worker
		pinnedError <- err
	}()
	select {
	case worker := <-pinnedResult:
		t.Fatalf("pinned account %q was handed out while busy (got %v)", first.ID, worker)
	case <-time.After(40 * time.Millisecond):
	}
	// The unpinned second account is free, but a pinned waiter must not use it.
	if got, err := m.acquireScheduled(ctx, second.ID, 10); err != nil || got.ID != second.ID {
		t.Fatalf("pinned free account result=(%v,%v), want %s", got, err, second.ID)
	} else {
		m.scheduler.Release(got, 0, nil)
	}
	m.scheduler.Release(first, 0, nil)
	select {
	case got := <-pinnedResult:
		if err := <-pinnedError; err != nil || got.ID != first.ID {
			t.Fatalf("waiting pin result=(%v,%v), want %s", got, err, first.ID)
		}
		m.scheduler.Release(got, 0, nil)
	case <-ctx.Done():
		t.Fatal("pinned waiter did not acquire its account after release")
	}
}

func TestAllAccountsExpansionSnapshotsTheReadyAccountSet(t *testing.T) {
	m := newSchedulerManager(t, 2)
	job := store.Job{ID: "fanout", Kind: "video", Payload: `{"prompt":"x","all_accounts":true}`}
	children, expanded, err := m.ExpandForQueue(context.Background(), job)
	if err != nil || !expanded || len(children) != 2 {
		t.Fatalf("fanout expansion=(%d,%v,%v), want two pinned account children", len(children), expanded, err)
	}
	for _, child := range children {
		var payload generationPayload
		if err := json.Unmarshal([]byte(child.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.AccountID != "account-a" && payload.AccountID != "account-b" {
			t.Fatalf("child account %q was not in the acceptance-time snapshot", payload.AccountID)
		}
		if payload.AllAccounts {
			t.Fatalf("child %s retained all_accounts and would fan out twice", child.ID)
		}
	}
	m.accounts["account-c"] = &accountRuntime{id: "account-c", gate: make(chan struct{}, 1)}
	if len(children) != 2 {
		t.Fatal("adding an account after acceptance changed the durable child snapshot")
	}
}

func TestStrictFreshSchedulerExcludesDisconnectedAccount(t *testing.T) {
	m := newSchedulerManager(t, 1)
	m.strictFresh = true
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := m.acquire(ctx, "account-a"); err == nil {
		t.Fatal("offline account was schedulable in strict freshness mode")
	}
}

func TestCookieRotationInvalidatesOldRuntimeBeforeReload(t *testing.T) {
	m := newSchedulerManager(t, 1)
	worker := pool.NewWorker("account-a", nil)
	worker.Concurrency = 1
	m.scheduleWorkers["account-a"] = worker
	m.scheduler.Register(worker)
	if m.account("account-a") == nil {
		t.Fatal("test account runtime was not registered")
	}

	m.Notify(bridge.CookieUpdate{AccountID: "account-a", Path: filepath.Join(t.TempDir(), "account_account-a.json")})
	if m.account("account-a") != nil {
		t.Fatal("old runtime remained available after cookie rotation notification")
	}
	if m.scheduler.Size() != 0 {
		t.Fatalf("old scheduler worker remained registered after rotation: %d", m.scheduler.Size())
	}
	select {
	case update := <-m.updates:
		if update.AccountID != "account-a" {
			t.Fatalf("queued update account=%q, want account-a", update.AccountID)
		}
	default:
		t.Fatal("cookie rotation update was not queued for runtime reload")
	}
}

func TestCookieRotationKeepsPerAccountGateAcrossRuntimeReplacement(t *testing.T) {
	m := newSchedulerManager(t, 1)
	oldRuntime := m.account("account-a")
	if _, err := m.acquire(context.Background(), "account-a"); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "account_account-a.json")
	bundle := &cookiejar.Bundle{
		AccountID: "account-a", ProjectID: "project-new", At: "at-new", Fsid: "fsid-new",
		Fingerprint: &cookiejar.Fingerprint{UserAgent: "agent-new"},
		Cookies:     []cookiejar.Cookie{{Domain: ".google.com", Name: "SID", Path: "/", Value: "rotated-cookie"}},
	}
	if err := bundle.Save(path); err != nil {
		t.Fatal(err)
	}
	m.Notify(bridge.CookieUpdate{AccountID: "account-a", Path: path, Bundle: bundle})
	m.runtimeFactory = func(_ context.Context, id, path string, _ *cookiejar.Bundle, _ bool) (*accountRuntime, *pool.Worker, error) {
		runtime := &accountRuntime{id: id, path: path, gate: make(chan struct{}, 1)}
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		return runtime, worker, nil
	}
	if err := m.reload(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	newRuntime := m.account("account-a")
	if newRuntime == nil || newRuntime == oldRuntime {
		t.Fatalf("runtime after rotation=%p, want a replacement for %p", newRuntime, oldRuntime)
	}
	if newRuntime.gate != oldRuntime.gate {
		t.Fatal("rotation replaced the account gate while an old runtime request could still be active")
	}

	acquired := make(chan *accountRuntime, 1)
	acquireErr := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		runtime, err := m.acquire(ctx, "account-a")
		if err != nil {
			acquireErr <- err
			return
		}
		acquired <- runtime
	}()
	select {
	case runtime := <-acquired:
		m.release(runtime)
		t.Fatal("new runtime ran concurrently with a request holding the previous snapshot")
	case err := <-acquireErr:
		t.Fatalf("new runtime acquire failed before old request released its gate: %v", err)
	case <-time.After(40 * time.Millisecond):
	}
	m.release(oldRuntime)
	select {
	case runtime := <-acquired:
		if runtime != newRuntime {
			t.Fatalf("acquired runtime=%p, want published replacement %p", runtime, newRuntime)
		}
		m.release(runtime)
	case err := <-acquireErr:
		t.Fatalf("acquire after old request release failed: %v", err)
	case <-ctx.Done():
		t.Fatal("replacement runtime did not acquire the account gate after old request completed")
	}
}

func TestPersistedBridgeRotationCallbackInvalidatesOnlyMatchingAccount(t *testing.T) {
	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		m.scheduleWorkers[id] = worker
		m.accounts[id].schedulerWorker = worker
		m.scheduler.Register(worker)
	}
	path := filepath.Join(t.TempDir(), "account_account-a.json")
	bundle := &cookiejar.Bundle{
		AccountID: "account-a", ProjectID: "project-a", At: "at-new", Fsid: "fsid-new",
		Fingerprint: &cookiejar.Fingerprint{UserAgent: "rotated-agent"},
		Cookies:     []cookiejar.Cookie{{Domain: ".google.com", Name: "SID", Path: "/", Value: "rotated-cookie"}},
	}
	if err := bundle.Save(path); err != nil {
		t.Fatal(err)
	}
	m.bridge.OnCookieSync = m.Notify
	m.bridge.OnCookieSync(bridge.CookieUpdate{AccountID: bundle.AccountID, Path: path, Bundle: bundle})

	if m.account("account-a") != nil || m.account("account-b") == nil {
		t.Fatal("rotation did not invalidate only account-a's old runtime")
	}
	if m.scheduler.Size() != 1 {
		t.Fatalf("schedulable workers after rotation=%d, want healthy account-b only", m.scheduler.Size())
	}
	select {
	case update := <-m.updates:
		if update.AccountID != "account-a" || update.Path != path {
			t.Fatalf("queued rotation update=(%q,%q), want account-a and persisted bundle path", update.AccountID, update.Path)
		}
	default:
		t.Fatal("bridge rotation callback did not queue runtime reload")
	}
	persisted, err := cookiejar.LoadBundleFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.AccountID != "account-a" || persisted.ProjectID != "project-a" || persisted.At != "at-new" || persisted.Fsid != "fsid-new" || persisted.Fingerprint.UserAgent != "rotated-agent" || len(persisted.Cookies) != 1 || persisted.Cookies[0].Value != "rotated-cookie" {
		t.Fatalf("persisted rotation snapshot=%+v, account/session metadata must remain paired", persisted)
	}
}

func TestAccountManagerReloadPublishesRotatedRuntimeFromLatestBundle(t *testing.T) {
	cookieDir := t.TempDir()
	t.Setenv("FLOW_COOKIE_DIR", cookieDir)
	path := filepath.Join(cookieDir, "account_account-a.json")
	makeBundle := func(value, project, at string) *cookiejar.Bundle {
		return &cookiejar.Bundle{
			AccountID: "account-a", ProjectID: project, At: at, Fsid: "fsid-" + at,
			Fingerprint: &cookiejar.Fingerprint{UserAgent: "agent-" + at},
			Cookies: []cookiejar.Cookie{
				{Domain: ".google.com", Name: "SAPISID", Path: "/", Value: "stable-sapisid"},
				{Domain: ".google.com", Name: "SID", Path: "/", Value: value},
			},
		}
	}
	initialBundle := makeBundle("cookie-old", "project-old", "at-old")
	if err := initialBundle.Save(path); err != nil {
		t.Fatal(err)
	}
	m := newSchedulerManager(t, 2)
	oldPublished := make(chan *accountRuntime, 1)
	midBuilderEntered := make(chan struct{})
	allowMidBuilder := make(chan struct{})
	var allowMidBuilderOnce sync.Once
	releaseMidBuilder := func() { allowMidBuilderOnce.Do(func() { close(allowMidBuilder) }) }
	seenBundles := make(chan *cookiejar.Bundle, 2)
	m.runtimeFactory = func(_ context.Context, id, path string, bundle *cookiejar.Bundle, _ bool) (*accountRuntime, *pool.Worker, error) {
		if id != "account-a" || bundle.AccountID != id {
			return nil, nil, fmt.Errorf("account identity mismatch: id=%q bundle=%q", id, bundle.AccountID)
		}
		seenBundles <- bundle
		eng, err := engine.New(m.store, m.bridge.AccountView(id), engine.Options{
			AccountID: id, CookieFile: path, RefreshCreditsOnBoot: false,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := eng.Bootstrap(context.Background()); err != nil {
			return nil, nil, fmt.Errorf("bootstrap account %s from rotated bundle: %w", id, err)
		}
		if !eng.Ready() || eng.AccountID() != id || eng.ProjectID() != bundle.ProjectID || eng.Jar() == nil || !strings.Contains(eng.Jar().Raw(), bundle.Cookies[1].Value) {
			return nil, nil, fmt.Errorf("bootstrapped engine did not adopt complete account snapshot %q", bundle.At)
		}
		snapshot, err := eng.SessionSnapshot(context.Background())
		if err != nil || snapshot.AccountID != id || snapshot.ProjectID != bundle.ProjectID || snapshot.At != bundle.At || snapshot.Fsid != bundle.Fsid || snapshot.Fingerprint == nil || snapshot.Fingerprint.UserAgent != bundle.Fingerprint.UserAgent || len(snapshot.Cookies) != 2 || snapshot.Cookies[1].Value != bundle.Cookies[1].Value {
			return nil, nil, fmt.Errorf("session snapshot does not match account bundle %q: snapshot=%+v err=%v", bundle.At, snapshot, err)
		}
		if bundle.At == "at-mid" {
			close(midBuilderEntered)
			<-allowMidBuilder
		}
		encoded, _ := json.Marshal(bundle)
		runtime := &accountRuntime{
			id: id, path: path, engine: eng, gate: make(chan struct{}, 1),
			snapshot: fmt.Sprintf("%x", sha256.Sum256(encoded)),
		}
		worker := m.prepareSchedulerWorker(id, eng, false)
		runtime.schedulerWorker = worker
		if bundle.At == "at-old" {
			oldPublished <- runtime
		}
		return runtime, worker, nil
	}
	m.bridge.OnCookieSync = m.Notify
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { m.Start(ctx); close(done) }()
	t.Cleanup(func() {
		releaseMidBuilder()
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("account manager did not stop")
		}
	})

	var oldRuntime *accountRuntime
	select {
	case oldRuntime = <-oldPublished:
	case <-time.After(3 * time.Second):
		t.Fatal("initial persisted bundle was not loaded")
	}
	select {
	case bundle := <-seenBundles:
		if bundle.Cookies[1].Value != "cookie-old" || bundle.ProjectID != "project-old" || bundle.At != "at-old" {
			t.Fatalf("initial runtime got mixed snapshot: %+v", bundle)
		}
	case <-time.After(time.Second):
		t.Fatal("initial snapshot was not passed to runtime factory")
	}
	select {
	case <-m.initialized:
	case <-time.After(3 * time.Second):
		t.Fatal("initial registry scan did not finish")
	}
	if got := m.account("account-a"); got != oldRuntime {
		t.Fatalf("initial published runtime=%p, want %p", got, oldRuntime)
	}

	intermediateBundle := makeBundle("cookie-mid", "project-mid", "at-mid")
	if err := intermediateBundle.Save(path); err != nil {
		t.Fatal(err)
	}
	m.bridge.OnCookieSync(bridge.CookieUpdate{AccountID: "account-a", Path: path, Bundle: intermediateBundle})
	select {
	case <-midBuilderEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("first cookie rotation did not trigger a runtime rebuild")
	}
	rotatedBundle := makeBundle("cookie-new", "project-new", "at-new")
	if err := rotatedBundle.Save(path); err != nil {
		t.Fatal(err)
	}
	m.bridge.OnCookieSync(bridge.CookieUpdate{AccountID: "account-a", Path: path, Bundle: rotatedBundle})
	if m.account("account-a") != nil || m.scheduler.Size() != 0 {
		t.Fatal("old runtime remained schedulable while newer snapshots were rebuilding")
	}
	releaseMidBuilder()

	deadline := time.Now().Add(3 * time.Second)
	var newRuntime *accountRuntime
	for time.Now().Before(deadline) {
		newRuntime = m.account("account-a")
		if newRuntime != nil && newRuntime.snapshot != oldRuntime.snapshot {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if newRuntime == nil {
		t.Fatal("rotated runtime was not published")
	}
	if newRuntime == oldRuntime || newRuntime.snapshot == oldRuntime.snapshot || m.scheduler.Size() != 1 {
		t.Fatalf("rotated runtime=(%p snapshot=%q scheduler=%d), old=(%p snapshot=%q)", newRuntime, newRuntime.snapshot, m.scheduler.Size(), oldRuntime, oldRuntime.snapshot)
	}
	fingerprint := newRuntime.engine.Fingerprint()
	jar := newRuntime.engine.Jar()
	if newRuntime.path != path || newRuntime.schedulerWorker.ProjectID() != "project-new" || newRuntime.engine.ProjectID() != "project-new" || newRuntime.engine.AccountID() != "account-a" || fingerprint == nil || fingerprint.UserAgent != "agent-at-new" || jar == nil || !strings.Contains(jar.Raw(), "cookie-new") {
		t.Fatalf("rotated runtime did not adopt latest cookie/project/fingerprint: path=%q scheduler-project=%q engine-account=%q engine-project=%q fingerprint=%+v jar=%v", newRuntime.path, newRuntime.schedulerWorker.ProjectID(), newRuntime.engine.AccountID(), newRuntime.engine.ProjectID(), fingerprint, jar)
	}
	latestSeen := false
	for deadline := time.After(time.Second); !latestSeen; {
		select {
		case bundle := <-seenBundles:
			if bundle.At == "at-new" {
				if bundle.Cookies[1].Value != "cookie-new" || bundle.ProjectID != "project-new" || bundle.Fingerprint.UserAgent != "agent-at-new" {
					t.Fatalf("rebuilt runtime received mixed rotated snapshot: %+v", bundle)
				}
				latestSeen = true
			}
		case <-deadline:
			t.Fatal("latest rotated bundle was not passed to runtime factory")
		}
	}
}

func TestAccountManagerRejectsBundleIdentityThatDiffersFromFile(t *testing.T) {
	m := newSchedulerManager(t, 1)
	path := filepath.Join(t.TempDir(), "account_account-a.json")
	bundle := &cookiejar.Bundle{
		AccountID: "account-b", ProjectID: "project-b", At: "at-b", Fsid: "fsid-b",
		Fingerprint: &cookiejar.Fingerprint{UserAgent: "agent-b"},
		Cookies:     []cookiejar.Cookie{{Domain: ".google.com", Name: "SID", Path: "/", Value: "cookie-b"}},
	}
	if err := bundle.Save(path); err != nil {
		t.Fatal(err)
	}
	if err := m.reload(context.Background(), path); err == nil || !strings.Contains(err.Error(), "does not match file identity") {
		t.Fatalf("reload identity mismatch error=%v, want explicit account binding rejection", err)
	}
	if m.account("account-a") != nil || m.scheduler.Size() != 0 {
		t.Fatal("mismatched bundle was published as account-a")
	}
}

func TestAccountManagerCookieRemovalKeepsAccountOutOfScheduler(t *testing.T) {
	m := newSchedulerManager(t, 1)
	worker := pool.NewWorker("account-a", nil)
	worker.Concurrency = 1
	m.scheduleWorkers[worker.ID] = worker
	m.scheduler.Register(worker)
	path := filepath.Join(t.TempDir(), "account_account-a.json")
	removed := &cookiejar.Bundle{
		AccountID: "account-a", ProjectID: "project-a", At: "at-a", Fsid: "fsid-a",
		Fingerprint: &cookiejar.Fingerprint{UserAgent: "agent-a"},
		Cookies:     []cookiejar.Cookie{{Domain: ".google.com", Name: "PREF", Path: "/", Value: "non-auth-cookie"}},
	}
	if err := removed.Save(path); err != nil {
		t.Fatal(err)
	}
	m.bridge.OnCookieSync = m.Notify
	m.bridge.OnCookieSync(bridge.CookieUpdate{AccountID: "account-a", Path: path, Bundle: removed})
	if err := m.reload(context.Background(), path); err == nil || !strings.Contains(err.Error(), "incomplete session bundle") {
		t.Fatalf("reload after auth-cookie removal error=%v, want incomplete-session rejection", err)
	}
	if m.account("account-a") != nil || m.scheduler.Size() != 0 {
		t.Fatal("account with removed auth cookies remained schedulable")
	}
}

func TestNeedsLoginRemovesOnlyTheAffectedAccountFromScheduling(t *testing.T) {
	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		if err := m.store.UpsertAccount(store.Account{AccountID: id, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		worker := pool.NewWorker(id, nil)
		m.scheduleWorkers[id] = worker
		m.scheduler.Register(worker)
	}
	runtimeA := m.account("account-a")
	m.markNeedsLogin("account-a", "session refresh failed", runtimeA)

	if m.scheduler.Size() != 1 {
		t.Fatalf("registered scheduler accounts=%d, want only the healthy neighbor", m.scheduler.Size())
	}
	if m.account("account-a") != nil {
		t.Fatal("account requiring login still has an active runtime")
	}
	status, err := m.store.AccountStatus("account-a")
	if err != nil || status != "needs_login" {
		t.Fatalf("account status=(%q,%v), want needs_login", status, err)
	}
	if _, err := m.acquireScheduled(context.Background(), "account-a", 0); err == nil {
		t.Fatal("account requiring login remained schedulable")
	}
	healthyWorker, err := m.acquireScheduled(context.Background(), "account-b", 0)
	if err != nil {
		t.Fatalf("healthy neighboring account became unavailable: %v", err)
	}
	m.scheduler.Release(healthyWorker, 0, nil)
}

func TestUnauthorizedSessionFailureMarksOnlyItsAccountNeedsLogin(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()
	previousOrigin := batchexecute.Origin
	batchexecute.Origin = upstream.URL
	defer func() { batchexecute.Origin = previousOrigin }()

	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		if err := m.store.UpsertAccount(store.Account{AccountID: id, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		m.scheduleWorkers[id] = worker
		m.scheduler.Register(worker)
	}

	eng, err := engine.New(m.store, m.bridge.AccountView("account-a"), engine.Options{AccountID: "account-a"})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar := cookiejar.FromRaw("SAPISID=sapisid-a; SID=session-a", parsed.Hostname(), "account-a")
	httpClient, err := httpx.New(httpx.WithQUIC(false))
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedClient := eng.NewBatchexecuteClient(jar, httpClient)
	_, refreshErr := unauthorizedClient.Generate(context.Background(), batchexecute.GenerateRequest{
		ProjectID: "project-a", Model: "NARWHAL", Prompt: "first request", CaptchaToken: "captcha-a",
	}, batchexecute.CallOptions{})
	if refreshErr == nil || eng.SessionFailure() == "" {
		t.Fatalf("401 recovery=(err=%v, session failure=%q), want failed refresh latched", refreshErr, eng.SessionFailure())
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream request count=%d, want one attempt and no retry after refresh failure", got)
	}
	m.accounts["account-a"] = &accountRuntime{id: "account-a", gate: make(chan struct{}, 1), engine: eng}

	_, jobErr := m.runOne(context.Background(), "job-auth-failure", "image", generationPayload{
		Prompt: "must not be submitted", AccountID: "account-a", NoDownload: true,
	}, "account-a", nil)
	if jobErr == nil {
		t.Fatal("job unexpectedly succeeded after the engine latched an expired session")
	}
	if m.account("account-a") != nil || m.scheduler.Size() != 1 {
		t.Fatalf("bad account/runtime still scheduled: runtime=%v scheduler size=%d", m.account("account-a"), m.scheduler.Size())
	}
	status, err := m.store.AccountStatus("account-a")
	if err != nil || status != "needs_login" {
		t.Fatalf("account-a status=(%q,%v), want needs_login", status, err)
	}
	if _, err := m.acquireScheduled(context.Background(), "account-a", 0); err == nil {
		t.Fatal("account requiring login remained schedulable")
	}
	healthy, err := m.acquireScheduled(context.Background(), "account-b", 0)
	if err != nil {
		t.Fatalf("healthy neighboring account became unavailable: %v", err)
	}
	m.scheduler.Release(healthy, 0, nil)
}

func TestUnauthorizedBurstAfterRefreshFailureDoesNotCreateRetryStorm(t *testing.T) {
	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalls.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()
	previousOrigin := batchexecute.Origin
	batchexecute.Origin = upstream.URL
	defer func() { batchexecute.Origin = previousOrigin }()

	m := newSchedulerManager(t, 2)
	for _, id := range []string{"account-a", "account-b"} {
		if err := m.store.UpsertAccount(store.Account{AccountID: id, Status: "active"}); err != nil {
			t.Fatal(err)
		}
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		m.scheduleWorkers[id] = worker
		m.scheduler.Register(worker)
	}
	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	jar := cookiejar.FromRaw("SAPISID=sapisid-a; SID=session-a", parsed.Hostname(), "account-a")
	eng, err := engine.New(m.store, m.bridge.AccountView("account-a"), engine.Options{AccountID: "account-a", Jar: jar})
	if err != nil {
		t.Fatal(err)
	}
	m.accounts["account-a"] = &accountRuntime{id: "account-a", gate: make(chan struct{}, 1), engine: eng}
	httpClient, err := httpx.New(httpx.WithQUIC(false))
	if err != nil {
		t.Fatal(err)
	}
	unauthorizedClient := eng.NewBatchexecuteClient(jar, httpClient)
	_, refreshErr := unauthorizedClient.Generate(context.Background(), batchexecute.GenerateRequest{
		ProjectID: "project-a", Model: "NARWHAL", Prompt: "trigger stale session", CaptchaToken: "captcha-a",
	}, batchexecute.CallOptions{})
	if refreshErr == nil || eng.SessionFailure() == "" {
		t.Fatalf("initial 401 recovery=(err=%v, session failure=%q), want extension refresh failure", refreshErr, eng.SessionFailure())
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream request count=%d, want one rejected attempt and no retry", got)
	}
	if _, err := m.runOne(context.Background(), "auth-burst-trigger", "image", generationPayload{
		Prompt: "latched failure", AccountID: "account-a", NoDownload: true,
	}, "account-a", nil); err == nil {
		t.Fatal("stale account runtime unexpectedly accepted a generation after failed refresh")
	}
	if m.account("account-a") != nil {
		t.Fatal("account with failed session refresh remained published")
	}

	runner := &authBurstServiceRunner{manager: m}
	jobs, err := NewJobs(m.store, runner, "burst-token", 12)
	if err != nil {
		t.Fatal(err)
	}
	const burst = 24
	jobIDs := make([]string, 0, burst)
	for i := 0; i < burst; i++ {
		id := fmt.Sprintf("auth-burst-%02d", i)
		payload, marshalErr := json.Marshal(generationPayload{Prompt: "burst", AccountID: "account-a", NoDownload: true})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		if _, _, err := m.store.CreateJob(context.Background(), store.Job{ID: id, Kind: "image", Payload: string(payload)}); err != nil {
			t.Fatal(err)
		}
		jobIDs = append(jobIDs, id)
	}
	accountBJobID := "auth-burst-account-b"
	accountBPayload, err := json.Marshal(generationPayload{Prompt: "healthy account remains available", AccountID: "account-b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := m.store.CreateJob(context.Background(), store.Job{ID: accountBJobID, Kind: "image", Payload: string(accountBPayload)}); err != nil {
		t.Fatal(err)
	}
	jobIDs = append(jobIDs, accountBJobID)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { jobs.Start(ctx); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("auth burst service workers did not stop")
		}
	}()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		terminal := 0
		for _, id := range jobIDs {
			job, err := m.store.JobByID(context.Background(), id)
			if err == nil && (job.Status == "succeeded" || job.Status == "failed" || job.Status == "submission_unknown") {
				terminal++
			}
		}
		if terminal == burst+1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, id := range jobIDs {
		job, err := m.store.JobByID(context.Background(), id)
		if err != nil || job.Status == "queued" || job.Status == "running" {
			t.Fatalf("job %s did not settle after auth burst: (%+v,%v)", id, job, err)
		}
		if id == accountBJobID && job.Status != "succeeded" {
			t.Fatalf("healthy account-b job ended %s: %s", job.Status, job.Error)
		}
		if id != accountBJobID && job.Status == "succeeded" {
			t.Fatalf("account-a job %s unexpectedly succeeded after refresh failure", id)
		}
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream submissions after initial 401 and %d queued jobs=%d, want exactly the initial rejected request", burst, got)
	}
	if got := runner.accountBJobs.Load(); got != 1 {
		t.Fatalf("healthy account-b jobs run=%d, want one successful job during the auth burst", got)
	}
	if m.account("account-a") != nil || m.scheduler.Size() != 1 {
		t.Fatalf("bad account isolation after 401 burst: runtimeA=%v scheduler=%d", m.account("account-a"), m.scheduler.Size())
	}
	status, err := m.store.AccountStatus("account-a")
	if err != nil || status != "needs_login" {
		t.Fatalf("account-a status=(%q,%v), want needs_login", status, err)
	}
	neighbor, err := m.acquireScheduled(context.Background(), "account-b", 0)
	if err != nil {
		t.Fatalf("healthy account-b stopped serving during account-a auth failure: %v", err)
	}
	m.scheduler.Release(neighbor, 0, nil)
}

func TestNeedsLoginFromSupersededRuntimeDoesNotDisableNewSnapshot(t *testing.T) {
	m := newSchedulerManager(t, 1)
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-a", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	worker := pool.NewWorker("account-a", nil)
	m.scheduleWorkers[worker.ID] = worker
	m.scheduler.Register(worker)
	oldRuntime := &accountRuntime{id: worker.ID, gate: make(chan struct{}, 1)}
	m.accounts[worker.ID] = &accountRuntime{id: worker.ID, gate: make(chan struct{}, 1)}
	m.markNeedsLogin(worker.ID, "old session expired", oldRuntime)

	if m.scheduler.Size() != 1 || m.account(worker.ID) == nil {
		t.Fatal("stale runtime failure disabled the replacement snapshot")
	}
	status, err := m.store.AccountStatus(worker.ID)
	if err != nil || status != "active" {
		t.Fatalf("replacement account status=(%q,%v), want active", status, err)
	}
}

func TestCookieRotationNotifyDoesNotWaitForLongAccountReload(t *testing.T) {
	m := newSchedulerManager(t, 1)
	lock := m.reloadLock("account-a")
	lock.Lock()
	done := make(chan struct{})
	go func() {
		m.Notify(bridge.CookieUpdate{AccountID: "account-a", Path: filepath.Join(t.TempDir(), "account_account-a.json")})
		close(done)
	}()
	select {
	case <-done:
		lock.Unlock()
	case <-time.After(100 * time.Millisecond):
		lock.Unlock()
		<-done
		t.Fatal("cookie event callback waited on the account bootstrap lock")
	}
}

func TestOlderAccountBootstrapCannotPublishAfterCookieRotation(t *testing.T) {
	m := newSchedulerManager(t, 1)
	m.Notify(bridge.CookieUpdate{AccountID: "account-a", Path: filepath.Join(t.TempDir(), "account_account-a.json")})
	staleRuntime := &accountRuntime{id: "account-a", gate: make(chan struct{}, 1), snapshot: "old-revision"}
	staleWorker := pool.NewWorker("account-a", nil)
	if m.publishRuntimeIfCurrent("account-a", 0, staleRuntime, staleWorker) {
		t.Fatal("older cookie bootstrap published after a newer update revision")
	}
	if m.account("account-a") != nil || m.scheduler.Size() != 0 {
		t.Fatal("stale runtime or worker became schedulable after rotation")
	}
}

func TestGenerationCreditCostUsesVideoEstimate(t *testing.T) {
	cost, known := engine.EstimateVideoCredits(8, 1, "standard")
	if !known || cost <= 0 {
		t.Fatalf("video estimate = (%d, %v), want a known positive cost", cost, known)
	}
	if got := generationCreditCost("video", generationPayload{Duration: "8s", Count: 1, Quality: "standard"}); got != cost {
		t.Fatalf("scheduled video cost = %d, want engine cost %d", got, cost)
	}
	if got := generationCreditCost("image", generationPayload{}); got != 0 {
		t.Fatalf("image cost = %d, want unknown cost represented by zero", got)
	}
	if got := generationCreditCost("video", generationPayload{References: []string{"asset-1"}}); got != config.CreditsPerReferenceVideo {
		t.Fatalf("reference video cost = %d, want %d", got, config.CreditsPerReferenceVideo)
	}
}

func TestAccountManagerRestoresPersistedCooldown(t *testing.T) {
	m := newSchedulerManager(t, 1)
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-a"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().UTC().Add(time.Minute).Round(0)
	if err := m.store.SetAccountCooldown(context.Background(), "account-a", deadline, "throttled"); err != nil {
		t.Fatal(err)
	}
	worker := pool.NewWorker("account-a", nil)
	m.restoreAccountCooldown(worker, "account-a")
	if worker.State() != pool.StateCircuitOpen || !worker.CooldownUntil().Equal(deadline) {
		t.Fatalf("restored scheduler state=(%s,%v), want cooldown until %v", worker.State(), worker.CooldownUntil(), deadline)
	}
}

func TestCooldownOnOneAccountDoesNotBlockHealthyAccountOrChangePinnedSelection(t *testing.T) {
	m := newSchedulerManager(t, 2)
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-a", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-b", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute).UTC().Round(0)
	if err := m.store.SetAccountCooldown(context.Background(), "account-a", deadline, "throttled"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"account-a", "account-b"} {
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		m.scheduleWorkers[id] = worker
		m.scheduler.Register(worker)
		if id == "account-a" {
			m.restoreAccountCooldown(worker, id)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	selected, err := m.acquireScheduled(ctx, "", 0)
	if err != nil {
		t.Fatalf("automatic selection failed while account-a was cooling down: %v", err)
	}
	if selected.ID != "account-b" {
		t.Fatalf("automatic selection returned %s, want healthy account-b", selected.ID)
	}
	m.scheduler.Release(selected, 0, nil)

	if _, err := m.acquireScheduled(ctx, "account-a", 0); err == nil {
		t.Fatal("pinned request silently used a different account or ignored account-a cooldown")
	} else {
		var unavailable *pool.NoWorkerError
		if !errors.As(err, &unavailable) || unavailable.RetryAfter.IsZero() {
			t.Fatalf("pinned cooldown error=%T %v, want a retry deadline", err, err)
		}
	}
}

func TestStartupBalanceClearsPersistedOutOfCreditsCooldown(t *testing.T) {
	m := newSchedulerManager(t, 1)
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-a"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute)
	if err := m.store.SetAccountCooldown(context.Background(), "account-a", deadline, "insufficient_credits"); err != nil {
		t.Fatal(err)
	}
	worker := pool.NewWorker("account-a", nil)
	worker.SetCredits(20, "G1_TIER1")
	m.restoreAccountCooldown(worker, "account-a")
	if worker.State() != pool.StateIdle {
		t.Fatalf("account with refreshed positive balance stayed parked: %s", worker.State())
	}
	if _, _, exists, err := m.store.AccountCooldown(context.Background(), "account-a"); err != nil || exists {
		t.Fatalf("persisted out-of-credit cooldown remains (exists=%v,err=%v)", exists, err)
	}
}

func TestBalanceExcludesPersistedAccountsWithoutCurrentRuntimes(t *testing.T) {
	m := newSchedulerManager(t, 1)
	credits := 23
	for _, account := range []store.Account{
		{AccountID: "account-a", Credits: &credits, IdentitySource: "override"},
		{AccountID: "account-b", IdentitySource: "override"},
		{AccountID: "legacy-core-id", Credits: &credits, IdentitySource: "cookie-core"},
	} {
		if err := m.store.UpsertAccount(account); err != nil {
			t.Fatalf("upsert account %s: %v", account.AccountID, err)
		}
	}

	result, err := m.Balance(context.Background(), false)
	if err != nil {
		t.Fatalf("read balance: %v", err)
	}
	accounts, ok := result["accounts"].([]store.Account)
	if !ok {
		t.Fatalf("accounts have type %T, want []store.Account", result["accounts"])
	}
	if len(accounts) != 2 {
		t.Fatalf("balance returned %d accounts, want the 2 registered runtimes", len(accounts))
	}
	if result["total_credits"] != 23 || result["unread"] != 1 {
		t.Fatalf("balance totals = %v, unread = %v; want 23 and 1", result["total_credits"], result["unread"])
	}
	for _, account := range accounts {
		if account.AccountID == "legacy-core-id" {
			t.Fatal("balance included a persisted identity without a current runtime")
		}
	}
}

func TestImageBalanceRefreshPublishesOnlyForCurrentRuntime(t *testing.T) {
	m := newSchedulerManager(t, 1)
	runtime := m.account("account-a")
	worker := pool.NewWorker("account-a", nil)
	m.mu.Lock()
	m.scheduleWorkers["account-a"] = worker
	m.mu.Unlock()
	m.scheduler.Register(worker)
	if !m.publishObservedBalance(runtime, 41) {
		t.Fatal("current runtime balance was not published")
	}
	if credits, known := worker.Credits(); !known || credits != 41 {
		t.Fatalf("scheduler balance=(%d,known=%v), want 41 known", credits, known)
	}
	accounts, err := m.store.ListAccounts()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, account := range accounts {
		if account.AccountID == "account-a" {
			found = true
			if account.Credits == nil || *account.Credits != 41 || account.CreditsCheckedAt == nil {
				t.Fatalf("persisted balance=%+v, want 41 with observation time", account)
			}
		}
	}
	if !found {
		t.Fatal("refreshed account balance was not persisted")
	}

	m.mu.Lock()
	m.accounts["account-a"] = &accountRuntime{id: "account-a", gate: make(chan struct{}, 1)}
	m.mu.Unlock()
	if m.publishObservedBalance(runtime, 99) {
		t.Fatal("stale runtime published a balance after replacement")
	}
	if credits, known := worker.Credits(); !known || credits != 41 {
		t.Fatalf("stale runtime changed scheduler balance=(%d,known=%v), want 41 known", credits, known)
	}
}

func TestWaitBackgroundDrainsRunningTasksAndRejectsNewOnes(t *testing.T) {
	m := &AccountManager{}
	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	if !m.startBackground(func() {
		close(started)
		<-release
	}) {
		t.Fatal("background task was rejected before shutdown")
	}
	<-started
	go func() {
		m.WaitBackground()
		close(finished)
	}()
	select {
	case <-finished:
		t.Fatal("shutdown drain returned while a background task was still running")
	case <-time.After(25 * time.Millisecond):
	}
	close(release)
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown drain did not finish after background task exited")
	}
	if m.startBackground(func() {}) {
		t.Fatal("background task was accepted after shutdown drain began")
	}
}

func TestPositiveBalanceRefreshUpdatesSchedulerAndClearsCreditCooldown(t *testing.T) {
	m := newSchedulerManager(t, 1)
	if err := m.store.UpsertAccount(store.Account{AccountID: "account-a"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Minute)
	if err := m.store.SetAccountCooldown(context.Background(), "account-a", deadline, "insufficient_credits"); err != nil {
		t.Fatal(err)
	}
	scheduled := pool.NewWorker("account-a", nil)
	scheduled.Concurrency = 1
	scheduled.SetCredits(0, "G1_TIER1")
	scheduled.RestoreCooldown(deadline)
	m.scheduleWorkers["account-a"] = scheduled
	eng, err := engine.New(m.store, m.bridge.AccountView("account-a"), engine.Options{AccountID: "account-a"})
	if err != nil {
		t.Fatal(err)
	}
	observed := pool.NewWorker("account-a", nil)
	observed.SetCredits(25, "G1_TIER1")
	eng.Pool().Register(observed)
	m.syncSchedulerBalance(&accountRuntime{id: "account-a", engine: eng})
	if credits, known := scheduled.Credits(); !known || credits != 25 {
		t.Fatalf("scheduler credits=(%d,known=%v), want 25 known", credits, known)
	}
	if scheduled.State() != pool.StateIdle {
		t.Fatalf("scheduler state=%s, want idle after replenishment", scheduled.State())
	}
	if _, _, exists, err := m.store.AccountCooldown(context.Background(), "account-a"); err != nil || exists {
		t.Fatalf("credit cooldown remains (exists=%v,err=%v)", exists, err)
	}
}
