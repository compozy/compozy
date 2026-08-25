package core

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) requireGoalCommandService(c *gin.Context) (GoalCommandService, bool) {
	if h == nil || h.Loops == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("goal command service is not configured"))
		return nil, false
	}
	service, ok := h.Loops.(GoalCommandService)
	if !ok || service == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("goal command service is unavailable"))
		return nil, false
	}
	return service, true
}

// MutateSessionGoal executes one authenticated typed Goal operation for the route session.
func (h *BaseHandlers) MutateSessionGoal(c *gin.Context) {
	service, ok := h.requireGoalCommandService(c)
	if !ok {
		return
	}
	scope, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	var request contract.SessionGoalCommandRequest
	if err := decodeStrictJSONBody(c, &request); err != nil {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("decode Goal command request: %w", err))
		return
	}
	if err := request.Validate(); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	workspaceID := scope.SessionWorkspaceID()
	caller, err := h.PromptCallerForWorkspace(c, workspaceID)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return
	}
	decision, err := service.Handle(
		c.Request.Context(),
		workspaceID,
		sessionID,
		caller,
		session.GoalCommand{
			Verb:          strings.TrimSpace(string(request.Operation)),
			Objective:     strings.TrimSpace(request.Objective),
			ExpectedRunID: strings.TrimSpace(request.ExpectedRunID),
			Runtime:       contract.PromptRuntimeSelectionFromPayload(request.Runtime),
		},
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	if decision.Kind != session.GoalDispatchRespond || decision.Result == nil {
		h.respondLoopError(c, fmt.Errorf("%w: Goal command did not return a structured result", looppkg.ErrValidation))
		return
	}
	payload, err := GoalCommandResultPayloadFromSession(decision.Result)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(GoalCommandHTTPStatus(payload), payload)
}
