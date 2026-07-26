package core

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

const (
	mcpOAuthCallbackPath      = "/api/mcp/oauth/callback"
	defaultManualCallbackPort = 2123
)

var errInvalidMCPAuthBeginMode = errors.New("settings: MCP OAuth begin mode must be automatic or manual")

func settingsMCPAuthStatusPayload(value *settingspkg.MCPAuthStatus) *contract.SettingsMCPAuthStatusPayload {
	if value == nil {
		return nil
	}
	return &contract.SettingsMCPAuthStatusPayload{
		ServerName:       strings.TrimSpace(value.ServerName),
		Scope:            strings.TrimSpace(string(value.Scope)),
		WorkspaceID:      strings.TrimSpace(value.WorkspaceID),
		Status:           strings.TrimSpace(string(value.Status)),
		RemoteURL:        strings.TrimSpace(value.RemoteURL),
		AuthType:         strings.TrimSpace(value.AuthType),
		ClientID:         strings.TrimSpace(value.ClientID),
		Issuer:           strings.TrimSpace(value.Issuer),
		Scopes:           cloneStrings(value.Scopes),
		ExpiresAt:        cloneTimePtr(value.ExpiresAt),
		UpdatedAt:        cloneTimePtr(value.UpdatedAt),
		Refreshable:      value.Refreshable,
		TokenPresent:     value.TokenPresent,
		RevocationURL:    strings.TrimSpace(value.RevocationURL),
		Diagnostic:       strings.TrimSpace(value.Diagnostic),
		AuthorizationURL: strings.TrimSpace(value.AuthorizationURL),
	}
}

// MCPSettingsAuth exposes daemon-owned MCP OAuth operations to API transports.
type MCPSettingsAuth interface {
	BeginMCPAuth(context.Context, settingspkg.MCPAuthBeginRequest) (mcpauth.BeginResult, error)
	ExchangeMCPAuth(context.Context, settingspkg.MCPAuthExchangeRequest) (mcpauth.Status, error)
	CompleteMCPAuthCallback(context.Context, string) (mcpauth.Status, error)
	LogoutMCPAuth(context.Context, settingspkg.MCPAuthTargetRequest) (mcpauth.Status, error)
}

// BeginSettingsMCPAuth creates one daemon-owned PKCE session.
func (h *BaseHandlers) BeginSettingsMCPAuth(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	target, err := parseSettingsMCPAuthTarget(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	var body contract.SettingsMCPAuthBeginRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, NewSettingsValidationError(
			errors.New("decode MCP auth begin request"),
		))
		return
	}
	callbackURL, err := h.mcpOAuthCallbackURLForMode(body.Mode)
	if err != nil {
		if errors.Is(err, errInvalidMCPAuthBeginMode) {
			h.respondError(c, http.StatusBadRequest, NewSettingsValidationError(err))
			return
		}
		h.respondError(c, http.StatusServiceUnavailable, err)
		return
	}
	result, err := h.Settings.BeginMCPAuth(c.Request.Context(), settingspkg.MCPAuthBeginRequest{
		MCPAuthTargetRequest: target,
		CallbackURL:          callbackURL,
	})
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.SettingsMCPAuthBeginResponse{
		AuthorizationURL: result.AuthorizationURL,
		State:            result.State,
		ExpiresAt:        result.ExpiresAt,
		CallbackURL:      result.CallbackURL,
		ManualSupported:  result.ManualSupported,
	})
}

func (h *BaseHandlers) mcpOAuthCallbackURLForMode(mode contract.SettingsMCPAuthBeginMode) (string, error) {
	switch mode {
	case contract.SettingsMCPAuthBeginModeAutomatic:
		return h.mcpOAuthCallbackURL()
	case contract.SettingsMCPAuthBeginModeManual:
		port := h.HTTPPortValue()
		if port <= 0 {
			port = h.Config.HTTP.Port
		}
		if port <= 0 {
			port = defaultManualCallbackPort
		}
		return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + mcpOAuthCallbackPath, nil
	default:
		return "", errInvalidMCPAuthBeginMode
	}
}

