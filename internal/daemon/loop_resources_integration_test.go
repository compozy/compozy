//go:build integration

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	devcycle "github.com/compozy/agh/extensions/dev-cycle"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestLoopSourceSyncerIntegrationShouldProjectFSPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should project global and workspace records with workspace precedence", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := loopIntegrationHome(t)
		if _, _, err := looppkg.WriteDefinition(
			homePaths.LoopsDir,
			[]byte(testLoopYAML("software-delivery", "global shadow")),
			looppkg.WriteDefinitionOptions{Source: looppkg.SourceUser},
		); err != nil {
			t.Fatalf("WriteDefinition(global) error = %v", err)
		}

		workspaceRoot := t.TempDir()
		workspaceLoopsDir := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.LoopsDirName)
		if _, _, err := looppkg.WriteDefinition(
			workspaceLoopsDir,
			[]byte(testLoopYAML("software-delivery", "workspace shadow")),
			looppkg.WriteDefinitionOptions{Source: looppkg.SourceWorkspace},
		); err != nil {
			t.Fatalf("WriteDefinition(workspace) error = %v", err)
		}

		harness := newLoopIntegrationHarness(t)
		syncer := newLoopSourceSyncer(
			harness.store,
			harness.codec,
			loopSyncActor(),
			discardLogger(),
			nil,
			func(context.Context) ([]loopPublicationInput, error) {
				var desired []loopPublicationInput
				global, err := scanLoopResourceDir(ctx, homePaths.LoopsDir, looppkg.SourceUser)
				if err != nil {
					return nil, err
				}
				appendLoopResources(
					&desired,
					resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
					"test/global",
					global,
				)
				workspace, err := scanLoopResourceDir(ctx, workspaceLoopsDir, looppkg.SourceWorkspace)
				if err != nil {
					return nil, err
				}
				appendLoopResources(
					&desired,
					resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
					"test/workspace/ws-1",
					workspace,
				)
				return desired, nil
			},
		)
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}
		if err := harness.driver.RunBoot(ctx); err != nil {
			t.Fatalf("driver.RunBoot() error = %v", err)
		}

		records := harness.catalog.Snapshot()
		if got, wantMin := len(records), 2; got < wantMin {
			t.Fatalf("len(catalog.Snapshot()) = %d, want at least %d", got, wantMin)
		}
		resolved := looppkg.ResolveEffectiveResources(records, "ws-1")
		found := false
		for _, record := range resolved {
			if record.Spec.Name != "software-delivery" {
				continue
			}
			found = true
			if got, want := record.Spec.Description, "workspace shadow"; got != want {
				t.Fatalf("resolved software-delivery description = %q, want %q", got, want)
			}
		}
		if !found {
			t.Fatal("resolved software-delivery not found")
		}
	})

	t.Run("Should resolve extension contributed loop below user global override", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		extensionSpec := testLoopSpec(t, "extension-loop", looppkg.SourceMarketplace)
		extensionSpec.Description = "marketplace extension"
		extensionSpec.InstalledFromExtension = "market-ext"
		userSpec := testLoopSpec(t, "extension-loop", looppkg.SourceUser)
		userSpec.Description = "user override"

		harness := newLoopIntegrationHarness(t)
		syncer := newLoopSourceSyncer(
			harness.store,
			harness.codec,
			loopSyncActor(),
			discardLogger(),
			nil,
			func(context.Context) ([]loopPublicationInput, error) {
				return []loopPublicationInput{
					{
						sourceKey: "extension/market-ext/loops/extension-loop",
						scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
						spec:      extensionSpec,
					},
					{
						sourceKey: "global/user/extension-loop",
						scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
						spec:      userSpec,
					},
				}, nil
			},
		)
		if err := syncer.Sync(ctx); err != nil {
			t.Fatalf("syncer.Sync() error = %v", err)
		}
		if err := harness.driver.RunBoot(ctx); err != nil {
			t.Fatalf("driver.RunBoot() error = %v", err)
		}
		resolved := looppkg.ResolveEffectiveResources(harness.catalog.Snapshot(), "")
		if got, want := len(resolved), 1; got != want {
			t.Fatalf("len(resolved) = %d, want %d", got, want)
		}
		if got, want := resolved[0].Spec.Description, "user override"; got != want {
			t.Fatalf("resolved extension-loop description = %q, want %q", got, want)
		}
	})
}

