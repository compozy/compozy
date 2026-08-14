package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/coordinator"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (r *coordinatorRuntime) activeWorkerWorktrees(
	ctx context.Context,
	workspaceID string,
) ([]coordinator.WorkerWorktreeBinding, error) {
	runs, err := r.store.ListTaskRunsByStatus(ctx, coordinator.ExecutableRunStatuses())
	if err != nil {
		return nil, fmt.Errorf("daemon: list active worker worktrees: %w", err)
	}
	workspaceID = strings.TrimSpace(workspaceID)
	bindings := make([]coordinator.WorkerWorktreeBinding, 0, len(runs))
	for _, run := range runs {
		if strings.TrimSpace(run.WorkspaceID) != workspaceID ||
			run.RunKind.Normalize() != taskpkg.RunKindWorker ||
			run.IsLoopWorker() {
			continue
		}
		runID := strings.TrimSpace(run.ID)
		worktreeID := strings.TrimSpace(run.WorktreeIDValue())
		if runID == "" || worktreeID == "" {
			continue
		}
		bindings = append(bindings, coordinator.WorkerWorktreeBinding{
			RunID:      runID,
			WorktreeID: worktreeID,
		})
	}
	return bindings, nil
}
