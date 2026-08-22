//go:build integration

package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const settingsCatalogMCPHelperEnv = "COMPOZY_SETTINGS_CATALOG_MCP_HELPER"

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
		MCPServer: &compozyconfig.MCPServer{
			Command: "workspace-command",
		},
	})
	if err != nil {
		t.Fatalf("PutCollectionItem(workspace mcp) error = %v", err)
	}
	if got, want := result.WriteTarget, WriteTargetWorkspaceMCPSidecar; got != want {
		t.Fatalf("workspace mcp write target = %q, want %q", got, want)
	}

	sidecarPath := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.MCPJSONName)
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
		"launch":{"type":"npm","package":"catalog-helper","version":"1.0.0"},
		"inputs":[
			{"id":"helper_mode","prompt":"Helper mode","type":"string","required":false,"default":"1","binding":{"type":"env","name":"` + settingsCatalogMCPHelperEnv + `"}},
			{"id":"catalog_token","prompt":"Catalog token","type":"secret","required":true,"binding":{"type":"env","name":"CATALOG_TOKEN"}}
		],
		"default_scope":"user"
	}`)
		service := testService(t, homePaths, Dependencies{
			MCPCatalog:      fakeMCPCatalog{entry: entry},
			ProviderSecrets: vaultService,
		})

		installed, err := service.InstallMCPCatalog(ctx, MCPCatalogInstallRequest{
			EntryID: "github",
			Scope:   ScopeUser,
			Values: MCPCatalogInstallValues{Inputs: map[string]MCPSecretInput{
				"catalog_token": {Value: "executor-secret"},
			}},
		})
		if err != nil {
			t.Fatalf("InstallMCPCatalog() error = %v", err)
		}
		assertMCPSecretKeys(t, installed.Item, "CATALOG_TOKEN")
		ref := "vault:mcp/user/catalog-helper/env/CATALOG_TOKEN"
		record, err := store.GetVaultSecret(ctx, ref)
		if err != nil {
			t.Fatalf("GetVaultSecret(%q) error = %v", ref, err)
		}
		if strings.Contains(record.EncryptedValue, "executor-secret") {
			t.Fatalf("vault record contains plaintext: %#v", record)
		}
		servers, err := compozyconfig.LoadMCPServersJSONFile(
			filepath.Join(homePaths.HomeDir, compozyconfig.MCPJSONName),
		)
		if err != nil {
			t.Fatalf("LoadMCPServersJSONFile() error = %v", err)
		}
		if len(servers) != 1 {
			t.Fatalf("len(sidecar servers) = %d, want 1", len(servers))
		}
		if got, want := servers[0].Command, "npx"; got != want {
			t.Fatalf("catalog server command = %q, want %q", got, want)
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
				server := servers[0]
				server.Command = os.Args[0]
				server.Args = []string{"-test.run=TestSettingsCatalogMCPStdioHelperProcess"}
				return mcppkg.ResolvedServer{
					Server: server,
					Target: mcpauth.Target{
						Scope:      mcpauth.ScopeUser,
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
		if got, want := result.Preview, "env: [REDACTED]"; got != want {
			t.Fatalf("CallTool().Preview = %q, want %q", got, want)
		}
		if len(result.Redactions) == 0 {
			t.Fatal("CallTool().Redactions is empty, want secret redaction evidence")
		}
	})
}

func TestSettingsCatalogMCPStdioHelperProcess(t *testing.T) {
	if os.Getenv(settingsCatalogMCPHelperEnv) != "1" {
		return
	}
	// Intentionally serial: this subprocess test owns stdin/stdout as the MCP stdio protocol.
	t.Run("Should serve the catalog MCP helper over stdio", func(_ *testing.T) {
		server := mcp.NewServer(
			&mcp.Implementation{Name: "settings-catalog-helper", Version: "1.0.0"},
			&mcp.ServerOptions{
				Capabilities: &mcp.ServerCapabilities{Tools: &mcp.ToolCapabilities{}},
			},
		)
		server.AddTool(&mcp.Tool{
			Name: "env",
			InputSchema: json.RawMessage(
				`{"type":"object","properties":{"message":{"type":"string"}},"required":["message"],"additionalProperties":false}`,
			),
		}, func(_ context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var input struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(request.Params.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode env tool input: %w", err)
			}
			value := os.Getenv(input.Message)
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: "env: " + value}},
				StructuredContent: map[string]string{"message": value},
			}, nil
		})
		if err := server.Run(context.Background(), &mcp.IOTransport{Reader: os.Stdin, Writer: os.Stdout}); err != nil {
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

func TestProfilePaletteSettingsPreserveMachineShortcutIdentityIT092(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	homePaths := testHomePaths(t)
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	writeFile(t, homePaths.ConfigFile, baseSettingsConfig()+`

[cmd_palette]
fallback_targets = ["agent"]
personalization = true

[cmd_palette.aliases]
review = "agent:user"

