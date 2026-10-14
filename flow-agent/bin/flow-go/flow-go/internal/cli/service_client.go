package cli

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

const defaultServiceURL = "http://127.0.0.1:9223"

type serviceJobEnvelope struct {
	JobID  string          `json:"job_id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

func configuredServiceURL() string {
	return strings.TrimRight(strings.TrimSpace(os.Getenv("FLOW_SERVICE_URL")), "/")
}

func serviceAccountID(cookiePath string) (string, error) {
	if strings.TrimSpace(cookiePath) == "" {
		return "", nil
	}
	name := filepath.Base(cookiePath)
	if !strings.HasPrefix(name, "account_") || !strings.HasSuffix(name, ".json") {
		return "", fmt.Errorf("--cookies must name a registered account_<key>.json bundle when using the service")
	}
	id := strings.TrimSuffix(strings.TrimPrefix(name, "account_"), ".json")
	if id == "" {
		return "", fmt.Errorf("--cookies bundle has an empty account id")
	}
	return id, nil
}

// runServiceJob submits exactly once, then polls the durable job. On a lost
// acknowledgement it looks up the same idempotency key instead of resubmitting.
func runServiceJob(kind string, payload any, idempotencyKey string) int {
	baseURL := configuredServiceURL()
	if baseURL == "" {
		return -1
	}
	token := strings.TrimSpace(os.Getenv("FLOW_INTERNAL_TOKEN"))
	if token == "" {
		return fail(fmt.Errorf("FLOW_INTERNAL_TOKEN is required when FLOW_SERVICE_URL is configured"))
	}
	ctx, stop := signalContext()
	defer stop()
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		var err error
		key, err = randomIdempotencyKey()
		if err != nil {
			return fail(err)
		}
	}
	body, err := json.Marshal(map[string]any{
		"kind": kind, "payload": payload, "payload_version": store.CurrentJobPayloadVersion,
	})
	if err != nil {
		return fail(err)
	}
	client := &http.Client{Timeout: 35 * time.Second}
	statusNeedsFetch := true
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/internal/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return fail(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", key)
	response, postErr := client.Do(request)
	var job serviceJobEnvelope
	if postErr == nil {
		if response.StatusCode >= 400 && response.StatusCode < 500 {
			_, err = decodeServiceJob(response)
			return fail(err)
		}
		job, err = decodeServiceJob(response)
	}
	if postErr != nil || err != nil {
		// The server may have committed the job before the connection failed.
		// Resolve by key; a second POST could submit a duplicate generation.
		job, err = lookupServiceJob(ctx, client, baseURL, token, key)
		if err != nil {
			if postErr != nil {
				return fail(fmt.Errorf("service submit acknowledgement is unresolved (key %s); rerun with --idempotency-key %s to resolve safely: %w", key, key, postErr))
			}
			return fail(fmt.Errorf("service submit acknowledgement is unresolved (key %s); rerun with --idempotency-key %s to resolve safely: %w", key, key, err))
		}
		statusNeedsFetch = false
	}
	if job.JobID == "" {
		return fail(fmt.Errorf("service returned an empty job_id"))
	}
	fmt.Fprintf(os.Stderr, "queued job %s\n", job.JobID)
	for {
		if !statusNeedsFetch && terminalServiceStatus(job.Status) {
			if job.Status != "succeeded" && job.Status != "partial" {
				message := job.Error
				if message == "" {
					message = job.Status
				}
				return fail(fmt.Errorf("job %s %s: %s", job.JobID, job.Status, message))
			}
			if len(job.Result) == 0 || string(job.Result) == "null" {
				fmt.Println("{}")
			} else {
				fmt.Println(string(job.Result))
			}
			return 0
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			statusURL := baseURL + "/internal/v1/jobs/" + url.PathEscape(job.JobID)
			return fail(fmt.Errorf("job %s is still accepted; query %s: %w", job.JobID, statusURL, ctx.Err()))
		case <-timer.C:
		}
		job, err = getServiceJob(ctx, client, baseURL, token, job.JobID)
		if err != nil {
			// Polling is read-only, so transient errors can be retried safely.
			fmt.Fprintf(os.Stderr, "service status read failed; retrying: %v\n", err)
			continue
		}
		statusNeedsFetch = false
	}
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func randomIdempotencyKey() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("create idempotency key: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

func decodeServiceJob(response *http.Response) (serviceJobEnvelope, error) {
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return serviceJobEnvelope{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return serviceJobEnvelope{}, fmt.Errorf("service HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	var job serviceJobEnvelope
	if err := json.Unmarshal(data, &job); err != nil {
		return serviceJobEnvelope{}, fmt.Errorf("decode service job response: %w", err)
	}
	return job, nil
}

func lookupServiceJob(ctx context.Context, client *http.Client, baseURL, token, key string) (serviceJobEnvelope, error) {
	endpoint := baseURL + "/internal/v1/jobs/by-key?key=" + url.QueryEscape(key)
	return getServiceEnvelope(ctx, client, endpoint, token)
}

func getServiceJob(ctx context.Context, client *http.Client, baseURL, token, id string) (serviceJobEnvelope, error) {
	return getServiceEnvelope(ctx, client, baseURL+"/internal/v1/jobs/"+url.PathEscape(id), token)
}

func getServiceEnvelope(ctx context.Context, client *http.Client, endpoint, token string) (serviceJobEnvelope, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return serviceJobEnvelope{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		return serviceJobEnvelope{}, err
	}
	return decodeServiceJob(response)
}

func terminalServiceStatus(status string) bool {
	switch status {
	case "succeeded", "partial", "failed", "cancelled", "submission_unknown":
		return true
	default:
		return false
	}
}
