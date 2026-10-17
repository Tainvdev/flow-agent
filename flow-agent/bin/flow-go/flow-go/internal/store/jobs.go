package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	ErrQueueFull    = errors.New("store: job queue is full")
	ErrNotPreSubmit = errors.New("store: latest attempt may have reached upstream")
	ErrNotResumable = errors.New("store: latest attempt has no accepted media to resume")
)

const maxOutstandingJobs = 1000

// CurrentJobPayloadVersion is the version used when a caller omits the field.
// Old persisted jobs are migrated to this version so their payload remains readable.
const CurrentJobPayloadVersion = 1

// Job is one durable service request.
type Job struct {
	ID             string      `json:"job_id"`
	Kind           string      `json:"kind"`
	Payload        string      `json:"payload"`
	PayloadVersion int         `json:"payload_version"`
	IdempotencyKey string      `json:"idempotency_key,omitempty"`
	Status         string      `json:"status"`
	Result         string      `json:"result,omitempty"`
	Error          string      `json:"error,omitempty"`
	ErrorCode      int         `json:"error_code,omitempty"`
	LeaseOwner     string      `json:"-"`
	LeaseUntil     *time.Time  `json:"-"`
	Attempts       int         `json:"attempts"`
	ParentID       string      `json:"parent_id,omitempty"`
	ChildIndex     int         `json:"child_index,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	FinishedAt     *time.Time  `json:"finished_at,omitempty"`
	ResumeAttempt  *JobAttempt `json:"-"`
}

// JobAttempt is one durable worker execution, including the bound account and
// session snapshot revision used for that execution.
type JobAttempt struct {
	AttemptNo       int                    `json:"attempt_no"`
	AccountID       string                 `json:"account_id,omitempty"`
	ProjectID       string                 `json:"project_id,omitempty"`
	SessionRevision string                 `json:"session_revision,omitempty"`
	Phase           string                 `json:"phase"`
	UpstreamIDs     []string               `json:"upstream_ids,omitempty"`
	Result          string                 `json:"-"`
	Error           string                 `json:"error,omitempty"`
	StartedAt       time.Time              `json:"started_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	FinishedAt      *time.Time             `json:"finished_at,omitempty"`
	PhaseEvents     []JobAttemptPhaseEvent `json:"phase_events,omitempty"`
}

// JobAttemptPhaseEvent is the durable timestamp for one phase transition.
type JobAttemptPhaseEvent struct {
	Phase string    `json:"phase"`
	At    time.Time `json:"at"`
}

// JobEvent is one durable, sequence-addressable event for an SSE consumer.
type JobEvent struct {
	ID    int64           `json:"id"`
	JobID string          `json:"job_id"`
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	At    time.Time       `json:"at"`
}

// JobQueueStats is a bounded snapshot of durable job states for service
// readiness and queue-pressure monitoring.
type JobQueueStats struct {
	Queued            int64      `json:"queued"`
	Running           int64      `json:"running"`
	WaitingChildren   int64      `json:"waiting_children"`
	Outstanding       int64      `json:"outstanding"`
	Capacity          int        `json:"capacity"`
	Succeeded         int64      `json:"succeeded"`
	Partial           int64      `json:"partial"`
	Failed            int64      `json:"failed"`
	Cancelled         int64      `json:"cancelled"`
	SubmissionUnknown int64      `json:"submission_unknown"`
	OldestQueuedAt    *time.Time `json:"oldest_queued_at,omitempty"`
	OldestQueuedAgeS  float64    `json:"oldest_queued_age_seconds"`
}

// JobQueueStats returns current counts and the age of the oldest accepted
// queued job. It reads aggregate state only; no request payloads are exposed.
func (s *Store) JobQueueStats(ctx context.Context) (JobQueueStats, error) {
	var stats JobQueueStats
	var oldest string
	err := s.db.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(status='queued'),0),
		COALESCE(SUM(status='running'),0),
		COALESCE(SUM(status='waiting_children'),0),
		COALESCE(SUM(status='succeeded'),0),
		COALESCE(SUM(status='partial'),0),
		COALESCE(SUM(status='failed'),0),
		COALESCE(SUM(status='cancelled'),0),
		COALESCE(SUM(status='submission_unknown'),0),
		COALESCE(MIN(CASE WHEN status='queued' THEN created_at END),'')
		FROM jobs`).Scan(&stats.Queued, &stats.Running, &stats.WaitingChildren,
		&stats.Succeeded, &stats.Partial, &stats.Failed, &stats.Cancelled,
		&stats.SubmissionUnknown, &oldest)
	if err != nil {
		return stats, fmt.Errorf("store: read job queue stats: %w", err)
	}
	stats.Outstanding = stats.Queued + stats.Running + stats.WaitingChildren
	stats.Capacity = s.queueLimit()
	if queuedAt, ok := parseSQLiteTime(oldest); ok {
		stats.OldestQueuedAt = &queuedAt
		age := time.Since(queuedAt).Seconds()
		if age > 0 {
			stats.OldestQueuedAgeS = age
		}
	}
	return stats, nil
}

func (s *Store) StartJobAttempt(ctx context.Context, jobID string, attemptNo int) error {
	if attemptNo < 1 {
		return fmt.Errorf("store: attempt number must be positive")
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_attempts(job_id, attempt_no, phase) VALUES(?, ?, 'preparing')`, jobID, attemptNo)
	if err != nil {
		return fmt.Errorf("store: start job attempt: %w", err)
	}
	return nil
}

