package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestLoopProjectorShouldBuildAndApplyCatalogSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should build and apply a defensive loop catalog snapshot", func(t *testing.T) {
		t.Parallel()

		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		projector := newLoopProjector(catalog)
		if projector == nil {
			t.Fatal("newLoopProjector() = nil, want projector")
		}

		records := []resources.Record[looppkg.ResourceSpec]{
			{
				ID:      "loop-a",
				Version: 7,
				Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Spec:    testLoopSpec(t, "loop-a", looppkg.SourceUser),
			},
		}
		plan, err := projector.Build(context.Background(), records)
		if err != nil {
			t.Fatalf("projector.Build() error = %v", err)
		}
		if got, want := plan.Kind(), looppkg.ResourceKind; got != want {
			t.Fatalf("plan.Kind() = %q, want %q", got, want)
		}
		if got, want := plan.Revision(), int64(7); got != want {
			t.Fatalf("plan.Revision() = %d, want %d", got, want)
		}
		if err := projector.Apply(context.Background(), plan); err != nil {
			t.Fatalf("projector.Apply() error = %v", err)
		}
		snapshot := catalog.Snapshot()
		if got, want := len(snapshot), 1; got != want {
			t.Fatalf("len(catalog.Snapshot()) = %d, want %d", got, want)
		}
		snapshot[0].Spec.Name = "mutated"
		if got, want := catalog.Snapshot()[0].Spec.Name, "loop-a"; got != want {
			t.Fatalf("catalog.Snapshot()[0].Spec.Name = %q, want %q", got, want)
		}
	})
}