[window_manager.global_shortcuts]
"palette.summon.global" = "meta+shift+Space"
`)
	service := testService(t, homePaths, Dependencies{WorkspaceResolver: fakeWorkspaceResolver{
		resolved: map[string]workspacepkg.ResolvedWorkspace{
			"ws-1": {Workspace: workspacepkg.Workspace{ID: "ws-1", RootDir: workspaceRoot}},
		},
	}})

	writeProfilePalette := func(profileName string, enabled bool, personalized bool, alias string) {
		t.Helper()
		aliases := map[string]string{"review": alias}
		result, err := service.UpdateSection(ctx, SectionUpdateRequest{
			SectionRequest: SectionRequest{
				Section: SectionCmdPalette, Scope: ScopeProfile, ProfileName: profileName,
			},
			CmdPalette: &CmdPaletteUpdate{
				FallbackAgentEnabled: &enabled, Personalization: &personalized, Aliases: &aliases,
			},
		})
		if err != nil {
			t.Fatalf("UpdateSection(cmd palette profile %q) error = %v", profileName, err)
		}
		if result.WriteTarget != WriteTargetProfileConfig || result.ProfileName != profileName {
			t.Fatalf("profile %q palette result = %#v", profileName, result)
		}
	}
	writeProfilePalette("marketing", false, false, "agent:marketing")
	writeProfilePalette("sales", true, true, "agent:sales")

	assertPalette := func(label string, cfg compozyconfig.Config, enabled bool, personalized bool, alias string) {
		t.Helper()
		if got := slices.Contains(cfg.CmdPalette.FallbackTargets, compozyconfig.CmdPaletteFallbackAgent); got != enabled ||
			cfg.CmdPalette.Personalization != personalized || cfg.CmdPalette.Aliases["review"] != alias {
			t.Fatalf("%s palette = %#v, want enabled=%v personalized=%v alias=%q", label, cfg.CmdPalette, enabled, personalized, alias)
		}
	}
	for _, test := range []struct {
		name         string
		enabled      bool
		personalized bool
		alias        string
	}{
		{name: "marketing", enabled: false, personalized: false, alias: "agent:marketing"},
		{name: "sales", enabled: true, personalized: true, alias: "agent:sales"},
	} {
		cfg, err := compozyconfig.LoadForHome(homePaths, compozyconfig.WithProfile(test.name))
		if err != nil {
			t.Fatalf("LoadForHome(profile %q) error = %v", test.name, err)
		}
		assertPalette(test.name, cfg, test.enabled, test.personalized, test.alias)
	}

	workspaceAliases := map[string]string{"review": "agent:workspace"}
	workspaceEnabled, workspacePersonalized := true, false
	workspaceResult, err := service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: SectionRequest{
			Section: SectionCmdPalette, Scope: ScopeWorkspace, WorkspaceID: "ws-1",
		},
		CmdPalette: &CmdPaletteUpdate{
			FallbackAgentEnabled: &workspaceEnabled,
			Personalization:      &workspacePersonalized,
			Aliases:              &workspaceAliases,
		},
	})
	if err != nil {
		t.Fatalf("UpdateSection(cmd palette workspace) error = %v", err)
	}
	if workspaceResult.WriteTarget != WriteTargetWorkspaceConfig {
		t.Fatalf("workspace palette write target = %q", workspaceResult.WriteTarget)
	}
	workspaceConfig, err := compozyconfig.LoadForHome(
		homePaths,
		compozyconfig.WithWorkspaceRoot(workspaceRoot),
		compozyconfig.WithProfile("marketing"),
	)
	if err != nil {
		t.Fatalf("LoadForHome(workspace marketing) error = %v", err)
	}
	assertPalette("workspace marketing", workspaceConfig, true, false, "agent:workspace")

	shortcutAttempt := map[string]string{"session.new": "meta+shift+Space"}
	_, err = service.UpdateSection(ctx, SectionUpdateRequest{
		SectionRequest: SectionRequest{
			Section: SectionWindowManager, Scope: ScopeProfile, ProfileName: "marketing",
		},
		WindowManagerGlobalShortcuts: &shortcutAttempt,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("profile shortcut Settings write error = %v, want scope conflict", err)
	}

	profileConfigPath := filepath.Join(homePaths.ProfilesDir, "marketing", compozyconfig.ConfigName)
	profileConfig := readFile(t, profileConfigPath)
	writeFile(t, profileConfigPath, profileConfig+`

[window_manager.global_shortcuts]
"session.new" = "meta+shift+Space"
`)
	if _, loadErr := compozyconfig.LoadForHome(homePaths, compozyconfig.WithProfile("marketing")); loadErr == nil {
		t.Fatal("personal profile shortcut load error = nil, want profile_config_key_denied")
	} else {
		var validation compozyconfig.ValidationError
		if !errors.As(loadErr, &validation) || validation.Code != "profile_config_key_denied" {
			t.Fatalf("personal profile shortcut load error = %v", loadErr)
		}
	}
	writeFile(t, profileConfigPath, profileConfig)

	workspaceProfilePath := filepath.Join(
		workspaceRoot, compozyconfig.DirName, compozyconfig.ProfilesDirName, "marketing", compozyconfig.ConfigName,
	)
	writeFile(t, workspaceProfilePath, `[window_manager.global_shortcuts]
"session.new" = "meta+shift+Space"
`)
	if _, loadErr := compozyconfig.LoadForHome(
		homePaths,
		compozyconfig.WithWorkspaceRoot(workspaceRoot),
		compozyconfig.WithProfile("marketing"),
	); loadErr == nil {
		t.Fatal("workspace profile shortcut load error = nil, want profile_config_key_denied")
	} else {
		var validation compozyconfig.ValidationError
		if !errors.As(loadErr, &validation) || validation.Code != "profile_config_key_denied" {
			t.Fatalf("workspace profile shortcut load error = %v", loadErr)
		}
	}
	if removeErr := os.Remove(workspaceProfilePath); removeErr != nil {
		t.Fatalf("remove workspace profile shortcut fixture: %v", removeErr)
	}

	machineConfig, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		t.Fatalf("LoadForHome(machine config) error = %v", err)
	}
	if got, want := machineConfig.WindowManager.GlobalShortcuts["palette.summon.global"], "meta+shift+Space"; got != want {
		t.Fatalf("machine global shortcut = %q, want %q", got, want)
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
