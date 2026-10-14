package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/kodelyx/flow-go/flow-go/internal/pool"
)

type rejectedSubmission struct{}

func (rejectedSubmission) Error() string { return "unusual activity" }

func TestAccountThrottleErrorIsSafeAndClassified(t *testing.T) {
	cause := rejectedSubmission{}
	err := &AccountThrottleError{Cause: cause}
	var safe interface{ SafeToRetry() bool }
	var classifier interface{ RetryClass() string }
	var throttle interface{ Throttled() bool }
	var unwrapped rejectedSubmission
	if !errors.As(err, &safe) || !safe.SafeToRetry() {
		t.Fatal("account throttle must be safe to retry after upstream rejection")
	}
	if !errors.As(err, &classifier) || classifier.RetryClass() != "account_throttle" {
		t.Fatal("account throttle should have its own retry budget")
	}
	if !errors.As(err, &throttle) || !throttle.Throttled() {
		t.Fatal("scheduler must recognize this error and cool down the account")
	}
	if !errors.As(err, &unwrapped) {
		t.Fatal("account throttle should preserve its rejection cause")
	}
}

func TestAccountThrottleErrorCoolsSchedulerWorker(t *testing.T) {
	scheduler := pool.New()
	worker := pool.NewWorker("account-a", nil)
	scheduler.Register(worker)
	if _, err := scheduler.Acquire(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	scheduler.Release(worker, 0, &AccountThrottleError{Cause: rejectedSubmission{}})
	if worker.State() != pool.StateCircuitOpen {
		t.Fatalf("worker state=%s, want account cooldown", worker.State())
	}
}
