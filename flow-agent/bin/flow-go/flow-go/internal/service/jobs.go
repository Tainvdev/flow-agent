// Package service exposes durable generation jobs to local callers.
package service

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

const (
	jobLease              = 2 * time.Minute
	leaseHeartbeat        = 30 * time.Second
	maxPayloadSize        = 2 << 20
	defaultJobMaxAttempts = 3
	maxJobMaxAttempts     = 20
)

var queuePoll = 250 * time.Millisecond

// Runner executes a claimed job. The production implementation is the
// account-aware scheduler; keeping it behind this interface makes job storage
// and HTTP behavior testable without contacting Google Flow.
type Runner interface {
	Run(context.Context, store.Job) (any, error)
}

// RetrySafety lets a runner say the upstream definitely did not accept this
// attempt. Errors without this guarantee become terminal to prevent duplicate
// generations after ambiguous network failures.
type RetrySafety interface {
	SafeToRetry() bool
}

// RetryClassifier gives safe failures a named budget. A retry class is only
// consulted after RetrySafety has proved that replay cannot duplicate work.
type RetryClassifier interface {
	RetryClass() string
}

const (
	retryClassDefault   = "default"
	retryClassPreSubmit = "pre_submit_deadline"
)

// SubmissionAmbiguity marks an error where the request may have been accepted
// upstream, so the job must not be sent again automatically.
type SubmissionAmbiguity interface {
	SubmissionUnknown() bool
}

