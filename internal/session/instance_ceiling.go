package session

import (
	"context"
	"fmt"
	"github.com/compozy/compozy/internal/acp"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"slices"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type InstanceCeilingReader func(context.Context, string, string) (*store.SessionPermissionPolicy, string, error)

func (m *Manager) SetInstanceCeilingReader(reader InstanceCeilingReader) { m.instanceCeiling = reader }

func (m *Manager) applyInstanceCeiling(
	ctx context.Context,
	spec *sessionStartSpec,
	resolved *compozyconfig.ResolvedAgent,
) error {
	if m.instanceCeiling == nil {
		return nil
	}
	ceiling, grant, err := m.instanceCeiling(ctx, spec.sessionID, spec.instanceID)
	if err != nil || ceiling == nil {
		return err
	}
	tools, err := concreteDelegationTools(*resolved, m.toolsetCatalog, m.toolUniverse)
	if err != nil {
		return err
	}
	policy := *ceiling
	policy.Tools = agentinstance.IntersectPermissions(*ceiling, store.SessionPermissionPolicy{Tools: tools}).Tools
	if err := ValidatePermissionSubset(*ceiling, policy); err != nil {
		return err
	}
	resolved.ToolPolicyEnforced = true
	resolved.Tools = slices.Clone(policy.Tools)
	resolved.Toolsets = nil
	resolved.MCPServers = slices.DeleteFunc(
		slices.Clone(resolved.MCPServers),
		func(server compozyconfig.MCPServer) bool {
			return !slices.Contains(policy.MCPServers, server.Name)
		},
	)
	spec.instancePermissionCeiling = &policy
	spec.workspace.Skills = slices.DeleteFunc(slices.Clone(spec.workspace.Skills), func(skill workspacepkg.SkillPath) bool {
		return !slices.Contains(policy.Skills, skill.Name)
	})
	spec.lineage = store.NormalizeSessionLineage(spec.sessionID, spec.lineage)
	spec.lineage.PermissionPolicy = policy
	spec.origin = workspaceaccess.Origin{Kind: workspaceaccess.ActorDaemon, Ceiling: grant}
	spec.effectiveWorkspaceAccess = workspaceaccess.NarrowGrant(spec.effectiveWorkspaceAccess, grant)
	return nil
}

func validateInstanceLaunch(spec *sessionStartSpec, before, after acp.StartOpts) error {
	if spec.instancePermissionCeiling == nil {
		return nil
	}
	if before.Permissions != after.Permissions {
		return fmt.Errorf("%w: instance hook changed permission mode", ErrValidation)
	}
	for _, server := range after.MCPServers {
		if !slices.ContainsFunc(before.MCPServers, func(allowed compozyconfig.MCPServer) bool { return allowed.Name == server.Name }) {
			return fmt.Errorf("%w: instance hook added MCP server", ErrValidation)
		}
	}
	paths := append([]string{after.Cwd}, after.AdditionalDirs...)
	for _, path := range paths {
		allowed := false
		for _, root := range spec.instancePermissionCeiling.WorkspacePaths {
			if _, err := resolveContainedDirectory(root, path); err == nil {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: instance execution path exceeds permission ceiling", ErrValidation)
		}
	}
	return nil
}
