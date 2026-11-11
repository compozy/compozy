package daemon

import (
	"context"
	"errors"
	"strings"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
)

type sessionMoveToProfileInput struct {
	Workspace     string `json:"workspace,omitempty"`
	SessionID     string `json:"session_id"`
	TargetProfile string `json:"target_profile"`
}

// sessionMoveToProfile copies a session of the caller's workspace and profile into
// another profile. An agent never moves itself, and its write to the target profile
// passes the same workspace-access policy as any other cross-profile write.
func (n *daemonNativeTools) sessionMoveToProfile(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input sessionMoveToProfileInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(input.SessionID) != "" && strings.TrimSpace(input.SessionID) == scope.SessionID {
		return toolspkg.ToolResult{}, nativeSelfTargetError(req.ToolID)
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	source, err := n.nativeReparentTarget(ctx, scope, req.ToolID, workspaceID, input.SessionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	targetProfile := strings.TrimSpace(input.TargetProfile)
	if targetProfile == "" {
		return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "target_profile")
	}
	targetID, err := n.nativeProfileTarget(ctx, scope, req, targetProfile)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := n.authorizeNativeProfileMove(ctx, scope, req, targetProfile, targetID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	mover, ok := n.deps.Sessions.(core.SessionProfileMover)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session profile move is unavailable")
	}
	if !scope.Operator {
		ctx = session.WithActingSession(ctx, scope.SessionID)
	}
	result, err := mover.MoveToProfile(ctx, session.MoveToProfileOpts{
		WorkspaceID: workspaceID, ProfileID: source.ProfileID,
		SessionID: source.ID, TargetProfileID: targetID,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeProfileMoveError(req.ToolID, err)
	}
	copyPayload := core.SessionPayloadFromInfo(result.Copy)
	return sessionMoveToProfileViews.result(req.View, sessionMoveToProfileResult{
		Session: copyPayload, Original: core.SessionPayloadFromInfo(result.Original),
		WorktreeID: result.WorktreeID, Branch: result.Branch,
	})
}

// authorizeNativeProfileMove admits an agent's write to the target profile through the
// workspace-access policy; an operator call needs no grant, and any other call without
// a session is denied.
func (n *daemonNativeTools) authorizeNativeProfileMove(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	targetProfile, targetID string,
) error {
	if scope.Operator {
		return nil
	}
	if scope.SessionID == "" {
		// Only an operator call or an agent session can name who writes the target profile.
		return nativeWorkspaceAccessDeniedError(req.ToolID)
	}
	for _, descriptor := range builtintools.NativeDescriptors() {
		if descriptor.ID == req.ToolID {
			selection := nativeProfileSelectionInput{requested: targetProfile, present: true}
			return n.authorizeNativeProfileSwitch(ctx, scope, req, descriptor, selection, targetID)
		}
	}
	return nativeWorkspaceAccessDeniedError(req.ToolID)
}

func nativeProfileMoveError(id toolspkg.ToolID, err error) error {
	if errors.Is(err, session.ErrProfileMoveRefused) {
		if refusal, ok := errors.AsType[*session.ProfileMoveError](err); ok &&
			refusal.Reason == session.ProfileMoveRefusalOwnSession {
			return nativeSelfTargetError(id)
		}
	}
	if code := core.SessionProfileMoveErrorCode(err); code != "" {
		return toolspkg.NewToolError(toolspkg.ErrorCode(code), id, err.Error(), err)
	}
	return nativeHTTPStatusToolError(id, err, core.StatusForSessionError(err))
}
