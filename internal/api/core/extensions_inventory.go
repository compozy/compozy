package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ExtensionInventory returns the shipped and live resource union for one extension.
func (h *BaseHandlers) ExtensionInventory(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	actor, ok := h.extensionScopedActorContext(c, "inventory", false)
	if !ok {
		return
	}
	payload, err := service.InventoryScoped(c.Request.Context(), name, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// PreviewExtensionEnable returns the mutation-free enable projection for one extension.
func (h *BaseHandlers) PreviewExtensionEnable(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	payload, err := service.Preview(c.Request.Context(), name)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, payload)
}
