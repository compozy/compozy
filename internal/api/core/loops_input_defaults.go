package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

// GetLoopInputDefaults returns one explicit file-backed defaults layer.
func (h *BaseHandlers) GetLoopInputDefaults(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	scope, err := parseLoopInputDefaultsScope(c.Query("scope"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	response, err := service.GetLoopInputDefaults(
		c.Request.Context(), c.Param("workspace_id"), c.Param("name"), scope,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetLoopInputDefault returns one configured key with an explicit presence bit.
func (h *BaseHandlers) GetLoopInputDefault(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	scope, err := parseLoopInputDefaultsScope(c.Query("scope"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	response, err := service.GetLoopInputDefault(
		c.Request.Context(), c.Param("workspace_id"), c.Param("name"), c.Param("key"), scope,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopInputDefaults replaces one Loop defaults table at one explicit scope.
func (h *BaseHandlers) PutLoopInputDefaults(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopInputDefaultsRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop input defaults request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.PutLoopInputDefaults(
		c.Request.Context(), c.Param("workspace_id"), c.Param("name"), req,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopInputDefault sets one configured key at one explicit scope.
func (h *BaseHandlers) PutLoopInputDefault(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopInputDefaultRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode Loop input default request: %v", looppkg.ErrValidation, err))
		return
	}
	response, err := service.PutLoopInputDefault(
		c.Request.Context(), c.Param("workspace_id"), c.Param("name"), c.Param("key"), req,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// DeleteLoopInputDefault removes one configured key at one explicit scope.
func (h *BaseHandlers) DeleteLoopInputDefault(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	scope, err := parseLoopInputDefaultsScope(c.Query("scope"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	response, err := service.DeleteLoopInputDefault(
		c.Request.Context(), c.Param("workspace_id"), c.Param("name"), c.Param("key"), scope,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func parseLoopInputDefaultsScope(raw string) (contract.LoopInputDefaultsScope, error) {
	scope := contract.LoopInputDefaultsScope(strings.ToLower(strings.TrimSpace(raw)))
	switch scope {
	case contract.LoopInputDefaultsScopeUser, contract.LoopInputDefaultsScopeWorkspace:
		return scope, nil
	default:
		return "", fmt.Errorf(
			"%w: input-default scope must be user or workspace",
			looppkg.ErrValidation,
		)
	}
}
