package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/gin-gonic/gin"
)

// CreateLoop creates or forks one writable Loop definition.
func (h *BaseHandlers) CreateLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.CreateLoopRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, loopDecodeError("create", err))
		return
	}
	profileID, ok := h.loopDefinitionMutationProfileID(c)
	if !ok {
		return
	}
	response, err := service.CreateLoop(c.Request.Context(), c.Param("workspace_id"), profileID, req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusCreated, response)
}

// GetLoop inspects one resolved Loop definition.
func (h *BaseHandlers) GetLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	profileID, ok := h.loopDefinitionReadProfileID(c)
	if !ok {
		return
	}
	response, err := service.GetLoop(c.Request.Context(), c.Param("workspace_id"), profileID, c.Param("name"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PatchLoop atomically lints, compiles, and publishes one Loop definition.
func (h *BaseHandlers) PatchLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PatchLoopRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, loopDecodeError("patch", err))
		return
	}
	profileID, ok := h.loopDefinitionMutationProfileID(c)
	if !ok {
		return
	}
	response, err := service.PatchLoop(c.Request.Context(), c.Param("workspace_id"), profileID, c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ValidateLoop lints and compiles one Loop definition without saving it.
func (h *BaseHandlers) ValidateLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.ValidateLoopRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, loopDecodeError("validate", err))
		return
	}
	profileID, ok := h.loopDefinitionReadProfileID(c)
	if !ok {
		return
	}
	response, err := service.ValidateLoop(c.Request.Context(), c.Param("workspace_id"), profileID, c.Param("name"), req)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// DeleteLoop deletes one writable Loop definition.
func (h *BaseHandlers) DeleteLoop(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	profileID, ok := h.loopDefinitionMutationProfileID(c)
	if !ok {
		return
	}
	if err := service.DeleteLoop(c.Request.Context(), c.Param("workspace_id"), profileID, c.Param("name")); err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetLoopConfig returns one no-fork Loop config override.
func (h *BaseHandlers) GetLoopConfig(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	profileID, ok := h.loopDefinitionReadProfileID(c)
	if !ok {
		return
	}
	response, err := service.GetLoopConfig(c.Request.Context(), c.Param("workspace_id"), profileID, c.Param("name"))
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopConfig replaces one no-fork Loop config override.
func (h *BaseHandlers) PutLoopConfig(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopConfigRequest
	if err := decodeStrictLoopJSONBody(c, &req); err != nil {
		h.respondLoopError(c, loopDecodeError("config", err))
		return
	}
	profileID, ok := h.loopDefinitionMutationProfileID(c)
	if !ok {
		return
	}
	response, err := service.PutLoopConfig(
		c.Request.Context(),
		c.Param("workspace_id"),
		profileID,
		c.Param("name"),
		req,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetLoopAnnotations returns editor node positions for one Loop.
func (h *BaseHandlers) GetLoopAnnotations(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	profileID, ok := h.loopDefinitionReadProfileID(c)
	if !ok {
		return
	}
	response, err := service.GetLoopAnnotations(
		c.Request.Context(),
		c.Param("workspace_id"),
		profileID,
		c.Param("name"),
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// PutLoopAnnotations replaces editor node positions for one Loop.
func (h *BaseHandlers) PutLoopAnnotations(c *gin.Context) {
	service, ok := h.requireLoopService(c)
	if !ok {
		return
	}
	var req contract.PutLoopAnnotationsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondLoopError(c, fmt.Errorf("%w: decode loop annotations request: %v", looppkg.ErrValidation, err))
		return
	}
	profileID, ok := h.loopDefinitionMutationProfileID(c)
	if !ok {
		return
	}
	response, err := service.PutLoopAnnotations(
		c.Request.Context(),
		c.Param("workspace_id"),
		profileID,
		c.Param("name"),
		req,
	)
	if err != nil {
		h.respondLoopError(c, err)
		return
	}
	c.JSON(http.StatusOK, response)
}

func loopDecodeError(operation string, err error) error {
	message := fmt.Sprintf("decode %s loop request: %v", operation, err)
	if strings.Contains(message, `unknown field "model_defaults"`) ||
		strings.Contains(message, `unknown field "model"`) ||
		strings.Contains(message, "unknown_field: model_defaults") ||
		strings.Contains(message, "unknown_field: model") {
		message += "; see MIGRATION_GUIDE.md#per-task-runtime-selection"
	}
	return fmt.Errorf("%w: %s", looppkg.ErrValidation, message)
}
