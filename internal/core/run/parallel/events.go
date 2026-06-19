package parallelrun

import (
	"context"
	"strings"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// Phase labels carried by TaskParallelPayload.Phase. They mirror the FSM states
// the operator-facing pane renders.
const (
	parallelPhaseRunning   = "running"
	parallelPhaseMerging   = "merging"
	parallelPhaseResolving = "resolving"
	parallelPhaseMerged    = "merged"
)

// ParallelEventEmitter publishes task.parallel.* lifecycle events for one parallel
// run. Implementations must be safe for use from the orchestrator goroutine and
// must never block the FSM: event delivery is observability, not control flow, so
// emit failures are absorbed (logged) by the implementation rather than returned.
type ParallelEventEmitter interface {
	EmitParallelEvent(ctx context.Context, kind events.EventKind, payload kinds.TaskParallelPayload)
}

// noopEventEmitter is the default emitter used when no observability sink is wired
// (unit tests, library callers without a journal).
type noopEventEmitter struct{}

func (noopEventEmitter) EmitParallelEvent(context.Context, events.EventKind, kinds.TaskParallelPayload) {
}

var _ ParallelEventEmitter = noopEventEmitter{}

// emit publishes one parallel event, defaulting RunID/IntegrationBranch from the
// plan so call sites stay terse.
func (o *ExecutionOrchestrator) emit(
	ctx context.Context,
	plan ParallelPlan,
	kind events.EventKind,
	payload kinds.TaskParallelPayload,
) {
	if o == nil || o.emitter == nil {
		return
	}
	if payload.RunID == "" {
		payload.RunID = strings.TrimSpace(plan.RunID)
	}
	if payload.IntegrationBranch == "" {
		payload.IntegrationBranch = strings.TrimSpace(plan.IntegrationBranch)
	}
	o.emitter.EmitParallelEvent(ctx, kind, payload)
}

// emitWaveStarted announces one task entering a running wave so the TUI can assign
// its sidebar card to the wave and mark the wave running.
func (o *ExecutionOrchestrator) emitWaveStarted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
	task TaskSpec,
) {
	o.emit(ctx, plan, events.EventKindTaskParallelWaveStarted, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		WaveTotal: waveTotal,
		TaskID:    string(task.ID),
		Phase:     parallelPhaseRunning,
	})
}

// emitMergeStarted announces a wave entering its serial squash-merge phase.
func (o *ExecutionOrchestrator) emitMergeStarted(ctx context.Context, plan ParallelPlan, waveIndex, waveTotal int) {
	o.emit(ctx, plan, events.EventKindTaskParallelMergeStarted, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		WaveTotal: waveTotal,
		Phase:     parallelPhaseMerging,
	})
}

// emitConflictDetected announces a squash merge that produced unmerged files.
func (o *ExecutionOrchestrator) emitConflictDetected(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex int,
	task TaskSpec,
	conflicts ConflictSet,
	attempt, maxAttempts int,
) {
	o.emit(ctx, plan, events.EventKindTaskParallelConflictDetected, kinds.TaskParallelPayload{
		WaveIndex:     waveIndex,
		TaskID:        string(task.ID),
		ConflictFiles: normalizedConflictFiles(conflicts.Files),
		Attempt:       attempt,
		MaxAttempts:   maxAttempts,
	})
}

// emitConflictResolving announces the bounded resolver starting work on a conflict.
func (o *ExecutionOrchestrator) emitConflictResolving(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex int,
	task TaskSpec,
	conflicts ConflictSet,
	attempt, maxAttempts int,
) {
	o.emit(ctx, plan, events.EventKindTaskParallelConflictResolving, kinds.TaskParallelPayload{
		WaveIndex:     waveIndex,
		TaskID:        string(task.ID),
		ConflictFiles: normalizedConflictFiles(conflicts.Files),
		Attempt:       attempt,
		MaxAttempts:   maxAttempts,
		Phase:         parallelPhaseResolving,
	})
}

// emitTaskOutcome announces one task reaching a terminal per-task status (merged,
// recovered, failed, or skipped).
func (o *ExecutionOrchestrator) emitTaskOutcome(ctx context.Context, plan ParallelPlan, outcome TaskOutcome) {
	o.emit(ctx, plan, events.EventKindTaskParallelMerged, kinds.TaskParallelPayload{
		WaveIndex:    outcome.WaveIndex,
		TaskID:       string(outcome.Task.ID),
		WorktreePath: strings.TrimSpace(outcome.WorktreePath),
		Status:       string(outcome.Status),
		Phase:        parallelPhaseMerged,
	})
}

// emitWaveCompleted announces a wave finishing its run+merge phases.
func (o *ExecutionOrchestrator) emitWaveCompleted(ctx context.Context, plan ParallelPlan, waveIndex, waveTotal int) {
	o.emit(ctx, plan, events.EventKindTaskParallelWaveCompleted, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		WaveTotal: waveTotal,
	})
}

// emitRolledBack announces an atomic rollback: the integration branch is discarded
// and the working branch is left untouched.
func (o *ExecutionOrchestrator) emitRolledBack(ctx context.Context, plan ParallelPlan, waveIndex int) {
	o.emit(ctx, plan, events.EventKindTaskParallelRolledBack, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
	})
}
