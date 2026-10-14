package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kodelyx/flow-go/flow-go/internal/bridge"
	"github.com/kodelyx/flow-go/flow-go/internal/pool"
	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

type queueLatencySample struct {
	jobID string
}

type queueLatencySubmit struct {
	jobID   string
	started time.Time
	err     error
}

type queueLatencyRunner struct {
	manager   *AccountManager
	completed chan<- queueLatencySample
	delay     time.Duration
}

func (r *queueLatencyRunner) Run(ctx context.Context, job store.Job) (any, error) {
	worker, err := r.manager.acquireScheduled(ctx, "", 0)
	if err != nil {
		return nil, err
	}
	defer r.manager.scheduler.Release(worker, 0, nil)
	timer := time.NewTimer(r.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	r.completed <- queueLatencySample{jobID: job.ID}
	return map[string]string{"account_id": worker.ID}, nil
}

// BenchmarkQueueLatencyByAccountCount measures local durable enqueue-to-runner
// completion latency with a fixed fake-upstream delay. It includes SQLite,
// HTTP acceptance, worker wake-up and scheduler contention, but no network call
// to Google Flow and no browser/extension latency.
func BenchmarkQueueLatencyByAccountCount(b *testing.B) {
	const batchSize = 40
	for _, kind := range []string{"image", "video"} {
		for _, accountCount := range []int{1, 5} {
			b.Run(fmt.Sprintf("kind=%s/accounts=%d", kind, accountCount), func(b *testing.B) {
				previousLogOutput := log.Writer()
				log.SetOutput(io.Discard)
				defer log.SetOutput(previousLogOutput)

				st, err := store.Open(filepath.Join(b.TempDir(), "queue-latency.db"))
				if err != nil {
					b.Fatal(err)
				}
				defer st.Close()
				manager, err := NewAccountManager(st, bridge.NewBridge(nil, nil, b.TempDir()), 20)
				if err != nil {
					b.Fatal(err)
				}
				manager.strictFresh = false
				manager.initOnce.Do(func() { close(manager.initialized) })
				for i := 0; i < accountCount; i++ {
					id := fmt.Sprintf("benchmark-account-%d", i)
					worker := pool.NewWorker(id, nil)
					worker.Concurrency = 1
					manager.scheduleWorkers[id] = worker
					manager.scheduler.Register(worker)
				}
				completed := make(chan queueLatencySample, batchSize*b.N)
				runner := &queueLatencyRunner{manager: manager, completed: completed, delay: 3 * time.Millisecond}
				jobs, err := NewJobs(st, runner, "benchmark-token", 20)
				if err != nil {
					b.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				workerDone := make(chan struct{})
				go func() { jobs.Start(ctx); close(workerDone) }()
				defer func() {
					cancel()
					select {
					case <-workerDone:
					case <-time.After(5 * time.Second):
						b.Error("queue workers did not stop")
					}
				}()

				allSamples := make([]float64, 0, batchSize*b.N)
				warmSamples := make([]float64, 0, batchSize*max(b.N-1, 0))
				coldSamples := make([]float64, 0, batchSize)
				failures := 0
				b.ResetTimer()
				for iteration := 0; iteration < b.N; iteration++ {
					startedAt := make(map[string]time.Time, batchSize)
					jobIDs := make([]string, 0, batchSize)
					submitted := make(chan queueLatencySubmit, batchSize)
					for i := 0; i < batchSize; i++ {
						go func(i int) {
							submitStarted := time.Now()
							payload := fmt.Sprintf(`{"kind":%q,"payload":{"prompt":"benchmark-%d-%d"}}`, kind, iteration, i)
							req := httptest.NewRequest(http.MethodPost, "/internal/v1/jobs", strings.NewReader(payload))
							req.Header.Set("Authorization", "Bearer benchmark-token")
							req.Header.Set("Idempotency-Key", fmt.Sprintf("%s-%d-%d", kind, iteration, i))
							res := httptest.NewRecorder()
							jobs.ServeHTTP(res, req)
							if res.Code != http.StatusAccepted {
								submitted <- queueLatencySubmit{err: fmt.Errorf("submit returned %d: %s", res.Code, res.Body.String())}
								return
							}
							var accepted struct {
								JobID string `json:"job_id"`
							}
							if err := json.Unmarshal(res.Body.Bytes(), &accepted); err != nil || accepted.JobID == "" {
								submitted <- queueLatencySubmit{err: fmt.Errorf("invalid acceptance response %q: %v", res.Body.String(), err)}
								return
							}
							submitted <- queueLatencySubmit{jobID: accepted.JobID, started: submitStarted}
						}(i)
					}
					for i := 0; i < batchSize; i++ {
						result := <-submitted
						if result.err != nil {
							b.Fatal(result.err)
						}
						startedAt[result.jobID] = result.started
						jobIDs = append(jobIDs, result.jobID)
					}
					for i := 0; i < batchSize; i++ {
						select {
						case sample := <-completed:
							if _, ok := startedAt[sample.jobID]; !ok {
								b.Fatalf("runner completed unknown job %q", sample.jobID)
							}
						case <-time.After(30 * time.Second):
							b.Fatal("timed out waiting for fake jobs")
						}
					}
					pending := len(jobIDs)
					deadline := time.Now().Add(30 * time.Second)
					for pending > 0 && time.Now().Before(deadline) {
						pending = 0
						for _, id := range jobIDs {
							started, awaiting := startedAt[id]
							if !awaiting {
								continue
							}
							job, err := st.JobByID(context.Background(), id)
							if err != nil {
								pending++
								continue
							}
							if job.Status != "succeeded" && job.Status != "failed" && job.Status != "submission_unknown" && job.Status != "partial" {
								pending++
								continue
							}
							if job.Status != "succeeded" || job.FinishedAt == nil {
								failures++
								delete(startedAt, id)
								continue
							}
							latencyMS := float64(job.FinishedAt.Sub(started)) / float64(time.Millisecond)
							allSamples = append(allSamples, latencyMS)
							if iteration == 0 {
								coldSamples = append(coldSamples, latencyMS)
							} else {
								warmSamples = append(warmSamples, latencyMS)
							}
							delete(startedAt, id)
						}
						if pending > 0 {
							time.Sleep(time.Millisecond)
						}
					}
					if pending > 0 {
						b.Fatalf("%d fake jobs did not reach a terminal state", pending)
					}
				}
				b.StopTimer()
				b.ReportMetric(percentileMS(allSamples, 50), "p50_ms")
				b.ReportMetric(percentileMS(allSamples, 95), "p95_ms")
				b.ReportMetric(float64(failures)*100/float64(max(batchSize*b.N, 1)), "failure_pct")
				b.ReportMetric(percentileMS(coldSamples, 50), "cold_p50_ms")
				b.ReportMetric(percentileMS(coldSamples, 95), "cold_p95_ms")
				b.ReportMetric(percentileMS(warmSamples, 50), "warm_p50_ms")
				b.ReportMetric(percentileMS(warmSamples, 95), "warm_p95_ms")
				b.ReportMetric(float64(len(allSamples)), "jobs")
			})
		}
	}
}

func percentileMS(samples []float64, percentile int) float64 {
	if len(samples) == 0 {
		return 0
	}
	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)
	index := (percentile*len(sorted)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}