func TestDaemonLoopAPIServiceShouldPublishWithServerManagedCASVersion(t *testing.T) {
	t.Parallel()

	t.Run("Should stamp the next version and reject stale publishes", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        "ws-1",
			Name:      "workspace-one",
			RootDir:   workspaceRoot,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}

		loopRoot := filepath.Join(workspaceRoot, aghconfig.DirName, aghconfig.LoopsDirName)
		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		initialSpec := testLoopSpec(t, "alpha", looppkg.SourceWorkspace)
		initialSpec.Dir = filepath.Join(loopRoot, "alpha")
		initialSpec.FilePath = filepath.Join(loopRoot, "alpha", "loop.yaml")
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{{
			ID:      "loop:ws-1:alpha",
			Version: 1,
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
			Spec:    initialSpec,
		}})
		service := &daemonLoopAPIService{
			catalog:           catalog,
			publisher:         &loopAPITestPublisher{},
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}

		expected := 1
		response, err := service.PatchLoop(ctx, "ws-1", "alpha", contract.PatchLoopRequest{
			ExpectedVersion: &expected,
			Definition:      loopAPITestDocument(t, "alpha", 1, "first publish"),
		})
		if err != nil {
			t.Fatalf("PatchLoop(first) error = %v", err)
		}
		if response.Loop.Version != 2 {
			t.Fatalf("PatchLoop(first) version = %d, want 2", response.Loop.Version)
		}
		published := readLoopDefinitionFile(t, filepath.Join(loopRoot, "alpha", "loop.yaml"))
		if published.Meta.Version != 2 {
			t.Fatalf("published meta.version = %d, want 2", published.Meta.Version)
		}

		_, err = service.PatchLoop(ctx, "ws-1", "alpha", contract.PatchLoopRequest{
			ExpectedVersion: &expected,
			Definition:      loopAPITestDocument(t, "alpha", 1, "stale publish"),
		})
		var conflict *core.LoopVersionConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("PatchLoop(stale) error = %v, want LoopVersionConflictError", err)
		}
		if conflict.CurrentVersion != 2 {
			t.Fatalf("PatchLoop(stale) current version = %d, want 2", conflict.CurrentVersion)
		}

		expected = 2
		_, err = service.PatchLoop(ctx, "ws-1", "alpha", contract.PatchLoopRequest{
			ExpectedVersion: &expected,
			Definition:      loopAPITestDocument(t, "alpha", 1, "mismatched client version"),
		})
		if !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("PatchLoop(mismatched definition version) error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should return created loops before asynchronous catalog projection runs", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        "ws-create",
			Name:      "workspace-create",
			RootDir:   workspaceRoot,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}

		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		service := &daemonLoopAPIService{
			catalog:           catalog,
			publisher:         &loopAPITestPublisher{},
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}
		def := loopAPITestDocument(t, "alpha", 999, "created publish")

		response, err := service.CreateLoop(ctx, "ws-create", contract.CreateLoopRequest{Definition: &def})
		if err != nil {
			t.Fatalf("CreateLoop() error = %v", err)
		}
		if response.Loop.Name != "alpha" || response.Loop.Version != 1 {
			t.Fatalf("CreateLoop() payload = %#v, want alpha version 1", response.Loop)
		}

		getResponse, err := service.GetLoop(ctx, "ws-create", "alpha")
		if err != nil {
			t.Fatalf("GetLoop(created) error = %v", err)
		}
		if getResponse.Loop.Name != "alpha" || getResponse.Loop.Version != 1 {
			t.Fatalf("GetLoop(created) payload = %#v, want alpha version 1", getResponse.Loop)
		}

		_, err = service.CreateLoop(ctx, "ws-create", contract.CreateLoopRequest{Definition: &def})
		if !errors.Is(err, looppkg.ErrDefinitionExists) {
			t.Fatalf("CreateLoop(duplicate) error = %v, want ErrDefinitionExists", err)
		}
	})

	t.Run("Should fork a file-backed extension Loop into the workspace", func(t *testing.T) {
		t.Parallel()

		fixture := newLoopAPIForkFixture(t)

		response, err := fixture.service.CreateLoop(fixture.ctx, fixture.workspaceID, contract.CreateLoopRequest{
			ForkFromName: "software-delivery",
		})
		if err != nil {
			t.Fatalf("CreateLoop(fork extension) error = %v", err)
		}
		if got, want := response.Loop.Source, contract.LoopSourceWorkspace; got != want {
			t.Fatalf("CreateLoop(fork extension) source = %q, want %q", got, want)
		}
		if _, _, err := looppkg.ParseResourceFile(fixture.forkPath, looppkg.ResourceParseOptions{
			Source: looppkg.SourceWorkspace,
			Linter: newLoopLinterWithSchemaSource(newLoopToolSchemaSource(fixture.ctx, fixture.registry)),
		}); err != nil {
			t.Fatalf("ParseResourceFile(fork) error = %v", err)
		}
		if fixture.sourcePath == fixture.forkPath {
			t.Fatalf("fork path = source path %q, want a workspace-owned copy", fixture.forkPath)
		}

		definition := response.Loop.Definition
		definition.Meta.Version = response.Loop.Version
		definition.Meta.Description = "first workspace edit"
		_, err = fixture.service.PatchLoop(
			fixture.ctx,
			fixture.workspaceID,
			response.Loop.Name,
			contract.PatchLoopRequest{Definition: definition},
		)
		if !errors.Is(err, looppkg.ErrValidation) || !strings.Contains(err.Error(), "expected_version is required") {
			t.Fatalf("PatchLoop(fork without CAS) error = %v, want required expected_version validation", err)
		}

		expectedVersion := 0
		published, err := fixture.service.PatchLoop(
			fixture.ctx,
			fixture.workspaceID,
			response.Loop.Name,
			contract.PatchLoopRequest{
				ExpectedVersion: &expectedVersion,
				Definition:      definition,
			},
		)
		if err != nil {
			t.Fatalf("PatchLoop(fork version zero) error = %v", err)
		}
		if got, want := published.Loop.Version, 1; got != want {
			t.Fatalf("PatchLoop(fork version zero) version = %d, want %d", got, want)
		}
	})

	t.Run("Should remove the workspace fork after post-copy failures", func(t *testing.T) {
		t.Parallel()

		syncErr := errors.New("sync fork resources")
		tests := []struct {
			name      string
			configure func(*loopAPIForkFixture)
			matches   func(error) bool
		}{
			{
				name: "Should roll back after resource sync fails",
				configure: func(fixture *loopAPIForkFixture) {
					requestCtx, cancel := context.WithCancel(fixture.ctx)
					fixture.ctx = requestCtx
					firstSync := true
					fixture.service.publisher = &loopAPITestPublisher{syncFn: func(ctx context.Context) error {
						if firstSync {
							firstSync = false
							cancel()
							return syncErr
						}
						return ctx.Err()
					}}
				},
				matches: func(err error) bool { return errors.Is(err, syncErr) },
			},
			{
				name: "Should roll back after response projection fails",
				configure: func(fixture *loopAPIForkFixture) {
					fixture.service.publisher = &loopAPITestPublisher{syncFn: func(context.Context) error {
						if _, err := os.Stat(fixture.forkPath); errors.Is(err, os.ErrNotExist) {
							removeLoopAPIFixtureWorkspaceRecord(fixture)
							return nil
						} else if err != nil {
							return fmt.Errorf("inspect copied fork: %w", err)
						}
						if _, _, err := fixture.service.refreshCatalogLoopRecordFromFile(
							fixture.ctx,
							fixture.forkPath,
							looppkg.WorkspaceID(fixture.workspaceID),
							"software-delivery",
						); err != nil {
							return fmt.Errorf("project copied fork: %w", err)
						}
						data, err := os.ReadFile(fixture.forkPath)
						if err != nil {
							return fmt.Errorf("read copied fork: %w", err)
						}
						data = []byte(strings.Replace(string(data), "name: software-delivery", "name: other", 1))
						if err := os.WriteFile(fixture.forkPath, data, 0o600); err != nil {
							return fmt.Errorf("corrupt copied fork: %w", err)
						}
						return nil
					}}
				},
				matches: func(err error) bool { return errors.Is(err, looppkg.ErrValidation) },
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				fixture := newLoopAPIForkFixture(t)
				tt.configure(&fixture)
				_, err := fixture.service.CreateLoop(
					fixture.ctx,
					fixture.workspaceID,
					contract.CreateLoopRequest{ForkFromName: "software-delivery"},
				)
				if !tt.matches(err) {
					t.Fatalf("CreateLoop(fork failure) error = %v, want injected post-copy failure", err)
				}
				if _, statErr := os.Stat(filepath.Dir(fixture.forkPath)); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("Stat(fork directory) error = %v, want os.ErrNotExist", statErr)
				}
				publisher, ok := fixture.service.publisher.(*loopAPITestPublisher)
				if !ok {
					t.Fatalf("publisher type = %T, want *loopAPITestPublisher", fixture.service.publisher)
				}
				if got, want := publisher.calls, 2; got != want {
					t.Fatalf("publisher.Sync() calls = %d, want %d after cleanup reconciliation", got, want)
				}
				resolved := looppkg.ResolveEffectiveResources(
					fixture.service.catalog.Snapshot(),
					fixture.workspaceID,
				)
				if got, want := len(resolved), 1; got != want {
					t.Fatalf("len(resolved loops after rollback) = %d, want %d", got, want)
				}
				if got, want := resolved[0].Spec.Source, looppkg.SourceMarketplace; got != want {
					t.Fatalf("resolved source after rollback = %q, want %q", got, want)
				}
			})
		}
	})

	t.Run("Should upsert published record without dropping existing catalog records", func(t *testing.T) {
		t.Parallel()

		catalog := newResourceCatalog(looppkg.CloneResourceSpec)
		otherSpec := testLoopSpec(t, "other", looppkg.SourceMarketplace)
		oldSpec := testLoopSpec(t, "alpha", looppkg.SourceWorkspace)
		scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"}
		catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{
			{ID: "loop:ext:other", Version: 1, Scope: resources.ResourceScope{}, Spec: otherSpec},
			{ID: "loop:ws-1:alpha:old", Version: 1, Scope: scope, Spec: oldSpec},
		})
		service := &daemonLoopAPIService{catalog: catalog}

		nextSpec := testLoopSpec(t, "alpha", looppkg.SourceWorkspace)
		nextSpec.Description = "updated"
		service.upsertPublishedLoopRecord(resources.Record[looppkg.ResourceSpec]{
			ID:      "loop:ws-1:alpha:new",
			Version: 2,
			Scope:   scope,
			Spec:    nextSpec,
		})

		records := catalog.Snapshot()
		if got, want := len(records), 2; got != want {
			t.Fatalf("len(catalog.Snapshot()) = %d, want %d: %#v", got, want, records)
		}
		if got, want := catalog.Revision(), int64(2); got != want {
			t.Fatalf("catalog.Revision() = %d, want %d", got, want)
		}
		if got, want := records[0].Spec.Name, "other"; got != want {
			t.Fatalf("records[0].Spec.Name = %q, want %q", got, want)
		}
		if got, want := records[1].ID, "loop:ws-1:alpha:new"; got != want {
			t.Fatalf("records[1].ID = %q, want %q", got, want)
		}
		if got, want := records[1].Spec.Description, "updated"; got != want {
			t.Fatalf("records[1].Spec.Description = %q, want %q", got, want)
		}
	})

	t.Run("Should classify missing fork sources as not found", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		db := openDaemonTestGlobalDB(t)
		homePaths := testHomePaths(t)
		workspaceRoot := filepath.Join(t.TempDir(), "workspace")
		if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
		}
		now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
		if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
			ID:        "ws-fork",
			Name:      "workspace-fork",
			RootDir:   workspaceRoot,
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		resolver, err := workspacepkg.NewResolver(
			db,
			workspacepkg.WithHomePaths(homePaths),
			workspacepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("workspace.NewResolver() error = %v", err)
		}
		service := &daemonLoopAPIService{
			catalog:           newResourceCatalog(looppkg.CloneResourceSpec),
			publisher:         &loopAPITestPublisher{},
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		}

		_, err = service.CreateLoop(ctx, "ws-fork", contract.CreateLoopRequest{ForkFromName: "missing-loop"})
		if !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("CreateLoop(fork missing) error = %v, want ErrDefinitionNotFound", err)
		}
	})

	t.Run("Should classify absent definitions as not found", func(t *testing.T) {
		t.Parallel()

		service := &daemonLoopAPIService{catalog: newResourceCatalog(looppkg.CloneResourceSpec)}
		if _, err := service.GetLoop(
			testutil.Context(t),
			"ws-1",
			"missing",
		); !errors.Is(err, looppkg.ErrDefinitionNotFound) {
			t.Fatalf("GetLoop(missing) error = %v, want ErrDefinitionNotFound", err)
		}
		if _, err := service.GetLoop(
			testutil.Context(t),
			"ws-1",
			"MyLoop",
		); !errors.Is(err, looppkg.ErrValidation) {
			t.Fatalf("GetLoop(malformed name) error = %v, want ErrValidation", err)
		}
	})
}

