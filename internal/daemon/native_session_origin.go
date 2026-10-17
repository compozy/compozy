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

type nativeSessionMessageOriginBuilder interface {
	SessionMessageOrigin(context.Context, session.MessageOriginRequest) (*acp.PromptOriginMeta, error)
}

func (n *daemonNativeTools) sessionPromptOrigin(
	ctx context.Context,
	scope toolspkg.Scope,
	toolID toolspkg.ToolID,
	target string,
	input sessionPromptInput,
) (*acp.PromptOriginMeta, error) {
	if scope.Operator || strings.TrimSpace(scope.SessionID) == "" {
		if input.NotifyOnComplete {
			return nil, toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidRequest,
				toolID,
				"notify_on_complete requires an agent session",
				toolspkg.ErrToolInvalidInput,
			)
		}
		if strings.TrimSpace(input.InReplyTo) != "" {
			return nil, toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidRequest,
				toolID,
				"in_reply_to requires an agent session",
				toolspkg.ErrToolInvalidInput,
			)
		}
		if input.Supersede {
			return nil, toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidRequest,
				toolID,
				"supersede requires an agent session",
				toolspkg.ErrToolInvalidInput,
			)
		}
		return nil, nil
	}
	builder, ok := n.deps.Sessions.(nativeSessionMessageOriginBuilder)
	if !ok {
		return nil, errors.New("session message origin builder is unavailable")
	}
	origin, err := builder.SessionMessageOrigin(ctx, session.MessageOriginRequest{
		SenderSessionID: scope.SessionID, AgentName: scope.AgentName, TargetSessionID: target,
		InReplyTo: input.InReplyTo, NotifyOnComplete: input.NotifyOnComplete,
	})
	if err != nil {
		return nil, nativeSessionMessageError(toolID, err)
	}
	return origin, nil
}

// nativeSessionMessageError maps the session message guards to their stable tool error codes.
func nativeSessionMessageError(toolID toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, session.ErrSessionMessageSelfTarget):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidRequest, toolID, err.Error(), toolspkg.ErrToolInvalidInput,
		)
	case errors.Is(err, session.ErrSessionMessageHopLimit):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeSessionMessageHopLimit, toolID, session.ErrSessionMessageHopLimit.Error(), err,
		)
	case errors.Is(err, session.ErrSessionMessageRateLimited):
		return toolspkg.NewToolError(toolspkg.ErrorCodeSessionMessageRateLimited, toolID, err.Error(), err)
	case errors.Is(err, acp.ErrInvalidPromptMetadata):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidRequest, toolID, err.Error(), toolspkg.ErrToolInvalidInput,
		)
	}
	return err
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
		"in_reply_to",
		origin.InReplyTo,
		"mode",
		mode,
	)
}

type nativeSessionReplayReader interface {
	PromptAdmissionReplayOrigin(context.Context, string, string, string, string) (bool, *acp.PromptOriginMeta, error)
}

// sessionPromptReplayOrigin lets a retry of an already admitted send reach its original receipt when a new
// send would now be refused (self-target since the upgrade, or a hop raised by a later steer). Only the
// sender that admitted the message may replay it; any other caller keeps the refusal, and a changed request
// still fails the admission fingerprint.
func (n *daemonNativeTools) sessionPromptReplayOrigin(
	ctx context.Context,
	scope toolspkg.Scope,
	workspaceID, target, messageID, idempotencyKey string,
	refusal error,
) (*acp.PromptOriginMeta, error) {
	if scope.Operator || strings.TrimSpace(scope.SessionID) == "" {
		return nil, refusal
	}
	reader, ok := n.deps.Sessions.(nativeSessionReplayReader)
	if !ok {
		return nil, refusal
	}
	found, origin, err := reader.PromptAdmissionReplayOrigin(ctx, workspaceID, target, messageID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	if !found || (origin != nil && origin.SessionID != scope.SessionID) {
		return nil, refusal
	}
	return origin, nil
}