func TestDaemonE2EDevCycleEnrollmentShouldPublishAndToggleLoops(t *testing.T) {
	t.Parallel()

	t.Run("Should auto-enroll dev-cycle and remove or restore its loops when disabled or enabled", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		extRegistry := extensionpkg.NewRegistry(db.DB())
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return &fakeExtensionRuntime{
				getFn: func(name string) (*extensionpkg.Extension, error) {
					return loadRegisteredDevCycleExtensionSnapshot(name, extRegistry)
				},
			}
		}

		state := newDevCycleLoopE2EState(t, d, db, cfg)
		cleanup := &bootCleanup{}
		if err := d.bootBundles(ctx, state); err != nil {
			t.Fatalf("bootBundles() error = %v", err)
		}
		if err := d.bootResourceReconcile(ctx, state, cleanup); err != nil {
			t.Fatalf("bootResourceReconcile() error = %v", err)
		}
		t.Cleanup(func() {
			for idx := len(cleanup.fns) - 1; idx >= 0; idx-- {
				if err := cleanup.fns[idx](context.Background()); err != nil {
					t.Fatalf("cleanup[%d]() error = %v", idx, err)
				}
			}
		})

		if err := d.bootExtensions(ctx, state, cleanup); err != nil {
			t.Fatalf("bootExtensions() error = %v", err)
		}
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after bootExtensions) error = %v", err)
		}
		assertDevCycleLoopCatalog(t, state.loopCatalog, true)

		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh extension disable")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(disable) error = %v", err)
		}
		if _, err := state.deps.Extensions.Disable(ctx, devcycle.Name, actor); err != nil {
			t.Fatalf("Extensions.Disable(%s) error = %v", devcycle.Name, err)
		}
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after disable) error = %v", err)
		}
		assertDevCycleLoopCatalog(t, state.loopCatalog, false)

		actor, err = taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "agh extension enable")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(enable) error = %v", err)
		}
		if _, err := state.deps.Extensions.Enable(ctx, devcycle.Name, actor); err != nil {
			t.Fatalf("Extensions.Enable(%s) error = %v", devcycle.Name, err)
		}
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after enable) error = %v", err)
		}
		assertDevCycleLoopCatalog(t, state.loopCatalog, true)
	})
}

