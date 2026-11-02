package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/workspaceaccess"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/session"

	"github.com/compozy/compozy/internal/store"

	toolspkg "github.com/compozy/compozy/internal/tools"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type nativeBoundSessionScope struct {
	sessionID   string
	workspaceID string
}

func (n *daemonNativeTools) nativeBoundSession(
	ctx context.Context,
	scope toolspkg.Scope,
) (*nativeBoundSessionScope, error) {
	sessionID := strings.TrimSpace(scope.SessionID)
	if sessionID == "" {
		return nil, nil
	}
	if n == nil || n.deps == nil || n.deps.Sessions == nil {
		return nil, errors.New("daemon: sessions are required")
	}
	info, err := n.deps.Sessions.Status(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	bound := &nativeBoundSessionScope{
		sessionID:   strings.TrimSpace(info.ID),
		workspaceID: strings.TrimSpace(info.WorkspaceID),
	}
	return bound, nil
}

func nativeBoundWorkspaceRef(bound *nativeBoundSessionScope, workspaceRef string) string {
	if bound == nil {
		return workspaceRef
	}
	return bound.workspaceID
}

func nativeBoundSessionID(bound *nativeBoundSessionScope, values ...string) string {
	if bound == nil {
		return firstNonEmpty(values...)
	}
	return bound.sessionID
}

func (n *daemonNativeTools) nativeResolvedWorkspace(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (workspacepkg.ResolvedWorkspace, error) {
	ref := nativeCallerWorkspaceInput(workspaceRef, scope)
	if ref == "" {
		return workspacepkg.ResolvedWorkspace{}, nativeRequiredInputError(id, nativeWorkspaceInputKey)
	}
	if n == nil || n.deps == nil || n.deps.Workspaces == nil {
		return workspacepkg.ResolvedWorkspace{}, nativeInputError(
			id,
			workspacepkg.ErrWorkspaceResolverUnavailable,
		)
	}
	if workspacepkg.ParseRef(ref).Kind == workspacepkg.RefUser {
		resolver, ok := n.deps.Workspaces.(workspacepkg.UserRuntimeResolver)
		if !ok {
			return workspacepkg.ResolvedWorkspace{}, nativeInputError(id, workspacepkg.ErrUserScopeUnsupported)
		}
		return resolver.ResolveUser(ctx, scope.ProfileID)
	}
	resolved, err := n.deps.Workspaces.Resolve(ctx, ref)
	if errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) && nativeAllowsMissingWorkspace(id) {
		registration, getErr := n.deps.Workspaces.Get(ctx, ref)
		if getErr != nil {
			return workspacepkg.ResolvedWorkspace{}, nativeInputError(id, getErr)
		}
		registration.RootMissing = true
		return workspacepkg.ResolvedWorkspace{
			Workspace: registration, WorkspaceID: registration.ID, Scope: workspacepkg.ScopeWorkspace,
		}, nil
	}
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, nativeInputError(id, err)
	}
	return resolved, nil
}

func nativeAllowsMissingWorkspace(id toolspkg.ToolID) bool {
	switch id {
	case toolspkg.ToolIDWorktreeRemove,
		toolspkg.ToolIDSessionStatus,
		toolspkg.ToolIDSessionDescribe,
		toolspkg.ToolIDSessionArchive,
		toolspkg.ToolIDSessionUnarchive,
		toolspkg.ToolIDSessionLater,
		toolspkg.ToolIDSessionResumeLater:
		return true
	default:
		return false
	}
}

func nativeCallerWorkspaceInput(
	value string,
	scope toolspkg.Scope,
) string {
	trusted := strings.TrimSpace(scope.WorkspaceID)
	current := strings.TrimSpace(value)
	if current != "" {
		return current
	}
	if trusted == "" && scope.SessionID != "" {
		return workspaceaccess.UserSelector
	}
	return trusted
}

func nativeScopeMismatchError(id toolspkg.ToolID, field string) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		fmt.Sprintf("tool %q call %s conflicts with caller scope", id, field),
		toolspkg.ErrToolDenied,
		toolspkg.ReasonScopeMismatch,
	)
}

func (n *daemonNativeTools) nativeWorkspaceID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceRef string,
	scope toolspkg.Scope,
) (string, error) {
	resolved, err := n.nativeResolvedWorkspace(ctx, id, workspaceRef, scope)
	if err != nil {
		return "", err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return "", nativeInputError(id, err)
	}
	return workspaceID, nil
}

func nativeResolvedWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("daemon: resolved workspace is required")
	}
	workspaceID := strings.TrimSpace(resolved.WorkspaceID)
	if workspaceID == "" {
		return "", errors.New("daemon: resolved workspace_id is empty")
	}
	return workspaceID, nil
}

func nativeResolvedRegistryWorkspaceID(resolved *workspacepkg.ResolvedWorkspace) (string, error) {
	if resolved == nil {
		return "", errors.New("daemon: resolved workspace is required")
	}
	if resolved.Scope == workspacepkg.ScopeUser {
		if resolved.ID != "" || resolved.WorkspaceID != "" {
			return "", errors.New("daemon: User resolution cannot carry a workspace id")
		}
		return "", nil
	}
	workspaceID := strings.TrimSpace(resolved.ID)
	if workspaceID == "" {
		return "", errors.New("daemon: resolved workspace registry id is empty")
	}
	return workspaceID, nil
}

func (n *daemonNativeTools) nativeSessionInWorkspace(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, nativeRequiredInputError(id, "session_id")
	}
	if n == nil || n.deps == nil || n.deps.Sessions == nil {
		return nil, errors.New("daemon: sessions are required")
	}
	info, err := sessionInWorkspace(ctx, n.deps.Sessions, workspaceID, sessionID)
	if err == nil && workspaceID == "" && (info.ProfileID != scope.ProfileID || scope.ProfileID == "") {
		return nil, nativeWorkspaceAccessDeniedError(id)
	}
	if errors.Is(err, errSessionOutsideWorkspace) {
		return nil, nativeWorkspaceAccessDeniedError(id)
	}
	return info, err
}

// errSessionOutsideWorkspace reports a session that does not belong to the requested workspace.
var errSessionOutsideWorkspace = errors.New("daemon: session belongs to another workspace")

// sessionInWorkspace is the workspace-scoped session lookup shared by compozy__session_history and
// session mention references: the session must exist and belong to workspaceID.
func sessionInWorkspace(
	ctx context.Context,
	sessions nativeToolPolicySessionReader,
	workspaceID string,
	sessionID string,
) (*session.Info, error) {
	info, err := sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if info != nil && info.Type == session.SessionTypeTriage {
		return nil, session.ErrSessionNotFound
	}
	if info == nil || strings.TrimSpace(info.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return nil, errSessionOutsideWorkspace
	}
	return info, nil
}

func trimNativeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if next := strings.TrimSpace(value); next != "" {
			trimmed = append(trimmed, next)
		}
	}
	return trimmed
}

func decodeSessionEventQueryInput(req toolspkg.CallRequest) (sessionEventQueryInput, store.EventQuery, error) {
	var input sessionEventQueryInput
	if err := decodeNativeInput(req, &input); err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	sessionID, err := requiredNativeString(req.ToolID, "session_id", input.SessionID)
	if err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	input.SessionID = sessionID
	query, err := input.eventQuery(req.ToolID)
	if err != nil {
		return sessionEventQueryInput{}, store.EventQuery{}, err
	}
	return input, query, nil
}
