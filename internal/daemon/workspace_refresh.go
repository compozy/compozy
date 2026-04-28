package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type workspaceRefreshOptions struct {
	SyncPresent bool
}

func refreshRegisteredWorkspaces(
	ctx context.Context,
	db *globaldb.GlobalDB,
	opts workspaceRefreshOptions,
) (apicore.WorkspaceSyncResult, error) {
	if db == nil {
		return apicore.WorkspaceSyncResult{}, apicore.NewProblem(
			500,
			"workspace_registry_unavailable",
			"workspace registry is unavailable",
			nil,
			nil,
		)
	}

	workspaces, err := db.List(ctx)
	if err != nil {
		return apicore.WorkspaceSyncResult{}, err
	}

	var result apicore.WorkspaceSyncResult
	for idx := range workspaces {
		workspace := &workspaces[idx]
		if err := ctx.Err(); err != nil {
			return result, err
		}
		result.Checked++

		pathState := checkWorkspacePath(workspace.RootDir)
		if pathState.Warning != "" {
			result.Warnings = append(result.Warnings, pathState.Warning)
		}
		switch {
		case pathState.Present:
			updated, err := db.UpdateWorkspaceState(ctx, globaldb.WorkspaceStateUpdate{
				WorkspaceID:     workspace.ID,
				FilesystemState: globaldb.WorkspaceFilesystemStatePresent,
				CheckedAt:       time.Now().UTC(),
			})
			if err != nil {
				return result, err
			}
			if opts.SyncPresent {
				if err := syncWorkspace(ctx, db, updated, &result); err != nil {
					return result, err
				}
			}
		case pathState.Missing:
			if !workspace.HasCatalogData {
				removed, err := db.DeleteWorkspaceIfNoCatalogData(ctx, workspace.ID)
				if err != nil {
					return result, err
				}
				if removed {
					result.Removed++
				}
				continue
			}
			result.Missing++
			if _, err := db.UpdateWorkspaceState(ctx, globaldb.WorkspaceStateUpdate{
				WorkspaceID:     workspace.ID,
				FilesystemState: globaldb.WorkspaceFilesystemStateMissing,
				CheckedAt:       time.Now().UTC(),
			}); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func syncWorkspace(
	ctx context.Context,
	db *globaldb.GlobalDB,
	workspace globaldb.Workspace,
	result *apicore.WorkspaceSyncResult,
) error {
	syncResult, err := corepkg.SyncWithDB(ctx, db, workspace, corepkg.SyncConfig{
		WorkspaceRoot: workspace.RootDir,
	})
	if err != nil {
		errText := err.Error()
		if _, updateErr := db.UpdateWorkspaceState(ctx, globaldb.WorkspaceStateUpdate{
			WorkspaceID:     workspace.ID,
			FilesystemState: globaldb.WorkspaceFilesystemStatePresent,
			CheckedAt:       time.Now().UTC(),
			LastSyncError:   &errText,
		}); updateErr != nil {
			return updateErr
		}
		result.Warnings = append(
			result.Warnings,
			fmt.Sprintf("%s: sync failed: %v", workspace.Name, err),
		)
		return nil
	}

	syncedAt := time.Now().UTC()
	syncError := ""
	if _, err := db.UpdateWorkspaceState(ctx, globaldb.WorkspaceStateUpdate{
		WorkspaceID:     workspace.ID,
		FilesystemState: globaldb.WorkspaceFilesystemStatePresent,
		CheckedAt:       syncedAt,
		LastSyncedAt:    &syncedAt,
		LastSyncError:   &syncError,
	}); err != nil {
		return err
	}
	result.Synced++
	if syncResult == nil {
		return nil
	}
	result.SnapshotsUpserted += syncResult.SnapshotsUpserted
	result.TaskItemsUpserted += syncResult.TaskItemsUpserted
	result.ReviewRoundsUpserted += syncResult.ReviewRoundsUpserted
	result.ReviewIssuesUpserted += syncResult.ReviewIssuesUpserted
	result.Warnings = append(result.Warnings, syncResult.Warnings...)
	return nil
}

func requireWorkspacePathAvailable(workspace globaldb.Workspace) error {
	if workspace.FilesystemState == globaldb.WorkspaceFilesystemStateMissing {
		return apicore.WorkspacePathMissingProblem(workspace.ID, workspace.RootDir, nil)
	}
	pathState := checkWorkspacePath(workspace.RootDir)
	if pathState.Present {
		return nil
	}
	message := strings.TrimSpace(pathState.Warning)
	if message == "" {
		message = "workspace path is missing"
	}
	return apicore.WorkspacePathMissingProblem(workspace.ID, workspace.RootDir, errors.New(message))
}

type workspacePathState struct {
	Present bool
	Missing bool
	Warning string
}

func checkWorkspacePath(rootDir string) workspacePathState {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return workspacePathState{
			Missing: true,
			Warning: "workspace has an empty root path",
		}
	}

	info, err := os.Stat(rootDir)
	if err == nil {
		if info.IsDir() {
			return workspacePathState{Present: true}
		}
		return workspacePathState{
			Missing: true,
			Warning: fmt.Sprintf("%s: workspace path is not a directory", rootDir),
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return workspacePathState{
			Missing: true,
			Warning: fmt.Sprintf("%s: workspace path is missing", rootDir),
		}
	}
	return workspacePathState{
		Warning: fmt.Sprintf("%s: workspace path check failed: %v", rootDir, err),
	}
}
