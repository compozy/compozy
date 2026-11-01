package daemon

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/managedsidecar"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func resolveInstanceCheckPolicy(
	ctx context.Context,
	definitions instanceDefinitions,
	resolver heartbeat.PolicyResolver,
	target *heartbeat.CheckTarget,
) error {
	instance := target.Instance
	var workspace workspacepkg.ResolvedWorkspace
	var err error
	if instance.WorkspaceID == "" {
		workspace, err = definitions.workspaces.ResolveUser(ctx, instance.ProfileID)
	} else {
		name, nameErr := definitions.profiles.ProfileName(ctx, instance.ProfileID)
		if nameErr != nil {
			return nameErr
		}
		workspace, err = definitions.workspaces.ResolveForProfile(ctx, instance.WorkspaceID, name)
	}
	if err != nil {
		return err
	}
	agent, err := definitions.agents.ResolveAgent(instance.AgentName, &workspace)
	if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		target.Gate = "definition_missing"
		return nil
	}
	if err != nil {
		return err
	}
	if agent.Kind != "stays_on" {
		target.Gate = "definition_not_stays_on"
		return nil
	}
	authoring := heartbeat.AuthoringTarget{
		ProfileID:     instance.ProfileID,
		ProfileName:   workspace.ProfileName,
		WorkspaceID:   instance.WorkspaceID,
		WorkspaceRoot: workspace.RootDir,
		AgentName:     instance.AgentName,
		AgentPath:     agent.SourcePath,
		Config:        workspace.Config.Agents.Heartbeat,
	}
	policy, err := resolveTimerPolicy(ctx, resolver, authoring)
	if errors.Is(err, heartbeat.ErrInvalid) || errors.Is(err, heartbeat.ErrPathEscape) {
		target.Gate = "heartbeat_invalid"
		return nil
	}
	if err != nil {
		return err
	}
	target.Policy = policy
	target.MainPrompting = definitions.sessions.IsPrompting(instance.MainSessionID)
	target.Scope.ProfileID = instance.ProfileID
	target.Scope.WorkspaceID = instance.WorkspaceID
	target.Scope.MainSessionID = instance.MainSessionID
	target.Scope.Unrestricted = instance.GrantCeiling == "unrestricted" && agent.WorkspaceAccess == "unrestricted"
	return nil
}

// timerPolicyRoot contains HEARTBEAT.md by the agent's own directory when the
// definition lives outside the workspace, as every user-level agent does.
func timerPolicyRoot(target heartbeat.AuthoringTarget) string {
	if _, within := managedsidecar.RelativeWithinRoot(target.WorkspaceRoot, target.AgentPath); within {
		return target.WorkspaceRoot
	}
	return filepath.Dir(target.AgentPath)
}

func resolveTimerPolicy(
	ctx context.Context,
	resolver heartbeat.PolicyResolver,
	target heartbeat.AuthoringTarget,
) (heartbeat.ResolvedPolicy, error) {
	var policy heartbeat.ResolvedPolicy
	var found bool
	var err error
	if resolver != nil {
		policy, found, err = resolver.ResolveHeartbeatPolicy(ctx, target)
		if err != nil {
			return policy, err
		}
	}
	if !found {
		policy, err = heartbeat.Resolve(
			ctx,
			heartbeat.ResolveRequest{
				AgentPath:     target.AgentPath,
				WorkspaceRoot: timerPolicyRoot(target),
				Config:        target.Config,
			},
		)
		if err != nil {
			return policy, err
		}
	}
	policy.Enabled = target.Config.Enabled && (!policy.Present || policy.Frontmatter.Enabled)
	return policy, nil
}
