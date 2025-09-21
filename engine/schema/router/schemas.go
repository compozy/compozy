package schemarouter

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	resourceutil "github.com/compozy/compozy/engine/resourceutil"
	schemauc "github.com/compozy/compozy/engine/schema/uc"
	"github.com/gin-gonic/gin"
)

// listSchemas handles GET /schemas.
//
// @Summary List schemas
// @Description List schemas with cursor pagination.
// @Tags schemas
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by schema ID prefix"
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=object{schemas=[]map[string]any,page=object}} "Schemas retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid cursor"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /schemas [get]
func listSchemas(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	project := router.ProjectFromQueryOrDefault(c)
	if project == "" {
		return
	}
	limit := router.LimitOrDefault(c, c.Query("limit"), 50, 500)
	cursor, cursorErr := router.DecodeCursor(c.Query("cursor"))
	if cursorErr != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	input := &schemauc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}
	out, err := schemauc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondSchemaError(c, err)
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
	router.RespondOK(c, "schemas retrieved", gin.H{"schemas": items, "page": page})
}

// getSchema handles GET /schemas/{schema_id}.
//
// @Summary Get schema
// @Description Retrieve a schema by ID.
// @Tags schemas
// @Accept json
// @Produce json
// @Param schema_id path string true "Schema ID" example("user-profile")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=map[string]any} "Schema retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid input"
// @Failure 404 {object} router.ProblemDocument "Schema not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /schemas/{schema_id} [get]
func getSchema(c *gin.Context) {
	schemaID := router.GetURLParam(c, "schema_id")
	if schemaID == "" {
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
	out, err := schemauc.NewGet(store).Execute(c.Request.Context(), &schemauc.GetInput{Project: project, ID: schemaID})
	if err != nil {
		respondSchemaError(c, err)
		return
	}
	filtered := router.FilterMapFields(out.Schema, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Schema["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "schema retrieved", filtered)
}

// upsertSchema handles PUT /schemas/{schema_id}.
//
// @Summary Create or update schema
// @Description Create a schema when absent or update an existing schema using strong ETag concurrency.
// @Tags schemas
// @Accept json
// @Produce json
// @Param schema_id path string true "Schema ID" example("user-profile")
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Schema definition payload"
// @Success 200 {object} router.Response{data=map[string]any} "Schema updated"
// @Success 201 {object} router.Response{data=map[string]any} "Schema created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the created schema"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid request"
// @Failure 404 {object} router.ProblemDocument "Schema not found"
// @Failure 412 {object} router.ProblemDocument "ETag mismatch"
// @Failure 409 {object} router.ProblemDocument "Schema referenced"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /schemas/{schema_id} [put]
func upsertSchema(c *gin.Context) {
	schemaID := router.GetURLParam(c, "schema_id")
	if schemaID == "" {
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
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid request body"})
		return
	}
	ifMatch, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	input := &schemauc.UpsertInput{Project: project, ID: schemaID, Body: body, IfMatch: ifMatch}
	out, execErr := schemauc.NewUpsert(store).Execute(c.Request.Context(), input)
	if execErr != nil {
		respondSchemaError(c, execErr)
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	filtered := router.FilterMapFields(out.Schema, fields)
	if len(fields) != 0 && !fields["_etag"] {
		filtered["_etag"] = out.Schema["_etag"]
	}
	c.Header("ETag", string(out.ETag))
	status := http.StatusOK
	message := "schema updated"
	if out.Created {
		status = http.StatusCreated
		message = "schema created"
		c.Header("Location", routes.Schemas()+"/"+schemaID)
	}
	if status == http.StatusCreated {
		router.RespondCreated(c, message, filtered)
		return
	}
	router.RespondOK(c, message, filtered)
}

// deleteSchema handles DELETE /schemas/{schema_id}.
//
// @Summary Delete schema
// @Description Delete a schema configuration. Returns conflict when referenced.
// @Tags schemas
// @Produce json
// @Param schema_id path string true "Schema ID" example("user-profile")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Header 204 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 204 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 204 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 404 {object} router.ProblemDocument "Schema not found"
// @Failure 409 {object} router.ProblemDocument "Schema referenced"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /schemas/{schema_id} [delete]
func deleteSchema(c *gin.Context) {
	schemaID := router.GetURLParam(c, "schema_id")
	if schemaID == "" {
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
	deleteInput := &schemauc.DeleteInput{Project: project, ID: schemaID}
	if err := schemauc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondSchemaError(c, err)
		return
	}
	router.RespondNoContent(c)
}

func respondSchemaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, schemauc.ErrInvalidInput),
		errors.Is(err, schemauc.ErrProjectMissing),
		errors.Is(err, schemauc.ErrIDMissing),
		errors.Is(err, schemauc.ErrIDMismatch):
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrNotFound):
		router.RespondProblem(c, &router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrETagMismatch),
		errors.Is(err, schemauc.ErrStaleIfMatch):
		router.RespondProblem(c, &router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrReferenced):
		resourceutil.RespondConflict(c, err, nil)
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			resourceutil.RespondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
