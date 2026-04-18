package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

func transportWorkspace(row globaldb.Workspace) apicore.Workspace {
	return apicore.Workspace{
		ID:        row.ID,
		RootDir:   row.RootDir,
		Name:      row.Name,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func transportWorkflowSummary(row globaldb.Workflow) apicore.WorkflowSummary {
	return apicore.WorkflowSummary{
		ID:           row.ID,
		WorkspaceID:  row.WorkspaceID,
		Slug:         row.Slug,
		ArchivedAt:   row.ArchivedAt,
		LastSyncedAt: row.LastSyncedAt,
	}
}

func transportSyncResult(
	workspaceID string,
	workflowSlug string,
	syncedAt *time.Time,
	result *corepkg.SyncResult,
) apicore.SyncResult {
	out := apicore.SyncResult{
		WorkspaceID:  workspaceID,
		WorkflowSlug: workflowSlug,
		SyncedAt:     syncedAt,
	}
	if result == nil {
		return out
	}

	out.Target = result.Target
	out.WorkflowsScanned = result.WorkflowsScanned
	out.SnapshotsUpserted = result.SnapshotsUpserted
	out.TaskItemsUpserted = result.TaskItemsUpserted
	out.ReviewRoundsUpserted = result.ReviewRoundsUpserted
	out.ReviewIssuesUpserted = result.ReviewIssuesUpserted
	out.CheckpointsUpdated = result.CheckpointsUpdated
	out.LegacyArtifactsRemoved = result.LegacyArtifactsRemoved
	out.SyncedPaths = append([]string(nil), result.SyncedPaths...)
	out.Warnings = append([]string(nil), result.Warnings...)
	return out
}

func resolveWorkspaceReference(
	ctx context.Context,
	globalDB *globaldb.GlobalDB,
	ref string,
) (globaldb.Workspace, error) {
	if globalDB == nil {
		return globaldb.Workspace{}, apicore.NewProblem(
			500,
			"workspace_registry_unavailable",
			"workspace registry is unavailable",
			nil,
			nil,
		)
	}

	trimmedRef := strings.TrimSpace(ref)
	row, err := globalDB.Get(ctx, trimmedRef)
	if err == nil {
		return row, nil
	}
	if !errors.Is(err, globaldb.ErrWorkspaceNotFound) {
		return globaldb.Workspace{}, err
	}
	return globalDB.ResolveOrRegister(ctx, trimmedRef)
}
