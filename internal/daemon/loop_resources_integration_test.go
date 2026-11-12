//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	speccycle "github.com/compozy/compozy/extensions/spec-cycle"
	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/tools/builtin"
	"github.com/compozy/compozy/internal/workspace"
)

func TestLoopSourceSyncerIntegrationShouldProjectFSPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should project global and workspace records with workspace precedence", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
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
					resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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
		resolved := looppkg.ResolveEffectiveResources(records, looppkg.ResourceLens{
			WorkspaceID: "ws-1", ProfileID: store.DefaultProfileID, ProfileName: "default",
		})
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

		ctx := t.Context()
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
						scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
						spec:      extensionSpec,
					},
					{
						sourceKey: "global/user/extension-loop",
						scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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
		resolved := looppkg.ResolveEffectiveResources(harness.catalog.Snapshot(), looppkg.ResourceLens{
			ProfileID: store.DefaultProfileID, ProfileName: "default",
		})
		if got, want := len(resolved), 1; got != want {
			t.Fatalf("len(resolved) = %d, want %d", got, want)
		}
		if got, want := resolved[0].Spec.Description, "user override"; got != want {
			t.Fatalf("resolved extension-loop description = %q, want %q", got, want)
		}
	})
}

func TestDaemonE2ESpecCycleEnrollmentShouldKeepLoopCatalogEmpty(t *testing.T) {
	t.Parallel()

	t.Run("Should keep resource-only spec-cycle loops absent across enablement", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		cfg := testConfig(t, homePaths)
		d := newTestDaemon(t, homePaths, &cfg)
		extRegistry := extensionpkg.NewRegistry(db.DB())
		d.newExtensionManager = func(extensionManagerDeps) extensionRuntime {
			return &specCycleProfileRuntime{fakeExtensionRuntime: &fakeExtensionRuntime{
				getFn: func(name string) (*extensionpkg.Extension, error) {
					return loadRegisteredSpecCycleExtensionSnapshot(name, extRegistry)
				},
			}, registry: extRegistry}
		}

		state := newSpecCycleLoopE2EState(t, d, db, cfg)
		cleanup := &bootCleanup{}
		if err := d.bootResourceReconcile(ctx, state, cleanup); err != nil {
			t.Fatalf("bootResourceReconcile() error = %v", err)
		}
		t.Cleanup(func() {
			for idx, fn := range slices.Backward(cleanup.fns) {
				if err := fn(context.Background()); err != nil {
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

type specCycleProfileRuntime struct {
	*fakeExtensionRuntime
	registry *extensionpkg.Registry
}

func (r *specCycleProfileRuntime) ProjectForProfile(
	_ context.Context,
	key extensionpkg.InstanceKey,
	profile extensionpkg.ProfileLens,
) (*extensionpkg.Extension, bool, error) {
	if key.Name != speccycle.Name {
		return &extensionpkg.Extension{}, false, nil
	}
	extension, err := r.Get(key.Name)
	if err != nil {
		return nil, false, err
	}
	enabled, err := r.registry.IsEnabledForProfile(key.Name, profile.ID)
	if err != nil {
		return nil, false, err
	}
	extension.Info.Enabled = enabled
	extension.Status.Enabled = enabled
	return extension, enabled, nil
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
		ctx, cancel := context.WithCancel(t.Context())
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
	codecs, err := d.buildResourceCodecs()
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
	records := looppkg.ResolveEffectiveResources(catalog.Snapshot(), looppkg.ResourceLens{
		ProfileID: store.DefaultProfileID, ProfileName: "default",
	})
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
	wantNames := []string{}
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
		resourceRoot := filepath.Join(rootDir, filepath.FromSlash(resourcePath.Path))
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

// Invariant: persisted Loop definitions execute the pinned view through the real registry and hook dispatcher.
func TestLoopResultViewsIntegration(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		full, patch  bool
		singleResult bool
	}{
		{name: "Should complete an unchanged compact definition"},
		{name: "Should complete a single-result tool without a selector", singleResult: true},
		{name: "Should complete a pinned full node", full: true},
		{name: "Should reject a hook downgrade of a pinned full node", full: true, patch: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openDaemonTestGlobalDB(t)
			now := time.Now().UTC()
			const workspaceID = "ws-result-views"
			if err := db.InsertWorkspace(
				ctx,
				workspace.Workspace{
					ID:        workspaceID,
					Name:      "Result views",
					RootDir:   t.TempDir(),
					CreatedAt: now,
					UpdatedAt: now,
				},
			); err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int32
			live := loopResultViewDescriptor(t)
			if tc.singleResult {
				for _, descriptor := range builtin.NativeDescriptors() {
					if descriptor.ID == toolspkg.ToolIDLoopRequest {
						live = descriptor
						break
					}
				}
			}
			registry := loopResultViewRegistry(t, live, &calls, nil)
			node := loopdsl.Node{
				ID:     "inspect",
				Class:  loopdsl.NodeClassAction,
				Kind:   live.ID.String(),
				Params: loopdsl.NodeParams{},
			}
			if tc.singleResult {
				node.Params = loopdsl.NodeParams{"run_id": "run-request", "generation": 1, "node_id": "ask"}
			}
			if tc.full {
				node.Params[toolspkg.ResultViewInputKey] = "full"
			}
			definition := loopdsl.Definition{
				APIVersion: loopdsl.APIVersion,
				Kind:       loopdsl.KindLoop,
				Meta:       loopdsl.Meta{Name: "result-views"},
				Contract: loopdsl.Contract{
					Goal:             "Read a tool",
					DefinitionOfDone: "Read the pinned record",
					IterationCap:     1,
					StopWhen: loopdsl.StopWhenSpec{
						Expr: `nodes.inspect.status == "succeeded" && nodes.inspect.output.id == "fixture"`,
					},
				},
				Graph: loopdsl.Graph{Nodes: []loopdsl.Node{node}},
			}
			if tc.singleResult {
				definition.Contract.StopWhen.Expr = `nodes.inspect.status == "succeeded" && nodes.inspect.output.node_id == "ask"`
			}
			resolved, err := newLoopCompilerWithSchemaSource(newLoopToolSchemaSource(ctx, registry)).Compile(definition)
			if err != nil {
				t.Fatal(err)
			}
			service, err := looppkg.NewService(
				db,
				looppkg.DefinitionResolverFunc(
					func(context.Context, looppkg.WorkspaceID, string, string) (*looppkg.ResolvedDefinition, error) {
						return resolved, nil
					},
				),
				looppkg.GoalRunPolicyResolverFunc(
					func(context.Context, looppkg.WorkspaceID) (*looppkg.GoalRunPolicy, error) {
						return &looppkg.GoalRunPolicy{ContextNudgeRatio: 0.8}, nil
					},
				),
			)
			if err != nil {
				t.Fatal(err)
			}
			human, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "result-views")
			if err != nil {
				t.Fatal(err)
			}
			run, err := service.Start(
				ctx,
				workspaceID,
				definition.Meta.Name,
				looppkg.Inputs{ProfileID: store.DefaultProfileID},
				human,
			)
			if err != nil {
				t.Fatal(err)
			}
			if tc.patch {
				registry = loopResultViewRegistry(t, live, &calls, loopResultViewDowngradeHook(t, db, workspaceID))
			}
			actions, err := looppkg.NewActionRegistry(registry)
			if err != nil {
				t.Fatal(err)
			}
			coordinator, err := looppkg.NewCoordinatorRunner(
				db,
				db,
				db,
				discardLogger(),
				looppkg.WithCoordinatorActionRegistry(actions),
			)
			if err != nil {
				t.Fatal(err)
			}
			manager, err := taskpkg.NewManager(
				taskpkg.WithStore(db),
				taskpkg.WithCoordinatorRunner(coordinator),
				taskpkg.WithGenerationStateFinalizer(looppkg.NewStoreFinalizer()),
				taskpkg.WithCoordinatorTerminalStatusValidator(
					func(status string) bool { return looppkg.Status(status).Valid() },
				),
				taskpkg.WithCoordinatorTerminalHookStatusValidator(
					func(status string) bool { return looppkg.Status(status).Terminal() },
				),
			)
			if err != nil {
				t.Fatal(err)
			}
			actor, err := taskpkg.DeriveDaemonActorContext("loop-action", "daemon.loop-action")
			if err != nil {
				t.Fatal(err)
			}
			claim := claimLoopRunForIntegration(t, manager, db, *run, taskpkg.RunKindCoordinator, "", actor)
			if _, err := manager.StartRun(
				ctx,
				claim.Run.ID,
				taskpkg.StartRun{IdempotencyKey: "start-" + claim.Run.ID, ClaimToken: claim.ClaimToken},
				actor,
			); err != nil {
				t.Fatal(err)
			}
			worker := queuedLoopWorkerForIntegration(t, db, run.ID, 1)
			taskRecord, err := db.GetTask(ctx, worker.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := newLoopActionRuntime(manager, db, coordinator, nil, discardLogger(), nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := runtime.shutdown(shutdownCtx); err != nil {
					t.Error(err)
				}
			})
			err = runtime.executeQueuedRun(runtime.root, taskRecord, worker, loopActionRuntimeReasonEnqueued)
			if tc.patch {
				toolErr, ok := errors.AsType[*toolspkg.ToolError](err)
				if !ok || !slices.Contains(toolErr.ReasonCodes, toolspkg.ReasonResultViewPinned) || calls.Load() != 0 {
					t.Fatalf("error=%v calls=%d", err, calls.Load())
				}
			} else if err != nil {
				t.Fatal(err)
			}
			outputs, err := db.ListGenerationOutputs(ctx, run.WorkspaceID, run.ID, 1)
			if err != nil {
				t.Fatal(err)
			}
			if len(outputs) != 1 {
				t.Fatalf("outputs=%#v", outputs)
			}
			if tc.patch {
				if outputs[0].Status != "failed" || !strings.Contains(outputs[0].OutputRef, "tool_denied") {
					t.Fatalf("output=%#v", outputs[0])
				}
				return
			}
			settled, err := db.GetTaskRun(ctx, worker.ID)
			if err != nil {
				t.Fatal(err)
			}
			if outputs[0].Status != "succeeded" || settled.Status != taskpkg.TaskRunStatusCompleted ||
				calls.Load() != 1 {
				t.Fatalf("output=%#v worker=%#v calls=%d", outputs[0], settled, calls.Load())
			}
			hasDetails := strings.Contains(string(settled.ResultValue()), "full-only")
			if !tc.singleResult && hasDetails != tc.full {
				t.Fatalf("result=%s", settled.ResultValue())
			}
			wake, added, err := db.EnqueueLoopCoordinatorWake(
				ctx,
				string(run.ID),
				"view-result-settled",
				actor.Origin,
				time.Now().UTC(),
			)
			if err != nil || !added {
				t.Fatalf("enqueue completion wake: added=%v error=%v", added, err)
			}
			next := claimLoopRunForIntegration(t, manager, db, *run, taskpkg.RunKindCoordinator, wake.ID, actor)
			if _, err := manager.StartRun(
				ctx,
				next.Run.ID,
				taskpkg.StartRun{IdempotencyKey: "start-" + next.Run.ID, ClaimToken: next.ClaimToken},
				actor,
			); err != nil {
				t.Fatal(err)
			}
			finished, err := db.GetLoopRun(ctx, run.WorkspaceID, run.ID)
			if err != nil {
				t.Fatal(err)
			}
			if finished.Status != looppkg.StatusDone {
				t.Fatalf("run status=%s, want done", finished.Status)
			}
		})
	}
}

func loopResultViewDescriptor(t *testing.T) toolspkg.Descriptor {
	t.Helper()
	const full = `{"type":"object","properties":{"id":{"type":"string"},"details":{"type":"string"}},"additionalProperties":false}`
	descriptor := toolspkg.Descriptor{
		ID:           "compozy__view_fixture",
		Description:  "Read a fixture record",
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false}`),
		OutputSchema: json.RawMessage(full),
		Backend: toolspkg.BackendRef{
			Kind:       toolspkg.BackendNativeGo,
			NativeName: "view_fixture",
		},
		Source:     toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: "daemon"},
		Visibility: toolspkg.VisibilityModel,
		Risk:       toolspkg.RiskRead,
		ReadOnly:   true,
	}
	descriptor.InputSchema = json.RawMessage(
		`{"type":"object","properties":{"result_view":{"type":"string","enum":["compact","full"]}},"additionalProperties":false}`,
	)
	descriptor.OutputSchema = json.RawMessage(
		`{"type":"object","properties":{"id":{"type":"string"}},"additionalProperties":false}`,
	)
	descriptor.SetResultViews(
		&toolspkg.ResultViews{
			Compact: toolspkg.ResultViewContract{Schema: descriptor.OutputSchema},
			Full:    toolspkg.ResultViewContract{Schema: json.RawMessage(full)},
		},
	)
	result, err := toolspkg.DescriptorWithSchemaDigests(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func loopResultViewRegistry(
	t *testing.T,
	descriptor toolspkg.Descriptor,
	calls *atomic.Int32,
	hooks toolspkg.HookRunner,
) *toolspkg.RuntimeRegistry {
	t.Helper()
	provider, err := toolspkg.NewNativeProvider(
		descriptor.Source,
		toolspkg.NativeTool{
			Descriptor: descriptor,
			Call: func(_ context.Context, _ toolspkg.Scope, call toolspkg.CallRequest) (toolspkg.ToolResult, error) {
				calls.Add(1)
				if descriptor.ID == toolspkg.ToolIDLoopRequest {
					var input map[string]any
					if err := json.Unmarshal(call.Input, &input); err != nil {
						return toolspkg.ToolResult{}, err
					}
					if _, ok := input[toolspkg.ResultViewInputKey]; ok || call.PinnedView != "" {
						return toolspkg.ToolResult{}, errors.New("single-result call received a selector")
					}
					service := &nativeLoopServiceStub{
						getLoopRequestFn: func(context.Context, string, string, int, string, int) (contract.LoopRequestPayload, error) {
							return contract.LoopRequestPayload{
								LoopRunID:   "run-request",
								Generation:  1,
								NodeID:      "ask",
								Kind:        "ask",
								State:       "pending",
								ProfileID:   store.DefaultProfileID,
								WorkspaceID: "ws-result-views",
								Context:     json.RawMessage(`{}`),
								Decisions:   []string{"respond"},
								Agents:      "allow",
								OpenedAt:    time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC),
							}, nil
						},
					}
					native := &daemonNativeTools{
						deps: &daemonNativeToolsDeps{Loops: func() core.LoopService { return service }},
					}
					return native.loopRequest(t.Context(), toolspkg.Scope{WorkspaceID: "ws-result-views"}, call)
				}
				return resultViews[map[string]string]{
					Compact: func(value map[string]string) any { return map[string]string{"id": value["id"]} },
				}.result(
					call.View,
					map[string]string{"id": "fixture", "details": "full-only"},
				)
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	options := []toolspkg.RegistryOption{toolspkg.WithProviders(provider)}
	if hooks != nil {
		options = append(options, toolspkg.WithHookRunner(hooks))
	}
	registry, err := toolspkg.NewRegistry(options...)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func loopResultViewDowngradeHook(t *testing.T, db *globaldb.GlobalDB, workspaceID string) toolspkg.HookRunner {
	t.Helper()
	h := newHookBindingIntegrationHarness(t, map[string]hookspkg.Executor{"downgrade": hookspkg.NewTypedNativeExecutor(
		func(_ context.Context, _ hookspkg.RegisteredHook, _ hookspkg.ToolPreCallPayload) (hookspkg.ToolCallPatch, error) {
			return hookspkg.ToolCallPatch{ToolInput: json.RawMessage(`{"result_view":"compact"}`)}, nil
		},
	)}, db)
	h.putBinding(
		t,
		"view-downgrade",
		0,
		resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspaceID},
		hookspkg.HookDecl{
			Name:         "downgrade",
			Event:        hookspkg.HookToolPreCall,
			Source:       hookspkg.HookSourceNative,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
			Matcher:      hookspkg.HookMatcher{ToolID: "compozy__view_fixture"},
		},
	)
	if err := h.driver.RunBoot(t.Context()); err != nil {
		t.Fatal(err)
	}
	return &nativeToolHookRunner{hooks: h.notifier, binder: &nativeWorkspaceInputBinder{}}
}