func TestLoopWatcherIntegrationShouldResyncForkedFileBackedEdits(t *testing.T) {
	t.Parallel()

	t.Run("Should trigger sync when a forked file-backed Loop changes", func(t *testing.T) {
		t.Parallel()

		sourceRoot := t.TempDir()
		_, sourcePath, err := looppkg.WriteDefinition(
			sourceRoot,
			[]byte(testLoopYAML("software-delivery", "source")),
			looppkg.WriteDefinitionOptions{Source: looppkg.SourceUser},
		)
		if err != nil {
			t.Fatalf("WriteDefinition(source) error = %v", err)
		}
		root := t.TempDir()
		forkPath, err := looppkg.ForkDefinitionFile(sourcePath, root)
		if err != nil {
			t.Fatalf("ForkDefinitionFile() error = %v", err)
		}
		publisher := &loopWatcherTestPublisher{synced: make(chan struct{}, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		stop, done := startLoopWatcher(
			ctx,
			10*time.Millisecond,
			[]string{root},
			nil,
			publisher,
			discardLogger(),
		)
		t.Cleanup(func() {
			if err := stopLoopWatcher(context.Background(), stop, done); err != nil {
				t.Fatalf("stopLoopWatcher() error = %v", err)
			}
		})

		time.Sleep(30 * time.Millisecond)
		data, err := os.ReadFile(forkPath)
		if err != nil {
			t.Fatalf("os.ReadFile(forkPath) error = %v", err)
		}
		updated := append([]byte(nil), data...)
		updated = append(updated, []byte("\n# editor published update\n")...)
		if err := os.WriteFile(forkPath, updated, 0o644); err != nil {
			t.Fatalf("os.WriteFile(forkPath) error = %v", err)
		}

		select {
		case <-publisher.synced:
		case <-time.After(2 * time.Second):
			t.Fatal("loop watcher did not sync after fork edit")
		}
	})
}

type loopIntegrationHarness struct {
	codec   resources.KindCodec[looppkg.ResourceSpec]
	store   resources.Store[looppkg.ResourceSpec]
	catalog *resourceCatalog[looppkg.ResourceSpec]
	driver  resources.ReconcileDriver
}

func newLoopIntegrationHarness(t *testing.T) loopIntegrationHarness {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	codec, err := looppkg.NewResourceCodec()
	if err != nil {
		t.Fatalf("looppkg.NewResourceCodec() error = %v", err)
	}
	store, err := resources.NewStore[looppkg.ResourceSpec](kernel, codec)
	if err != nil {
		t.Fatalf("resources.NewStore() error = %v", err)
	}
	catalog := newResourceCatalog(looppkg.CloneResourceSpec)
	registration, err := resources.NewTypedProjectorRegistration(codec, newLoopProjector(catalog))
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(loop) error = %v", err)
	}
	driver, err := resources.NewReconcileDriver(
		kernel,
		resourceReconcileActor(),
		[]resources.ProjectorRegistration{registration},
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
	return loopIntegrationHarness{codec: codec, store: store, catalog: catalog, driver: driver}
}

func loopIntegrationHome(t *testing.T) aghconfig.HomePaths {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("aghconfig.ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("aghconfig.EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func newDevCycleLoopE2EState(
	t *testing.T,
	d *Daemon,
	db *globaldb.GlobalDB,
	cfg aghconfig.Config,
) *bootState {
	t.Helper()

	kernel, err := resources.NewKernel(db.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	codecs, err := d.buildResourceCodecs(nil)
	if err != nil {
		t.Fatalf("buildResourceCodecs() error = %v", err)
	}
	return &bootState{
		cfg:            cfg,
		logger:         discardLogger(),
		registry:       db,
		sessions:       &fakeSessionManager{},
		observer:       &fakeObserver{},
		resourceKernel: kernel,
		resourceCodecs: codecs,
		loopCatalog:    newResourceCatalog(looppkg.CloneResourceSpec),
	}
}

func assertDevCycleLoopCatalog(
	t *testing.T,
	catalog *resourceCatalog[looppkg.ResourceSpec],
	wantPresent bool,
) {
	t.Helper()

	if catalog == nil {
		t.Fatal("loop catalog = nil")
	}
	records := looppkg.ResolveEffectiveResources(catalog.Snapshot(), "")
	found := map[string]looppkg.ResourceSpec{}
	for _, record := range records {
		if record.Spec.InstalledFromExtension == devcycle.Name {
			found[record.Spec.Name] = record.Spec
		}
	}
	if !wantPresent {
		if len(found) != 0 {
			t.Fatalf("dev-cycle loops = %#v, want none after disable", found)
		}
		return
	}
	for _, name := range []string{"software-delivery", "reviews-watch"} {
		spec, ok := found[name]
		if !ok {
			t.Fatalf("dev-cycle loop %q missing from catalog; found %#v", name, found)
		}
		if got, want := spec.InstalledFromExtension, devcycle.Name; got != want {
			t.Fatalf("%s installed_from_extension = %q, want %q", name, got, want)
		}
		if got, want := spec.Source, looppkg.SourceMarketplace; got != want {
			t.Fatalf("%s source = %q, want %q", name, got, want)
		}
	}
}

func loadRegisteredDevCycleExtensionSnapshot(
	name string,
	registry *extensionpkg.Registry,
) (*extensionpkg.Extension, error) {
	if name != devcycle.Name {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	info, err := registry.Get(devcycle.Name)
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Dir(info.ManifestPath)
	manifest, err := extensionpkg.LoadManifest(rootDir)
	if err != nil {
		return nil, err
	}
	loops, err := loadDevCycleLoopResourceSpecs(rootDir, manifest)
	if err != nil {
		return nil, err
	}
	return &extensionpkg.Extension{
		Info:     *info,
		Manifest: manifest,
		RootDir:  rootDir,
		Loops:    loops,
		Status: extensionpkg.ExtensionStatus{
			Name:       info.Name,
			Version:    info.Version,
			Source:     info.Source,
			Enabled:    info.Enabled,
			Registered: true,
		},
	}, nil
}

func loadDevCycleLoopResourceSpecs(
	rootDir string,
	manifest *extensionpkg.Manifest,
) ([]looppkg.ResourceSpec, error) {
	loaded := map[string]looppkg.ResourceSpec{}
	for _, resourcePath := range manifest.Resources.Loops {
		resourceRoot := filepath.Join(rootDir, filepath.FromSlash(resourcePath))
		err := filepath.WalkDir(resourceRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !looppkg.IsDefinitionFileName(entry.Name()) {
				return nil
			}
			spec, _, err := looppkg.ParseResourceFile(path, looppkg.ResourceParseOptions{
				Source:                 looppkg.SourceMarketplace,
				InstalledFromExtension: devcycle.Name,
			})
			if err != nil {
				return err
			}
			loaded[spec.Name] = spec
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	loops := make([]looppkg.ResourceSpec, 0, len(loaded))
	names := make([]string, 0, len(loaded))
	for name := range loaded {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		loops = append(loops, loaded[name])
	}
	return loops, nil
}

type loopWatcherTestPublisher struct {
	synced chan struct{}
}

func (p *loopWatcherTestPublisher) Sync(context.Context) error {
	select {
	case p.synced <- struct{}{}:
	default:
	}
	return nil
}