func jobErrorHTTPStatus(err error) int {
	var unavailable *pool.NoWorkerError
	if errors.As(err, &unavailable) {
		return http.StatusServiceUnavailable
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

// Jobs owns job HTTP endpoints and durable queue workers.
type Jobs struct {
	store        *store.Store
	runner       Runner
	token        string
	workers      int
	maxAttempts  int
	retryBudgets map[string]int
	owner        string

	startOnce   sync.Once
	wg          sync.WaitGroup
	claimMu     sync.Mutex
	stopOnce    sync.Once
	stopClaim   chan struct{}
	inflight    sync.WaitGroup
	activeMu    sync.Mutex
	active      map[string]context.CancelFunc
	wakeWorkers chan struct{}
}

func NewJobs(st *store.Store, runner Runner, token string, workers int) (*Jobs, error) {
	if st == nil || runner == nil {
		return nil, fmt.Errorf("service: store and runner are required")
	}
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("service: internal API token is required")
	}
	if workers < 1 {
		workers = 1
	}
	return &Jobs{
		store: st, runner: runner, token: token, workers: workers,
		maxAttempts: defaultJobMaxAttempts, retryBudgets: make(map[string]int),
		owner: uuid.NewString(), stopClaim: make(chan struct{}), active: make(map[string]context.CancelFunc),
		wakeWorkers: make(chan struct{}, workers),
	}, nil
}

// signalWorkers wakes idle workers when durable work becomes runnable. The
// ticker remains as a recovery backstop for already-persisted jobs.
func (j *Jobs) signalWorkers(count int) {
	for i := 0; i < count; i++ {
		select {
		case j.wakeWorkers <- struct{}{}:
		default:
			return
		}
	}
}

// SetMaxAttempts configures the total number of executions allowed for a job.
// It must be called before Start. Automatic replay still requires the runner to
// prove the prior attempt was safe to retry; this setting never retries an
// ambiguous upstream submission.
func (j *Jobs) SetMaxAttempts(total int) error {
	if total < 1 || total > maxJobMaxAttempts {
		return fmt.Errorf("service: max job attempts must be between 1 and %d", maxJobMaxAttempts)
	}
	j.maxAttempts = total
	return nil
}

// SetRetryBudget configures a total execution limit for one safe retry class.
// Unconfigured classes inherit SetMaxAttempts, preserving the global fallback.
func (j *Jobs) SetRetryBudget(class string, total int) error {
	class = strings.TrimSpace(class)
	if class == "" || class == retryClassDefault {
		return fmt.Errorf("service: retry class must be a non-default name")
	}
	if total < 1 || total > maxJobMaxAttempts {
		return fmt.Errorf("service: retry budget must be between 1 and %d", maxJobMaxAttempts)
	}
	j.retryBudgets[class] = total
	return nil
}

// Start launches workers. It is safe to call once and returns when ctx is
// cancelled and every active runner has had a chance to stop.
func (j *Jobs) Start(ctx context.Context) {
	j.startOnce.Do(func() {
		for i := 0; i < j.workers; i++ {
			j.wg.Add(1)
			go j.worker(ctx, fmt.Sprintf("%s-%d", j.owner, i))
		}
	})
	<-ctx.Done()
	j.wg.Wait()
}

// StopClaiming prevents workers from leasing additional queued jobs. A claim
// already in progress is registered as inflight before this method returns.
func (j *Jobs) StopClaiming() {
	j.stopOnce.Do(func() {
		j.claimMu.Lock()
		close(j.stopClaim)
		j.claimMu.Unlock()
	})
}

// Drain waits for every job leased before StopClaiming to finish, or returns
// when the caller's shutdown deadline expires. Call StopClaiming first.
func (j *Jobs) Drain(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		j.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (j *Jobs) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !authorized(r, j.token) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/internal/v1/jobs":
		started := time.Now()
		defer func() {
			log.Printf("service_metric phase=job_accept_response elapsed_ms=%d", time.Since(started).Milliseconds())
		}()
		j.create(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/internal/v1/jobs/by-key":
		j.getByIdempotencyKey(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/v1/jobs/") && strings.HasSuffix(r.URL.Path, "/events"):
		j.events(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/internal/v1/jobs/") && strings.HasSuffix(r.URL.Path, "/cancel"):
		j.cancel(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/internal/v1/jobs/"):
		j.get(w, r)
	default:
		http.NotFound(w, r)
	}
}

// RequireToken protects auxiliary internal service endpoints with the same
// bearer token as the durable jobs API.
func RequireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !authorized(r, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func authorized(r *http.Request, token string) bool {
	const prefix = "Bearer "
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, prefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, prefix)), []byte(token)) == 1
}

type createJobRequest struct {
	Kind           string          `json:"kind"`
	Payload        json.RawMessage `json:"payload"`
	PayloadVersion int             `json:"payload_version,omitempty"`
}

func (j *Jobs) create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPayloadSize)
	defer r.Body.Close()
	var req createJobRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return
	}
	if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
		http.Error(w, "request must contain one JSON value", http.StatusBadRequest)
		return
	}
	if req.Kind != "image" && req.Kind != "video" && req.Kind != "upload-video" {
		http.Error(w, "kind must be image, video or upload-video", http.StatusBadRequest)
		return
	}
	if req.PayloadVersion == 0 {
		req.PayloadVersion = store.CurrentJobPayloadVersion
	}
	if req.PayloadVersion != store.CurrentJobPayloadVersion {
		http.Error(w, fmt.Sprintf("unsupported payload_version %d", req.PayloadVersion), http.StatusBadRequest)
		return
	}
	if len(req.Payload) == 0 || !json.Valid(req.Payload) {
		http.Error(w, "payload must be a JSON value", http.StatusBadRequest)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key != "" {
		if existing, lookupErr := j.store.JobByIdempotencyKey(r.Context(), key); lookupErr == nil {
			if existing.Kind != req.Kind || existing.Payload != string(req.Payload) || existing.PayloadVersion != req.PayloadVersion {
				http.Error(w, "idempotency key was already used with a different request", http.StatusConflict)
				return
			}
			writeAcceptedJob(w, existing, false)
			return
		} else if !errors.Is(lookupErr, sql.ErrNoRows) {
			http.Error(w, "could not check idempotency key", http.StatusInternalServerError)
			return
		}
	}
	if validator, ok := j.runner.(JobPayloadValidator); ok {
		candidate := store.Job{Kind: req.Kind, Payload: string(req.Payload), PayloadVersion: req.PayloadVersion}
		if err := validator.ValidateJobPayload(r.Context(), candidate); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
	}
	newJob := store.Job{
		ID: uuid.NewString(), Kind: req.Kind, Payload: string(req.Payload), PayloadVersion: req.PayloadVersion, IdempotencyKey: key,
	}
	var job *store.Job
	created := false
	var err error
	if expander, ok := j.runner.(ParentExpander); ok {
		children, expand, expandErr := expander.ExpandForQueue(r.Context(), newJob)
		if expandErr != nil {
			code := http.StatusServiceUnavailable
			if strings.Contains(expandErr.Error(), "count must be between") {
				code = http.StatusBadRequest
			}
			http.Error(w, expandErr.Error(), code)
			return
		}
		if expand {
			job, created, err = j.store.CreateExpandedJob(r.Context(), newJob, children)
		} else {
			job, created, err = j.store.CreateJob(r.Context(), newJob)
		}
	} else {
		job, created, err = j.store.CreateJob(r.Context(), newJob)
	}
	if err != nil {
		if errors.Is(err, store.ErrQueueFull) {
			http.Error(w, "job queue is full", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "could not persist job", http.StatusInternalServerError)
		return
	}
	if !created && (job.Kind != req.Kind || job.Payload != string(req.Payload) || job.PayloadVersion != req.PayloadVersion) {
		http.Error(w, "idempotency key was already used with a different request", http.StatusConflict)
		return
	}
	if created {
		j.signalWorkers(j.workers)
	}
	writeAcceptedJob(w, job, created)
}

func writeAcceptedJob(w http.ResponseWriter, job *store.Job, created bool) {
	statusURL := "/internal/v1/jobs/" + job.ID
	eventsURL := statusURL + "/events"
	w.Header().Set("Location", statusURL)
	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusAccepted)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"job_id": job.ID, "status": job.Status, "status_url": statusURL, "events_url": eventsURL,
	})
}

