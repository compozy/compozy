package daemon

import (
	"context"
	"errors"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

type daemonLoopInputEntityCatalog struct {
	state *bootState
}

type loopInputEntityValidator func(context.Context, string, string, string) (bool, error)

type loopInputEntityDescriptor struct {
	ListSurface string
	IDShape     string
	Validate    loopInputEntityValidator
}

var _ looppkg.InputEntityCatalog = daemonLoopInputEntityCatalog{}

func (c daemonLoopInputEntityCatalog) HasInputEntity(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
	profileID string,
	kind dsl.EntityKind,
	value string,
) (bool, error) {
	target := strings.TrimSpace(value)
	if target == "" || c.state == nil {
		return false, nil
	}
	descriptor, exists := c.entityRegistry()[kind]
	if !exists {
		return false, nil
	}
	return descriptor.Validate(ctx, string(workspaceID), strings.TrimSpace(profileID), target)
}

func (c daemonLoopInputEntityCatalog) entityRegistry() map[dsl.EntityKind]loopInputEntityDescriptor {
	return map[dsl.EntityKind]loopInputEntityDescriptor{
		dsl.EntityKindAgent: {
			ListSurface: "agents.list", IDShape: "agent name", Validate: c.hasAgent,
		},
		dsl.EntityKindSkill: {
			ListSurface: "skills.list", IDShape: "skill name", Validate: c.hasSkill,
		},
		dsl.EntityKindLoop: {
			ListSurface: "loops.list", IDShape: "Loop name",
			Validate: c.hasLoop,
		},
		dsl.EntityKindWorktree: {
			ListSurface: "worktrees.list", IDShape: "worktree id", Validate: c.hasWorktree,
		},
		dsl.EntityKindSession: {
			ListSurface: "sessions.list", IDShape: "session id", Validate: c.hasSession,
		},
		dsl.EntityKindWorkspace: {
			ListSurface: "workspaces.list", IDShape: "workspace id",
			Validate: func(ctx context.Context, _, _ string, value string) (bool, error) {
				return c.hasWorkspace(ctx, value)
			},
		},
		dsl.EntityKindSecret: {
			ListSurface: "vault.list", IDShape: "vault reference",
			Validate: func(ctx context.Context, _, _ string, value string) (bool, error) {
				return c.hasSecret(ctx, value)
			},
		},
	}
}

func (c daemonLoopInputEntityCatalog) resolvedWorkspace(
	ctx context.Context,
	workspaceID string,
	profileID string,
) (*workspacepkg.ResolvedWorkspace, error) {
	if c.state.workspaceResolver == nil {
		return nil, nil
	}
	lens, err := resolveLoopResourceLens(ctx, c.state.profiles, workspaceID, profileID)
	if err != nil {
		return nil, err
	}
	resolved, err := c.state.workspaceResolver.ResolveForProfile(ctx, workspaceID, lens.ProfileName)
	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

func (c daemonLoopInputEntityCatalog) hasAgent(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
) (bool, error) {
	resolved, err := c.resolvedWorkspace(ctx, workspaceID, profileID)
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
	profileID string,
	name string,
) (bool, error) {
	if c.state.skillsRegistry == nil {
		return false, nil
	}
	resolved, err := c.resolvedWorkspace(ctx, workspaceID, profileID)
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

func (c daemonLoopInputEntityCatalog) hasLoop(
	ctx context.Context,
	workspaceID string,
	profileID string,
	name string,
) (bool, error) {
	if c.state.loopCatalog == nil {
		return false, nil
	}
	lens, err := resolveLoopResourceLens(ctx, c.state.profiles, workspaceID, profileID)
	if err != nil {
		return false, err
	}
	for _, record := range looppkg.ResolveEffectiveResources(c.state.loopCatalog.Snapshot(), lens) {
		if strings.TrimSpace(record.Spec.Name) != name {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (c daemonLoopInputEntityCatalog) hasWorktree(
	ctx context.Context,
	workspaceID string,
	_ string,
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
	_ string,
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
	if err := vault.ValidateSecretRef(ref); err != nil {
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
