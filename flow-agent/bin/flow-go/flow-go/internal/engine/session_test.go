package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
)

// readyEngine builds the state a request arrives in on a healthy engine: it has
// bootstrapped, it has a registered worker, and it has no bridge.
//
// No bridge is deliberate. The engine is designed to run without one — the
// cookies persisted under cookies/ are a supported source — so "no browser" must
// not, on its own, be treated as unserviceable. Several of the tests below assert
// exactly that.
func readyEngine(t *testing.T) *Engine {
	t.Helper()
	e := identityEngine(t, Options{})
	e.accountID = "acct-test"
	e.ready = true
	e.pool.Register(pool.NewWorker("acct-test", nil))
	return e
}

func TestEngineSessionRecoveryCoalescesRefreshAndBootstrap(t *testing.T) {
	const callers = 12
	e := readyEngine(t)
	jar := cookiejar.FromRaw("SAPISID=fresh", ".google.com", "test")
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	refresh := func(context.Context) (*cookiejar.Jar, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return jar, nil
	}

	start := make(chan struct{})
	results := make(chan *cookiejar.Jar, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := e.coalesceSessionRefresh(context.Background(), refresh)
			if err != nil {
				t.Errorf("coalesced session recovery: %v", err)
			}
			results <- got
		}()
	}
	close(start)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("session recovery did not start")
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		e.refreshMu.Lock()
		waiters := 0
		if e.refreshing != nil {
			waiters = e.refreshing.waiters
		}
		e.refreshMu.Unlock()
		if waiters == callers-1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	e.refreshMu.Lock()
	waiters := 0
	if e.refreshing != nil {
		waiters = e.refreshing.waiters
	}
	e.refreshMu.Unlock()
	if waiters != callers-1 {
		close(release)
		t.Fatalf("session recovery waiters=%d, want %d", waiters, callers-1)
	}
	close(release)
	wg.Wait()
	close(results)
	for got := range results {
		if got == nil || got.Hash() != jar.Hash() {
			t.Errorf("recovery returned jar %v, want the fresh account jar", got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("refresh/bootstrap recovery executions=%d, want 1", got)
	}
	if got, err := e.coalesceSessionRefresh(context.Background(), func(context.Context) (*cookiejar.Jar, error) {
		calls.Add(1)
		return nil, errors.New("must not bootstrap again inside the recovery window")
	}); err != nil || got == nil || got.Hash() != jar.Hash() {
		t.Fatalf("recent session recovery=(%v,%v), want the cached fresh jar", got, err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("recent recovery ran bootstrap again: calls=%d", got)
	}
}

// TestPreflightPassesWithoutABrowser is the boundary the design turns on.
//
// A browserless engine is a supported mode, not a broken one. If the preflight
// refused on "no extension attached" it would break that mode, and it would be
// the same over-reach in the opposite direction from the slow failure it is here
// to fix.
func TestPreflightPassesWithoutABrowser(t *testing.T) {
	e := readyEngine(t)

	if e.bridge != nil {
		t.Fatal("the fixture is wrong: this engine is meant to have no bridge")
	}
	if err := e.preflight(); err != nil {
		t.Errorf("preflight refused an engine with no bridge: %v — a browserless run is "+
			"a supported mode and must not be rejected for lacking one", err)
	}
	if got := e.SessionFailure(); got != "" {
		t.Errorf("SessionFailure() = %q on a healthy engine, want empty", got)
	}
}

// TestPreflightRejectsAnUnreadyEngine keeps the original gate's contract.
//
// The wording is load-bearing: `statusFor` used to map "not ready" to a 503 for
// the HTTP API, and that is gone — but the phrase is still what a caller or a log
// grep matches on, so it is kept stable rather than reworded.
func TestPreflightRejectsAnUnreadyEngine(t *testing.T) {
	e := identityEngine(t, Options{}) // ready is false
	e.accountID = "acct-test"

	err := e.preflight()
	if err == nil {
		t.Fatal("preflight passed an engine that has never bootstrapped")
	}

	var unavailable *UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error is %T, want *UnavailableError", err)
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("error %q must keep the phrase callers match on", err)
	}
	if unavailable.Hint == "" {
		t.Error("an unready engine should say how to make it ready")
	}
}

// TestPreflightRejectsADeadSession is the #4 regression test: the whole point of
// the change is that this is refused up front rather than 25 seconds later.
func TestPreflightRejectsADeadSession(t *testing.T) {
	e := readyEngine(t)

	e.markSessionDead(deadSessionReason())

	err := e.preflight()
	if err == nil {
		t.Fatal("preflight passed an engine whose session is dead")
	}

	var unavailable *UnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("error is %T, want *UnavailableError so a caller can tell "+
			"'this will not work until something changes' from 'this request failed'", err)
	}
	if !strings.Contains(err.Error(), "session expired") {
		t.Errorf("error %q should name the session, not the symptom", err)
	}
	// The action is the part that makes this useful rather than merely accurate.
	// It has to name a command that exists: this hint used to say "call POST
	// /v1/bridge/refresh", and removing the HTTP API turned that into advice
	// leading to a refused connection.
	if !strings.Contains(err.Error(), "flow-go bridge") {
		t.Errorf("error %q should say what to do about it", err)
	}
	if strings.Contains(err.Error(), "/v1/") {
		t.Errorf("error %q names an endpoint that no longer exists", err)
	}
}

