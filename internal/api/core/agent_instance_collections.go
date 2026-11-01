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

// userScopeWorkspaceRef names the User scope: a Global instance.
const userScopeWorkspaceRef = "@user"

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
	if !ok || !h.admitInstanceApproval(c, actor, "create") {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	// A Global instance has no workspace to resolve; the service validates the definition.
	workspaceID := ""
	if request.Workspace != userScopeWorkspaceRef {
		target, err := h.resolveAuthoredAgentTargetForRequest(c, request.Workspace, request.Agent)
		if err != nil {
			h.respondError(c, 422, err)
			return
		}
		workspaceID = target.sessionWorkspaceID
	}
	instance, created, err := h.AgentInstances.Service.Create(
		c.Request.Context(),
		agentinstance.CreateRequest{
			Agent:     request.Agent,
			Workspace: workspaceID,
			Profile:   scope.ProfileID,
			Actor:     actor,
		},
	)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), instance)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	payload.Created = created
	c.JSON(status, contract.AgentInstanceResponse{Instance: payload})
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
	instance, err := h.AgentInstances.Service.Adopt(
		c.Request.Context(),
		agentinstance.AdoptRequest{
			Agent:     request.Agent,
			Workspace: info.WorkspaceID,
			Profile:   scope.ProfileID,
			Actor:     actor,
			SessionID: request.SessionID,
		},
	)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	payload, err := h.AgentInstances.Payload(c.Request.Context(), instance)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	c.JSON(201, contract.AgentInstanceResponse{Instance: payload})
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
	if ref := c.Query("workspace"); ref != "" && ref != userScopeWorkspaceRef {
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
	rows, err := h.AgentInstances.Store.List(
		c.Request.Context(),
		agentinstance.ListQuery{
			ProfileID:     scope.ProfileID,
			WorkspaceID:   workspaceID,
			AllWorkspaces: all,
			AfterID:       c.Query("cursor"),
			AgentName:     c.Query("agent"), State: c.Query("state"),
			Limit: limit,
		},
	)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	result, err := h.AgentInstances.ListResponse(c.Request.Context(), scope.ProfileID)
	if err != nil {
		h.instanceError(c, err)
		return
	}
	for _, row := range rows {
		if err := h.AgentInstances.Authority.Authorize(c.Request.Context(), actor, row, "list"); err != nil {
			if errors.Is(err, agentinstance.ErrForbidden) {
				continue
			}
			h.instanceError(c, err)
			return
		}
		payload, err := h.AgentInstances.Payload(c.Request.Context(), row)
		if err != nil {
			h.instanceError(c, err)
			return
		}
		result.Instances = append(result.Instances, payload)
	}
	if len(rows) == limit {
		result.NextCursor = rows[len(rows)-1].ID
	}
	c.JSON(200, result)
}

func (h *BaseHandlers) allowAgentDefinitionDeletion(c *gin.Context, profileID, workspaceID, name string) bool {
	if h.AgentInstances == nil || h.AgentInstances.Store == nil {
		return true
	}
	query := agentinstance.ListQuery{
		ProfileID:     profileID,
		WorkspaceID:   workspaceID,
		AllWorkspaces: workspaceID == "",
		Limit:         100,
	}
	for {
		rows, err := h.AgentInstances.Store.List(c.Request.Context(), query)
		if err != nil {
			h.instanceError(c, err)
			return false
		}
		for _, instance := range rows {
			if instance.AgentName == name {
				c.JSON(
					http.StatusConflict,
					contract.ErrorPayload{Error: "agent_has_instances", Code: "agent_has_instances"},
				)
				return false
			}
		}
		if len(rows) < query.Limit {
			return true
		}
		query.AfterID = rows[len(rows)-1].ID
	}
}
