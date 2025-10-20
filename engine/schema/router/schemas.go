package schemarouter

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	schemauc "github.com/compozy/compozy/engine/schema/uc"
	"github.com/compozy/compozy/pkg/config"
	"github.com/gin-gonic/gin"
)

var errSchemaExceedsLimit = errors.New("stored schema exceeds configured size limit")

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
// @Success 200 {object} router.Response{data=schemarouter.SchemasListResponse} "Schemas retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
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
	input, limit, parseErr := parseListSchemasInput(c, project)
	if parseErr != nil {
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	out, err := schemauc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondSchemaError(c, err)
		return
	}
	nextCursor, prevCursor := buildSchemaPagination(out)
	router.SetLinkHeaders(c, nextCursor, prevCursor)
	items, buildErr := buildSchemaListItems(c.Request.Context(), out.Items)
	if buildErr != nil {
		if errors.Is(buildErr, errSchemaExceedsLimit) {
			router.RespondProblem(
				c,
				&core.Problem{Status: http.StatusInternalServerError, Detail: buildErr.Error()},
			)
			return
		}
		router.RespondProblem(
			c,
			&core.Problem{Status: http.StatusInternalServerError, Detail: "failed to encode schema"},
		)
		return
	}
	page := httpdto.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "schemas retrieved", SchemasListResponse{Schemas: items, Page: page})
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
// @Success 200 {object} router.Response{data=schemarouter.SchemaDTO} "Schema retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "Schema not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
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
	out, err := schemauc.NewGet(store).Execute(c.Request.Context(), &schemauc.GetInput{Project: project, ID: schemaID})
	if err != nil {
		respondSchemaError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	maxBytes := 0
	if cfg := config.FromContext(c.Request.Context()); cfg != nil {
		maxBytes = cfg.Limits.MaxConfigFileSize
	}
	dto, n, err := toSchemaDTO(out.Schema)
	if err != nil {
		router.RespondProblem(
			c,
			&core.Problem{Status: http.StatusInternalServerError, Detail: "failed to encode schema"},
		)
		return
	}
	if maxBytes > 0 && n > maxBytes {
		router.RespondProblem(
			c,
			&core.Problem{
				Status: http.StatusInternalServerError,
				Detail: "stored schema exceeds configured size limit",
			},
		)
		return
	}
	router.RespondOK(c, "schema retrieved", dto)
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
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Schema definition payload"
// @Success 200 {object} router.Response{data=schemarouter.SchemaDTO} "Schema updated"
// @Success 201 {object} router.Response{data=schemarouter.SchemaDTO} "Schema created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the created schema"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 404 {object} core.ProblemDocument "Schema not found"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 409 {object} core.ProblemDocument "Schema referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
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
	body, err := readSchemaBody(c)
	if err != nil {
		respondSchemaBodyError(c, err)
		return
	}
	ifMatch, err := parseIfMatchHeader(c)
	if err != nil {
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid If-Match header"})
		return
	}
	input := &schemauc.UpsertInput{Project: project, ID: schemaID, Body: body, IfMatch: ifMatch}
	out, execErr := schemauc.NewUpsert(store).Execute(c.Request.Context(), input)
	if execErr != nil {
		respondSchemaError(c, execErr)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	if err := respondUpsertResult(c, schemaID, out); err != nil {
		return
	}
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
// @Failure 404 {object} core.ProblemDocument "Schema not found"
// @Failure 409 {object} core.ProblemDocument "Schema referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
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
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrNotFound):
		router.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrETagMismatch),
		errors.Is(err, schemauc.ErrStaleIfMatch):
		router.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	case errors.Is(err, schemauc.ErrReferenced):
		router.RespondConflict(c, err, nil)
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			router.RespondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}

// parseListSchemasInput builds the UC input for schema listing while enforcing cursor constraints.
func parseListSchemasInput(c *gin.Context, project string) (*schemauc.ListInput, int, error) {
	limit := router.LimitOrDefault(c, c.Query("limit"), 50, 500)
	cursor, err := router.DecodeCursor(c.Query("cursor"))
	if err != nil {
		return nil, 0, err
	}
	return &schemauc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}, limit, nil
}

// buildSchemaPagination computes RFC 8288 pagination links.
func buildSchemaPagination(out *schemauc.ListOutput) (string, string) {
	if out == nil {
		return "", ""
	}
	next := ""
	prev := ""
	if out.NextCursorValue != "" && out.NextCursorDirection != resourceutil.CursorDirectionNone {
		next = router.EncodeCursor(string(out.NextCursorDirection), out.NextCursorValue)
	}
	if out.PrevCursorValue != "" && out.PrevCursorDirection != resourceutil.CursorDirectionNone {
		prev = router.EncodeCursor(string(out.PrevCursorDirection), out.PrevCursorValue)
	}
	return next, prev
}

// buildSchemaListItems converts raw schemas to response DTOs while enforcing size limits.
func buildSchemaListItems(ctx context.Context, items []map[string]any) ([]SchemaListItem, error) {
	maxBytes := schemaSizeLimit(ctx)
	list := make([]SchemaListItem, 0, len(items))
	for i := range items {
		dto, n, err := toSchemaListItem(items[i])
		if err != nil {
			return nil, err
		}
		if maxBytes > 0 && n > maxBytes {
			return nil, errSchemaExceedsLimit
		}
		list = append(list, dto)
	}
	return list, nil
}

// schemaSizeLimit returns the configured schema size limit in bytes, or zero when unrestricted.
func schemaSizeLimit(ctx context.Context) int {
	if cfg := config.FromContext(ctx); cfg != nil {
		return cfg.Limits.MaxConfigFileSize
	}
	return 0
}

var errSchemaBodyTooLarge = errors.New("schema body exceeds configured size limit")

// readSchemaBody decodes the schema request payload while enforcing size limits.
func readSchemaBody(c *gin.Context) (map[string]any, error) {
	maxReq := schemaSizeLimit(c.Request.Context())
	if maxReq > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxReq))
	}
	body := make(map[string]any)
	if err := c.ShouldBindJSON(&body); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, errSchemaBodyTooLarge
		}
		return nil, err
	}
	return body, nil
}

// parseIfMatchHeader validates the If-Match header and returns the strong ETag value.
func parseIfMatchHeader(c *gin.Context) (string, error) {
	tag, err := router.ParseStrongETag(c.GetHeader("If-Match"))
	if err != nil {
		return "", err
	}
	return tag, nil
}

// respondSchemaBodyError maps parsing errors to proper HTTP problems.
func respondSchemaBodyError(c *gin.Context, err error) {
	var detail string
	status := http.StatusBadRequest
	switch {
	case errors.Is(err, http.ErrBodyNotAllowed):
		detail = "invalid request body"
	case errors.Is(err, errSchemaBodyTooLarge):
		detail = "request body exceeds configured size limit"
		status = http.StatusRequestEntityTooLarge
	default:
		detail = "invalid request body"
	}
	router.RespondProblem(c, &core.Problem{Status: status, Detail: detail})
}

// respondUpsertResult writes the upsert response payload based on creation status.
func respondUpsertResult(c *gin.Context, schemaID string, out *schemauc.UpsertOutput) error {
	dto, _, err := toSchemaDTO(out.Schema)
	if err != nil {
		router.RespondProblem(
			c,
			&core.Problem{Status: http.StatusInternalServerError, Detail: "failed to encode schema"},
		)
		return err
	}
	if out.Created {
		c.Header("Location", routes.Schemas()+"/"+schemaID)
		router.RespondCreated(c, "schema created", dto)
		return nil
	}
	router.RespondOK(c, "schema updated", dto)
	return nil
}
