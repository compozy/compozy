//go:build integration

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestToolMCPStaticPublicationAndBootRebuild(t *testing.T) {
	t.Run("Should Publish Static Resources And Rebuild On Boot", func(t *testing.T) {
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
		mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
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
		if err := registry.Install(manifest, extensionDir, checksum); err != nil {
			t.Fatalf("registry.Install() error = %v", err)
		}
		info, err := registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("registry.Get() error = %v", err)
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
		publishedConfig := aghconfig.Config{
			MCPServers: []aghconfig.MCPServer{{
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
				return driver.Trigger(ctx, kind, reason)
			},
			daemonConfigMCPDeclarationProvider(&publishedConfig, nil, nil, discardLogger()),
			extensionManifestToolMCPDeclarationProvider(
				registry,
				func() extensionRuntime { return runtime },
				nil,
				discardLogger(),
			),
		)
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
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

		servers, err := mcpStore.List(
			testutil.Context(t),
			toolMCPSyncActor(),
			resources.ResourceFilter{Source: &source},
		)
		if err != nil {
			t.Fatalf("mcpStore.List() error = %v", err)
		}
		if got, want := len(servers), 2; got != want {
			t.Fatalf("len(mcpStore.List()) = %d, want %d", got, want)
		}

		candidateConfig := aghconfig.Config{
			MCPServers: []aghconfig.MCPServer{{Name: "github", Command: "mcp-github"}},
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
		if got, want := mcpServerNames(servers), []string{"github", "kubectl"}; !slices.Equal(got, want) {
			t.Fatalf("candidate MCP servers = %#v, want %#v", got, want)
		}
		if got := publishedConfig.MCPServers[0].Name; got != "git" {
			t.Fatalf("published config MCP name = %q, want unchanged git", got)
		}
		if err := syncer.SyncConfig(testutil.Context(t), &aghconfig.Config{}); err != nil {
			t.Fatalf("syncer.SyncConfig(empty candidate) error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 1, 1)
		if err := syncer.Sync(testutil.Context(t)); err != nil {
			t.Fatalf("syncer.Sync(published config) error = %v", err)
		}
		assertToolMCPStoreCounts(t, toolStore, mcpStore, 1, 2)

		rebuiltToolCatalog := newResourceCatalog(cloneToolSpec)
		rebuiltMCPCatalog := newResourceCatalog(cloneDaemonMCPServer)
		bootDriver := newToolMCPIntegrationDriver(t, kernel, toolCodec, mcpCodec, rebuiltToolCatalog, rebuiltMCPCatalog)
		if err := bootDriver.RunBoot(testutil.Context(t)); err != nil {
			t.Fatalf("bootDriver.RunBoot() error = %v", err)
		}

		if got, want := len(rebuiltToolCatalog.Snapshot()), 1; got != want {
			t.Fatalf("len(rebuiltToolCatalog.Snapshot()) = %d, want %d", got, want)
		}
		if got, want := len(rebuiltMCPCatalog.Snapshot()), 2; got != want {
			t.Fatalf("len(rebuiltMCPCatalog.Snapshot()) = %d, want %d", got, want)
		}
	})
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
		mcpCodec, err := aghconfig.NewMCPServerResourceCodec()
		if err != nil {
			t.Fatalf("aghconfig.NewMCPServerResourceCodec() error = %v", err)
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
		if err := registry.Install(manifest, extensionDir, checksum); err != nil {
			t.Fatalf("registry.Install() error = %v", err)
		}
		info, err := registry.Get(manifest.Name)
		if err != nil {
			t.Fatalf("registry.Get() error = %v", err)
		}
		runtime := &toolMCPIntegrationRuntime{
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
		syncer := newToolMCPSourceSyncer(
			kernel,
			toolStore,
			toolCodec,
			mcpStore,
			mcpCodec,
			toolMCPSyncActor(),
			discardLogger(),
			func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
				return driver.Trigger(ctx, kind, reason)
			},
			extensionManifestToolMCPDeclarationProvider(
				registry,
				func() extensionRuntime { return runtime },
				nil,
				discardLogger(),
			),
		)

		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 1, 1)

		if err := registry.Disable(manifest.Name); err != nil {
			t.Fatalf("registry.Disable() error = %v", err)
		}
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 1, 0)

		if err := registry.Enable(manifest.Name); err != nil {
			t.Fatalf("registry.Enable() error = %v", err)
		}
		runtime.extension.Status.Registered = false
		syncAndAssertToolMCPStoreCounts(t, syncer, toolStore, mcpStore, 1, 0)

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
	extension *extensionpkg.Extension
}

func (r *toolMCPIntegrationRuntime) Start(context.Context) error  { return nil }
func (r *toolMCPIntegrationRuntime) Stop(context.Context) error   { return nil }
func (r *toolMCPIntegrationRuntime) Reload(context.Context) error { return nil }

func (r *toolMCPIntegrationRuntime) Get(name string) (*extensionpkg.Extension, error) {
	if r.extension == nil || r.extension.Info.Name != name {
		return nil, &extensionpkg.ExtensionNotFoundError{Name: name}
	}
	return r.extension, nil
}

func (r *toolMCPIntegrationRuntime) HookDeclarations(context.Context) ([]hookspkg.HookDecl, error) {
	return nil, nil
}

func newToolMCPIntegrationDriver(
	t *testing.T,
	kernel resources.RawStore,
	toolCodec resources.KindCodec[toolspkg.Tool],
	mcpCodec resources.KindCodec[aghconfig.MCPServer],
	toolCatalog *resourceCatalog[toolspkg.Tool],
	mcpCatalog *resourceCatalog[aghconfig.MCPServer],
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
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
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
min_agh_version = "0.5.0"

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
	mcpStore resources.Store[aghconfig.MCPServer],
	wantTools int,
	wantMCPServers int,
) {
	t.Helper()

	if err := syncer.Sync(testutil.Context(t)); err != nil {
		t.Fatalf("syncer.Sync() error = %v", err)
	}
	assertToolMCPStoreCounts(t, toolStore, mcpStore, wantTools, wantMCPServers)
}

func mcpServerNames(records []resources.Record[aghconfig.MCPServer]) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Spec.Name)
	}
	slices.Sort(names)
	return names
}
