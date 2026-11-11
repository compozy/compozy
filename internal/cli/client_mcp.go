package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	mcppkg "github.com/compozy/compozy/internal/mcp"
)

// SettingsMCPAuthTarget selects one exact daemon-owned MCP credential set.
type SettingsMCPAuthTarget struct {
	Owner       string
	Name        string
	Scope       contract.SettingsLayeredScopeKind
	WorkspaceID string
	Profile     string
}

// SettingsMCPAuthBeginRecord is the verifier-free daemon OAuth handoff.
type SettingsMCPAuthBeginRecord = contract.SettingsMCPAuthBeginResponse

// SettingsMCPAuthBeginRequest selects automatic or manual callback delivery.
type SettingsMCPAuthBeginRequest = contract.SettingsMCPAuthBeginRequest

// SettingsMCPAuthExchangeRequest is the secret-bearing one-time exchange body.
type SettingsMCPAuthExchangeRequest = contract.SettingsMCPAuthExchangeRequest

// SettingsMCPAuthStatusRecord is the redacted daemon OAuth status.
type SettingsMCPAuthStatusRecord = contract.SettingsMCPAuthStatusPayload

// MCPSettingsClient exposes settings-owned MCP management.
type MCPSettingsClient interface {
	ListSettingsMCPServers(
		ctx context.Context,
		scope contract.SettingsLayeredScopeKind,
		workspaceID string,
		profile string,
	) (contract.SettingsMCPServersResponse, error)
	GetSettingsMCPAuthStatus(ctx context.Context, target SettingsMCPAuthTarget) (SettingsMCPAuthStatusRecord, error)
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

func (c *daemonClient) ListSettingsMCPServers(
	ctx context.Context,
	scope contract.SettingsLayeredScopeKind,
	workspaceID string,
	profile string,
) (contract.SettingsMCPServersResponse, error) {
	query := url.Values{}
	query.Set("scope", strings.TrimSpace(string(scope)))
	if normalized := strings.TrimSpace(workspaceID); normalized != "" {
		query.Set("workspace_id", normalized)
	}
	if normalized := strings.TrimSpace(profile); normalized != "" {
		query.Set(profileFlagName, normalized)
	}
	var response contract.SettingsMCPServersResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/settings/mcp-servers", query, nil, &response); err != nil {
		return contract.SettingsMCPServersResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) GetSettingsMCPAuthStatus(
	ctx context.Context,
	target SettingsMCPAuthTarget,
) (SettingsMCPAuthStatusRecord, error) {
	var response SettingsMCPAuthStatusRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		settingsMCPAuthPath(target, "status"),
		settingsMCPAuthQuery(target),
		nil,
		&response,
	); err != nil {
		return SettingsMCPAuthStatusRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) BeginSettingsMCPAuth(
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

func (c *daemonClient) ExchangeSettingsMCPAuth(
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

func (c *daemonClient) LogoutSettingsMCPAuth(
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
	if owner := strings.TrimSpace(target.Owner); owner != "" {
		query.Set("owner", owner)
	}
	query.Set("scope", strings.TrimSpace(string(target.Scope)))
	if workspaceID := strings.TrimSpace(target.WorkspaceID); workspaceID != "" {
		query.Set("workspace_id", workspaceID)
	}
	if profile := strings.TrimSpace(target.Profile); profile != "" {
		query.Set(profileFlagName, profile)
	}
	return query
}

func (c *daemonClient) BindHostedMCP(
	ctx context.Context,
	request mcppkg.HostedBindRequest,
) (mcppkg.HostedBindResponse, error) {
	var response mcppkg.HostedBindResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/bind", nil, request, &response); err != nil {
		return mcppkg.HostedBindResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) HostedMCPProjection(
	ctx context.Context,
	bindID string,
) (mcppkg.HostedProjectionResponse, error) {
	return c.hostedMCPProjection(ctx, bindID, false)
}

func (c *daemonClient) hostedMCPProjection(
	ctx context.Context,
	bindID string,
	streamSnapshot bool,
) (mcppkg.HostedProjectionResponse, error) {
	query := url.Values{}
	query.Set("bind_id", strings.TrimSpace(bindID))
	if streamSnapshot {
		query.Set("stream_snapshot", "true")
	}
	var response mcppkg.HostedProjectionResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/internal/hosted-mcp/projection", query, nil, &response); err != nil {
		return mcppkg.HostedProjectionResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) StreamHostedMCPProjection(
	ctx context.Context,
	bindID string,
	lastDigest string,
	handler mcppkg.HostedProjectionHandler,
) error {
	query := url.Values{}
	query.Set("bind_id", strings.TrimSpace(bindID))
	// Full catalogs can exceed the SSE frame limit; fetch JSON after each invalidation.
	query.Set("notifications_only", "true")
	if trimmed := strings.TrimSpace(lastDigest); trimmed != "" {
		query.Set("last_digest", trimmed)
	}
	err := c.doSSE(
		ctx,
		"/api/internal/hosted-mcp/projection/stream",
		query,
		"",
		func(event SSEEvent) error {
			if event.Event == clientErrorKey {
				return readAPIErrorBody(0, "", event.Data)
			}
			if event.Event == "projection_changed" {
				snapshot, err := c.hostedMCPProjection(ctx, bindID, true)
				if err != nil {
					return err
				}
				if handler == nil {
					return nil
				}
				return handler(snapshot)
			}
			if event.Event != "projection" {
				return nil
			}
			var snapshot mcppkg.HostedProjectionResponse
			if err := json.Unmarshal(event.Data, &snapshot); err != nil {
				return fmt.Errorf("cli: decode hosted MCP projection event: %w", err)
			}
			if handler == nil {
				return nil
			}
			return handler(snapshot)
		},
	)
	if apiErr, ok := errors.AsType[*daemonAPIError](err); ok {
		switch {
		case strings.HasPrefix(apiErr.payload.Error, "hosted_mcp_bind_not_found:"):
			return errors.Join(mcppkg.ErrHostedBindNotFound, err)
		case strings.HasPrefix(apiErr.payload.Error, "hosted_mcp_bind_required:"):
			return errors.Join(mcppkg.ErrHostedBindRequired, err)
		}
	}
	return err
}

func (c *daemonClient) CallHostedMCP(
	ctx context.Context,
	request mcppkg.HostedCallRequest,
) (mcppkg.HostedCallResponse, error) {
	var response mcppkg.HostedCallResponse
	if err := c.doJSONWithClient(
		ctx,
		http.MethodPost,
		"/api/internal/hosted-mcp/tools/call",
		nil,
		request,
		&response,
		c.streamHTTPClient(),
	); err != nil {
		return mcppkg.HostedCallResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ReleaseHostedMCP(
	ctx context.Context,
	request mcppkg.HostedReleaseRequest,
) error {
	return c.doJSON(ctx, http.MethodPost, "/api/internal/hosted-mcp/release", nil, request, nil)
}