func (j *Jobs) cancel(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/internal/v1/jobs/"), "/cancel")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	job, err := j.store.JobByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not read job", http.StatusInternalServerError)
		return
	}
	if job.Status == "queued" {
		if err := j.store.CancelQueuedJob(r.Context(), id); err == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		job, err = j.store.JobByID(r.Context(), id)
		if err != nil {
			http.Error(w, "could not read job", http.StatusInternalServerError)
			return
		}
	}
	if job.Status == "waiting_children" {
		if err := j.store.CancelParentAndQueuedChildren(r.Context(), id); err != nil {
			http.Error(w, "could not cancel parent job", http.StatusConflict)
			return
		}
		children, err := j.store.ChildJobs(r.Context(), id)
		if err != nil {
			http.Error(w, "could not read child jobs", http.StatusInternalServerError)
			return
		}
		running := false
		j.activeMu.Lock()
		for _, child := range children {
			if child.Status != "running" {
				continue
			}
			if cancel := j.active[child.ID]; cancel != nil {
				cancel()
				running = true
			}
		}
		j.activeMu.Unlock()
		if running {
			w.WriteHeader(http.StatusAccepted)
		} else {
			w.WriteHeader(http.StatusNoContent)
		}
		return
	}
	if job.Status == "running" {
		j.activeMu.Lock()
		cancel := j.active[id]
		j.activeMu.Unlock()
		if cancel != nil {
			cancel()
			w.WriteHeader(http.StatusAccepted)
			return
		}
	}
	if job.Status == "cancelled" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Error(w, "job is already terminal", http.StatusConflict)
}

func (j *Jobs) get(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/internal/v1/jobs/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	job, err := j.store.JobByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "could not read job", http.StatusInternalServerError)
		return
	}
	j.writeJobStatus(w, r, job)
}

