package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func (n *daemonNativeTools) nativeProfileSelection(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	descriptor toolspkg.Descriptor,
) (toolspkg.Scope, toolspkg.CallRequest, error) {
	// Extension lifecycle handlers own installation profile selection.
	if slices.Contains(descriptor.Toolsets, toolspkg.ToolsetIDExtensions) {
		return scope, req, nil
	}
	selection, err := decodeNativeProfileSelection(descriptor, req)
	if err != nil || !selection.present {
		return scope, req, err
	}
	if selection.all && (!descriptor.ReadOnly || selection.requested != "") {
		return scope, req, nativeInputError(req.ToolID, errors.New("all_profiles requires a read without profile"))
	}
	targetProfile, err := n.nativeProfileTarget(ctx, scope, req, selection.requested)
	if err != nil {
		return scope, req, err
	}
	if scope.SessionID != "" && (selection.all || targetProfile != scope.ProfileID) {
		if err := n.authorizeNativeProfileSwitch(ctx, scope, req, descriptor, selection, targetProfile); err != nil {
			return scope, req, err
		}
	}
	scope.ProfileID, scope.AllProfiles = targetProfile, selection.all
	delete(selection.input, "profile")
	delete(selection.input, "all_profiles")
	encoded, err := json.Marshal(selection.input)
	if err != nil {
		return scope, req, err
	}
	req.Input = encoded
	return scope, req, nil
}

// nativeProfileSelectionInput is a workspace tool call's decoded profile selector.
type nativeProfileSelectionInput struct {
	input     map[string]json.RawMessage
	requested string
	all       bool
	// present reports a profile or all_profiles field; without one the call keeps its scope.
	present bool
}

func decodeNativeProfileSelection(
	descriptor toolspkg.Descriptor,
	req toolspkg.CallRequest,
) (nativeProfileSelectionInput, error) {
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(descriptor.InputSchema, &schema); err != nil {
		return nativeProfileSelectionInput{}, err
	}
	if _, ok := schema.Properties["workspace"]; !ok {
		return nativeProfileSelectionInput{}, nil
	}
	selection := nativeProfileSelectionInput{}
	if err := json.Unmarshal(req.Input, &selection.input); err != nil {
		return nativeProfileSelectionInput{}, err
	}
	rawProfile, hasProfile := selection.input["profile"]
	if hasProfile {
		if err := json.Unmarshal(rawProfile, &selection.requested); err != nil {
			return nativeProfileSelectionInput{}, err
		}
	}
	rawAll, hasAll := selection.input["all_profiles"]
	if hasAll {
		if err := json.Unmarshal(rawAll, &selection.all); err != nil {
			return nativeProfileSelectionInput{}, err
		}
	}
	selection.requested = strings.TrimSpace(selection.requested)
	selection.present = hasProfile || hasAll
	return selection, nil
}

// nativeProfileTarget resolves a requested active profile by ID or name; none keeps the scope's.
func (n *daemonNativeTools) nativeProfileTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	requested string,
) (string, error) {
	if requested == "" {
		return scope.ProfileID, nil
	}
	if n.deps.Profiles == nil {
		return "", errors.New("profile service is unavailable")
	}
	profiles, err := n.deps.Profiles.List(ctx)
	if err != nil {
		return "", err
	}
	for _, profile := range profiles {
		if (profile.ID == requested || profile.Name == requested) && profile.State != profilepkg.StateArchived {
			return profile.ID, nil
		}
	}
	return "", nativeInputError(req.ToolID, errors.New("active profile not found"))
}

// authorizeNativeProfileSwitch admits an agent session's cross-profile call only by its grant.
func (n *daemonNativeTools) authorizeNativeProfileSwitch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	descriptor toolspkg.Descriptor,
	selection nativeProfileSelectionInput,
	targetProfile string,
) error {
	if descriptor.ExecutionMetadata().ReservedWorkspacePartition || n.deps.WorkspaceAccess == nil {
		return nativeWorkspaceAccessDeniedError(req.ToolID)
	}
	targetWorkspace := scope.WorkspaceID
	if raw, ok := selection.input["workspace"]; ok {
		if err := json.Unmarshal(raw, &targetWorkspace); err != nil {
			return err
		}
	}
	target := workspaceaccess.ScopeTarget(targetWorkspace, targetProfile)
	if selection.all {
		target.ProfileID = ""
		target.AllProfiles = true
	}
	decision, err := n.deps.WorkspaceAccess.Authorize(ctx, workspaceaccess.Request{
		Actor: workspaceaccess.ActorRef{
			Kind:        workspaceaccess.ActorAgentSession,
			SessionID:   scope.SessionID,
			WorkspaceID: scope.WorkspaceID,
			AgentName:   scope.AgentName,
		},
		Target:   target,
		ReadOnly: descriptor.ReadOnly,
		Seam:     workspaceaccess.SeamTool,
	})
	if err != nil {
		return err
	}
	if !decision.Allowed || decision.Source != workspaceaccess.SourceAgentGrant {
		return nativeWorkspaceAccessDeniedError(req.ToolID)
	}
	return nil
}
