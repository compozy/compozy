package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	executorBearerValue     = "Bearer"
	authStatusRefreshFailed = "refresh_failed"
)

func (e *CallExecutor) prepareAuthorization(ctx context.Context, resolved *ResolvedServer) (string, error) {
	if err := e.ensureAuthorized(ctx, resolved); err != nil {
		return "", err
	}
	return e.authorizationHeader(ctx, resolved)
}

func (e *CallExecutor) ensureAuthorized(ctx context.Context, resolved *ResolvedServer) error {
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
		return toolspkg.MCPAuthRequiredError("", server.Name)
	case string(mcpauth.StatusAuthenticated):
		return nil
	case string(mcpauth.StatusInvalid):
		return toolspkg.NewToolError(toolspkg.ErrorCodeUnavailable, "", status.Diagnostic,
			toolspkg.ErrToolUnavailable, toolspkg.ReasonMCPAuthInvalid)
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
			if e.authRecovery != nil {
				if err := e.authRecovery(ctx, resolved); err != nil {
					slog.WarnContext(ctx, "mcp.auth.recovery_failed", "server", server.Name)
				}
			}
			return nil
		}
		reason, ok := toolspkg.MCPAuthStatusReason(refreshed)
		if !ok {
			reason = toolspkg.ReasonMCPAuthRefreshFailed
		}
		return unavailableAuthError(reason)
	default:
		reason, ok := toolspkg.MCPAuthStatusReason(status)
		if !ok || reason == toolspkg.ReasonMCPAuthRequired || reason == toolspkg.ReasonMCPAuthUnconfigured {
			return e.authorizationRequired(ctx, resolved)
		}
		return unavailableAuthError(reason)
	}
}

func (e *CallExecutor) authorizationRequired(ctx context.Context, resolved *ResolvedServer) error {
	if e.auth == nil {
		return toolspkg.MCPAuthRequiredError("", resolved.Server.Name)
	}
	cfg, err := e.serverAuthConfig(ctx, resolved, nil)
	if err != nil {
		return authorizationConfigError(err)
	}
	_, err = e.auth.AuthorizationToken(ctx, cfg)
	if err != nil && !errors.Is(err, mcpauth.ErrTokenNotFound) {
		return authorizationLookupError(err)
	}
	return toolspkg.MCPAuthRequiredError("", resolved.Server.Name)
}

func (e *CallExecutor) authStatus(
	ctx context.Context,
	resolved *ResolvedServer,
) (toolspkg.MCPAuthStatus, error) {
	server := resolved.Server
	cfg, err := e.serverAuthConfig(ctx, resolved, e.resolveSecretRef)
	if err != nil {
		return toolspkg.MCPAuthStatus{}, authorizationConfigError(err)
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
		if cfg.Implicit && canProbeWithoutStoredCredential(err) {
			return redactedAuthStatus(unconfigured), nil
		}
		return toolspkg.MCPAuthStatus{}, authorizationLookupError(err)
	}
	return redactedAuthStatus(status), nil
}

