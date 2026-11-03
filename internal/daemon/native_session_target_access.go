package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

const nativeSessionTargetInputKey = "session_id"

// nativeSessionTargetID returns the top-level target session a session tool names. A schema
// without both session_id and workspace has no session target the boundary resolves, and
// session_describe always describes the calling session.
func nativeSessionTargetID(
	id toolspkg.ToolID,
	schema map[string]json.RawMessage,
	payload map[string]json.RawMessage,
) string {
	if id == toolspkg.ToolIDSessionDescribe {
		return ""
	}
	properties := nativeWorkspaceSchemaProperties(schema)
	if _, ok := properties[nativeSessionTargetInputKey]; !ok {
		return ""
	}
	if _, ok := properties[nativeWorkspaceInputKey]; !ok {
		return ""
	}
	value, ok := nativeWorkspaceInputValue(payload[nativeSessionTargetInputKey])
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// nativeSessionTarget is a target session resolved by id. A nil info means no visible session
// has that id; the access request is then unresolved.
type nativeSessionTarget struct {
	sessionID string
	info      *session.Info
}

func (t nativeSessionTarget) ref() workspaceaccess.TargetRef {
	if t.info == nil {
		return workspaceaccess.TargetRef{Kind: workspaceaccess.TargetUnresolved}
	}
	return workspaceaccess.ScopeTarget(strings.TrimSpace(t.info.WorkspaceID), strings.TrimSpace(t.info.ProfileID))
}

// workspaceSelector is the workspace input that addresses the target session's owner.
func (t nativeSessionTarget) workspaceSelector() string {
	if t.info == nil {
		return ""
	}
	return nativeReplyWorkspace(t.info)
}

// promptLabel is identical for found and missing sessions, so the prompt reveals nothing.
func (t nativeSessionTarget) promptLabel() string {
	return nativeAccessPromptSessionPrefix + t.sessionID
}

func (b *nativeWorkspaceInputBinder) resolveNativeSessionTarget(
	ctx context.Context,
	sessionID string,
) (nativeSessionTarget, error) {
	target := nativeSessionTarget{sessionID: sessionID}
	if b == nil || b.sessions == nil {
		return target, errors.New("daemon: native session target lookup is unavailable")
	}
	info, err := b.sessions.Status(ctx, sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		return target, nil
	}
	if err != nil {
		return target, fmt.Errorf("daemon: resolve native session target: %w", err)
	}
	if info == nil || info.Type == session.SessionTypeTriage || strings.TrimSpace(info.ID) != sessionID {
		return target, nil
	}
	target.info = info
	return target, nil
}

// bindNativeSessionTargetWorkspace fills an omitted workspace from the named target session, so
// a foreign session id is authorized against its real owner instead of the caller's workspace.
// A missing session keeps the caller's workspace; the authorizer answers for it.
func (b *nativeWorkspaceInputBinder) bindNativeSessionTargetWorkspace(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	schema map[string]json.RawMessage,
	payload map[string]json.RawMessage,
) error {
	if nativeWorkspaceScopeHasOperatorAuthority(scope) || strings.TrimSpace(scope.SessionID) == "" {
		return nil
	}
	sessionID := nativeSessionTargetID(id, schema, payload)
	if sessionID == "" || sessionID == strings.TrimSpace(scope.SessionID) {
		return nil
	}
	if requested, _ := nativeWorkspaceInputValue(payload[nativeWorkspaceInputKey]); strings.TrimSpace(requested) != "" {
		return nil
	}
	// Lookup failures keep the caller's workspace; the authorizer reports them without
	// revealing whether the session exists.
	target, lookupErr := b.resolveNativeSessionTarget(ctx, sessionID)
	if lookupErr == nil && target.info != nil &&
		strings.TrimSpace(target.info.WorkspaceID) != strings.TrimSpace(scope.WorkspaceID) &&
		b.nativeSessionOwnerBindable(ctx, id, target.workspaceSelector()) {
		return setNativeWorkspaceInput(payload, target.workspaceSelector())
	}
	return nil
}

// nativeSessionOwnerBindable reports whether the owner workspace of a named session binds
// without an input error. An owner that cannot bind (its project folder is gone) is not filled
// in: binding would fail before authorization and tell a denied caller the session exists. The
// authorizer still decides the call against the session's real owner.
func (b *nativeWorkspaceInputBinder) nativeSessionOwnerBindable(
	ctx context.Context,
	id toolspkg.ToolID,
	selector string,
) bool {
	if selector == workspaceaccess.UserSelector {
		return true
	}
	if b == nil || b.workspaces == nil {
		return false
	}
	_, err := b.workspaces.Resolve(ctx, selector)
	return err == nil || (errors.Is(err, workspacepkg.ErrWorkspaceRootMissing) && nativeAllowsMissingWorkspace(id))
}

// authorizeNativeSessionTarget authorizes the named target session against its own workspace and
// profile. It returns the workspace selector the decision covers, so the caller does not ask
// again for the same owner.
// nativeSessionTargetRequest is the actor and named session one native call authorizes.
type nativeSessionTargetRequest struct {
	actor          workspaceaccess.ActorRef
	actorProfileID string
	sessionID      string
	// otherCrossing reports an explicit target outside the actor's workspace in the same call.
	otherCrossing bool
}

func (r nativeSessionTargetRequest) ownsTarget(info *session.Info) bool {
	if info == nil || strings.TrimSpace(info.WorkspaceID) != r.actor.WorkspaceID {
		return false
	}
	if r.actor.Kind == workspaceaccess.ActorDaemon {
		return r.actor.WorkspaceID != ""
	}
	return r.actorProfileID != "" && strings.TrimSpace(info.ProfileID) == r.actorProfileID
}

func (b *nativeWorkspaceInputBinder) authorizeNativeSessionTarget(
	ctx context.Context,
	scope toolspkg.Scope,
	descriptor toolspkg.Descriptor,
	call toolspkg.CallRequest,
	req nativeSessionTargetRequest,
) (string, error) {
	sessionID, actor := req.sessionID, req.actor
	if sessionID == "" || sessionID == actor.SessionID {
		return "", nil
	}
	if descriptor.ID == toolspkg.ToolIDSessionPrompt {
		var input sessionPromptInput
		if err := json.Unmarshal(call.Input, &input); err != nil {
			return "", err
		}
		reply, err := nativeSessionReplyTarget(ctx, b.sessions, scope, input)
		if err != nil {
			return "", err
		}
		if reply != nil {
			// A verified reply is authorized by the conversation, not by reach.
			return nativeReplyWorkspace(reply), nil
		}
	}
	target, err := b.resolveNativeSessionTarget(ctx, sessionID)
	if err != nil {
		return "", errors.Join(nativeWorkspaceAccessDeniedError(descriptor.ID), err)
	}
	if req.ownsTarget(target.info) {
		// The actor's own workspace and profile cross no boundary; handlers keep their checks.
		return target.workspaceSelector(), nil
	}
	if target.info == nil && (req.otherCrossing || actor.Kind == workspaceaccess.ActorDaemon) {
		// The explicit foreign workspace is authorized on its own, and daemon actors never cross;
		// a missing session is then reported by the handler.
		return "", nil
	}
	allowed, err := b.authorizeAccessTarget(ctx, scope, descriptor, call, actor, nativeAccessTarget{
		ref:   target.ref(),
		label: target.promptLabel(),
	})
	if err != nil {
		return "", err
	}
	if allowed && target.info != nil {
		toolspkg.MarkSessionTargetAuthorized(ctx, target.info.ID)
	}
	return target.workspaceSelector(), nil
}

// markNativeCrossing marks a handler-level admission outside the caller's own workspace and
// profile, so dispatch redacts the call's result like any other crossing.
func markNativeCrossing(ctx context.Context, decision workspaceaccess.Decision) {
	if decision.Allowed && decision.Source != workspaceaccess.SourceSameWorkspace {
		toolspkg.MarkOutsideCallerScope(ctx)
	}
}
