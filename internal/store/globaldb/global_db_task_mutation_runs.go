package globaldb

import (
	"context"
	"fmt"
	"strings"

	redactpkg "github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (g *TaskRepo) transitionTerminalRunWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	mutation taskpkg.TerminalRunMutation,
) (taskpkg.Run, error) {
	normalized, err := g.normalizeTaskRunForUpdate(mutation.NextRun())
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !isTerminalTaskRunStatus(normalized.Status) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: terminal task run mutation requires a terminal status",
			taskpkg.ErrInvalidStatusTransition,
		)
	}

	fence := mutation.Fence()
	if strings.TrimSpace(normalized.ID) != fence.RunID() {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: terminal task run mutation identity changed",
			taskpkg.ErrInvalidStatusTransition,
		)
	}
	if err := fence.Status().Validate("task_run.terminal_mutation.expected_status"); err != nil {
		return taskpkg.Run{}, err
	}
	if strings.TrimSpace(normalized.SessionID) != fence.SessionID() {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q session changed before terminal transition",
			taskpkg.ErrInvalidStatusTransition,
			normalized.ID,
		)
	}

	current, err := g.getTaskRunWithExecutor(ctx, exec, normalized.ID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	if !fence.Matches(current) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q changed before terminal transition",
			taskpkg.ErrInvalidStatusTransition,
			normalized.ID,
		)
	}
	if !allowsTerminalTaskRunTransition(current.Status, normalized.Status) {
		return taskpkg.Run{}, fmt.Errorf(
			"%w: task run %q cannot transition from %q to %q",
			taskpkg.ErrInvalidStatusTransition,
			normalized.ID,
			current.Status.Normalize(),
			normalized.Status.Normalize(),
		)
	}
	if err := taskpkg.ValidateImmutableRunFields(current, normalized); err != nil {
		return taskpkg.Run{}, err
	}
	if err := transitionTerminalRunRecordWithExecutor(ctx, exec, &normalized, fence); err != nil {
		return taskpkg.Run{}, err
	}
	if normalized.IsTaskAnchored() {
		if err := updateTaskCurrentRunProjectionForRunUpdate(ctx, exec, current, normalized); err != nil {
			return taskpkg.Run{}, err
		}
	}

	updated, err := g.getTaskRunWithExecutor(ctx, exec, normalized.ID)
	if err != nil {
		return taskpkg.Run{}, err
	}
	return updated, nil
}

func transitionTerminalRunRecordWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run *taskpkg.Run,
	fence taskpkg.RunMutationFence,
) error {
	affected, err := sqlcgen.New(exec).TransitionTerminalTaskRun(
		ctx,
		sqlcgen.TransitionTerminalTaskRunParams{
			Status:                 run.Status.String(),
			EndedAt:                nullableTaskTime(run.EndedAt),
			Error:                  nullableTaskString(run.Error),
			ResultJson:             nullableTaskRawJSON(run.ResultValue()),
			ID:                     run.ID,
			ExpectedTaskID:         nullableTaskString(fence.TaskID()),
			ExpectedWorkspaceID:    nullableTaskString(fence.WorkspaceID()),
			ExpectedStatus:         fence.Status().String(),
			ExpectedSessionID:      nullableTaskString(fence.SessionID()),
			ExpectedClaimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
			ExpectedLeaseUntil:     nullableTaskTime(fence.LeaseUntil().UTC()),
		},
	)
	if err != nil {
		return fmt.Errorf("store: transition terminal task run %q: %w", run.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf(
			"%w: task run %q no longer matches terminal transition fence",
			taskpkg.ErrInvalidStatusTransition,
			run.ID,
		)
	}
	return nil
}

func isTerminalTaskRunStatus(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusCompleted, taskpkg.TaskRunStatusFailed, taskpkg.TaskRunStatusCanceled:
		return true
	default:
		return false
	}
}

func allowsTerminalTaskRunTransition(current taskpkg.RunStatus, next taskpkg.RunStatus) bool {
	switch current.Normalize() {
	case taskpkg.TaskRunStatusQueued, taskpkg.TaskRunStatusClaimed:
		return next.Normalize() == taskpkg.TaskRunStatusCanceled
	case taskpkg.TaskRunStatusStarting:
		return next.Normalize() == taskpkg.TaskRunStatusFailed || next.Normalize() == taskpkg.TaskRunStatusCanceled
	case taskpkg.TaskRunStatusRunning:
		switch next.Normalize() {
		case taskpkg.TaskRunStatusCompleted, taskpkg.TaskRunStatusFailed, taskpkg.TaskRunStatusCanceled:
			return true
		}
	}
	return false
}

