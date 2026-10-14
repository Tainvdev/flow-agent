package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateJobIsIdempotent(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()

	first, created, err := st.CreateJob(ctx, Job{
		ID: "job-1", Kind: "image", Payload: `{"prompt":"first"}`, IdempotencyKey: "caller-1",
	})
	if err != nil || !created {
		t.Fatalf("first create = (%v, %v), want created", first, err)
	}
	second, created, err := st.CreateJob(ctx, Job{
		ID: "job-2", Kind: "image", Payload: `{"prompt":"second"}`, IdempotencyKey: "caller-1",
	})
	if err != nil || created {
		t.Fatalf("duplicate create = (%v, %v), want existing job", second, err)
	}
	if second.ID != first.ID || second.Payload != first.Payload {
		t.Fatalf("duplicate key returned %+v, want original job %+v", second, first)
	}
}

func TestCreateJobDefaultsAndValidatesPayloadVersion(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	job, created, err := st.CreateJob(ctx, Job{ID: "version-default", Kind: "image", Payload: `{}`})
	if err != nil || !created || job.PayloadVersion != CurrentJobPayloadVersion {
		t.Fatalf("default version job=(%+v,created=%v,err=%v), want current version %d", job, created, err, CurrentJobPayloadVersion)
	}
	if _, _, err := st.CreateJob(ctx, Job{ID: "version-unknown", Kind: "image", Payload: `{}`, PayloadVersion: CurrentJobPayloadVersion + 1}); err == nil {
		t.Fatal("unsupported payload version was accepted")
	}
}

func TestCreateJobRejectsMalformedJSONPayload(t *testing.T) {
	st := openTemp(t)
	if _, _, err := st.CreateJob(context.Background(), Job{
		ID: "bad-payload", Kind: "image", Payload: `{"account_id":`,
	}); err == nil {
		t.Fatal("malformed JSON payload was accepted into the durable queue")
	}
	stats, err := st.JobQueueStats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if stats.Outstanding != 0 {
		t.Fatalf("outstanding jobs=%d after rejected payload, want 0", stats.Outstanding)
	}
}

