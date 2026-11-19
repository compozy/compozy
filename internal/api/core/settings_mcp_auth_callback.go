package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/mcpoauthpage"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/gin-gonic/gin"
)

// unknownMCPOAuthServer names the server in a diagnostic when the callback no longer resolves.
const unknownMCPOAuthServer = "unknown"

// oauthErrorCode is the RFC 6749 error-code alphabet; anything else from the provider is not shown.
var oauthErrorCode = regexp.MustCompile(`^[a-z0-9_.-]{1,64}$`)

// CompleteSettingsMCPAuthCallback completes a browser redirect and renders the outcome page.
func (h *BaseHandlers) CompleteSettingsMCPAuthCallback(c *gin.Context) {
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if h.Settings == nil {
		h.renderMCPOAuthPage(c, http.StatusServiceUnavailable, mcpoauthpage.Page{
			Outcome: mcpoauthpage.OutcomeUnavailable,
			Code:    "settings_unavailable",
			Now:     now,
		})
		return
	}
	callbackURL, err := h.mcpOAuthCallbackURL()
	if err != nil {
		h.renderMCPOAuthFailure(c, http.StatusServiceUnavailable, mcpoauthpage.Page{
			Outcome: mcpoauthpage.OutcomeFailed,
			Code:    "callback_unavailable",
			Now:     now,
		})
		return
	}
	if c.Request.URL.RawQuery != "" {
		callbackURL += "?" + c.Request.URL.RawQuery
	}
	status, err := h.Settings.CompleteMCPAuthCallback(c.Request.Context(), callbackURL)
	if err != nil {
		code, page := mcpOAuthFailurePage(err, c.Query("error"))
		page.Now = now
		h.renderMCPOAuthFailure(c, code, page)
		return
	}
	if status.Status != mcpauth.StatusAuthenticated || !status.TokenPresent {
		h.renderMCPOAuthFailure(c, http.StatusBadRequest, mcpoauthpage.Page{
			Outcome: mcpoauthpage.OutcomeFailed,
			Server:  &mcpoauthpage.Server{Name: status.ServerName},
			Code:    "token_missing",
			Now:     now,
		})
		return
	}
	h.renderMCPOAuthPage(c, http.StatusOK, mcpoauthpage.Page{
		Outcome:     mcpoauthpage.OutcomeConnected,
		Server:      &mcpoauthpage.Server{Name: status.ServerName, Host: remoteHost(status.RemoteURL)},
		Scope:       status.Scope,
		ProjectName: h.mcpOAuthProjectName(c.Request.Context(), status),
		Scopes:      status.Scopes,
		Refreshable: status.Refreshable,
		ExpiresAt:   status.ExpiresAt,
		Now:         now,
	})
}

// mcpOAuthFailurePage maps a callback error to the page a person can act on. A provider `error`
// redirect wins: whatever else failed afterwards, the provider is the reason nothing connected.
func mcpOAuthFailurePage(err error, providerError string) (int, mcpoauthpage.Page) {
	page := mcpoauthpage.Page{Outcome: mcpoauthpage.OutcomeFailed}
	if callbackErr, ok := errors.AsType[*settingspkg.MCPAuthCallbackError](err); ok &&
		strings.TrimSpace(callbackErr.Target.ServerName) != "" {
		page.Server = &mcpoauthpage.Server{Name: callbackErr.Target.ServerName}
	}
	switch providerError = strings.TrimSpace(providerError); {
	case providerError != "":
		page.Outcome = mcpoauthpage.OutcomeDeclined
		page.Code = "provider_error"
		if oauthErrorCode.MatchString(providerError) {
			page.Code = providerError
		}
		return http.StatusBadRequest, page
	case errors.Is(err, mcpauth.ErrLoginSessionExpired):
		page.Outcome, page.Code = mcpoauthpage.OutcomeExpired, "login_session_expired"
		return http.StatusBadRequest, page
	case errors.Is(err, mcpauth.ErrLoginSessionNotFound):
		page.Outcome, page.Code = mcpoauthpage.OutcomeExpired, "login_session_not_found"
		return http.StatusBadRequest, page
	case errors.Is(err, mcpauth.ErrLoginSessionStale):
		page.Outcome, page.Code = mcpoauthpage.OutcomeChanged, "login_session_stale"
		return http.StatusBadRequest, page
	case StatusForSettingsError(err) == http.StatusServiceUnavailable:
		page.Outcome, page.Code = mcpoauthpage.OutcomeUnavailable, "settings_unavailable"
		return http.StatusServiceUnavailable, page
	}
	if errors.Is(err, mcpauth.ErrInvalidExchange) {
		page.Code = "invalid_exchange"
	}
	return http.StatusBadRequest, page
}

// renderMCPOAuthFailure logs and renders a failure. Neither the log nor the page carries the
// error text: token-endpoint errors can echo the authorization code or other credentials, so
// both state only the classified reason, the server and the HTTP status.
func (h *BaseHandlers) renderMCPOAuthFailure(c *gin.Context, code int, page mcpoauthpage.Page) {
	reason, server := mcpOAuthReason(page), mcpOAuthServer(page)
	if h.Logger != nil {
		h.Logger.WarnContext(
			c.Request.Context(),
			"MCP OAuth callback failed",
			"outcome", string(page.Outcome),
			"reason", reason,
			"server", server,
			"http_status", code,
		)
	}
	if page.Outcome == mcpoauthpage.OutcomeFailed {
		page.Diagnostic = fmt.Sprintf(
			"reason: %s\nserver: %s\nhttp_status: %d\ntime: %s",
			reason, server, code, page.Now.Format(time.RFC3339),
		)
	}
	h.renderMCPOAuthPage(c, code, page)
}

func mcpOAuthReason(page mcpoauthpage.Page) string {
	if page.Code == "" {
		return "unclassified"
	}
	return page.Code
}

func mcpOAuthServer(page mcpoauthpage.Page) string {
	if page.Server == nil || strings.TrimSpace(page.Server.Name) == "" {
		return unknownMCPOAuthServer
	}
	return page.Server.Name
}

func (h *BaseHandlers) renderMCPOAuthPage(c *gin.Context, code int, page mcpoauthpage.Page) {
	var body bytes.Buffer
	if err := mcpoauthpage.Render(&body, page); err != nil {
		if h.Logger != nil {
			h.Logger.Error("render MCP OAuth callback page", "error", err)
		}
		c.Data(
			http.StatusInternalServerError,
			"text/plain; charset=utf-8",
			[]byte("MCP sign-in page failed to render."),
		)
		return
	}
	c.Header("Content-Security-Policy", mcpoauthpage.ContentSecurityPolicy())
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Data(code, "text/html; charset=utf-8", body.Bytes())
}

// mcpOAuthProjectName names the project a workspace-scoped server belongs to; empty when unknown.
func (h *BaseHandlers) mcpOAuthProjectName(ctx context.Context, status mcpauth.Status) string {
	if h.Workspaces == nil || strings.TrimSpace(status.WorkspaceID) == "" {
		return ""
	}
	workspace, err := h.Workspaces.Get(ctx, status.WorkspaceID)
	if err != nil {
		return ""
	}
	return workspace.Name
}

func remoteHost(remoteURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(remoteURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}
