package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestUnixSocketClientMCPAuthRoutesCarryExactWorkspaceIdentity(t *testing.T) {
	t.Parallel()
	// Invariant: all OAuth UDS requests preserve an explicit definition owner.
	// Owner: daemon CLI client; canonical suite: client_mcp_test.go.
	t.Run("Should retain the extension owner through every OAuth request", func(t *testing.T) {
		t.Parallel()
		calls := 0
		client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Query().Get("owner") != "extension:linear" || req.URL.Query().Get("scope") != "user" {
					return nil, fmt.Errorf("OAuth request lost its definition owner: %s", req.URL)
				}
				return newHTTPResponse(http.StatusOK, confirmedClientMCPAuthStatusJSON), nil
			})}}
		target := SettingsMCPAuthTarget{
			Name:  "linear",
			Owner: " extension:linear ",
			Scope: contract.SettingsLayeredScopeUser,
		}
		if _, err := client.GetSettingsMCPAuthStatus(t.Context(), target); err != nil {
			t.Fatal(err)
		}
		if _, err := client.BeginSettingsMCPAuth(t.Context(), target, SettingsMCPAuthBeginRequest{}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.ExchangeSettingsMCPAuth(t.Context(), target, SettingsMCPAuthExchangeRequest{}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.LogoutSettingsMCPAuth(t.Context(), target); err != nil {
			t.Fatal(err)
		}
		if calls != 4 {
			t.Fatalf("OAuth calls = %d, want 4", calls)
		}
	})

	t.Run("Should carry exact workspace identity through the MCP auth lifecycle", func(t *testing.T) {
		t.Parallel()

		requestIndex := 0
		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
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
					if req.Method != http.MethodGet ||
						req.URL.Path != "/api/settings/mcp-servers/linear/auth/status" {
						return nil, fmt.Errorf("status request = %s %s", req.Method, req.URL.Path)
					}
					return newHTTPResponse(http.StatusOK, confirmedClientMCPAuthStatusJSON), nil
				case 3:
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
				case 4:
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
					if exchange.RedirectURL != "http://127.0.0.1:2123/api/mcp/oauth/callback?code=sensitive-code&state=public" {
						return nil, fmt.Errorf("exchange input classification is incorrect")
					}
					return newHTTPResponse(http.StatusOK, confirmedClientMCPAuthStatusJSON), nil
				case 5:
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
		ctx := t.Context()
		scope := contract.SettingsLayeredScopeWorkspace
		target := SettingsMCPAuthTarget{Name: "linear", Scope: scope, WorkspaceID: "workspace-a"}
		if _, err := client.ListSettingsMCPServers(ctx, scope, "workspace-a", ""); err != nil {
			t.Fatalf("ListSettingsMCPServers() error = %v", err)
		}
		readStatus, err := client.GetSettingsMCPAuthStatus(ctx, target)
		if err != nil {
			t.Fatalf("GetSettingsMCPAuthStatus() error = %v", err)
		}
		if !readStatus.TokenPresent || readStatus.WorkspaceID != "workspace-a" {
			t.Fatalf("GetSettingsMCPAuthStatus() = %#v", readStatus)
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
			SettingsMCPAuthExchangeRequest{
				RedirectURL: "http://127.0.0.1:2123/api/mcp/oauth/callback" +
					"?code=sensitive-code&state=public",
			},
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
		if requestIndex != 5 {
			t.Fatalf("request count = %d, want 5", requestIndex)
		}
	})

	t.Run("Should carry exact profile identity through MCP reads", func(t *testing.T) {
		t.Parallel()

		requestIndex := 0
		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestIndex++
				if got := req.URL.Query().Get("scope"); got != "profile" {
					return nil, fmt.Errorf("profile request scope = %q", got)
				}
				if got := req.URL.Query().Get("profile"); got != "marketing" {
					return nil, fmt.Errorf("profile request profile = %q", got)
				}
				if got := req.URL.Query().Get("workspace_id"); got != "" {
					return nil, fmt.Errorf("profile request workspace_id = %q", got)
				}
				switch requestIndex {
				case 1:
					return newHTTPResponse(http.StatusOK, `{"mcp_servers":[]}`), nil
				case 2:
					return newHTTPResponse(http.StatusOK, confirmedClientMCPAuthStatusJSON), nil
				default:
					return nil, fmt.Errorf("unexpected profile request %d", requestIndex)
				}
			})},
		}
		if _, err := client.ListSettingsMCPServers(
			t.Context(), contract.SettingsLayeredScopeProfile, "", "marketing",
		); err != nil {
			t.Fatalf("ListSettingsMCPServers(profile) error = %v", err)
		}
		if _, err := client.GetSettingsMCPAuthStatus(t.Context(), SettingsMCPAuthTarget{
			Name: "linear", Scope: contract.SettingsLayeredScopeProfile, Profile: "marketing",
		}); err != nil {
			t.Fatalf("GetSettingsMCPAuthStatus(profile) error = %v", err)
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

func TestMCPListCommand(t *testing.T) {
	t.Parallel()
	// Invariant: CLI JSON preserves the runtime route's typed rows and adds CompozyOS definitions. Owner: CLI; UT-093.
	t.Run("Should compose runtime and CompozyOS JSON responses UT-093", func(t *testing.T) {
		t.Parallel()
		runtimeJSON := `{"runtimes":[{"runtime":"codex","display_name":"Codex","installed":false,"runtime_mcp":{"mode":"load","can_disable":false},"read_state":"ok","config_files":["/home/test/.codex/config.toml"],"servers":[{"runtime":"codex","name":"savee","scope":"user","transport":"http","auth":"unknown","env_keys":[],"header_keys":[],"copyable":true}]}],"generated_at":"2026-11-14T16:30:00Z"}`
		client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case "/api/settings/runtime-mcp-servers":
				return newHTTPResponse(http.StatusOK, runtimeJSON), nil
			case "/api/settings/mcp-servers":
				return newHTTPResponse(http.StatusOK, `{"mcp_servers":[{"name":"manual","owner":"manual","scope":"user","transport":"stdio"}]}`), nil
			default:
				return nil, fmt.Errorf("unexpected route %s", req.URL.Path)
			}
		})}}
		deps := newWorkspaceTestDeps(t, client)
		stdout, _, err := executeRootCommand(t, deps, "mcp", "list", "-o", "json")
		if err != nil {
			t.Fatal(err)
		}
		var actual contract.MCPListResponse
		if err := json.Unmarshal([]byte(stdout), &actual); err != nil {
			t.Fatal(err)
		}
		var expected contract.RuntimeMCPServersResponse
		if err := json.Unmarshal([]byte(runtimeJSON), &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual.Runtimes, expected.Runtimes) || len(actual.CompozyServers) != 1 || actual.CompozyServers[0].Name != "manual" {
			t.Fatalf("payload=%+v", actual)
		}
	})
}

