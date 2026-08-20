package loop

import (
	"context"

	taskpkg "github.com/compozy/compozy/internal/task"
)

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

// SweepReport summarizes one reconciliation pass.
type SweepReport struct {
	RunsExamined    int
	RecordsSettled  int
	OrphansRepaired int
}

// ReconciliationStore owns durable reconciliation writes.
type ReconciliationStore interface {
	NeutralizeLoopRunOrphans(context.Context) (SweepReport, error)
	SweepLoopRunOrphans(context.Context) (SweepReport, error)
	BackfillLoopProvenance(context.Context) (int, error)
}

// RunReconciler converges Loop execution records without claiming work.
type RunReconciler interface {
	NeutralizeOrphans(context.Context) (SweepReport, error)
	SweepOnce(context.Context) (SweepReport, error)
	BackfillProvenance(context.Context) (int, error)
}

type runReconciler struct {
	store ReconciliationStore
}

// NewRunReconciler constructs the reconciliation boundary over its durable store.
func NewRunReconciler(store ReconciliationStore) RunReconciler {
	return &runReconciler{store: store}
}

func (r *runReconciler) NeutralizeOrphans(ctx context.Context) (SweepReport, error) {
	return r.store.NeutralizeLoopRunOrphans(ctx)
}

func (r *runReconciler) SweepOnce(ctx context.Context) (SweepReport, error) {
	return r.store.SweepLoopRunOrphans(ctx)
}

func (r *runReconciler) BackfillProvenance(ctx context.Context) (int, error) {
	return r.store.BackfillLoopProvenance(ctx)
}
