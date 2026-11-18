package settings

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestMCPServerTargetSelectorValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid put selectors without mutating MCP sources", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "config-before"
`)
		sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
		writeFile(t, sidecarPath, `{
  "mcpServers": {
    "alpha": { "command": "sidecar-before" }
  }
}`)
		configBefore := readFile(t, homePaths.ConfigFile)
		sidecarBefore := readFile(t, sidecarPath)
		service := testService(t, homePaths, Dependencies{})

		_, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			Collection: CollectionMCPServers,
			Name:       "alpha",
			Target:     TargetSelector("cfg"),
			MCPServer: &compozyconfig.MCPServer{
				Command: "after",
			},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("PutCollectionItem(invalid target) error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "unsupported MCP target selector") {
			t.Fatalf("PutCollectionItem(invalid target) error = %v, want selector context", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != configBefore {
			t.Fatalf("config payload changed after invalid target:\n%s", got)
		}
		if got := readFile(t, sidecarPath); got != sidecarBefore {
			t.Fatalf("sidecar payload changed after invalid target:\n%s", got)
		}
	})

	t.Run("Should reject invalid delete selectors without mutating MCP sources", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "alpha"
command = "config-before"
`)
		sidecarPath := filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName)
		writeFile(t, sidecarPath, `{
  "mcpServers": {
    "alpha": { "command": "sidecar-before" }
  }
}`)
		configBefore := readFile(t, homePaths.ConfigFile)
		sidecarBefore := readFile(t, sidecarPath)
		service := testService(t, homePaths, Dependencies{})

		_, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			Collection: CollectionMCPServers,
			Name:       "alpha",
			Target:     TargetSelector("CONFIG"),
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("DeleteCollectionItem(invalid target) error = %v, want ErrValidation", err)
		}
		if !strings.Contains(err.Error(), "unsupported MCP target selector") {
			t.Fatalf("DeleteCollectionItem(invalid target) error = %v, want selector context", err)
		}
		if got := readFile(t, homePaths.ConfigFile); got != configBefore {
			t.Fatalf("config payload changed after invalid target:\n%s", got)
		}
		if got := readFile(t, sidecarPath); got != sidecarBefore {
			t.Fatalf("sidecar payload changed after invalid target:\n%s", got)
		}
	})
}

func TestMCPServerDefinitionMutationsInvalidatePendingOAuthSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should invalidate pending state before deleting durable auth state", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, `
[[mcp_servers]]
name = "linear"
transport = "http"
url = "https://original.example/mcp"

