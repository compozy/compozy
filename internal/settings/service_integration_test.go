//go:build integration

package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	mcppkg "github.com/compozy/agh/internal/mcp"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

const settingsCatalogMCPHelperEnv = "AGH_SETTINGS_CATALOG_MCP_HELPER"

func TestProviderOverlayDeleteRevealsBuiltinFallbackMetadataCorrectly(t *testing.T) {
	ctx := context.Background()
	homePaths := testHomePaths(t)
	writeFile(t, homePaths.ConfigFile, `
[providers.codex]
command = "custom-codex"
`)

	service := testService(t, homePaths, Dependencies{})

	before, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
	if err != nil {
		t.Fatalf("ListCollection(before providers) error = %v", err)
	}
	codex := findProviderItem(t, before.Providers, "codex")
	if got, want := codex.SourceMetadata.EffectiveSource.Kind, SourceKindGlobalConfig; got != want {
		t.Fatalf("before delete effective source = %q, want %q", got, want)
	}
	if codex.Fallback == nil {
		t.Fatal("before delete fallback = nil, want builtin fallback metadata")
	}

	if _, err := service.DeleteCollectionItem(ctx, CollectionItemDeleteRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "codex",
	}); err != nil {
		t.Fatalf("DeleteCollectionItem(provider) error = %v", err)
	}

	after, err := service.ListCollection(ctx, CollectionRequest{Collection: CollectionProviders})
	if err != nil {
		t.Fatalf("ListCollection(after providers) error = %v", err)
	}
	codex = findProviderItem(t, after.Providers, "codex")
	if got, want := codex.SourceMetadata.EffectiveSource.Kind, SourceKindBuiltinProvider; got != want {
		t.Fatalf("after delete effective source = %q, want %q", got, want)
	}
	if codex.Fallback != nil {
		t.Fatalf("after delete fallback = %#v, want nil", codex.Fallback)
	}
}

func TestWorkspaceScopedMCPMutationResolvesWorkspaceRootAndPersistsToTarget(t *testing.T) {
	ctx := context.Background()
	homePaths := testHomePaths(t)
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")

	service := testService(t, homePaths, Dependencies{
		WorkspaceResolver: fakeWorkspaceResolver{
			resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-1": {
					Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot},
				},
			},
		},
	})

	result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{
			Collection:  CollectionMCPServers,
			Scope:       ScopeWorkspace,
			WorkspaceID: "ws-1",
		},
		Name: "workspace-alpha",
		MCPServer: &aghconfig.MCPServer{
			Command: "workspace-command",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(workspace mcp) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetWorkspaceMCPSidecar; got != want {
		t.Fatalf("workspace mcp write target = %q, want %q", got, want)
	}

	sidecarPath := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.MCPJSONName)
	payload := readFile(t, sidecarPath)
	if !strings.Contains(payload, `"workspace-alpha"`) || !strings.Contains(payload, `"workspace-command"`) {
		t.Fatalf("workspace sidecar missing persisted MCP server:\n%s", payload)
	}
}

