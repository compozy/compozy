package core

import (
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/gin-gonic/gin"
)

var errExtensionEnablementServiceUnavailable = errors.New("extension enablement service is unavailable")

// ListExtensionEnablement returns effective state for every profile.
func (h *BaseHandlers) ListExtensionEnablement(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	enablement, ok := service.(ExtensionEnablementService)
	if !ok {
		h.respondExtensionError(c, http.StatusServiceUnavailable, errExtensionEnablementServiceUnavailable)
		return
	}
	items, err := enablement.ListEnablement(c.Request.Context(), name)
	if err != nil {
		h.respondExtensionEnablementError(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}

// SetExtensionEnablement writes one profile-specific state.
func (h *BaseHandlers) SetExtensionEnablement(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	var req contract.SetExtensionEnablementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	req.Profile = strings.TrimSpace(req.Profile)
	if req.Profile == "" {
		h.respondExtensionError(c, http.StatusBadRequest, profilepkg.ErrInvalidInput)
		return
	}
	actor, ok := h.extensionActorContext(c, "enablement")
	if !ok {
		return
	}
	enablement, ok := service.(ExtensionEnablementService)
	if !ok {
		h.respondExtensionError(c, http.StatusServiceUnavailable, errExtensionEnablementServiceUnavailable)
		return
	}
	item, err := enablement.SetEnablement(c.Request.Context(), name, req, actor)
	if err != nil {
		h.respondExtensionEnablementError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *BaseHandlers) respondExtensionEnablementError(c *gin.Context, err error) {
	if isProfileDomainError(err) {
		status, payload := profileErrorResponse(err)
		c.AbortWithStatusJSON(status, payload)
		return
	}
	h.respondExtensionError(c, ExtensionStatusCode(err), err)
}