[mcp_servers.auth]
registration = "pre_registered"
issuer_url = "https://issuer.example"
client_id = "client-id"
`)
		runtime := &recordingMCPAuthRuntime{}
		service := testService(t, homePaths, Dependencies{MCPAuth: runtime})
		// Invariant: manual definition changes retain manual ownership during OAuth invalidation and cleanup.
		// Owner: Settings mutation lifecycle; canonical suite: mcp_target_test.go.
		target := mcpauth.Target{Owner: "manual", Scope: mcpauth.ScopeUser, ServerName: "linear"}

		if _, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
			Collection: CollectionMCPServers,
			Name:       "linear",
			Target:     TargetConfig,
			MCPServer: &compozyconfig.MCPServer{
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://replacement.example/mcp",
				Auth: compozyconfig.MCPAuthConfig{
					Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
					IssuerURL:    "https://issuer.example",
					ClientID:     "client-id",
				},
			},
		}); err != nil {
			t.Fatalf("PutCollectionItem() error = %v", err)
		}
		assertMCPAuthLifecycle(t, runtime.operations, target, 1)

		if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
			Collection: CollectionMCPServers,
			Name:       "linear",
			Target:     TargetConfig,
		}); err != nil {
			t.Fatalf("DeleteCollectionItem() error = %v", err)
		}
		assertMCPAuthLifecycle(t, runtime.operations, target, 2)
	})
}

func assertMCPAuthLifecycle(
	t *testing.T,
	operations []recordingMCPAuthOperation,
	want mcpauth.Target,
	wantCycles int,
) {
	t.Helper()
	if got, wantCount := len(operations), wantCycles*2; got != wantCount {
		t.Fatalf("MCP auth lifecycle operation count = %d, want %d", got, wantCount)
	}
	for index, operation := range operations {
		wantKind := recordingMCPAuthInvalidate
		if index%2 == 1 {
			wantKind = recordingMCPAuthDeleteState
		}
		if operation.kind != wantKind || operation.target != want {
			t.Fatalf("MCP auth lifecycle[%d] = %#v, want %s for %#v", index, operation, wantKind, want)
		}
	}
}

// Invariant: secret rollback follows the durable definition outcome. Owner: Settings MCP mutation; UT-062.
func TestMCPImportSecretCommitOwnership(t *testing.T) {
	t.Parallel()
	for _, outcome := range []string{"uncommitted", "restored", "committed"} {
		t.Run("Should retain only secrets owned by a committed copy "+outcome, func(t *testing.T) {
			t.Parallel()
			home := testHomePaths(t)
			root := t.TempDir()
			store := newFakeProviderSecretStore()
			failure := errors.New("injected mutation boundary failure")
			deps := Dependencies{
				ProviderSecrets: store,
				WorkspaceResolver: fakeWorkspaceResolver{
					resolved: map[string]workspacepkg.ResolvedWorkspace{"ws-copy": {ID: "ws-copy", RootDir: root}},
				},
			}
			switch outcome {
			case "uncommitted":
				deps.MCPDefinitionWriter = func(compozyconfig.HomePaths, string, string, compozyconfig.WriteTarget, compozyconfig.MCPServer) error {
					return failure
				}
			case "restored":
				deps.MCPAuth = &recordingMCPAuthRuntime{deleteStateErrs: map[int]error{1: failure}}
			case "committed":
				deps.WorkspaceMCPApprovals = importCommitRecorder{afterCommit: failure}
			}
			settings := testService(t, home, deps).(*service)
			result, err := settings.putMCPServer(
				t.Context(),
				ScopeWorkspace,
				"ws-copy",
				"",
				"copy",
				TargetAuto,
				compozyconfig.MCPServer{Command: "echo"},
				MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "copied-value"}},
				MCPSecretPreservation{},
				nil,
				true,
			)
			ref := "vault:mcp/ws/ws-copy/copy/env/TOKEN"
			if outcome != "committed" {
				if !errors.Is(err, failure) {
					t.Fatalf("wrong write failure: %v", err)
				}
				if _, found := store.plaintext[ref]; found {
					t.Fatal("uncommitted copy retained new secret")
				}
				collection, err := settings.ListCollection(
					t.Context(),
					CollectionRequest{
						Collection:        CollectionMCPServers,
						Scope:             ScopeWorkspace,
						WorkspaceID:       "ws-copy",
						SkipRuntimeStatus: true,
					},
				)
				if err != nil || len(collection.MCPServers) != 0 {
					t.Fatalf("failed copy left a definition: %v", err)
				}
				return
			}
			if err != nil || result.MCPServer == nil || len(result.Warnings) != 1 ||
				result.Warnings[0] != mcpImportCleanupWarning("copy") {
				t.Fatalf("committed copy did not return its warning: %v", err)
			}
			if store.plaintext[ref] != "copied-value" {
				t.Fatal("committed copy lost its secret")
			}
			reopened := testService(t, home, Dependencies{ProviderSecrets: store, WorkspaceResolver: deps.WorkspaceResolver}).(*service)
			_, sources, err := reopened.resolveMCPTargetContext(t.Context(), ScopeWorkspace, "ws-copy", "")
			if err != nil {
				t.Fatal(err)
			}
			entry, found := mcpSourceForTarget("copy", result.WriteTarget, sources)
			if !found || entry.Server.SecretEnv["TOKEN"] != ref || len(entry.Server.Env) != 0 {
				t.Fatal("reopened definition lost its Vault binding")
			}
			if value, err := store.ResolveRef(
				t.Context(),
				entry.Server.SecretEnv["TOKEN"],
			); err != nil ||
				value != "copied-value" {
				t.Fatal("committed definition references a removed secret")
			}
		})
	}
	// Invariant: partial Vault storage removes only new unreferenced entries. Owner: Settings mutation; canonical suite: TestMCPImportSecretCommitOwnership.
	for _, referenced := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should roll back partial Vault storage with referenced=%t", referenced), func(t *testing.T) {
			t.Parallel()
			home := testHomePaths(t)
			ref := "vault:mcp/user/copy/env/A"
			if referenced {
				writeFile(
					t,
					home.ConfigFile,
					"[[mcp_servers]]\nname = \"other\"\ncommand = \"echo\"\n[mcp_servers.secret_env]\nTOKEN = \""+ref+"\"\n",
				)
			}
			secretStore := newFakeProviderSecretStore()
			failure := errors.New("second secret write failed")
			secretStore.putErrors["vault:mcp/user/copy/env/B"] = failure
			settings := testService(t, home, Dependencies{ProviderSecrets: secretStore}).(*service)
			_, err := settings.putMCPServer(
				t.Context(),
				ScopeUser,
				"",
				"",
				"copy",
				TargetAuto,
				compozyconfig.MCPServer{Command: "echo"},
				MCPSecretValues{SecretEnv: map[string]string{"A": "first", "B": "second"}},
				MCPSecretPreservation{},
				nil,
				true,
			)
			if !errors.Is(err, failure) {
				t.Fatalf("unexpected storage failure: %v", err)
			}
			_, exists := secretStore.plaintext[ref]
			if exists != referenced {
				t.Fatal("partial storage violated reference ownership")
			}
			if _, sources, err := settings.resolveMCPTargetContext(
				t.Context(),
				ScopeUser,
				"",
				"",
			); err != nil ||
				len(sources["copy"]) != 0 {
				t.Fatalf("partial storage wrote a definition: %v", err)
			}
		})
	}

	t.Run("Should retain a newly stored entry still referenced by another definition", func(t *testing.T) {
		t.Parallel()
		home := testHomePaths(t)
		writeFile(
			t,
			home.ConfigFile,
			"[[mcp_servers]]\nname = \"other\"\ncommand = \"echo\"\n[mcp_servers.secret_env]\nTOKEN = \"vault:mcp/user/copy/env/TOKEN\"\n",
		)
		store := newFakeProviderSecretStore()
		failure := errors.New("write refused")
		settings := testService(t, home, Dependencies{ProviderSecrets: store, MCPDefinitionWriter: func(compozyconfig.HomePaths, string, string, compozyconfig.WriteTarget, compozyconfig.MCPServer) error {
			return failure
		}}).(*service)
		_, err := settings.putMCPServer(
			t.Context(),
			ScopeUser,
			"",
			"",
			"copy",
			TargetAuto,
			compozyconfig.MCPServer{Command: "echo"},
			MCPSecretValues{SecretEnv: map[string]string{"TOKEN": "referenced-value"}},
			MCPSecretPreservation{},
			nil,
			true,
		)
		if !errors.Is(err, failure) {
			t.Fatalf("unexpected failure: %v", err)
		}
		if value, err := store.ResolveRef(
			t.Context(),
			"vault:mcp/user/copy/env/TOKEN",
		); err != nil ||
			value != "referenced-value" {
			t.Fatal("rollback deleted a referenced entry")
		}
	})
}

type importCommitRecorder struct{ afterCommit error }

var _ WorkspaceMCPApprovalRecorder = importCommitRecorder{}

func (r importCommitRecorder) RecordWorkspaceMCPWrite(_ context.Context, write WorkspaceMCPWrite) (string, error) {
	if err := write.Write(); err != nil {
		return "", err
	}
	return "", r.afterCommit
}