func TestLoopSourceSyncerShouldProjectAndDeleteManagedRecords(t *testing.T) {
	t.Parallel()

	t.Run("Should write desired loops and delete stale managed loops", func(t *testing.T) {
		t.Parallel()

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

		providerItems := []loopPublicationInput{
			{
				sourceKey: "test/global/loop-a",
				scope:     resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				spec:      testLoopSpec(t, "loop-a", looppkg.SourceUser),
			},
		}
		syncer := newLoopSourceSyncer(
			store,
			codec,
			loopSyncActor(),
			discardLogger(),
			nil,
			func(context.Context) ([]loopPublicationInput, error) {
				return append([]loopPublicationInput(nil), providerItems...), nil
			},
		)

		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("Sync(first) error = %v", err)
		}
		records, err := store.List(
			context.Background(),
			loopSyncActor(),
			resources.ResourceFilter{Kind: looppkg.ResourceKind},
		)
		if err != nil {
			t.Fatalf("store.List(first) error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("len(records first) = %d, want %d", got, want)
		}
		if got, want := records[0].Spec.Name, "loop-a"; got != want {
			t.Fatalf("records[0].Spec.Name = %q, want %q", got, want)
		}

		providerItems = nil
		if err := syncer.Sync(context.Background()); err != nil {
			t.Fatalf("Sync(second) error = %v", err)
		}
		records, err = store.List(
			context.Background(),
			loopSyncActor(),
			resources.ResourceFilter{Kind: looppkg.ResourceKind},
		)
		if err != nil {
			t.Fatalf("store.List(second) error = %v", err)
		}
		if got := len(records); got != 0 {
			t.Fatalf("len(records second) = %d, want 0", got)
		}
	})
}

