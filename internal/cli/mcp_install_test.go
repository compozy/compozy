package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestMCPInstallCommandMapsFlagsAndPreservesStructuredResponse(t *testing.T) {
	t.Run("Should map flags and preserve the structured response", func(t *testing.T) {
		t.Parallel()

		want := InstallSettingsMCPServerRecord{
			MCPServer: contract.SettingsMCPServerItemPayload{
				Name:          "github-workspace",
				Transport:     "stdio",
				Command:       "npx",
				SecretEnvKeys: []string{"GITHUB_TOKEN"},
				Auth: &contract.SettingsMCPAuthConfigViewPayload{
					ClientSecretConfigured: true,
				},
				Scope:          contract.SettingsScopeWorkspace,
				WorkspaceID:    "ws-1",
				CatalogEntry:   "github",
				CatalogVersion: "1.2.3",
			},
			Apply: mcpInstallApplyFixture(
				contract.SettingsScopeWorkspace,
				contract.SettingsWriteTargetWorkspaceMCPSidecar,
				"ws-1",
			),
			NextStep: "none",
		}
		client := &stubClient{
			installSettingsMCPServerFn: func(
				_ context.Context,
				request InstallSettingsMCPServerRequest,
			) (InstallSettingsMCPServerRecord, error) {
				if got, want := request.EntryID, "github"; got != want {
					t.Fatalf("EntryID = %q, want %q", got, want)
				}
				if got, want := request.Name, "github-workspace"; got != want {
					t.Fatalf("Name = %q, want %q", got, want)
				}
				if got, want := request.Scope, contract.SettingsLayeredScopeWorkspace; got != want {
					t.Fatalf("Scope = %q, want %q", got, want)
				}
				if got, want := request.WorkspaceID, "ws-1"; got != want {
					t.Fatalf("WorkspaceID = %q, want %q", got, want)
				}
				if request.Values == nil {
					t.Fatal("request.Values = nil")
				}
				if got, want := request.Values.Inputs["github_token"].Value, "write-only-secret"; got != want {
					t.Fatalf("github_token value = %q, want mapped secret", got)
				}
				if got, want := request.Values.Inputs["config"].VaultRef, "vault:mcp/shared/config"; got != want {
					t.Fatalf("config VaultRef = %q, want %q", got, want)
				}
				if got, want := request.Values.Inputs["project"].Value, "compozy"; got != want {
					t.Fatalf("project value = %q, want mapped setting", got)
				}
				return want, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		args := []string{
			"mcp",
			"install",
			"github",
			"--name",
			"github-workspace",
			"--scope",
			"workspace",
			"--workspace",
			"ws-1",
			"--secret",
			"github_token",
			"--vault-ref",
			"config=vault:mcp/shared/config",
			"--set",
			"project=compozy",
			"-o",
			"json",
		}
		for _, arg := range args {
			if strings.Contains(arg, "write-only-secret") {
				t.Fatalf("MCP install argv leaked a literal secret: %q", arg)
			}
		}
		stdout, _, err := executeMCPInstallCommand(
			t,
			deps,
			"write-only-secret\n",
			args...,
		)
		if err != nil {
			t.Fatalf("mcp install command error = %v", err)
		}
		if strings.Contains(stdout, "write-only-secret") {
			t.Fatalf("mcp install output leaked write-only secret: %s", stdout)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(stdout), &fields); err != nil {
			t.Fatalf("json.Unmarshal(mcp install fields) error = %v", err)
		}
		if _, found := fields["resolution_source"]; found {
			t.Fatalf("mcp install JSON contains resolution_source, want raw daemon payload: %s", stdout)
		}
		for _, secretMaterial := range []string{
			"vault:mcp/ws/ws-1/github-workspace/env/GITHUB_TOKEN",
			"vault:mcp/shared/config",
		} {
			if strings.Contains(stdout, secretMaterial) {
				t.Fatalf("mcp install output leaked secret binding %q: %s", secretMaterial, stdout)
			}
		}
		var got InstallSettingsMCPServerRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(mcp install) error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mcp install response = %#v, want %#v", got, want)
		}
	})

	t.Run("Should render the config apply truth in human output", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{installSettingsMCPServerFn: func(
			context.Context,
			InstallSettingsMCPServerRequest,
		) (InstallSettingsMCPServerRecord, error) {
			return InstallSettingsMCPServerRecord{
				MCPServer: contract.SettingsMCPServerItemPayload{
					Name: "github", Transport: "stdio", Scope: contract.SettingsScopeUser,
					CatalogEntry: "github", CatalogVersion: "1.2.3",
				},
				Apply: contract.SettingsApplyResponse{
					Applied:          false,
					Lifecycle:        contract.SettingsApplyLifecycleLiveAdd,
					ApplyRecordID:    "cfgapp-failed",
					ActiveGeneration: 2,
					NextAction:       contract.SettingsApplyNextActionRetry,
				},
				NextStep: contract.SettingsMCPInstallNextStepNone,
			}, nil
		}}
		stdout, _, err := executeRootCommand(t, newWorkspaceTestDeps(t, client), "mcp", "install", "github")
		if err != nil {
			t.Fatalf("mcp install command error = %v", err)
		}
		for _, detail := range []string{"Applied", "false", "live-add", "cfgapp-failed", "retry"} {
			if !strings.Contains(stdout, detail) {
				t.Fatalf("human output missing %q:\n%s", detail, stdout)
			}
		}
	})

	t.Run("Should map a catalog secret Vault ref", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			installSettingsMCPServerFn: func(
				_ context.Context,
				request InstallSettingsMCPServerRequest,
			) (InstallSettingsMCPServerRecord, error) {
				if request.Values == nil {
					t.Fatal("request.Values = nil")
				}
				if got, want := request.Values.Inputs["linear_token"].VaultRef, "vault:mcp/shared/oauth"; got != want {
					t.Fatalf("linear_token VaultRef = %q, want %q", got, want)
				}
				return InstallSettingsMCPServerRecord{
					MCPServer: contract.SettingsMCPServerItemPayload{
						Name: "github", Scope: contract.SettingsScopeUser,
					},
					NextStep: contract.SettingsMCPInstallNextStepAuthorize,
				}, nil
			},
		}
		stdout, _, err := executeRootCommand(
			t,
			newWorkspaceTestDeps(t, client),
			"mcp",
			"install",
			"github",
			"--vault-ref",
			"linear_token=vault:mcp/shared/oauth",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("mcp install command error = %v", err)
		}
		if strings.Contains(stdout, "vault:mcp/shared/oauth") {
			t.Fatalf("mcp install output leaked catalog secret ref: %s", stdout)
		}
	})

	t.Run("Should carry the active profile into a profile install", func(t *testing.T) {
		t.Parallel()

		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{installSettingsMCPServerFn: func(
				_ context.Context,
				request InstallSettingsMCPServerRequest,
			) (InstallSettingsMCPServerRecord, error) {
				if request.Scope != contract.SettingsLayeredScopeProfile || request.Profile != "marketing" ||
					request.WorkspaceID != "" {
					t.Fatalf("profile install request = %#v", request)
				}
				return InstallSettingsMCPServerRecord{MCPServer: contract.SettingsMCPServerItemPayload{
					Name: "github", Scope: contract.SettingsScopeProfile, Profile: "marketing",
				}}, nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		stdout, _, err := executeRootCommand(
			t, newTestDeps(t, client),
			"--profile", "marketing", "mcp", "install", "github", "--scope", "profile", "-o", "json",
		)
		if err != nil {
			t.Fatalf("profile MCP install error = %v", err)
		}
		var response InstallSettingsMCPServerRecord
		if err := json.Unmarshal([]byte(stdout), &response); err != nil {
			t.Fatalf("decode profile MCP install output: %v", err)
		}
		if response.MCPServer.Profile != "marketing" {
			t.Fatalf("profile MCP install output = %#v", response)
		}
	})
}

func TestMCPInstallCommandRejectsAmbiguousInputsBeforeCallingDaemon(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		detail string
	}{
		{
			name: "Should reject duplicate typed and Vault modes",
			args: []string{
				"mcp", "install", "github",
				"--set", "TOKEN=secret",
				"--vault-ref", "TOKEN=vault:mcp/shared/token",
			},
			detail: "assigned more than once",
		},
		{
			name:   "Should reject workspace ID without workspace scope",
			args:   []string{"mcp", "install", "github", "--workspace", "ws-1"},
			detail: "requires --scope workspace",
		},
		{
			name: "Should reject scope before reading requested secret input",
			args: []string{
				"mcp", "install", "github", "--scope", "invalid", "--set", "TOKEN=secret",
			},
			detail: "unsupported MCP install scope",
		},
		{
			name:   "Should reject malformed assignments",
			args:   []string{"mcp", "install", "github", "--set", "TOKEN"},
			detail: "requires KEY=VALUE",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps := newWorkspaceTestDeps(t, &stubClient{})
			_, _, err := executeRootCommand(t, deps, tc.args...)
			if err == nil || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("executeRootCommand() error = %v, want detail %q", err, tc.detail)
			}
		})
	}

	t.Run("Should reject duplicate secret and Vault modes before reading secret input", func(t *testing.T) {
		t.Parallel()

		secretRead := false
		_, err := mcpInstallInputs(
			nil,
			[]string{"TOKEN"},
			[]string{"TOKEN=vault:mcp/shared/token"},
			func(string) (string, error) {
				secretRead = true
				return "secret", nil
			},
		)
		if err == nil || !strings.Contains(err.Error(), "assigned more than once") {
			t.Fatalf("mcpInstallInputs() error = %v, want duplicate assignment rejection", err)
		}
		if secretRead {
			t.Fatal("mcpInstallInputs() read secret input before validating all assignments")
		}
	})
}

