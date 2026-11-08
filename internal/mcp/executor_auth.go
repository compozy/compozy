package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	executorBearerValue     = "Bearer"
	authStatusRefreshFailed = "refresh_failed"
)

func (e *CallExecutor) ensureAuthorized(ctx context.Context, resolved ResolvedServer) error {
	server := resolved.Server
	if !mcpauth.OAuthCapable(server) {
		return nil
	}
	status, err := e.authStatus(ctx, resolved)
	if err != nil {
		return err
	}
	switch status.Status {
	case string(mcpauth.StatusUnconfigured):
		if server.Auth.IsZero() {
			return nil
		}
		return unavailableAuthError(toolspkg.ReasonMCPAuthRequired)
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
		reason = e.authorizationFailureReason(ctx, resolved, reason)
		return unavailableAuthError(reason)
	}
}

func (e *CallExecutor) authorizationFailureReason(
	ctx context.Context,
	resolved ResolvedServer,
	fallback toolspkg.ReasonCode,
) toolspkg.ReasonCode {
	if e == nil || e.auth == nil || fallback != toolspkg.ReasonMCPAuthRequired {
		return fallback
	}
	cfg, err := e.serverAuthConfig(ctx, resolved, nil)
	if err != nil {
		return fallback
	}
	_, err = e.auth.AuthorizationToken(ctx, cfg)
	switch {
	case errors.Is(err, mcpauth.ErrTokenBindingInvalid):
		return toolspkg.ReasonMCPAuthInvalid
	case errors.Is(err, mcpauth.ErrTokenExpired):
		return toolspkg.ReasonMCPAuthExpired
	default:
		return fallback
	}
}

func (e *CallExecutor) authStatus(
	ctx context.Context,
	resolved ResolvedServer,
) (toolspkg.MCPAuthStatus, error) {
	server := resolved.Server
	cfg, err := e.serverAuthConfig(ctx, ResolvedServer{Server: server, Target: resolved.Target}, e.resolveSecretRef)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, fmt.Errorf("mcp: build auth config: %w", err)
	}
	unconfigured := mcpauth.Status{
		Owner: resolved.Target.Owner, ServerName: server.Name, Scope: resolved.Target.Scope,
		WorkspaceID: resolved.Target.WorkspaceID, Status: mcpauth.StatusUnconfigured, RemoteURL: server.URL,
	}
	if !mcpauth.OAuthCapable(server) || (server.Auth.IsZero() && e.auth == nil) {
		return redactedAuthStatus(unconfigured), nil
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
		if cfg.Implicit {
			return redactedAuthStatus(unconfigured), nil
		}
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
	cfg, err := e.serverAuthConfig(ctx, resolved, e.resolveSecretRef)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, fmt.Errorf("mcp: build auth config: %w", err)
	}
	status, err := e.auth.Refresh(ctx, cfg)
	if err != nil {
		redacted := redactedAuthStatus(mcpauth.Status{
			Owner:        cfg.Target.Owner,
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

func (e *CallExecutor) authorizationHeader(ctx context.Context, resolved ResolvedServer) string {
	if e == nil || e.auth == nil {
		return ""
	}
	cfg, err := e.serverAuthConfig(ctx, resolved, nil)
	if err != nil {
		return ""
	}
	token, err := e.auth.AuthorizationToken(ctx, cfg)
	if err != nil || strings.TrimSpace(token.AccessToken) == "" {
		return ""
	}
	tokenType := strings.TrimSpace(token.TokenType)
	if tokenType == "" {
		tokenType = executorBearerValue
	}
	if !strings.EqualFold(tokenType, executorBearerValue) {
		return ""
	}
	registerMCPRequestSecret(ctx, token.AccessToken)
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
		expiresAt = new(status.ExpiresAt.UTC())
	}
	return toolspkg.MCPAuthStatus{
		Owner: status.Owner, Scope: string(status.Scope), WorkspaceID: status.WorkspaceID,
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

func (e *CallExecutor) serverAuthConfig(
	ctx context.Context,
	resolved ResolvedServer,
	resolve func(context.Context, string) (string, error),
) (mcpauth.ServerConfig, error) {
	server, secrets, err := resolved.Server.RenderInputs(
		func(ref string) (string, error) { return e.resolveSecretRef(ctx, ref) },
	)
	if err != nil {
		return mcpauth.ServerConfig{}, err
	}
	for _, secret := range secrets {
		registerMCPRequestSecret(ctx, secret)
	}
	return mcpauth.ServerConfigFromMCP(ctx, resolved.Target, server, resolve)
}
