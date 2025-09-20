package resourcesrouter

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/resources/uc"
	"github.com/gin-gonic/gin"
)

// @Summary Create resource
// @Description Create a resource in the current project
// @Tags resources
// @Accept json
// @Produce json
// @Param type path string true "Resource type (agent|tool|mcp|schema|model|workflow|memory|project)"
// @Param resource body object true "Resource payload (must include id; 'type' optional but if present must match path)"
// @Success 201 {object} router.Response{data=object} "Created resource"
// @Header 201 {string} ETag "ETag for the stored value"
// @Failure 400 {object} router.Response{error=router.ErrorInfo}
// @Failure 401 {object} router.Response{error=router.ErrorInfo}
// @Failure 403 {object} router.Response{error=router.ErrorInfo}
// @Router /resources/{type} [post]
func createResource(c *gin.Context) {
	rs, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	typ, ok := resourceTypeFromPath(c)
	if !ok {
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid input", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	prj := projectFromQueryOrDefault(c)
	if prj == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "project is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewCreateResource(rs)
	out, err := usecase.Execute(c.Request.Context(), &uc.CreateInput{Project: prj, Type: typ, Body: body})
	if err != nil {
		switch err {
		case uc.ErrInvalidPayload:
			reqErr := router.NewRequestError(http.StatusBadRequest, "invalid input", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		case uc.ErrProjectInBody:
			reqErr := router.NewRequestError(http.StatusBadRequest, "project field is not allowed in body", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		case uc.ErrMissingID:
			reqErr := router.NewRequestError(http.StatusBadRequest, "missing id in body", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		case uc.ErrIDMismatch:
			reqErr := router.NewRequestError(http.StatusBadRequest, "id mismatch between path and body", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		case uc.ErrTypeMismatch:
			reqErr := router.NewRequestError(http.StatusBadRequest, "type mismatch between path and body", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		case uc.ErrInvalidID:
			reqErr := router.NewRequestError(http.StatusBadRequest, "invalid id", nil)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
		default:
			router.RespondWithServerError(c, router.ErrInternalCode, "failed to store resource", err)
		}
		return
	}
	setETag(c, out.ETag)
	setLocation(c, c.Param("type"), out.ID)
	router.RespondCreated(c, "resource created", out.Value)
}

// @Summary List resources
// @Description List resource keys for a given type and project
// @Tags resources
// @Produce json
// @Param type path string true "Resource type"
// @Param project query string false "Project override"
// @Param q query string false "ID prefix filter"
// @Success 200 {object} router.Response{data=object{keys=[]string}} "Keys listed"
// @Router /resources/{type} [get]
func listResources(c *gin.Context) {
	rs, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	typ, ok := resourceTypeFromPath(c)
	if !ok {
		return
	}
	prj := projectFromQueryOrDefault(c)
	if prj == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "project is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewListResources(rs)
	out, err := usecase.Execute(
		c.Request.Context(),
		&uc.ListInput{Project: prj, Type: typ, Prefix: strings.TrimSpace(c.Query("q"))},
	)
	if err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to list resources", err)
		return
	}
	router.RespondOK(c, "keys listed", gin.H{"keys": out.Keys})
}

// @Summary Get resource
// @Description Get a resource by ID
// @Tags resources
// @Produce json
// @Param type path string true "Resource type"
// @Param id path string true "Resource ID"
// @Success 200 {object} router.Response{data=object} "Resource returned"
// @Header 200 {string} ETag "ETag for the stored value"
// @Failure 404 {object} router.Response{error=router.ErrorInfo}
// @Router /resources/{type}/{id} [get]
func getResourceByID(c *gin.Context) {
	rs, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	typ, ok := resourceTypeFromPath(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "id is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	prj := projectFromQueryOrDefault(c)
	if prj == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "project is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewGetResource(rs)
	out, err := usecase.Execute(c.Request.Context(), &uc.GetInput{Project: prj, Type: typ, ID: id})
	if err != nil {
		if err == uc.ErrNotFound {
			reqErr := router.NewRequestError(http.StatusNotFound, "resource not found", err)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to get resource", err)
		return
	}
	setETag(c, out.ETag)
	router.RespondOK(c, "resource", out.Value)
}

// @Summary Upsert resource
// @Description Upserts a resource. If-Match enforces optimistic locking when provided.
// @Tags resources
// @Accept json
// @Produce json
// @Param type path string true "Resource type"
// @Param id path string true "Resource ID"
// @Param If-Match header string false "Current ETag for optimistic locking"
// @Param resource body object true "Resource payload (full object with id and fields; 'type' optional but if present must match path)"
// @Success 200 {object} router.Response{data=object} "Updated resource"
// @Header 200 {string} ETag "New ETag for the stored value"
// @Failure 409 {object} router.Response{error=router.ErrorInfo}
// @Failure 400 {object} router.Response{error=router.ErrorInfo}
// @Failure 401 {object} router.Response{error=router.ErrorInfo}
// @Failure 403 {object} router.Response{error=router.ErrorInfo}
// @Router /resources/{type}/{id} [put]
func putResourceByID(c *gin.Context) {
	rs, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	typ, ok := resourceTypeFromPath(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "id is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid input", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	prj := projectFromQueryOrDefault(c)
	if prj == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "project is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	ifMatch := strings.TrimSpace(c.GetHeader("If-Match"))
	usecase := uc.NewUpsertResource(rs)
	out, err := usecase.Execute(
		c.Request.Context(),
		&uc.UpsertInput{Project: prj, Type: typ, ID: id, Body: body, IfMatch: ifMatch},
	)
	if err != nil {
		handleUpsertError(c, err)
		return
	}
	setETag(c, out.ETag)
	router.RespondOK(c, "resource updated", out.Value)
}

func handleUpsertError(c *gin.Context, err error) {
	switch err {
	case uc.ErrInvalidPayload:
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid input", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrProjectInBody:
		reqErr := router.NewRequestError(http.StatusBadRequest, "project field is not allowed in body", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrIDMismatch:
		reqErr := router.NewRequestError(http.StatusBadRequest, "id mismatch between path and body", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrTypeMismatch:
		reqErr := router.NewRequestError(http.StatusBadRequest, "type mismatch between path and body", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrInvalidID:
		reqErr := router.NewRequestError(http.StatusBadRequest, "invalid id", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrIfMatchStaleOrMissing:
		reqErr := router.NewRequestError(http.StatusConflict, "stale or missing resource for If-Match", err)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	case uc.ErrETagMismatch:
		reqErr := router.NewRequestError(http.StatusConflict, "etag mismatch", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
	default:
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to upsert resource", err)
	}
}

// @Summary Delete resource
// @Description Delete a resource by ID (idempotent)
// @Tags resources
// @Param type path string true "Resource type"
// @Param id path string true "Resource ID"
// @Success 200 {object} router.Response{data=object} "Deleted"
// @Failure 401 {object} router.Response{error=router.ErrorInfo}
// @Failure 403 {object} router.Response{error=router.ErrorInfo}
// @Router /resources/{type}/{id} [delete]
func deleteResourceByID(c *gin.Context) {
	rs, ok := router.GetResourceStore(c)
	if !ok {
		return
	}
	typ, ok := resourceTypeFromPath(c)
	if !ok {
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "id is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	prj := projectFromQueryOrDefault(c)
	if prj == "" {
		reqErr := router.NewRequestError(http.StatusBadRequest, "project is required", nil)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}
	usecase := uc.NewDeleteResource(rs)
	if err := usecase.Execute(c.Request.Context(), &uc.DeleteInput{Project: prj, Type: typ, ID: id}); err != nil {
		router.RespondWithServerError(c, router.ErrInternalCode, "failed to delete resource", err)
		return
	}
	router.RespondOK(c, "resource deleted", gin.H{"id": id})
}