func TestJobQueueStatsReportLiveQueueAndAmbiguousJobs(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, id := range []string{"queued-1", "queued-2", "unknown-1"} {
		if _, _, err := st.CreateJob(ctx, Job{ID: id, Kind: "image", Payload: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	claimed, err := st.ClaimNextJob(ctx, "worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim = (%+v,%v), want one job", claimed, err)
	}
	if err := st.FinishJob(ctx, claimed.ID, "worker", "submission_unknown", "", "ambiguous"); err != nil {
		t.Fatal(err)
	}

	stats, err := st.JobQueueStats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Queued != 2 || stats.SubmissionUnknown != 1 || stats.Running != 0 || stats.Outstanding != 2 || stats.Capacity != maxOutstandingJobs {
		t.Fatalf("queue stats = %+v, want 2 queued/outstanding, 1 ambiguous, 0 running, capacity %d", stats, maxOutstandingJobs)
	}
	if stats.OldestQueuedAt == nil || stats.OldestQueuedAgeS < 0 {
		t.Fatalf("oldest queued age = %+v, want a timestamp and nonnegative age", stats)
	}
}

func TestClaimLeaseAndFinishJob(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-1", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}

	claimed, err := st.ClaimNextJob(ctx, "worker-1", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim = (%v, %v), want a job", claimed, err)
	}
	if claimed.Status != "running" || claimed.Attempts != 1 || claimed.LeaseOwner != "worker-1" {
		t.Fatalf("claimed job state = %+v", claimed)
	}
	if other, err := st.ClaimNextJob(ctx, "worker-2", time.Minute); err != nil || other != nil {
		t.Fatalf("second claim = (%v, %v), want no job while lease is live", other, err)
	}
	if err := st.FinishJob(ctx, "job-1", "worker-2", "succeeded", `{"ok":true}`, ""); err == nil {
		t.Fatal("a worker that does not own the lease finished the job")
	}
	if err := st.FinishJob(ctx, "job-1", "worker-1", "succeeded", `{"ok":true}`, ""); err != nil {
		t.Fatalf("finish job: %v", err)
	}
	got, err := st.JobByID(ctx, "job-1")
	if err != nil || got.Status != "succeeded" || got.Result != `{"ok":true}` || got.FinishedAt == nil {
		t.Fatalf("finished job = (%+v, %v)", got, err)
	}
}

func TestFinishJobPersistsHTTPErrorCode(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "unavailable", Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "worker", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim=(%+v,%v), want job", claimed, err)
	}
	if err := st.FinishJobWithErrorCode(ctx, claimed.ID, "worker", "failed", "", "no account ready", 503); err != nil {
		t.Fatalf("finish job with status mapping: %v", err)
	}
	got, err := st.JobByID(ctx, claimed.ID)
	if err != nil || got.ErrorCode != 503 || got.Error != "no account ready" {
		t.Fatalf("persisted failure=(%+v,%v), want message and HTTP 503", got, err)
	}
}

func TestClaimNextJobSkipsQueuedWorkForBusyPinnedAccount(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, job := range []Job{
		{ID: "a-running", Kind: "image", Payload: `{"prompt":"a1","account_id":"account-a"}`},
		{ID: "a-waiting", Kind: "image", Payload: `{"prompt":"a2","account_id":"account-a"}`},
		{ID: "b-ready", Kind: "image", Payload: `{"prompt":"b1","account_id":"account-b"}`},
	} {
		if _, _, err := st.CreateJob(ctx, job); err != nil {
			t.Fatalf("create %s: %v", job.ID, err)
		}
	}

	first, err := st.ClaimNextJob(ctx, "worker-a", time.Minute)
	if err != nil || first == nil || first.ID != "a-running" {
		t.Fatalf("first claim=(%+v,%v), want a-running", first, err)
	}
	second, err := st.ClaimNextJob(ctx, "worker-b", time.Minute)
	if err != nil || second == nil || second.ID != "b-ready" {
		t.Fatalf("second claim=(%+v,%v), want runnable b-ready while account-a is busy", second, err)
	}
	if err := st.FinishJob(ctx, first.ID, "worker-a", "succeeded", `{}`, ""); err != nil {
		t.Fatalf("finish first account-a job: %v", err)
	}
	if err := st.FinishJob(ctx, second.ID, "worker-b", "succeeded", `{}`, ""); err != nil {
		t.Fatalf("finish account-b job: %v", err)
	}
	third, err := st.ClaimNextJob(ctx, "worker-c", time.Minute)
	if err != nil || third == nil || third.ID != "a-waiting" {
		t.Fatalf("third claim=(%+v,%v), want deferred a-waiting after account-a is free", third, err)
	}
}

func TestClaimNextJobUsesBoundAccountFromRunningAutomaticJob(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, job := range []Job{
		{ID: "auto-running", Kind: "image", Payload: `{"prompt":"automatic"}`},
		{ID: "pinned-waiting", Kind: "image", Payload: `{"prompt":"pinned","account_id":"account-a"}`},
		{ID: "other-account", Kind: "image", Payload: `{"prompt":"other","account_id":"account-b"}`},
	} {
		if _, _, err := st.CreateJob(ctx, job); err != nil {
			t.Fatalf("create %s: %v", job.ID, err)
		}
	}
	active, err := st.ClaimNextJob(ctx, "worker-a", time.Minute)
	if err != nil || active == nil || active.ID != "auto-running" {
		t.Fatalf("first claim=(%+v,%v), want auto-running", active, err)
	}
	if err := st.StartJobAttempt(ctx, active.ID, active.Attempts); err != nil {
		t.Fatalf("start automatic attempt: %v", err)
	}
	if err := st.UpdateJobAttempt(ctx, active.ID, active.Attempts, "preparing", "account-a", "project-a", "revision-a", ""); err != nil {
		t.Fatalf("bind automatic attempt to account-a: %v", err)
	}
	next, err := st.ClaimNextJob(ctx, "worker-b", time.Minute)
	if err != nil || next == nil || next.ID != "other-account" {
		t.Fatalf("second claim=(%+v,%v), want other-account while auto job owns account-a", next, err)
	}
}

func TestExpiredLeaseBecomesSubmissionUnknown(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-1", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "crashed-worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-1", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "job-1", 1, "executing", "acct-1", "project-1", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE jobs SET lease_until = ? WHERE job_id = 'job-1'`, dbTimestamp(time.Now().UTC().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}

	claimed, err := st.ClaimNextJob(ctx, "recovery-worker", time.Minute)
	if err != nil || claimed != nil {
		t.Fatalf("recovery claim = (%v, %v), want no replay of ambiguous submission", claimed, err)
	}
	got, err := st.JobByID(ctx, "job-1")
	if err != nil || got.Status != "submission_unknown" || got.Attempts != 1 {
		t.Fatalf("expired job state = (%+v, %v), want terminal unknown", got, err)
	}
}

func TestExpiredPreparingAttemptCanBeReclaimedSafely(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-prep", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-prep", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='job-prep'`, dbTimestamp(time.Now().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "worker-2", time.Minute)
	if err != nil || claimed == nil || claimed.ID != "job-prep" || claimed.Attempts != 2 {
		t.Fatalf("safe recovery claim=(%+v,%v)", claimed, err)
	}
	attempts, err := st.JobAttempts(ctx, "job-prep")
	if err != nil || len(attempts) != 1 || attempts[0].Phase != "interrupted_before_submit" {
		t.Fatalf("attempts after safe recovery=(%+v,%v)", attempts, err)
	}
}

func TestGracefulShutdownRequeuesPreparingAttempt(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-stop-prep", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-stop-prep", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "job-stop-prep", 1, "preparing", "acct-1", "project-1", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeuePreSubmitJob(ctx, "job-stop-prep", "worker-1"); err != nil {
		t.Fatalf("requeue preparing job: %v", err)
	}
	job, err := st.JobByID(ctx, "job-stop-prep")
	if err != nil || job.Status != "queued" {
		t.Fatalf("job=(%+v,%v), want queued", job, err)
	}
	attempts, err := st.JobAttempts(ctx, "job-stop-prep")
	if err != nil || len(attempts) != 1 || attempts[0].Phase != "interrupted_before_submit" {
		t.Fatalf("attempts=(%+v,%v), want interrupted_before_submit", attempts, err)
	}
}

func TestGracefulShutdownDoesNotRequeueAfterSubmitBoundary(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-stop-submit", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-stop-submit", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "job-stop-submit", 1, "submitting", "acct-1", "project-1", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeuePreSubmitJob(ctx, "job-stop-submit", "worker-1"); !errors.Is(err, ErrNotPreSubmit) {
		t.Fatalf("requeue submitting job error=%v, want ErrNotPreSubmit", err)
	}
	job, err := st.JobByID(ctx, "job-stop-submit")
	if err != nil || job.Status != "running" || job.LeaseOwner != "worker-1" {
		t.Fatalf("job=(%+v,%v), want leased and running", job, err)
	}
}

func TestGracefulShutdownRequeuesAcceptedMediaForResume(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-stop-rendering", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "worker-1", time.Minute)
	if err != nil || claimed == nil {
		t.Fatalf("claim accepted job=(%+v,%v)", claimed, err)
	}
	if err := st.StartJobAttempt(ctx, claimed.ID, claimed.Attempts); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, claimed.ID, claimed.Attempts, "rendering", "account-a", "project-a", "revision-a", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobAttemptUpstreamIDs(ctx, claimed.ID, claimed.Attempts, []string{"media-a"}); err != nil {
		t.Fatal(err)
	}
	if err := st.RequeueAcceptedJobForResume(ctx, claimed.ID, "worker-1"); err != nil {
		t.Fatalf("requeue accepted generation: %v", err)
	}
	resumed, err := st.ClaimNextJob(ctx, "worker-2", time.Minute)
	if err != nil || resumed == nil || resumed.ID != claimed.ID || resumed.ResumeAttempt == nil {
		t.Fatalf("claim resume=(%+v,%v), want durable resume attempt", resumed, err)
	}
	if resumed.ResumeAttempt.AccountID != "account-a" || resumed.ResumeAttempt.ProjectID != "project-a" || len(resumed.ResumeAttempt.UpstreamIDs) != 1 || resumed.ResumeAttempt.UpstreamIDs[0] != "media-a" {
		t.Fatalf("resume metadata=(%+v), want original account/project/media IDs", resumed.ResumeAttempt)
	}
}

func TestAttemptPhaseTransitionsArePersistedInOrder(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-phases", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-phases", 1); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"submitting", "rendering", "downloading"} {
		if err := st.UpdateJobAttempt(ctx, "job-phases", 1, phase, "acct-1", "project-1", "rev-1", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CompleteJobAttempt(ctx, "job-phases", 1, `{"media_ids":["m1"]}`); err != nil {
		t.Fatal(err)
	}
	attempts, err := st.JobAttempts(ctx, "job-phases")
	if err != nil || len(attempts) != 1 {
		t.Fatalf("attempts=(%+v,%v)", attempts, err)
	}
	want := []string{"preparing", "submitting", "rendering", "downloading", "complete"}
	if len(attempts[0].PhaseEvents) != len(want) {
		t.Fatalf("phase events=%+v, want phases %v", attempts[0].PhaseEvents, want)
	}
	for i, phase := range want {
		if attempts[0].PhaseEvents[i].Phase != phase {
			t.Fatalf("event %d phase=%q, want %q", i, attempts[0].PhaseEvents[i].Phase, phase)
		}
		if i > 0 && attempts[0].PhaseEvents[i].At.Before(attempts[0].PhaseEvents[i-1].At) {
			t.Fatalf("phase event time moved backwards: %+v", attempts[0].PhaseEvents)
		}
	}
}

func TestMigrationBackfillsLegacyAttemptPhaseEvent(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-legacy-phase", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`DROP TRIGGER trg_job_attempt_phase_insert; DROP TRIGGER trg_job_attempt_phase_update; INSERT INTO job_attempts(job_id,attempt_no,phase,started_at,updated_at) VALUES('job-legacy-phase',1,'rendering','2026-01-01T00:00:00Z','2026-01-01T00:01:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(st.db); err != nil {
		t.Fatalf("migrate old attempt database: %v", err)
	}
	attempts, err := st.JobAttempts(ctx, "job-legacy-phase")
	if err != nil || len(attempts) != 1 || len(attempts[0].PhaseEvents) != 1 {
		t.Fatalf("legacy attempt events=(%+v,%v)", attempts, err)
	}
	if event := attempts[0].PhaseEvents[0]; event.Phase != "rendering" || !event.At.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("backfilled event=%+v", event)
	}
}