// ExchangeSettingsMCPAuth consumes one active PKCE session.
func (h *BaseHandlers) ExchangeSettingsMCPAuth(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	target, err := parseSettingsMCPAuthTarget(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	var body contract.SettingsMCPAuthExchangeRequest
	if err := decodeStrictJSONBody(c, &body); err != nil {
		h.respondError(c, http.StatusBadRequest, NewSettingsValidationError(
			errors.New("decode MCP auth exchange request"),
		))
		return
	}
	status, err := h.Settings.ExchangeMCPAuth(c.Request.Context(), settingspkg.MCPAuthExchangeRequest{
		MCPAuthTargetRequest: target,
		Code:                 body.Code,
		RedirectURL:          body.RedirectURL,
	})
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, settingsMCPAuthStatusPayload(&status))
}

// LogoutSettingsMCPAuth revokes and removes one scoped credential set.
func (h *BaseHandlers) LogoutSettingsMCPAuth(c *gin.Context) {
	if h.Settings == nil {
		h.respondError(c, http.StatusServiceUnavailable, errSettingsServiceUnavailable)
		return
	}
	target, err := parseSettingsMCPAuthTarget(c)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	status, err := h.Settings.LogoutMCPAuth(c.Request.Context(), target)
	if err != nil {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	c.JSON(http.StatusOK, settingsMCPAuthStatusPayload(&status))
}

// CompleteSettingsMCPAuthCallback completes a browser redirect and renders a close-tab page.
func (h *BaseHandlers) CompleteSettingsMCPAuthCallback(c *gin.Context) {
	if h.Settings == nil {
		c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", []byte(mcpOAuthFailurePage))
		return
	}
	callbackURL, err := h.mcpOAuthCallbackURL()
	if err != nil {
		c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", []byte(mcpOAuthFailurePage))
		return
	}
	if strings.TrimSpace(c.Request.URL.RawQuery) != "" {
		callbackURL += "?" + c.Request.URL.RawQuery
	}
	status, err := h.Settings.CompleteMCPAuthCallback(c.Request.Context(), callbackURL)
	if err != nil || status.Status != mcpauth.StatusAuthenticated || !status.TokenPresent {
		c.Data(http.StatusBadRequest, "text/html; charset=utf-8", []byte(mcpOAuthFailurePage))
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(mcpOAuthSuccessPage))
}

func parseSettingsMCPAuthTarget(c *gin.Context) (settingspkg.MCPAuthTargetRequest, error) {
	name := strings.TrimSpace(c.Param("name"))
	scope := strings.TrimSpace(c.Query("scope"))
	if name == "" {
		return settingspkg.MCPAuthTargetRequest{}, NewSettingsValidationError(
			errors.New("mcp auth server name is required"),
		)
	}
	if scope == "" {
		return settingspkg.MCPAuthTargetRequest{}, NewSettingsValidationError(
			errors.New("mcp auth scope query parameter is required"),
		)
	}
	return settingspkg.MCPAuthTargetRequest{
		Scope: settingspkg.ScopeKind(scope), WorkspaceID: strings.TrimSpace(c.Query("workspace_id")), Name: name,
	}, nil
}

func (h *BaseHandlers) mcpOAuthCallbackURL() (string, error) {
	port := h.HTTPPortValue()
	if port <= 0 {
		port = h.Config.HTTP.Port
	}
	if port <= 0 {
		return "", errors.New("settings: daemon HTTP port is unavailable for MCP OAuth callback")
	}
	host := canonicalMCPCallbackHost(h.Config.HTTP.Host)
	if !isLoopbackMCPCallbackHost(host) {
		return "", errors.New(
			"settings: automatic MCP OAuth callback requires an effective loopback HTTP listener; use manual exchange",
		)
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(port)) + mcpOAuthCallbackPath, nil
}

func canonicalMCPCallbackHost(value string) string {
	host := strings.TrimSpace(value)
	if host == "" {
		return "localhost"
	}
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return ""
		}
		host = parsed.Hostname()
	} else if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	return strings.Trim(strings.TrimSpace(host), "[]")
}

func isLoopbackMCPCallbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

const mcpOAuthSuccessPage = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Authorization complete</title></head><body><main><h1>Authorization complete</h1><p>You can close this tab and return to AGH.</p></main></body></html>`

const mcpOAuthFailurePage = `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>Authorization failed</title></head><body><main><h1>Authorization failed</h1><p>Return to AGH and try again, or use manual authorization.</p></main></body></html>`
