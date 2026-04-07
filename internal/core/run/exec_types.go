package run

import "time"

const (
	exitCodeTimeout       = -2
	exitCodeCanceled      = -1
	activityCheckInterval = 5 * time.Second
)

type failInfo struct {
	codeFile string
	exitCode int
	outLog   string
	errLog   string
	err      error
}

type jobPhase string

const (
	jobPhaseQueued    jobPhase = "queued"
	jobPhaseScheduled jobPhase = "scheduled"
	jobPhaseRunning   jobPhase = "running"
	jobPhaseRetrying  jobPhase = "retrying"
	jobPhaseSucceeded jobPhase = "succeeded"
	jobPhaseFailed    jobPhase = "failed"
	jobPhaseCanceled  jobPhase = "canceled"
)

type jobAttemptStatus string

const (
	attemptStatusSuccess     jobAttemptStatus = "success"
	attemptStatusFailure     jobAttemptStatus = "failure"
	attemptStatusTimeout     jobAttemptStatus = "timeout"
	attemptStatusCanceled    jobAttemptStatus = "canceled"
	attemptStatusSetupFailed jobAttemptStatus = "setup_failed"
)

type jobAttemptResult struct {
	status    jobAttemptStatus
	exitCode  int
	failure   *failInfo
	retryable bool
}

func (r jobAttemptResult) Successful() bool {
	return r.status == attemptStatusSuccess
}

func (r jobAttemptResult) NeedsRetry() bool {
	return r.retryable
}

func (r jobAttemptResult) IsCanceled() bool {
	return r.status == attemptStatusCanceled
}
