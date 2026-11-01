package daemon

import (
	"context"
	"errors"
	"slices"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type instanceDefinitions struct {
	workspaces *workspacepkg.Resolver
	profiles   session.ProfileNameResolver
	agents     session.AgentResolver
	sessions   *session.Manager
}

var _ instanceDefinitionReader = instanceDefinitions{}

func (r instanceDefinitions) InstanceDefinition(
	ctx context.Context,
	profileID, workspaceID, agentName string,
) (instanceDefinition, error) {
	var resolved workspacepkg.ResolvedWorkspace
	var err error
	if workspaceID == "" {
		resolved, err = r.workspaces.ResolveUser(ctx, profileID)
	} else {
		var name string
		name, err = r.profiles.ProfileName(ctx, profileID)
		if err == nil {
			resolved, err = r.workspaces.ResolveForProfile(ctx, workspaceID, name)
		}
	}
	if err != nil {
		return instanceDefinition{}, err
	}
	agent, err := r.agents.ResolveAgent(agentName, &resolved)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return instanceDefinition{}, agentinstance.ErrDefinitionMissing
	}
	if err != nil {
		return instanceDefinition{}, err
	}
	if agent.Kind != "stays_on" {
		return instanceDefinition{}, agentinstance.ErrNotStaysOn
	}
	runtime, err := resolved.Config.ResolveAgent(agent)
	if err != nil {
		return instanceDefinition{}, err
	}
	tools, err := r.sessions.DelegationTools(runtime)
	if err != nil {
		return instanceDefinition{}, err
	}
	policy := store.SessionPermissionPolicy{
		Tools:          tools,
		WorkspacePaths: append([]string{resolved.RootDir}, resolved.AdditionalDirs...),
	}
	for _, skill := range resolved.Skills {
		if !slices.Contains(agent.Skills.Disabled, skill.Name) {
			policy.Skills = append(policy.Skills, skill.Name)
		}
	}
	for _, server := range runtime.MCPServers {
		policy.MCPServers = append(policy.MCPServers, server.Name)
	}
	return instanceDefinition{
		Kind:   agent.Kind,
		Grant:  agent.WorkspaceAccess,
		Policy: store.NormalizeSessionPermissionPolicy(policy),
	}, nil
}
