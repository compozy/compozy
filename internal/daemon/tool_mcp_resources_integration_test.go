//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/extensionmcp"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

// Invariant: extension auth policy survives desired-resource persistence and boot reconstruction.
// Owner: daemon resource publication, canonical real SQLite integration suite.
func TestToolMCPStaticPublicationAndBootRebuild(t *testing.T) {
	t.Parallel()
	// Invariant: explicit profile installations publish only into their owning
	// profile, including default; owner: daemon resource projection, canonical integration suite.
	t.Run("Should publish explicit profile installations from the real manager", func(t *testing.T) {
		t.Parallel()
		testInstalledProfileMCPPublication(t)
	})
	t.Run("Should Publish Static Resources And Rebuild On Boot", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}

		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		toolStore, err := resources.NewStore(kernel, toolCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(tool) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}

		registry := extensionpkg.NewRegistry(db.DB())
		extensionDir := writeToolMCPIntegrationExtension(t)
		manifestPath := filepath.Join(extensionDir, "extension.toml")
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		raw = append(raw, []byte(`
[resources.mcp_servers.remote]
transport = "http"
url = "https://mcp.example.com/mcp?workspace="
default_scope = "global"
[resources.mcp_servers.remote.auth]
method = "oauth"
registration = "dynamic"
issuer_url = "https://issuer.example.com"
scopes = ["tools.read"]
[[inputs]]
id = "workspace"
prompt = "Workspace"
type = "identifier"
required = true
binding = { type = "url_query", name = "workspace" }
`)...)
		if err := os.WriteFile(manifestPath, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		manifest, err := extensionpkg.LoadManifest(extensionDir)
		if err != nil {
			t.Fatalf("extensionpkg.LoadManifest() error = %v", err)
		}
		checksum, err := extensionpkg.ComputeDirectoryChecksum(extensionDir)
		if err != nil {
			t.Fatalf("extensionpkg.ComputeDirectoryChecksum() error = %v", err)
		}
		if err := registry.Install(manifest, extensionDir, checksum); err != nil {
			t.Fatalf("registry.Install() error = %v", err)
		}
		info, err := registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("registry.Get() error = %v", err)
		}

		// Invariant: publication reloads the exact instance URL input from SQLite without process env.
		// Owner: daemon input application; canonical publication and boot integration suite.
		input := extensioninput.Record{
			Type:      "identifier",
			Value:     json.RawMessage(`"team-a"`),
			Active:    true,
			UpdatedAt: time.Now().UTC(),
		}
		if err := db.ExtensionInputs.Apply(testutil.Context(t), extensioninput.Instance{
			Extension: manifest.Name, ProfileID: store.DefaultProfileID,
		}, []extensioninput.Mutation{{InputID: "workspace", After: &input}}); err != nil {
			t.Fatal(err)
		}
		initialToolCatalog := newResourceCatalog(cloneToolSpec)
		initialMCPServerCatalog := newResourceCatalog(cloneDaemonMCPServer)
		driver := newToolMCPIntegrationDriver(
			t,
			kernel,
			toolCodec,
			mcpCodec,
			initialToolCatalog,
			initialMCPServerCatalog,
		)

		runtime := &toolMCPIntegrationRuntime{
			registry: registry,
			extension: &extensionpkg.Extension{
				Info:     *info,
				Manifest: manifest,
				RootDir:  extensionDir,
				Status: extensionpkg.ExtensionStatus{
					Name:       info.Name,
					Version:    info.Version,
					Source:     info.Source,
					Enabled:    info.Enabled,
					Registered: true,
				},
			},
		}
		publishedConfig := compozyconfig.Config{
			MCPServers: []compozyconfig.MCPServer{{
				Name:    "git",
				Command: "npx",
				Args:    []string{"@modelcontextprotocol/server-git"},
			}},
		}
		syncer := newToolMCPSourceSyncerWithConfigProvider(
			kernel,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				ticket, err := driver.Trigger(ctx, kind, reason)
				if err != nil {
					return err
				}
				return driver.WaitForIdle(ctx, ticket)
			},
			daemonConfigMCPDeclarationProvider(&publishedConfig, nil, nil, discardLogger()),
			extensionManifestToolMCPDeclarationProvider(
				registry,
				func() extensionRuntime { return runtime },
				nil,
				defaultToolMCPProfileCatalog{},
				extensionInputReader{inputs: db.ExtensionInputs},
			),
		)
		syncer.prepareMCP = extensionMCPPublicationPreparer(&bootState{extensionMCP: db.ExtensionMCP})
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}

		// Invariant: actual persisted/projected extension declarations reach Settings without copying them to manual config.
		// Owner: publication-to-Settings integration; canonical tagged suite.
		settingsAdapter := settingsMCPExtensionDefinitions{
			state: &bootState{extensionMCP: db.ExtensionMCP, mcpServerCatalog: initialMCPServerCatalog},
		}
		settingsHome := testHomePaths(t)
		settingsProfiles, err := profilepkg.NewManager(
			profilepkg.WithStore(db),
			profilepkg.WithHomePaths(settingsHome),
			profilepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatal(err)
		}
		authManager, err := newSettingsMCPAuthManagerWithConfig(db, nil, nil, nil, compozyconfig.MCPOAuthConfig{})
		if err != nil {
			t.Fatal(err)
		}
		settingsAdapter.auth = &settingsRuntimeSurface{mcpAuthManager: authManager}
		for _, owner := range []string{"manual", "extension:" + manifest.Name} {
			target := mcpauth.Target{Scope: mcpauth.ScopeUser, Owner: owner, ServerName: "remote"}
			cfg, err := mcpauth.ServerConfigFromMCP(testutil.Context(t), target, compozyconfig.MCPServer{
				Name:      "remote",
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.example.com/mcp?workspace=team-a",
				Auth: compozyconfig.MCPAuthConfig{
					Registration: compozyconfig.MCPAuthRegistrationAuto,
					IssuerURL:    "https://issuer.example.com",
				},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SaveMCPAuthToken(testutil.Context(t), mcpauth.TokenRecord{
				Target: target, DefinitionFingerprint: fingerprint,
				Issuer:      "https://issuer.example.com",
				ClientID:    "fixture",
				AccessToken: "fixture-secret",
				TokenType:   "Bearer",
			}); err != nil {
				t.Fatal(err)
			}
		}

		settingsService, err := settingspkg.NewService(
			settingsHome,
			settingspkg.Dependencies{
				MCPExtensionManagement:  settingsAdapter,
				MCPExtensions:           settingsAdapter,
				ProfileResolver:         settingsProfiles,
				AttentionWorkspaceMutes: db,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		collection, err := settingsService.ListCollection(
			testutil.Context(t),
			settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers},
		)
		if err != nil || len(collection.MCPServers) != 2 {
			t.Fatalf("published extensions missing from Settings: %#v %v", collection.MCPServers, err)
		}
		for _, item := range collection.MCPServers {
			if item.Owner != "extension:"+manifest.Name || item.RuntimeName == "" ||
				item.SourceMetadata.EffectiveSource.Kind != settingspkg.SourceKindExtension ||
				len(item.SourceMetadata.AvailableTargets) != 0 {
				t.Fatalf("published Settings row lost identity: %#v", item)
			}
		}
		_, err = settingsService.PutCollectionItem(testutil.Context(t), settingspkg.CollectionItemPutRequest{
			CollectionRequest: settingspkg.CollectionRequest{
				Collection: settingspkg.CollectionMCPServers, Owner: "manual",
			},
			Name:      "remote",
			MCPServer: &compozyconfig.MCPServer{Name: "remote", Command: "manual-mcp"},
		})
		if !errors.Is(err, settingspkg.ErrMCPServerNameTaken) {
			t.Fatalf("real published allocation did not protect manual mutation: %v", err)
		}

		// Invariant: overrides validate against the declaration, publish live, and reset without reallocating the runtime name.
		// Owner: daemon override transaction; canonical real SQLite/resource publisher integration suite.
		settingsAdapter.state.toolMCPResources = syncer
		settingsAdapter.state.deps.Extensions = &daemonExtensionService{
			runtime:   runtime,
			inputs:    db.ExtensionInputs,
			lifecycle: newExtensionLifecycleCoordinator(),
		}
		overrideRequest := settingspkg.MCPAuthTargetRequest{
			Scope: settingspkg.ScopeUser,
			Name:  "remote",
			Owner: "extension:" + manifest.Name,
		}
		for _, invalid := range []extensionmcp.Override{
			{Env: map[string]string{"REGION": "eu"}},
			{Headers: map[string]string{"Authorization": "Bearer forbidden"}},
		} {
			if _, err := settingsAdapter.UpdateMCPExtensionOverride(
				testutil.Context(t),
				overrideRequest,
				invalid,
			); !errors.Is(
				err,
				settingspkg.ErrValidation,
			) {
				t.Fatalf("invalid override bypassed actual HTTP/auth policy: %v", err)
			}
		}
		edited, err := settingsAdapter.UpdateMCPExtensionOverride(
			testutil.Context(t),
			overrideRequest,
			extensionmcp.Override{URL: "https://mcp.example.com/changed"},
		)
		if err != nil || edited.Server.URL != "https://mcp.example.com/changed" ||
			edited.Server.RuntimeName != "remote" {
			t.Fatalf("override not applied: %#v %v", edited, err)
		}
		_, published, found, err := settingsAdapter.ResolveMCPExtensionDefinition(testutil.Context(t), overrideRequest)
		if err != nil || !found || published.URL != edited.Server.URL {
			t.Fatalf("override not reconciled: %#v %v", published, err)
		}
		reset, err := settingsAdapter.UpdateMCPExtensionOverride(
			testutil.Context(t),
			overrideRequest,
			extensionmcp.Override{},
		)
		if err != nil || reset.Server.URL != "https://mcp.example.com/mcp?workspace=team-a" ||
			reset.Server.RuntimeName != "remote" {
			t.Fatalf("reset did not restore declaration/runtime name: %#v %v", reset, err)
		}

		if _, err := db.GetMCPAuthToken(
			testutil.Context(t),
			mcpauth.Target{Scope: mcpauth.ScopeUser, Owner: "extension:" + manifest.Name, ServerName: "remote"},
		); !errors.Is(
			err,
			mcpauth.ErrTokenNotFound,
		) {
			t.Fatalf("override retained its previous auth state: %v", err)
		}
		if _, err := db.GetMCPAuthToken(
			testutil.Context(t),
			mcpauth.Target{Scope: mcpauth.ScopeUser, Owner: "manual", ServerName: "remote"},
		); err != nil {
			t.Fatalf("extension override deleted manual credentials: %v", err)
		}
		mutation, err := settingsService.PutCollectionItem(testutil.Context(t), settingspkg.CollectionItemPutRequest{
			CollectionRequest: settingspkg.CollectionRequest{
				Collection: settingspkg.CollectionMCPServers,
				Owner:      "extension:" + manifest.Name,
			},
			Name:      "remote",
			MCPServer: &compozyconfig.MCPServer{URL: "https://mcp.example.com/settings-edit"},
		})
		if err != nil || mutation.MCPServer == nil ||
			mutation.MCPServer.URL != "https://mcp.example.com/settings-edit" {
			t.Fatalf("Settings override dispatch failed: %#v %v", mutation, err)
		}
		mutation, err = settingsService.DeleteCollectionItem(
			testutil.Context(t),
			settingspkg.CollectionItemDeleteRequest{
				CollectionRequest: settingspkg.CollectionRequest{
					Collection: settingspkg.CollectionMCPServers,
					Owner:      "extension:" + manifest.Name,
				},
				Name: "remote",
			},
		)
		if err != nil || mutation.MCPServer == nil ||
			mutation.MCPServer.URL != "https://mcp.example.com/mcp?workspace=team-a" {
			t.Fatalf("Settings reset dispatch failed: %#v %v", mutation, err)
		}

		// Invariant: reset publication and persisted OAuth retirement are observable through native diagnostics.
		// Owner: real publication/credential integration; canonical tagged suite.
		executor, err := mcppkg.NewMCPCallExecutor(
			newDaemonMCPServerResolver(settingsAdapter.state),
			mcppkg.WithTokenStore(db),
		)
		if err != nil {
			t.Fatal(err)
		}
		nativeRegistry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Sessions: nativeNetworkTestSessionManager(""),
			MCPAuth:  func() toolspkg.MCPAuthStatusProvider { return executor },
			Settings: func() core.SettingsService { return settingsService },
		}, nativeApproveAllPolicyInputs())
		for _, toolID := range []toolspkg.ToolID{toolspkg.ToolIDMCPAuthStatus, toolspkg.ToolIDMCPStatus} {
			input, err := json.Marshal(
				map[string]string{"server_name": "remote", "owner": "extension:" + manifest.Name},
			)
			if err != nil {
				t.Fatal(err)
			}
			result, err := nativeRegistry.Call(
				testutil.Context(t),
				toolspkg.Scope{SessionID: "sess-1"},
				toolspkg.CallRequest{ToolID: toolID, Input: input},
			)
			if err != nil {
				t.Fatalf("native status after reset: %v", err)
			}
			var payload struct {
				Status toolspkg.MCPAuthStatus `json:"status"`
				Auth   toolspkg.MCPAuthStatus `json:"auth"`
			}
			if toolID == toolspkg.ToolIDMCPStatus {
				var statusPayload mcpStatusPayload
				if err := json.Unmarshal(result.Structured, &statusPayload); err != nil {
					t.Fatal(err)
				}
				payload.Auth = statusPayload.Auth
			} else {
				if err := json.Unmarshal(result.Structured, &payload); err != nil {
					t.Fatal(err)
				}
				payload.Auth = payload.Status
			}
			if payload.Auth.Owner != "extension:"+manifest.Name || payload.Auth.Status != "needs_login" ||
				payload.Auth.TokenPresent {
				t.Fatalf("native status missed owned credential retirement: %#v", payload.Auth)
			}
		}

		source := toolMCPSyncActor().Source
		tools, err := toolStore.List(testutil.Context(t), toolMCPSyncActor(), resources.ResourceFilter{Source: &source})
		if err != nil {
			t.Fatalf("toolStore.List() error = %v", err)
		}
		if got, want := len(tools), 1; got != want {
			t.Fatalf("len(toolStore.List()) = %d, want %d", got, want)
		}
		if got, want := tools[0].Spec.ID, toolspkg.ToolID("ext__static_tool_mcp__lookup"); got != want {
			t.Fatalf("tools[0].Spec.ID = %q, want %q", got, want)
		}
		if got, want := tools[0].Spec.Source.Kind, toolspkg.ToolSourceExtension; got != want {
			t.Fatalf("tools[0].Spec.Source.Kind = %q, want %q", got, want)
		}
		extensionResourceOwner := extensionOwner(manifest.Name).Normalize()
		publicationSource := toolMCPSyncActor().Source.Normalize()
		if got, want := tools[0].ID, "extension/"+manifest.Name+"/tool/"+tools[0].Spec.ID.String()+"/profile/"+store.DefaultProfileID; got != want {
			t.Fatalf("tools[0].ID = %q, want source-key ID %q", got, want)
		}
		if got, want := tools[0].Owner.Normalize(), extensionResourceOwner; got != want {
			t.Fatalf("tools[0].Owner = %#v, want extension owner %#v", got, want)
		}
		if got, want := tools[0].Scope, (resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID}); got != want {
			t.Fatalf("tools[0].Scope = %#v, want %#v", got, want)
		}
		if got, want := tools[0].Source.Normalize(), publicationSource; got != want {
			t.Fatalf("tools[0].Source = %#v, want daemon source %#v", got, want)
		}

		servers, err := mcpStore.List(
			testutil.Context(t),
			toolMCPSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("mcpStore.List() error = %v", err)
		}
		if got, want := len(servers), 3; got != want {
			t.Fatalf("len(mcpStore.List()) = %d, want %d", got, want)
		}
		remote := requireMCPServerRecord(t, servers, "remote")
		// Invariant: persisted publication retains allocated runtime identity and declaration owner.
		// Owner: daemon publication; canonical real SQLite integration suite.
		if remote.Spec.Owner != "extension:"+manifest.Name || remote.Spec.RuntimeName != "remote" {
			t.Fatalf("published MCP lost its allocated identity: %#v", remote.Spec)
		}
		if remote.Spec.URL != "https://mcp.example.com/mcp?workspace=team-a" {
			t.Fatalf("stored URL input was not applied: %s", remote.Spec.URL)
		}
		if remote.Spec.Auth.Registration != compozyconfig.MCPAuthRegistrationAuto ||
			remote.Spec.Auth.IssuerURL != "https://issuer.example.com" ||
			!slices.Equal(remote.Spec.Auth.Scopes, []string{"tools.read"}) {
			t.Fatalf("persisted remote auth = %#v", remote.Spec.Auth)
		}
		if remote.Scope.Kind != resources.ResourceScopeKindProfile || remote.Scope.ID != store.DefaultProfileID {
			t.Fatalf("manifest default broadened installed scope: %#v", remote.Scope)
		}
		extensionServer := requireMCPServerRecord(t, servers, "kubectl")
		if got, want := extensionServer.ID, "extension/"+manifest.Name+"/mcp_server/kubectl/profile/"+store.DefaultProfileID; got != want {
			t.Fatalf("extension MCP ID = %q, want source-key ID %q", got, want)
		}
		if got, want := extensionServer.Owner.Normalize(), extensionResourceOwner; got != want {
			t.Fatalf("extension MCP owner = %#v, want %#v", got, want)
		}
		if got, want := extensionServer.Scope, (resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID}); got != want {
			t.Fatalf("extension MCP scope = %#v, want %#v", got, want)
		}
		if got, want := extensionServer.Source.Normalize(), publicationSource; got != want {
			t.Fatalf("extension MCP source = %#v, want %#v", got, want)
		}

		configServer := requireMCPServerRecord(t, servers, "git")
		configEncoded, err := mcpCodec.Encode(configServer.Spec)
		if err != nil {
			t.Fatalf("mcpCodec.Encode(config server) error = %v", err)
		}
		configSourceKey := "config/global/git"
		wantConfigID := contentAddressedManagedPublicationID(
			mcpServerManagedIDPrefix,
			resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			configSourceKey,
			configEncoded,
		)
		if got := configServer.ID; got != wantConfigID || got == configSourceKey {
			t.Fatalf("config MCP ID = %q, want stable content-addressed ID %q", got, wantConfigID)
		}
		if got, want := configServer.Owner.Normalize(), managedDraftOwner(toolMCPSyncActor(), nil); got != want {
			t.Fatalf("config MCP owner = %#v, want daemon owner %#v", got, want)
		}
		if got, want := configServer.Source.Normalize(), publicationSource; got != want {
			t.Fatalf("config MCP source = %#v, want daemon source %#v", got, want)
		}

		candidateConfig := compozyconfig.Config{
			MCPServers: []compozyconfig.MCPServer{{Name: "github", Command: "mcp-github"}},
		}
		if err := syncer.SyncConfig(testutil.Context(t), &candidateConfig); err != nil {
			t.Fatalf("syncer.SyncConfig(candidate) error = %v", err)
		}
		servers, err = mcpStore.List(
			testutil.Context(t),
			toolMCPSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("mcpStore.List(candidate) error = %v", err)
		}
		if got, want := mcpServerNames(servers), []string{"github", "kubectl", "remote"}; !slices.Equal(got, want) {
			t.Fatalf("candidate MCP servers = %#v, want %#v", got, want)
		}
		if got := publishedConfig.MCPServers[0].Name; got != "git" {
			t.Fatalf("published config MCP name = %q, want unchanged git", got)
		}
		if err := syncer.SyncConfig(testutil.Context(t), &compozyconfig.Config{}); err != nil {
			t.Fatalf("syncer.SyncConfig(empty candidate) error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 1, 2)
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync(published config) error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 1, 3)

		rebuiltToolCatalog := newResourceCatalog(cloneToolSpec)
		rebuiltMCPCatalog := newResourceCatalog(cloneDaemonMCPServer)
		bootDriver := newToolMCPIntegrationDriver(t, kernel, toolCodec, mcpCodec, rebuiltToolCatalog, rebuiltMCPCatalog)
		if err := bootDriver.RunBoot(testutil.Context(t)); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		if got, want := len(rebuiltToolCatalog.Snapshot()), 1; got != want {
			t.Fatalf("len(rebuiltToolCatalog.Snapshot()) = %d, want %d", got, want)
		}
		if got, want := len(rebuiltMCPCatalog.Snapshot()), 3; got != want {
			t.Fatalf("len(rebuiltMCPCatalog.Snapshot()) = %d, want %d", got, want)
		}
		rebuilt := requireMCPServerRecord(t, rebuiltMCPCatalog.Snapshot(), "remote")
		if rebuilt.Spec.URL != remote.Spec.URL || rebuilt.Spec.Auth.IssuerURL != remote.Spec.Auth.IssuerURL ||
			!slices.Equal(rebuilt.Spec.Auth.Scopes, remote.Spec.Auth.Scopes) ||
			rebuilt.Scope != remote.Scope {
			t.Fatalf("rebuilt remote = %#v", rebuilt)
		}

	})
}

