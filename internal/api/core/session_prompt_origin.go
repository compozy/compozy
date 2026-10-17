package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

type sessionMessageOriginBuilder interface {
	SessionMessageOrigin(context.Context, session.MessageOriginRequest) (*acp.PromptOriginMeta, error)
}

type sessionPromptReplayReader interface {
	PromptAdmissionReplayOrigin(context.Context, string, string, string, string) (bool, *acp.PromptOriginMeta, error)
}

var (
	errPromptReplyRequiresAgentSession     = errors.New("in_reply_to requires a calling agent session")
	errPromptSupersedeRequiresAgentSession = errors.New("supersede requires a calling agent session")
)

// sessionPromptOrigin stamps a session origin for prompts sent from inside a managed agent session.
//
// The calling session comes only from the daemon-validated agent identity headers (the CLI sends them from
// COMPOZY_SESSION_ID or --from-session). Calls without them stay operator input, and a session addressing
// itself keeps the historical plain prompt.
func (h *BaseHandlers) sessionPromptOrigin(
	c *gin.Context,
	workspaceID, targetSessionID string,
	request contract.SendPromptRequest,
	input contract.PromptInput,
) (*acp.PromptOriginMeta, error) {
	inReplyTo := strings.TrimSpace(request.InReplyTo)
	if err := acp.ValidatePromptOriginInReplyTo(inReplyTo); err != nil {
		return nil, invalidPromptRequestError{cause: err}
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		if inReplyTo != "" {
			return nil, invalidPromptRequestError{cause: errPromptReplyRequiresAgentSession}
		}
		if request.Supersede {
			return nil, invalidPromptRequestError{cause: errPromptSupersedeRequiresAgentSession}
		}
		return nil, nil
	}
	ctx := c.Request.Context()
	caller, err := h.resolveAgentCallerForWorkspace(ctx, credentials, "session_prompt", workspaceID)
	if err != nil {
		return nil, err
	}
	sender := strings.TrimSpace(caller.Session.ID)
	if sender == "" || sender == targetSessionID {
		return nil, nil
	}
	builder, ok := h.Sessions.(sessionMessageOriginBuilder)
	if !ok {
		return nil, nil
	}
	origin, err := builder.SessionMessageOrigin(ctx, session.MessageOriginRequest{
		SenderSessionID: sender, AgentName: caller.Session.AgentName, TargetSessionID: targetSessionID,
		InReplyTo: inReplyTo,
	})
	if err == nil {
		return origin, nil
	}
	return h.sessionPromptReplayOrigin(ctx, workspaceID, targetSessionID, sender, input, err)
}

// sessionPromptReplayOrigin lets a retry of an admitted send reach its receipt after a guard would refuse it.
func (h *BaseHandlers) sessionPromptReplayOrigin(
	ctx context.Context,
	workspaceID, targetSessionID, sender string,
	input contract.PromptInput,
	refusal error,
) (*acp.PromptOriginMeta, error) {
	reader, ok := h.Sessions.(sessionPromptReplayReader)
	if !ok {
		return nil, refusal
	}
	found, origin, err := reader.PromptAdmissionReplayOrigin(
		ctx, workspaceID, targetSessionID, input.MessageID, input.IdempotencyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("api: read prompt admission replay: %w", err)
	}
	if !found || origin == nil || origin.SessionID != sender {
		return nil, refusal
	}
	return origin, nil
}

func (h *BaseHandlers) respondPromptOriginError(c *gin.Context, err error) {
	if _, ok := errors.AsType[invalidPromptRequestError](err); ok {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	h.respondError(c, StatusForSessionError(err), err)
}
