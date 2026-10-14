package app

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/service"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

func TestJobMaxAttemptsConfiguration(t *testing.T) {
	t.Setenv("FLOW_JOB_MAX_ATTEMPTS", "")
	if got, err := jobMaxAttemptsFromEnv(); err != nil || got != 3 {
		t.Fatalf("default max attempts=(%d,%v), want (3,nil)", got, err)
	}
	for _, value := range []string{"1", "5", "20"} {
		t.Setenv("FLOW_JOB_MAX_ATTEMPTS", value)
		got, err := jobMaxAttemptsFromEnv()
		if err != nil || got == 0 {
			t.Errorf("configured max attempts %q=(%d,%v), want a positive budget", value, got, err)
		}
	}
	for _, value := range []string{"0", "-1", "21", "many"} {
		t.Setenv("FLOW_JOB_MAX_ATTEMPTS", value)
		if _, err := jobMaxAttemptsFromEnv(); err == nil {
			t.Errorf("max attempts value %q should be rejected", value)
		}
	}
}

func TestServiceStatsExposeLatencyByKind(t *testing.T) {
	t.Setenv("FLOW_COOKIE_DIR", t.TempDir())
	st, err := store.Open(filepath.Join(t.TempDir(), "stats.db"))
	if err != nil {
		t.Fatal(err)
	}
	br := bridge.NewBridge(nil, nil, t.TempDir())
	br.ListenAddr = "127.0.0.1:0"
	app := &App{Store: st, Bridge: br}
	defer app.Close()

	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.RunService(ctx, addr, "stats-test-token", 1) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("stop service: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("service did not stop")
		}
	})

	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	var response *http.Response
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/internal/v1/stats", nil)
		req.Header.Set("Authorization", "Bearer stats-test-token")
		response, err = client.Do(req)
		if err == nil {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("request service stats: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("stats status=%d, want 200", response.StatusCode)
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"latency", "latency_by_kind"} {
		if _, ok := body[key]; !ok {
			t.Errorf("stats response misses %q", key)
		}
	}
}

func TestPositiveEnvInt(t *testing.T) {
	t.Setenv("FLOW_TEST_LIMIT", "")
	if got, err := positiveEnvInt("FLOW_TEST_LIMIT", 7); err != nil || got != 7 {
		t.Fatalf("default=(%d,%v), want (7,nil)", got, err)
	}
	t.Setenv("FLOW_TEST_LIMIT", "12")
	if got, err := positiveEnvInt("FLOW_TEST_LIMIT", 7); err != nil || got != 12 {
		t.Fatalf("configured=(%d,%v), want (12,nil)", got, err)
	}
	for _, value := range []string{"0", "-2", "many"} {
		t.Setenv("FLOW_TEST_LIMIT", value)
		if _, err := positiveEnvInt("FLOW_TEST_LIMIT", 7); err == nil {
			t.Errorf("value %q should be rejected", value)
		}
	}
}

func TestJobRetryBudgetConfiguration(t *testing.T) {
	t.Setenv("FLOW_JOB_MAX_RESUME_ATTEMPTS", "")
	if got, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_RESUME_ATTEMPTS", 4); err != nil || got != 4 {
		t.Fatalf("default retry budget=(%d,%v), want (4,nil)", got, err)
	}
	for _, value := range []string{"1", "7", "20"} {
		t.Setenv("FLOW_JOB_MAX_RESUME_ATTEMPTS", value)
		got, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_RESUME_ATTEMPTS", 4)
		if err != nil || got < 1 {
			t.Errorf("configured retry budget %q=(%d,%v), want positive budget", value, got, err)
		}
	}
	for _, value := range []string{"0", "21", "many"} {
		t.Setenv("FLOW_JOB_MAX_RESUME_ATTEMPTS", value)
		if _, err := jobRetryBudgetFromEnv("FLOW_JOB_MAX_RESUME_ATTEMPTS", 4); err == nil {
			t.Errorf("retry budget value %q should be rejected", value)
		}
	}
}

