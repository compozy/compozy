package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
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
		status := StatusForSessionError(err)
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
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	result, err := checker.CheckMCPServer(c.Request.Context(), target)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, settingsMCPServerItemPayloads([]settingspkg.MCPServerItem{result})[0])
}