// TestPreflightIsCheap proves the gate makes no upstream call.
//
// The fixture has no bridge, no cookies and no client, so anything that tried to
// reach the network would fail rather than pass. A preflight that verified the
// cookies were still valid would cost a round trip on every request — which is
// the expense it exists to avoid.
func TestPreflightIsCheap(t *testing.T) {
	e := readyEngine(t)

	for i := 0; i < 100; i++ {
		if err := e.preflight(); err != nil {
			t.Fatalf("preflight %d failed on a healthy engine: %v", i, err)
		}
	}
}

// TestSessionFailureIsRecordedAndCleared covers the latch itself.
func TestSessionFailureIsRecordedAndCleared(t *testing.T) {
	e := readyEngine(t)

	if got := e.SessionFailure(); got != "" {
		t.Fatalf("SessionFailure() = %q before anything went wrong, want empty", got)
	}

	e.markSessionDead("the session expired")
	if got := e.SessionFailure(); got != "the session expired" {
		t.Errorf("SessionFailure() = %q, want the recorded reason", got)
	}

	e.noteSessionRestored()
	if got := e.SessionFailure(); got != "" {
		t.Errorf("SessionFailure() = %q after a successful bootstrap, want empty", got)
	}
	if err := e.preflight(); err != nil {
		t.Errorf("preflight still refuses after the session was restored: %v", err)
	}
}

// TestMarkSessionDeadKeepsTheFirstReason checks the cause is not overwritten.
//
// The first caller is the one that saw the actual failure — a 401 with no
// browser attached. Every later caller is only seeing the consequence, and a
// consequence is a worse description of the problem than the cause.
func TestMarkSessionDeadKeepsTheFirstReason(t *testing.T) {
	e := readyEngine(t)

	e.markSessionDead("the first and true reason")
	e.markSessionDead("a later and vaguer reason")

	if got := e.SessionFailure(); got != "the first and true reason" {
		t.Errorf("SessionFailure() = %q, want the first reason to survive", got)
	}
}

// TestMarkSessionDeadDrainsThePool is the test that gives Pool.Remove a caller.
//
// The pool's worker wraps the legacy REST client, which is authenticated by the
// same dead session, so leaving it registered keeps a worker in rotation that
// cannot serve. Nothing else ever told the pool the session had gone — Remove
// existed with no production caller at all.
func TestMarkSessionDeadDrainsThePool(t *testing.T) {
	e := readyEngine(t)

	if e.pool.Size() != 1 {
		t.Fatalf("the fixture should start with one registered worker, got %d", e.pool.Size())
	}

	e.markSessionDead(deadSessionReason())

	if got := e.pool.Size(); got != 0 {
		t.Errorf("pool size = %d after the session died, want 0 — the worker cannot "+
			"authenticate and must not stay in rotation", got)
	}
}

// TestUnavailableErrorOmitsAnEmptyHint checks the two-part message degrades
// cleanly, since the type is used for cases where there is no advice to give.
func TestUnavailableErrorOmitsAnEmptyHint(t *testing.T) {
	plain := (&UnavailableError{Reason: "something is wrong"}).Error()
	if plain != "something is wrong" {
		t.Errorf("Error() = %q, want the reason alone", plain)
	}

	withHint := (&UnavailableError{Reason: "something is wrong", Hint: "do this"}).Error()
	if withHint != "something is wrong — do this" {
		t.Errorf("Error() = %q, want the hint appended", withHint)
	}
}