func TestMCPCatalogInstallPersistsEncryptedSecretAndExecutorResolvesIt(t *testing.T) {
	t.Run("Should persist an encrypted secret and resolve it in the MCP executor", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		store := newSettingsIntegrationVaultStore()
		vaultService, err := vault.NewService(store, settingsIntegrationKeyProvider{})
		if err != nil {
			t.Fatalf("vault.NewService() error = %v", err)
		}
		entry := stdioMCPCatalogEntry()
		entry.Name = "catalog-helper"
		entry.Payload = json.RawMessage(`{
		"transport":"stdio",
		"command":"` + strings.ReplaceAll(os.Args[0], `\`, `\\`) + `",
		"args":["-test.run=TestSettingsCatalogMCPStdioHelperProcess"],
		"env":[
			{"name":"` + settingsCatalogMCPHelperEnv + `","required":true,"secret":false,"default":"1"},
			{"name":"CATALOG_TOKEN","required":true,"secret":true}
		],
		"default_scope":"global"
	}`)
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: entry},
			ProviderSecrets: vaultService,
		})

		installed, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeGlobal,
			Values: MCPCatalogInstallValues{Env: map[string]MCPSecretInput{
				"CATALOG_TOKEN": {Value: "executor-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog() error = %v", err)
		}
		assertMCPSecretKeys(t, installed.Item, "CATALOG_TOKEN")
		ref := "vault:mcp/global/catalog-helper/env/CATALOG_TOKEN"
		record, err := store.GetVaultSecret(ctx, ref)
		if err != nil {
			t.Fatalf("GetVaultSecret(%q) error = %v", ref, err)
		}
		if strings.Contains(record.EncryptedValue, "executor-secret") {
			t.Fatalf("vault record contains plaintext: %#v", record)
		}
		servers, err := aghconfig.LoadMCPServersJSONFile(filepath.Join(homePaths.HomeDir, aghconfig.MCPJSONName))
		if err != nil {
			t.Fatalf("LoadMCPServersJSONFile() error = %v", err)
		}
		if len(servers) != 1 {
			t.Fatalf("len(sidecar servers) = %d, want 1", len(servers))
		}
		executor, err := mcppkg.NewMCPCallExecutor(
			mcppkg.ServerResolverFunc(func(
				_ context.Context,
				source toolspkg.SourceRef,
			) (mcppkg.ResolvedServer, error) {
				if source.RawServerName != installed.Item.Name {
					return mcppkg.ResolvedServer{}, fmt.Errorf(
						"resolve unexpected MCP server %q",
						source.RawServerName,
					)
				}
				return mcppkg.ResolvedServer{
					Server: servers[0],
					Target: mcpauth.Target{
						Scope:      mcpauth.ScopeGlobal,
						ServerName: installed.Item.Name,
					},
				}, nil
			}),
			mcppkg.WithSecretResolver(vaultService),
			mcppkg.WithTimeout(5*time.Second),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         installed.Item.Name,
			RawServerName: installed.Item.Name,
			RawToolName:   "*",
		}
		descriptors, err := executor.ListTools(ctx, source)
		if err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		if len(descriptors) != 1 {
			t.Fatalf("len(ListTools()) = %d, want 1", len(descriptors))
		}
		result, err := executor.CallTool(ctx, descriptors[0].Source, toolspkg.MCPToolCallRequest{
			ToolID:      descriptors[0].ID,
			RawToolName: descriptors[0].RawName,
			Input:       json.RawMessage(`{"message":"CATALOG_TOKEN"}`),
		})
		if err != nil {
			t.Fatalf("CallTool() error = %v", err)
		}
		if got, want := result.Preview, "env: executor-secret"; got != want {
			t.Fatalf("CallTool().Preview = %q, want %q", got, want)
		}
	})
}

func TestSettingsCatalogMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv(settingsCatalogMCPHelperEnv) != "1" {
		return
	}
	// Intentionally serial: this subprocess test owns stdin/stdout as the MCP stdio protocol.
	t.Run("Should serve the catalog MCP helper over stdio", func(_ *testing.T) {
		server := mcpsrv.NewMCPServer("settings-catalog-helper", "1.0.0", mcpsrv.WithToolCapabilities(true))
		server.AddTool(
			mcpsdk.NewTool("env", mcpsdk.WithString("message")),
			func(_ context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				name := mcpsdk.ParseString(req, "message", "")
				value := os.Getenv(name)
				return mcpsdk.NewToolResultStructured(
					map[string]string{"message": value},
					"env: "+value,
				), nil
			},
		)
		if err := mcpsrv.ServeStdio(server); err != nil {
			if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
				os.Exit(3)
			}
			os.Exit(2)
		}
		os.Exit(0)
	})
}

type settingsIntegrationKeyProvider struct{}

func (settingsIntegrationKeyProvider) Key() ([]byte, error) {
	return []byte("01234567890123456789012345678901"), nil
}

type settingsIntegrationVaultStore struct {
	records map[string]vault.Record
}

func newSettingsIntegrationVaultStore() *settingsIntegrationVaultStore {
	return &settingsIntegrationVaultStore{records: make(map[string]vault.Record)}
}

func (s *settingsIntegrationVaultStore) PutVaultSecret(_ context.Context, record vault.Record) error {
	s.records[record.Ref] = record
	return nil
}

func (s *settingsIntegrationVaultStore) GetVaultSecret(_ context.Context, ref string) (vault.Record, error) {
	record, ok := s.records[vault.NormalizeRef(ref)]
	if !ok {
		return vault.Record{}, vault.ErrSecretNotFound
	}
	return record, nil
}

func (s *settingsIntegrationVaultStore) ListVaultSecrets(
	_ context.Context,
	prefix string,
) ([]vault.Record, error) {
	records := make([]vault.Record, 0, len(s.records))
	for ref, record := range s.records {
		if vault.RefMatchesPrefix(ref, prefix) {
			records = append(records, record)
		}
	}
	slices.SortFunc(records, func(left vault.Record, right vault.Record) int {
		return strings.Compare(left.Ref, right.Ref)
	})
	return records, nil
}

func (s *settingsIntegrationVaultStore) DeleteVaultSecret(_ context.Context, ref string) error {
	normalized := vault.NormalizeRef(ref)
	if _, ok := s.records[normalized]; !ok {
		return vault.ErrSecretNotFound
	}
	delete(s.records, normalized)
	return nil
}

func TestMutationResultExposesSemanticWriteTarget(t *testing.T) {
	ctx := context.Background()
	homePaths := testHomePaths(t)
	service := testService(t, homePaths, Dependencies{})

	result, err := service.PutCollectionItem(ctx, CollectionItemPutRequest{
		CollectionRequest: CollectionRequest{Collection: CollectionProviders},
		Name:              "custom",
		Provider: &ProviderSettings{
			Command: "custom-provider",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(provider) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetGlobalConfig; got != want {
		t.Fatalf("provider write target = %q, want %q", got, want)
	}
	if strings.Contains(string(result.WriteTarget), "/") {
		t.Fatalf("provider write target = %q, want semantic identifier not path", result.WriteTarget)
	}
}

func findProviderItem(t *testing.T, items []ProviderItem, name string) ProviderItem {
	t.Helper()
	for _, item := range items {
		if item.Name == name {
			return item
		}
	}
	t.Fatalf("Provider item %q not found in %#v", name, items)
	return ProviderItem{}
}
