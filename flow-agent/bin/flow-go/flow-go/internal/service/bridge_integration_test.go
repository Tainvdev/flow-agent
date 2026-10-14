package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

type accountGateJobRunner struct {
	manager *AccountManager
	entered chan string
	started chan *accountRuntime
	release chan struct{}
	calls   atomic.Int32
}

func (r *accountGateJobRunner) Run(ctx context.Context, job store.Job) (any, error) {
	r.entered <- job.ID
	runtime, err := r.manager.acquireScheduledRuntime(ctx, "")
	if err != nil {
		return nil, err
	}
	defer r.manager.release(runtime)
	r.started <- runtime
	if r.calls.Add(1) == 1 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-r.release:
		}
	}
	return map[string]any{"account_id": runtime.id, "snapshot": runtime.snapshot}, nil
}

func TestBridgeRotationReplacesServiceRuntimeWithoutOverlappingAccountJobs(t *testing.T) {
	cookieDir := t.TempDir()
	t.Setenv("FLOW_COOKIE_DIR", cookieDir)
	rotationValue := atomic.Value{}
	rotationValue.Store("rotating-old")
	var extensionWrites sync.Mutex

	initialJar := cookiejar.FromRaw("SAPISID=stable-account-cookie; SID=sid-stable; __Secure-1PSIDTS=rotating-old", ".google.com", "test")
	accountID := cookiejar.AccountKey(initialJar)
	if accountID == "" {
		t.Fatal("test cookie jar did not produce a stable account ID")
	}
	initial := &cookiejar.Bundle{
		AccountID: accountID, ProjectID: "project-old", At: "at-old", Fsid: "fsid-old",
		Fingerprint: &cookiejar.Fingerprint{UserAgent: "agent-old"}, Cookies: initialJar.Cookies(),
	}
	path := filepath.Join(cookieDir, "account_"+accountID+".json")
	if err := initial.Save(path); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(filepath.Join(t.TempDir(), "bridge-integration.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	br := bridge.NewBridge(nil, []string{".google.com"}, cookieDir)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	br.ListenAddr = listener.Addr().String()
	_ = listener.Close()
	manager, err := NewAccountManager(st, br, 2)
	if err != nil {
		t.Fatal(err)
	}
	manager.strictFresh = false
	manager.runtimeFactory = func(_ context.Context, id, path string, bundle *cookiejar.Bundle, _ bool) (*accountRuntime, *pool.Worker, error) {
		if id != accountID || bundle.AccountID != accountID {
			return nil, nil, fmt.Errorf("runtime identity mismatch: id=%q bundle=%q", id, bundle.AccountID)
		}
		encoded, _ := json.Marshal(bundle)
		runtime := &accountRuntime{
			id: id, path: path, gate: make(chan struct{}, 1),
			snapshot: fmt.Sprintf("%x", encoded),
		}
		worker := pool.NewWorker(id, nil)
		worker.Concurrency = 1
		worker.SetProjectID(bundle.ProjectID)
		runtime.schedulerWorker = worker
		return runtime, worker, nil
	}
	br.OnCookieSync = manager.Notify

	serviceCtx, stopService := context.WithCancel(context.Background())
	managerDone := make(chan struct{})
	go func() { manager.Start(serviceCtx); close(managerDone) }()
	bridgeDone := make(chan error, 1)
	go func() { bridgeDone <- br.Listen(serviceCtx) }()
	if err := waitForTCPListener(br.ListenAddr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopService()
		select {
		case <-managerDone:
		case <-time.After(2 * time.Second):
			t.Error("account manager did not stop")
		}
		select {
		case <-bridgeDone:
		case <-time.After(2 * time.Second):
			t.Error("bridge listener did not stop")
		}
	})
	select {
	case <-manager.initialized:
	case <-time.After(3 * time.Second):
		t.Fatal("initial persisted account scan did not finish")
	}
	oldRuntime := waitForRuntime(t, manager, accountID, nil)

	connect := func() *websocket.Conn {
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+br.ListenAddr, nil)
		if err != nil {
			t.Fatalf("connect fake extension: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		go func() {
			for {
				var request struct {
					ID string `json:"id"`
					Op string `json:"op"`
				}
				if err := conn.ReadJSON(&request); err != nil {
					return
				}
				var result any = map[string]any{}
				switch request.Op {
				case "ping":
					result = map[string]any{"version": "integration-test"}
				case "config.set":
					result = map[string]any{}
				case "tabs.list":
					result = []any{}
				case "cookies.list":
					result = []map[string]any{
						{"domain": ".google.com", "path": "/", "name": "SAPISID", "value": "stable-account-cookie"},
						{"domain": ".google.com", "path": "/", "name": "SID", "value": "sid-stable"},
						{"domain": ".google.com", "path": "/", "name": "__Secure-1PSIDTS", "value": rotationValue.Load()},
					}
				}
				extensionWrites.Lock()
				err := conn.WriteJSON(map[string]any{"id": request.ID, "result": result})
				extensionWrites.Unlock()
				if err != nil {
					return
				}
			}
		}()
		return conn
	}
	_ = connect()
	afterFirstConnect := waitForRuntime(t, manager, accountID, oldRuntime)
	if !br.AccountConnected(accountID) {
		t.Fatal("initial extension connection was not bound to the account")
	}
	if afterFirstConnect.schedulerWorker.ProjectID() != "project-old" {
		t.Fatalf("initial runtime project=%q, want project-old", afterFirstConnect.schedulerWorker.ProjectID())
	}
	activeConn := connect()
	afterReconnect := waitForRuntime(t, manager, accountID, afterFirstConnect)
	if !br.AccountConnected(accountID) {
		t.Fatal("reconnected extension was not bound to the account")
	}

	runner := &accountGateJobRunner{
		manager: manager, entered: make(chan string, 4), started: make(chan *accountRuntime, 4), release: make(chan struct{}),
	}
	jobs, err := NewJobs(st, runner, "bridge-integration-token", 2)
	if err != nil {
		t.Fatal(err)
	}
	jobsDone := make(chan struct{})
	go func() { jobs.Start(serviceCtx); close(jobsDone) }()
	t.Cleanup(func() {
		stopService()
		select {
		case <-jobsDone:
		case <-time.After(3 * time.Second):
			t.Error("job workers did not stop")
		}
	})

	submit := func(key string) string {
		t.Helper()
		body := `{"kind":"image","payload":{"prompt":"test ` + key + `"}}`
		req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer bridge-integration-token")
		req.Header.Set("Idempotency-Key", key)
		res := httptest.NewRecorder()
		jobs.ServeHTTP(res, req)
		if res.Code != http.StatusAccepted {
			t.Fatalf("submit %s returned %d: %s", key, res.Code, res.Body.String())
		}
		var accepted struct {
			JobID string `json:"job_id"`
		}
		if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil || accepted.JobID == "" {
			t.Fatalf("submit %s acknowledgement=%q err=%v", key, res.Body.String(), err)
		}
		return accepted.JobID
	}
	firstJob := submit("rotation-first")
	if _, ok := waitSignal(t, runner.entered, "first job did not enter runner"); !ok {
		t.Fatal("first job missing runner entry")
	}
	var activeRuntime *accountRuntime
	select {
	case activeRuntime = <-runner.started:
	case <-time.After(3 * time.Second):
		t.Fatal("first job did not acquire the account runtime")
	}
	if activeRuntime != afterReconnect {
		t.Fatalf("first job runtime=%p, want reconnected runtime %p", activeRuntime, afterReconnect)
	}

	rotationValue.Store("rotating-new")
	if activeConn == nil {
		t.Fatal("could not find the connected extension websocket for the rotation event")
	}
	extensionWrites.Lock()
	err = activeConn.WriteJSON(map[string]any{
		"event":  "bridge.cookies_rotated",
		"params": map[string]any{"cookies": []any{}, "removed": []any{}, "changedAt": time.Now().UnixMilli()},
	})
	extensionWrites.Unlock()
	if err != nil {
		t.Fatalf("send cookie rotation event: %v", err)
	}
	rotatedRuntime := waitForRuntime(t, manager, accountID, afterReconnect)
	rotatedBundle, err := cookiejar.LoadBundleFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rotatedCookie := ""
	for _, cookie := range rotatedBundle.Cookies {
		if cookie.Name == "__Secure-1PSIDTS" {
			rotatedCookie = cookie.Value
		}
	}
	if rotatedCookie != "rotating-new" || rotatedRuntime.gate != activeRuntime.gate {
		t.Fatalf("rotation bundle/runtime gate mismatch: cookie=%q shared_gate=%v", rotatedCookie, rotatedRuntime.gate == activeRuntime.gate)
	}

	secondJob := submit("rotation-second")
	select {
	case <-runner.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("second job did not enter runner")
	}
	select {
	case runtime := <-runner.started:
		t.Fatalf("second job overlapped the first across cookie rotation; runtime=%p", runtime)
	case <-time.After(60 * time.Millisecond):
	}
	close(runner.release)
	select {
	case runtime := <-runner.started:
		if runtime != rotatedRuntime {
			t.Fatalf("second job runtime=%p, want latest rotated runtime %p", runtime, rotatedRuntime)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second job did not continue after first job released the account gate")
	}
	for _, id := range []string{firstJob, secondJob} {
		waitForJobStatus(t, st, id, "succeeded")
	}
}

func waitForRuntime(t *testing.T, manager *AccountManager, accountID string, previous *accountRuntime) *accountRuntime {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if runtime := manager.account(accountID); runtime != nil && runtime != previous {
			return runtime
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("account %s did not publish a replacement runtime", accountID)
	return nil
}

func waitForTCPListener(addr string) error {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("bridge listener %s did not start", addr)
}

func waitSignal[T any](t *testing.T, ch <-chan T, message string) (T, bool) {
	t.Helper()
	select {
	case value := <-ch:
		return value, true
	case <-time.After(3 * time.Second):
		var zero T
		t.Error(message)
		return zero, false
	}
}

func waitForJobStatus(t *testing.T, st *store.Store, id, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := st.JobByID(context.Background(), id)
		if err == nil && job.Status == want {
			return
		}
		if err == nil && (job.Status == "failed" || job.Status == "submission_unknown") {
			t.Fatalf("job %s ended %s: %s", id, job.Status, job.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	job, err := st.JobByID(context.Background(), id)
	t.Fatalf("job %s did not reach %s: status=%q err=%v", id, want, job.Status, err)
}
