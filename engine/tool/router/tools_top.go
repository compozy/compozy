package toolrouter

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	"github.com/compozy/compozy/engine/resourceutil"
	tooluc "github.com/compozy/compozy/engine/tool/uc"
	"github.com/gin-gonic/gin"
)

// listToolsTop handles GET /tools.
//
// @Summary List tools
// @Description List tools with cursor pagination. Optionally filter by workflow usage.
// @Tags tools
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param workflow_id query string false "Return only tools referenced by the given workflow" example("wf1")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by tool ID prefix"
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=object{tools=[]map[string]any,page=object}} "Tools retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid cursor"
// @Failure 404 {object} router.ProblemDocument "Workflow not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tools [get]
func listToolsTop(c *gin.Context) {
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
	input := &tooluc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
		WorkflowID:      strings.TrimSpace(c.Query("workflow_id")),
	}
	out, err := tooluc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondToolError(c, err)
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
	router.RespondOK(c, "tools retrieved", gin.H{"tools": items, "page": page})
}

// getToolTop handles GET /tools/{tool_id}.
//
// @Summary Get tool
// @Description Retrieve a tool configuration by ID.
// @Tags tools
// @Accept json
// @Produce json
// @Param tool_id path string true "Tool ID" example("http-client")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=map[string]any} "Tool retrieved"
// @Failure 400 {object} router.ProblemDocument "Invalid input"
// @Failure 404 {object} router.ProblemDocument "Tool not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tools/{tool_id} [get]
func getToolTop(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
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
	out, err := tooluc.NewGet(store).Execute(c.Request.Context(), &tooluc.GetInput{Project: project, ID: toolID})
	if err != nil {
		respondToolError(c, err)
		return
	}
	filtered := router.FilterMapFields(out.Tool, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Tool["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "tool retrieved", filtered)
}

// upsertToolTop handles PUT /tools/{tool_id}.
//
// @Summary Create or update tool
// @Description Create a tool configuration when absent or update an existing one using strong ETag concurrency.
// @Tags tools
// @Accept json
// @Produce json
// @Param tool_id path string true "Tool ID" example("http-client")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Tool configuration payload"
// @Success 200 {object} router.Response{data=map[string]any} "Tool updated"
// @Success 201 {object} router.Response{data=map[string]any} "Tool created"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the tool"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid request"
// @Failure 404 {object} router.ProblemDocument "Tool not found"
// @Failure 409 {object} router.ProblemDocument "Tool referenced"
// @Failure 412 {object} router.ProblemDocument "ETag mismatch"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tools/{tool_id} [put]
func upsertToolTop(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
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
	input := &tooluc.UpsertInput{Project: project, ID: toolID, Body: body, IfMatch: ifMatch}
	out, execErr := tooluc.NewUpsert(store).Execute(c.Request.Context(), input)
	if execErr != nil {
		respondToolError(c, execErr)
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	filtered := router.FilterMapFields(out.Tool, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Tool["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	status := http.StatusOK
	message := "tool updated"
	if out.Created {
		status = http.StatusCreated
		message = "tool created"
		c.Header("Location", routes.Tools()+"/"+toolID)
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, filtered)
		return
	}
	router.RespondOK(c, message, filtered)
}

// deleteToolTop handles DELETE /tools/{tool_id}.
//
// @Summary Delete tool
// @Description Delete a tool configuration. Returns conflict when referenced.
// @Tags tools
// @Produce json
// @Param tool_id path string true "Tool ID" example("http-client")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Failure 404 {object} router.ProblemDocument "Tool not found"
// @Failure 409 {object} router.ProblemDocument "Tool referenced"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /tools/{tool_id} [delete]
func deleteToolTop(c *gin.Context) {
	toolID := router.GetToolID(c)
	if toolID == "" {
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
	deleteInput := &tooluc.DeleteInput{Project: project, ID: toolID}
	if err := tooluc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondToolError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondToolError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, tooluc.ErrInvalidInput),
		errors.Is(err, tooluc.ErrProjectMissing),
		errors.Is(err, tooluc.ErrIDMissing):
		router.RespondProblem(c, router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, tooluc.ErrNotFound):
		router.RespondProblem(c, router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, tooluc.ErrETagMismatch),
		errors.Is(err, tooluc.ErrStaleIfMatch):
		router.RespondProblem(c, router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	case errors.Is(err, tooluc.ErrWorkflowNotFound):
		router.RespondProblem(c, router.Problem{Status: http.StatusNotFound, Detail: "workflow not found"})
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
