//go:build integration

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
)

func TestLoopSourceSyncerIntegrationShouldProjectFSPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should project global and workspace records with workspace precedence", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := loopIntegrationHome(t)
		if _, _, err := looppkg.WriteDefinition(
			homePaths.LoopsDir,
			[]byte(testLoopYAML("implement-tasks", "global shadow")),
			looppkg.WriteDefinitionOptions{Source: looppkg.SourceUser},
		); err != nil {
			t.Fatalf("WriteDefinition(global) error = %v", err)
		}

		workspaceRoot := t.TempDir()
		workspaceLoopsDir := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.LoopsDirName)
		if _, _, err := looppkg.WriteDefinition(
			workspaceLoopsDir,
			[]byte(testLoopYAML("implement-tasks", "workspace shadow")),
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
			if record.Spec.Name != "implement-tasks" {
				continue
			}
			found = true
			if got, want := record.Spec.Description, "workspace shadow"; got != want {
				t.Fatalf("resolved implement-tasks description = %q, want %q", got, want)
			}
		}
		if !found {
			t.Fatal("resolved implement-tasks not found")
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

func TestDaemonE2ESpecCycleEnrollmentShouldPublishAndToggleLoops(t *testing.T) {
	t.Parallel()

	t.Run("Should auto-enroll spec-cycle and remove or restore its loops when disabled or enabled", func(t *testing.T) {
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
					return loadRegisteredSpecCycleExtensionSnapshot(name, extRegistry)
				},
			}
		}

		state := newSpecCycleLoopE2EState(t, d, db, cfg)
		cleanup := &bootCleanup{}
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
		enableSpecCycle := func() {
			t.Helper()

			actor, err := taskpkg.DeriveHumanActorContext(
				"operator",
				taskpkg.OriginKindCLI,
				"compozy extension enable",
			)
			if err != nil {
				t.Fatalf("DeriveHumanActorContext(enable) error = %v", err)
			}
			if _, err := state.deps.Extensions.Enable(
				ctx,
				speccycle.Name,
				contract.EnableExtensionRequest{},
				actor,
			); err != nil {
				t.Fatalf("Extensions.Enable(%s) error = %v", speccycle.Name, err)
			}
		}
		enableSpecCycle()
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after initial enable) error = %v", err)
		}
		assertSpecCycleLoopCatalog(t, state.loopCatalog, true)

		actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "compozy extension disable")
		if err != nil {
			t.Fatalf("DeriveHumanActorContext(disable) error = %v", err)
		}
		if _, err := state.deps.Extensions.Disable(ctx, speccycle.Name, actor); err != nil {
			t.Fatalf("Extensions.Disable(%s) error = %v", speccycle.Name, err)
		}
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after disable) error = %v", err)
		}
		assertSpecCycleLoopCatalog(t, state.loopCatalog, false)

		enableSpecCycle()
		if err := state.resourceReconcile.RunBoot(ctx); err != nil {
			t.Fatalf("RunBoot(after enable) error = %v", err)
		}
		assertSpecCycleLoopCatalog(t, state.loopCatalog, true)
	})
}

func TestLoopWatcherIntegrationShouldResyncForkedFileBackedEdits(t *testing.T) {
	t.Parallel()

	t.Run("Should trigger sync when a forked file-backed Loop changes", func(t *testing.T) {
		t.Parallel()
		cancelCalled := false
		if err := stopLoopWatcher(nil, func() { cancelCalled = true }, make(chan struct{})); err == nil {
			t.Fatal("stopLoopWatcher(nil context) error = nil, want context validation failure")
		}
		if cancelCalled {
			t.Fatal("stopLoopWatcher(nil context) canceled watcher")
		}

		sourceRoot := t.TempDir()
		_, sourcePath, err := looppkg.WriteDefinition(
			sourceRoot,
			[]byte(testLoopYAML("implement-tasks", "source")),
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

func loopIntegrationHome(t *testing.T) compozyconfig.HomePaths {
	t.Helper()

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("compozyconfig.ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("compozyconfig.EnsureHomeLayout() error = %v", err)
	}
	return homePaths
}

func newSpecCycleLoopE2EState(
	t *testing.T,
	d *Daemon,
	db *globaldb.GlobalDB,
	cfg compozyconfig.Config,
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
		automation:     &fakeAutomationManager{},
		resourceKernel: kernel,
		resourceCodecs: codecs,
		loopCatalog:    newResourceCatalog(looppkg.CloneResourceSpec),
	}
}

func assertSpecCycleLoopCatalog(
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
		if record.Spec.InstalledFromExtension == speccycle.Name {
			found[record.Spec.Name] = record.Spec
		}
	}
	if !wantPresent {
		if len(found) != 0 {
			t.Fatalf("spec-cycle loops = %#v, want none after disable", found)
		}
		return
	}
	wantNames := []string{"implement-tasks", "orchestrate-tasks", "review-and-fix"}
	if got, want := len(found), len(wantNames); got != want {
		t.Fatalf("spec-cycle loops = %#v, want exactly %d bundled loops", found, want)
	}
	for _, name := range wantNames {
		spec, ok := found[name]
		if !ok {
			t.Fatalf("spec-cycle loop %q missing from catalog; found %#v", name, found)
		}
		if got, want := spec.InstalledFromExtension, speccycle.Name; got != want {
			t.Fatalf("%s installed_from_extension = %q, want %q", name, got, want)
		}
		if got, want := spec.Source, looppkg.SourceMarketplace; got != want {
			t.Fatalf("%s source = %q, want %q", name, got, want)
		}
	}
}

func loadRegisteredSpecCycleExtensionSnapshot(
	name string,
	registry *extensionpkg.Registry,
) (*extensionpkg.Extension, error) {
	if name != speccycle.Name {
		return nil, extensionpkg.ErrExtensionNotFound
	}
	info, err := registry.Get(speccycle.Name)
	if err != nil {
		return nil, err
	}
	rootDir := filepath.Dir(info.ManifestPath)
	manifest, err := extensionpkg.LoadManifest(rootDir)
	if err != nil {
		return nil, err
	}
	loops, err := loadSpecCycleLoopResourceSpecs(rootDir, manifest)
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

func loadSpecCycleLoopResourceSpecs(
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
				InstalledFromExtension: speccycle.Name,
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
