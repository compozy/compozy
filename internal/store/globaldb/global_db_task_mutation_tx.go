package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type taskMutationTxStore struct {
	*taskExecutionTxStore
}

var _ taskpkg.RunMutationTransactionStore = (*TaskRepo)(nil)
var _ taskpkg.RunMutationTransactionStore = (*GlobalDB)(nil)

// WithTaskMutationTransaction runs one authoritative task mutation under an
// immediate writer transaction and publishes collected events only after commit.
func (g *TaskRepo) WithTaskMutationTransaction(
	ctx context.Context,
	action string,
	mutation taskpkg.RunMutation,
) error {
	if mutation == nil {
		return fmt.Errorf("%w: task mutation is required", taskpkg.ErrValidation)
	}
	if err := g.checkReady(ctx, action); err != nil {
		return err
	}
	return g.withTaskImmediateTransaction(ctx, action, func(exec taskSQLExecutor) error {
		return mutation(&taskMutationTxStore{
			taskExecutionTxStore: &taskExecutionTxStore{
				deleteTaskTxStore: &deleteTaskTxStore{tasks: g, exec: exec},
			},
		})
	})
}

func (s *taskMutationTxStore) TransitionTerminalRun(
	ctx context.Context,
	mutation taskpkg.TerminalRunMutation,
) (taskpkg.Run, error) {
	return s.tasks.transitionTerminalRunWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) SettleCompletedTaskHierarchy(
	ctx context.Context,
	completedTaskID string,
	actor taskpkg.ActorContext,
	settledAt time.Time,
) (taskpkg.CompletedRunSettlement, error) {
	return s.tasks.settleCompletedTaskHierarchyWithExecutor(
		ctx,
		s.exec,
		completedTaskID,
		actor,
		settledAt,
	)
}

func (s *taskMutationTxStore) ConsumeTerminalRunCommand(
	ctx context.Context,
	command taskpkg.TerminalRunCommand,
	expectedPhase taskpkg.TerminalRunCommandPhase,
) (taskpkg.TerminalRunCommand, error) {
	row, err := sqlcgen.New(s.exec).ConsumeTaskRunTerminalCommand(
		ctx,
		sqlcgen.ConsumeTaskRunTerminalCommandParams{
			CommandID: command.CommandID(), RunID: command.RunID(),
			TaskID: command.TaskID(), WorkspaceID: command.WorkspaceID(),
			ExpectedPhase: string(expectedPhase),
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf(
			"%w: task run %q",
			taskpkg.ErrTerminalRunCommandInProgress,
			command.RunID(),
		)
	}
	if err != nil {
		return taskpkg.TerminalRunCommand{}, fmt.Errorf(
			"store: consume terminal command for run %q: %w",
			command.RunID(),
			err,
		)
	}
	consumed, err := taskRunTerminalCommandFromRow(row)
	if err != nil {
		return taskpkg.TerminalRunCommand{}, err
	}
	return consumed, nil
}

func (s *taskMutationTxStore) AdmitRunDirectExecution(
	ctx context.Context,
	mutation taskpkg.RunDirectExecutionAdmissionMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.admitRunDirectExecutionWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) TransitionRunStarting(
	ctx context.Context,
	mutation taskpkg.RunStartingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.transitionRunStartingWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) BindRunSession(
	ctx context.Context,
	mutation taskpkg.RunSessionBindingMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.bindRunSessionWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) TransitionRunRunning(
	ctx context.Context,
	mutation taskpkg.RunRunningMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.transitionRunRunningWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) RecoverTaskRunOnBoot(
	ctx context.Context,
	mutation taskpkg.RunBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.recoverTaskRunOnBootWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) RecoverNetworkWakeOnBoot(
	ctx context.Context,
	mutation taskpkg.NetworkWakeBootRecoveryMutation,
) (taskpkg.NominalRunMutationResult, error) {
	return s.tasks.runs.recoverNetworkWakeOnBootWithExecutor(ctx, s.exec, mutation)
}

func (s *taskMutationTxStore) GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error) {
	return s.tasks.getTaskRunWithExecutor(ctx, s.exec, id)
}