func (g *TaskRunRepo) markTaskRunNeedsAttentionMutationWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	command taskpkg.RunNeedsAttentionCommand,
) (taskpkg.RunNeedsAttentionMutation, error) {
	diagnostic := command.Diagnostic()
	if err := validateNeedsAttentionDiagnostic(diagnostic); err != nil {
		return taskpkg.RunNeedsAttentionMutation{}, err
	}
	fence := command.Fence()
	id, err := requireTaskValue(fence.RunID(), "task run id")
	if err != nil {
		return taskpkg.RunNeedsAttentionMutation{}, err
	}
	previous, err := g.tasks.getTaskRunWithExecutor(ctx, exec, id)
	if err != nil {
		return taskpkg.RunNeedsAttentionMutation{}, err
	}
	if !fence.Matches(previous) {
		return taskpkg.RunNeedsAttentionMutation{}, fmt.Errorf(
			"%w: task run %q changed before needs_attention applied",
			taskpkg.ErrInvalidStatusTransition,
			id,
		)
	}
	if previous.Status.Normalize() == taskpkg.TaskRunStatusNeedsAttention {
		return taskpkg.RunNeedsAttentionMutation{Previous: previous, Run: previous}, nil
	}
	if !runStatusAllowsNeedsAttentionForMutation(previous.Status) {
		return taskpkg.RunNeedsAttentionMutation{}, fmt.Errorf(
			"%w: task run %q is not nonterminal",
			taskpkg.ErrInvalidStatusTransition,
			id,
		)
	}
	if err := g.persistTaskRunNeedsAttention(ctx, exec, previous, fence, diagnostic); err != nil {
		return taskpkg.RunNeedsAttentionMutation{}, err
	}
	updated, err := g.tasks.getTaskRunWithExecutor(ctx, exec, id)
	if err != nil {
		return taskpkg.RunNeedsAttentionMutation{}, err
	}
	if updated.IsLoopWorker() {
		if err := projectLoopTaskRunAttentionWithExecutor(
			ctx,
			exec,
			updated,
			diagnostic,
			command.Now(),
		); err != nil {
			return taskpkg.RunNeedsAttentionMutation{}, err
		}
	}
	return taskpkg.RunNeedsAttentionMutation{Previous: previous, Run: updated, Applied: true}, nil
}

func (g *TaskRunRepo) persistTaskRunNeedsAttention(
	ctx context.Context,
	exec taskSQLExecutor,
	previous taskpkg.Run,
	fence taskpkg.RunMutationFence,
	diagnostic string,
) error {
	candidate := previous
	candidate.Status = taskpkg.TaskRunStatusNeedsAttention
	candidate.Error = diagnostic
	if err := candidate.Validate(); err != nil {
		return err
	}
	affected, err := sqlcgen.New(exec).MarkTaskRunNeedsAttention(
		ctx,
		sqlcgen.MarkTaskRunNeedsAttentionParams{
			Error:                  nullableTaskString(diagnostic),
			ID:                     previous.ID,
			ExpectedTaskID:         nullableTaskString(fence.TaskID()),
			ExpectedWorkspaceID:    nullableTaskString(fence.WorkspaceID()),
			ExpectedStatus:         fence.Status().String(),
			ExpectedSessionID:      nullableTaskString(fence.SessionID()),
			ExpectedClaimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
			ExpectedLeaseUntil:     nullableTaskTime(fence.LeaseUntil().UTC()),
		},
	)
	if err != nil {
		return fmt.Errorf("store: mark task run needs attention: %w", err)
	}
	if affected != 0 {
		return nil
	}
	if _, err := g.tasks.getTaskRunWithExecutor(ctx, exec, previous.ID); err != nil {
		return err
	}
	return fmt.Errorf(
		"%w: task run %q changed before needs_attention applied",
		taskpkg.ErrInvalidStatusTransition,
		previous.ID,
	)
}

func validateNeedsAttentionDiagnostic(diagnostic string) error {
	if redactpkg.ContainsRawClaimToken(diagnostic) {
		return fmt.Errorf(
			"%w: task run needs_attention diagnostic must not embed a claim token",
			taskpkg.ErrValidation,
		)
	}
	return taskpkg.ValidateDiagnosticSize(diagnostic, "task_run.needs_attention.diagnostic")
}

func runStatusAllowsNeedsAttentionForMutation(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusQueued,
		taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return true
	default:
		return false
	}
}
