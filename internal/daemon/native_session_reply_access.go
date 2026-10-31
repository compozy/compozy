package daemon

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type nativeSessionReplyReader interface {
	CanReplyToSession(context.Context, string, string, string) (bool, error)
}

// nativeSessionReplyTarget proves a text-only reply to a persisted sender in the
// caller's profile. A nil target leaves ordinary workspace authorization in force.
func nativeSessionReplyTarget(
	ctx context.Context,
	sessions nativeToolPolicySessionReader,
	scope toolspkg.Scope,
	input sessionPromptInput,
) (*session.Info, error) {
	reader, ok := sessions.(nativeSessionReplyReader)
	if !ok || scope.Operator || scope.SessionID == "" || input.InReplyTo == "" {
		return nil, nil
	}
	// A reply grants message delivery only, never file access or runtime reconfiguration.
	if len(input.Attachments) != 0 || input.Runtime != nil {
		return nil, nil
	}
	allowed, err := reader.CanReplyToSession(ctx, scope.SessionID,
		strings.TrimSpace(input.SessionID), strings.TrimSpace(input.InReplyTo))
	if err != nil || !allowed {
		return nil, err
	}
	info, err := sessions.Status(ctx, strings.TrimSpace(input.SessionID))
	if err != nil {
		return nil, err
	}
	if info == nil || info.ProfileID != scope.ProfileID || scope.ProfileID == "" {
		return nil, nil
	}
	return info, nil
}

// nativeSessionPromptWorkspace resolves a verified reply's destination without
// granting the caller access to any other operation in that workspace.
func (n *daemonNativeTools) nativeSessionPromptWorkspace(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input sessionPromptInput,
) (workspacepkg.ResolvedWorkspace, error) {
	target, err := nativeSessionReplyTarget(ctx, n.deps.Sessions, scope, input)
	if err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	if target != nil {
		input.Workspace = nativeReplyWorkspace(target)
	}
	return n.nativeResolvedWorkspace(ctx, id, input.Workspace, scope)
}

// nativeReplyWorkspace translates stored User ownership into its API selector.
func nativeReplyWorkspace(info *session.Info) string {
	if info.WorkspaceID == "" {
		return workspaceaccess.UserSelector
	}
	return info.WorkspaceID
}
