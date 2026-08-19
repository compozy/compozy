package daemon

import (
	"context"
	"errors"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

type daemonLoopInputEntityCatalog struct {
	state *bootState
}

var _ looppkg.InputEntityCatalog = daemonLoopInputEntityCatalog{}

func (c daemonLoopInputEntityCatalog) HasInputEntity(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	kind dsl.EntityKind,
	value string,
) (bool, error) {
	target := strings.TrimSpace(value)
	if target == "" || c.state == nil {
		return false, nil
	}
	switch kind {
	case dsl.EntityKindAgent:
		return c.hasAgent(ctx, string(workspaceID), target)
	case dsl.EntityKindSkill:
		return c.hasSkill(ctx, string(workspaceID), target)
	case dsl.EntityKindLoop:
		return c.hasLoop(string(workspaceID), target), nil
	case dsl.EntityKindWorktree:
		return c.hasWorktree(ctx, string(workspaceID), target)
	case dsl.EntityKindSession:
		return c.hasSession(ctx, string(workspaceID), target)
	case dsl.EntityKindWorkspace:
		return c.hasWorkspace(ctx, target)
	case dsl.EntityKindSecret:
		return c.hasSecret(ctx, target)
	default:
		return false, nil
	}
}

func (c daemonLoopInputEntityCatalog) resolvedWorkspace(
	ctx context.Context,
	workspaceID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	if c.state.workspaceResolver == nil {
		return nil, nil
	}
	resolved, err := c.state.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (c daemonLoopInputEntityCatalog) hasAgent(
	ctx context.Context,
	workspaceID string,
	name string,
) (bool, error) {
	resolved, err := c.resolvedWorkspace(ctx, workspaceID)
	if err != nil || resolved == nil {
		return false, err
	}
	catalog := agentCatalogDependency(c.state.agentCatalog, agentSidecarCatalogs{
		soul: c.state.soulCatalog, heartbeat: c.state.heartbeatCatalog,
	})
	if catalog == nil {
		return false, nil
	}
	if _, err := catalog.ResolveAgent(name, resolved); err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c daemonLoopInputEntityCatalog) hasSkill(
	ctx context.Context,
	workspaceID string,
	name string,
) (bool, error) {
	if c.state.skillsRegistry == nil {
		return false, nil
	}
	resolved, err := c.resolvedWorkspace(ctx, workspaceID)
	if err != nil || resolved == nil {
		return false, err
	}
	items, err := c.state.skillsRegistry.ForWorkspace(ctx, resolved)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item != nil && strings.TrimSpace(item.Meta.Name) == name {
			return true, nil
		}
	}
	return false, nil
}

func (c daemonLoopInputEntityCatalog) hasLoop(workspaceID string, name string) bool {
	if c.state.loopCatalog == nil {
		return false
	}
	for _, record := range c.state.loopCatalog.Snapshot() {
		if strings.TrimSpace(record.Spec.Name) != name {
			continue
		}
		scope := record.Scope.Normalize()
		if scope.Kind == resources.ResourceScopeKindGlobal ||
			(scope.Kind == resources.ResourceScopeKindWorkspace && scope.ID == workspaceID) {
			return true
		}
	}
	return false
}

func (c daemonLoopInputEntityCatalog) hasWorktree(
	ctx context.Context,
	workspaceID string,
	ref string,
) (bool, error) {
	if c.state.worktrees == nil {
		return false, nil
	}
	_, err := c.state.worktrees.Get(ctx, workspaceID, ref)
	if errors.Is(err, worktree.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (c daemonLoopInputEntityCatalog) hasSession(
	ctx context.Context,
	workspaceID string,
	id string,
) (bool, error) {
	if c.state.sessions == nil {
		return false, nil
	}
	info, err := c.state.sessions.Status(ctx, id)
	if errors.Is(err, session.ErrSessionNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info != nil && strings.TrimSpace(info.WorkspaceID) == workspaceID, nil
}

func (c daemonLoopInputEntityCatalog) hasWorkspace(ctx context.Context, id string) (bool, error) {
	if c.state.workspaceResolver == nil {
		return false, nil
	}
	workspace, err := c.state.workspaceResolver.Get(ctx, id)
	if errors.Is(err, workspacepkg.ErrWorkspaceNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(workspace.ID) == id, nil
}

func (c daemonLoopInputEntityCatalog) hasSecret(ctx context.Context, ref string) (bool, error) {
	if c.state.providerVault == nil {
		return false, nil
	}
	metadata, err := c.state.providerVault.GetMetadata(ctx, ref)
	if errors.Is(err, vault.ErrSecretNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return metadata.Present, nil
}