func testLoopSpec(t *testing.T, name string, source looppkg.Source) looppkg.ResourceSpec {
	t.Helper()

	opts := looppkg.ResourceParseOptions{
		Source:   source,
		FilePath: "/tmp/" + name + "/loop.yaml",
	}
	spec, _, err := looppkg.ParseResource([]byte(testLoopYAML(name, "test")), opts)
	if err != nil {
		t.Fatalf("looppkg.ParseResource(%q) error = %v", name, err)
	}
	return spec
}

func testLoopYAML(name string, description string) string {
	return `apiVersion: agh.loop/v1
kind: Loop
meta:
  name: ` + name + `
  version: 1
  description: ` + description + `
  catalog:
    use_when: Test daemon projection
    keywords: [projection]
    category: test
concurrency: queue
inputs:
  target:
    type: string
    required: true
contract:
  goal: Test daemon projection
  definition_of_done: Projection works
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress:
    window: 2
    hash_fields: []
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
start:
  - kind: cli
graph:
  nodes:
    - id: target_input
      class: source
      kind: input
      input_ref: target
    - id: normalize_target
      class: action
      kind: transform
      params:
        map:
          target:
            value: ok
  edges:
    - from: target_input
      to: normalize_target
`
}

type loopAPITestPublisher struct {
	calls  int
	syncFn func(context.Context) error
}