func (s *Store) UpdateJobAttempt(ctx context.Context, jobID string, attemptNo int, phase, accountID, projectID, revision, message string) error {
	if strings.TrimSpace(phase) == "" {
		return fmt.Errorf("store: attempt phase is required")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE job_attempts SET
		phase=?, account_id=COALESCE(NULLIF(?,''),account_id), project_id=COALESCE(NULLIF(?,''),project_id),
		session_revision=COALESCE(NULLIF(?,''),session_revision), error=NULLIF(?,''), updated_at=?
		WHERE job_id=? AND attempt_no=?`, phase, accountID, projectID, revision, message, dbTimestamp(time.Now().UTC().Round(0)), jobID, attemptNo)
	if err != nil {
		return fmt.Errorf("store: update job attempt: %w", err)
	}
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM job_attempts WHERE job_id=? AND attempt_no=?`, jobID, attemptNo).Scan(&exists); err != nil {
		return fmt.Errorf("store: verify job attempt: %w", err)
	}
	return nil
}

// SetJobAttemptUpstreamIDs persists identifiers as soon as the upstream accepts
// a generation, so a restart can reconcile the accepted work without resubmitting it.
func (s *Store) SetJobAttemptUpstreamIDs(ctx context.Context, jobID string, attemptNo int, ids []string) error {
	encoded, err := json.Marshal(ids)
	if err != nil {
		return fmt.Errorf("store: encode upstream ids: %w", err)
	}
	r, err := s.db.ExecContext(ctx, `UPDATE job_attempts SET upstream_ids=?, updated_at=? WHERE job_id=? AND attempt_no=?`, string(encoded), dbTimestamp(time.Now().UTC().Round(0)), jobID, attemptNo)
	if err != nil {
		return fmt.Errorf("store: persist upstream ids: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: attempt %d for job %q not found", attemptNo, jobID)
	}
	return nil
}

func (s *Store) CompleteJobAttempt(ctx context.Context, jobID string, attemptNo int, result string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	encodedIDs, _ := json.Marshal(extractUpstreamIDs(result))
	r, err := s.db.ExecContext(ctx, `UPDATE job_attempts SET phase='complete', result=?, upstream_ids=?, error=NULL, updated_at=?, finished_at=? WHERE job_id=? AND attempt_no=?`, result, string(encodedIDs), now, now, jobID, attemptNo)
	if err != nil {
		return fmt.Errorf("store: complete job attempt: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: attempt %d for job %q not found", attemptNo, jobID)
	}
	return nil
}