// Invariant: CLI approval requests preserve the exact digest and project/profile
// identity over UDS, and DELETE uses the same identity. Owner: CLI transport boundary.
func TestMCPApprovalClientRoutes(t *testing.T) {
	t.Parallel()
	calls := 0
	client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.Path != "/api/settings/mcp-servers/sentry/approval" {
			return nil, fmt.Errorf("wrong approval path: %s", req.URL)
		}
		if req.Method == http.MethodDelete {
			if req.URL.Query().Get("workspace_id") != "ws-project" || req.URL.Query().Get("profile") != "work" {
				return nil, fmt.Errorf("revoke lost identity: %s", req.URL)
			}
			return newHTTPResponse(http.StatusNoContent, ""), nil
		}
		var body contract.SettingsMCPApprovalRequest
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		if req.Method != http.MethodPut || body.WorkspaceID != "ws-project" || body.Profile != "work" || body.DefinitionDigest != "observed" {
			return nil, fmt.Errorf("wrong approval request: %+v", body)
		}
		return newHTTPResponse(http.StatusOK, `{"approval":{"server_name":"sentry","workspace_id":"ws-project","profile":"work","state":"allowed"}}`), nil
	})}}
	body := contract.SettingsMCPApprovalRequest{WorkspaceID: "ws-project", Profile: "work", DefinitionDigest: "observed"}
	result, err := client.SetSettingsMCPApproval(t.Context(), "sentry", body, false)
	if err != nil || result.Approval.State != "allowed" {
		t.Fatalf("allow=%+v %v", result, err)
	}
	if _, err := client.SetSettingsMCPApproval(t.Context(), "sentry", body, true); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

// Invariant: import preserves daemon error codes and escaped names. Owner: CLI transport; UT-095.
func TestMCPImportCommand(t *testing.T) {
	t.Parallel()
	t.Run("Should surface a missing runtime server UT-095", func(t *testing.T) {
		t.Parallel()
		client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost || req.URL.Path != "/api/settings/runtime-mcp-servers/claude/nope/import" {
				return nil, fmt.Errorf("unexpected import route %s %s", req.Method, req.URL.Path)
			}
			return newHTTPResponse(http.StatusNotFound, `{"code":"mcp_server_not_found","error":"No MCP server named nope in Claude Code"}`), nil
		})}}
		_, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), "mcp", "import", "claude/nope")
		if cliExitCodeForError(err) != 1 || err == nil || !strings.Contains(err.Error(), "mcp_server_not_found") || !strings.Contains(err.Error(), "No MCP server named nope in Claude Code") {
			t.Fatalf("wrong missing server error: %v", err)
		}
	})
	t.Run("Should escape a source name and forward the target name", func(t *testing.T) {
		t.Parallel()
		client := &daemonClient{target: LocalClientTarget("/tmp/compozy.sock"), httpClient: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.EscapedPath() != "/api/settings/runtime-mcp-servers/claude/plugin%3Afigma%3Afigma/import" {
				return nil, fmt.Errorf("unexpected import path %s", req.URL.EscapedPath())
			}
			var body contract.RuntimeMCPImportRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				return nil, err
			}
			if body.Scope != "user" || body.Name != "renamed" {
				return nil, fmt.Errorf("import lost target identity")
			}
			return newHTTPResponse(http.StatusUnprocessableEntity, `{"code":"mcp_import_unsupported_source","error":"plugin source cannot be copied"}`), nil
		})}}
		_, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), "mcp", "import", "claude/plugin:figma:figma", "--name", "renamed")
		if err == nil || !strings.Contains(err.Error(), "mcp_import_unsupported_source") {
			t.Fatalf("wrong source error: %v", err)
		}
	})
}