func (j *Jobs) events(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/internal/v1/jobs/"), "/events")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	job, err := j.store.JobByID(r.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not read job", http.StatusInternalServerError)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	afterID := int64(0)
	if raw := strings.TrimSpace(r.Header.Get("Last-Event-ID")); raw != "" {
		afterID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || afterID < 0 {
			http.Error(w, "Last-Event-ID must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "retry: 1000\n\n")
	flusher.Flush()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	lastKeepalive := time.Now()
	for {
		events, err := j.store.JobEventsAfter(r.Context(), id, afterID, 128)
		if err != nil {
			return
		}
		if !writeJobEvents(w, flusher, events, &afterID) {
			return
		}
		if len(events) == 128 {
			continue
		}
		job, err = j.store.JobByID(r.Context(), id)
		if err != nil {
			return
		}
		if terminalJobStatus(job.Status) {
			// A terminal status and its trigger event commit together. Re-read
			// after observing terminal state so a concurrent transition cannot
			// fall between the first event query and this state read.
			events, err = j.store.JobEventsAfter(r.Context(), id, afterID, 128)
			if err != nil || !writeJobEvents(w, flusher, events, &afterID) {
				return
			}
			if len(events) == 128 {
				continue
			}
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if time.Since(lastKeepalive) >= 15*time.Second {
				if _, err := io.WriteString(w, ": keep-alive\n\n"); err != nil {
					return
				}
				flusher.Flush()
				lastKeepalive = time.Now()
			}
		}
	}
}

func writeJobEvents(w http.ResponseWriter, flusher http.Flusher, events []store.JobEvent, cursor *int64) bool {
	for _, event := range events {
		if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Data); err != nil {
			return false
		}
		*cursor = event.ID
		flusher.Flush()
	}
	return true
}

func terminalJobStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled", "submission_unknown", "partial":
		return true
	default:
		return false
	}
}

func (j *Jobs) getByIdempotencyKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		http.Error(w, "key query parameter is required", http.StatusBadRequest)
		return
	}
	job, err := j.store.JobByIdempotencyKey(r.Context(), key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "could not read job", http.StatusInternalServerError)
		return
	}
	j.writeJobStatus(w, r, job)
}

func (j *Jobs) writeJobStatus(w http.ResponseWriter, r *http.Request, job *store.Job) {
	id := job.ID
	view := map[string]any{
		"job_id": job.ID, "kind": job.Kind, "payload_version": job.PayloadVersion, "status": job.Status,
		"attempts": job.Attempts, "created_at": job.CreatedAt, "updated_at": job.UpdatedAt,
		"events_url": "/internal/v1/jobs/" + job.ID + "/events",
	}
	attempts, err := j.store.JobAttempts(r.Context(), id)
	if err != nil {
		http.Error(w, "could not read job attempts", http.StatusInternalServerError)
		return
	}
	view["attempt_history"] = attempts
	if job.Result != "" {
		var result json.RawMessage
		if json.Unmarshal([]byte(job.Result), &result) == nil {
			view["result"] = result
		} else {
			view["result"] = job.Result
		}
	}
	if job.Error != "" {
		view["error"] = job.Error
	}
	if job.ErrorCode != 0 {
		view["error_code"] = job.ErrorCode
	}
	if job.FinishedAt != nil {
		view["finished_at"] = job.FinishedAt
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(view)
}

func (j *Jobs) worker(ctx context.Context, owner string) {
	defer j.wg.Done()
	ticker := time.NewTicker(queuePoll)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		select {
		case <-j.stopClaim:
			return
		default:
		}
		j.finalizeReadyParents(ctx)
		j.claimMu.Lock()
		select {
		case <-j.stopClaim:
			j.claimMu.Unlock()
			return
		default:
		}
		job, err := j.store.ClaimNextJob(ctx, owner, jobLease)
		if err == nil && job != nil {
			j.inflight.Add(1)
		}
		j.claimMu.Unlock()
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("service: claim job: %v", err)
			}
		} else if job != nil {
			j.execute(ctx, owner, job)
			j.inflight.Done()
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-j.stopClaim:
			return
		case <-j.wakeWorkers:
		case <-ticker.C:
		}
	}
}

