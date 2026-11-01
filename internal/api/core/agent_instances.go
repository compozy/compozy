package core

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/gin-gonic/gin"
)

type AgentInstanceAPI struct {
	ReadPayload   func(context.Context, agentinstance.Instance) (contract.AgentInstancePayload, error)
	CheckSettings AgentInstanceCheckSettings
	Checks        AgentInstanceChecker
	Service       *agentinstance.Service
	Store         agentinstance.InstanceStore
	Reports       agentinstance.ReportStore
	Authority     agentinstance.Authority
}

func (h *BaseHandlers) instanceActor(c *gin.Context) (workspaceaccess.ActorRef, bool) {
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return workspaceaccess.ActorRef{Kind: workspaceaccess.ActorHuman, Operator: true}, true
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, "agent instances")
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return workspaceaccess.ActorRef{}, false
	}
	return workspaceaccess.ActorRef{
		Kind:        workspaceaccess.ActorAgentSession,
		SessionID:   caller.Session.ID,
		WorkspaceID: caller.Session.WorkspaceID,
		AgentName:   caller.Session.AgentName,
	}, true
}

func (h *BaseHandlers) instanceTarget(c *gin.Context, verb string) (agentinstance.Instance, bool) {
	if h.AgentInstances == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("agent instances unavailable"))
		return agentinstance.Instance{}, false
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return agentinstance.Instance{}, false
	}
	instance, err := h.AgentInstances.Store.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.instanceError(c, err)
		return agentinstance.Instance{}, false
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return agentinstance.Instance{}, false
	}
	if instance.ProfileID != scope.ProfileID && !scope.AllProfiles {
		h.instanceError(c, agentinstance.ErrNotFound)
		return agentinstance.Instance{}, false
	}
	if err := h.AgentInstances.Authority.Authorize(c.Request.Context(), actor, instance, verb); err != nil {
		h.instanceError(c, err)
		return agentinstance.Instance{}, false
	}
	if !actor.Operator && h.WorkspaceAccess == nil {
		h.instanceError(c, agentinstance.ErrForbidden)
		return agentinstance.Instance{}, false
	}
	if !actor.Operator {
		decision, err := h.WorkspaceAccess.Authorize(
			c.Request.Context(),
			workspaceaccess.Request{
				Actor:    actor,
				Target:   workspaceaccess.ScopeTarget(instance.WorkspaceID, instance.ProfileID),
				ReadOnly: verb == "occurrence" || verb == "reports" || verb == "list" || verb == "check_dry_run",
				Seam:     workspaceaccess.SeamTool,
			},
		)
		if err != nil || !decision.Allowed {
			h.instanceError(c, agentinstance.ErrForbidden)
			return agentinstance.Instance{}, false
		}
	}
	return instance, true
}

func (h *BaseHandlers) GetAgentInstance(c *gin.Context) {
	instance, ok := h.instanceTarget(c, "list")
	if !ok {
		return
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), instance)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceResponse{Instance: payload})
}
func (h *BaseHandlers) PauseAgentInstance(c *gin.Context)  { h.mutateAgentInstance(c, "pause") }
func (h *BaseHandlers) ResumeAgentInstance(c *gin.Context) { h.mutateAgentInstance(c, "resume") }
func (h *BaseHandlers) EnsureAgentInstanceMain(c *gin.Context) {
	h.mutateAgentInstance(c, "ensure_main")
}
func (h *BaseHandlers) ArchiveAgentInstance(c *gin.Context) { h.mutateAgentInstance(c, "archive") }
func (h *BaseHandlers) mutateAgentInstance(c *gin.Context, verb string) {
	instance, ok := h.instanceTarget(c, verb)
	if !ok {
		return
	}
	var err error
	switch verb {
	case "pause":
		instance, err = h.AgentInstances.Service.Pause(c.Request.Context(), instance.ID, "manual")
	case "resume":
		instance, err = h.AgentInstances.Service.Resume(c.Request.Context(), instance.ID)
	case "ensure_main":
		instance, err = h.AgentInstances.Service.EnsureMain(c.Request.Context(), instance.ID)
	case "archive":
		err = h.AgentInstances.Service.Archive(c.Request.Context(), instance.ID)
	}
	if err != nil {
		h.instanceError(c, err)
		return
	}
	if verb == "archive" {
		c.Status(http.StatusNoContent)
		return
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), instance)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.AgentInstanceResponse{Instance: payload})
}

