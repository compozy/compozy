package agentrouter

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/gin-gonic/gin"
)

const (
	defaultAgentsLimit = 50
	maxAgentsLimit     = 500
)

// listAgentsTop handles GET /agents.
//
// @Summary List agents
// @Description List agents with cursor pagination. Optionally filter by workflow usage.
// @Tags agents
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param workflow_id query string false "Return only agents referenced by the given workflow" example("wf1")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by agent ID prefix"
// @Success 200 {object} router.Response{data=agentrouter.AgentsListResponse} "Agents retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor"
// @Failure 404 {object} core.ProblemDocument "Workflow not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /agents [get]
func listAgentsTop(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), defaultAgentsLimit, maxAgentsLimit)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	input := &agentuc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
		WorkflowID:      strings.TrimSpace(c.Query("workflow_id")),
	}
	out, err := agentuc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondAgentError(c, err)
		return
	}
	nextCursor := ""
	prevCursor := ""
	if out.NextCursorValue != "" && out.NextCursorDirection != resourceutil.CursorDirectionNone {
		nextCursor = router.EncodeCursor(string(out.NextCursorDirection), out.NextCursorValue)
	}
	if out.PrevCursorValue != "" && out.PrevCursorDirection != resourceutil.CursorDirectionNone {
		prevCursor = router.EncodeCursor(string(out.PrevCursorDirection), out.PrevCursorValue)
	}
	router.SetLinkHeaders(c, nextCursor, prevCursor)
	items := make([]AgentListItem, 0, len(out.Items))
	for i := range out.Items {
		item, err := toAgentListItem(out.Items[i])
		if err != nil {
			router.RespondWithServerError(c, router.ErrInternalCode, "failed to map agent", err)
			return
		}
		items = append(items, item)
	}
	page := httpdto.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "agents retrieved", AgentsListResponse{Agents: items, Page: page})
}

// getAgentTop handles GET /agents/{agent_id}.
//
// @Summary Get agent
// @Description Retrieve an agent configuration by ID.
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID" example("assistant")
// @Param project query string false "Project override" example("demo")
// @Success 200 {object} router.Response{data=agentrouter.AgentDTO} "Agent retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "Agent not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /agents/{agent_id} [get]
func getAgentTop(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	out, err := agentuc.NewGet(store).Execute(c.Request.Context(), &agentuc.GetInput{Project: project, ID: agentID})
	if err != nil {
		respondAgentError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	dto, err := toAgentDTO(out.Agent)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map agent", err)
		return
	}
	router.RespondOK(c, "agent retrieved", dto)
}

// upsertAgentTop handles PUT /agents/{agent_id}.
//
// @Summary Create or update agent
// @Description Create an agent when absent or update an existing one using strong ETag concurrency.
// @Tags agents
// @Accept json
// @Produce json
// @Param agent_id path string true "Agent ID" example("assistant")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Agent configuration payload"
// @Success 200 {object} router.Response{data=agentrouter.AgentDTO} "Agent updated"
// @Success 201 {object} router.Response{data=agentrouter.AgentDTO} "Agent created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the agent"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 404 {object} core.ProblemDocument "Agent not found"
// @Failure 409 {object} core.ProblemDocument "Agent referenced"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /agents/{agent_id} [put]
func upsertAgentTop(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	body := make(map[string]any)
	if err := c.ShouldBindJSON(&body); err != nil {
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	input := &agentuc.UpsertInput{Project: project, ID: agentID, Body: body, IfMatch: ifMatch}
	out, execErr := agentuc.NewUpsert(store).Execute(c.Request.Context(), input)
	if execErr != nil {
		respondAgentError(c, execErr)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	status := http.StatusOK
	message := "agent updated"
	if out.Created {
		status = http.StatusCreated
		message = "agent created"
		c.Header("Location", routes.Agents()+"/"+agentID)
	}
	dto, err := toAgentDTO(out.Agent)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map agent", err)
		return
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, dto)
		return
	}
	router.RespondOK(c, message, dto)
}

// deleteAgentTop handles DELETE /agents/{agent_id}.
//
// @Summary Delete agent
// @Description Delete an agent configuration. Returns conflict when referenced.
// @Tags agents
// @Produce json
// @Param agent_id path string true "Agent ID" example("assistant")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Header 204 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 204 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 204 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 404 {object} core.ProblemDocument "Agent not found"
// @Failure 409 {object} core.ProblemDocument "Agent referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /agents/{agent_id} [delete]
func deleteAgentTop(c *gin.Context) {
	agentID := router.GetAgentID(c)
	if agentID == "" {
		return
	}
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	deleteInput := &agentuc.DeleteInput{Project: project, ID: agentID}
	if err := agentuc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondAgentError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondAgentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, agentuc.ErrInvalidInput),
		errors.Is(err, agentuc.ErrProjectMissing),
		errors.Is(err, agentuc.ErrIDMissing):
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, agentuc.ErrNotFound):
		router.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, agentuc.ErrETagMismatch), errors.Is(err, agentuc.ErrStaleIfMatch):
		router.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	case errors.Is(err, agentuc.ErrWorkflowNotFound):
		router.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: "workflow not found"})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			router.RespondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