func TestPhaseLatencySummariesComputeDurableStagePercentiles(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-latency", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-latency", 1); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"submitting", "media_available", "downloading", "download_complete"} {
		if err := st.UpdateJobAttempt(ctx, "job-latency", 1, phase, "acct", "project", "rev", ""); err != nil {
			t.Fatal(err)
		}
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := st.db.Exec(`UPDATE jobs SET created_at=? WHERE job_id=?`, dbTimestamp(base), "job-latency"); err != nil {
		t.Fatal(err)
	}
	phaseOffsets := map[string]int{
		"preparing": 1, "submitting": 3, "media_available": 8,
		"downloading": 11, "download_complete": 13,
	}
	for phase, seconds := range phaseOffsets {
		if _, err := st.db.Exec(`UPDATE job_attempt_phase_events SET occurred_at=? WHERE job_id=? AND phase=?`,
			dbTimestamp(base.Add(time.Duration(seconds)*time.Second)), "job-latency", phase); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.PhaseLatencySummaries(ctx, 5000)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]float64{
		"queue_wait": 1000, "prepare_to_submit": 2000, "submit_to_media": 5000,
		"accepted_to_media": 8000, "render_wait": 3000, "download": 2000,
	}
	for phase, wantMS := range want {
		stat, ok := got[phase]
		if !ok || stat.Samples != 1 || stat.P50MS != wantMS || stat.P95MS != wantMS {
			t.Errorf("latency %s=%+v (exists=%v), want one sample at %.0fms", phase, stat, ok, wantMS)
		}
	}
}

