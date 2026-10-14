package batchexecute

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kodelyx/flow-go/flow-go/internal/cookiejar"
)

func TestSessionRefreshRunsOnlyForUnauthorizedResponses(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(fmt.Sprintf("http_%d", status), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				http.Error(w, "request refused", status)
			}))
			defer server.Close()
			pointAt(t, server)

			client := testClient(t)
			var refreshes atomic.Int32
			client.SetUnauthorizedHandler(func(context.Context) (*cookiejar.Jar, error) {
				refreshes.Add(1)
				return nil, nil
			})
			if _, err := client.call(context.Background(), RPCIDProjectList, CallOptions{}, payloadWithToken); err == nil {
				t.Fatal("expected the HTTP refusal to be returned")
			}
			if got := refreshes.Load(); got != 0 {
				t.Fatalf("session refresh count=%d for HTTP %d, want 0", got, status)
			}
			if got := requests.Load(); got != 1 {
				t.Fatalf("request count=%d for HTTP %d, want 1", got, status)
			}
		})
	}
}

func TestUnauthorizedRefreshFailureDoesNotRetryRequest(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "expired session", http.StatusUnauthorized)
	}))
	defer server.Close()
	pointAt(t, server)

	client := testClient(t)
	var refreshes atomic.Int32
	client.SetUnauthorizedHandler(func(context.Context) (*cookiejar.Jar, error) {
		refreshes.Add(1)
		return nil, fmt.Errorf("extension refresh failed")
	})
	_, err := client.call(context.Background(), RPCIDProjectList, CallOptions{}, payloadWithToken)
	if err == nil || !strings.Contains(err.Error(), "session refresh failed") {
		t.Fatalf("401 refresh result=%v, want refresh failure", err)
	}
	if refreshes.Load() != 1 || requests.Load() != 1 {
		t.Fatalf("refreshes=%d requests=%d, want exactly 1 refresh and no replay", refreshes.Load(), requests.Load())
	}
}

func TestUnauthorizedRefreshRetriesOnceWithFreshSession(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "expired session", http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprint(w, ")]}'\n\n[[\"wrb.fr\",\"UpteDb\",\"[]\",null,null,null,\"generic\"]]\n")
	}))
	defer server.Close()
	pointAt(t, server)

	client := testClient(t)
	fresh := cookiejar.FromRaw("SAPISID=fresh", ".google.com", "test")
	var refreshes atomic.Int32
	client.SetUnauthorizedHandler(func(context.Context) (*cookiejar.Jar, error) {
		refreshes.Add(1)
		return fresh, nil
	})
	if _, err := client.call(context.Background(), RPCIDProjectList, CallOptions{}, payloadWithToken); err != nil {
		t.Fatalf("read RPC after session recovery: %v", err)
	}
	if refreshes.Load() != 1 || requests.Load() != 2 {
		t.Fatalf("refreshes=%d requests=%d, want one refresh and one replay", refreshes.Load(), requests.Load())
	}
}