func TestMCPInstallInfersWorkspaceForWorkspaceScope(t *testing.T) {
	t.Parallel()

	t.Run("Should infer workspace for workspace scope", func(t *testing.T) {
		t.Parallel()

		var captured InstallSettingsMCPServerRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if ref != "/workspace/project" {
					t.Fatalf("GetWorkspace() ref = %q, want cwd", ref)
				}
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: "ws-cwd"}}, nil
			},
			installSettingsMCPServerFn: func(
				_ context.Context,
				request InstallSettingsMCPServerRequest,
			) (InstallSettingsMCPServerRecord, error) {
				captured = request
				return InstallSettingsMCPServerRecord{}, nil
			},
		})
		_, _, err := executeRootCommand(t, deps, "mcp", "install", "github", "--scope", "workspace")
		if err != nil {
			t.Fatalf("mcp install error = %v", err)
		}
		if captured.WorkspaceID != "ws-cwd" {
			t.Fatalf("request.WorkspaceID = %q, want ws-cwd", captured.WorkspaceID)
		}
	})
}

func TestMCPInstallCommandRejectsBlankSecretInputBeforeCallingDaemon(t *testing.T) {
	t.Run("Should reject a blank piped field value", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		_, _, err := executeMCPInstallCommand(
			t,
			deps,
			"\n",
			"mcp",
			"install",
			"github",
			"--secret",
			"TOKEN",
		)
		if err == nil || !strings.Contains(err.Error(), "requires a non-blank value from stdin") {
			t.Fatalf("mcp install error = %v, want blank stdin rejection", err)
		}
	})
}