func extractUpstreamIDs(raw string) []string {
	var root any
	if json.Unmarshal([]byte(raw), &root) != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var walk func(any)
	walk = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, child := range node {
				if key == "media_id" {
					if id, ok := child.(string); ok && id != "" {
						seen[id] = struct{}{}
					}
				}
				if key == "media_ids" || key == "upstream_ids" {
					if values, ok := child.([]any); ok {
						for _, value := range values {
							if id, ok := value.(string); ok && id != "" {
								seen[id] = struct{}{}
							}
						}
					}
				}
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(root)
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (s *Store) FailJobAttempt(ctx context.Context, jobID string, attemptNo int, phase, message string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	r, err := s.db.ExecContext(ctx, `UPDATE job_attempts SET phase=?, error=NULLIF(?,''), updated_at=?, finished_at=? WHERE job_id=? AND attempt_no=?`, phase, message, now, now, jobID, attemptNo)
	if err != nil {
		return fmt.Errorf("store: fail job attempt: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: attempt %d for job %q not found", attemptNo, jobID)
	}
	return nil
}

func (s *Store) JobAttempts(ctx context.Context, jobID string) ([]JobAttempt, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT attempt_no, COALESCE(account_id,''), COALESCE(project_id,''), COALESCE(session_revision,''), phase, COALESCE(upstream_ids,'[]'), COALESCE(result,''), COALESCE(error,''), started_at, updated_at, finished_at FROM job_attempts WHERE job_id=? ORDER BY attempt_no`, jobID)
	if err != nil {
		return nil, err
	}
	attempts := make([]JobAttempt, 0)
	for rows.Next() {
		var item JobAttempt
		var ids string
		var finished sql.NullTime
		if err := rows.Scan(&item.AttemptNo, &item.AccountID, &item.ProjectID, &item.SessionRevision, &item.Phase, &ids, &item.Result, &item.Error, &item.StartedAt, &item.UpdatedAt, &finished); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ids), &item.UpstreamIDs)
		if finished.Valid {
			item.FinishedAt = &finished.Time
		}
		attempts = append(attempts, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	byAttempt := make(map[int][]JobAttemptPhaseEvent, len(attempts))
	eventRows, err := s.db.QueryContext(ctx, `SELECT attempt_no, phase, occurred_at FROM job_attempt_phase_events WHERE job_id=? ORDER BY attempt_no, event_id`, jobID)
	if err != nil {
		return nil, fmt.Errorf("store: read attempt phase events: %w", err)
	}
	for eventRows.Next() {
		var attemptNo int
		var event JobAttemptPhaseEvent
		if err := eventRows.Scan(&attemptNo, &event.Phase, &event.At); err != nil {
			_ = eventRows.Close()
			return nil, err
		}
		byAttempt[attemptNo] = append(byAttempt[attemptNo], event)
	}
	if err := eventRows.Err(); err != nil {
		_ = eventRows.Close()
		return nil, err
	}
	if err := eventRows.Close(); err != nil {
		return nil, err
	}
	for i := range attempts {
		attempts[i].PhaseEvents = byAttempt[attempts[i].AttemptNo]
	}
	return attempts, nil
}

// JobEventsAfter returns ordered durable status/phase changes after a global
// event cursor. IDs are global so Last-Event-ID remains unambiguous per stream.
func (s *Store) JobEventsAfter(ctx context.Context, jobID string, afterID int64, limit int) ([]JobEvent, error) {
	if limit < 1 || limit > 1024 {
		return nil, fmt.Errorf("store: event limit must be between 1 and 1024")
	}
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, job_id, event_type, data, occurred_at
		FROM job_events WHERE job_id=? AND event_id>? ORDER BY event_id LIMIT ?`, jobID, afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("store: query job events: %w", err)
	}
	defer rows.Close()
	events := make([]JobEvent, 0)
	for rows.Next() {
		var event JobEvent
		var data string
		if err := rows.Scan(&event.ID, &event.JobID, &event.Type, &data, &event.At); err != nil {
			return nil, fmt.Errorf("store: read job event: %w", err)
		}
		event.Data = json.RawMessage(data)
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate job events: %w", err)
	}
	return events, nil
}

// JobAttemptPhase returns the current durable phase for one execution attempt.
// The worker uses this at failure time to distinguish a deadline before the
// submit boundary from an outcome that may have reached upstream.
func (s *Store) JobAttemptPhase(ctx context.Context, jobID string, attemptNo int) (string, error) {
	var phase string
	err := s.db.QueryRowContext(ctx, `SELECT phase FROM job_attempts WHERE job_id=? AND attempt_no=?`, jobID, attemptNo).Scan(&phase)
	return phase, err
}

type PhaseLatencySummary struct {
	Samples int     `json:"samples"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
}

type attemptTiming struct {
	attemptNo int
	createdAt time.Time
	kind      string
	phases    map[string]time.Time
}

// PhaseLatencySummaries computes recent stage latency percentiles from durable
// attempt events. The query is capped so a stats poll stays bounded as history
// grows; incomplete attempts simply do not contribute to a stage sample.
func (s *Store) PhaseLatencySummaries(ctx context.Context, limit int) (map[string]PhaseLatencySummary, error) {
	overall, _, err := s.PhaseLatencyBreakdown(ctx, limit)
	return overall, err
}

// PhaseLatencyBreakdown computes the same bounded summaries overall and split
// by job kind, so image and video latency can be compared from the durable
// phase stream without changing existing stats consumers.
func (s *Store) PhaseLatencyBreakdown(ctx context.Context, limit int) (map[string]PhaseLatencySummary, map[string]map[string]PhaseLatencySummary, error) {
	if limit < 1 {
		limit = 5000
	}
	if limit > 20000 {
		limit = 20000
	}
	rows, err := s.db.QueryContext(ctx, `WITH recent AS (
		SELECT a.job_id,a.attempt_no,a.started_at,j.created_at,j.kind
		FROM job_attempts a JOIN jobs j ON j.job_id=a.job_id
		ORDER BY a.started_at DESC LIMIT ?
	)
	SELECT r.job_id,r.attempt_no,r.created_at,r.kind,e.phase,e.occurred_at
	FROM recent r JOIN job_attempt_phase_events e
	ON e.job_id=r.job_id AND e.attempt_no=r.attempt_no
	ORDER BY r.started_at DESC,e.event_id`, limit)
	if err != nil {
		return nil, nil, fmt.Errorf("store: query phase latency events: %w", err)
	}
	defer rows.Close()
	timings := make(map[string]*attemptTiming)
	for rows.Next() {
		var jobID, kind, phase string
		var attemptNo int
		var createdAt, occurredAt time.Time
		if err := rows.Scan(&jobID, &attemptNo, &createdAt, &kind, &phase, &occurredAt); err != nil {
			return nil, nil, err
		}
		key := fmt.Sprintf("%s/%d", jobID, attemptNo)
		timing := timings[key]
		if timing == nil {
			timing = &attemptTiming{attemptNo: attemptNo, createdAt: createdAt, kind: kind, phases: make(map[string]time.Time)}
			timings[key] = timing
		}
		if _, exists := timing.phases[phase]; !exists {
			timing.phases[phase] = occurredAt
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	samples := map[string][]float64{
		"queue_wait": {}, "prepare_to_submit": {}, "submit_to_media": {},
		"accepted_to_media": {}, "render_wait": {}, "download": {}, "request_to_complete": {},
	}
	byKindSamples := make(map[string]map[string][]float64)
	appendSample := func(timing *attemptTiming, name string, start, end time.Time) {
		samples[name] = appendLatencySample(samples[name], start, end)
		if timing.kind == "" {
			return
		}
		if byKindSamples[timing.kind] == nil {
			byKindSamples[timing.kind] = map[string][]float64{}
		}
		byKindSamples[timing.kind][name] = appendLatencySample(byKindSamples[timing.kind][name], start, end)
	}
	for _, timing := range timings {
		preparing, hasPreparing := timing.phases["preparing"]
		submitting, hasSubmitting := timing.phases["submitting"]
		media, hasMedia := timing.phases["media_available"]
		downloading, hasDownloading := timing.phases["downloading"]
		complete, hasComplete := timing.phases["download_complete"]
		terminal, hasTerminal := timing.phases["complete"]
		if timing.attemptNo == 1 && hasPreparing {
			appendSample(timing, "queue_wait", timing.createdAt, preparing)
		}
		if hasPreparing && hasSubmitting {
			appendSample(timing, "prepare_to_submit", preparing, submitting)
		}
		if hasSubmitting && hasMedia {
			appendSample(timing, "submit_to_media", submitting, media)
		}
		if timing.attemptNo == 1 && hasMedia {
			appendSample(timing, "accepted_to_media", timing.createdAt, media)
		}
		if hasMedia && hasDownloading {
			appendSample(timing, "render_wait", media, downloading)
		}
		if hasDownloading && hasComplete {
			appendSample(timing, "download", downloading, complete)
		}
		if hasTerminal {
			appendSample(timing, "request_to_complete", timing.createdAt, terminal)
		}
	}
	result := summarizeLatencySamples(samples)
	grouped := make(map[string]map[string]PhaseLatencySummary, len(byKindSamples))
	for kind, kindSamples := range byKindSamples {
		grouped[kind] = summarizeLatencySamples(kindSamples)
	}
	return result, grouped, nil
}

func summarizeLatencySamples(samples map[string][]float64) map[string]PhaseLatencySummary {
	result := make(map[string]PhaseLatencySummary, len(samples))
	for name, values := range samples {
		if len(values) == 0 {
			continue
		}
		sort.Float64s(values)
		result[name] = PhaseLatencySummary{Samples: len(values), P50MS: percentile(values, 50), P95MS: percentile(values, 95)}
	}
	return result
}

func appendLatencySample(samples []float64, start, end time.Time) []float64 {
	delta := end.Sub(start)
	if delta >= 0 {
		return append(samples, float64(delta)/float64(time.Millisecond))
	}
	return samples
}

func percentile(sortedValues []float64, p int) float64 {
	index := (len(sortedValues)*p+99)/100 - 1
	if index < 0 {
		index = 0
	}
	return sortedValues[index]
}

// CreateJob persists a request before the service acknowledges it. If the
// idempotency key already exists, it returns that job and created=false.
func (s *Store) CreateJob(ctx context.Context, job Job) (*Job, bool, error) {
	if job.ID == "" || job.Kind == "" || job.Payload == "" {
		return nil, false, fmt.Errorf("store: job id, kind and payload are required")
	}
	if !json.Valid([]byte(job.Payload)) {
		return nil, false, fmt.Errorf("store: job payload must be valid JSON")
	}
	if job.PayloadVersion == 0 {
		job.PayloadVersion = CurrentJobPayloadVersion
	}
	if job.PayloadVersion != CurrentJobPayloadVersion {
		return nil, false, fmt.Errorf("store: unsupported job payload version %d", job.PayloadVersion)
	}
	key := any(job.IdempotencyKey)
	if job.IdempotencyKey == "" {
		key = nil
	}
	// Keep admission atomic under concurrent HTTP callers. An existing key is
	// always allowed through so retries can recover their original job at capacity.
	query := `INSERT INTO jobs(job_id, kind, payload, payload_version, idempotency_key)
		SELECT ?, ?, ?, ?, ? WHERE (? IS NOT NULL AND EXISTS (SELECT 1 FROM jobs WHERE idempotency_key = ?))
		   OR (SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running','waiting_children')) < ?
		ON CONFLICT(idempotency_key) DO NOTHING
		RETURNING ` + jobColumns
	created, err := s.jobQuery(ctx, query, job.ID, job.Kind, job.Payload, job.PayloadVersion, key, key, key, s.queueLimit())
	if err == nil {
		return created, true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("store: create job: %w", err)
	}
	if job.IdempotencyKey == "" {
		return nil, false, ErrQueueFull
	}
	existing, err := s.jobQuery(ctx, `SELECT `+jobColumns+` FROM jobs WHERE idempotency_key = ?`, job.IdempotencyKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, ErrQueueFull
		}
		return nil, false, err
	}
	return existing, false, nil
}

func (s *Store) JobByIdempotencyKey(ctx context.Context, key string) (*Job, error) {
	if strings.TrimSpace(key) == "" {
		return nil, sql.ErrNoRows
	}
	return s.jobQuery(ctx, `SELECT `+jobColumns+` FROM jobs WHERE idempotency_key=?`, key)
}

// CreateExpandedJob atomically persists a parent and its already-snapshotted
// children before acknowledging an all_accounts or image-count request.
func (s *Store) CreateExpandedJob(ctx context.Context, parent Job, children []Job) (*Job, bool, error) {
	if parent.ID == "" || parent.Kind == "" || parent.Payload == "" || len(children) == 0 {
		return nil, false, fmt.Errorf("store: parent and at least one valid child are required")
	}
	if !json.Valid([]byte(parent.Payload)) {
		return nil, false, fmt.Errorf("store: parent payload must be valid JSON")
	}
	if parent.PayloadVersion == 0 {
		parent.PayloadVersion = CurrentJobPayloadVersion
	}
	if parent.PayloadVersion != CurrentJobPayloadVersion {
		return nil, false, fmt.Errorf("store: unsupported parent payload version %d", parent.PayloadVersion)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("store: begin expanded create: %w", err)
	}
	defer tx.Rollback()
	if parent.IdempotencyKey != "" {
		var existingID string
		err := tx.QueryRowContext(ctx, `SELECT job_id FROM jobs WHERE idempotency_key=?`, parent.IdempotencyKey).Scan(&existingID)
		if err == nil {
			if err := tx.Commit(); err != nil {
				return nil, false, err
			}
			existing, err := s.JobByID(ctx, existingID)
			return existing, false, err
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
	}
	var outstanding int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running','waiting_children')`).Scan(&outstanding); err != nil {
		return nil, false, err
	}
	if outstanding+1+len(children) > s.queueLimit() {
		return nil, false, ErrQueueFull
	}
	var idem any
	if parent.IdempotencyKey != "" {
		idem = parent.IdempotencyKey
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO jobs(job_id,kind,payload,payload_version,idempotency_key,status) VALUES(?,?,?,?,?, 'waiting_children')`, parent.ID, parent.Kind, parent.Payload, parent.PayloadVersion, idem); err != nil {
		if parent.IdempotencyKey != "" {
			var existingID string
			if lookupErr := tx.QueryRowContext(ctx, `SELECT job_id FROM jobs WHERE idempotency_key=?`, parent.IdempotencyKey).Scan(&existingID); lookupErr == nil {
				_ = tx.Commit()
				existing, readErr := s.JobByID(ctx, existingID)
				return existing, false, readErr
			}
		}
		return nil, false, fmt.Errorf("store: create expanded parent: %w", err)
	}
	for index, child := range children {
		if child.ID == "" || child.Kind == "" || child.Payload == "" {
			return nil, false, fmt.Errorf("store: child %d is missing id, kind, or payload", index)
		}
		if !json.Valid([]byte(child.Payload)) {
			return nil, false, fmt.Errorf("store: child %d payload must be valid JSON", index)
		}
		if child.PayloadVersion == 0 {
			child.PayloadVersion = CurrentJobPayloadVersion
		}
		if child.PayloadVersion != CurrentJobPayloadVersion {
			return nil, false, fmt.Errorf("store: unsupported child payload version %d", child.PayloadVersion)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO jobs(job_id,kind,payload,payload_version,parent_id,child_index) VALUES(?,?,?,?,?,?)`, child.ID, child.Kind, child.Payload, child.PayloadVersion, parent.ID, index); err != nil {
			return nil, false, fmt.Errorf("store: create expanded child %d: %w", index, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("store: commit expanded create: %w", err)
	}
	created, err := s.JobByID(ctx, parent.ID)
	return created, err == nil, err
}

// ExpandParentJob atomically releases a claimed parent into durable child jobs.
func (s *Store) ExpandParentJob(ctx context.Context, parentID, owner string, children []Job) error {
	if len(children) == 0 {
		return fmt.Errorf("store: parent expansion requires at least one child")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin parent expansion: %w", err)
	}
	defer tx.Rollback()
	var outstanding int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN ('queued','running','waiting_children')`).Scan(&outstanding); err != nil {
		return fmt.Errorf("store: count outstanding jobs: %w", err)
	}
	if outstanding-1+len(children) > s.queueLimit() {
		return ErrQueueFull
	}
	parent, err := tx.ExecContext(ctx, `UPDATE jobs SET status='waiting_children', error=NULL, lease_owner=NULL, lease_until=NULL, updated_at=? WHERE job_id=? AND status='running' AND lease_owner=?`, dbTimestamp(time.Now().UTC().Round(0)), parentID, owner)
	if err != nil {
		return fmt.Errorf("store: release expanded parent: %w", err)
	}
	if changed, err := parent.RowsAffected(); err != nil || changed != 1 {
		if err != nil {
			return err
		}
		return fmt.Errorf("store: parent %q is not leased by %q", parentID, owner)
	}
	for index, child := range children {
		if child.ID == "" || child.Kind == "" || child.Payload == "" {
			return fmt.Errorf("store: child %d is missing id, kind, or payload", index)
		}
		if !json.Valid([]byte(child.Payload)) {
			return fmt.Errorf("store: child %d payload must be valid JSON", index)
		}
		if child.PayloadVersion == 0 {
			child.PayloadVersion = CurrentJobPayloadVersion
		}
		if child.PayloadVersion != CurrentJobPayloadVersion {
			return fmt.Errorf("store: unsupported child payload version %d", child.PayloadVersion)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO jobs(job_id,kind,payload,payload_version,parent_id,child_index) VALUES(?,?,?,?,?,?)`, child.ID, child.Kind, child.Payload, child.PayloadVersion, parentID, index); err != nil {
			return fmt.Errorf("store: persist child %d: %w", index, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit parent expansion: %w", err)
	}
	return nil
}

// WaitingParents returns batches whose children have all reached a terminal state.
func (s *Store) WaitingParents(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.job_id FROM jobs p WHERE p.status='waiting_children' AND NOT EXISTS (
		SELECT 1 FROM jobs c WHERE c.parent_id=p.job_id AND c.status IN ('queued','running')) ORDER BY p.created_at,p.job_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ChildJobs returns one parent's durable results in their requested order.
func (s *Store) ChildJobs(ctx context.Context, parentID string) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM jobs WHERE parent_id=? ORDER BY child_index`, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var jobs []Job
	for rows.Next() {
		var job Job
		if err := scanJob(rows, &job); err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

// FinishParent records an aggregate after all children are terminal.
func (s *Store) FinishParent(ctx context.Context, parentID, status, result, message string) error {
	if status != "succeeded" && status != "partial" && status != "failed" {
		return fmt.Errorf("store: invalid parent status %q", status)
	}
	now := dbTimestamp(time.Now().UTC().Round(0))
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET status=?,result=NULLIF(?,''),error=NULLIF(?,''),updated_at=?,finished_at=? WHERE job_id=? AND status='waiting_children'`, status, result, message, now, now, parentID)
	if err != nil {
		return fmt.Errorf("store: finish parent: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: parent %q is not waiting for children", parentID)
	}
	return nil
}

// CancelQueuedJob cancels a job that has not been claimed. Running jobs are
// controlled by the worker context and are never silently requeued.
func (s *Store) CancelQueuedJob(ctx context.Context, id string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='cancelled', updated_at=?, finished_at=?
		WHERE job_id=? AND status='queued'`, dbTimestamp(time.Now().UTC().Round(0)), dbTimestamp(time.Now().UTC().Round(0)), id)
	if err != nil {
		return fmt.Errorf("store: cancel queued job: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	return nil
}

// CancelParentAndQueuedChildren cancels an expanded batch and every child that
// has not started. Running children are cancelled by the service worker contexts.
func (s *Store) CancelParentAndQueuedChildren(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin parent cancellation: %w", err)
	}
	defer tx.Rollback()
	now := dbTimestamp(time.Now().UTC().Round(0))
	parent, err := tx.ExecContext(ctx, `UPDATE jobs SET status='cancelled',error=NULL,lease_owner=NULL,lease_until=NULL,updated_at=?,finished_at=? WHERE job_id=? AND status='waiting_children'`, now, now, id)
	if err != nil {
		return fmt.Errorf("store: cancel parent: %w", err)
	}
	n, err := parent.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `UPDATE jobs SET status='cancelled',updated_at=?,finished_at=? WHERE parent_id=? AND status='queued'`, now, now, id); err != nil {
		return fmt.Errorf("store: cancel queued children: %w", err)
	}
	return tx.Commit()
}

// SetJobCancelled marks an owned running job cancelled after its runner exits.
func (s *Store) SetJobCancelled(ctx context.Context, id, owner string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='cancelled', lease_owner=NULL, lease_until=NULL,
		updated_at=?, finished_at=? WHERE job_id=? AND status='running' AND lease_owner=?`,
		now, now, id, owner)
	if err != nil {
		return fmt.Errorf("store: mark job cancelled: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: job %q is not leased by %q", id, owner)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE job_attempts SET phase='cancelled', updated_at=?, finished_at=? WHERE job_id=? AND attempt_no=(SELECT MAX(attempt_no) FROM job_attempts WHERE job_id=?)`, now, now, id, id)
	if err != nil {
		return fmt.Errorf("store: cancel attempt: %w", err)
	}
	return nil
}

// DeleteTerminalJobsBefore removes only terminal queue records older than the
// supplied cutoff. Active and queued work is never eligible for retention cleanup.
func (s *Store) DeleteTerminalJobsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	r, err := s.db.ExecContext(ctx, `DELETE FROM jobs WHERE status IN
		('succeeded','partial','failed','cancelled','submission_unknown') AND finished_at IS NOT NULL AND julianday(finished_at) < julianday(?)`,
		dbTimestamp(cutoff.UTC().Round(0)))
	if err != nil {
		return 0, fmt.Errorf("store: delete old terminal jobs: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

const jobColumns = `job_id, kind, payload, payload_version, COALESCE(idempotency_key, ''), status,
	COALESCE(result, ''), COALESCE(error, ''), error_code, COALESCE(lease_owner, ''), lease_until,
	attempts, created_at, updated_at, finished_at, COALESCE(parent_id,''), child_index`

// JobByID reads one durable job.
func (s *Store) JobByID(ctx context.Context, id string) (*Job, error) {
	return s.jobQuery(ctx, `SELECT `+jobColumns+` FROM jobs WHERE job_id = ?`, id)
}

func (s *Store) jobQuery(ctx context.Context, query string, args ...any) (*Job, error) {
	var job Job
	err := scanJob(s.db.QueryRowContext(ctx, query, args...), &job)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner, job *Job) error {
	var leaseUntil, finishedAt sql.NullTime
	err := row.Scan(
		&job.ID, &job.Kind, &job.Payload, &job.PayloadVersion, &job.IdempotencyKey, &job.Status,
		&job.Result, &job.Error, &job.ErrorCode, &job.LeaseOwner, &leaseUntil, &job.Attempts,
		&job.CreatedAt, &job.UpdatedAt, &finishedAt, &job.ParentID, &job.ChildIndex,
	)
	if err != nil {
		return err
	}
	if leaseUntil.Valid {
		job.LeaseUntil = &leaseUntil.Time
	}
	if finishedAt.Valid {
		job.FinishedAt = &finishedAt.Time
	}
	return nil
}

// ClaimNextJob atomically leases the oldest runnable queued job. A job pinned
// to an account already serving another live job stays queued so it cannot
// park a service worker ahead of work for an idle account. An expired running
// lease is reconciled from its persisted phase before another job is claimed.
func (s *Store) ClaimNextJob(ctx context.Context, owner string, lease time.Duration) (*Job, error) {
	if owner == "" || lease <= 0 {
		return nil, fmt.Errorf("store: claim owner and positive lease are required")
	}
	now := time.Now().UTC().Round(0)
	until := now.Add(lease)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("store: begin expired-lease recovery: %w", err)
	}
	nowStamp := dbTimestamp(now)
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='succeeded',
		result=(SELECT a.result FROM job_attempts a WHERE a.job_id=jobs.job_id ORDER BY a.attempt_no DESC LIMIT 1),
		lease_owner=NULL, lease_until=NULL, updated_at=?, finished_at=?
		WHERE status='running' AND julianday(lease_until)<=julianday(?) AND EXISTS (
		 SELECT 1 FROM job_attempts a WHERE a.job_id=jobs.job_id AND a.phase='complete'
		 AND a.attempt_no=(SELECT MAX(b.attempt_no) FROM job_attempts b WHERE b.job_id=jobs.job_id))`, nowStamp, nowStamp, nowStamp)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("store: recover completed attempts: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='queued', error='worker stopped before submitting upstream work', error_code=0,
		lease_owner=NULL, lease_until=NULL, updated_at=?
		WHERE status='running' AND julianday(lease_until)<=julianday(?) AND NOT EXISTS (
		 SELECT 1 FROM job_attempts a WHERE a.job_id=jobs.job_id
		 AND a.attempt_no=(SELECT MAX(b.attempt_no) FROM job_attempts b WHERE b.job_id=jobs.job_id)
		 AND a.phase NOT IN ('preparing','interrupted_before_submit','retryable_failed'))`, nowStamp, nowStamp)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("store: recover pre-submit attempts: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='queued', error='resume accepted upstream media', error_code=0,
		lease_owner=NULL, lease_until=NULL, updated_at=?
		WHERE status='running' AND julianday(lease_until)<=julianday(?) AND EXISTS (
		 SELECT 1 FROM job_attempts a WHERE a.job_id=jobs.job_id
		 AND a.attempt_no=(SELECT MAX(b.attempt_no) FROM job_attempts b WHERE b.job_id=jobs.job_id)
		 AND a.phase IN ('media_available','rendering','downloading','download_complete')
		 AND a.account_id IS NOT NULL AND a.account_id<>'' AND a.account_id<>'all_accounts'
		 AND a.project_id IS NOT NULL AND a.project_id<>''
		 AND a.upstream_ids IS NOT NULL AND a.upstream_ids<>'[]')`, nowStamp, nowStamp)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("store: queue accepted media for resume: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE job_attempts SET phase='interrupted_before_submit', updated_at=?, finished_at=?
		WHERE phase='preparing' AND job_id IN (SELECT job_id FROM jobs WHERE status='queued' AND error='worker stopped before submitting upstream work')`, nowStamp, nowStamp)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("store: close pre-submit attempts: %w", err)
	}
	_, err = tx.ExecContext(ctx, `UPDATE jobs SET status='submission_unknown', error='worker lease expired after execution began; upstream submission state is unknown',
		lease_owner=NULL, lease_until=NULL, updated_at=?, finished_at=?
		WHERE status='running' AND julianday(lease_until)<=julianday(?)`, nowStamp, nowStamp, nowStamp)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("store: mark ambiguous expired jobs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("store: commit expired-lease recovery: %w", err)
	}
	var job Job
	err = scanJob(s.db.QueryRowContext(ctx, `
		UPDATE jobs
		   SET status = 'running', lease_owner = ?, lease_until = ?,
		       attempts = attempts + 1, updated_at = ?
		 WHERE job_id = (
		   SELECT job_id FROM jobs
		    WHERE status = 'queued'
		      AND (
		        COALESCE(NULLIF(json_extract(payload, '$.account_id'), ''), '') IN ('', 'all_accounts')
		        OR NOT EXISTS (
		          SELECT 1 FROM jobs active
		           WHERE active.status = 'running'
		             AND COALESCE(
		               NULLIF(json_extract(active.payload, '$.account_id'), ''),
		               (SELECT a.account_id FROM job_attempts a
		                 WHERE a.job_id = active.job_id
		                 ORDER BY a.attempt_no DESC LIMIT 1)
		             ) = json_extract(jobs.payload, '$.account_id')
		        )
		      )
		    ORDER BY created_at, job_id LIMIT 1
		 )
		RETURNING `+jobColumns, owner, dbTimestamp(until), dbTimestamp(now)), &job)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("store: claim job: %w", err)
	}
	if job.Error == "resume accepted upstream media" {
		attempts, attemptsErr := s.JobAttempts(ctx, job.ID)
		if attemptsErr != nil {
			return nil, fmt.Errorf("store: load attempt for resume: %w", attemptsErr)
		}
		if len(attempts) == 0 {
			return nil, fmt.Errorf("store: resume marker has no attempt for job %q", job.ID)
		}
		job.ResumeAttempt = &attempts[len(attempts)-1]
		if _, clearErr := s.db.ExecContext(ctx, `UPDATE jobs SET error=NULL WHERE job_id=? AND lease_owner=?`, job.ID, owner); clearErr != nil {
			return nil, fmt.Errorf("store: clear resume marker: %w", clearErr)
		}
		job.Error = ""
	}
	return &job, nil
}

// FinishJob records a terminal outcome only while the caller owns the lease.
func (s *Store) FinishJob(ctx context.Context, id, owner, status, result, message string) error {
	return s.FinishJobWithErrorCode(ctx, id, owner, status, result, message, 0)
}

// FinishJobWithErrorCode persists a terminal error's HTTP-compatible status
// mapping alongside the durable job so callers do not have to parse messages.
func (s *Store) FinishJobWithErrorCode(ctx context.Context, id, owner, status, result, message string, errorCode int) error {
	if status != "succeeded" && status != "partial" && status != "failed" && status != "submission_unknown" && status != "cancelled" {
		return fmt.Errorf("store: invalid terminal job status %q", status)
	}
	if errorCode != 0 && (errorCode < 400 || errorCode > 599) {
		return fmt.Errorf("store: invalid terminal error code %d", errorCode)
	}
	now := dbTimestamp(time.Now().UTC().Round(0))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin finish job: %w", err)
	}
	r, err := tx.ExecContext(ctx, `
		UPDATE jobs
		   SET status = ?, result = NULLIF(?, ''), error = NULLIF(?, ''), error_code = ?,
		       lease_owner = NULL, lease_until = NULL, updated_at = ?, finished_at = ?
		 WHERE job_id = ? AND status = 'running' AND lease_owner = ?`,
		status, result, message, errorCode, now, now, id, owner)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store: finish job: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n != 1 {
		_ = tx.Rollback()
		return fmt.Errorf("store: job %q is not leased by %q", id, owner)
	}
	_, err = tx.ExecContext(ctx, `UPDATE job_attempts SET phase=?, result=COALESCE(NULLIF(?,''),result), error=NULLIF(?,''), updated_at=?, finished_at=? WHERE job_id=? AND attempt_no=(SELECT MAX(attempt_no) FROM job_attempts WHERE job_id=?)`, status, result, message, now, now, id, id)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store: finish attempt: %w", err)
	}
	return tx.Commit()
}

// RequeueJob releases a failed pre-submit attempt for another worker. Callers
// must use this only when they know the upstream did not accept the submission.
func (s *Store) RequeueJob(ctx context.Context, id, owner, message string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin requeue: %w", err)
	}
	r, err := tx.ExecContext(ctx, `
		UPDATE jobs SET status = 'queued', error = NULLIF(?, ''), error_code=0, lease_owner = NULL,
		       lease_until = NULL, updated_at = ?
	 WHERE job_id = ? AND status = 'running' AND lease_owner = ?`,
		message, now, id, owner)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store: requeue job: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if n != 1 {
		_ = tx.Rollback()
		return fmt.Errorf("store: job %q is not leased by %q", id, owner)
	}
	_, err = tx.ExecContext(ctx, `UPDATE job_attempts SET phase='retryable_failed', error=?, updated_at=?, finished_at=? WHERE job_id=? AND attempt_no=(SELECT MAX(attempt_no) FROM job_attempts WHERE job_id=?)`, message, now, now, id, id)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("store: finish retryable attempt: %w", err)
	}
	return tx.Commit()
}

// RequeuePreSubmitJob releases a leased job only when its latest durable attempt
// is still preparing. This is used during graceful shutdown: cancellation
// before the generation RPC boundary is safe to replay, while later phases are
// deliberately left for ambiguous/recovery handling.
func (s *Store) RequeuePreSubmitJob(ctx context.Context, id, owner string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: begin pre-submit requeue: %w", err)
	}
	defer tx.Rollback()
	r, err := tx.ExecContext(ctx, `UPDATE jobs
		SET status='queued', error='worker stopped before submitting upstream work', error_code=0,
			lease_owner=NULL, lease_until=NULL, updated_at=?
		WHERE job_id=? AND status='running' AND lease_owner=? AND EXISTS (
			SELECT 1 FROM job_attempts a WHERE a.job_id=jobs.job_id
			AND a.attempt_no=(SELECT MAX(b.attempt_no) FROM job_attempts b WHERE b.job_id=jobs.job_id)
			AND a.phase='preparing')`, now, id, owner)
	if err != nil {
		return fmt.Errorf("store: requeue pre-submit job: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotPreSubmit
	}
	if _, err := tx.ExecContext(ctx, `UPDATE job_attempts
		SET phase='interrupted_before_submit', error='service stopped before upstream submission',
			updated_at=?, finished_at=?
		WHERE job_id=? AND attempt_no=(SELECT MAX(attempt_no) FROM job_attempts WHERE job_id=?)`, now, now, id, id); err != nil {
		return fmt.Errorf("store: mark pre-submit attempt interrupted: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit pre-submit requeue: %w", err)
	}
	return nil
}

// RequeueResumableJob releases a resumed attempt after service shutdown while
// preserving its durable phase and upstream ids for the next process.
func (s *Store) RequeueResumableJob(ctx context.Context, id, owner string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='queued', error='resume accepted upstream media', error_code=0,
		lease_owner=NULL, lease_until=NULL, updated_at=? WHERE job_id=? AND status='running' AND lease_owner=?`,
		dbTimestamp(time.Now().UTC().Round(0)), id, owner)
	if err != nil {
		return fmt.Errorf("store: requeue resumable job: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: job %q is not leased by %q", id, owner)
	}
	return nil
}

