package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kodelyx/flow-go/flow-go/internal/cdp"
	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
)

// newTestBridge returns a bridge with no extension attached and no persisted
// cookie file, so a refresh can only succeed by reusing what the test seeded.
func newTestBridge(t *testing.T) *Bridge {
	t.Helper()
	b := NewBridge(nil, nil, t.TempDir())
	b.cookieFile = filepath.Join(t.TempDir(), "absent.json")
	return b
}

func TestAccountViewPinsItsExtensionClient(t *testing.T) {
	b := newTestBridge(t)
	one := &cdp.Client{RemoteAddr: "profile-one"}
	two := &cdp.Client{RemoteAddr: "profile-two"}
	b.clients[one.RemoteAddr] = one
	b.clients[two.RemoteAddr] = two
	b.accountClients["account-one"] = one
	b.accountClients["account-two"] = two
	b.current = two
	b.jars[one.RemoteAddr] = cookiejar.FromRaw("SAPISID=one", ".google.com", "one")
	b.jars[two.RemoteAddr] = cookiejar.FromRaw("SAPISID=two", ".google.com", "two")

	view := b.AccountView("account-one")
	if view.Current() != one {
		t.Fatalf("account view current = %p, want its own profile %p", view.Current(), one)
	}
	if view.Current() == two {
		t.Fatal("account view borrowed the most recently connected profile")
	}
	if got, want := view.Jar().Hash(), b.jars[one.RemoteAddr].Hash(); got != want {
		t.Fatalf("account view jar hash = %q, want its bound profile jar %q", got, want)
	}
	if view.Jar().Hash() == b.jars[two.RemoteAddr].Hash() {
		t.Fatal("account view borrowed the most recently connected profile's cookie jar")
	}
	if view.cookieSyncLock(one.RemoteAddr) != b.cookieSyncLock(one.RemoteAddr) {
		t.Fatal("account view did not share the parent bridge's per-profile sync lock")
	}
}

func TestProfileOperationsSerializeAccountViewsSharingOneCDPClient(t *testing.T) {
	b := newTestBridge(t)
	shared := &cdp.Client{RemoteAddr: "shared-profile"}
	other := &cdp.Client{RemoteAddr: "other-profile"}
	b.clients[shared.RemoteAddr] = shared
	b.clients[other.RemoteAddr] = other
	b.accountClients["account-a"] = shared
	b.accountClients["account-b"] = shared
	b.accountClients["account-c"] = other
	viewA, viewB, viewC := b.AccountView("account-a"), b.AccountView("account-b"), b.AccountView("account-c")

	releaseA, err := viewA.acquireProfileOperation(context.Background(), viewA.Current())
	if err != nil {
		t.Fatal(err)
	}
	var releaseAOnce sync.Once
	releaseShared := func() { releaseAOnce.Do(releaseA) }
	defer releaseShared()

	acquiredB := make(chan func(), 1)
	failedB := make(chan error, 1)
	go func() {
		release, acquireErr := viewB.acquireProfileOperation(context.Background(), viewB.Current())
		if acquireErr != nil {
			failedB <- acquireErr
			return
		}
		acquiredB <- release
	}()
	select {
	case release := <-acquiredB:
		release()
		t.Fatal("second account entered a profile operation while the shared tab was in use")
	case err := <-failedB:
		t.Fatalf("second account acquire failed: %v", err)
	case <-time.After(30 * time.Millisecond):
	}

	// A distinct profile has an independent permit and must not queue behind the
	// shared browser tab.
	releaseC, err := viewC.acquireProfileOperation(context.Background(), viewC.Current())
	if err != nil {
		t.Fatalf("independent profile was blocked: %v", err)
	}
	releaseC()

	releaseShared()
	select {
	case release := <-acquiredB:
		release()
	case err := <-failedB:
		t.Fatalf("second account acquire failed after release: %v", err)
	case <-time.After(time.Second):
		t.Fatal("second account did not enter after the shared profile permit was released")
	}
	b.profileOperations.mu.Lock()
	remainingLocks := len(b.profileOperations.locks)
	b.profileOperations.mu.Unlock()
	if remainingLocks != 0 {
		t.Fatalf("profile-operation registry retained %d idle locks after all callers exited", remainingLocks)
	}
}

