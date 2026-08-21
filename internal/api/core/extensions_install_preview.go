package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/gin-gonic/gin"
)

var errExtensionInstallPreviewUnavailable = errors.New("extension install preview service is unavailable")

// PreviewExtensionInstall returns the exact declared-profile and placement summary without mutation.
func (h *BaseHandlers) PreviewExtensionInstall(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}
	var req contract.InstallExtensionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	normalizeInstallExtensionRequest(&req)
	if err := validateInstallExtensionRequest(req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionInstall+".preview")
	if !ok {
		return
	}
	previewService, ok := service.(ExtensionInstallPreviewService)
	if !ok {
		h.respondExtensionError(c, http.StatusServiceUnavailable, errExtensionInstallPreviewUnavailable)
		return
	}
	preview, err := previewService.PreviewInstall(c.Request.Context(), req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, preview)
}