func TestQueueWorkersConfiguration(t *testing.T) {
	t.Setenv("FLOW_QUEUE_WORKERS", "")
	if got, err := queueWorkersFromEnv(2); err != nil || got != 16 {
		t.Fatalf("default queue workers=(%d,%v), want (16,nil)", got, err)
	}
	if got, err := queueWorkersFromEnv(50); err != nil || got != 200 {
		t.Fatalf("scaled queue workers=(%d,%v), want (200,nil)", got, err)
	}
	t.Setenv("FLOW_QUEUE_WORKERS", "7")
	if got, err := queueWorkersFromEnv(2); err != nil || got != 7 {
		t.Fatalf("configured queue workers=(%d,%v), want (7,nil)", got, err)
	}
	for _, value := range []string{"0", "257", "many"} {
		t.Setenv("FLOW_QUEUE_WORKERS", value)
		if _, err := queueWorkersFromEnv(2); err == nil {
			t.Errorf("queue workers value %q should be rejected", value)
		}
	}
}

func TestServiceDrainSecondsConfiguration(t *testing.T) {
	t.Setenv("FLOW_SERVICE_DRAIN_SECONDS", "")
	if got, err := serviceDrainSecondsFromEnv(); err != nil || got != 30 {
		t.Fatalf("default drain timeout=(%d,%v), want (30,nil)", got, err)
	}
	for _, value := range []string{"1", "90", "3600"} {
		t.Setenv("FLOW_SERVICE_DRAIN_SECONDS", value)
		got, err := serviceDrainSecondsFromEnv()
		if err != nil || got < 1 {
			t.Errorf("configured drain timeout %q=(%d,%v), want positive timeout", value, got, err)
		}
	}
	for _, value := range []string{"0", "-1", "3601", "many"} {
		t.Setenv("FLOW_SERVICE_DRAIN_SECONDS", value)
		if _, err := serviceDrainSecondsFromEnv(); err == nil {
			t.Errorf("drain timeout value %q should be rejected", value)
		}
	}
}

func TestMediaConcurrencyConfiguration(t *testing.T) {
	t.Setenv("FLOW_MEDIA_POLL_CONCURRENCY", "")
	if got, err := mediaConcurrencyFromEnv("FLOW_MEDIA_POLL_CONCURRENCY", 6); err != nil || got != 6 {
		t.Fatalf("default media limit=(%d,%v), want (6,nil)", got, err)
	}
	if got, err := mediaConcurrencyFromEnv("FLOW_MEDIA_POLL_CONCURRENCY", 512); err != nil || got != 256 {
		t.Fatalf("clamped default media limit=(%d,%v), want (256,nil)", got, err)
	}
	t.Setenv("FLOW_MEDIA_POLL_CONCURRENCY", "3")
	if got, err := mediaConcurrencyFromEnv("FLOW_MEDIA_POLL_CONCURRENCY", 6); err != nil || got != 3 {
		t.Fatalf("configured media limit=(%d,%v), want (3,nil)", got, err)
	}
	for _, value := range []string{"0", "257", "many"} {
		t.Setenv("FLOW_MEDIA_POLL_CONCURRENCY", value)
		if _, err := mediaConcurrencyFromEnv("FLOW_MEDIA_POLL_CONCURRENCY", 6); err == nil {
			t.Errorf("media concurrency %q should be rejected", value)
		}
	}
}

func TestLivenessAndReadinessAreSeparate(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "health.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	manager, err := service.NewAccountManager(st, bridge.NewBridge(nil, nil, t.TempDir()), 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		readiness bool
		wantCode  int
		wantBody  string
	}{{"liveness", false, http.StatusOK, `"status":"ok"`}, {"readiness", true, http.StatusServiceUnavailable, `"status":"not_ready"`}} {
		t.Run(tc.name, func(t *testing.T) {
			res := httptest.NewRecorder()
			serviceHealthHandler(manager, tc.readiness).ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
			if res.Code != tc.wantCode || !strings.Contains(res.Body.String(), tc.wantBody) {
				t.Fatalf("health response=(%d,%s), want code %d and body containing %s", res.Code, res.Body.String(), tc.wantCode, tc.wantBody)
			}
		})
	}
}
