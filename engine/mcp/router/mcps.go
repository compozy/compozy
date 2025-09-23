package mcprouter

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	mcpuc "github.com/compozy/compozy/engine/mcp/uc"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	"github.com/gin-gonic/gin"
)

const (
	defaultMCPListLimit = 50
	maxMCPListLimit     = 500
)

// listMCPs handles GET /mcps.
//
// @Summary List MCP servers
// @Description List MCP server configurations with cursor pagination.
// @Tags mcps
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by MCP ID prefix"
// @Success 200 {object} router.Response{data=mcprouter.MCPsListResponse} "MCPs retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /mcps [get]
func listMCPs(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), defaultMCPListLimit, maxMCPListLimit)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	input := &mcpuc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}
	out, err := mcpuc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondMCPError(c, err)
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
	list := make([]MCPListItem, 0, len(out.Items))
	for i := range out.Items {
		item, err := toMCPListItem(out.Items[i])
		if err != nil {
			router.RespondWithServerError(c, router.ErrInternalCode, "failed to map mcp", err)
			return
		}
		list = append(list, item)
	}
	page := router.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "mcps retrieved", MCPsListResponse{MCPs: list, Page: page})
}

// getMCP handles GET /mcps/{mcp_id}.
//
// @Summary Get MCP server
// @Description Retrieve an MCP server configuration by ID.
// @Tags mcps
// @Accept json
// @Produce json
// @Param mcp_id path string true "MCP ID" example("filesystem")
// @Param project query string false "Project override" example("demo")
// @Success 200 {object} router.Response{data=mcprouter.MCPDTO} "MCP retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "MCP not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /mcps/{mcp_id} [get]
func getMCP(c *gin.Context) {
	mcpID := router.GetURLParam(c, "mcp_id")
	if mcpID == "" {
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
	out, err := mcpuc.NewGet(store).Execute(c.Request.Context(), &mcpuc.GetInput{Project: project, ID: mcpID})
	if err != nil {
		respondMCPError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	dto, mapErr := toMCPDTO(out.MCP)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map mcp", mapErr)
		return
	}
	router.RespondOK(c, "mcp retrieved", dto)
}

// upsertMCP handles PUT /mcps/{mcp_id}.
//
// @Summary Create or update MCP server
// @Description Create an MCP server when absent or update an existing one using strong ETag concurrency.
// @Tags mcps
// @Accept json
// @Produce json
// @Param mcp_id path string true "MCP ID" example("filesystem")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "MCP configuration payload"
// @Success 200 {object} router.Response{data=mcprouter.MCPDTO} "MCP updated"
// @Success 201 {object} router.Response{data=mcprouter.MCPDTO} "MCP created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Relative URL for the MCP"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 409 {object} core.ProblemDocument "MCP referenced"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /mcps/{mcp_id} [put]
func upsertMCP(c *gin.Context) {
	mcpID := router.GetURLParam(c, "mcp_id")
	if mcpID == "" {
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
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	out, execErr := mcpuc.NewUpsert(store).
		Execute(c.Request.Context(), &mcpuc.UpsertInput{Project: project, ID: mcpID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondMCPError(c, execErr)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	message := "mcp updated"
	if out.Created {
		message = "mcp created"
		c.Header("Location", routes.Mcps()+"/"+mcpID)
	}
	dto, mapErr := toMCPDTO(out.Config)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map mcp", mapErr)
		return
	}
	if out.Created {
		router.RespondCreated(c, message, dto)
		return
	}
	router.RespondOK(c, message, dto)
}

// deleteMCP handles DELETE /mcps/{mcp_id}.
//
// @Summary Delete MCP server
// @Description Delete an MCP server configuration. Returns conflict when referenced.
// @Tags mcps
// @Produce json
// @Param mcp_id path string true "MCP ID" example("filesystem")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Header 204 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 204 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 204 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 404 {object} core.ProblemDocument "MCP not found"
// @Failure 409 {object} core.ProblemDocument "MCP referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /mcps/{mcp_id} [delete]
func deleteMCP(c *gin.Context) {
	mcpID := router.GetURLParam(c, "mcp_id")
	if mcpID == "" {
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
	deleteInput := &mcpuc.DeleteInput{Project: project, ID: mcpID}
	if err := mcpuc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondMCPError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondMCPError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, mcpuc.ErrInvalidInput),
		errors.Is(err, mcpuc.ErrProjectMissing),
		errors.Is(err, mcpuc.ErrIDMissing):
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, mcpuc.ErrNotFound):
		core.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, mcpuc.ErrETagMismatch), errors.Is(err, mcpuc.ErrStaleIfMatch):
		core.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			resourceutil.RespondConflict(c, err, conflict.Details)
			return
		}
		core.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
