package core

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

const (
	sessionWaitInvalidCode = "invalid_wait"
	sessionWaitStateCode   = "invalid_state"
	sessionWaitExpiredCode = "wait_expired"
	sessionWaitGoneCode    = "session_gone"
	sessionWaitLimitCode   = "wait_limit_exceeded"
	sessionWaitInUseCode   = "wait_in_use"
)

// SessionWait performs one request-coupled, bounded wait for a canonical badge.
func (h *BaseHandlers) SessionWait(c *gin.Context) {
	manager, ok := h.Sessions.(SessionWaitManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session wait is required"))
		return
	}
	var request contract.SessionWaitRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondSessionWaitError(c, http.StatusBadRequest, sessionWaitInvalidCode, err)
		return
	}
	if request.TimeoutMS <= 0 || request.TimeoutMS > session.WaitTimeoutMax.Milliseconds() {
		h.respondSessionWaitError(c, http.StatusUnprocessableEntity, sessionWaitInvalidCode, fmt.Errorf(
			"api: timeout_ms must be greater than zero and at most %d",
			session.WaitTimeoutMax.Milliseconds(),
		))
		return
	}
	until := make([]session.Badge, 0, len(request.Until))
	for _, badge := range request.Until {
		until = append(until, session.Badge(badge))
	}
	normalizedUntil, err := session.NormalizeWaitBadges(until)
	if err != nil {
		h.respondSessionWaitError(c, http.StatusUnprocessableEntity, sessionWaitCodeForError(err), err)
		return
	}
	sessionID, ok := h.RequireRouteSessionInWorkspace(c)
	if !ok {
		return
	}
	outcome, err := manager.WaitForBadge(c.Request.Context(), session.WaitRequest{
		SessionID: sessionID,
		Until:     normalizedUntil,
		Timeout:   time.Duration(request.TimeoutMS) * time.Millisecond,
		Epoch:     request.Epoch,
		ResumeID:  request.ResumeID,
	})
	if err != nil {
		status := sessionWaitStatusForError(err)
		h.respondSessionWaitError(c, status, sessionWaitCodeForError(err), err)
		return
	}
	if outcome.Outcome == session.WaitResultSessionGone {
		h.respondSessionWaitError(
			c,
			http.StatusGone,
			sessionWaitGoneCode,
			fmt.Errorf("session %s no longer exists", sessionID),
		)
		return
	}
	responseUntil := make([]string, 0, len(normalizedUntil))
	for _, badge := range normalizedUntil {
		responseUntil = append(responseUntil, string(badge))
	}
	c.JSON(http.StatusOK, contract.SessionWaitResponse{
		SessionID: sessionID,
		Outcome:   string(outcome.Outcome),
		State:     string(outcome.State),
		WaitedMS:  outcome.Waited.Milliseconds(),
		Until:     responseUntil,
		Revision:  outcome.Revision,
		ResumeID:  outcome.ResumeID,
	})
}

func (h *BaseHandlers) respondSessionWaitError(c *gin.Context, status int, code string, err error) {
	payload := ErrorPayloadForStatus(status, err, h.MaskInternalErrors)
	payload.Code = code
	c.JSON(status, payload)
}

func sessionWaitStatusForError(err error) int {
	switch {
	case errors.Is(err, session.ErrWaitExpired):
		return http.StatusGone
	case errors.Is(err, session.ErrWaitLimit), errors.Is(err, session.ErrWaitInUse):
		return http.StatusConflict
	case errors.Is(err, session.ErrWaitValidation):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

func sessionWaitCodeForError(err error) string {
	switch {
	case errors.Is(err, session.ErrWaitStateInvalid):
		return sessionWaitStateCode
	case errors.Is(err, session.ErrWaitExpired):
		return sessionWaitExpiredCode
	case errors.Is(err, session.ErrWaitLimit):
		return sessionWaitLimitCode
	case errors.Is(err, session.ErrWaitInUse):
		return sessionWaitInUseCode
	default:
		return sessionWaitInvalidCode
	}
}
