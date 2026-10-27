package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// NodeReporter advertises one Go process to the central Gateway. It is
// outbound-only: the Gateway never accesses this node's SQLite or cookies.
type NodeReporter struct {
	manager      *AccountManager
	gatewayURL   string
	nodeID       string
	advertiseURL string
	token        string
	version      string
	maxJobs      int
	client       *http.Client
}

func NewNodeReporter(manager *AccountManager, workers int) *NodeReporter {
	gatewayURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FLOW_GATEWAY_URL")), "/")
	if gatewayURL == "" {
		return nil
	}
	nodeID := strings.TrimSpace(os.Getenv("FLOW_NODE_ID"))
	if nodeID == "" {
		nodeID, _ = os.Hostname()
	}
	advertiseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FLOW_NODE_ADVERTISE_URL")), "/")
	if advertiseURL == "" {
		advertiseURL = "http://127.0.0.1:9223"
	}
	return &NodeReporter{
		manager: manager, gatewayURL: gatewayURL, nodeID: nodeID,
		advertiseURL: advertiseURL, token: os.Getenv("FLOW_GATEWAY_SECRET"),
		version: strings.TrimSpace(os.Getenv("FLOW_NODE_VERSION")),
		maxJobs: max(1, workers), client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *NodeReporter) Start(ctx context.Context) {
	if r == nil || r.manager == nil || r.nodeID == "" {
		return
	}
	interval := 15 * time.Second
	if raw := strings.TrimSpace(os.Getenv("FLOW_NODE_HEARTBEAT_INTERVAL")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed >= time.Second {
			interval = parsed
		}
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	registered := false
	for {
		method, path := http.MethodPut, "/internal/router/v1/nodes/"+r.nodeID+"/heartbeat"
		if !registered {
			method, path = http.MethodPost, "/internal/router/v1/nodes/register"
		}
		if err := r.send(ctx, method, path); err != nil && ctx.Err() == nil {
			log.Printf("service: register node %s: %v", r.nodeID, err)
		} else if err == nil {
			registered = true
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *NodeReporter) send(ctx context.Context, method, path string) error {
	active, queued := r.manager.ClusterLoad()
	payload := map[string]any{
		"node_id": r.nodeID, "advertise_url": r.advertiseURL, "version": r.version,
		"max_jobs": r.maxJobs, "active_jobs": active, "queued_jobs": queued,
		"capabilities": map[string]any{"image": true, "video": true, "upload-video": true},
		"accounts":     r.manager.ClusterAccounts(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, method, r.gatewayURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(r.token) != "" {
		request.Header.Set("Authorization", "Bearer "+r.token)
	}
	response, err := r.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("gateway returned %s: %s", response.Status, strings.TrimSpace(string(data)))
	}
	return nil
}
