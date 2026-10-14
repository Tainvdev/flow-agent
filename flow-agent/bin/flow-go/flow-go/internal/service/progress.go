package service

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/kodelyx/flow-go/flow-go/internal/store"
)

// AttemptReporter records durable execution progress without exposing the
// queue's storage details to the account scheduler.
type AttemptReporter interface {
	SetPhase(phase, message string) error
	SetUpstreamIDs(ids []string) error
	BindAccount(accountID, projectID, sessionRevision string) error
}

// ProgressRunner may remain in the safe preparing phase while it selects and
// refreshes an account, then mark execution as started immediately before its
// first operation that may submit work upstream.
type ProgressRunner interface {
	RunWithProgress(context.Context, store.Job, AttemptReporter) (any, error)
}

// ResumeRunner continues accepted upstream media from an expired durable lease.
// It must never submit another generation request for the same job.
type ResumeRunner interface {
	ResumeWithProgress(context.Context, store.Job, store.JobAttempt, AttemptReporter) (any, error)
}

// ParentExpander turns account fan-out and image count into queue-owned children.
type ParentExpander interface {
	ExpandForQueue(context.Context, store.Job) ([]store.Job, bool, error)
}

// JobPayloadValidator rejects invalid requests before they consume durable
// queue capacity. Validation must be local and must not perform upstream work.
type JobPayloadValidator interface {
	ValidateJobPayload(context.Context, store.Job) error
}

type jobAttemptProgress struct {
	store     *store.Store
	jobID     string
	attemptNo int
}

// submissionGateReporter releases the service-wide submission permit as soon
// as an upstream media ID is durably observable. The account scheduler slot is
// still held by runOne until rendering/download reaches a terminal state.
type submissionGateReporter struct {
	next    AttemptReporter
	release func()
	once    sync.Once
}

func (p *submissionGateReporter) SetPhase(phase, message string) error {
	err := p.next.SetPhase(phase, message)
	if phase == "media_available" {
		p.once.Do(p.release)
	}
	return err
}

func (p *submissionGateReporter) SetUpstreamIDs(ids []string) error {
	return p.next.SetUpstreamIDs(ids)
}

func (p *submissionGateReporter) BindAccount(accountID, projectID, revision string) error {
	return p.next.BindAccount(accountID, projectID, revision)
}

func (p *jobAttemptProgress) SetPhase(phase, message string) error {
	return p.store.UpdateJobAttempt(context.Background(), p.jobID, p.attemptNo, phase, "", "", "", message)
}

func (p *jobAttemptProgress) SetUpstreamIDs(ids []string) error {
	return p.store.SetJobAttemptUpstreamIDs(context.Background(), p.jobID, p.attemptNo, ids)
}

func (p *jobAttemptProgress) BindAccount(accountID, projectID, revision string) error {
	return p.store.UpdateJobAttempt(context.Background(), p.jobID, p.attemptNo, "preparing", accountID, projectID, revision, "")
}

func (p *jobAttemptProgress) Complete(result any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return p.store.CompleteJobAttempt(context.Background(), p.jobID, p.attemptNo, string(encoded))
}

func (p *jobAttemptProgress) Fail(phase, message string) error {
	return p.store.FailJobAttempt(context.Background(), p.jobID, p.attemptNo, phase, message)
}
