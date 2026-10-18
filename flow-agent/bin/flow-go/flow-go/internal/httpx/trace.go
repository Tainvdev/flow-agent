package httpx

import (
	"encoding/json"
	"fmt"
	"io"
	stdhttp "net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
)

// HTTP tracing is opt-in because it records complete credentials, prompts,
// signed URLs, and upstream response bodies.
var traceAppendMu sync.Mutex
var traceSequence atomic.Uint64

type httpTraceRecord struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
	Duration  int64     `json:"duration_ms"`
	Method    string    `json:"method"`
	URL       string    `json:"url"`
	Request   tracePart `json:"request"`
	Response  tracePart `json:"response"`
	Error     string    `json:"error,omitempty"`
}

type tracePart struct {
	Headers   map[string][]string `json:"headers,omitempty"`
	BodyFile  string              `json:"body_file,omitempty"`
	BodyBytes int64               `json:"body_bytes"`
	Status    int                 `json:"status,omitempty"`
	Protocol  string              `json:"protocol,omitempty"`
}

type httpTrace struct {
	path       string
	bodyDir    string
	id         string
	started    time.Time
	method     string
	url        string
	request    tracePart
	requestOut *os.File
	requestErr error
	finished   bool
}

func newHTTPTrace(path string, req *Request) (*httpTrace, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	u, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("httpx trace: parse request URL: %w", err)
	}
	path = filepath.Clean(path)
	bodyDir := path + ".bodies"
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("httpx trace: create log directory: %w", err)
	}
	if err := os.MkdirAll(bodyDir, 0o700); err != nil {
		return nil, fmt.Errorf("httpx trace: create body directory: %w", err)
	}
	id := fmt.Sprintf("%d-%06d", time.Now().UnixNano(), traceSequence.Add(1))
	bodyPath := filepath.Join(bodyDir, id+"-request.bin")
	out, err := os.OpenFile(bodyPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("httpx trace: create request body file: %w", err)
	}
	return &httpTrace{
		path: path, bodyDir: bodyDir, id: id, started: time.Now().UTC(),
		method: req.Method, url: u.String(),
		request: tracePart{BodyFile: bodyPath}, requestOut: out,
	}, nil
}

func (t *httpTrace) wrapRequestBody(r io.Reader) io.Reader {
	if t == nil || t.requestOut == nil {
		return r
	}
	return io.TeeReader(r, bestEffortTraceWriter{w: t.requestOut, onError: func(err error) { t.requestErr = err }})
}

func (t *httpTrace) setRequest(req *fhttp.Request) {
	if t != nil {
		t.request.Headers = cloneTraceHeaders(map[string][]string(req.Header))
	}
}

func (t *httpTrace) setStdRequest(req *stdhttp.Request) {
	if t != nil {
		t.request.Headers = cloneTraceHeaders(map[string][]string(req.Header))
	}
}

func (t *httpTrace) finish(resp *fhttp.Response, body []byte, err error) error {
	if resp == nil {
		return t.finishRecord(0, "", nil, body, err)
	}
	return t.finishRecord(resp.StatusCode, resp.Proto, map[string][]string(resp.Header), body, err)
}

func (t *httpTrace) finishStd(resp *stdhttp.Response, body []byte, err error) error {
	if resp == nil {
		return t.finishRecord(0, "", nil, body, err)
	}
	return t.finishRecord(resp.StatusCode, resp.Proto, map[string][]string(resp.Header), body, err)
}

func (t *httpTrace) finishRecord(status int, protocol string, headers map[string][]string, body []byte, requestErr error) error {
	if t == nil || t.finished {
		return nil
	}
	t.finished = true
	if t.requestOut != nil {
		if err := t.requestOut.Close(); err != nil && t.requestErr == nil {
			t.requestErr = err
		}
	}
	if info, err := os.Stat(t.request.BodyFile); err == nil {
		t.request.BodyBytes = info.Size()
	}
	respPath := filepath.Join(t.bodyDir, t.id+"-response.bin")
	if body != nil {
		if err := os.WriteFile(respPath, body, 0o600); err != nil && requestErr == nil {
			requestErr = fmt.Errorf("write response sidecar: %w", err)
		}
	}
	record := httpTraceRecord{
		ID: t.id, StartedAt: t.started, Duration: time.Since(t.started).Milliseconds(),
		Method: t.method, URL: t.url, Request: t.request,
		Response: tracePart{Headers: cloneTraceHeaders(headers), BodyFile: respPath, BodyBytes: int64(len(body)), Status: status, Protocol: protocol},
	}
	if t.requestErr != nil {
		record.Error = t.requestErr.Error()
	}
	if requestErr != nil {
		if record.Error != "" {
			record.Error += "; "
		}
		record.Error += requestErr.Error()
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	traceAppendMu.Lock()
	defer traceAppendMu.Unlock()
	f, err := os.OpenFile(t.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func cloneTraceHeaders(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, values := range in {
		out[k] = append([]string(nil), values...)
	}
	return out
}

type bestEffortTraceWriter struct {
	w       io.Writer
	onError func(error)
}

func (w bestEffortTraceWriter) Write(p []byte) (int, error) {
	_, err := w.w.Write(p)
	if err != nil && w.onError != nil {
		w.onError(err)
	}
	// A diagnostic disk error must not change whether the upstream request is sent.
	return len(p), nil
}
