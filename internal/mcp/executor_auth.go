package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	toolspkg "github.com/compozy/agh/internal/tools"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
)

const (
	executorBearerValue     = "Bearer"
	authStatusRefreshFailed = "refresh_failed"
)

func (e *CallExecutor) ensureAuthorized(ctx context.Context, resolved ResolvedServer) error {
	server := resolved.Server
	if server.EffectiveTransport() == aghconfig.MCPServerTransportStdio || !server.Auth.Enabled() {
		return nil
	}
	status, err := e.authStatus(ctx, resolved)
	if err != nil {
		return err
	}
	switch status.Status {
	case string(mcpauth.StatusAuthenticated):
		return nil
	case string(mcpauth.StatusExpired):
		refreshed, refreshErr := e.refreshAuth(ctx, resolved)
		if refreshErr != nil {
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeUnavailable,
				"",
				"mcp auth refresh failed",
				toolspkg.ErrToolUnavailable,
				toolspkg.ReasonMCPAuthRefreshFailed,
			)
		}
		if refreshed.Status == string(mcpauth.StatusAuthenticated) {
			return nil
		}
		reason, ok := toolspkg.MCPAuthStatusReason(refreshed)
		if !ok {
			reason = toolspkg.ReasonMCPAuthRefreshFailed
		}
		return unavailableAuthError(reason)
	default:
		reason, ok := toolspkg.MCPAuthStatusReason(status)
		if !ok {
			reason = toolspkg.ReasonMCPAuthRequired
		}
		if reason == toolspkg.ReasonMCPAuthUnconfigured {
			reason = toolspkg.ReasonMCPAuthRequired
		}
		return unavailableAuthError(reason)
	}
}

func (e *CallExecutor) authStatus(
	ctx context.Context,
	resolved ResolvedServer,
) (toolspkg.MCPAuthStatus, error) {
	server := resolved.Server
	cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, server, e.resolveSecretRef)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, fmt.Errorf("mcp: build auth config: %w", err)
	}
	if !server.Auth.Enabled() {
		return redactedAuthStatus(mcpauth.Status{
			ServerName:  server.Name,
			Scope:       resolved.Target.Scope,
			WorkspaceID: resolved.Target.WorkspaceID,
			Status:      mcpauth.StatusUnconfigured,
			RemoteURL:   server.URL,
		}), nil
	}
	if e.auth == nil {
		return toolspkg.MCPAuthStatus{}, toolspkg.NewValidationError(
			"auth",
			toolspkg.ReasonDependencyMissing,
			"mcp auth service is required for auth-enabled servers",
		)
	}
	status, err := e.auth.Status(ctx, cfg)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, fmt.Errorf("mcp: read auth status: %w", err)
	}
	return redactedAuthStatus(status), nil
}

func (e *CallExecutor) refreshAuth(
	ctx context.Context,
	resolved ResolvedServer,
) (toolspkg.MCPAuthStatus, error) {
	if e.auth == nil {
		return toolspkg.MCPAuthStatus{}, toolspkg.NewValidationError(
			"auth",
			toolspkg.ReasonDependencyMissing,
			"mcp auth service is required for refresh",
		)
	}
	cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, resolved.Server, e.resolveSecretRef)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, fmt.Errorf("mcp: build auth config: %w", err)
	}
	status, err := e.auth.Refresh(ctx, cfg)
	if err != nil {
		redacted := redactedAuthStatus(mcpauth.Status{
			ServerName:   cfg.Target.ServerName,
			Scope:        cfg.Target.Scope,
			WorkspaceID:  cfg.Target.WorkspaceID,
			Status:       mcpauth.StatusInvalid,
			AuthType:     cfg.Type,
			ClientID:     cfg.ClientID,
			Scopes:       cfg.Scopes,
			Diagnostic:   "refresh failed",
			TokenPresent: false,
		})
		redacted.Status = authStatusRefreshFailed
		return redacted, err
	}
	return redactedAuthStatus(status), nil
}

func (e *CallExecutor) authHeaderFunc(resolved ResolvedServer) mcptransport.HTTPHeaderFunc {
	return func(ctx context.Context) map[string]string {
		header := e.authorizationHeader(ctx, resolved)
		if header == "" {
			return nil
		}
		return map[string]string{"Authorization": header}
	}
}

func (e *CallExecutor) authorizationHeader(ctx context.Context, resolved ResolvedServer) string {
	if e == nil || e.tokenStore == nil {
		return ""
	}
	token, err := e.tokenStore.GetMCPAuthToken(ctx, resolved.Target)
	if err != nil || strings.TrimSpace(token.AccessToken) == "" {
		return ""
	}
	cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, resolved.Server, nil)
	if err != nil {
		return ""
	}
	fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
	if err != nil || strings.TrimSpace(token.DefinitionFingerprint) != fingerprint {
		return ""
	}
	tokenType := strings.TrimSpace(token.TokenType)
	if tokenType == "" {
		tokenType = executorBearerValue
	}
	if !strings.EqualFold(tokenType, executorBearerValue) {
		return ""
	}
	diagnostics.RegisterDynamicSecret(token.AccessToken)
	return "Bearer " + strings.TrimSpace(token.AccessToken)
}

func unavailableAuthError(reason toolspkg.ReasonCode) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		"",
		"mcp authentication is unavailable",
		toolspkg.ErrToolUnavailable,
		reason,
	)
}

func redactedAuthStatus(status mcpauth.Status) toolspkg.MCPAuthStatus {
	var expiresAt *time.Time
	if status.ExpiresAt != nil {
		cloned := status.ExpiresAt.UTC()
		expiresAt = &cloned
	}
	return toolspkg.MCPAuthStatus{
		ServerName:   strings.TrimSpace(status.ServerName),
		Status:       strings.TrimSpace(string(status.Status)),
		AuthType:     strings.TrimSpace(status.AuthType),
		ClientID:     strings.TrimSpace(status.ClientID),
		Scopes:       trimStrings(status.Scopes),
		ExpiresAt:    expiresAt,
		Refreshable:  status.Refreshable,
		TokenPresent: status.TokenPresent,
		Diagnostic:   strings.TrimSpace(status.Diagnostic),
	}
}
