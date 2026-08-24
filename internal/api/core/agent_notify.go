package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

const agentActionNotify = "agent.notify"

type agentNotificationPublisher interface {
	PublishOperatorNotification(context.Context, session.NotifyRequest) (session.NotifyResult, error)
}

// AgentNotify publishes a bounded notification for the validated agent caller.
func (h *BaseHandlers) AgentNotify(c *gin.Context) {
	publisher, ok := h.Sessions.(agentNotificationPublisher)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: notification service is not configured"))
		return
	}
	caller, ok := h.requireAgentCaller(c, agentActionNotify)
	if !ok {
		return
	}
	var req contract.AgentNotifyRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusUnprocessableEntity, err)
		return
	}
	result, err := publisher.PublishOperatorNotification(c.Request.Context(), session.NotifyRequest{
		SessionID: caller.Session.ID, ProfileID: caller.Session.ProfileID, WorkspaceID: caller.Session.WorkspaceID,
		Title: req.Title, Body: req.Body,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, session.ErrInvalidNotification) {
			status = http.StatusUnprocessableEntity
		}
		h.respondError(c, status, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentNotifyResponse{
		Outcome: string(result.Outcome), RetryAfterMS: result.RetryAfterMS,
	})
}
