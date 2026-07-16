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
	JobPhaseStalled   JobPhase = "stalled"
	JobPhaseRetrying  JobPhase = "retrying"
	JobPhaseSucceeded JobPhase = "succeeded"
	JobPhaseFailed    JobPhase = "failed"
	JobPhaseCanceled  JobPhase = "canceled"
	JobPhaseParked    JobPhase = "parked"
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
	// Stalled marks an attempt the stall watchdog canceled because the agent went
	// silent, as opposed to an ordinary retryable failure. Only a stalled attempt
	// draws on the stall-retry budget and can end in a parked job.
	Stalled bool
	// LastToolCall identifies the tool call the agent was executing when it went
	// silent. Populated on stalled attempts to give a parked job triage context.
	LastToolCall string
}

// ReusableAgentExecution carries reusable-agent metadata needed for runtime
// observability once the job prompt and MCP servers are fully prepared.
type ReusableAgentExecution struct {
	Name                string
	Source              string
	AvailableAgentCount int
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

// IsStalled reports whether the attempt ended because the agent stopped making
// progress. Stalled attempts are always retryable, but they take the stall
// recovery path (clean-state retry, then park) instead of the ordinary one.
func (r JobAttemptResult) IsStalled() bool {
	return r.Stalled
}

func AtLeastOne(value int) int {
	if value < 1 {
		return 1
	}
	return value
}
