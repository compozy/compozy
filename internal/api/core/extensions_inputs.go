package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	extensionActionInputsGet = "inputs.get"
	extensionActionInputsSet = "inputs.set"
)

var errExtensionInputsUnavailable = errors.New("extension input setup is unavailable")

// ExtensionInputsService reads and writes the inputs of one installed extension instance. The workspace
// comes from the actor and the profile from its read scope; scope selects global or workspace.
type ExtensionInputsService interface {
	ExtensionInputs(
		ctx context.Context,
		name string,
		scope string,
		actor taskpkg.ActorContext,
	) (contract.ExtensionInputsPayload, error)
	SetExtensionInputs(
		ctx context.Context,
		name string,
		scope string,
		req contract.SetExtensionInputsRequest,
		actor taskpkg.ActorContext,
	) (contract.ExtensionInputsPayload, error)
}

// GetExtensionInputs returns declared inputs with their setup state, never values or Vault references.
func (h *BaseHandlers) GetExtensionInputs(c *gin.Context) {
	service, name, ok := h.namedExtensionInputsService(c)
	if !ok {
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionInputsGet, false)
	if !ok {
		return
	}
	payload, err := service.ExtensionInputs(c.Request.Context(), name, strings.TrimSpace(c.Query("scope")), actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// SetExtensionInputs merges values into one instance and returns the state after the write.
func (h *BaseHandlers) SetExtensionInputs(c *gin.Context) {
	service, name, ok := h.namedExtensionInputsService(c)
	if !ok {
		return
	}
	var req contract.SetExtensionInputsRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionInputsSet, false)
	if !ok {
		return
	}
	payload, err := service.SetExtensionInputs(
		c.Request.Context(), name, strings.TrimSpace(c.Query("scope")), req, actor,
	)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) namedExtensionInputsService(c *gin.Context) (ExtensionInputsService, string, bool) {
	base, name, ok := h.namedExtensionService(c)
	if !ok {
		return nil, "", false
	}
	service, ok := base.(ExtensionInputsService)
	if !ok {
		h.respondExtensionError(c, http.StatusServiceUnavailable, errExtensionInputsUnavailable)
		return nil, "", false
	}
	return service, name, true
}
