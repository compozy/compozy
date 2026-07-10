package parallelrun

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

// Phase labels carried by TaskParallelPayload.Phase. They mirror the FSM states
// the operator-facing pane renders.
const (
	parallelPhaseRunning          = "running"
	parallelPhaseMerging          = "merging"
	parallelPhaseResolving        = "resolving"
	parallelPhaseMerged           = "merged"
	parallelPhaseAdvancingBase    = "advancing_base"
	parallelPhaseFinalizing       = "finalizing"
	parallelPhaseFastForwarding   = "fast_forwarding"
	parallelPhaseSyncingArtifacts = "syncing_artifacts"
	parallelPhaseCleaningUp       = "cleaning_up"
	parallelPhaseCompleted        = "completed"
	parallelPhaseCanceled         = "canceled"
	parallelPhaseFailed           = "failed"
	parallelPhaseRolledBack       = "rolled_back"
)

// ParallelEventEmitter publishes task.parallel.* lifecycle events for one parallel
// run. Implementations must be safe for use from the orchestrator goroutine.
// Lifecycle delivery is part of the run contract, so implementations return
// persistence errors to the orchestrator rather than absorbing them.
type ParallelEventEmitter interface {
	EmitParallelPlanEvent(ctx context.Context, payload kinds.TaskParallelPlanPayload) error
	EmitParallelEvent(ctx context.Context, kind events.EventKind, payload kinds.TaskParallelPayload) error
}

// noopEventEmitter is the default emitter used when no observability sink is wired
// (unit tests, library callers without a journal).
type noopEventEmitter struct{}

func (noopEventEmitter) EmitParallelPlanEvent(context.Context, kinds.TaskParallelPlanPayload) error {
	return nil
}

func (noopEventEmitter) EmitParallelEvent(context.Context, events.EventKind, kinds.TaskParallelPayload) error {
	return nil
}

var _ ParallelEventEmitter = noopEventEmitter{}

// emit publishes one parallel event, defaulting RunID/IntegrationBranch from the
// plan so call sites stay terse.
func (o *ExecutionOrchestrator) emit(
	ctx context.Context,
	plan ParallelPlan,
	kind events.EventKind,
	payload kinds.TaskParallelPayload,
) error {
	if o == nil || o.emitter == nil {
		return nil
	}
	if payload.RunID == "" {
		payload.RunID = strings.TrimSpace(plan.RunID)
	}
	if payload.IntegrationBranch == "" {
		payload.IntegrationBranch = strings.TrimSpace(plan.IntegrationBranch)
	}
	if err := o.emitter.EmitParallelEvent(ctx, kind, payload); err != nil {
		return fmt.Errorf("emit %s: %w", kind, err)
	}
	return nil
}

func (o *ExecutionOrchestrator) emitPlanStarted(ctx context.Context, plan ParallelPlan) error {
	if o == nil || o.emitter == nil {
		return nil
	}
	taskWave := make(map[TaskID]int)
	levels := plan.Waves.Levels()
	waves := make([]kinds.TaskParallelPlanWave, 0, len(levels))
	for waveIndex, level := range levels {
		wave := kinds.TaskParallelPlanWave{Index: waveIndex, TaskIDs: make([]string, 0, len(level))}
		for _, taskID := range level {
			taskWave[taskID] = waveIndex
			wave.TaskIDs = append(wave.TaskIDs, string(taskID))
		}
		waves = append(waves, wave)
	}
	dependencies := dependenciesByTask(plan.Waves)
	tasks := make([]kinds.TaskParallelPlanTask, 0, len(plan.Tasks))
	orderedTasks := append([]TaskSpec(nil), plan.Tasks...)
	sort.SliceStable(orderedTasks, func(i, j int) bool {
		return orderedTasks[i].Number < orderedTasks[j].Number
	})
	for _, task := range orderedTasks {
		deps := make([]string, 0, len(dependencies[task.ID]))
		for _, dep := range dependencies[task.ID] {
			deps = append(deps, string(dep))
		}
		tasks = append(tasks, kinds.TaskParallelPlanTask{
			ID:           string(task.ID),
			Number:       task.Number,
			Title:        strings.TrimSpace(task.Title),
			File:         fmt.Sprintf("task_%02d.md", task.Number),
			Dependencies: deps,
			WaveIndex:    taskWave[task.ID],
		})
	}
	if err := o.emitter.EmitParallelPlanEvent(ctx, kinds.TaskParallelPlanPayload{
		RunID:             strings.TrimSpace(plan.RunID),
		Workflow:          workflowFromTasks(plan.Tasks),
		IntegrationBranch: strings.TrimSpace(plan.IntegrationBranch),
		ParallelLimit:     maxConcurrency(plan.Config),
		Tasks:             tasks,
		Waves:             waves,
	}); err != nil {
		return fmt.Errorf("emit %s: %w", events.EventKindTaskParallelPlanStarted, err)
	}
	return nil
}

