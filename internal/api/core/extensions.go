package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
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
	ExtensionSecretsService

	List(ctx context.Context) ([]contract.ExtensionPayload, error)
	Search(ctx context.Context, req contract.ExtensionSearchRequest) (contract.ExtensionSearchResponse, error)
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
	UpdateBatch(
		ctx context.Context,
		req contract.UpdateExtensionsRequest,
		actor taskpkg.ActorContext,
	) ([]contract.ManagedExtensionUpdatePayload, error)
	Remove(ctx context.Context, name string, actor taskpkg.ActorContext) (contract.ManagedExtensionRemovePayload, error)
	Enable(
		ctx context.Context,
		name string,
		req contract.EnableExtensionRequest,
		actor taskpkg.ActorContext,
	) (contract.ExtensionEnableResult, error)
	Disable(ctx context.Context, name string, actor taskpkg.ActorContext) (contract.ExtensionPayload, error)
	Status(ctx context.Context, name string) (contract.ExtensionPayload, error)
	Provenance(ctx context.Context, name string) (contract.ExtensionProvenancePayload, error)
	Inventory(ctx context.Context, name string) (contract.ExtensionInventoryPayload, error)
	Preview(ctx context.Context, name string) (contract.ExtensionEnablePreviewPayload, error)
}

// ExtensionSecretsService exposes presence-only, caller-scoped secret bindings.
type ExtensionSecretsService interface {
	ListExtensionSecrets(
		ctx context.Context,
		name string,
		actor taskpkg.ActorContext,
	) (contract.ExtensionSecretsPayload, error)
	SetExtensionSecrets(
		ctx context.Context,
		name string,
		req contract.SetExtensionSecretsRequest,
		actor taskpkg.ActorContext,
	) (contract.ExtensionSecretsPayload, error)
	DeleteExtensionSecret(
		ctx context.Context,
		name string,
		envName string,
		actor taskpkg.ActorContext,
	) error
}

// SearchExtensions returns one stable page from curated and GitHub discovery.
func (h *BaseHandlers) SearchExtensions(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}
	limit := 20
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed <= 0 || parsed > 100 {
			h.respondExtensionError(
				c,
				http.StatusBadRequest,
				errors.New("extension search limit must be between 1 and 100"),
			)
			return
		}
		limit = parsed
	}
	sources := []string{}
	for source := range strings.SplitSeq(c.Query("sources"), ",") {
		if source = strings.TrimSpace(source); source != "" {
			sources = append(sources, source)
		}
	}
	response, err := service.Search(c.Request.Context(), contract.ExtensionSearchRequest{
		Query: c.Query("q"), Sources: sources, Limit: limit, Cursor: c.Query("cursor"),
	})
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, response)
}

// ListExtensions returns daemon-owned installed extension state.
func (h *BaseHandlers) ListExtensions(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}

	var items []contract.ExtensionPayload
	var err error
	if scoped, supportsScope := service.(ExtensionScopedReadService); supportsScope && hasExtensionScopeRequest(c) {
		actor, actorOK := h.extensionScopedActorContext(c, "list", false)
		if !actorOK {
			return
		}
		items, err = scoped.ListScoped(c.Request.Context(), actor)
	} else {
		items, err = service.List(c.Request.Context())
	}
	if err != nil {
		h.respondExtensionError(c, http.StatusInternalServerError, err)
		return
	}
	if err := h.joinInstalledExtensionMarketplace(c.Request.Context(), items); err != nil {
		h.Logger.Warn("api: installed extension marketplace enrichment failed", "error", err)
	}
	c.JSON(http.StatusOK, contract.ExtensionsResponse{Extensions: items})
}

// InstallExtension installs one extension from the closed distribution-source union.
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
	if err := validateInstallExtensionRequest(req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
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

// UpdateExtensions updates a daemon-selected batch and preserves per-item outcomes.
func (h *BaseHandlers) UpdateExtensions(c *gin.Context) {
	service, ok := h.extensionService(c)
	if !ok {
		return
	}
	var req contract.UpdateExtensionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	if err := validateUpdateExtensionsRequest(req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	actor, ok := h.extensionActorContext(c, extensionActionUpdate)
	if !ok {
		return
	}
	items, err := service.UpdateBatch(c.Request.Context(), req, actor)
	if err != nil {
		var batchErr *extensionpkg.MarketplaceUpdateBatchError
		if !errors.As(err, &batchErr) || len(items) == 0 {
			h.respondExtensionError(c, ExtensionStatusCode(err), err)
			return
		}
		h.Logger.Warn("api: extension update batch completed with per-item failure", "error", err)
	}
	c.JSON(http.StatusOK, contract.ExtensionUpdateBatchResponse{Updates: items})
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

	var item contract.ManagedExtensionRemovePayload
	var err error
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

	var item contract.ExtensionPayload
	var err error
	if scoped, supportsScope := service.(ExtensionScopedReadService); supportsScope && hasExtensionScopeRequest(c) {
		actor, actorOK := h.extensionScopedActorContext(c, "status", false)
		if !actorOK {
			return
		}
		item, err = scoped.StatusScoped(c.Request.Context(), name, actor)
	} else {
		item, err = service.Status(c.Request.Context(), name)
	}
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionResponse{Extension: item})
}

func hasExtensionScopeRequest(c *gin.Context) bool {
	return hasAgentCallerIdentityCredentials(agentCallerCredentialsFromRequest(c)) ||
		strings.TrimSpace(c.Query("workspace")) != ""
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

	if enabled {
		var req contract.EnableExtensionRequest
		if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
			h.respondExtensionError(c, http.StatusBadRequest, err)
			return
		}
		result, err := service.Enable(c.Request.Context(), name, req, actor)
		if err != nil {
			h.respondExtensionError(c, ExtensionStatusCode(err), err)
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}
	item, err := service.Disable(c.Request.Context(), name, actor)
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

func normalizeInstallExtensionRequest(req *contract.InstallExtensionRequest) {
	if req == nil {
		return
	}
	req.Source = contract.InstallExtensionSource(strings.ToLower(strings.TrimSpace(string(req.Source))))
	req.Ref = strings.TrimSpace(req.Ref)
	req.Version = strings.TrimSpace(req.Version)
	req.Asset = strings.TrimSpace(req.Asset)
}

func validateInstallExtensionRequest(req contract.InstallExtensionRequest) error {
	switch req.Source {
	case contract.InstallExtensionSourceCurated,
		contract.InstallExtensionSourceGitHub,
		contract.InstallExtensionSourceGit,
		contract.InstallExtensionSourceLocalPath:
	case "":
		return errors.New("extension source is required")
	default:
		return fmt.Errorf("unsupported extension source %q", req.Source)
	}
	if req.Ref == "" {
		return errors.New("extension ref is required")
	}
	return nil
}

func validateUpdateExtensionsRequest(req contract.UpdateExtensionsRequest) error {
	if req.All && len(req.Names) > 0 {
		return errors.New("extension update accepts names or all, not both")
	}
	if !req.All && len(req.Names) == 0 {
		return errors.New("extension update requires at least one name unless all is set")
	}
	for _, name := range req.Names {
		if strings.TrimSpace(name) == "" {
			return errors.New("extension update names must not be blank")
		}
	}
	return nil
}