func (j *Jobs) execute(parent context.Context, owner string, job *store.Job) {
	if expander, ok := j.runner.(ParentExpander); ok {
		children, expand, err := expander.ExpandForQueue(parent, *job)
		if expand {
			if err != nil {
				if finishErr := j.store.FinishJob(context.Background(), job.ID, owner, "failed", "", err.Error()); finishErr != nil {
					log.Printf("service: reject parent job %s: %v (finish: %v)", job.ID, err, finishErr)
				}
				return
			}
			if err := j.store.ExpandParentJob(context.Background(), job.ID, owner, children); err != nil {
				if finishErr := j.store.FinishJob(context.Background(), job.ID, owner, "failed", "", err.Error()); finishErr != nil {
					log.Printf("service: expand parent job %s: %v (finish: %v)", job.ID, err, finishErr)
				}
			} else {
				j.signalWorkers(len(children))
			}
			return
		}
	}
	attemptNo := job.Attempts
	if job.ResumeAttempt != nil {
		attemptNo = job.ResumeAttempt.AttemptNo
	} else if err := j.store.StartJobAttempt(context.Background(), job.ID, attemptNo); err != nil {
		if finishErr := j.store.FinishJob(context.Background(), job.ID, owner, "failed", "", err.Error()); finishErr != nil {
			log.Printf("service: create attempt for job %s failed: %v (finish: %v)", job.ID, err, finishErr)
		}
		return
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	j.activeMu.Lock()
	j.active[job.ID] = cancel
	j.activeMu.Unlock()
	defer func() { j.activeMu.Lock(); delete(j.active, job.ID); j.activeMu.Unlock() }()
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		ticker := time.NewTicker(leaseHeartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := j.store.RenewJobLease(ctx, job.ID, owner, jobLease); err != nil {
					log.Printf("service: renew lease for job %s: %v", job.ID, err)
					cancel()
					return
				}
			}
		}
	}()

	progress := &jobAttemptProgress{store: j.store, jobID: job.ID, attemptNo: attemptNo}
	var result any
	var runErr error
	if job.ResumeAttempt != nil {
		if runner, ok := j.runner.(ResumeRunner); ok {
			result, runErr = runner.ResumeWithProgress(ctx, *job, *job.ResumeAttempt, progress)
		} else {
			runErr = fmt.Errorf("service: runner cannot resume accepted upstream media")
			_ = progress.Fail("submission_unknown", runErr.Error())
		}
	} else if runner, ok := j.runner.(ProgressRunner); ok {
		result, runErr = runner.RunWithProgress(ctx, *job, progress)
	} else {
		if err := progress.SetPhase("executing", ""); err != nil {
			runErr = err
		} else {
			result, runErr = j.runner.Run(ctx, *job)
		}
	}
	runCancelled := ctx.Err() != nil && parent.Err() == nil
	cancel()
	<-heartbeatDone
	if runCancelled {
		_ = progress.Fail("cancelled", "cancelled by caller")
		if err := j.store.SetJobCancelled(context.Background(), job.ID, owner); err != nil {
			log.Printf("service: cancel job %s: %v", job.ID, err)
		}
		return
	}
	if parent.Err() != nil {
		if err := j.store.RequeueAcceptedJobForResume(context.Background(), job.ID, owner); err == nil {
			return
		} else if !errors.Is(err, store.ErrNotResumable) {
			log.Printf("service: requeue accepted job %s for resume during shutdown: %v", job.ID, err)
		}
		if job.ResumeAttempt != nil {
			if err := j.store.RequeueResumableJob(context.Background(), job.ID, owner); err != nil {
				log.Printf("service: preserve resumable job %s during shutdown: %v", job.ID, err)
			}
			return
		}
		if err := j.store.RequeuePreSubmitJob(context.Background(), job.ID, owner); err == nil {
			return
		} else if !errors.Is(err, store.ErrNotPreSubmit) {
			log.Printf("service: requeue pre-submit job %s during shutdown: %v", job.ID, err)
		}
	}
	if runErr == nil {
		data, err := json.Marshal(result)
		if err != nil {
			runErr = fmt.Errorf("service: encode job result: %w", err)
		} else {
			status := "succeeded"
			var resultState struct {
				Status string `json:"status"`
			}
			if json.Unmarshal(data, &resultState) == nil && resultState.Status == "partial" {
				status = "partial"
			}
			if err := progress.Complete(result); err != nil {
				runErr = fmt.Errorf("service: persist completed attempt: %w", err)
			} else if err := j.store.FinishJob(context.Background(), job.ID, owner, status, string(data), ""); err != nil {
				log.Printf("service: finish job %s: %v", job.ID, err)
				return
			} else {
				return
			}
		}
	}
	// Once accepted media ids are durable, every later failure belongs to the
	// resume path. Requeueing the original runner here could submit a duplicate
	// generation even when the error itself is otherwise retry-safe.
	if runErr != nil && job.ResumeAttempt == nil && parent.Err() == nil {
		resumeBudget := j.retryBudgets["resume_media"]
		if resumeBudget == 0 {
			resumeBudget = j.maxAttempts
		}
		if job.Attempts < resumeBudget {
			if err := j.store.RequeueAcceptedJobForResume(context.Background(), job.ID, owner); err == nil {
				j.signalWorkers(1)
				return
			} else if !errors.Is(err, store.ErrNotResumable) {
				log.Printf("service: preserve accepted media for job %s: %v", job.ID, err)
			}
		}
	}
	var safe RetrySafety
	safeRetry := errors.As(runErr, &safe) && safe.SafeToRetry()
	retryClass := retryClassDefault
	var classifier RetryClassifier
	if safeRetry && errors.As(runErr, &classifier) && strings.TrimSpace(classifier.RetryClass()) != "" {
		retryClass = strings.TrimSpace(classifier.RetryClass())
	}
	if !safeRetry && job.ResumeAttempt == nil && errors.Is(runErr, context.DeadlineExceeded) && parent.Err() == nil {
		phase, phaseErr := j.store.JobAttemptPhase(context.Background(), job.ID, attemptNo)
		// A deadline is replay-safe only while the durable phase proves that
		// the runner never crossed its generation submit boundary.
		safeRetry = phaseErr == nil && phase == "preparing"
		if safeRetry {
			retryClass = retryClassPreSubmit
		}
	}
	budget := j.retryBudgets[retryClass]
	if budget == 0 {
		budget = j.maxAttempts
	}
	if job.ResumeAttempt != nil && safeRetry && parent.Err() == nil && job.Attempts < budget {
		if err := j.store.RequeueResumableJob(context.Background(), job.ID, owner); err != nil {
			log.Printf("service: requeue media resume for job %s: %v", job.ID, err)
		} else {
			j.signalWorkers(1)
		}
		return
	}
	if safeRetry && parent.Err() == nil && job.Attempts < budget {
		_ = progress.Fail("retryable_failed", runErr.Error())
		if err := j.store.RequeueJob(context.Background(), job.ID, owner, runErr.Error()); err != nil {
			log.Printf("service: requeue job %s: %v", job.ID, err)
		} else {
			j.signalWorkers(1)
		}
		return
	}
	status := "failed"
	var ambiguous SubmissionAmbiguity
	if errors.As(runErr, &ambiguous) && ambiguous.SubmissionUnknown() {
		status = "submission_unknown"
	}
	phase := "failed"
	if status == "submission_unknown" {
		phase = "submission_unknown"
	}
	_ = progress.Fail(phase, runErr.Error())
	if parent.Err() != nil && status == "failed" {
		status = "submission_unknown"
	}
	if err := j.store.FinishJobWithErrorCode(context.Background(), job.ID, owner, status, "", runErr.Error(), jobErrorHTTPStatus(runErr)); err != nil {
		log.Printf("service: finish failed job %s: %v", job.ID, err)
	}
}

