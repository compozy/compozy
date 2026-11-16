package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) CheckSessionMCP(c *gin.Context) {
	_, id, _, ok := h.routeScopedSessionInWorkspace(c)
	if !ok {
		return
	}
	checker, ok := h.Sessions.(session.MCPChecker)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("session MCP check unavailable"))
		return
	}
	result, err := checker.CheckSessionMCP(
		c.Request.Context(),
		id,
		strings.TrimSpace(c.Param("name")),
		strings.TrimSpace(c.Query("owner")),
	)
	if err != nil {
		status := statusForMCPCheckError(err, StatusForSessionError)
		if errors.Is(err, settingspkg.ErrNotFound) {
			status = http.StatusNotFound
		}
		h.respondError(c, status, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

type MCPSettingsChecker interface {
	CheckMCPServer(context.Context, settingspkg.MCPAuthTargetRequest) (settingspkg.MCPServerItem, error)
}

func (h *BaseHandlers) CheckSettingsMCPServer(c *gin.Context) {
	checker, ok := h.Settings.(MCPSettingsChecker)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	target, err := parseSettingsMCPAuthTarget(c)
	if err != nil {
		h.respondError(c, statusForMCPCheckError(err, StatusForSettingsError), err)
		return
	}
	result, err := checker.CheckMCPServer(c.Request.Context(), target)
	if err != nil {
		h.respondError(c, statusForMCPCheckError(err, StatusForSettingsError), err)
		return
	}
	c.JSON(http.StatusOK, settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{result})[0])
}

func statusForMCPCheckError(err error, fallback func(error) int) int {
	reason, _ := toolspkg.ReasonOf(err)
	switch {
	case reason == toolspkg.ReasonSourceDisabled:
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled), errors.Is(err, toolspkg.ErrToolCanceled):
		return 499
	case errors.Is(err, settingspkg.ErrUnavailable), errors.Is(err, toolspkg.ErrToolUnavailable):
		return http.StatusServiceUnavailable
	default:
		return fallback(err)
	}
}
