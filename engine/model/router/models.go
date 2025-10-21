package modelrouter

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	modeluc "github.com/compozy/compozy/engine/model/uc"
	resourceutil "github.com/compozy/compozy/engine/resources/utils"
	"github.com/gin-gonic/gin"
)

// listModels handles GET /models.
//
// @Summary List models
// @Description List models with cursor pagination.
// @Tags models
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param limit query int false "Page size (max 500)" example(50)
// @Param cursor query string false "Opaque pagination cursor"
// @Param q query string false "Filter by model ID prefix"
// @Success 200 {object} router.Response{data=modelrouter.ModelsListResponse} "Models retrieved"
// @Header 200 {string} Link "RFC 8288 pagination links for next/prev"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid cursor"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /models [get]
func listModels(c *gin.Context) {
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
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: "invalid cursor parameter"})
		return
	}
	input := &modeluc.ListInput{
		Project:         project,
		Prefix:          strings.TrimSpace(c.Query("q")),
		CursorValue:     cursor.Value,
		CursorDirection: resourceutil.CursorDirection(cursor.Direction),
		Limit:           limit,
	}
	out, err := modeluc.NewList(store).Execute(c.Request.Context(), input)
	if err != nil {
		respondModelError(c, err)
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
	list := make([]ModelListItem, 0, len(out.Items))
	for i := range out.Items {
		item, err := toModelListItem(out.Items[i])
		if err != nil {
			router.RespondWithServerError(c, router.ErrInternalCode, "failed to map model", err)
			return
		}
		list = append(list, item)
	}
	page := httpdto.PageInfoDTO{Limit: limit, Total: out.Total, NextCursor: nextCursor, PrevCursor: prevCursor}
	router.RespondOK(c, "models retrieved", ModelsListResponse{Models: list, Page: page})
}

// getModel handles GET /models/{model_id}.
//
// @Summary Get model
// @Description Retrieve a model configuration by ID.
// @Tags models
// @Accept json
// @Produce json
// @Param model_id path string true "Model ID" example("openai:gpt-4o-mini")
// @Param project query string false "Project override" example("demo")
// @Success 200 {object} router.Response{data=modelrouter.ModelDTO} "Model retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid input"
// @Failure 404 {object} core.ProblemDocument "Model not found"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /models/{model_id} [get]
func getModel(c *gin.Context) {
	modelID := router.GetURLParam(c, "model_id")
	if modelID == "" {
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
	out, err := modeluc.NewGet(store).Execute(c.Request.Context(), &modeluc.GetInput{Project: project, ID: modelID})
	if err != nil {
		respondModelError(c, err)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	dto, err := toModelDTO(out.Model)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map model", err)
		return
	}
	router.RespondOK(c, "model retrieved", dto)
}

// upsertModel handles PUT /models/{model_id}.
//
// @Summary Create or update model
// @Description Create a model when absent or update an existing one using strong ETag concurrency.
// @Tags models
// @Accept json
// @Produce json
// @Param model_id path string true "Model ID" example("openai:gpt-4o-mini")
// @Param project query string false "Project override" example("demo")
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Model configuration payload"
// @Success 200 {object} router.Response{data=modelrouter.ModelDTO} "Model updated"
// @Success 201 {object} router.Response{data=modelrouter.ModelDTO} "Model created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Relative URL for the model"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} core.ProblemDocument "Invalid request"
// @Failure 409 {object} core.ProblemDocument "Model referenced"
// @Failure 412 {object} core.ProblemDocument "ETag mismatch"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /models/{model_id} [put]
func upsertModel(c *gin.Context) {
	modelID := router.GetURLParam(c, "model_id")
	if modelID == "" {
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
	out, execErr := modeluc.NewUpsert(store).
		Execute(c.Request.Context(), &modeluc.UpsertInput{Project: project, ID: modelID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondModelError(c, execErr)
		return
	}
	c.Header("ETag", fmt.Sprintf("%q", out.ETag))
	message := "model updated"
	if out.Created {
		message = "model created"
		c.Header("Location", routes.Models()+"/"+modelID)
	}
	dto, mapErr := toModelDTO(out.Model)
	if mapErr != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to map model", mapErr)
		return
	}
	if out.Created {
		router.RespondCreated(c, message, dto)
	} else {
		router.RespondOK(c, message, dto)
	}
}

// deleteModel handles DELETE /models/{model_id}.
//
// @Summary Delete model
// @Description Delete a model configuration. Returns conflict when referenced.
// @Tags models
// @Produce json
// @Param model_id path string true "Model ID" example("openai:gpt-4o-mini")
// @Param project query string false "Project override" example("demo")
// @Success 204 {string} string ""
// @Header 204 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 204 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 204 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 404 {object} core.ProblemDocument "Model not found"
// @Failure 409 {object} core.ProblemDocument "Model referenced"
// @Failure 500 {object} core.ProblemDocument "Internal server error"
// @Router /models/{model_id} [delete]
func deleteModel(c *gin.Context) {
	modelID := router.GetURLParam(c, "model_id")
	if modelID == "" {
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
	deleteInput := &modeluc.DeleteInput{Project: project, ID: modelID}
	if err := modeluc.NewDelete(store).Execute(c.Request.Context(), deleteInput); err != nil {
		respondModelError(c, err)
		return
	}
	router.RespondNoContent(c)
}
func respondModelError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, modeluc.ErrInvalidInput),
		errors.Is(err, modeluc.ErrProjectMissing),
		errors.Is(err, modeluc.ErrIDMissing):
		router.RespondProblem(c, &core.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, modeluc.ErrNotFound):
		router.RespondProblem(c, &core.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, modeluc.ErrETagMismatch), errors.Is(err, modeluc.ErrStaleIfMatch):
		router.RespondProblem(c, &core.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		var conflict resourceutil.ConflictError
		if errors.As(err, &conflict) {
			router.RespondConflict(c, err, conflict.Details)
			return
		}
		router.RespondProblem(c, &core.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