func testInstalledProfileMCPPublication(t *testing.T) {
	t.Helper()
	db := openDaemonTestGlobalDB(t)
	profiles, err := profilepkg.NewManager(profilepkg.WithStore(db), profilepkg.WithHomePaths(testHomePaths(t)))
	if err != nil {
		t.Fatal(err)
	}
	marketing, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "finance"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(`[extension]
name = "installed-profile-mcp"
version = "1.0.0"
min_compozy_version = "0.5.0"
[resources.mcp_servers.lookup]
transport = "http"
url = "https://mcp.example.invalid/mcp"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	registry := extensionpkg.NewRegistry(db.DB())
	if err := registry.Install(manifest, dir, checksum, extensionpkg.WithInstallScope(extensionpkg.InstallationScope{
		ProfileID: marketing.ID,
	})); err != nil {
		t.Fatal(err)
	}
	if err := registry.AttachInstallation(t.Context(), manifest.Name, extensionpkg.InstallationScope{
		ProfileID: store.DefaultProfileID,
	}); err != nil {
		t.Fatal(err)
	}
	manager := extensionpkg.NewManager(registry, extensionpkg.WithProfileNameResolver(profiles))
	if err := manager.Start(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()
		if err := manager.Stop(ctx); err != nil {
			t.Error(err)
		}
	})
	provider := extensionManifestToolMCPDeclarationProvider(registry, func() extensionRuntime { return manager },
		nil, profiles, extensionInputReader{})
	desired, err := provider(t.Context())
	if err != nil || len(desired.mcpServers) != 2 {
		t.Fatalf("profile MCP declarations = %#v, %v", desired, err)
	}
	owners := make([]string, 0, 2)
	for _, server := range desired.mcpServers {
		if server.scope.Kind != resources.ResourceScopeKindProfile || server.spec.Owner != "extension:"+manifest.Name {
			t.Fatalf("profile MCP declaration widened its owner: %#v", server)
		}
		owners = append(owners, server.scope.ID)
	}
	slices.Sort(owners)
	want := []string{store.DefaultProfileID, marketing.ID}
	slices.Sort(want)
	if !slices.Equal(owners, want) {
		t.Fatalf("MCP profile owners = %#v, want %#v", owners, want)
	}
	legacy, err := extensionResourceSnapshots(registry, manager, discardLogger())
	if err != nil || len(legacy) != 1 || legacy[0].scope.Kind != resources.ResourceScopeKindProfile ||
		legacy[0].scope.ID != store.DefaultProfileID {
		t.Fatalf("default-only publisher widened profile scope: %#v, %v", legacy, err)
	}
}

func TestToolMCPStaticPublicationExtensionLifecycle(t *testing.T) {
	t.Run("Should Publish Tools Across Extension Lifecycle", func(t *testing.T) {
		db := openDaemonTestGlobalDB(t)
		kernel, err := resources.NewKernel(db.DB())
		if err != nil {
			t.Fatalf("resources.NewKernel() error = %v", err)
		}
		toolCodec, err := toolspkg.NewResourceCodec()
		if err != nil {
			t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
		}
		toolStore, err := resources.NewStore(kernel, toolCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(tool) error = %v", err)
		}
		mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
		}
		mcpStore, err := resources.NewStore(kernel, mcpCodec)
		if err != nil {
			t.Fatalf("resources.NewStore(mcp) error = %v", err)
		}
		registry := extensionpkg.NewRegistry(db.DB())
		extensionDir := writeToolMCPIntegrationExtension(t)
		manifest, err := extensionpkg.LoadManifest(extensionDir)
		if err != nil {
			t.Fatalf("extensionpkg.LoadManifest() error = %v", err)
		}
		checksum, err := extensionpkg.ComputeDirectoryChecksum(extensionDir)
		if err != nil {
			t.Fatalf("extensionpkg.ComputeDirectoryChecksum() error = %v", err)
		}
		// Invariant: the requested reservation is consumed by actual desired-state publication.
		// Owner: publication integration; canonical tagged suite.
		allocationService := &daemonExtensionService{registry: registry, mcpAllocations: db.ExtensionMCP,
			resourceStore: kernel, resourceActor: resourceReconcileActor()}
		if _, err := allocationService.prepareInstallMCPAllocations(t.Context(), preparedDaemonExtensionInstall{
			name: manifest.Name, manifest: manifest,
		}, "requested-server"); err != nil {
			t.Fatal(err)
		}
		if err := registry.Install(manifest, extensionDir, checksum); err != nil {
			t.Fatalf("registry.Install() error = %v", err)
		}
		info, err := registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("registry.Get() error = %v", err)
		}
		runtime := &toolMCPIntegrationRuntime{
			registry: registry,
			extension: &extensionpkg.Extension{
				Info:     *info,
				Manifest: manifest,
				RootDir:  extensionDir,
				Status: extensionpkg.ExtensionStatus{
					Name:       info.Name,
					Version:    info.Version,
					Source:     info.Source,
					Enabled:    info.Enabled,
					Registered: true,
					Active:     true,
					Healthy:    true,
				},
			},
		}
		toolCatalog := newResourceCatalog(cloneToolSpec)
		mcpCatalog := newResourceCatalog(cloneDaemonMCPServer)
		driver := newToolMCPIntegrationDriver(t, kernel, toolCodec, mcpCodec, toolCatalog, mcpCatalog)
		syncer := newToolMCPSourceSyncerWithConfigProvider(
			kernel,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				_, err := driver.Trigger(ctx, kind, reason)
				return err
			},
			nil,
			extensionManifestToolMCPDeclarationProvider(
				registry,
				func() extensionRuntime { return runtime },
				nil,
				defaultToolMCPProfileCatalog{},
				extensionInputReader{},
			),
		)

		syncer.prepareMCP = extensionMCPPublicationPreparer(&bootState{extensionMCP: db.ExtensionMCP})

		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 1, 1)
		publishedServers, err := mcpStore.List(t.Context(), toolMCPSyncActor(), resources.ResourceFilter{})
		if err != nil || len(publishedServers) != 1 || publishedServers[0].Spec.RuntimeName != "requested-server" ||
			publishedServers[0].Spec.Owner != "extension:"+manifest.Name {
			t.Fatalf("publication lost requested identity: %#v %v", publishedServers, err)
		}

		if err := registry.Disable(manifest.Name); err != nil {
			t.Fatalf("registry.Disable() error = %v", err)
		}
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 0, 0)

		if err := registry.Enable(manifest.Name); err != nil {
			t.Fatalf("registry.Enable() error = %v", err)
		}
		runtime.extension.Status.Registered = false
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 0, 0)

		runtime.extension.Status.Registered = true
		runtime.extension.Status.Healthy = false
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 1, 1)

		runtime.extension = nil
		if err := registry.Uninstall(manifest.Name); err != nil {
			t.Fatalf("registry.Uninstall() error = %v", err)
		}
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 0, 0)
	})
}

type toolMCPIntegrationRuntime struct {
	registry  *extensionpkg.Registry
	extension *extensionpkg.Extension
}

var _ extensionRuntime = (*toolMCPIntegrationRuntime)(nil)

func (r *toolMCPIntegrationRuntime) Start(context.Context) error  { return nil }
func (r *toolMCPIntegrationRuntime) Stop(context.Context) error   { return nil }
func (r *toolMCPIntegrationRuntime) Reload(context.Context) error { return nil }

func (r *toolMCPIntegrationRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if r.extension == nil || r.extension.Info.Name != name {
		return nil, &extensionpkg.ExtensionNotFoundError{Name: name}
	}
	return r.extension, nil
}

func (r *toolMCPIntegrationRuntime) ProjectForProfile(
	context.Context,
	extensionpkg.InstanceKey,
	extensionpkg.ProfileLens,
) (*extensionpkg.Extension, bool, error) {
	if r.extension == nil {
		return nil, false, &extensionpkg.ExtensionNotFoundError{Name: "extension"}
	}
	projected := *r.extension
	enabled := projected.Info.Enabled
	if r.registry != nil {
		resolved, err := r.registry.IsEnabledForProfile(projected.Info.Name, store.DefaultProfileID)
		if err != nil {
			return nil, false, err
		}
		enabled = resolved
	}
	projected.Info.Enabled = enabled
	projected.Status.Enabled = enabled
	return &projected, enabled, nil
}

type defaultToolMCPProfileCatalog struct{}

func (defaultToolMCPProfileCatalog) List(context.Context) ([]profilepkg.WithCounts, error) {
	return []profilepkg.WithCounts{{
		Profile: profilepkg.Profile{ID: store.DefaultProfileID, Name: "default", State: profilepkg.StateActive},
	}}, nil
}

func (r *toolMCPIntegrationRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func (r *toolMCPIntegrationRuntime) InspectPackageResources(
	_ context.Context,
	name string,
) (*extensionpkg.Extension, error) {
	return r.Get(name)
}

func newToolMCPIntegrationDriver(
	t *testing.T,
	kernel resources.RawStore,
	toolCodec resources.KindCodec[toolspkg.Tool],
	mcpCodec resources.KindCodec[compozyconfig.MCPServer],
	toolCatalog *resourceCatalog[toolspkg.Tool],
	mcpCatalog *resourceCatalog[compozyconfig.MCPServer],
) resources.ReconcileDriver {
	t.Helper()

	toolRegistration, err := resources.NewTypedProjectorRegistration(toolCodec, newToolProjector(toolCatalog))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(tool) error = %v", err)
	}
	mcpRegistration, err := resources.NewTypedProjectorRegistration(mcpCodec, newMCPServerProjector(mcpCatalog))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(mcp) error = %v", err)
	}
	driver, err := resources.NewReconcileDriver(
		kernel,
		resources.MutationActor{
			Kind: resources.MutationActorKindDaemon,
			ID:   "tool-mcp-integration",
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("daemon"),
				ID:   "tool-mcp-integration",
			},
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		},
		[]resources.ProjectorRegistration{toolRegistration, mcpRegistration},
		resources.WithReconcileLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("resources.NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		if err := driver.Close(context.Background()); err != nil {
			t.Fatalf("driver.Close() error = %v", err)
		}
	})
	return driver
}

func writeToolMCPIntegrationExtension(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	manifest := `[extension]
name = "static-tool-mcp"
version = "0.1.0"
min_compozy_version = "0.5.0"

[resources.tools.lookup]
description = "Search extension data"
read_only = true

[resources.tools.lookup.backend]
kind = "extension_host"
handler = "lookup"

[resources.mcp_servers.kubectl]
command = "./bin/mcp-kubectl"
args = ["--cluster", "prod"]
`
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	return dir
}

func syncAndAssertToolMCPStoreCounts(
	t *testing.T,
	syncer toolMCPPublisher,
	toolStore resources.Store[toolspkg.Tool],
	mcpStore resources.Store[compozyconfig.MCPServer],
	wantTools int,
	wantMCPServers int,
) {
	t.Helper()

	if err := syncer.Sync(testutil.Context(t)); err != nil {
		t.Fatalf("syncer.Sync() error = %v", err)
	}
	assertToolMCPStoreCounts(t, toolStore, mcpStore, wantTools, wantMCPServers)
}

func mcpServerNames(records []resources.Record[compozyconfig.MCPServer]) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Spec.Name)
	}
	slices.Sort(names)
	return names
}

func requireMCPServerRecord(
	t *testing.T,
	records []resources.Record[compozyconfig.MCPServer],
	name string,
) resources.Record[compozyconfig.MCPServer] {
	t.Helper()

	for _, record := range records {
		if record.Spec.Name == name {
			return record
		}
	}
	t.Fatalf("MCP server %q not found in persisted records", name)
	return resources.Record[compozyconfig.MCPServer]{}
}
