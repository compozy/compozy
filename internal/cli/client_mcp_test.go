package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/compozy/agh/internal/api/contract"
)

func TestUnixSocketClientMCPAuthRoutesCarryExactWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should carry exact workspace identity through the MCP auth lifecycle", func(t *testing.T) {
		t.Parallel()

		requestIndex := 0
		client := &unixSocketClient{
			socketPath: "/tmp/agh.sock",
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestIndex++
				if got, want := req.URL.Query().Get("scope"), "workspace"; got != want {
					return nil, fmt.Errorf("request %d scope = %q, want %q", requestIndex, got, want)
				}
				if got, want := req.URL.Query().Get("workspace_id"), "workspace-a"; got != want {
					return nil, fmt.Errorf("request %d workspace_id = %q, want %q", requestIndex, got, want)
				}
				switch requestIndex {
				case 1:
					if req.Method != http.MethodGet || req.URL.Path != "/api/settings/mcp-servers" {
						return nil, fmt.Errorf("list request = %s %s", req.Method, req.URL.Path)
					}
					return newHTTPResponse(http.StatusOK, `{"mcp_servers":[]}`), nil
				case 2:
					if req.Method != http.MethodPost ||
						req.URL.Path != "/api/settings/mcp-servers/linear/auth/begin" {
						return nil, fmt.Errorf("begin request = %s %s", req.Method, req.URL.Path)
					}
					body, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, fmt.Errorf("read begin body: %w", err)
					}
					var begin SettingsMCPAuthBeginRequest
					if err := json.Unmarshal(body, &begin); err != nil {
						return nil, fmt.Errorf("decode begin body: %w", err)
					}
					if begin.Mode != contract.SettingsMCPAuthBeginModeAutomatic {
						return nil, fmt.Errorf("begin mode = %q, want automatic", begin.Mode)
					}
					return newHTTPResponse(http.StatusOK, `{
                  "authorization_url":"https://auth.example/authorize?state=public",
                  "state":"public",
                  "expires_at":"2026-07-13T16:05:00Z",
                  "callback_url":"http://127.0.0.1:2123/api/mcp/oauth/callback",
                  "manual_supported":true
                }`), nil
				case 3:
					if req.Method != http.MethodPost ||
						req.URL.Path != "/api/settings/mcp-servers/linear/auth/exchange" {
						return nil, fmt.Errorf("exchange request = %s %s", req.Method, req.URL.Path)
					}
					body, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, fmt.Errorf("read exchange body: %w", err)
					}
					var exchange SettingsMCPAuthExchangeRequest
					if err := json.Unmarshal(body, &exchange); err != nil {
						return nil, fmt.Errorf("decode exchange body: %w", err)
					}
					if exchange.Code != "sensitive-code" || exchange.RedirectURL != "" {
						return nil, fmt.Errorf("exchange input classification is incorrect")
					}
					return newHTTPResponse(http.StatusOK, confirmedClientMCPAuthStatusJSON), nil
				case 4:
					if req.Method != http.MethodPost ||
						req.URL.Path != "/api/settings/mcp-servers/linear/auth/logout" {
						return nil, fmt.Errorf("logout request = %s %s", req.Method, req.URL.Path)
					}
					return newHTTPResponse(http.StatusOK, loggedOutClientMCPAuthStatusJSON), nil
				default:
					return nil, fmt.Errorf(
						"unexpected MCP auth request %d: %s %s",
						requestIndex,
						req.Method,
						req.URL.Path,
					)
				}
			})},
		}
		ctx := context.Background()
		scope := contract.SettingsWorkspaceScopeWorkspace
		target := SettingsMCPAuthTarget{Name: "linear", Scope: scope, WorkspaceID: "workspace-a"}
		if _, err := client.ListSettingsMCPServers(ctx, scope, "workspace-a"); err != nil {
			t.Fatalf("ListSettingsMCPServers() error = %v", err)
		}
		begin, err := client.BeginSettingsMCPAuth(ctx, target, SettingsMCPAuthBeginRequest{
			Mode: contract.SettingsMCPAuthBeginModeAutomatic,
		})
		if err != nil {
			t.Fatalf("BeginSettingsMCPAuth() error = %v", err)
		}
		if begin.State != "public" || !begin.ManualSupported {
			t.Fatalf("BeginSettingsMCPAuth() = %#v", begin)
		}
		status, err := client.ExchangeSettingsMCPAuth(
			ctx,
			target,
			SettingsMCPAuthExchangeRequest{Code: "sensitive-code"},
		)
		if err != nil {
			t.Fatalf("ExchangeSettingsMCPAuth() error = %v", err)
		}
		if !status.TokenPresent || status.WorkspaceID != "workspace-a" {
			t.Fatalf("ExchangeSettingsMCPAuth() = %#v", status)
		}
		loggedOut, err := client.LogoutSettingsMCPAuth(ctx, target)
		if err != nil {
			t.Fatalf("LogoutSettingsMCPAuth() error = %v", err)
		}
		if loggedOut.TokenPresent || loggedOut.Status != "needs_login" {
			t.Fatalf("LogoutSettingsMCPAuth() = %#v", loggedOut)
		}
		if requestIndex != 4 {
			t.Fatalf("request count = %d, want 4", requestIndex)
		}
	})
}

const confirmedClientMCPAuthStatusJSON = `{
  "server_name":"linear",
  "scope":"workspace",
  "workspace_id":"workspace-a",
  "status":"authenticated",
  "refreshable":true,
  "token_present":true
}`

const loggedOutClientMCPAuthStatusJSON = `{
  "server_name":"linear",
  "scope":"workspace",
  "workspace_id":"workspace-a",
  "status":"needs_login",
  "refreshable":false,
  "token_present":false
}`
