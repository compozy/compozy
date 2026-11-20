package core

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// extensionActionRestore names the restore action in actor-resolution errors and audit records.
const extensionActionRestore = "restore"

var (
	errExtensionRestoreUnsupported = errors.New("restore is not available on this extension service")
	errExtensionRemovedUnsupported = errors.New("removed built-in listing is not available on this extension service")
)

// ExtensionBuiltinRestoreService restores a permanently removed built-in at its bundled version.
// The daemon service clears the tombstone and enrolls the built-in; transports only dispatch to it.
type ExtensionBuiltinRestoreService interface {
	RestoreBuiltin(
		ctx context.Context,
		name string,
		actor taskpkg.ActorContext,
	) (contract.ExtensionRestorePayload, error)
}

// ExtensionRemovedBuiltinsService lists the built-ins that are permanently removed and restorable.
type ExtensionRemovedBuiltinsService interface {
	ListRemovedBuiltins(ctx context.Context) ([]contract.ExtensionRemovedBuiltinPayload, error)
}

// RestoreExtension restores one permanently removed built-in. Write authority is checked by the daemon
// service, which also refuses names with no tombstone (409) and names with a non-bundled row (409).
func (h *BaseHandlers) RestoreExtension(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	restorer, supportsRestore := service.(ExtensionBuiltinRestoreService)
	if !supportsRestore {
		h.respondExtensionError(c, http.StatusNotImplemented, errExtensionRestoreUnsupported)
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionRestore)
	if !ok {
		return
	}
	item, err := restorer.RestoreBuiltin(c.Request.Context(), name, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, item)
}

// ListRemovedBuiltinExtensions returns the permanently removed built-ins. An empty listing is `[]`.
func (h *BaseHandlers) ListRemovedBuiltinExtensions(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}
	lister, supportsList := service.(ExtensionRemovedBuiltinsService)
	if !supportsList {
		h.respondExtensionError(c, http.StatusNotImplemented, errExtensionRemovedUnsupported)
		return
	}
	items, err := lister.ListRemovedBuiltins(c.Request.Context())
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	if items == nil {
		items = []contract.ExtensionRemovedBuiltinPayload{}
	}
	c.JSON(http.StatusOK, contract.ExtensionRemovedBuiltinsResponse{Removed: items})
}
