package memoryrouter

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	memoryuc "github.com/compozy/compozy/engine/memoryconfig/uc"
	"github.com/compozy/compozy/engine/resourceutil"
	"github.com/gin-gonic/gin"
)

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
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=object{memories=[]map[string]any,page=object}} "Memories retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid cursor"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /memories [get]
func listMemories(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c.Query("limit"), 50, 500)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
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
	items := make([]map[string]any, 0, len(out.Items))
	for i := range out.Items {
		filtered := router.FilterMapFields(out.Items[i], fields)
		if len(fields) != 0 && !fields["_etag"] {
			filtered["_etag"] = out.Items[i]["_etag"]
		}
		items = append(items, filtered)
	}
	page := map[string]any{"limit": limit, "total": out.Total}
	if nextCursor != "" {
		page["next_cursor"] = nextCursor
	}
	if prevCursor != "" {
		page["prev_cursor"] = prevCursor
	}
	router.RespondOK(c, "memories retrieved", gin.H{"memories": items, "page": page})
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
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=map[string]any} "Memory retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid input"
// @Failure 404 {object} router.ProblemDocument "Memory not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [get]
func getMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
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
	fields := router.ParseFieldsQuery(c.Query("fields"))
	out, err := memoryuc.NewGet(store).Execute(c.Request.Context(), &memoryuc.GetInput{Project: project, ID: memoryID})
	if err != nil {
		respondMemoryError(c, err)
		return
	}
	filtered := router.FilterMapFields(out.Memory, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Memory["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "memory retrieved", filtered)
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
// @Param fields query string false "Comma-separated list of fields to include"
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Memory configuration payload"
// @Success 200 {object} router.Response{data=map[string]any} "Memory updated"
// @Success 201 {object} router.Response{data=map[string]any} "Memory created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the memory"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid request"
// @Failure 409 {object} router.ProblemDocument "Memory referenced"
// @Failure 412 {object} router.ProblemDocument "ETag mismatch"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [put]
func upsertMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
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
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	out, execErr := memoryuc.NewUpsert(store).
		Execute(c.Request.Context(), &memoryuc.UpsertInput{Project: project, ID: memoryID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondMemoryError(c, execErr)
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	filtered := router.FilterMapFields(out.Memory, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Memory["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	status := http.StatusOK
	message := "memory updated"
	if out.Created {
		status = http.StatusCreated
		message = "memory created"
		c.Header("Location", routes.Memories()+"/"+memoryID)
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, filtered)
		return
	}
	router.RespondOK(c, message, filtered)
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
// @Failure 404 {object} router.ProblemDocument "Memory not found"
// @Failure 409 {object} router.ProblemDocument "Memory referenced"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /memories/{memory_id} [delete]
func deleteMemory(c *gin.Context) {
	memoryID := router.GetURLParam(c, "memory_id")
	if memoryID == "" {
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
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, memoryuc.ErrNotFound):
		router.RespondProblem(c, router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, memoryuc.ErrETagMismatch), errors.Is(err, memoryuc.ErrStaleIfMatch):
		router.RespondProblem(c, router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			respondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}

func respondConflict(c *gin.Context, err error, details []resourceutil.ReferenceDetail) {
	extras := map[string]any{}
	if len(details) > 0 {
		refs := make([]map[string]any, 0, len(details))
		for i := range details {
			d := map[string]any{"resource": details[i].Resource, "ids": details[i].IDs}
			refs = append(refs, d)
		}
		extras["references"] = refs
	}
	detail := err.Error()
	if detail == "" {
		detail = "resource has active references"
	}
	router.RespondProblem(c, router.Problem{Status: http.StatusConflict, Detail: detail, Extras: extras})
}