func (j *Jobs) finalizeReadyParents(ctx context.Context) {
	parents, err := j.store.WaitingParents(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("service: list ready parent jobs: %v", err)
		}
		return
	}
	for _, parentID := range parents {
		children, err := j.store.ChildJobs(ctx, parentID)
		if err != nil {
			log.Printf("service: read children of %s: %v", parentID, err)
			continue
		}
		parent, err := j.store.JobByID(ctx, parentID)
		if err != nil {
			log.Printf("service: read parent %s: %v", parentID, err)
			continue
		}
		result, status, message := aggregateChildren(*parent, children)
		data, err := json.Marshal(result)
		if err != nil {
			log.Printf("service: encode parent result %s: %v", parentID, err)
			continue
		}
		if err := j.store.FinishParent(ctx, parentID, status, string(data), message); err != nil && !strings.Contains(err.Error(), "not waiting for children") {
			log.Printf("service: finish parent %s: %v", parentID, err)
		}
	}
}

func aggregateChildren(parent store.Job, children []store.Job) (any, string, string) {
	var request generationPayload
	_ = json.Unmarshal([]byte(parent.Payload), &request)
	if request.AllAccounts {
		return aggregateAccounts(parent, children)
	}
	results := make([]map[string]any, 0, len(children))
	errorsList := make([]string, 0)
	succeeded := 0
	hasPartialChild := false
	media, files, childIDs := []any{}, []any{}, []any{}
	var elapsed float64
	accountID := ""
	for _, child := range children {
		var payload generationPayload
		_ = json.Unmarshal([]byte(child.Payload), &payload)
		if accountID == "" {
			accountID = payload.AccountID
		}
		item := map[string]any{"account_id": payload.AccountID}
		var value any
		if child.Result != "" {
			_ = json.Unmarshal([]byte(child.Result), &value)
		}
		if child.Status == "succeeded" || child.Status == "partial" {
			succeeded++
			item["result"] = value
			if child.Status == "partial" {
				hasPartialChild = true
				item["error"] = "partial child result"
				errorsList = append(errorsList, payload.AccountID+": partial child result")
			} else if valueMap, ok := value.(map[string]any); ok && valueMap["status"] == "partial" {
				hasPartialChild = true
				item["error"] = "partial child result"
				errorsList = append(errorsList, payload.AccountID+": partial child result")
			}
		} else {
			message := child.Error
			if message == "" {
				message = "child job " + child.Status
			}
			item["error"] = message
			errorsList = append(errorsList, payload.AccountID+": "+message)
		}
		results = append(results, item)
		childIDs = append(childIDs, child.ID)
		if valueMap, ok := value.(map[string]any); ok {
			if values, ok := valueMap["media"].([]any); ok {
				media = append(media, values...)
			}
			if values, ok := valueMap["files"].([]any); ok {
				files = append(files, values...)
			}
			if seconds, ok := valueMap["elapsed_seconds"].(float64); ok {
				elapsed += seconds
			}
		}
	}
	status := "succeeded"
	if succeeded == 0 {
		status = "failed"
	} else if succeeded != len(children) || hasPartialChild {
		status = "partial"
	}
	message := strings.Join(errorsList, "; ")
	if request.Count > 1 && parent.Kind == "image" {
		return map[string]any{"job_id": parent.ID, "account_id": accountID, "media": media, "files": files,
			"child_job_ids": childIDs, "status": status, "elapsed_seconds": elapsed, "errors": errorsList}, status, message
	}
	return map[string]any{"status": status, "results": results}, status, message
}