func dependenciesByTask(waves Waves) map[TaskID][]TaskID {
	dependencies := make(map[TaskID][]TaskID)
	for from, successors := range waves.successors {
		for _, to := range successors {
			dependencies[to] = append(dependencies[to], from)
		}
	}
	for taskID := range dependencies {
		sortTaskIDs(dependencies[taskID])
	}
	return dependencies
}

func workflowFromTasks(tasks []TaskSpec) string {
	for _, task := range tasks {
		if slug := strings.TrimSpace(task.Slug); slug != "" {
			return slug
		}
	}
	return ""
}

// emitWaveStarted announces one task entering a running wave so the TUI can assign
// its sidebar card to the wave and mark the wave running.
func (o *ExecutionOrchestrator) emitWaveStarted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
	task TaskSpec,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelWaveStarted, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		WaveTotal: waveTotal,
		TaskID:    string(task.ID),
		Phase:     parallelPhaseRunning,
	})
}

// emitMergeStarted announces a wave entering its serial squash-merge phase.
func (o *ExecutionOrchestrator) emitMergeStarted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelMergeStarted, kinds.TaskParallelPayload{
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
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelConflictDetected, kinds.TaskParallelPayload{
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
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelConflictResolving, kinds.TaskParallelPayload{
		WaveIndex:     waveIndex,
		TaskID:        string(task.ID),
		ConflictFiles: normalizedConflictFiles(conflicts.Files),
		Attempt:       attempt,
		MaxAttempts:   maxAttempts,
		Phase:         parallelPhaseResolving,
	})
}

// emitTaskMerged preserves the legacy specialized signal for successfully
// integrated task worktrees.
func (o *ExecutionOrchestrator) emitTaskMerged(
	ctx context.Context,
	plan ParallelPlan,
	outcome TaskOutcome,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelMerged, kinds.TaskParallelPayload{
		WaveIndex:      outcome.WaveIndex,
		TaskID:         string(outcome.Task.ID),
		WorktreePath:   strings.TrimSpace(outcome.WorktreePath),
		WorktreeStatus: strings.TrimSpace(outcome.WorktreeStatus),
		WorktreeReason: strings.TrimSpace(outcome.WorktreeReason),
		Status:         string(outcome.Status),
		Phase:          parallelPhaseMerged,
	})
}

// emitTaskCompleted announces one task reaching any terminal per-task status.
func (o *ExecutionOrchestrator) emitTaskCompleted(
	ctx context.Context,
	plan ParallelPlan,
	outcome TaskOutcome,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelTaskCompleted, kinds.TaskParallelPayload{
		WaveIndex:      outcome.WaveIndex,
		TaskID:         string(outcome.Task.ID),
		WorktreePath:   strings.TrimSpace(outcome.WorktreePath),
		WorktreeStatus: strings.TrimSpace(outcome.WorktreeStatus),
		WorktreeReason: strings.TrimSpace(outcome.WorktreeReason),
		Status:         string(outcome.Status),
		Error:          strings.TrimSpace(outcome.Error),
	})
}

// emitWaveCompleted announces a wave finishing its run+merge phases.
func (o *ExecutionOrchestrator) emitWaveCompleted(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex, waveTotal int,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelWaveCompleted, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		WaveTotal: waveTotal,
	})
}

// emitPhaseChanged announces an operator-visible finalize transition.
func (o *ExecutionOrchestrator) emitPhaseChanged(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex int,
	phase string,
) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelPhaseChanged, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		Phase:     phase,
	})
}

// emitCompleted announces successful orchestrator settlement.
func (o *ExecutionOrchestrator) emitCompleted(ctx context.Context, plan ParallelPlan) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelCompleted, kinds.TaskParallelPayload{
		Status: string(ParallelOutcomeCompleted),
		Phase:  parallelPhaseCompleted,
	})
}

// emitCanceled announces cancellation settlement.
func (o *ExecutionOrchestrator) emitCanceled(ctx context.Context, plan ParallelPlan, cause error) error {
	message := ""
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	return o.emit(ctx, plan, events.EventKindTaskParallelCanceled, kinds.TaskParallelPayload{
		Status: string(ParallelOutcomeCanceled),
		Error:  message,
		Phase:  parallelPhaseCanceled,
	})
}

// emitRolledBack announces an atomic rollback: the integration branch is discarded
// and the working branch is left untouched.
func (o *ExecutionOrchestrator) emitRolledBack(ctx context.Context, plan ParallelPlan, waveIndex int) error {
	return o.emit(ctx, plan, events.EventKindTaskParallelRolledBack, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		Status:    string(ParallelOutcomeRolledBack),
		Phase:     parallelPhaseRolledBack,
	})
}

// emitFailed announces a non-rollback terminal failure that preserves the
// integration worktree for inspection.
func (o *ExecutionOrchestrator) emitFailed(
	ctx context.Context,
	plan ParallelPlan,
	waveIndex int,
	cause error,
) error {
	message := ""
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	return o.emit(ctx, plan, events.EventKindTaskParallelFailed, kinds.TaskParallelPayload{
		WaveIndex: waveIndex,
		Status:    string(ParallelOutcomeFailed),
		Error:     message,
		Phase:     parallelPhaseFailed,
	})
}
