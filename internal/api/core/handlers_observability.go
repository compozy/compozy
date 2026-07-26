package core

import (
	"context"

	"errors"
	"fmt"

	"log/slog"
	"net/http"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/gin-gonic/gin"
)

// HookCatalog returns the resolved hook catalog for the supplied workspace and agent view.
func (h *BaseHandlers) HookCatalog(c *gin.Context) {
	filter, err := ParseHookCatalogFilter(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	if workspaceRef := strings.TrimSpace(c.Query("workspace")); workspaceRef != "" {
		resolved, err := h.Workspaces.Resolve(c.Request.Context(), workspaceRef)
		if err != nil {
			h.respondError(c, StatusForWorkspaceError(err), err)
			return
		}
		filter.WorkspaceID = strings.TrimSpace(resolved.WorkspaceID)
		filter.WorkspaceRoot = strings.TrimSpace(resolved.RootDir)
	}

	entries, err := h.Observer.QueryHookCatalog(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.HookCatalogResponse{Hooks: HookCatalogPayloadsFromEntries(entries)})
}

// HookRuns returns persisted hook execution history for a session.
func (h *BaseHandlers) HookRuns(c *gin.Context) {
	query, err := ParseHookRunsQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(query.SessionID) == "" {
		h.respondError(c, http.StatusBadRequest, errors.New("session query is required"))
		return
	}

	scope, ok := h.resolveWorkspaceScope(c)
	if !ok {
		return
	}
	if _, err := h.requireSessionInWorkspace(
		c.Request.Context(),
		scope.SessionWorkspaceID(),
		query.SessionID,
	); err != nil {
		h.respondError(c, statusForWorkspaceScopedResourceError(err), err)
		return
	}

	records, err := h.Observer.QueryHookRuns(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.HookRunsResponse{Runs: HookRunPayloadsFromRecords(records)})
}

// HookEvents returns the supported hook taxonomy metadata.
func (h *BaseHandlers) HookEvents(c *gin.Context) {
	filter, err := ParseHookEventFilter(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	events, err := h.Observer.QueryHookEvents(c.Request.Context(), filter)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, contract.HookEventsResponse{Events: HookEventPayloadsFromDescriptors(events)})
}

// ListLogs returns the filtered runtime log list.
func (h *BaseHandlers) ListLogs(c *gin.Context) {
	if h.Observer == nil {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: observer is required"))
		return
	}
	query, err := ParseLogsQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	events, err := h.Observer.QueryEvents(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	payload := make([]contract.LogEventPayload, 0, len(events))
	for _, event := range events {
		payload = append(payload, LogEventPayloadFromSummary(event))
	}

	c.JSON(http.StatusOK, contract.LogsListResponse{Events: payload})
}

func (h *BaseHandlers) networkStatusPayload(ctx context.Context) (*contract.NetworkStatusPayload, error) {
	if !h.Config.Network.Enabled {
		return &contract.NetworkStatusPayload{
			Enabled: false,
			Status:  memoryHealthStatusDisabled,
		}, nil
	}
	if h.Network == nil {
		return nil, errors.New("api: network service is required when network is enabled")
	}

	status, err := h.Network.Status(ctx)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, errors.New("api: network status is required")
	}

	payload := NetworkStatusPayloadFromStatus(status)
	if h.Bundles == nil {
		return payload, nil
	}

	settings, err := h.Bundles.NetworkSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("api: load bundle network settings: %w", err)
	}
	payload.DeclaredChannels = DeclaredNetworkChannelPayloads(settings.DeclaredChannels)
	return payload, nil
}

func (h *BaseHandlers) daemonUserHomeDir() string {
	userHomeDir, err := aghconfig.ResolveOperatorHomeDir(h.HomePaths)
	if err == nil {
		return userHomeDir
	}

	logger := h.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Warn("api: daemon status user home directory unavailable", "err", err)
	return ""
}
