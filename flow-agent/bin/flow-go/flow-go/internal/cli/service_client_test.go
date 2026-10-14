package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func TestServiceAccountIDExtractsRegisteredBundleID(t *testing.T) {
	if got, err := serviceAccountID(`cookies\account_ab12cd34.json`); err != nil || got != "ab12cd34" {
		t.Fatalf("service account selector=(%q,%v), want ab12cd34", got, err)
	}
	for _, path := range []string{"cookies.json", "account_.json", "other_account_ab12.json"} {
		if _, err := serviceAccountID(path); err == nil {
			t.Errorf("service selector %q should be rejected", path)
		}
	}
}

func TestRunServiceJobSubmitsOnceAndPollsDurableResult(t *testing.T) {
	previousURL, hadURL := os.LookupEnv("FLOW_SERVICE_URL")
	previousToken, hadToken := os.LookupEnv("FLOW_INTERNAL_TOKEN")
	t.Cleanup(func() {
		if hadURL {
			_ = os.Setenv("FLOW_SERVICE_URL", previousURL)
		} else {
			_ = os.Unsetenv("FLOW_SERVICE_URL")
		}
		if hadToken {
			_ = os.Setenv("FLOW_INTERNAL_TOKEN", previousToken)
		} else {
			_ = os.Unsetenv("FLOW_INTERNAL_TOKEN")
		}
	})

	var posts atomic.Int32
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cli-test-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/internal/v1/jobs":
			posts.Add(1)
			if r.Header.Get("Idempotency-Key") == "" {
				http.Error(w, "missing idempotency key", http.StatusBadRequest)
				return
			}
			var body struct {
				Kind    string         `json:"kind"`
				Payload map[string]any `json:"payload"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Kind != "image" || body.Payload["prompt"] != "test prompt" {
				http.Error(w, "bad payload", http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"cli-job-1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/jobs/cli-job-1":
			if polls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"job_id":"cli-job-1","status":"running"}`))
				return
			}
			_, _ = w.Write([]byte(`{"job_id":"cli-job-1","status":"succeeded","result":{"status":"succeeded","media":[{"media_id":"asset-1"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	_ = os.Setenv("FLOW_SERVICE_URL", server.URL)
	_ = os.Setenv("FLOW_INTERNAL_TOKEN", "cli-test-token")

	if code := runServiceJob("image", map[string]any{"prompt": "test prompt"}, ""); code != 0 {
		t.Fatalf("runServiceJob exit=%d, want success", code)
	}
	if got := posts.Load(); got != 1 {
		t.Fatalf("POST count=%d, want one durable submission", got)
	}
	if got := polls.Load(); got < 2 {
		t.Fatalf("poll count=%d, want queued job to be polled through terminal state", got)
	}
}

func TestRunServiceJobDoesNotPostWithoutToken(t *testing.T) {
	previousURL, hadURL := os.LookupEnv("FLOW_SERVICE_URL")
	previousToken, hadToken := os.LookupEnv("FLOW_INTERNAL_TOKEN")
	t.Cleanup(func() {
		if hadURL {
			_ = os.Setenv("FLOW_SERVICE_URL", previousURL)
		} else {
			_ = os.Unsetenv("FLOW_SERVICE_URL")
		}
		if hadToken {
			_ = os.Setenv("FLOW_INTERNAL_TOKEN", previousToken)
		} else {
			_ = os.Unsetenv("FLOW_INTERNAL_TOKEN")
		}
	})
	_ = os.Setenv("FLOW_SERVICE_URL", "http://127.0.0.1:1")
	_ = os.Unsetenv("FLOW_INTERNAL_TOKEN")
	if code := runServiceJob("image", map[string]any{"prompt": "test prompt"}, ""); code == 0 {
		t.Fatal("runServiceJob succeeded without the required token")
	}
}

func TestRunServiceJobRecoversServerErrorByIdempotencyKeyWithoutResubmitting(t *testing.T) {
	previousURL, hadURL := os.LookupEnv("FLOW_SERVICE_URL")
	previousToken, hadToken := os.LookupEnv("FLOW_INTERNAL_TOKEN")
	t.Cleanup(func() {
		if hadURL {
			_ = os.Setenv("FLOW_SERVICE_URL", previousURL)
		} else {
			_ = os.Unsetenv("FLOW_SERVICE_URL")
		}
		if hadToken {
			_ = os.Setenv("FLOW_INTERNAL_TOKEN", previousToken)
		} else {
			_ = os.Unsetenv("FLOW_INTERNAL_TOKEN")
		}
	})
	var posts atomic.Int32
	var lookups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cli-test-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			posts.Add(1)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/internal/v1/jobs/by-key" {
			lookups.Add(1)
			if r.URL.Query().Get("key") != "stable-key" {
				http.Error(w, "missing key", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"job_id":"recovered-job","status":"succeeded","result":{"status":"succeeded"}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	_ = os.Setenv("FLOW_SERVICE_URL", server.URL)
	_ = os.Setenv("FLOW_INTERNAL_TOKEN", "cli-test-token")

	if code := runServiceJob("image", map[string]any{"prompt": "retry-safe"}, "stable-key"); code != 0 {
		t.Fatalf("runServiceJob exit=%d, want recovered success", code)
	}
	if posts.Load() != 1 || lookups.Load() != 1 {
		t.Fatalf("requests: posts=%d lookups=%d, want one POST and one by-key lookup", posts.Load(), lookups.Load())
	}
}

func TestRunServiceJobFetchesStoredResultForTerminalReplayAcknowledgement(t *testing.T) {
	previousURL, hadURL := os.LookupEnv("FLOW_SERVICE_URL")
	previousToken, hadToken := os.LookupEnv("FLOW_INTERNAL_TOKEN")
	t.Cleanup(func() {
		if hadURL {
			_ = os.Setenv("FLOW_SERVICE_URL", previousURL)
		} else {
			_ = os.Unsetenv("FLOW_SERVICE_URL")
		}
		if hadToken {
			_ = os.Setenv("FLOW_INTERNAL_TOKEN", previousToken)
		} else {
			_ = os.Unsetenv("FLOW_INTERNAL_TOKEN")
		}
	})
	var statusReads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cli-test-token" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			// This is the lightweight writeAcceptedJob response for an existing key.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"job_id":"replayed-job","status":"succeeded","status_url":"/internal/v1/jobs/replayed-job"}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/internal/v1/jobs/replayed-job" {
			statusReads.Add(1)
			_, _ = w.Write([]byte(`{"job_id":"replayed-job","status":"succeeded","result":{"media":[{"media_id":"stored-asset"}]}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	_ = os.Setenv("FLOW_SERVICE_URL", server.URL)
	_ = os.Setenv("FLOW_INTERNAL_TOKEN", "cli-test-token")

	if code := runServiceJob("image", map[string]any{"prompt": "same request"}, "existing-key"); code != 0 {
		t.Fatalf("runServiceJob exit=%d, want replayed success", code)
	}
	if statusReads.Load() != 1 {
		t.Fatalf("status reads=%d, want one read for the persisted terminal result", statusReads.Load())
	}
}

func TestTerminalServiceStatus(t *testing.T) {
	for _, status := range []string{"succeeded", "partial", "failed", "cancelled", "submission_unknown"} {
		if !terminalServiceStatus(status) {
			t.Errorf("%q should be terminal", status)
		}
	}
	for _, status := range []string{"queued", "running", "preparing", "rendering", "downloading"} {
		if terminalServiceStatus(status) {
			t.Errorf("%q should not be terminal", status)
		}
	}
}
