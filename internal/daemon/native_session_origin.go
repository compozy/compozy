package daemon

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type nativeSessionHopReader interface {
	CurrentTurnEffectiveHop(context.Context, string) (int, error)
}

func (n *daemonNativeTools) sessionPromptOrigin(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	target string,
	notify bool,
) (*acp.PromptOriginMeta, error) {
	if scope.Operator || strings.TrimSpace(scope.SessionID) == "" {
		if notify {
			return nil, nativeInputError(toolID, errors.New("notify_on_complete requires an agent session"))
		}
		return nil, nil
	}
	if target == scope.SessionID {
		slog.WarnContext(ctx, "session.message.rejected", "reason", "self_target", "sender_session_id", scope.SessionID)
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidRequest,
			toolID,
			"session_prompt cannot target the calling session.",
			toolspkg.ErrToolInvalidInput,
		)
	}
	reader, ok := n.deps.Sessions.(nativeSessionHopReader)
	if !ok {
		return nil, errors.New("session message hop reader is unavailable")
	}
	hop, err := reader.CurrentTurnEffectiveHop(ctx, scope.SessionID)
	if err != nil {
		return nil, err
	}
	if hop >= acp.MaxSessionMessageHops {
		slog.WarnContext(ctx, "session.message.rejected", "reason", "hop_limit", "sender_session_id", scope.SessionID)
		return nil, toolspkg.NewToolError(
			toolspkg.ErrorCodeSessionMessageHopLimit,
			toolID,
			session.ErrSessionMessageHopLimit.Error(),
			session.ErrSessionMessageHopLimit,
		)
	}
	sender, err := n.deps.Sessions.Status(ctx, scope.SessionID)
	if err != nil {
		return nil, err
	}
	origin := acp.PromptOriginMeta{Kind: acp.PromptOriginKindSession, SessionID: scope.SessionID,
		WorkspaceID: sender.WorkspaceID, AgentName: scope.AgentName, TitleAtSend: sender.Name,
		Hop: hop + 1, NotifyOnComplete: notify}.Normalize()
	if err := origin.Validate(); err != nil {
		return nil, err
	}
	return &origin, nil
}

func nativeSessionPromptCaller(scope toolspkg.Scope, toolID toolspkg.ToolID) session.PromptCaller {
	kind, id := "agent", firstNonEmpty(scope.AgentName, scope.SessionID)
	if scope.Operator {
		kind, id = "human", "operator"
	}
	return session.PromptCaller{Kind: kind, ID: id, Source: toolID.String()}
}

func logSessionMessageSent(
	ctx context.Context,
	origin *acp.PromptOriginMeta,
	target, messageID string,
	mode session.BusyInputMode,
) {
	if origin == nil {
		return
	}
	slog.InfoContext(
		ctx,
		"session.message.sent",
		"sender_session_id",
		origin.SessionID,
		"target_session_id",
		target,
		"message_id",
		messageID,
		"hop",
		origin.Hop,
		"notify",
		origin.NotifyOnComplete,
		"mode",
		mode,
	)
}
