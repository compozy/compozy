package recovery

import "context"

// PreparedRun is the execution seam driven by RunRecoveryOrchestrator.
type PreparedRun interface {
	Execute(ctx context.Context) (RunOutcome, error)
	RestartFailed(ctx context.Context, failedJobIDs []string) (RunOutcome, error)
}
