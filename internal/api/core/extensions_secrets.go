package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

const (
	extensionActionSecretsList  = "secrets.list"
	extensionActionSecretsSet   = "secrets.set"
	extensionActionSecretsUnset = "secrets.unset"
)

// ListExtensionSecrets returns declared and bound env names without secret material.
func (h *BaseHandlers) ListExtensionSecrets(c *gin.Context) {
	service, name, ok := h.namedExtensionSecretsService(c)
	if !ok {
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionSecretsList, false)
	if !ok {
		return
	}
	payload, err := service.ListExtensionSecrets(c.Request.Context(), name, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// SetExtensionSecrets applies write-only values or existing Vault bindings.
func (h *BaseHandlers) SetExtensionSecrets(c *gin.Context) {
	service, name, ok := h.namedExtensionSecretsService(c)
	if !ok {
		return
	}
	var req contract.SetExtensionSecretsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionSecretsSet, false)
	if !ok {
		return
	}
	payload, err := service.SetExtensionSecrets(c.Request.Context(), name, req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// DeleteExtensionSecret unbinds one environment name.
func (h *BaseHandlers) DeleteExtensionSecret(c *gin.Context) {
	service, name, ok := h.namedExtensionSecretsService(c)
	if !ok {
		return
	}
	envName := strings.TrimSpace(c.Param("env_name"))
	if envName == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("environment name is required"))
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionSecretsUnset, false)
	if !ok {
		return
	}
	if err := service.DeleteExtensionSecret(c.Request.Context(), name, envName, actor); err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) namedExtensionSecretsService(
	c *gin.Context,
) (ExtensionSecretsService, string, bool) {
	base, name, ok := h.namedExtensionService(c)
	if !ok {
		return nil, "", false
	}
	return base, name, true
}
