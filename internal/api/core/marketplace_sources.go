package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	"github.com/gin-gonic/gin"
)

// MarketplaceSourcesService owns global source registration and persistence.
type MarketplaceSourcesReader interface {
	Status(context.Context) ([]marketplace.SourceState, error)
}

type MarketplaceSourcesService interface {
	MarketplaceSourcesReader
	AddSource(context.Context, string, string, bool) (marketplace.SourceState, error)
	UpdateSource(context.Context, string, bool) (marketplace.SourceState, error)
	RemoveSource(context.Context, string) error
	RefreshSource(context.Context, string) (marketplace.SourceState, error)
}

func (h *BaseHandlers) marketplaceSources(c *gin.Context) MarketplaceSourcesService {
	service, ok := h.MarketplaceCatalog.(MarketplaceSourcesService)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("marketplace sources are unavailable"))
		return nil
	}
	return service
}

func (h *BaseHandlers) ListMarketplaceSources(c *gin.Context) {
	service := h.marketplaceSources(c)
	if service == nil {
		return
	}
	states, err := service.Status(c.Request.Context())
	if err != nil {
		h.respondMarketplaceSourceError(c, err)
		return
	}
	response := contract.MarketplaceSourcesResponse{Sources: make([]contract.MarketplaceSourcePayload, 0, len(states))}
	for _, state := range states {
		response.Sources = append(response.Sources, MarketplaceSourcePayloadFromState(state))
	}
	c.JSON(http.StatusOK, response)
}

func (h *BaseHandlers) AddMarketplaceSource(c *gin.Context) {
	service := h.marketplaceSources(c)
	if service == nil {
		return
	}
	var req contract.AddMarketplaceSourceRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Ref) == "" {
		h.respondMarketplaceSourceError(c, pluginsource.ErrInvalidRef)
		return
	}
	dryRun := false
	if raw, present := c.GetQuery("dry_run"); present {
		var err error
		dryRun, err = strconv.ParseBool(raw)
		if err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
	}
	state, err := service.AddSource(c.Request.Context(), req.Ref, req.Name, dryRun)
	if err != nil {
		h.respondMarketplaceSourceError(c, err)
		return
	}
	payload := MarketplaceSourcePayloadFromState(state)
	if dryRun {
		c.JSON(http.StatusOK, contract.MarketplaceSourcePreviewPayload{
			Name: payload.Name, Owner: payload.Owner, DocumentPath: payload.DocumentPath,
			Plugins: payload.Plugins, Installable: payload.Installable, Diagnostics: payload.Diagnostics,
		})
		return
	}
	if !h.applyMarketplaceSourceConfig(c) {
		return
	}
	c.JSON(http.StatusCreated, contract.MarketplaceSourceResponse{Source: payload})
}

func (h *BaseHandlers) UpdateMarketplaceSource(c *gin.Context) {
	service := h.marketplaceSources(c)
	if service == nil {
		return
	}
	var req contract.UpdateMarketplaceSourceRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if req.Enabled == nil {
		h.respondError(c, http.StatusBadRequest, errors.New("enabled is required"))
		return
	}
	state, err := service.UpdateSource(c.Request.Context(), c.Param("name"), *req.Enabled)
	if err != nil {
		h.respondMarketplaceSourceError(c, err)
		return
	}
	if !h.applyMarketplaceSourceConfig(c) {
		return
	}
	c.JSON(http.StatusOK, contract.MarketplaceSourceResponse{Source: MarketplaceSourcePayloadFromState(state)})
}

