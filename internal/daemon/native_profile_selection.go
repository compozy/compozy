package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) nativeProfileSelection(ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest, descriptor toolspkg.Descriptor) (toolspkg.Scope, toolspkg.CallRequest, error) {
	var input map[string]json.RawMessage
	if err := json.Unmarshal(req.Input, &input); err != nil {
		return scope, req, err
	}
	var requested string
	var all bool
	if raw, ok := input["profile"]; ok {
		if err := json.Unmarshal(raw, &requested); err != nil {
			return scope, req, err
		}
	}
	if raw, ok := input["all_profiles"]; ok {
		if err := json.Unmarshal(raw, &all); err != nil {
			return scope, req, err
		}
	}
	requested = strings.TrimSpace(requested)
	if all && (!descriptor.ReadOnly || requested != "") {
		return scope, req, nativeInputError(req.ToolID, errors.New("all_profiles requires a read without profile"))
	}
	if _, hasProfile := input["profile"]; !hasProfile {
		if _, hasAll := input["all_profiles"]; !hasAll {
			return scope, req, nil
		}
	}
	targetProfile := scope.ProfileID
	if requested != "" {
		if n.deps.Profiles == nil {
			return scope, req, errors.New("profile service is unavailable")
		}
		profiles, err := n.deps.Profiles.List(ctx)
		if err != nil {
			return scope, req, err
		}
		targetProfile = ""
		for _, profile := range profiles {
			if (profile.ID == requested || profile.Name == requested) && profile.State != profilepkg.StateArchived {
				targetProfile = profile.ID
				break
			}
		}
		if targetProfile == "" {
			return scope, req, nativeInputError(req.ToolID, errors.New("active profile not found"))
		}
	}
	if scope.SessionID != "" && (all || targetProfile != scope.ProfileID) {
		if descriptor.ExecutionMetadata().ReservedWorkspacePartition || n.deps.WorkspaceAccess == nil {
			return scope, req, nativeWorkspaceAccessDeniedError(req.ToolID)
		}
		targetWorkspace := scope.WorkspaceID
		if raw, ok := input["workspace"]; ok {
			if err := json.Unmarshal(raw, &targetWorkspace); err != nil {
				return scope, req, err
			}
		}
		target := workspaceaccess.ScopeTarget(targetWorkspace, targetProfile)
		if all {
			target.ProfileID = ""
			target.AllProfiles = true
		}
		decision, err := n.deps.WorkspaceAccess.Authorize(ctx, workspaceaccess.Request{
			Actor:  workspaceaccess.ActorRef{Kind: workspaceaccess.ActorAgentSession, SessionID: scope.SessionID, WorkspaceID: scope.WorkspaceID, AgentName: scope.AgentName},
			Target: target, ReadOnly: descriptor.ReadOnly, Seam: workspaceaccess.SeamTool,
		})
		if err != nil {
			return scope, req, err
		}
		if !decision.Allowed || decision.Source != workspaceaccess.SourceAgentGrant {
			return scope, req, nativeWorkspaceAccessDeniedError(req.ToolID)
		}
	}
	scope.ProfileID, scope.AllProfiles = targetProfile, all
	delete(input, "profile")
	delete(input, "all_profiles")
	encoded, err := json.Marshal(input)
	if err != nil {
		return scope, req, err
	}
	req.Input = encoded
	return scope, req, nil
}
