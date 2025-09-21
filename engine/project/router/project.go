package projectrouter

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/infra/server/routes"
	projectuc "github.com/compozy/compozy/engine/project/uc"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// getProject handles GET /project.
//
// @Summary Get project
// @Description Retrieve the active project configuration.
// @Tags project
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Success 200 {object} router.Response{data=map[string]any} "Project retrieved"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid input"
// @Failure 404 {object} router.ProblemDocument "Project not found"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /project [get]
func getProject(c *gin.Context) {
	log := logger.FromContext(c.Request.Context())
	log.Debug("handling GET /project request")
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	projectID := router.ProjectFromQueryOrDefault(c)
	if projectID == "" {
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	out, err := projectuc.NewGet(store).Execute(c.Request.Context(), &projectuc.GetInput{Project: projectID})
	if err != nil {
		respondProjectError(c, err)
		return
	}
	payload, err := out.Config.AsMap()
	if err != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
		return
	}
	response := router.FilterMapFields(payload, fields)
	response["_etag"] = string(out.ETag)
	c.Header("ETag", string(out.ETag))
	router.RespondOK(c, "project retrieved", response)
}

// upsertProject handles PUT /project.
//
// @Summary Create or update project
// @Description Create the project configuration when absent or update an existing one using strong ETag concurrency.
// @Tags project
// @Accept json
// @Produce json
// @Param project query string false "Project override" example("demo")
// @Param fields query string false "Comma-separated list of fields to include"
// @Param If-Match header string false "Strong ETag for optimistic concurrency" example("\"abc123\"")
// @Param payload body map[string]any true "Project configuration payload"
// @Success 200 {object} router.Response{data=map[string]any} "Project updated"
// @Success 201 {object} router.Response{data=map[string]any} "Project created"
// @Header 200 {string} ETag "Strong entity tag for concurrency control"
// @Header 200 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 200 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 200 {string} RateLimit-Reset "Seconds until the window resets"
// @Header 201 {string} Location "Absolute URL for the project"
// @Header 201 {string} ETag "Strong entity tag for concurrency control"
// @Header 201 {string} RateLimit-Limit "Requests allowed in the current window"
// @Header 201 {string} RateLimit-Remaining "Remaining requests in the current window"
// @Header 201 {string} RateLimit-Reset "Seconds until the window resets"
// @Failure 400 {object} router.ProblemDocument "Invalid request"
// @Failure 412 {object} router.ProblemDocument "ETag mismatch"
// @Failure 500 {object} router.ProblemDocument "Internal server error"
// @Router /project [put]
func upsertProject(c *gin.Context) {
	store, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	projectID := router.ProjectFromQueryOrDefault(c)
	if projectID == "" {
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
	out, execErr := projectuc.NewUpsert(store).
		Execute(c.Request.Context(), &projectuc.UpsertInput{Project: projectID, Body: body, IfMatch: ifMatch})
	if execErr != nil {
		respondProjectError(c, execErr)
		return
	}
	payload, err := out.Config.AsMap()
	if err != nil {
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
		return
	}
	fields := router.ParseFieldsQuery(c.Query("fields"))
	response := router.FilterMapFields(payload, fields)
	response["_etag"] = string(out.ETag)
	c.Header("ETag", string(out.ETag))
	if out.Created {
		c.Header("Location", routes.Project())
		router.RespondCreated(c, "project created", response)
	} else {
		router.RespondOK(c, "project updated", response)
	}
}

// deleteProject handles DELETE /project.
//
// @Summary Delete project
// @Description Project deletion is not supported; returns 405.
// @Tags project
// @Produce json
// @Failure 405 {object} router.ProblemDocument "Method not allowed"
// @Router /project [delete]
func deleteProject(c *gin.Context) {
	router.RespondProblem(
		c,
		&router.Problem{Status: http.StatusMethodNotAllowed, Detail: "project deletion not supported"},
	)
}

func respondProjectError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, projectuc.ErrInvalidInput),
		errors.Is(err, projectuc.ErrProjectMissing),
		errors.Is(err, projectuc.ErrNameMismatch):
		router.RespondProblem(c, &router.Problem{Status: http.StatusBadRequest, Detail: err.Error()})
	case errors.Is(err, projectuc.ErrNotFound):
		router.RespondProblem(c, &router.Problem{Status: http.StatusNotFound, Detail: err.Error()})
	case errors.Is(err, projectuc.ErrETagMismatch), errors.Is(err, projectuc.ErrStaleIfMatch):
		router.RespondProblem(c, &router.Problem{Status: http.StatusPreconditionFailed, Detail: err.Error()})
	default:
		router.RespondProblem(c, &router.Problem{Status: http.StatusInternalServerError, Detail: err.Error()})
	}
}