func (s *taskMutationTxStore) MarkRunNeedsAttentionMutation(
	ctx context.Context,
	command taskpkg.RunNeedsAttentionCommand,
) (taskpkg.RunNeedsAttentionMutation, error) {
	return s.tasks.runs.markTaskRunNeedsAttentionMutationWithExecutor(ctx, s.exec, command)
}

func (s *taskMutationTxStore) ForceReleaseTaskRun(
	ctx context.Context,
	release taskpkg.ForceReleaseRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return s.tasks.runs.forceReleaseTaskRunWithExecutor(ctx, s.exec, release)
}

func (s *taskMutationTxStore) ForceFailTaskRun(
	ctx context.Context,
	failure taskpkg.ForceFailRunMutation,
) (taskpkg.ForceRunMutationResult, error) {
	return s.tasks.runs.forceFailTaskRunWithExecutor(ctx, s.exec, failure)
}

func (s *taskMutationTxStore) RetryTaskRun(
	ctx context.Context,
	retry taskpkg.RetryRunMutation,
) (taskpkg.RetryRunResult, error) {
	args, err := normalizeRetryTaskRunArgs(retry, s.tasks.now)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return s.tasks.runs.retryTaskRunWithExecutor(ctx, s.exec, args)
}

func (s *taskMutationTxStore) RecoverTaskRun(
	ctx context.Context,
	mutation taskpkg.RecoverRunMutation,
) (taskpkg.RetryRunResult, error) {
	args, err := normalizeRecoverTaskRunArgs(mutation, s.tasks.now)
	if err != nil {
		return taskpkg.RetryRunResult{}, err
	}
	return s.tasks.runs.recoverTaskRunWithExecutor(ctx, s.exec, args)
}

func (s *taskMutationTxStore) InvalidateForceRunInputs(
	ctx context.Context,
	sessionID string,
	now time.Time,
) (taskpkg.ForceRunInputInvalidation, error) {
	target := strings.TrimSpace(sessionID)
	if target == "" {
		return taskpkg.ForceRunInputInvalidation{}, nil
	}
	if now.IsZero() {
		now = s.tasks.now()
	}
	now = now.UTC()
	queries := sqlcgen.New(s.exec)
	affected, err := queries.AdvanceSessionInputGeneration(ctx, sqlcgen.AdvanceSessionInputGenerationParams{
		UpdatedAt: storepkg.FormatTimestamp(now),
		ID:        target,
	})
	if err != nil {
		return taskpkg.ForceRunInputInvalidation{}, fmt.Errorf("store: advance force-run input generation: %w", err)
	}
	if affected == 0 {
		return taskpkg.ForceRunInputInvalidation{}, fmt.Errorf(
			"%w: %s",
			storepkg.ErrSessionInputQueueEntryNotFound,
			target,
		)
	}
	generation, err := queries.GetSessionInputGeneration(ctx, target)
	if err != nil {
		return taskpkg.ForceRunInputInvalidation{}, fmt.Errorf("store: read force-run input generation: %w", err)
	}
	rows, err := queries.CancelPendingSessionInputs(ctx, sqlcgen.CancelPendingSessionInputsParams{
		CanceledStatus:    storepkg.SessionInputQueueStatusCanceled,
		Now:               nullableSessionTime(now),
		UpdatedAt:         storepkg.FormatTimestamp(now),
		SessionID:         target,
		SessionGeneration: generation,
		QueuedStatus:      storepkg.SessionInputQueueStatusQueued,
		DispatchingStatus: storepkg.SessionInputQueueStatusDispatching,
	})
	if err != nil {
		return taskpkg.ForceRunInputInvalidation{}, fmt.Errorf("store: cancel force-run session inputs: %w", err)
	}
	return taskpkg.ForceRunInputInvalidation{
		QueueGeneration: generation,
		CanceledInputs:  int(rows),
	}, nil
}
