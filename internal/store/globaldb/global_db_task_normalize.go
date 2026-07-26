package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func (g *TaskRepo) normalizeTaskForCreate(record taskpkg.Task) (taskpkg.Task, error) {
	normalized := normalizeTaskRecord(record)
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = g.now()
	}
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.Task{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskForUpdate(record taskpkg.Task) (taskpkg.Task, error) {
	normalized := normalizeTaskRecord(record)
	if normalized.UpdatedAt.IsZero() {
		normalized.UpdatedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.Task{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskRunForCreate(run taskpkg.Run) (taskpkg.Run, error) {
	normalized := normalizeTaskRunRecord(run)
	if normalized.Attempt == 0 {
		normalized.Attempt = 1
	}
	if normalized.QueuedAt.IsZero() {
		normalized.QueuedAt = g.now()
	}
	if err := normalized.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	return normalized, nil
}

func (g *TaskRepo) normalizeTaskRunForUpdate(run taskpkg.Run) (taskpkg.Run, error) {
	normalized := normalizeTaskRunRecord(run)
	if err := normalized.Validate(); err != nil {
		return taskpkg.Run{}, err
	}
	return normalized, nil
}

func insertTaskRunWithExecutor(ctx context.Context, exec taskSQLExecutor, run taskpkg.Run) error {
	params, err := taskRunParams(run)
	if err != nil {
		return err
	}
	if err := sqlcgen.New(exec).InsertTaskRun(ctx, params); err != nil {
		return fmt.Errorf("store: create task run %q: %w", run.ID, err)
	}
	return nil
}

func replaceTaskRunCapabilitiesWithExecutor(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) error {
	queries := sqlcgen.New(exec)
	if err := queries.DeleteRequiredTaskRunCapabilities(ctx, run.ID); err != nil {
		return fmt.Errorf("store: delete required capability rows for task run %q: %w", run.ID, err)
	}
	if err := queries.DeletePreferredTaskRunCapabilities(ctx, run.ID); err != nil {
		return fmt.Errorf("store: delete preferred capability rows for task run %q: %w", run.ID, err)
	}
	for _, capabilityID := range run.RequiredCapabilities {
		if err := queries.InsertRequiredTaskRunCapability(ctx, sqlcgen.InsertRequiredTaskRunCapabilityParams{
			RunID: run.ID, CapabilityID: capabilityID,
		}); err != nil {
			return fmt.Errorf("store: insert required capability %q for task run %q: %w", capabilityID, run.ID, err)
		}
	}
	for _, capabilityID := range run.PreferredCapabilities {
		if err := queries.InsertPreferredTaskRunCapability(ctx, sqlcgen.InsertPreferredTaskRunCapabilityParams{
			RunID: run.ID, CapabilityID: capabilityID,
		}); err != nil {
			return fmt.Errorf("store: insert preferred capability %q for task run %q: %w", capabilityID, run.ID, err)
		}
	}
	return nil
}

func (g *TaskRepo) loadTaskRunCapabilities(
	ctx context.Context,
	exec taskSQLExecutor,
	run taskpkg.Run,
) (taskpkg.Run, error) {
	runs, err := g.loadTaskRunCapabilitiesForList(ctx, exec, []taskpkg.Run{run})
	if err != nil {
		return taskpkg.Run{}, err
	}
	if len(runs) == 0 {
		return taskpkg.Run{}, nil
	}
	return runs[0], nil
}

func (g *TaskRepo) loadTaskRunCapabilitiesForList(
	ctx context.Context,
	exec taskSQLExecutor,
	runs []taskpkg.Run,
) ([]taskpkg.Run, error) {
	if len(runs) == 0 {
		return runs, nil
	}
	runIDs := make([]string, 0, len(runs))
	indexByRunID := make(map[string]int, len(runs))
	for idx := range runs {
		runID := strings.TrimSpace(runs[idx].ID)
		runIDs = append(runIDs, runID)
		indexByRunID[runID] = idx
	}
	required, err := requiredTaskRunCapabilityRows(ctx, exec, runIDs)
	if err != nil {
		return nil, err
	}
	preferred, err := preferredTaskRunCapabilityRows(ctx, exec, runIDs)
	if err != nil {
		return nil, err
	}
	for runID, idx := range indexByRunID {
		runs[idx].RequiredCapabilities = required[runID]
		runs[idx].PreferredCapabilities = preferred[runID]
	}
	return runs, nil
}

func requiredTaskRunCapabilityRows(
	ctx context.Context,
	exec taskSQLExecutor,
	runIDs []string,
) (map[string][]string, error) {
	rows, err := sqlcgen.New(exec).ListRequiredTaskRunCapabilities(ctx, runIDs)
	if err != nil {
		return nil, fmt.Errorf("store: query required capability rows: %w", err)
	}
	result := make(map[string][]string, len(runIDs))
	for _, row := range rows {
		result[row.RunID] = append(result[row.RunID], row.CapabilityID)
	}
	return result, nil
}

func preferredTaskRunCapabilityRows(
	ctx context.Context,
	exec taskSQLExecutor,
	runIDs []string,
) (map[string][]string, error) {
	rows, err := sqlcgen.New(exec).ListPreferredTaskRunCapabilities(ctx, runIDs)
	if err != nil {
		return nil, fmt.Errorf("store: query preferred capability rows: %w", err)
	}
	result := make(map[string][]string, len(runIDs))
	for _, row := range rows {
		result[row.RunID] = append(result[row.RunID], row.CapabilityID)
	}
	return result, nil
}

func (g *TaskRepo) ensureTaskCreateReferences(ctx context.Context, record taskpkg.Task) error {
	if err := taskpkg.ValidateScopeBinding(record.Scope, record.WorkspaceID, "task", "workspace_id"); err != nil {
		return err
	}
	if record.Scope == taskpkg.ScopeWorkspace {
		if err := g.ensureWorkspaceExists(ctx, record.WorkspaceID); err != nil {
			return err
		}
	}
	if strings.TrimSpace(record.ParentTaskID) != "" {
		if err := g.ensureTaskExists(ctx, record.ParentTaskID); err != nil {
			return err
		}
	}
	return nil
}

func (g *TaskRepo) ensureWorkspaceExists(ctx context.Context, workspaceID string) error {
	trimmedID := strings.TrimSpace(workspaceID)
	if trimmedID == "" {
		return nil
	}

	if _, err := g.queries.GetWorkspaceID(ctx, trimmedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return aghworkspace.ErrWorkspaceNotFound
		}
		return fmt.Errorf("store: lookup workspace %q: %w", trimmedID, err)
	}
	return nil
}

func (g *TaskRepo) ensureTaskExists(ctx context.Context, taskID string) error {
	trimmedID := strings.TrimSpace(taskID)
	if trimmedID == "" {
		return taskpkg.ErrTaskNotFound
	}

	if _, err := g.queries.GetTaskID(ctx, trimmedID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return taskpkg.ErrTaskNotFound
		}
		return fmt.Errorf("store: lookup task %q: %w", trimmedID, err)
	}
	return nil
}
