package daemon

import (
	"context"
	"errors"
	"strings"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type sessionLaterInput struct {
	Workspace    string     `json:"workspace,omitempty"`
	SessionID    string     `json:"session_id"`
	Reason       string     `json:"reason,omitempty"`
	ReturnAt     *time.Time `json:"return_at,omitempty"`
	WithChildren bool       `json:"with_children,omitempty"`
}

// nativeLaterOperationTimeout keeps an accepted park running when the calling
// turn ends; stopping a session tree outlives one tool call.
const nativeLaterOperationTimeout = 3 * time.Minute

func (n *daemonNativeTools) sessionLaterToolBindings(
	catalogAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDSessionLater:       {call: n.sessionLater, availability: catalogAvailability},
		toolspkg.ToolIDSessionResumeLater: {call: n.sessionResumeLater, availability: catalogAvailability},
	}
}

func (n *daemonNativeTools) sessionLater(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.changeSessionLater(ctx, scope, req, true)
}

func (n *daemonNativeTools) sessionResumeLater(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	return n.changeSessionLater(ctx, scope, req, false)
}

func (n *daemonNativeTools) changeSessionLater(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	park bool,
) (toolspkg.ToolResult, error) {
	var input sessionLaterInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	resolved, err := n.nativeResolvedWorkspace(ctx, req.ToolID, input.Workspace, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := nativeResolvedRegistryWorkspaceID(&resolved)
	if err != nil {
		return toolspkg.ToolResult{}, nativeInputError(req.ToolID, err)
	}
	if park {
		// Parking the caller or one of its ancestors would stop the turn running this call.
		within, err := n.nativeCallerWithinLaterTree(ctx, scope.SessionID, input.SessionID)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		if within {
			return toolspkg.ToolResult{}, nativeSelfTargetError(req.ToolID)
		}
	}
	if _, err := n.nativeSessionInWorkspace(ctx, scope, req.ToolID, workspaceID, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	manager, ok := n.deps.Sessions.(core.SessionLaterManager)
	if !ok {
		return toolspkg.ToolResult{}, errors.New("daemon: session later manager is required")
	}
	var info *session.Info
	if park {
		if err := ctx.Err(); err != nil {
			return toolspkg.ToolResult{}, err
		}
		parkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), nativeLaterOperationTimeout)
		defer cancel()
		info, err = manager.MoveToLater(parkCtx, workspaceID, input.SessionID, session.LaterRequest{
			Reason: input.Reason, ReturnAt: input.ReturnAt,
		})
	} else {
		info, err = manager.ResumeFromLater(ctx, workspaceID, input.SessionID, input.WithChildren)
	}
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload := core.SessionPayloadFromInfo(info)
	return structuredResult(map[string]any{nativeToolsSessionKey: payload}, payload.ID)
}

// nativeCallerWithinLaterTree reports whether parking target would park the
// calling session: the caller is target or one of its hierarchy descendants.
// Continue and fork links name a source session without being its child.
func (n *daemonNativeTools) nativeCallerWithinLaterTree(
	ctx context.Context,
	callerID string,
	target string,
) (bool, error) {
	target = strings.TrimSpace(target)
	current := strings.TrimSpace(callerID)
	visited := make(map[string]struct{})
	for current != "" {
		if _, seen := visited[current]; seen {
			return false, nil
		}
		visited[current] = struct{}{}
		if current == target {
			return true, nil
		}
		info, err := n.deps.Sessions.Status(ctx, current)
		if errors.Is(err, session.ErrSessionNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info == nil {
			return false, nil
		}
		current = strings.TrimSpace(store.HierarchyParentSessionID(info.Lineage))
	}
	return false, nil
}