func TestUnixSocketClientInstallSettingsMCPServerUsesCanonicalEndpoint(t *testing.T) {
	t.Run("Should use the canonical MCP install endpoint", func(t *testing.T) {
		t.Parallel()

		want := InstallSettingsMCPServerRecord{
			MCPServer: contract.SettingsMCPServerItemPayload{
				Name: "github", Transport: "stdio", Scope: contract.SettingsScopeUser,
				CatalogEntry: "github", CatalogVersion: "1.2.3",
			},
			Apply: mcpInstallApplyFixture(
				contract.SettingsScopeUser,
				contract.SettingsWriteTargetGlobalMCPSidecar,
				"",
			),
			NextStep: "none",
		}
		request := InstallSettingsMCPServerRequest{
			EntryID: "github",
			Scope:   contract.SettingsLayeredScopeUser,
			Values:  &contract.SettingsMCPCatalogInstallValuesPayload{},
		}
		transport := roundTripperFunc(func(httpRequest *http.Request) (*http.Response, error) {
			if got, want := httpRequest.Method, http.MethodPost; got != want {
				t.Fatalf("method = %q, want %q", got, want)
			}
			if got, want := httpRequest.URL.Path, "/api/settings/mcp-servers/install"; got != want {
				t.Fatalf("path = %q, want %q", got, want)
			}
			var gotRequest InstallSettingsMCPServerRequest
			if err := json.NewDecoder(httpRequest.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("Decode(request) error = %v", err)
			}
			if !reflect.DeepEqual(gotRequest, request) {
				t.Fatalf("request = %#v, want %#v", gotRequest, request)
			}
			payload, err := json.Marshal(want)
			if err != nil {
				t.Fatalf("json.Marshal(response) error = %v", err)
			}
			return newHTTPResponse(http.StatusOK, string(payload)), nil
		})
		client := &daemonClient{
			target:     LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: transport},
		}
		client.streamClient = client.httpClient

		got, err := client.InstallSettingsMCPServer(t.Context(), request)
		if err != nil {
			t.Fatalf("InstallSettingsMCPServer() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("InstallSettingsMCPServer() = %#v, want %#v", got, want)
		}
	})
}

func mcpInstallApplyFixture(
	scope contract.SettingsScopeKind,
	writeTarget contract.SettingsWriteTargetKind,
	workspaceID string,
) contract.SettingsApplyResponse {
	return contract.SettingsApplyResponse{
		Section:          contract.SettingsApplyTargetMCPServers,
		Scope:            scope,
		WriteTarget:      writeTarget,
		WorkspaceID:      workspaceID,
		Applied:          true,
		Lifecycle:        contract.SettingsApplyLifecycleLiveAdd,
		ApplyRecordID:    "cfgapp-mcp-install",
		ActiveGeneration: 3,
		ActiveConfigHash: "sha256:mcp-install",
		NextAction:       contract.SettingsApplyNextActionNone,
	}
}

func TestMCPInstallClientPropagatesTransportErrors(t *testing.T) {
	t.Run("Should preserve the transport error detail", func(t *testing.T) {
		t.Parallel()

		client := &daemonClient{
			target: LocalClientTarget("/tmp/compozy.sock"),
			httpClient: &http.Client{Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) {
				return nil, errors.New("transport unavailable")
			})},
		}
		client.streamClient = client.httpClient
		_, err := client.InstallSettingsMCPServer(t.Context(), InstallSettingsMCPServerRequest{})
		if err == nil || !strings.Contains(err.Error(), "transport unavailable") {
			t.Fatalf("InstallSettingsMCPServer() error = %v, want transport unavailable", err)
		}
	})
}

func executeMCPInstallCommand(
	t *testing.T,
	deps commandDeps,
	stdin string,
	args ...string,
) (string, string, error) {
	t.Helper()

	cmd := newRootCommand(deps)
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(t.Context())
	return stdout.String(), stderr.String(), err
}