func TestEnsureProjectTabDoesNotInterleaveAcrossSharedAccountViews(t *testing.T) {
	upgrader := websocket.Upgrader{}
	projectNavigation := make(chan struct{}, 4)
	var listCalls atomic.Int32
	var stateMu sync.Mutex
	currentURL := "https://flow.google.com/"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request struct {
				ID     string          `json:"id"`
				Op     string          `json:"op"`
				Params json.RawMessage `json:"params"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			var result any
			switch request.Op {
			case "ping":
				result = map[string]any{"version": "test", "ops": []string{"flow.captcha"}}
			case "tabs.list":
				listCalls.Add(1)
				stateMu.Lock()
				result = []map[string]any{{"id": 1, "url": currentURL}}
				stateMu.Unlock()
			case "tab.attach", "tab.current":
				stateMu.Lock()
				result = map[string]any{"id": 1, "url": currentURL}
				stateMu.Unlock()
			case "flow.navigate":
				var params struct {
					URL string `json:"url"`
				}
				if err := json.Unmarshal(request.Params, &params); err != nil {
					return
				}
				stateMu.Lock()
				currentURL = params.URL
				stateMu.Unlock()
				result = map[string]any{"id": 1, "url": params.URL}
				if strings.Contains(params.URL, "/project/") {
					projectNavigation <- struct{}{}
				}
			case "flow.projects":
				stateMu.Lock()
				pageURL := currentURL
				stateMu.Unlock()
				account := 0
				_, _ = fmt.Sscanf(pageURL, "https://flow.google.com/u/%d/", &account)
				result = []string{fmt.Sprintf("https://flow.google.com/u/%d/project/project-%d", account, account)}
			default:
				result = map[string]any{}
			}
			if err := conn.WriteJSON(map[string]any{"id": request.ID, "result": result}); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	dialCtx, cancelDial := context.WithTimeout(context.Background(), time.Second)
	defer cancelDial()
	client, err := cdp.Dial(dialCtx, "ws"+strings.TrimPrefix(server.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	b := newTestBridge(t)
	b.mu.Lock()
	b.clients[client.RemoteAddr] = client
	b.current = client
	b.accountClients["account-a"] = client
	b.accountClients["account-b"] = client
	b.mu.Unlock()
	viewA, viewB := b.AccountView("account-a"), b.AccountView("account-b")

	ctxA, cancelA := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancelA()
	resultA := make(chan error, 1)
	go func() { _, callErr := viewA.EnsureProjectTab(ctxA, 0); resultA <- callErr }()
	select {
	case <-projectNavigation:
	case <-ctxA.Done():
		t.Fatal("first account never navigated to its project")
	}

	ctxB, cancelB := context.WithCancel(context.Background())
	resultB := make(chan error, 1)
	go func() { _, callErr := viewB.EnsureProjectTab(ctxB, 1); resultB <- callErr }()
	select {
	case err := <-resultA:
		if err == nil || err != context.DeadlineExceeded {
			t.Fatalf("first account result=%v, want its bounded wait to be cancelled", err)
		}
	case <-time.After(100 * time.Millisecond):
	}
	if got := listCalls.Load(); got != 1 {
		cancelB()
		t.Fatalf("second account listed tabs %d times while the first owned the shared tab operation", got)
	}
	select {
	case err := <-resultA:
		if err == nil || err != context.DeadlineExceeded {
			t.Fatalf("first account result=%v, want its bounded wait to be cancelled", err)
		}
	case <-ctxA.Done():
	}

	deadline := time.Now().Add(time.Second)
	for listCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := listCalls.Load(); got < 2 {
		cancelB()
		t.Fatal("second account did not enter after the first account released the shared profile")
	}
	cancelB()
	select {
	case <-resultB:
	case <-time.After(time.Second):
		t.Fatal("second account did not observe cancellation")
	}
}

func TestProfileOperationRegistryCleansCancelledWaiter(t *testing.T) {
	b := newTestBridge(t)
	client := &cdp.Client{RemoteAddr: "cancelled-profile"}
	release, err := b.acquireProfileOperation(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := b.acquireProfileOperation(ctx, client); err != context.DeadlineExceeded {
		release()
		t.Fatalf("blocked acquire error=%v, want context deadline exceeded", err)
	}
	release()
	b.profileOperations.mu.Lock()
	remainingLocks := len(b.profileOperations.locks)
	b.profileOperations.mu.Unlock()
	if remainingLocks != 0 {
		t.Fatalf("profile-operation registry retained %d locks after cancellation and release", remainingLocks)
	}
}

func TestCookieSnapshotIsNotPublishedWhenBundlePersistenceFails(t *testing.T) {
	b := newTestBridge(t)
	client := &cdp.Client{RemoteAddr: "profile-failing-write"}
	jar := cookiejar.FromRaw("SAPISID=sapisid-value; __Secure-1PSID=psid-value", ".google.com", "test")
	accountID := cookiejar.AccountKey(jar)
	bundle := &cookiejar.Bundle{AccountID: accountID, Cookies: jar.Cookies()}

	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(blocker, "account.json")
	callbackCount := 0
	b.OnCookieSync = func(CookieUpdate) { callbackCount++ }
	if err := b.persistCookieSnapshot(client, jar, bundle, path); err == nil {
		t.Fatal("persistence failure should be returned to the caller")
	}
	if b.lastJar != nil {
		t.Fatal("failed snapshot was published as the bridge's latest jar")
	}
	if b.jars[client.RemoteAddr] != nil {
		t.Fatal("failed snapshot was published for the extension client")
	}
	if b.accountClients[accountID] != nil {
		t.Fatal("failed snapshot bound an account to a live extension")
	}
	if callbackCount != 0 {
		t.Fatalf("cookie sync callback count = %d after failed persistence, want 0", callbackCount)
	}
}

func TestReconnectPublishesUnchangedBundleForNewExtensionClient(t *testing.T) {
	b := newTestBridge(t)
	jar := cookiejar.FromRaw("SAPISID=sapisid-value; __Secure-1PSID=psid-value", ".google.com", "test")
	accountID := cookiejar.AccountKey(jar)
	bundle := &cookiejar.Bundle{AccountID: accountID, Cookies: jar.Cookies()}
	path := filepath.Join(t.TempDir(), "account_"+accountID+".json")
	updates := make(chan CookieUpdate, 4)
	b.OnCookieSync = func(update CookieUpdate) { updates <- update }
	first := &cdp.Client{RemoteAddr: "profile-first-connection"}
	second := &cdp.Client{RemoteAddr: "profile-reconnected"}

	if err := b.persistCookieSnapshot(first, jar, bundle, path); err != nil {
		t.Fatal(err)
	}
	if err := b.persistCookieSnapshot(first, jar, bundle, path); err != nil {
		t.Fatal(err)
	}
	if got := len(updates); got != 1 {
		t.Fatalf("same-client syncs published %d updates, want one initial account registration", got)
	}
	if err := b.persistCookieSnapshot(second, jar, bundle, path); err != nil {
		t.Fatal(err)
	}
	if got := len(updates); got != 2 {
		t.Fatalf("unchanged bundle on a new client published %d updates, want reconnect update", got)
	}
	update := <-updates
	if update.Client != first {
		t.Fatalf("first update client=%p, want initial client %p", update.Client, first)
	}
	update = <-updates
	if update.Client != second || update.AccountID != accountID {
		t.Fatalf("reconnect update=(client %p account %s), want (%p,%s)", update.Client, update.AccountID, second, accountID)
	}
}

func TestCookieRotationEventResyncsAndPublishesFreshBundle(t *testing.T) {
	upgrader := websocket.Upgrader{}
	eventConn := make(chan *websocket.Conn, 1)
	rotationValue := atomic.Value{}
	rotationValue.Store("old-rotating-cookie")
	var writeMu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		eventConn <- conn
		for {
			var req struct {
				ID string `json:"id"`
				Op string `json:"op"`
			}
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			var result any = map[string]any{}
			switch req.Op {
			case "ping":
				result = map[string]any{"version": "test"}
			case "cookies.list":
				result = []map[string]any{
					{"domain": ".google.com", "path": "/", "name": "SAPISID", "value": "stable-account-cookie"},
					{"domain": ".google.com", "path": "/", "name": "__Secure-1PSIDTS", "value": rotationValue.Load()},
				}
			case "tabs.list":
				result = []any{}
			}
			writeMu.Lock()
			err := conn.WriteJSON(map[string]any{"id": req.ID, "result": result})
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	client, err := cdp.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"))
	if err != nil {
		t.Fatalf("dial fake extension: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	b := newTestBridge(t)
	b.CookieDomains = []string{".google.com"}
	bundleDir := t.TempDir()
	b.cookieFile = filepath.Join(bundleDir, "cookies.json")
	updates := make(chan CookieUpdate, 2)
	b.OnCookieSync = func(update CookieUpdate) { updates <- update }

	connected := make(chan struct{})
	go func() {
		b.onConnect(ctx, client)
		close(connected)
	}()
	select {
	case <-connected:
	case <-time.After(3 * time.Second):
		t.Fatal("extension handshake and initial cookie sync did not finish")
	}
	var initial CookieUpdate
	select {
	case initial = <-updates:
	case <-time.After(time.Second):
		t.Fatal("initial connection did not publish its cookie bundle")
	}
	if initial.Jar == nil || initial.Jar.Count() != 2 {
		t.Fatalf("initial sync jar = %#v, want both account cookies", initial.Jar)
	}

	rotationValue.Store("new-rotating-cookie")
	select {
	case conn := <-eventConn:
		writeMu.Lock()
		err := conn.WriteJSON(map[string]any{
			"event":  "bridge.cookies_rotated",
			"params": map[string]any{"cookies": []any{}, "removed": []any{}, "changedAt": time.Now().UnixMilli()},
		})
		writeMu.Unlock()
		if err != nil {
			t.Fatalf("send rotation event: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fake extension did not retain its websocket")
	}

	var rotated CookieUpdate
	select {
	case rotated = <-updates:
	case <-time.After(3 * time.Second):
		t.Fatal("rotation event did not trigger a full cookie sync")
	}
	if rotated.AccountID != initial.AccountID || rotated.Path != initial.Path {
		t.Fatalf("rotation rebound account: before=(%q,%q) after=(%q,%q)", initial.AccountID, initial.Path, rotated.AccountID, rotated.Path)
	}
	if rotated.Jar == nil || !strings.Contains(rotated.Jar.Raw(), "new-rotating-cookie") {
		t.Fatalf("rotation published stale jar: %v", rotated.Jar)
	}
	persisted, err := cookiejar.LoadBundleFile(rotated.Path)
	if err != nil {
		t.Fatalf("load persisted rotated bundle: %v", err)
	}
	persistedRotation := false
	for _, cookie := range persisted.Cookies {
		if cookie.Name == "__Secure-1PSIDTS" && cookie.Value == "new-rotating-cookie" {
			persistedRotation = true
		}
	}
	if !persistedRotation {
		t.Fatalf("persisted bundle did not contain the rotated cookie: %#v", persisted.Cookies)
	}
}

func TestCookieSyncAgeUsesDurableBundleTimestamp(t *testing.T) {
	b := newTestBridge(t)
	accountID := "account-test"
	path := b.AccountCookieFile(accountID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"cookies":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	wantTime := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(path, wantTime, wantTime); err != nil {
		t.Fatal(err)
	}
	age, ok := b.CookieSyncAge(accountID)
	if !ok {
		t.Fatal("existing account bundle was reported as missing")
	}
	if age < 110*time.Second || age > 130*time.Second {
		t.Fatalf("cookie sync age = %s, want approximately 2m", age)
	}
	if _, ok := b.CookieSyncAge("missing-account"); ok {
		t.Fatal("missing bundle returned a sync age")
	}
}

func seedJar(b *Bridge, raw string) *cookiejar.Jar {
	jar := cookiejar.FromRaw(raw, ".google.com", "test")
	b.mu.Lock()
	b.lastJar = jar
	b.mu.Unlock()
	return jar
}

// TestRefreshSessionReusesARecentRefresh covers the coalescing rule.
//
// Every worker draws on the same account, so they share its cookies and go stale
// together: a fleet hitting a 401 at the same moment would otherwise each attach
// to the same tab and wait out the full pause. A refresh that finished inside the
// window is served from the jar instead.
//
// No extension is attached here, so anything that tried a real refresh would fail
// with "no extension connected" — returning the jar is the proof it was reused.
func TestRefreshSessionReusesARecentRefresh(t *testing.T) {
	b := newTestBridge(t)
	jar := seedJar(b, "__Secure-1PSID=abc; SID=def")
	b.lastRefresh = time.Now()

	got, err := b.RefreshSession(context.Background())
	if err != nil {
		t.Fatalf("a refresh inside the window should be reused, got: %v", err)
	}
	if got == nil || got.Hash() != jar.Hash() {
		t.Fatalf("reused the wrong jar: %v", got)
	}
}

// TestRefreshSessionOutsideTheWindowDoesNotReuse is the other half of the rule.
//
// Once the window has passed the jar must not be handed back, or a genuinely
// stale session would be served forever and the engine would never recover.
func TestRefreshSessionOutsideTheWindowDoesNotReuse(t *testing.T) {
	b := newTestBridge(t)
	seedJar(b, "__Secure-1PSID=abc")
	b.lastRefresh = time.Now().Add(-RefreshCoalesceWindow - time.Second)

	if _, err := b.RefreshSession(context.Background()); err == nil {
		t.Fatal("a refresh older than the window must not be reused")
	}
}

// TestRefreshSessionWithoutAJarFallsThrough pins that the timestamp alone is not
// enough. A window that is recent but has no jar to serve must still attempt a
// real refresh, otherwise a first-ever call would report success with nothing.
func TestRefreshSessionWithoutAJarFallsThrough(t *testing.T) {
	b := newTestBridge(t)
	b.lastRefresh = time.Now()

	if _, err := b.RefreshSession(context.Background()); err == nil {
		t.Fatal("a recent timestamp with no jar must fall through to a real refresh")
	}
}

// TestRefreshSessionDoesNotCoalesceFailures pins that only a *completed* refresh
// opens the window. If a failure reset it, the next caller would be told a good
// refresh had just happened while the session was still dead.
func TestRefreshSessionDoesNotCoalesceFailures(t *testing.T) {
	b := newTestBridge(t)
	seedJar(b, "__Secure-1PSID=abc")

	// No extension, so this fails and must leave the window closed.
	if _, err := b.RefreshSession(context.Background()); err == nil {
		t.Fatal("expected the refresh to fail with no extension attached")
	}

	if !b.lastRefresh.IsZero() {
		t.Fatalf("a failed refresh must not open the coalescing window, got %s", b.lastRefresh)
	}

	// And a second caller must therefore also try for real rather than reuse.
	if _, err := b.RefreshSession(context.Background()); err == nil {
		t.Fatal("a failed refresh must not be reused by the next caller")
	}
}

func TestConcurrentRefreshFailureIsSharedWithoutRetryStorm(t *testing.T) {
	const callers = 16
	upgrader := websocket.Upgrader{}
	attachStarted := make(chan struct{})
	releaseAttach := make(chan struct{})
	var attachCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request struct {
				ID string `json:"id"`
				Op string `json:"op"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			if request.Op == "tab.attach" {
				if attachCalls.Add(1) == 1 {
					close(attachStarted)
				}
				<-releaseAttach
				encoded, _ := json.Marshal(map[string]any{
					"id":    request.ID,
					"error": map[string]string{"message": "simulated extension attach failure"},
				})
				if err := conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client, err := cdp.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	b := newTestBridge(t)
	b.mu.Lock()
	b.clients[client.RemoteAddr] = client
	b.current = client
	b.mu.Unlock()

	start := make(chan struct{})
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() {
			<-start
			_, refreshErr := b.RefreshSession(ctx)
			results <- refreshErr
		}()
	}
	close(start)
	select {
	case <-attachStarted:
	case <-ctx.Done():
		t.Fatal("no refresh attempted extension attach")
	}

	// Keep the leader inside Attach until every other caller has joined its
	// in-flight refresh. The wait count is protected by the same flight mutex.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		b.refreshMu.Lock()
		waiters := 0
		if b.refreshing != nil {
			waiters = b.refreshing.waiters
		}
		b.refreshMu.Unlock()
		if waiters == callers-1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	b.refreshMu.Lock()
	waiters := 0
	if b.refreshing != nil {
		waiters = b.refreshing.waiters
	}
	b.refreshMu.Unlock()
	if waiters != callers-1 {
		close(releaseAttach)
		t.Fatalf("joined refresh waiters=%d, want %d", waiters, callers-1)
	}
	close(releaseAttach)

	for i := 0; i < callers; i++ {
		select {
		case refreshErr := <-results:
			if refreshErr == nil || !strings.Contains(refreshErr.Error(), "simulated extension attach failure") {
				t.Errorf("refresh error=%v, want shared attach failure", refreshErr)
			}
		case <-ctx.Done():
			t.Fatal("refresh callers did not receive the completed flight result")
		}
	}
	if got := attachCalls.Load(); got != 1 {
		t.Fatalf("attach calls for concurrent failure=%d, want exactly one", got)
	}
	if !b.lastRefresh.IsZero() {
		t.Fatalf("failed shared refresh opened the successful-cache window at %s", b.lastRefresh)
	}

	// Failure is shared only with callers in that flight. A later independent
	// request remains able to attempt recovery instead of reusing a cached error.
	if _, err := b.RefreshSession(ctx); err == nil {
		t.Fatal("a new refresh attempt after the failed flight unexpectedly succeeded")
	}
	if got := attachCalls.Load(); got != 2 {
		t.Fatalf("later independent attach calls=%d, want 2 total", got)
	}
}

func TestAccountRefreshFlightsDoNotBlockOtherAccounts(t *testing.T) {
	upgrader := websocket.Upgrader{}
	attachStarted := make(chan struct{}, 2)
	releaseAttach := make(chan struct{})
	var attachCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request struct {
				ID string `json:"id"`
				Op string `json:"op"`
			}
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			if request.Op != "tab.attach" {
				continue
			}
			attachCalls.Add(1)
			attachStarted <- struct{}{}
			<-releaseAttach
			encoded, _ := json.Marshal(map[string]any{
				"id":    request.ID,
				"error": map[string]string{"message": "simulated account-scoped attach failure"},
			})
			if err := conn.WriteMessage(websocket.TextMessage, encoded); err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	newClient := func() *cdp.Client {
		t.Helper()
		client, err := cdp.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		return client
	}
	clientA, clientB := newClient(), newClient()
	b := newTestBridge(t)
	b.mu.Lock()
	b.clients[clientA.RemoteAddr] = clientA
	b.clients[clientB.RemoteAddr] = clientB
	b.accountClients["account-a"] = clientA
	b.accountClients["account-b"] = clientB
	b.current = clientA
	b.jars[clientA.RemoteAddr] = cookiejar.FromRaw("SAPISID=a", ".google.com", "account-a")
	b.jars[clientB.RemoteAddr] = cookiejar.FromRaw("SAPISID=b", ".google.com", "account-b")
	b.mu.Unlock()
	viewA, viewB := b.AccountView("account-a"), b.AccountView("account-b")

	results := make(chan error, 2)
	go func() { _, err := viewA.RefreshSession(ctx); results <- err }()
	go func() { _, err := viewB.RefreshSession(ctx); results <- err }()
	for i := 0; i < 2; i++ {
		select {
		case <-attachStarted:
		case <-ctx.Done():
			close(releaseAttach)
			t.Fatal("one account's refresh blocked the other account from attaching")
		}
	}
	close(releaseAttach)
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err == nil || !strings.Contains(err.Error(), "simulated account-scoped attach failure") {
				t.Errorf("account refresh error=%v, want its own attach error", err)
			}
		case <-ctx.Done():
			t.Fatal("both account refreshes did not complete")
		}
	}
	if got := attachCalls.Load(); got != 2 {
		t.Fatalf("account-scoped attach calls=%d, want 2 concurrent attempts", got)
	}
}

