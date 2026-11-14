package session

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	workspacepkg "github.com/compozy/compozy/internal/workspace"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// InstanceCeiling is the authority an instance lineage imposes on one session.
type InstanceCeiling struct {
	Policy store.SessionPermissionPolicy
	Grant  string
	// Main marks the instance's own session, which always runs at its configured level.
	Main bool
}

// InstanceCeilingReader resolves the ceiling for a session, or nil outside an instance lineage.
type InstanceCeilingReader func(ctx context.Context, sessionID, instanceID string) (*InstanceCeiling, error)

func (m *Manager) SetInstanceCeilingReader(reader InstanceCeilingReader) { m.instanceCeiling = reader }

func (m *Manager) applyInstanceCeiling(
	ctx context.Context,
	spec *sessionStartSpec,
	resolved *compozyconfig.ResolvedAgent,
) error {
	if m.instanceCeiling == nil {
		return nil
	}
	ceiling, err := m.instanceCeiling(ctx, spec.sessionID, spec.instanceID)
	if err == nil && ceiling == nil && spec.lineage != nil && spec.lineage.ParentSessionID != "" {
		ceiling, err = m.instanceCeiling(ctx, spec.lineage.ParentSessionID, "")
		if ceiling != nil {
			ceiling.Main = false
		}
	}
	if err != nil || ceiling == nil {
		return err
	}
	tools, err := concreteDelegationTools(*resolved, m.toolsetCatalog, m.toolUniverse)
	if err != nil {
		return err
	}
	policy := ceiling.Policy
	if spec.lineage != nil && (spec.sessionType == SessionTypeSpawned || spec.startAction == sessionStartActionResume) {
		stored := spec.lineage.PermissionPolicy
		policy = agentinstance.IntersectPermissions(policy, stored)
		policy.MaterializedTools = stored.MaterializedTools && slices.Equal(policy.Tools, stored.Tools)
		policy.MaterializedMCPServers = stored.MaterializedMCPServers &&
			slices.Equal(policy.MCPServers, stored.MCPServers)
	}
	definitionPolicy := store.SessionPermissionPolicy{Tools: tools, ToolsMode: store.PermissionInherit}
	if resolved.ToolPolicyEnforced || len(resolved.Tools) > 0 || len(resolved.Toolsets) > 0 {
		definitionPolicy.ToolsMode = store.PermissionList
	}
	boundedTools := agentinstance.IntersectPermissions(policy, definitionPolicy).Tools
	policy.MaterializedTools = policy.MaterializedTools && slices.Equal(policy.Tools, boundedTools)
	policy.Tools = boundedTools
	effective, err := m.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, &Info{
		ID: spec.sessionID, ProfileID: spec.profileID, WorkspaceID: spec.workspace.ID,
		AgentName: spec.agentName, Type: spec.sessionType,
		Lineage: &store.SessionLineage{PermissionPolicy: policy},
	}), spec.sessionID)
	if err != nil {
		return err
	}
	policy.WorkspacePaths = withManagedWorktree(policy, spec)
	resolved.ToolPolicyEnforced = true
	resolved.Tools = slices.Clone(policy.Tools)
	resolved.Toolsets = nil
	resolved.MCPServers = slices.DeleteFunc(
		slices.Clone(resolved.MCPServers),
		func(server compozyconfig.MCPServer) bool {
			return !slices.Contains(effective.MCPServers, server.Name)
		},
	)
	spec.instancePermissionCeiling = &policy
	spec.permissions = m.instanceLevel(spec, *resolved, ceiling.Main)
	spec.workspace.Skills = slices.DeleteFunc(
		slices.Clone(spec.workspace.Skills),
		func(skill workspacepkg.SkillPath) bool {
			name := strings.TrimSpace(skill.Name)
			if name == "" && skill.Dir != "" {
				name = filepath.Base(skill.Dir)
			}
			return !slices.Contains(policy.Skills, name)
		},
	)
	spec.lineage = store.NormalizeSessionLineage(spec.sessionID, spec.lineage)
	spec.lineage.PermissionPolicy = policy
	spec.origin = workspaceaccess.Origin{Kind: workspaceaccess.ActorDaemon, Ceiling: ceiling.Grant}
	spec.effectiveWorkspaceAccess = workspaceaccess.NarrowGrant(spec.effectiveWorkspaceAccess, ceiling.Grant)
	return nil
}

// withManagedWorktree admits a worktree of a workspace inside the ceiling. Managed worktrees belong
// to their workspace, yet their default root sits outside it.
func withManagedWorktree(ceiling store.SessionPermissionPolicy, spec *sessionStartSpec) []string {
	if spec.worktreeRoot == "" || !withinCeiling(ceiling, spec.workspace.RootDir) {
		return ceiling.WorkspacePaths
	}
	return append(slices.Clone(ceiling.WorkspacePaths), spec.worktreeRoot)
}

func withinCeiling(ceiling store.SessionPermissionPolicy, path string) bool {
	return slices.ContainsFunc(ceiling.WorkspacePaths, func(root string) bool {
		_, err := resolveContainedDirectory(root, path)
		return err == nil
	})
}

// instanceLevel resolves the main definition independently from descendant settings.
func (m *Manager) instanceLevel(
	spec *sessionStartSpec,
	resolved compozyconfig.ResolvedAgent,
	main bool,
) compozyconfig.PermissionMode {
	configured := m.startPermissions(resolved.Permissions)
	requested := m.startPermissions(startSpecPermissions(spec, resolved.Permissions))
	if main {
		if !resolved.PermissionsConfigured {
			return compozyconfig.AlwaysOnDefaultPermissionMode
		}
		return configured
	}
	if requested.Rank() > configured.Rank() {
		return configured
	}
	return requested
}

func validateInstanceLaunch(spec *sessionStartSpec, before, after acp.StartOpts) error {
	if spec.instancePermissionCeiling == nil {
		return nil
	}
	if before.Permissions != after.Permissions {
		return fmt.Errorf("%w: instance hook changed permission mode", ErrValidation)
	}
	for _, server := range after.MCPServers {
		if !slices.ContainsFunc(
			before.MCPServers,
			func(allowed compozyconfig.MCPServer) bool { return allowed.Name == server.Name },
		) {
			return fmt.Errorf("%w: instance hook added MCP server", ErrValidation)
		}
	}
	paths := append([]string{after.Cwd}, after.AdditionalDirs...)
	for _, path := range paths {
		if !withinCeiling(*spec.instancePermissionCeiling, path) {
			return fmt.Errorf("%w: instance execution path exceeds permission ceiling", ErrValidation)
		}
	}
	return nil
}

func (m *Manager) SetInstancePauseHandler(handler func(context.Context, store.InstancePauseTrip) error) {
	m.instancePaused = handler
}
