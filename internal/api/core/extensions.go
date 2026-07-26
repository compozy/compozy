package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	extensionActionInstall           = "install"
	extensionActionUpdate            = "update"
	extensionActionRemove            = "remove"
	extensionActionEnable            = "enable"
	extensionActionDisable           = "disable"
	extensionReservedMarketplaceName = "marketplace"
)

// ExtensionService exposes daemon-backed extension management to API transports.
type ExtensionService interface {
	List(ctx context.Context) ([]contract.ExtensionPayload, error)
	MarketplaceTrust(
		ctx context.Context,
		evidence extensionpkg.MarketplaceTrustEvidence,
	) (contract.ExtensionTrustReportPayload, error)
	Install(
		ctx context.Context,
		req contract.InstallExtensionRequest,
		actor taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	Update(
		ctx context.Context,
		name string,
		req contract.UpdateExtensionRequest,
		actor taskpkg.ActorContext,
	) (contract.ManagedExtensionUpdatePayload, error)
	Remove(ctx context.Context, name string, actor taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	Enable(ctx context.Context, name string, actor taskpkg.ActorContext) (contract.ExtensionPayload, error)
	Disable(ctx context.Context, name string, actor taskpkg.ActorContext) (contract.ExtensionPayload, error)
	Status(ctx context.Context, name string) (contract.ExtensionPayload, error)
	Provenance(ctx context.Context, name string) (contract.ExtensionProvenancePayload, error)
}

// ListExtensions returns daemon-owned installed extension state.
func (h *BaseHandlers) ListExtensions(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}

	items, err := service.List(c.Request.Context())
	if err != nil {
		h.respondExtensionError(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.joinInstalledExtensionMarketplace(c.Request.Context(), items); err != nil {
		h.Logger.Warn("api: installed extension marketplace enrichment failed", "error", err)
	}
	c.JSON(http.StatusOK, contract.ExtensionsResponse{Extensions: items})
}

// InstallExtension installs either a local path or a marketplace slug via the daemon.
func (h *BaseHandlers) InstallExtension(c *gin.Context) {
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
	if req.Path == "" && req.Slug == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("path or slug is required"))
		return
	}
	if req.Path != "" && req.Slug != "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("path and slug are mutually exclusive"))
		return
	}
	if req.Path != "" && req.Checksum == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("checksum is required for local installs"))
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionInstall)
	if !ok {
		return
	}

	item, err := service.Install(c.Request.Context(), req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.ExtensionResponse{Extension: item})
}

// UpdateExtension updates one marketplace-installed extension via the daemon.
func (h *BaseHandlers) UpdateExtension(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}

	var req contract.UpdateExtensionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	req.Version = strings.TrimSpace(req.Version)
	actor, ok := h.extensionActorContext(c, extensionActionUpdate)
	if !ok {
		return
	}
	item, err := service.Update(c.Request.Context(), name, req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionUpdateResponse{Update: item})
}

// RemoveExtension removes one managed extension via the daemon.
func (h *BaseHandlers) RemoveExtension(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionRemove)
	if !ok {
		return
	}

	item, err := service.Remove(c.Request.Context(), name, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionRemoveResponse{Extension: item})
}

// EnableExtension enables one installed extension.
func (h *BaseHandlers) EnableExtension(c *gin.Context) {
	h.mutateExtensionEnabled(c, true)
}

// DisableExtension disables one installed extension.
func (h *BaseHandlers) DisableExtension(c *gin.Context) {
	h.mutateExtensionEnabled(c, false)
}

// ExtensionStatus returns one installed extension's runtime status.
func (h *BaseHandlers) ExtensionStatus(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}

	item, err := service.Status(c.Request.Context(), name)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionResponse{Extension: item})
}

// ExtensionProvenance returns one installed extension's persisted trust report.
func (h *BaseHandlers) ExtensionProvenance(c *gin.Context) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}

	item, err := service.Provenance(c.Request.Context(), name)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionProvenanceResponse{Provenance: item})
}

// ExtensionStatusCode maps extension-domain errors onto transport status codes.
func ExtensionStatusCode(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, extensionpkg.ErrExtensionNotFound):
		return http.StatusNotFound
	case errors.Is(err, extensionpkg.ErrExtensionExists):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionArchiveDigestMismatch):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified),
		errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked):
		return http.StatusUnprocessableEntity
	case errors.Is(err, extensionpkg.ErrManifestInvalid):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrManifestIncompatible):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrManifestNotFound):
		return http.StatusBadRequest
	case errors.Is(err, extensionpkg.ErrExtensionHasActiveBundles):
		return http.StatusConflict
	case errors.Is(err, extensionpkg.ErrMarketplaceSourceUnavailable):
		return http.StatusServiceUnavailable
	case errors.Is(err, os.ErrNotExist):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func (h *BaseHandlers) mutateExtensionEnabled(c *gin.Context, enabled bool) {
	service, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	action := extensionActionDisable
	if enabled {
		action = extensionActionEnable
	}
	actor, ok := h.extensionActorContext(c, action)
	if !ok {
		return
	}

	var (
		item contract.ExtensionPayload
		err  error
	)
	if enabled {
		item, err = service.Enable(c.Request.Context(), name, actor)
	} else {
		item, err = service.Disable(c.Request.Context(), name, actor)
	}
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionResponse{Extension: item})
}

func (h *BaseHandlers) namedExtensionService(c *gin.Context) (ExtensionService, string, bool) {
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("name is required"))
		return nil, "", false
	}
	if name == extensionReservedMarketplaceName {
		h.respondExtensionError(
			c,
			http.StatusNotFound,
			&extensionpkg.ExtensionNotFoundError{Name: name},
		)
		return nil, "", false
	}
	service, ok := h.extensionService(c)
	if !ok {
		return nil, "", false
	}
	return service, name, true
}

func (h *BaseHandlers) extensionService(c *gin.Context) (ExtensionService, bool) {
	if h == nil || h.Extensions == nil {
		h.respondExtensionError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: extension service is not configured"),
		)
		return nil, false
	}
	return h.Extensions, true
}

func (h *BaseHandlers) extensionActorContext(c *gin.Context, action string) (taskpkg.ActorContext, bool) {
	action = "extensions." + strings.TrimSpace(action)
	if h.TaskActorContextResolver != nil {
		actor, err := h.TaskActorContextResolver(c, action)
		if err != nil {
			h.respondExtensionError(c, StatusForTaskError(err), err)
			return taskpkg.ActorContext{}, false
		}
		return actor, true
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, action, "")
		if err != nil {
			h.respondExtensionError(c, StatusForTaskError(err), err)
			return taskpkg.ActorContext{}, false
		}
		return caller.Actor, true
	}
	actor, err := taskpkg.DeriveHumanActorContext(
		defaultTaskActorRef,
		taskOriginKindForTransport(h.transportName()),
		action,
	)
	if err != nil {
		h.respondExtensionError(c, StatusForTaskError(err), err)
		return taskpkg.ActorContext{}, false
	}
	return actor, true
}

func (h *BaseHandlers) respondExtensionError(c *gin.Context, status int, err error) {
	mask := false
	if h != nil {
		mask = h.MaskInternalErrors
	}
	RespondError(c, status, err, mask)
}

func normalizeInstallExtensionRequest(req *contract.InstallExtensionRequest) {
	if req == nil {
		return
	}
	req.Path = strings.TrimSpace(req.Path)
	req.Checksum = strings.TrimSpace(req.Checksum)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Source = strings.TrimSpace(req.Source)
	req.Version = strings.TrimSpace(req.Version)
	req.Asset = strings.TrimSpace(req.Asset)
}
