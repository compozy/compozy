package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/runtimemcp"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

type RuntimeMCPReader interface {
	ListRuntimeMCP(context.Context, settingspkg.CollectionRequest, string) (runtimemcp.Inventory, error)
}

func (h *BaseHandlers) ListRuntimeMCPServers(c *gin.Context) {
	reader, ok := h.Settings.(RuntimeMCPReader)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionMCPServers)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	result, err := reader.ListRuntimeMCP(c.Request.Context(), req, strings.TrimSpace(c.Query("runtime")))
	if errors.Is(err, runtimemcp.ErrInvalidRuntime) {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func MCPList(ctx context.Context, settings SettingsService, req settingspkg.CollectionRequest, runtime string, includeRuntimes bool) (contract.MCPListResponse, error) {
	req.Collection = settingspkg.CollectionMCPServers
	req.SkipRuntimeStatus = true
	result := contract.MCPListResponse{Compozy: []contract.SettingsMCPServerItemPayload{}, Runtimes: []runtimemcp.Snapshot{}}
	if runtime == "" || !includeRuntimes {
		collection, err := settings.ListCollection(ctx, req)
		if err != nil {
			return result, err
		}
		result.Compozy = settingsMCPServerItemPayloads(collection.MCPServers)
	}
	if includeRuntimes {
		reader, ok := settings.(RuntimeMCPReader)
		if !ok {
			return result, errSettingsServiceUnavailable
		}
		inventory, err := reader.ListRuntimeMCP(ctx, req, runtime)
		if err != nil {
			return result, err
		}
		result.Runtimes = inventory.Runtimes
	}
	return result, nil
}
