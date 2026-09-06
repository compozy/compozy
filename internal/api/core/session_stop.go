package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
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
			Verified: true, Escalated: info.StopEscalated, StopCause: sessionStopCause(info),
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

// sessionStopCause exposes the live cause or derives the public token from the
// existing durable classification when reading a stopped session after restart.
func sessionStopCause(info *session.Info) string {
	if info == nil {
		return ""
	}
	if info.StopCause != session.CauseNone {
		return info.StopCause.String()
	}
	switch info.StopReason {
	case store.StopCompleted:
		switch info.StopDetail {
		case "conversation cleared":
			return session.CauseClearConversation.String()
		case "conversation rewound":
			return session.CauseConversationRewind.String()
		default:
			return session.CauseCompleted.String()
		}
	case store.StopUserCanceled, store.StopMaxIterations, store.StopLoopDetected, store.StopBudgetExceeded:
		return session.CauseUserRequested.String()
	case store.StopTimeout:
		if info.StopDetail == "inactivity" {
			return session.CauseInactivity.String()
		}
		return session.CauseTimeout.String()
	case store.StopAgentCrashed:
		return session.CauseProcessExited.String()
	case store.StopHookStopped:
		return session.CauseHookDenied.String()
	case store.StopShutdown:
		return session.CauseShutdown.String()
	case store.StopError:
		if info.StopDetail == "process exited unexpectedly" {
			return session.CauseProcessExited.String()
		}
		return session.CauseFailed.String()
	default:
		return ""
	}
}
