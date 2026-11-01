package core

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) instanceReady(c *gin.Context) bool {
	if h.AgentInstances == nil || h.AgentInstances.Service == nil {
		h.respondError(c, 503, errors.New("agent instances unavailable"))
		return false
	}
	return true
}
func (h *BaseHandlers) CreateAgentInstance(c *gin.Context) {
	if !h.instanceReady(c) {
		return
	}
	var request contract.AgentInstanceCreateRequest
	if err := decodeAuthoredJSONBody(c, &request, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	target, err := h.resolveAuthoredAgentTargetForRequest(c, request.Workspace, request.Agent)
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	instance, created, err := h.AgentInstances.Service.Create(c.Request.Context(), agentinstance.CreateRequest{Agent: request.Agent, Workspace: target.sessionWorkspaceID, Profile: scope.ProfileID, Actor: actor})
	if err != nil {
		h.instanceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, contract.AgentInstanceResponse{Instance: AgentInstancePayload(instance)})
}
func (h *BaseHandlers) AdoptAgentInstance(c *gin.Context) {
	if !h.instanceReady(c) {
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	if !actor.Operator {
		h.instanceError(c, agentinstance.ErrForbidden)
		return
	}
	var request contract.AgentInstanceAdoptRequest
	if err := decodeAuthoredJSONBody(c, &request, false); err != nil {
		h.respondError(c, 422, err)
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	info, err := h.Sessions.Status(c.Request.Context(), request.SessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	instance, err := h.AgentInstances.Service.Adopt(c.Request.Context(), agentinstance.AdoptRequest{CreateRequest: agentinstance.CreateRequest{Agent: request.Agent, Workspace: info.WorkspaceID, Profile: scope.ProfileID, Actor: actor}, SessionID: request.SessionID})
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(201, contract.AgentInstanceResponse{Instance: AgentInstancePayload(instance)})
}
func (h *BaseHandlers) ListAgentInstances(c *gin.Context) {
	if !h.instanceReady(c) {
		return
	}
	actor, ok := h.instanceActor(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if scope.AllProfiles {
		h.respondError(c, 422, errors.New("choose one profile"))
		return
	}
	all, err := parseBoolQuery(c, "all_workspaces")
	if err != nil {
		h.respondError(c, 422, err)
		return
	}
	if all && !h.requireOperatorOrGrantedAgent(c, "agent instances") {
		return
	}
	workspaceID := ""
	if ref := c.Query("workspace"); ref != "" && ref != "@user" {
		resolved, err := h.Workspaces.Resolve(c.Request.Context(), ref)
		if err != nil {
			h.respondError(c, 422, err)
			return
		}
		workspaceID = resolved.ID
	}
	limit := 50
	if value := c.Query("limit"); value != "" {
		limit, err = strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 100 {
			h.respondError(c, 422, errors.New("limit must be from 1 to 100"))
			return
		}
	}
	rows, err := h.AgentInstances.Store.List(c.Request.Context(), agentinstance.ListQuery{ProfileID: scope.ProfileID, WorkspaceID: workspaceID, AllWorkspaces: all, AfterID: c.Query("cursor"), Limit: limit})
	if err != nil {
		h.instanceError(c, err)
		return
	}
	result := contract.AgentInstanceListResponse{Instances: make([]contract.AgentInstancePayload, 0, len(rows))}
	for _, row := range rows {
		if c.Query("agent") != "" && row.AgentName != c.Query("agent") {
			continue
		}
		if c.Query("state") != "" && row.State != c.Query("state") {
			continue
		}
		if err := h.AgentInstances.Authority.Authorize(c.Request.Context(), actor, row, "list"); err != nil {
			if errors.Is(err, agentinstance.ErrForbidden) {
				continue
			}
			h.instanceError(c, err)
			return
		}
		result.Instances = append(result.Instances, AgentInstancePayload(row))
	}
	if len(rows) == limit {
		result.NextCursor = rows[len(rows)-1].ID
	}
	c.JSON(200, result)
}
