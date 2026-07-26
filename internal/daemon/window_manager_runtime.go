package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/compozy/agh/internal/clientstate"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/google/uuid"
)

type windowManagerBootState struct {
	windowManagerStoreResolver *windowManagerStoreWorkspaceResolver
	windowManagerStore         *clientstate.Engine
	windowManagerRepository    *windowManagerRepository
	windowLayoutCatalog        *resourceCatalog[windowmanager.LayoutResource]
	windowManager              *windowmanager.Manager
}

type windowManagerRuntime struct {
	windowManagerStore *clientstate.Engine
	windowManager      *windowmanager.Manager
}

type windowManagerWorkspaceRecord struct {
	generation clientstate.WorkspaceGeneration
	deleting   bool
}

type windowManagerStoreWorkspaceResolver struct {
	mu       sync.Mutex
	resolver *workspacepkg.Resolver
	logger   *slog.Logger
	records  map[clientstate.WorkspaceID]windowManagerWorkspaceRecord
}

var _ clientstate.WorkspaceResolver = (*windowManagerStoreWorkspaceResolver)(nil)

type windowManagerWorkspaceConfigResolver struct {
	resolver *workspacepkg.Resolver
}

var _ windowmanager.WorkspaceConfigResolver = windowManagerWorkspaceConfigResolver{}

func (r windowManagerWorkspaceConfigResolver) ResolveWindowManagerConfig(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	defaults windowmanager.Config,
) (windowmanager.Config, error) {
	if r.resolver == nil {
		return windowmanager.Config{}, errors.New("daemon: workspace config resolver is unavailable")
	}
	resolved, err := r.resolver.Resolve(ctx, string(workspaceID))
	if err != nil {
		return windowmanager.Config{}, fmt.Errorf("daemon: resolve workspace config %q: %w", workspaceID, err)
	}
	if resolved.ID != string(workspaceID) {
		return windowmanager.Config{}, fmt.Errorf(
			"daemon: workspace config %q resolved as %q: %w",
			workspaceID,
			resolved.ID,
			windowmanager.ErrWorkspaceNotFound,
		)
	}
	active, err := windowManagerConfig(defaults)
	if err != nil {
		return windowmanager.Config{}, err
	}
	overlaid, err := aghconfig.ApplyWindowManagerOverlayFile(
		filepath.Join(resolved.RootDir, aghconfig.DirName, aghconfig.ConfigName),
		active,
	)
	if err != nil {
		return windowmanager.Config{}, fmt.Errorf(
			"daemon: apply workspace window-manager config %q: %w",
			workspaceID,
			err,
		)
	}
	return windowManagerDefaults(overlaid), nil
}

func newWindowManagerStoreWorkspaceResolver(
	resolver *workspacepkg.Resolver,
	logger *slog.Logger,
) (*windowManagerStoreWorkspaceResolver, error) {
	if resolver == nil {
		return nil, errors.New("daemon: window-manager workspace resolver is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &windowManagerStoreWorkspaceResolver{
		resolver: resolver,
		logger:   logger.With("component", "window_manager_store"),
		records:  make(map[clientstate.WorkspaceID]windowManagerWorkspaceRecord),
	}, nil
}

func (r *windowManagerStoreWorkspaceResolver) ResolveWorkspace(
	ctx context.Context,
	workspaceID clientstate.WorkspaceID,
) (clientstate.WorkspaceGeneration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	workspace, err := r.resolver.Get(ctx, string(workspaceID))
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", ctxErr
		}
		return "", fmt.Errorf(
			"daemon: resolve window-manager workspace %q: %w",
			workspaceID,
			errors.Join(err, clientstate.ErrWorkspaceNotFound),
		)
	}
	if workspace.ID != string(workspaceID) {
		return "", fmt.Errorf(
			"daemon: window-manager workspace %q resolved as %q: %w",
			workspaceID,
			workspace.ID,
			clientstate.ErrWorkspaceNotFound,
		)
	}
	record := r.records[workspaceID]
	if record.deleting {
		return "", clientstate.ErrWorkspaceNotFound
	}
	if record.generation == "" {
		record.generation = newWindowManagerWorkspaceGeneration()
		r.records[workspaceID] = record
	}
	return record.generation, nil
}

func newWindowManagerWorkspaceGeneration() clientstate.WorkspaceGeneration {
	return clientstate.WorkspaceGeneration(uuid.NewString())
}

func (r *windowManagerStoreWorkspaceResolver) ResolveWorkspaceForPurge(
	ctx context.Context,
	workspaceID clientstate.WorkspaceID,
) (clientstate.WorkspaceGeneration, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record, ok := r.records[workspaceID]
	if !ok || !record.deleting || record.generation == "" {
		return "", clientstate.ErrWorkspaceNotFound
	}
	return record.generation, nil
}

type windowManagerWorkspaceAuthorizer struct {
	resolver *windowManagerStoreWorkspaceResolver
}

var _ windowmanager.WorkspaceResolver = windowManagerWorkspaceAuthorizer{}

func (a windowManagerWorkspaceAuthorizer) ResolveWorkspace(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) error {
	if a.resolver == nil {
		return errors.New("daemon: window-manager workspace authorizer is unavailable")
	}
	_, err := a.resolver.ResolveWorkspace(ctx, clientstate.WorkspaceID(workspaceID))
	return mapWindowManagerStoreError("authorize workspace", err)
}
