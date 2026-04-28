package daemon

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type transportSyncService struct {
	globalDB *globaldb.GlobalDB
}

var _ apicore.SyncService = (*transportSyncService)(nil)

func newTransportSyncService(globalDB *globaldb.GlobalDB) *transportSyncService {
	return &transportSyncService{globalDB: globalDB}
}

func (s *transportSyncService) Sync(ctx context.Context, req apicore.SyncRequest) (apicore.SyncResult, error) {
	if s == nil || s.globalDB == nil {
		return apicore.SyncResult{}, syncTransportUnavailable()
	}

	cfg, workspaceID, workflowSlug, err := s.resolveSyncConfig(ctx, req)
	if err != nil {
		return apicore.SyncResult{}, err
	}

	result, err := corepkg.SyncDirect(ctx, cfg)
	if err != nil {
		return apicore.SyncResult{}, err
	}

	syncedAt := time.Now().UTC()
	return transportSyncResult(workspaceID, workflowSlug, &syncedAt, result), nil
}

func (s *transportSyncService) resolveSyncConfig(
	ctx context.Context,
	req apicore.SyncRequest,
) (corepkg.SyncConfig, string, string, error) {
	path := strings.TrimSpace(req.Path)
	workflowSlug := strings.TrimSpace(req.WorkflowSlug)
	if path != "" {
		workspaceRoot, err := workspacecfg.Discover(ctx, path)
		if err != nil {
			return corepkg.SyncConfig{}, "", "", err
		}
		workspaceRow, err := s.globalDB.ResolveOrRegister(ctx, workspaceRoot)
		if err != nil {
			return corepkg.SyncConfig{}, "", "", err
		}

		cfg := corepkg.SyncConfig{WorkspaceRoot: workspaceRow.RootDir}
		if workflowSlug != "" {
			cfg.RootDir = path
			cfg.Name = workflowSlug
			return cfg, workspaceRow.ID, workflowSlug, nil
		}

		if looksLikeWorkflowDir(path) {
			cfg.TasksDir = path
			return cfg, workspaceRow.ID, filepath.Base(path), nil
		}

		cfg.RootDir = path
		return cfg, workspaceRow.ID, "", nil
	}

	workspaceRef := strings.TrimSpace(req.Workspace)
	if workspaceRef == "" {
		return corepkg.SyncConfig{}, "", "", apicore.NewProblem(
			http.StatusUnprocessableEntity,
			"sync_target_required",
			"workspace or path is required",
			nil,
			nil,
		)
	}

	workspaceRow, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return corepkg.SyncConfig{}, "", "", err
	}
	if workspaceRow.FilesystemState == globaldb.WorkspaceFilesystemStateMissing {
		return corepkg.SyncConfig{}, "", "", apicore.WorkspacePathMissingProblem(
			workspaceRow.ID,
			workspaceRow.RootDir,
			nil,
		)
	}
	return corepkg.SyncConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Name:          workflowSlug,
	}, workspaceRow.ID, workflowSlug, nil
}

func looksLikeWorkflowDir(path string) bool {
	base := strings.TrimSpace(filepath.Base(path))
	if base == "" {
		return false
	}
	if strings.HasPrefix(base, "reviews-") {
		return false
	}
	matches, err := filepath.Glob(filepath.Join(path, "task_*.md"))
	if err != nil {
		return false
	}
	return len(matches) > 0
}

func syncTransportUnavailable() error {
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"sync_service_unavailable",
		"workflow sync is not available in this daemon build",
		nil,
		nil,
	)
}
