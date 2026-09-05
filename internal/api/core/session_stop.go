package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

// SessionStopManager owns shared asynchronous stop requests and their outcomes.
type SessionStopManager interface {
	RequestStop(context.Context, string, session.StopCause) error
	AwaitStopped(context.Context, string) (session.StopOutcome, error)
}

func (h *BaseHandlers) stopSessionWithResult(c *gin.Context, info *session.Info, wait bool) {
	if info.State == session.StateStopped {
		c.JSON(http.StatusOK, contract.SessionStopPayload{
			SessionID: info.ID, Status: "already-stopped", State: info.State,
			Verified: true, Escalated: info.StopEscalated,
		})
		return
	}
	manager, ok := h.Sessions.(SessionStopManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session stop manager is required"))
		return
	}
	ctx := c.Request.Context()
	if err := manager.RequestStop(ctx, info.ID, session.CauseUserRequested); err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if !wait {
		c.JSON(http.StatusAccepted, contract.SessionStopPayload{
			SessionID: info.ID, Status: "stopping", State: session.StateStopping,
			StopCause: session.CauseUserRequested.String(), Escalated: info.StopEscalated,
		})
		return
	}
	outcome, err := manager.AwaitStopped(ctx, info.ID)
	if err != nil && !errors.Is(err, session.ErrStopVerificationFailed) {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusOK, SessionStopOutcomePayload(info.ID, outcome))
}

// SessionStopOutcomePayload projects the shared stop result without fabricating termination.
func SessionStopOutcomePayload(id string, outcome session.StopOutcome) contract.SessionStopPayload {
	payload := contract.SessionStopPayload{
		SessionID: id, Status: string(outcome.FinalState), State: outcome.FinalState,
		Verified: outcome.Verified, Escalated: outcome.Escalated,
		StopCause: outcome.Cause.String(), Phase: outcome.Phase, StoppedAfter: outcome.Elapsed.String(),
	}
	if !outcome.Verified {
		payload.State, payload.Status = session.StateStopping, "stopping"
		payload.Attention = session.StopVerificationFailedCode
	}
	return payload
}

func sessionStopAttention(info *session.Info) string {
	if info != nil && info.StopVerificationFailed && info.State != session.StateStopped {
		return session.StopVerificationFailedCode
	}
	return ""
}