func (h *BaseHandlers) ListAgentInstanceReports(c *gin.Context) {
	instance, ok := h.instanceTarget(c, "reports")
	if !ok {
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			h.respondError(c, 422, errors.New("limit must be from 1 to 100"))
			return
		}
		limit = value
	}
	state := c.Query("state")
	switch state {
	case "", "pending", "reserved", "delivered", "indeterminate":
	default:
		h.respondError(c, 422, errors.New("invalid report state"))
		return
	}
	query := agentinstance.ReportQuery{InstanceID: instance.ID, Limit: limit, State: state}
	if cursor := c.Query("cursor"); cursor != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			h.respondError(c, 422, errors.New("invalid report cursor"))
			return
		}
		parts := strings.Split(string(decoded), "\n")
		if len(parts) != 4 || parts[0] != instance.ID || parts[3] != state {
			h.respondError(c, 422, errors.New("invalid report cursor"))
			return
		}
		at, err := time.Parse(time.RFC3339Nano, parts[1])
		if err != nil {
			h.respondError(c, 422, errors.New("invalid report cursor"))
			return
		}
		query.AfterCreatedAt = at
		query.AfterID = parts[2]
	}
	reports, err := h.AgentInstances.Reports.ListReports(c.Request.Context(), query)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	result := contract.AgentInstanceReportsResponse{
		Reports: make([]contract.AgentInstanceReportPayload, 0, len(reports)),
	}
	for _, report := range reports {
		var payload contract.AgentInstanceReportPayload
		if err := json.Unmarshal(report.Payload, &payload); err != nil {
			h.respondError(c, 500, err)
			return
		}
		payload.ID, payload.Kind, payload.State, payload.CreatedAt = report.ID, report.Kind, report.State, report.CreatedAt
		result.Reports = append(result.Reports, payload)
	}
	if len(reports) == limit {
		last := reports[len(reports)-1]
		result.NextCursor = base64.RawURLEncoding.EncodeToString(
			[]byte(instance.ID + "\n" + last.CreatedAt.Format(time.RFC3339Nano) + "\n" + last.ID + "\n" + state),
		)
	}
	c.JSON(http.StatusOK, result)
}
func (api *AgentInstanceAPI) Payload(ctx context.Context, i agentinstance.Instance) (contract.AgentInstancePayload, error) {
	if api.ReadPayload != nil {
		return api.ReadPayload(ctx, i)
	}
	return AgentInstancePayload(i), nil
}

func AgentInstancePayload(i agentinstance.Instance) contract.AgentInstancePayload {
	return contract.AgentInstancePayload{
		Label: i.AgentName, Activity: "unknown", Scope: i.WorkspaceID,
		ID:            i.ID,
		ProfileID:     i.ProfileID,
		AgentName:     i.AgentName,
		WorkspaceID:   i.WorkspaceID,
		State:         i.State,
		PauseReason:   i.PauseReason,
		MainSessionID: i.MainSessionID,
		CreatedAt:     i.CreatedAt,
		UpdatedAt:     i.UpdatedAt,
	}
}
func (h *BaseHandlers) instanceError(c *gin.Context, err error) {
	for _, mapping := range []struct {
		domain error
		status int
	}{
		{agentinstance.ErrForbidden, http.StatusForbidden},
		{agentinstance.ErrWorkspaceAccessDenied, http.StatusForbidden},
		{agentinstance.ErrSessionNotRoot, http.StatusUnprocessableEntity},
		{agentinstance.ErrScopeMismatch, http.StatusUnprocessableEntity},
		{agentinstance.ErrPaused, http.StatusConflict},
		{agentinstance.ErrDepth, http.StatusForbidden},
		{agentinstance.ErrNotFound, http.StatusNotFound},
		{agentinstance.ErrDefinitionMissing, http.StatusNotFound},
		{agentinstance.ErrExists, http.StatusConflict},
		{agentinstance.ErrStarting, http.StatusConflict},
		{agentinstance.ErrLimit, http.StatusTooManyRequests},
		{agentinstance.ErrNotStaysOn, http.StatusUnprocessableEntity},
	} {
		if errors.Is(err, mapping.domain) {
			c.JSON(mapping.status, contract.ErrorPayload{Error: err.Error(), Code: mapping.domain.Error()})
			return
		}
	}
	h.respondError(c, http.StatusInternalServerError, err)
}