func aggregateAccounts(parent store.Job, children []store.Job) (any, string, string) {
	var request generationPayload
	_ = json.Unmarshal([]byte(parent.Payload), &request)
	if parent.Kind != "image" || request.Count <= 1 {
		results := make([]map[string]any, 0, len(children))
		succeeded, partial := 0, false
		var messages []string
		for _, child := range children {
			var payload generationPayload
			_ = json.Unmarshal([]byte(child.Payload), &payload)
			item := map[string]any{"account_id": payload.AccountID}
			var result any
			if child.Result != "" {
				_ = json.Unmarshal([]byte(child.Result), &result)
			}
			if child.Status == "succeeded" || child.Status == "partial" {
				succeeded++
				item["result"] = result
				if child.Status == "partial" {
					partial = true
					item["error"] = "partial child result"
					messages = append(messages, payload.AccountID+": partial child result")
				} else if value, ok := result.(map[string]any); ok && value["status"] == "partial" {
					partial = true
					item["error"] = "partial child result"
					messages = append(messages, payload.AccountID+": partial child result")
				}
			} else {
				message := child.Error
				if message == "" {
					message = "child job " + child.Status
				}
				item["error"] = message
				messages = append(messages, payload.AccountID+": "+message)
			}
			results = append(results, item)
		}
		status := "succeeded"
		if succeeded == 0 {
			status = "failed"
		} else if succeeded != len(children) || partial {
			status = "partial"
		}
		return map[string]any{"status": status, "results": results}, status, strings.Join(messages, "; ")
	}
	type accountResult struct {
		media, files, childIDs []any
		failures               []string
		succeeded              int
		elapsed                float64
		partial                bool
	}
	groups := make(map[string]*accountResult)
	order := make([]string, 0)
	for _, child := range children {
		var payload generationPayload
		_ = json.Unmarshal([]byte(child.Payload), &payload)
		group := groups[payload.AccountID]
		if group == nil {
			group = &accountResult{}
			groups[payload.AccountID] = group
			order = append(order, payload.AccountID)
		}
		var value map[string]any
		if child.Result != "" {
			_ = json.Unmarshal([]byte(child.Result), &value)
		}
		if child.Status == "succeeded" || child.Status == "partial" {
			group.succeeded++
			childPartial := child.Status == "partial"
			if value != nil {
				if items, ok := value["media"].([]any); ok {
					group.media = append(group.media, items...)
				}
				if items, ok := value["files"].([]any); ok {
					group.files = append(group.files, items...)
				}
				if elapsed, ok := value["elapsed_seconds"].(float64); ok {
					group.elapsed += elapsed
				}
				childPartial = childPartial || value["status"] == "partial"
				if id, ok := value["job_id"]; ok {
					group.childIDs = append(group.childIDs, id)
				} else {
					group.childIDs = append(group.childIDs, child.ID)
				}
			} else {
				group.childIDs = append(group.childIDs, child.ID)
			}
			if childPartial {
				group.partial = true
				group.failures = append(group.failures, "partial child result")
			}
		} else {
			message := child.Error
			if message == "" {
				message = "child job " + child.Status
			}
			group.failures = append(group.failures, message)
			group.childIDs = append(group.childIDs, child.ID)
		}
	}
	results := make([]map[string]any, 0, len(order))
	allSucceeded := 0
	anyPartial := false
	var messages []string
	for _, id := range order {
		group := groups[id]
		status := "succeeded"
		if group.succeeded == 0 {
			status = "failed"
		} else if len(group.failures) > 0 || group.partial {
			status = "partial"
		}
		if group.succeeded > 0 {
			allSucceeded++
		}
		if status == "partial" {
			anyPartial = true
		}
		result := map[string]any{"job_id": parent.ID, "account_id": id, "media": group.media, "files": group.files,
			"child_job_ids": group.childIDs, "status": status, "elapsed_seconds": group.elapsed}
		item := map[string]any{"account_id": id, "result": result}
		if len(group.failures) > 0 {
			item["error"] = strings.Join(group.failures, "; ")
			messages = append(messages, id+": "+strings.Join(group.failures, "; "))
		}
		if group.partial && len(group.failures) == 0 {
			item["error"] = "partial child result"
			messages = append(messages, id+": partial child result")
		}
		results = append(results, item)
	}
	status := "succeeded"
	if allSucceeded == 0 {
		status = "failed"
	} else if allSucceeded < len(order) || anyPartial {
		status = "partial"
	}
	return map[string]any{"status": status, "results": results}, status, strings.Join(messages, "; ")
}
