package engine

import "fmt"

// AccountThrottleError means Flow rejected a submission before accepting work.
// The service scheduler can safely retry it and select another account.
type AccountThrottleError struct {
	Cause error
}

func (e *AccountThrottleError) Error() string {
	return fmt.Sprintf("engine: account throttled before submission: %v", e.Cause)
}

func (e *AccountThrottleError) Unwrap() error      { return e.Cause }
func (e *AccountThrottleError) SafeToRetry() bool  { return true }
func (e *AccountThrottleError) RetryClass() string { return "account_throttle" }
func (e *AccountThrottleError) Throttled() bool    { return true }

// UploadOutcomeUnknown marks a resumable video data upload whose response did
// not establish whether Flow finalized the media. It is terminal to automatic
// retry so the queue cannot silently create a second copy of a large upload.
type UploadOutcomeUnknown struct{ Cause error }

func (e *UploadOutcomeUnknown) Error() string {
	return fmt.Sprintf("engine: video upload outcome is unknown: %v", e.Cause)
}

func (e *UploadOutcomeUnknown) Unwrap() error           { return e.Cause }
func (e *UploadOutcomeUnknown) SubmissionUnknown() bool { return true }

// UploadThrottleError cools an account on an explicit HTTP 429 during either
// half of the resumable video upload. It intentionally does not implement
// SafeToRetry: the data step may have transferred bytes before the refusal.
type UploadThrottleError struct {
	Status int
	Body   string
}

func (e *UploadThrottleError) Error() string {
	return fmt.Sprintf("engine: video upload was rate limited: HTTP %d %s", e.Status, e.Body)
}

func (e *UploadThrottleError) Throttled() bool { return e.Status == 429 }
