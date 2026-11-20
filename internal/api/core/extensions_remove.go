package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// extensionPermanentQueryKey is the DELETE query flag that selects permanent removal of a built-in.
const extensionPermanentQueryKey = "permanent"

var (
	errExtensionPermanentScoped = errors.New(
		"permanent removal applies to the global built-in; drop the workspace or profile query",
	)
	errExtensionPermanentUnsupported = errors.New("permanent removal is not available on this extension service")
)

// ExtensionPermanentRemoveService exposes operator-only permanent removal of a bundled built-in.
// The daemon service records the tombstone; transports only dispatch to it.
type ExtensionPermanentRemoveService interface {
	RemovePermanently(
		ctx context.Context,
		name string,
		actor taskpkg.ActorContext,
	) (contract.ManagedExtensionRemovePayload, error)
}

// RemoveExtension removes one managed extension via the daemon. The permanent query flag selects
// permanent removal of a bundled built-in; without it the response gains only additive fields.
func (h *BaseHandlers) RemoveExtension(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionRemove)
	if !ok {
		return
	}
	permanent, err := extensionPermanentRequested(c)
	if err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	if permanent {
		h.removeExtensionPermanently(c, service, name, actor)
		return
	}

	var item contract.ManagedExtensionRemovePayload
	if scoped, supportsScope := service.(ExtensionScopedRemoveService); supportsScope && hasExtensionScopeRequest(c) {
		actor, actorOK := h.extensionScopedActorContext(c, extensionActionRemove, false)
		if !actorOK {
			return
		}
		item, err = scoped.RemoveScoped(c.Request.Context(), name, actor)
	} else {
		item, err = service.Remove(c.Request.Context(), name, actor)
	}
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionRemoveResponse{Extension: item})
}

// removeExtensionPermanently writes the permanent-removal response. A built-in has no workspace or
// profile scope, so those query values are refused before the daemon is asked.
func (h *BaseHandlers) removeExtensionPermanently(
	c *gin.Context,
	service ExtensionService,
	name string,
	actor taskpkg.ActorContext,
) {
	if strings.TrimSpace(c.Query("workspace")) != "" || strings.TrimSpace(c.Query("profile")) != "" {
		h.respondExtensionError(c, http.StatusBadRequest, errExtensionPermanentScoped)
		return
	}
	permanent, ok := service.(ExtensionPermanentRemoveService)
	if !ok {
		h.respondExtensionError(c, http.StatusNotImplemented, errExtensionPermanentUnsupported)
		return
	}
	item, err := permanent.RemovePermanently(c.Request.Context(), name, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionRemoveResponse{Extension: item})
}

// extensionPermanentRequested parses the permanent flag. An absent flag selects non-permanent removal.
func extensionPermanentRequested(c *gin.Context) (bool, error) {
	raw := strings.TrimSpace(c.Query(extensionPermanentQueryKey))
	if raw == "" {
		return false, nil
	}
	permanent, err := strconv.ParseBool(raw)
	if err != nil {
		return false, errors.New("permanent must be a boolean")
	}
	return permanent, nil
}