// RequeueAcceptedJobForResume releases an active job only when the latest
// attempt has durable upstream IDs and enough account/project metadata to
// resume polling or downloading without submitting another generation.
func (s *Store) RequeueAcceptedJobForResume(ctx context.Context, id, owner string) error {
	now := dbTimestamp(time.Now().UTC().Round(0))
	r, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='queued', error='resume accepted upstream media', error_code=0,
		lease_owner=NULL, lease_until=NULL, updated_at=? WHERE job_id=? AND status='running' AND lease_owner=?
		AND EXISTS (SELECT 1 FROM job_attempts a WHERE a.job_id=jobs.job_id
			AND a.attempt_no=(SELECT MAX(b.attempt_no) FROM job_attempts b WHERE b.job_id=jobs.job_id)
			AND a.phase IN ('media_available','rendering','downloading','download_complete')
			AND a.account_id IS NOT NULL AND a.account_id<>'' AND a.account_id<>'all_accounts'
			AND a.project_id IS NOT NULL AND a.project_id<>''
			AND a.upstream_ids IS NOT NULL AND a.upstream_ids<>'[]')`, now, id, owner)
	if err != nil {
		return fmt.Errorf("store: requeue accepted job for resume: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotResumable
	}
	return nil
}

// RenewJobLease keeps a long-running job owned by one service worker.
func (s *Store) RenewJobLease(ctx context.Context, id, owner string, lease time.Duration) error {
	if lease <= 0 {
		return fmt.Errorf("store: positive lease is required")
	}
	now := time.Now().UTC().Round(0)
	r, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET lease_until = ?, updated_at = ?
		 WHERE job_id = ? AND status = 'running' AND lease_owner = ?`,
		dbTimestamp(now.Add(lease)), dbTimestamp(now), id, owner)
	if err != nil {
		return fmt.Errorf("store: renew job lease: %w", err)
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("store: job %q is not leased by %q", id, owner)
	}
	return nil
}

func dbTimestamp(t time.Time) string { return t.Format(time.RFC3339Nano) }