func (p *loopAPITestPublisher) Sync(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("loop api test publisher: %w", err)
	}
	p.calls++
	if p.syncFn != nil {
		return p.syncFn(ctx)
	}
	return nil
}

type loopAPIForkFixture struct {
	ctx         context.Context
	service     *daemonLoopAPIService
	workspaceID string
	registry    toolspkg.Registry
	sourcePath  string
	forkPath    string
}

func newLoopAPIForkFixture(t *testing.T) loopAPIForkFixture {
	t.Helper()

	ctx := testutil.Context(t)
	db := openDaemonTestGlobalDB(t)
	homePaths := testHomePaths(t)
	workspaceID := "ws-fork-extension"
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspaceRoot) error = %v", err)
	}
	now := time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID:        workspaceID,
		Name:      "workspace-fork-extension",
		RootDir:   workspaceRoot,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	descriptor := loopToolSchemaDescriptor(t)
	registry := loopToolSchemaRegistry{views: map[toolspkg.ToolID]toolspkg.ToolView{
		descriptor.ID: {Descriptor: descriptor},
	}}
	definition := strings.Replace(
		testLoopYAML("software-delivery", "extension source"),
		"version: 1",
		"version: 0",
		1,
	)
	definition = strings.Replace(
		definition,
		"kind: transform",
		"kind: "+descriptor.ID.String(),
		1,
	)
	sourceSpec, sourcePath, err := looppkg.WriteDefinition(
		t.TempDir(),
		[]byte(definition),
		looppkg.WriteDefinitionOptions{
			Source: looppkg.SourceMarketplace,
			Linter: newLoopLinterWithSchemaSource(newLoopToolSchemaSource(ctx, registry)),
		},
	)
	if err != nil {
		t.Fatalf("WriteDefinition(source) error = %v", err)
	}
	catalog := newResourceCatalog(looppkg.CloneResourceSpec)
	catalog.Replace(1, []resources.Record[looppkg.ResourceSpec]{
		{
			ID:      "loop:extension:software-delivery",
			Version: 1,
			Scope:   resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:    sourceSpec,
		},
	})
	return loopAPIForkFixture{
		ctx:         ctx,
		workspaceID: workspaceID,
		registry:    registry,
		sourcePath:  sourcePath,
		forkPath: filepath.Join(
			workspaceRoot,
			aghconfig.DirName,
			aghconfig.LoopsDirName,
			"software-delivery",
			looppkg.DefinitionFileName,
		),
		service: &daemonLoopAPIService{
			catalog:           catalog,
			publisher:         &loopAPITestPublisher{},
			toolRegistry:      registry,
			workspaceResolver: resolver,
			now:               func() time.Time { return now },
		},
	}
}

func removeLoopAPIFixtureWorkspaceRecord(fixture *loopAPIForkFixture) {
	fixture.service.catalog.Update(func(
		records []resources.Record[looppkg.ResourceSpec],
	) []resources.Record[looppkg.ResourceSpec] {
		filtered := make([]resources.Record[looppkg.ResourceSpec], 0, len(records))
		for _, record := range records {
			scope := record.Scope.Normalize()
			if scope.Kind == resources.ResourceScopeKindWorkspace &&
				scope.ID == fixture.workspaceID &&
				record.Spec.Name == "software-delivery" {
				continue
			}
			filtered = append(filtered, record)
		}
		return filtered
	})
}

func loopAPITestDefinition(t *testing.T, name string, version int, description string) dsl.Definition {
	t.Helper()

	raw := strings.Replace(testLoopYAML(name, description), "version: 1", fmt.Sprintf("version: %d", version), 1)
	def, err := dsl.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("dsl.Parse(%q) error = %v", name, err)
	}
	def.Normalize()
	return def
}

func loopAPITestDocument(t *testing.T, name string, version int, description string) contract.LoopDefinitionDocument {
	t.Helper()

	document, err := contract.NewLoopDefinitionDocument(loopAPITestDefinition(t, name, version, description))
	if err != nil {
		t.Fatalf("NewLoopDefinitionDocument(%q) error = %v", name, err)
	}
	return document
}

func readLoopDefinitionFile(t *testing.T, path string) dsl.Definition {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	def, err := dsl.Parse(data)
	if err != nil {
		t.Fatalf("dsl.Parse(%q) error = %v", path, err)
	}
	return def
}
