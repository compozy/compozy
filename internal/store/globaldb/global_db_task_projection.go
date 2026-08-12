package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func updateTaskCurrentRunProjectionForRunUpdate(
	ctx context.Context,
	exec taskSQLExecutor,
	current taskpkg.Run,
	next taskpkg.Run,
) error {
	currentActive := taskRunProjectsCurrent(current.Status)
	nextActive := taskRunProjectsCurrent(next.Status)
	currentTaskID := strings.TrimSpace(current.TaskID)
	nextTaskID := strings.TrimSpace(next.TaskID)
	if currentActive && currentTaskID != "" && currentTaskID != nextTaskID {
		if err := clearTaskCurrentRunProjection(ctx, exec, currentTaskID, current.ID); err != nil {
			return err
		}
	}
	if nextActive && nextTaskID != "" {
		return setTaskCurrentRunProjection(ctx, exec, nextTaskID, next.ID)
	}
	if currentActive && currentTaskID != "" {
		return clearTaskCurrentRunProjection(ctx, exec, currentTaskID, current.ID)
	}
	return nil
}

func setTaskCurrentRunProjectionForRun(
	ctx context.Context,
	exec taskSQLExecutor,
	runID string,
) error {
	trimmedRunID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return err
	}

	taskID, err := sqlcgen.New(exec).GetTaskRunTaskID(ctx, trimmedRunID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskRunNotFound
		}
		return fmt.Errorf("store: load task id for current run projection %q: %w", trimmedRunID, err)
	}
	if !taskID.Valid {
		return nil
	}
	return setTaskCurrentRunProjection(ctx, exec, taskID.String, trimmedRunID)
}

func setTaskCurrentRunProjection(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	runID string,
) error {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}
	trimmedRunID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return err
	}

	rowsAffected, err := sqlcgen.New(exec).SetTaskCurrentRunProjection(
		ctx,
		sqlcgen.SetTaskCurrentRunProjectionParams{
			RunID: sql.NullString{String: trimmedRunID, Valid: true}, TaskID: trimmedTaskID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: set current run projection for task %q to %q: %w",
			trimmedTaskID,
			trimmedRunID,
			err,
		)
	}
	if rowsAffected > 0 {
		return nil
	}

	currentRunID, err := currentRunProjection(ctx, exec, trimmedTaskID)
	if err != nil {
		return err
	}
	if currentRunID == trimmedRunID {
		return nil
	}
	if currentRunID == "" {
		return fmt.Errorf(
			"store: set current run projection for task %q to %q matched no rows",
			trimmedTaskID,
			trimmedRunID,
		)
	}
	shared, err := taskRunsShareDesignationGroup(ctx, exec, currentRunID, trimmedRunID)
	if err != nil {
		return err
	}
	if shared {
		return nil
	}
	return fmt.Errorf(
		"%w: task %q already projects active run %q",
		taskpkg.ErrInvalidStatusTransition,
		trimmedTaskID,
		currentRunID,
	)
}

func clearTaskCurrentRunProjection(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	runID string,
) error {
	trimmedTaskID, err := requireTaskValue(taskID, "task id")
	if err != nil {
		return err
	}
	trimmedRunID, err := requireTaskValue(runID, "task run id")
	if err != nil {
		return err
	}

	rowsAffected, err := sqlcgen.New(exec).ClearTaskCurrentRunProjection(
		ctx,
		sqlcgen.ClearTaskCurrentRunProjectionParams{
			TaskID: trimmedTaskID, RunID: sql.NullString{String: trimmedRunID, Valid: true},
		},
	)
	if err != nil {
		return fmt.Errorf(
			"store: clear current run projection for task %q run %q: %w",
			trimmedTaskID,
			trimmedRunID,
			err,
		)
	}
	if rowsAffected > 0 {
		return reprojectActiveDesignationSibling(ctx, exec, trimmedTaskID, trimmedRunID)
	}

	currentRunID, err := currentRunProjection(ctx, exec, trimmedTaskID)
	if err != nil {
		return err
	}
	if currentRunID == "" {
		return nil
	}
	shared, err := taskRunsShareDesignationGroup(ctx, exec, currentRunID, trimmedRunID)
	if err != nil {
		return err
	}
	if shared {
		return nil
	}
	return fmt.Errorf(
		"%w: task %q projects active run %q, not %q",
		taskpkg.ErrInvalidStatusTransition,
		trimmedTaskID,
		currentRunID,
		trimmedRunID,
	)
}

func taskRunsShareDesignationGroup(
	ctx context.Context,
	exec taskSQLExecutor,
	firstRunID string,
	secondRunID string,
) (bool, error) {
	queries := sqlcgen.New(exec)
	first, err := queries.GetTaskRunProjectionIdentity(ctx, firstRunID)
	if err != nil {
		return false, fmt.Errorf("store: load current run projection identity %q: %w", firstRunID, err)
	}
	second, err := queries.GetTaskRunProjectionIdentity(ctx, secondRunID)
	if err != nil {
		return false, fmt.Errorf("store: load candidate run projection identity %q: %w", secondRunID, err)
	}
	firstGroup := strings.TrimSpace(first.DesignationGroupID)
	return firstGroup != "" && firstGroup == strings.TrimSpace(second.DesignationGroupID) &&
		strings.TrimSpace(taskNullStringValue(first.TaskID)) == strings.TrimSpace(taskNullStringValue(second.TaskID)), nil
}

func reprojectActiveDesignationSibling(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
	terminalRunID string,
) error {
	identity, err := sqlcgen.New(exec).GetTaskRunProjectionIdentity(ctx, terminalRunID)
	if err != nil {
		return fmt.Errorf("store: load terminal run projection identity %q: %w", terminalRunID, err)
	}
	groupID := strings.TrimSpace(identity.DesignationGroupID)
	if groupID == "" {
		return nil
	}
	candidate, err := sqlcgen.New(exec).FindActiveDesignationProjectionCandidate(
		ctx,
		sqlcgen.FindActiveDesignationProjectionCandidateParams{
			TaskID: nullableTaskString(taskID), DesignationGroupID: groupID, ExcludedRunID: terminalRunID,
		},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("store: find active designation projection candidate: %w", err)
	}
	return setTaskCurrentRunProjection(ctx, exec, taskID, candidate)
}

func currentRunProjection(
	ctx context.Context,
	exec taskSQLExecutor,
	taskID string,
) (string, error) {
	current, err := sqlcgen.New(exec).GetTaskCurrentRunProjection(ctx, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", taskpkg.ErrTaskNotFound
		}
		return "", fmt.Errorf("store: load current run projection for task %q: %w", taskID, err)
	}
	if !current.Valid {
		return "", nil
	}
	return strings.TrimSpace(current.String), nil
}

func taskRunProjectsCurrent(status taskpkg.RunStatus) bool {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusClaimed,
		taskpkg.TaskRunStatusStarting,
		taskpkg.TaskRunStatusRunning:
		return true
	default:
		return false
	}
}
