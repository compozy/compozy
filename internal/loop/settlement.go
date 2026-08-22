package loop

import taskpkg "github.com/compozy/compozy/internal/task"

// TerminalCause is the terminal outcome that determines task settlement.
type TerminalCause string

const (
	TerminalCauseDone       TerminalCause = "done"
	TerminalCauseNoOp       TerminalCause = "no-op"
	TerminalCauseFailed     TerminalCause = "failed"
	TerminalCauseExhausted  TerminalCause = "exhausted"
	TerminalCauseStalled    TerminalCause = "stalled"
	TerminalCauseCanceled   TerminalCause = "canceled"
	TerminalCauseKilled     TerminalCause = "killed"
	TerminalCauseRunMissing TerminalCause = "run_missing"
)

// SettleResult summarizes one atomic Loop execution-record settlement.
type SettleResult struct {
	CellsSettled      int
	RunsCanceled      int
	CoordinatorStatus taskpkg.Status
}
