package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// InstallSettingsMCPServerRequest is the UDS-backed catalog install request.
type InstallSettingsMCPServerRequest = contract.InstallSettingsMCPServerRequest

// InstallSettingsMCPServerRecord is the UDS-backed catalog install response.
type InstallSettingsMCPServerRecord = contract.InstallSettingsMCPServerResponse

// SettingsMCPAuthTarget selects one exact daemon-owned MCP credential set.
type SettingsMCPAuthTarget struct {
	Name        string
	Scope       contract.SettingsWorkspaceScopeKind
	WorkspaceID string
}

// SettingsMCPAuthBeginRecord is the verifier-free daemon OAuth handoff.
type SettingsMCPAuthBeginRecord = contract.SettingsMCPAuthBeginResponse

// SettingsMCPAuthBeginRequest selects automatic or manual callback delivery.
type SettingsMCPAuthBeginRequest = contract.SettingsMCPAuthBeginRequest

// SettingsMCPAuthExchangeRequest is the secret-bearing one-time exchange body.
type SettingsMCPAuthExchangeRequest = contract.SettingsMCPAuthExchangeRequest

// SettingsMCPAuthStatusRecord is the redacted daemon OAuth status.
type SettingsMCPAuthStatusRecord = contract.SettingsMCPAuthStatusPayload

// MCPSettingsClient exposes settings-owned MCP catalog installation.
type MCPSettingsClient interface {
	InstallSettingsMCPServer(
		ctx context.Context,
		request InstallSettingsMCPServerRequest,
	) (InstallSettingsMCPServerRecord, error)
	ListSettingsMCPServers(
		ctx context.Context,
		scope contract.SettingsWorkspaceScopeKind,
		workspaceID string,
	) (contract.SettingsMCPServersResponse, error)
	BeginSettingsMCPAuth(
		ctx context.Context,
		target SettingsMCPAuthTarget,
		request SettingsMCPAuthBeginRequest,
	) (SettingsMCPAuthBeginRecord, error)
	ExchangeSettingsMCPAuth(
		ctx context.Context,
		target SettingsMCPAuthTarget,
		request SettingsMCPAuthExchangeRequest,
	) (SettingsMCPAuthStatusRecord, error)
	LogoutSettingsMCPAuth(ctx context.Context, target SettingsMCPAuthTarget) (SettingsMCPAuthStatusRecord, error)
}

func (c *unixSocketClient) InstallSettingsMCPServer(
	ctx context.Context,
	request InstallSettingsMCPServerRequest,
) (InstallSettingsMCPServerRecord, error) {
	var response InstallSettingsMCPServerRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/settings/mcp-servers/install",
		nil,
		request,
		&response,
	); err != nil {
		return InstallSettingsMCPServerRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListSettingsMCPServers(
	ctx context.Context,
	scope contract.SettingsWorkspaceScopeKind,
	workspaceID string,
) (contract.SettingsMCPServersResponse, error) {
	query := url.Values{}
	query.Set("scope", strings.TrimSpace(string(scope)))
	if normalized := strings.TrimSpace(workspaceID); normalized != "" {
		query.Set("workspace_id", normalized)
	}
	var response contract.SettingsMCPServersResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/mcp-servers", query, nil, &response); err != nil {
		return contract.SettingsMCPServersResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) BeginSettingsMCPAuth(
	ctx context.Context,
	target SettingsMCPAuthTarget,
	request SettingsMCPAuthBeginRequest,
) (SettingsMCPAuthBeginRecord, error) {
	var response SettingsMCPAuthBeginRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		settingsMCPAuthPath(target, "begin"),
		settingsMCPAuthQuery(target),
		request,
		&response,
	); err != nil {
		return SettingsMCPAuthBeginRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ExchangeSettingsMCPAuth(
	ctx context.Context,
	target SettingsMCPAuthTarget,
	request SettingsMCPAuthExchangeRequest,
) (SettingsMCPAuthStatusRecord, error) {
	var response SettingsMCPAuthStatusRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		settingsMCPAuthPath(target, "exchange"),
		settingsMCPAuthQuery(target),
		request,
		&response,
	); err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) LogoutSettingsMCPAuth(
	ctx context.Context,
	target SettingsMCPAuthTarget,
) (SettingsMCPAuthStatusRecord, error) {
	var response SettingsMCPAuthStatusRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		settingsMCPAuthPath(target, "logout"),
		settingsMCPAuthQuery(target),
		nil,
		&response,
	); err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	return response, nil
}

func settingsMCPAuthPath(target SettingsMCPAuthTarget, action string) string {
	return "/api/settings/mcp-servers/" + url.PathEscape(strings.TrimSpace(target.Name)) +
		"/auth/" + strings.TrimSpace(action)
}

func settingsMCPAuthQuery(target SettingsMCPAuthTarget) url.Values {
	query := url.Values{}
	query.Set("scope", strings.TrimSpace(string(target.Scope)))
	if workspaceID := strings.TrimSpace(target.WorkspaceID); workspaceID != "" {
		query.Set("workspace_id", workspaceID)
	}
	return query
}
