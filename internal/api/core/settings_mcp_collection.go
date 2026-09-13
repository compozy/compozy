package core

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/vault"
	"github.com/gin-gonic/gin"
)

// GetSettingsMCPServer returns an exact owner-qualified definition, with manual-first legacy lookup.
func (h *BaseHandlers) GetSettingsMCPServer(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionMCPServers)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	req.Owner = strings.TrimSpace(c.Query("owner"))
	if err := vault.ValidateMCPOwner(req.Owner); err != nil {
		h.respondError(c, http.StatusBadRequest, NewSettingsValidationError(err))
		return
	}
	envelope, err := h.Settings.ListCollection(c.Request.Context(), req)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	item, found := findSettingsMCPServer(envelope.MCPServers, req, name)
	if !found {
		h.respondError(c, http.StatusNotFound, NewSettingsNotFoundError(fmt.Errorf("MCP server %q not found", name)))
		return
	}
	c.JSON(
		http.StatusOK,
		contract.SettingsMCPServerResponse{Server: settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{item})[0]},
	)
}

func findSettingsMCPServer(
	items []settingspkg.MCPServerItem,
	req settingspkg.CollectionRequest,
	name string,
) (settingspkg.MCPServerItem, bool) {
	if req.Owner == "" || req.Owner == "manual" {
		for _, item := range items {
			if vault.NormalizeMCPOwner(item.Owner) == "manual" && item.Name == name {
				return item, true
			}
		}
	}
	var selected settingspkg.MCPServerItem
	selectedRank := 0
	for _, item := range items {
		owner := vault.NormalizeMCPOwner(item.Owner)
		if owner == "manual" || (req.Owner != "" && req.Owner != owner) {
			continue
		}
		rank := settingsMCPItemScopeRank(item, req)
		if rank <= selectedRank {
			continue
		}
		if settingsMCPRuntimeName(item) == name || (req.Owner != "" && item.Name == name) {
			selected, selectedRank = item, rank
		}
	}
	return selected, selectedRank > 0
}

func settingsMCPItemScopeRank(item settingspkg.MCPServerItem, req settingspkg.CollectionRequest) int {
	if item.ProfileName != req.ProfileName {
		return 0
	}
	if item.Scope == req.Scope && item.WorkspaceID == req.WorkspaceID {
		return 2
	}
	if req.WorkspaceID == "" || item.WorkspaceID != "" {
		return 0
	}
	if req.Scope == settingspkg.ScopeWorkspace && item.Scope == settingspkg.ScopeUser {
		return 1
	}
	if req.Scope == settingspkg.ScopeProfile && item.Scope == settingspkg.ScopeProfile {
		return 1
	}
	return 0
}