// TestRefreshCoalesceWindowIsShort keeps the window honest. It exists to absorb a
// burst of 401s, which arrives in well under a second; a long window would serve
// an ageing jar to callers that genuinely needed a new one.
func TestRefreshCoalesceWindowIsShort(t *testing.T) {
	if RefreshCoalesceWindow <= 0 {
		t.Fatal("the window must be positive")
	}
	if RefreshCoalesceWindow > 10*time.Second {
		t.Errorf("a %s window is long enough to serve a stale jar", RefreshCoalesceWindow)
	}
}

// TestURLMatchesAccount pins the account test that keeps a project from being
// reused across accounts.
//
// Projects belong to one account, so a URL remembered from another does not open
// — it 404s — and the engine would then hold a project id its session cannot use.
// Google addresses accounts with `/u/<n>/`, and a URL without one is the first.
func TestURLMatchesAccount(t *testing.T) {
	cases := []struct {
		url   string
		index int
		want  bool
	}{
		{"https://flow.google.com/project/abc", 0, true},
		{"https://flow.google.com/project/abc", 1, false},
		{"https://flow.google.com/project/abc", 2, false},

		{"https://flow.google.com/u/0/project/abc", 0, true},
		{"https://flow.google.com/u/1/project/abc", 1, true},
		{"https://flow.google.com/u/2/project/abc", 2, true},
		{"https://flow.google.com/u/1/project/abc", 0, false},
		{"https://flow.google.com/u/1/project/abc", 2, false},

		{"https://flow.google.com/u/2/?pli=1", 2, true},
		{"https://flow.google.com/u/2/?pli=1", 1, false},

		// A malformed or trailing index must not be mistaken for account 0 when
		// the caller asked for something else.
		{"https://flow.google.com/u/x/project/abc", 1, false},
		{"https://flow.google.com/u/", 1, false},
		{"https://flow.google.com/", 0, true},
		{"https://flow.google.com/", 2, false},
	}

	for _, tc := range cases {
		if got := urlMatchesAccount(tc.url, tc.index); got != tc.want {
			t.Errorf("urlMatchesAccount(%q, %d) = %v, want %v", tc.url, tc.index, got, tc.want)
		}
	}
}