func (e *CallExecutor) refreshAuth(
	ctx context.Context,
	resolved *ResolvedServer,
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

func (e *CallExecutor) authorizationHeader(ctx context.Context, resolved *ResolvedServer) (string, error) {
	if e == nil || e.auth == nil || !mcpauth.OAuthCapable(resolved.Server) {
		return "", nil
	}
	cfg, err := e.serverAuthConfig(ctx, resolved, nil)
	if err != nil {
		return "", authorizationConfigError(err)
	}
	if e.authGeneration != nil {
		resolved.authRevision, err = e.authGeneration.Revision(cfg.Target)
		if err != nil {
			return "", authorizationLookupError(err)
		}
	}
	token, err := e.auth.AuthorizationToken(ctx, cfg)
	switch {
	case cfg.Implicit && errors.Is(err, mcpauth.ErrTokenDefinitionChanged):
		return "", nil
	case err != nil && cfg.Implicit && canProbeWithoutStoredCredential(err):
		return "", nil
	case errors.Is(err, mcpauth.ErrTokenNotFound):
		return "", toolspkg.MCPAuthRequiredError("", resolved.Server.Name)
	case err != nil:
		return "", authorizationLookupError(err)
	}
	access := strings.TrimSpace(token.AccessToken)
	tokenType := strings.TrimSpace(token.TokenType)
	if access == "" || (tokenType != "" && !strings.EqualFold(tokenType, executorBearerValue)) {
		return "", authorizationLookupError(mcpauth.ErrTokenBindingInvalid)
	}
	registerMCPRequestSecret(ctx, access)
	registerMCPRequestSecret(ctx, token.RefreshToken)
	return "Bearer " + access, nil
}

func canProbeWithoutStoredCredential(err error) bool {
	return !errors.Is(err, mcpauth.ErrTokenResolution) && !errors.Is(err, mcpauth.ErrTokenBindingInvalid) &&
		!errors.Is(err, mcpauth.ErrTokenRejected) && !errors.Is(err, mcpauth.ErrTokenExpired) &&
		!errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func authorizationConfigError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return normalizeMCPError("", err)
	}
	return toolspkg.NewToolError(toolspkg.ErrorCodeUnavailable, "",
		"mcp: could not resolve OAuth configuration; check the configured credential references",
		toolspkg.ErrToolUnavailable, toolspkg.ReasonMCPAuthInvalid)
}

func authorizationLookupError(err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return normalizeMCPError("", err)
	case errors.Is(err, mcpauth.ErrTokenRejected):
		return rejectedBearerError("")
	case errors.Is(err, mcpauth.ErrTokenExpired):
		return unavailableAuthError(toolspkg.ReasonMCPAuthExpired)
	case errors.Is(err, mcpauth.ErrTokenBindingInvalid):
		return unavailableAuthError(toolspkg.ReasonMCPAuthInvalid)
	case errors.Is(err, mcpauth.ErrTokenResolution):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			"",
			"mcp: stored token could not be resolved; check the credential store",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonMCPAuthInvalid,
		)
	default:
		return toolspkg.NewToolError(toolspkg.ErrorCodeUnavailable, "",
			"mcp: token store unavailable; check the credential store",
			toolspkg.ErrToolUnavailable, toolspkg.ReasonMCPAuthInvalid)
	}
}

func rejectedBearerError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		id,
		mcpauth.ErrTokenRejected.Error()+"; check auth status",
		mcpauth.ErrTokenRejected,
		toolspkg.ReasonMCPAuthRequired,
	)
}

func (e *CallExecutor) recordBearerRejection(ctx context.Context, resolved *ResolvedServer, err error) {
	if e.authGeneration == nil || !errors.Is(err, mcpauth.ErrTokenRejected) {
		return
	}
	if err := e.authGeneration.RejectIfCurrent(resolved.Target, resolved.authRevision); err != nil {
		slog.WarnContext(ctx, "mcp.auth.record_rejection_failed", "server", resolved.Target.ServerName)
	}
}

func unavailableAuthError(reason toolspkg.ReasonCode) error {
	message := "mcp authentication is unavailable"
	if reason == toolspkg.ReasonMCPAuthInvalid {
		message = "mcp: stored token does not match the server definition; sign in again"
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeUnavailable,
		"",
		message,
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
	resolved *ResolvedServer,
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
	server.Headers = resolved.Server.Headers
	cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, server, resolve)
	cfg.ConsentScope = resolved.ConsentScope
	if cfg.ConsentScope == (mcpauth.ConsentScope{}) {
		cfg.ConsentScope = mcpauth.ConsentScopeFromContext(ctx)
	}
	if err != nil {
		return mcpauth.ServerConfig{}, err
	}
	cfg.DefinitionDigest, err = compozyconfig.MCPDefinitionDigest(resolved.Server, e.digestKey)
	return cfg, err
}