func (h *BaseHandlers) RemoveMarketplaceSource(c *gin.Context) {
	service := h.marketplaceSources(c)
	if service == nil {
		return
	}
	if err := service.RemoveSource(c.Request.Context(), c.Param("name")); err != nil {
		h.respondMarketplaceSourceError(c, err)
		return
	}
	if !h.applyMarketplaceSourceConfig(c) {
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) RefreshMarketplaceSource(c *gin.Context) {
	service := h.marketplaceSources(c)
	if service == nil {
		return
	}
	state, err := service.RefreshSource(c.Request.Context(), c.Param("name"))
	if err != nil {
		h.respondMarketplaceSourceError(c, err)
		return
	}
	c.JSON(http.StatusOK, contract.MarketplaceSourceResponse{Source: MarketplaceSourcePayloadFromState(state)})
}

func (h *BaseHandlers) applyMarketplaceSourceConfig(c *gin.Context) bool {
	if h.Settings == nil {
		return true
	}
	if _, err := h.Settings.Reload(c.Request.Context()); err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return false
	}
	return true
}

func MarketplaceSourcePayloadFromState(state marketplace.SourceState) contract.MarketplaceSourcePayload {
	payload := contract.MarketplaceSourcePayload{
		Name:         state.Source,
		Kind:         state.Kind,
		Source:       state.SourceRef,
		Enabled:      state.Enabled,
		State:        "never",
		Plugins:      state.EntryCount,
		Installable:  state.Installable,
		ErrorClass:   state.ErrorClass,
		Error:        state.LastError,
		DocumentPath: state.DocumentPath,
		Owner:        state.Owner,
		Stability:    "experimental",
		Diagnostics:  make([]contract.DiagnosticItem, 0, len(state.Diagnostics)),
	}
	switch {
	case !state.Enabled:
		payload.State = "off"
	case state.LastError != "":
		payload.State = "degraded"
	case !state.FetchedAt.IsZero():
		payload.State = "ok"
	}
	if !state.FetchedAt.IsZero() {
		payload.LastReadAt = &state.FetchedAt
	}
	for _, diagnostic := range state.Diagnostics {
		payload.Diagnostics = append(
			payload.Diagnostics,
			contract.DiagnosticItem{Code: diagnostic.Code, Message: diagnostic.Message},
		)
	}
	return payload
}

func (h *BaseHandlers) respondMarketplaceSourceError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	payload := contract.MarketplaceSourceErrorPayload{Error: err.Error(), Code: "marketplace_source_error"}
	var exists *marketplace.SourceExistsError
	var retained *marketplace.SourceNameRetainedError
	switch {
	case errors.As(err, &exists):
		status, payload.Code = http.StatusConflict, marketplace.ErrSourceExists.Error()
		payload.SuggestedName = exists.SuggestedName
	case errors.As(err, &retained):
		status, payload.Code = http.StatusConflict, marketplace.ErrSourceNameRetained.Error()
		payload.RetainedBy = retained.RetainedBy
	case errors.Is(err, marketplace.ErrSourceNotFound):
		status, payload.Code = http.StatusNotFound, marketplace.ErrSourceNotFound.Error()
	case errors.Is(err, marketplace.ErrSourcePresetReadonly):
		status, payload.Code = http.StatusForbidden, marketplace.ErrSourcePresetReadonly.Error()
	case errors.Is(err, pluginsource.ErrSourceNameReserved):
		status, payload.Code = http.StatusUnprocessableEntity, pluginsource.ErrSourceNameReserved.Error()
	case errors.Is(err, pluginsource.ErrSourceNameInvalid):
		status, payload.Code = http.StatusUnprocessableEntity, pluginsource.ErrSourceNameInvalid.Error()
	case errors.Is(err, pluginsource.ErrInvalidRef):
		status, payload.Code = http.StatusUnprocessableEntity, pluginsource.ErrInvalidRef.Error()
	case errors.Is(err, pluginsource.ErrDocumentTooLarge):
		status, payload.Code = http.StatusUnprocessableEntity, pluginsource.ErrDocumentTooLarge.Error()
	case errors.Is(err, pluginsource.ErrNotMarketplace):
		status, payload.Code = http.StatusUnprocessableEntity, pluginsource.ErrNotMarketplace.Error()
		payload.Checked = []string{"marketplace.json", ".claude-plugin/marketplace.json"}
	case errors.Is(err, pluginsource.ErrSourceUnreachable):
		status, payload.Code = http.StatusServiceUnavailable, "source_unreachable"
	}
	c.JSON(status, payload)
}
