package memoryrouter

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	memoryuc "github.com/compozy/compozy/engine/memory/config/uc"
	"github.com/compozy/compozy/engine/resources"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/gin-gonic/gin"
)

// validateRequestContext centralizes common store+project validation.
func validateRequestContext(c *gin.Context) (resources.ResourceStore, string, bool) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return nil, "", false
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return nil, "", false
	}
	return store, project, true
}

// listMemories handles GET /memories.
//
// @Summary List memories
// @Description List memory configurations with cursor pagination.
// @Tags memories
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by memory ID prefix"
// @Success 200 {object} router.Response{data=memoryrouter.MemoriesListResponse} "Memories retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /memories [get]
func listMemories(c *gin.Context) {
	store, project, ok := validateRequestContext(c)
	if !ok {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), 50, 500)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	input := &memoryuc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}
	out, err := memoryuc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondMemoryError(c, err)
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
	items := make([]MemoryListItem, 0, len(out.Items))
	for i := range out.Items {
		item, err := toMemoryListItem(out.Items[i])
		if err != nil {
			router.RespondWithServerError(c, router.ErrInternalCode, "failed to map memory", err)
			return
		}
		items = append(items, item)
	}
	page := httpdto.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "memories retrieved", MemoriesListResponse{Memories: items, Page: page})
}

// getMemory handles GET /memories/{memory_id}.
//
// @Summary Get memory
// @Description Retrieve a memory configuration by ID.
// @Tags memories
// @Accept json
// @Produce json
// @Param memory_id path string true "Memory ID" example("conversation")
// @Param project query string false "Project override" example("demo")
// @Success 200 {object} router.Response{data=memoryrouter.MemoryDTO} "Memory retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "Memory not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [get]
func getMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
		return
	}
	store, project, ok := validateRequestContext(c)
	if !ok {
		return
	}
	out, err := memoryuc.NewGet(store).Execute(c.Request.Context(), &memoryuc.GetInput{Project: project, ID: memoryID})
	if err != nil {
		respondMemoryError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	dto, mapErr := toMemoryDTO(out.Memory)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map memory", mapErr)
		return
	}
	router.RespondOK(c, "memory retrieved", dto)
}

// upsertMemory handles PUT /memories/{memory_id}.
//
// @Summary Create or update memory
// @Description Create a memory configuration when absent or update an existing one using strong ETag concurrency.
// @Tags memories
// @Accept json
// @Produce json
// @Param memory_id path string true "Memory ID" example("conversation")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Memory configuration payload"
// @Success 200 {object} router.Response{data=memoryrouter.MemoryDTO} "Memory updated"
// @Success 201 {object} router.Response{data=memoryrouter.MemoryDTO} "Memory created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the memory"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 409 {object} core.ProblemDocument "Memory referenced"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [put]
func upsertMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
		return
	}
	store, project, ok := validateRequestContext(c)
	if !ok {
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
	out, execErr := memoryuc.NewUpsert(store).
		Execute(c.Request.Context(), &memoryuc.UpsertInput{Project: project, ID: memoryID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondMemoryError(c, execErr)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	status := http.StatusOK
	message := "memory updated"
	if out.Created {
		status = http.StatusCreated
		message = "memory created"
		c.Header("Location", routes.Memories()+"/"+memoryID)
	}
	dto, mapErr := toMemoryDTO(out.Memory)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map memory", mapErr)
		return
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, dto)
		return
	}
	router.RespondOK(c, message, dto)
}

// deleteMemory handles DELETE /memories/{memory_id}.
//
// @Summary Delete memory
// @Description Delete a memory configuration. Returns conflict when referenced.
// @Tags memories
// @Produce json
// @Param memory_id path string true "Memory ID" example("conversation")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Header 204 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 204 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 204 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 404 {object} core.ProblemDocument "Memory not found"
// @Failure 409 {object} core.ProblemDocument "Memory referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [delete]
func deleteMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
		return
	}
	store, project, ok := validateRequestContext(c)
	if !ok {
		return
	}
	deleteInput := &memoryuc.DeleteInput{Project: project, ID: memoryID}
	if err := memoryuc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondMemoryError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondMemoryError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, memoryuc.ErrInvalidInput),
		errors.Is(err, memoryuc.ErrProjectMissing),
		errors.Is(err, memoryuc.ErrIDMissing):
		core.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, memoryuc.ErrNotFound):
		core.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, memoryuc.ErrETagMismatch), errors.Is(err, memoryuc.ErrStaleIfMatch):
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