func TestPhaseLatencyBreakdownSeparatesImageAndVideo(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, kind := range []string{"image", "video"} {
		id := "latency-" + kind
		if _, _, err := st.CreateJob(ctx, Job{ID: id, Kind: kind, Payload: `{}`}); err != nil {
			t.Fatal(err)
		}
		if _, err := st.ClaimNextJob(ctx, "worker-"+kind, time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := st.StartJobAttempt(ctx, id, 1); err != nil {
			t.Fatal(err)
		}
		for _, phase := range []string{"preparing", "submitting", "media_available", "downloading", "download_complete"} {
			if err := st.UpdateJobAttempt(ctx, id, 1, phase, "account-"+kind, "project-"+kind, "", ""); err != nil {
				t.Fatal(err)
			}
		}
		if err := st.CompleteJobAttempt(ctx, id, 1, `{}`); err != nil {
			t.Fatal(err)
		}
	}
	overall, grouped, err := st.PhaseLatencyBreakdown(ctx, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(grouped) != 2 || grouped["image"]["submit_to_media"].Samples != 1 || grouped["video"]["submit_to_media"].Samples != 1 || grouped["image"]["request_to_complete"].Samples != 1 || grouped["video"]["request_to_complete"].Samples != 1 {
		t.Fatalf("kind latency breakdown=%+v; want one image and one video submit sample", grouped)
	}
	if overall["submit_to_media"].Samples != 2 {
		t.Fatalf("overall submit samples=%d, want 2", overall["submit_to_media"].Samples)
	}
}

func TestExpiredCompletedAttemptRecoversStoredResult(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-done", Kind: "image", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-done", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.CompleteJobAttempt(ctx, "job-done", 1, `{"media_ids":["m1"],"media":[{"media_id":"m2"}]}`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='job-done'`, dbTimestamp(time.Now().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "worker-2", time.Minute)
	if err != nil || claimed != nil {
		t.Fatalf("completed attempt should not be re-run: (%+v,%v)", claimed, err)
	}
	job, err := st.JobByID(ctx, "job-done")
	if err != nil || job.Status != "succeeded" || job.Result == "" {
		t.Fatalf("recovered job=(%+v,%v)", job, err)
	}
	attempts, err := st.JobAttempts(ctx, "job-done")
	if err != nil || len(attempts) != 1 || len(attempts[0].UpstreamIDs) != 2 {
		t.Fatalf("recovered attempt metadata=(%+v,%v)", attempts, err)
	}
}

func TestMigrationBackfillsUnifiedJobEventsForExistingJobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-events.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "legacy-stream", Kind: "image", Payload: `{"prompt":"x"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "legacy-worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "legacy-stream", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "legacy-stream", 1, "submitting", "account-a", "project-a", "rev-a", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	admin, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(`DELETE FROM job_events`); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	if err := admin.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.JobEventsAfter(ctx, "legacy-stream", 0, 32)
	if err != nil {
		t.Fatal(err)
	}
	var sawStatus, sawPreparing, sawSubmitting bool
	for _, event := range events {
		var data map[string]any
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("event %d data %q: %v", event.ID, event.Data, err)
		}
		switch {
		case event.Type == "status" && data["status"] == "running":
			sawStatus = true
		case event.Type == "phase" && data["phase"] == "preparing":
			sawPreparing = true
		case event.Type == "phase" && data["phase"] == "submitting":
			sawSubmitting = true
		}
	}
	if !sawStatus || !sawPreparing || !sawSubmitting {
		t.Fatalf("migration backfill events=%+v, want current status and prior attempt phases", events)
	}
}

func TestExpiredRenderingAttemptIsClaimedForResume(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-rendering", Kind: "video", Payload: `{"prompt":"test"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-rendering", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "job-rendering", 1, "rendering", "acct-1", "project-1", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobAttemptUpstreamIDs(ctx, "job-rendering", 1, []string{"media-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='job-rendering'`, dbTimestamp(time.Now().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "worker-2", time.Minute)
	if err != nil || claimed == nil || claimed.ResumeAttempt == nil {
		t.Fatalf("resume claim=(%+v,%v), want accepted-media resume", claimed, err)
	}
	if claimed.ResumeAttempt.AttemptNo != 1 || claimed.ResumeAttempt.AccountID != "acct-1" || len(claimed.ResumeAttempt.UpstreamIDs) != 1 {
		t.Fatalf("resume metadata=(%+v), want original account and media id", claimed.ResumeAttempt)
	}
	if claimed.Attempts != 2 {
		t.Fatalf("job claim count=%d, want 2", claimed.Attempts)
	}
}

func TestExpiredDownloadCompleteAttemptIsResumedAfterRestart(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-download-complete", Kind: "video", Payload: `{"prompt":"test"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "before-restart", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-download-complete", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateJobAttempt(ctx, "job-download-complete", 1, "download_complete", "acct-1", "project-1", "rev-1", ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobAttemptUpstreamIDs(ctx, "job-download-complete", 1, []string{"media-1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE jobs SET lease_until=? WHERE job_id='job-download-complete'`, dbTimestamp(time.Now().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ClaimNextJob(ctx, "after-restart", time.Minute)
	if err != nil || claimed == nil || claimed.ResumeAttempt == nil {
		t.Fatalf("recovery claim=(%+v,%v), want a download-complete resume", claimed, err)
	}
	if claimed.ResumeAttempt.Phase != "download_complete" || claimed.ResumeAttempt.AccountID != "acct-1" || claimed.ResumeAttempt.ProjectID != "project-1" || len(claimed.ResumeAttempt.UpstreamIDs) != 1 {
		t.Fatalf("recovery metadata=(%+v), want original account/project/media", claimed.ResumeAttempt)
	}
}

func TestAttemptPersistsUpstreamIDsBeforeCompletion(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "job-accepted", Kind: "video", Payload: `{}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(ctx, "worker-1", time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := st.StartJobAttempt(ctx, "job-accepted", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobAttemptUpstreamIDs(ctx, "job-accepted", 1, []string{"video-1", "video-2"}); err != nil {
		t.Fatal(err)
	}
	attempts, err := st.JobAttempts(ctx, "job-accepted")
	if err != nil || len(attempts) != 1 || len(attempts[0].UpstreamIDs) != 2 || attempts[0].UpstreamIDs[1] != "video-2" {
		t.Fatalf("accepted attempt IDs=(%+v,%v)", attempts, err)
	}
}

func TestQueueCapacityRejectsNewJobsButAllowsIdempotentReplay(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	first, created, err := st.CreateJob(ctx, Job{ID: "existing", Kind: "image", Payload: `{}`, IdempotencyKey: "key-existing"})
	if err != nil || !created {
		t.Fatalf("seed job=(%+v,%v), created=%v", first, err, created)
	}
	_, err = st.db.Exec(`WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM seq WHERE n < 999)
		INSERT INTO jobs(job_id, kind, payload) SELECT 'queued-'||n, 'image', '{}' FROM seq`)
	if err != nil {
		t.Fatal(err)
	}
	if _, created, err := st.CreateJob(ctx, Job{ID: "overflow", Kind: "image", Payload: `{}`}); !errors.Is(err, ErrQueueFull) || created {
		t.Fatalf("over-capacity create=(created %v, err %v), want queue full", created, err)
	}
	replay, created, err := st.CreateJob(ctx, Job{ID: "new-id", Kind: "image", Payload: `{}`, IdempotencyKey: "key-existing"})
	if err != nil || created || replay.ID != first.ID {
		t.Fatalf("full-queue replay=(%+v, created %v, err %v)", replay, created, err)
	}
}

func TestQueueCapacityCanBeConfiguredPerStore(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if err := st.SetMaxOutstandingJobs(1); err != nil {
		t.Fatal(err)
	}
	if _, created, err := st.CreateJob(ctx, Job{ID: "one", Kind: "image", Payload: `{}`}); err != nil || !created {
		t.Fatalf("first job create=(created %v, err %v)", created, err)
	}
	if _, created, err := st.CreateJob(ctx, Job{ID: "two", Kind: "image", Payload: `{}`}); !errors.Is(err, ErrQueueFull) || created {
		t.Fatalf("second job create=(created %v, err %v), want queue full at configured limit", created, err)
	}
	if err := st.SetMaxOutstandingJobs(0); err == nil {
		t.Fatal("zero queue capacity should be rejected")
	}
}

func TestExpandedJobCountsParentAndChildrenAgainstCapacity(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if err := st.SetMaxOutstandingJobs(2); err != nil {
		t.Fatal(err)
	}
	_, _, err := st.CreateExpandedJob(ctx,
		Job{ID: "parent-over", Kind: "image", Payload: `{}`},
		[]Job{
			{ID: "child-over-1", Kind: "image", Payload: `{}`},
			{ID: "child-over-2", Kind: "image", Payload: `{}`},
		},
	)
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expanded create error = %v, want queue full for parent plus two children", err)
	}
	if _, err := st.JobByID(ctx, "parent-over"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("rejected parent was partially stored: %v", err)
	}
	if _, created, err := st.CreateExpandedJob(ctx,
		Job{ID: "parent-fit", Kind: "image", Payload: `{}`},
		[]Job{{ID: "child-fit", Kind: "image", Payload: `{}`}},
	); err != nil || !created {
		t.Fatalf("expanded create at exact capacity=(created %v, err %v)", created, err)
	}
}

func TestParentExpansionPersistsChildrenAndWaitsForTerminalResults(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	if _, _, err := st.CreateJob(ctx, Job{ID: "parent-1", Kind: "batch", Payload: `{"all_accounts":true}`}); err != nil {
		t.Fatal(err)
	}
	parent, err := st.ClaimNextJob(ctx, "worker-parent", time.Minute)
	if err != nil || parent == nil {
		t.Fatalf("claim parent=(%+v,%v)", parent, err)
	}
	children := []Job{
		{ID: "child-a", Kind: "image", Payload: `{"account_id":"a"}`},
		{ID: "child-b", Kind: "image", Payload: `{"account_id":"b"}`},
	}
	if err := st.ExpandParentJob(ctx, parent.ID, parent.LeaseOwner, children); err != nil {
		t.Fatal(err)
	}
	if ready, err := st.WaitingParents(ctx); err != nil || len(ready) != 0 {
		t.Fatalf("parents before child completion=(%v,%v)", ready, err)
	}
	parentAfter, err := st.JobByID(ctx, "parent-1")
	if err != nil || parentAfter.Status != "waiting_children" {
		t.Fatalf("expanded parent=(%+v,%v)", parentAfter, err)
	}
	childList, err := st.ChildJobs(ctx, "parent-1")
	if err != nil || len(childList) != 2 || childList[0].ParentID != "parent-1" || childList[1].ChildIndex != 1 {
		t.Fatalf("children=(%+v,%v)", childList, err)
	}
	for _, child := range children {
		claimed, err := st.ClaimNextJob(ctx, "worker-child", time.Minute)
		if err != nil || claimed == nil || claimed.ID != child.ID {
			t.Fatalf("claim child=(%+v,%v), want %s", claimed, err, child.ID)
		}
		status, result, message := "succeeded", `{"ok":true}`, ""
		if child.ID == "child-b" {
			status, result, message = "failed", "", "upstream denied"
		}
		if err := st.FinishJob(ctx, child.ID, claimed.LeaseOwner, status, result, message); err != nil {
			t.Fatal(err)
		}
	}
	ready, err := st.WaitingParents(ctx)
	if err != nil || len(ready) != 1 || ready[0] != "parent-1" {
		t.Fatalf("ready parents=(%v,%v)", ready, err)
	}
	if err := st.FinishParent(ctx, "parent-1", "partial", `{"status":"partial"}`, "b: upstream denied"); err != nil {
		t.Fatal(err)
	}
	got, err := st.JobByID(ctx, "parent-1")
	if err != nil || got.Status != "partial" || got.Result != `{"status":"partial"}` {
		t.Fatalf("final parent=(%+v,%v)", got, err)
	}
}

func TestCreateExpandedJobIsAtomicAndIdempotent(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	parent := Job{ID: "parent-idempotent", Kind: "image", Payload: `{"count":2}`, IdempotencyKey: "batch-key"}
	children := []Job{{ID: "image-1", Kind: "image", Payload: `{"account_id":"acct-1"}`}, {ID: "image-2", Kind: "image", Payload: `{"account_id":"acct-1"}`}}
	created, wasCreated, err := st.CreateExpandedJob(ctx, parent, children)
	if err != nil || !wasCreated || created.Status != "waiting_children" {
		t.Fatalf("create expanded=(%+v,%v,%v)", created, wasCreated, err)
	}
	replay, wasCreated, err := st.CreateExpandedJob(ctx, Job{ID: "ignored-parent", Kind: "image", Payload: parent.Payload, IdempotencyKey: parent.IdempotencyKey}, []Job{{ID: "ignored-child", Kind: "image", Payload: `{}`}})
	if err != nil || wasCreated || replay.ID != parent.ID {
		t.Fatalf("expanded replay=(%+v,%v,%v)", replay, wasCreated, err)
	}
	childrenNow, err := st.ChildJobs(ctx, parent.ID)
	if err != nil || len(childrenNow) != 2 {
		t.Fatalf("replayed children=(%+v,%v); duplicate children must not be inserted", childrenNow, err)
	}
}

func TestCancelExpandedParentCancelsQueuedChildren(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	_, _, err := st.CreateExpandedJob(ctx, Job{ID: "cancel-parent", Kind: "image", Payload: `{"all_accounts":true}`}, []Job{
		{ID: "cancel-child-1", Kind: "image", Payload: `{"account_id":"a"}`},
		{ID: "cancel-child-2", Kind: "image", Payload: `{"account_id":"b"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CancelParentAndQueuedChildren(ctx, "cancel-parent"); err != nil {
		t.Fatal(err)
	}
	parent, err := st.JobByID(ctx, "cancel-parent")
	children, childErr := st.ChildJobs(ctx, "cancel-parent")
	if err != nil || childErr != nil || parent.Status != "cancelled" || len(children) != 2 || children[0].Status != "cancelled" || children[1].Status != "cancelled" {
		t.Fatalf("cancel parent=%+v children=%+v errors=(%v,%v)", parent, children, err, childErr)
	}
}

func TestRetentionDeletesOnlyOldTerminalJobs(t *testing.T) {
	st := openTemp(t)
	ctx := context.Background()
	for _, id := range []string{"old-done", "old-queued"} {
		if _, _, err := st.CreateJob(ctx, Job{ID: id, Kind: "image", Payload: `{}`}); err != nil {
			t.Fatal(err)
		}
	}
	_, err := st.db.Exec(`UPDATE jobs SET status='succeeded', finished_at=?, updated_at=? WHERE job_id='old-done'`,
		dbTimestamp(time.Now().Add(-48*time.Hour)), dbTimestamp(time.Now().Add(-48*time.Hour)))
	if err != nil {
		t.Fatal(err)
	}
	n, err := st.DeleteTerminalJobsBefore(ctx, time.Now().Add(-24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("retention deleted %d rows, err=%v", n, err)
	}
	if _, err := st.JobByID(ctx, "old-done"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("old terminal job still exists: %v", err)
	}
	if job, err := st.JobByID(ctx, "old-queued"); err != nil || job.Status != "queued" {
		t.Fatalf("queued job changed by retention: (%+v,%v)", job, err)
	}
}
