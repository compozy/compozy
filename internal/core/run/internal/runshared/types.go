package runshared

import "time"

const (
	ExitCodeTimeout       = -2
	ExitCodeCanceled      = -1
	ActivityCheckInterval = 5 * time.Second
	RunStatusSucceeded    = "succeeded"
	RunStatusFailed       = "failed"
	RunStatusCanceled     = "canceled"
)

type FailInfo struct {
	CodeFile string
	ExitCode int
	OutLog   string
	ErrLog   string
	Err      error
}

type JobPhase string

const (
	JobPhaseQueued    JobPhase = "queued"
	JobPhaseScheduled JobPhase = "scheduled"
	JobPhaseRunning   JobPhase = "running"
	JobPhaseRetrying  JobPhase = "retrying"
	JobPhaseSucceeded JobPhase = "succeeded"
	JobPhaseFailed    JobPhase = "failed"
	JobPhaseCanceled  JobPhase = "canceled"
)

type JobAttemptStatus string

const (
	AttemptStatusSuccess     JobAttemptStatus = "success"
	AttemptStatusFailure     JobAttemptStatus = "failure"
	AttemptStatusTimeout     JobAttemptStatus = "timeout"
	AttemptStatusCanceled    JobAttemptStatus = "canceled"
	AttemptStatusSetupFailed JobAttemptStatus = "setup_failed"
)

type JobAttemptResult struct {
	Status    JobAttemptStatus
	ExitCode  int
	Failure   *FailInfo
	Retryable bool
}

func (r JobAttemptResult) Successful() bool {
	return r.Status == AttemptStatusSuccess
}

func (r JobAttemptResult) NeedsRetry() bool {
	return r.Retryable
}

func (r JobAttemptResult) IsCanceled() bool {
	return r.Status == AttemptStatusCanceled
}

func AtLeastOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
