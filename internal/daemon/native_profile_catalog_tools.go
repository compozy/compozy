package daemon

import (
	"context"
	"errors"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeProfileListItem struct {
	Name       string `json:"name"`
	State      string `json:"state"`
	Current    bool   `json:"current"`
	WorkItems  int    `json:"work_items"`
	NeedsSetup bool   `json:"needs_setup"`
}

type nativeProfileCurrentResult struct {
	Profile   string `json:"profile"`
	Source    string `json:"source"`
	Workspace string `json:"workspace,omitempty"`
}

func (n *daemonNativeTools) profileToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDProfileList:    {call: n.profileList, availability: availability},
		toolspkg.ToolIDProfileCurrent: {call: n.profileCurrent, availability: availability},
	}
}

func (n *daemonNativeTools) profileList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	profiles, err := n.deps.Profiles.List(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	items := make([]nativeProfileListItem, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, nativeProfileListItem{
			Name: profile.Name, State: string(profile.State), Current: profile.ID == profileID,
			WorkItems: profile.WorkItems, NeedsSetup: profile.NeedsSetup,
		})
	}
	return structuredResult(map[string]any{"profiles": items}, "profile catalog")
}

func (n *daemonNativeTools) profileCurrent(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	if err := decodeNativeInput(req, &struct{}{}); err != nil {
		return toolspkg.ToolResult{}, err
	}
	profileID, source, workspaceID, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	profiles, err := n.deps.Profiles.List(ctx)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	name := ""
	for _, profile := range profiles {
		if profile.ID == profileID {
			name = profile.Name
			break
		}
	}
	if name == "" {
		return toolspkg.ToolResult{}, errors.New("daemon: bound profile was not found")
	}
	workspaceName := workspaceID
	if workspaceID != "" && n.deps.Workspaces != nil {
		if workspace, getErr := n.deps.Workspaces.Get(ctx, workspaceID); getErr == nil {
			workspaceName = workspace.Name
		}
	}
	result := nativeProfileCurrentResult{Profile: name, Source: source, Workspace: workspaceName}
	return structuredResult(result, "current profile "+name)
}

func (n *daemonNativeTools) nativeCurrentProfileIdentity(
	ctx context.Context,
	scope toolspkg.Scope,
) (profileID, source, workspaceID string, err error) {
	requestedProfileID := strings.TrimSpace(scope.ProfileID)
	profileID, source = requestedProfileID, "scope"
	if profileID == "" {
		profileID, source = store.DefaultProfileID, "default"
	}
	workspaceID = strings.TrimSpace(scope.WorkspaceID)
	if sessionID := strings.TrimSpace(scope.SessionID); sessionID != "" {
		if n.deps.Sessions == nil {
			return "", "", "", errors.New("daemon: session profile lookup is unavailable")
		}
		info, statusErr := n.deps.Sessions.Status(ctx, sessionID)
		if statusErr != nil {
			return "", "", "", statusErr
		}
		sessionProfileID := strings.TrimSpace(info.ProfileID)
		if sessionProfileID == "" {
			return "", "", "", errors.New("daemon: session has no bound profile")
		}
		if requestedProfileID != "" && requestedProfileID != sessionProfileID {
			return "", "", "", &profilepkg.Error{
				Code: "profile_session_conflict", Message: "session is bound to another profile",
				Action: "drop the acting profile override; the session decides", Cause: profilepkg.ErrSessionConflict,
			}
		}
		profileID, source, workspaceID = sessionProfileID, "session", strings.TrimSpace(info.WorkspaceID)
	}
	return profileID, source, workspaceID, nil
}

func (n *daemonNativeTools) nativeProfileReadScope(
	ctx context.Context,
	scope toolspkg.Scope,
) (store.ReadScope, error) {
	profileID, _, _, err := n.nativeCurrentProfileIdentity(ctx, scope)
	if err != nil {
		return store.ReadScope{}, err
	}
	readScope := store.ReadScope{ProfileID: profileID}
	if err := readScope.Validate(); err != nil {
		return store.ReadScope{}, err
	}
	return readScope, nil
}
