package engine

import "context"

type ResumeMediaError struct{ Message string }

func (e *ResumeMediaError) Error() string { return e.Message }

func (e *ResumeMediaError) RetryClass() string { return "resume_media" }

// SafeToRetry means lookup/download failed but generation is known to have been
// accepted; another resume attempt cannot duplicate generation.
func (e *ResumeMediaError) SafeToRetry() bool { return true }

// EstimateVideoCredits returns the same cost tuple used by the submit path.
// A false known value means the scheduler must admit the job without a credit
// threshold; the engine still performs its own authoritative preflight.
func EstimateVideoCredits(duration, count int, quality string) (cost int, known bool) {
	cost, known, _ = videoJobCost(duration, count, quality)
	return cost, known
}

// ExecutionProgress is implemented by the durable job runner. Engine methods
// report transitions without depending on the service or store packages.
type ExecutionProgress interface {
	SetPhase(phase, message string) error
	SetUpstreamIDs(ids []string) error
}

type progressContextKey struct{}

// WithExecutionProgress attaches a durable progress reporter to an engine call.
func WithExecutionProgress(ctx context.Context, progress ExecutionProgress) context.Context {
	return context.WithValue(ctx, progressContextKey{}, progress)
}

func reportPhase(ctx context.Context, phase, message string) {
	if progress, ok := ctx.Value(progressContextKey{}).(ExecutionProgress); ok && progress != nil {
		if err := progress.SetPhase(phase, message); err != nil {
			// Progress persistence must not turn an already accepted upstream call
			// into a generation retry. The queue lease still protects ownership.
		}
	}
}

func reportUpstreamIDs(ctx context.Context, ids []string) {
	if progress, ok := ctx.Value(progressContextKey{}).(ExecutionProgress); ok && progress != nil {
		_ = progress.SetUpstreamIDs(ids)
	}
}
