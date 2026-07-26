package core

import (
	"fmt"

	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) getSettingsSection(c *gin.Context, section settingspkg.SectionName) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	req, err := parseSettingsSectionRequest(c, section)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	envelope, err := h.Settings.GetSection(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	payload, err := SettingsSectionResponseFromEnvelope(envelope)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) updateSettingsSection(c *gin.Context, req settingspkg.SectionUpdateRequest) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	result, err := h.Settings.ApplySection(
		settingspkg.WithMutationSource(c.Request.Context(), h.TransportName),
		req,
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsApplyResponseFromResult(result))
}

func (h *BaseHandlers) listSettingsCollection(c *gin.Context, collection settingspkg.CollectionName) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	req, err := parseSettingsCollectionRequest(c, collection)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	envelope, err := h.Settings.ListCollection(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	payload, err := SettingsCollectionResponseFromEnvelope(envelope)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, payload)
}

func (h *BaseHandlers) getSettingsCollectionItem(c *gin.Context, collection settingspkg.CollectionName) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	req, err := parseSettingsCollectionRequest(c, collection)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	envelope, err := h.Settings.ListCollection(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	switch collection {
	case settingspkg.CollectionProviders:
		item, found := findSettingsProvider(envelope.Providers, name)
		if !found {
			notFound := NewSettingsNotFoundError(fmt.Errorf("provider %q not found", name))
			h.respondError(c, StatusForSettingsError(notFound), notFound)
			return
		}
		c.JSON(http.StatusOK, contract.SettingsProviderResponse{Provider: settingsProviderItemPayload(&item)})
	case settingspkg.CollectionSandboxes:
		item, found := findSettingsSandbox(envelope.Sandboxes, name)
		if !found {
			notFound := NewSettingsNotFoundError(fmt.Errorf("sandbox %q not found", name))
			h.respondError(c, StatusForSettingsError(notFound), notFound)
			return
		}
		c.JSON(http.StatusOK, contract.SettingsSandboxResponse{
			Sandbox: contract.SettingsSandboxItemPayload{
				Name:                strings.TrimSpace(item.Name),
				Profile:             settingsSandboxProfilePayload(item.Profile),
				WorkspaceUsageCount: item.WorkspaceUsageCount,
				SourceMetadata:      settingsSourceMetadataPayload(item.SourceMetadata),
			},
		})
	default:
		h.respondError(
			c,
			http.StatusInternalServerError,
			fmt.Errorf("settings item lookup unsupported for %q", collection),
		)
	}
}

func (h *BaseHandlers) putSettingsCollectionItem(c *gin.Context, req settingspkg.CollectionItemPutRequest) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	result, err := h.Settings.ApplyCollectionItem(
		settingspkg.WithMutationSource(c.Request.Context(), h.TransportName),
		req,
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsApplyResponseFromResult(result))
}

func (h *BaseHandlers) deleteSettingsCollectionItem(c *gin.Context, req settingspkg.CollectionItemDeleteRequest) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}

	result, err := h.Settings.ApplyCollectionDelete(
		settingspkg.WithMutationSource(c.Request.Context(), h.TransportName),
		req,
	)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}

	c.JSON(http.StatusOK, SettingsApplyResponseFromResult(result))
}
