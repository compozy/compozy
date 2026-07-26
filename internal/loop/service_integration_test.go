//go:build integration

package loop_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type observingParticipationResolver struct {
	inner        participation.Resolver
	mu           sync.Mutex
	observations []participation.ResolvedObservation
}

func (r *observingParticipationResolver) Resolve(
	ctx context.Context,
	in participation.ResolveInput,
) (participation.Spec, error) {
	return r.inner.Resolve(ctx, in)
}

func (r *observingParticipationResolver) ObserveParticipationResolved(
	_ context.Context,
	observation participation.ResolvedObservation,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.observations = append(r.observations, observation)
	return nil
}

func (r *observingParticipationResolver) snapshot() []participation.ResolvedObservation {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]participation.ResolvedObservation(nil), r.observations...)
}

func TestServiceIntegrationShouldPersistConfigureAndReflectEffectiveConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should persist loop config and reflect it in later run config", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)
		onExceeded := dsl.BudgetExceededEscalate

		err := svc.Configure(ctx, "ws-1", "valid-loop", loop.LoopConfig{
			BudgetTokens:     new(2222),
			BudgetWallSec:    new(333),
			BudgetOnExceeded: &onExceeded,
			FanOutWidth:      new(9),
			NoProgressWindow: new(4),
		})
		if err != nil {
			t.Fatalf("Configure() error = %v", err)
		}

		run, err := svc.Start(ctx, "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if run.BudgetTokens != 2222 || run.BudgetWallSec != 333 ||
			run.BudgetOnExceeded != dsl.BudgetExceededEscalate {
			t.Fatalf("run budget = tokens:%d wall:%d on_exceeded:%q, want configured values",
				run.BudgetTokens,
				run.BudgetWallSec,
				run.BudgetOnExceeded,
			)
		}

		preview, err := svc.DryRun(ctx, "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		})
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if preview.EffectiveConfig.FanOutWidth != 9 {
			t.Fatalf("DryRun EffectiveConfig.FanOutWidth = %d, want 9", preview.EffectiveConfig.FanOutWidth)
		}
	})
}

func TestServiceIntegrationDryRunShouldCreateNoState(t *testing.T) {
	t.Parallel()

	t.Run("Should create no loop run or task run for a real resolved definition", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)

		loopRunsBefore := countRows(ctx, t, globalDB, "loop_runs")
		taskRunsBefore := countRows(ctx, t, globalDB, "task_runs")
		preview, err := svc.DryRun(ctx, "ws-1", "valid-loop", loop.Inputs{
			Values: map[string]any{"tasks": "task-ref"},
		})
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if preview.Generation != 1 || len(preview.Nodes) == 0 {
			t.Fatalf("DryRun preview = %#v, want gen-1 nodes", preview)
		}
		if got := countRows(ctx, t, globalDB, "loop_runs"); got != loopRunsBefore {
			t.Fatalf("loop_runs count = %d, want unchanged %d", got, loopRunsBefore)
		}
		if got := countRows(ctx, t, globalDB, "task_runs"); got != taskRunsBefore {
			t.Fatalf("task_runs count = %d, want unchanged %d", got, taskRunsBefore)
		}
	})
}

func TestServiceIntegrationParticipationShouldPersistOneSnapshotPerLoopRun(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a local multi-node run and dry-run free of network artifacts", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)
		inputs := loop.Inputs{Values: map[string]any{"tasks": "task-ref"}}
		preview, err := svc.DryRun(ctx, "ws-1", "valid-loop", inputs)
		if err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if got := preview.ResolvedNetworkParticipation; got != participation.LocalSpec() {
			t.Fatalf("DryRun participation = %#v, want canonical Local", got)
		}
		run, err := svc.Start(ctx, "ws-1", "valid-loop", inputs, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if got := run.NetworkSpecSnapshot(); got != participation.LocalSpec() {
			t.Fatalf("Run participation = %#v, want canonical Local", got)
		}
		for _, table := range []string{"network_channels", "network_channel_participants"} {
			if got := countRows(ctx, t, globalDB, table); got != 0 {
				t.Fatalf("%s rows = %d, want 0 for Local loop", table, got)
			}
		}
	})

	t.Run("Should isolate concurrent live loop-run snapshots", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		definition := validDefinition()
		definition.Concurrency = dsl.ConcurrencyAllow
		live := participation.ModeLive
		loopRunStrategy := participation.StrategyLoopRun
		definition.NetworkParticipation = &participation.Request{
			Mode:            &live,
			ChannelStrategy: &loopRunStrategy,
		}
		resolver := &observingParticipationResolver{inner: loopTestParticipationResolver(t, true)}
		svc := newIntegrationService(
			t,
			globalDB,
			definition,
			loop.WithParticipationResolver(resolver),
		)
		ctx := testutil.Context(t)
		inputs := loop.Inputs{Values: map[string]any{"tasks": "task-ref"}}
		if _, err := svc.DryRun(ctx, "ws-1", "valid-loop", inputs); err != nil {
			t.Fatalf("DryRun() error = %v", err)
		}
		if got := len(resolver.snapshot()); got != 0 {
			t.Fatalf("resolved observations after DryRun = %d, want 0", got)
		}
		first, err := svc.Start(ctx, "ws-1", "valid-loop", inputs, humanActor(t))
		if err != nil {
			t.Fatalf("Start(first) error = %v", err)
		}
		second, err := svc.Start(ctx, "ws-1", "valid-loop", inputs, humanActor(t))
		if err != nil {
			t.Fatalf("Start(second) error = %v", err)
		}
		firstSpec := first.NetworkSpecSnapshot()
		secondSpec := second.NetworkSpecSnapshot()
		if firstSpec.Mode != participation.ModeLive || secondSpec.Mode != participation.ModeLive ||
			firstSpec.ChannelID == secondSpec.ChannelID {
			t.Fatalf("live specs = %#v / %#v, want distinct per-run conversations", firstSpec, secondSpec)
		}
		storedFirst, err := globalDB.GetLoopRun(ctx, "ws-1", first.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(first) error = %v", err)
		}
		storedSecond, err := globalDB.GetLoopRun(ctx, "ws-1", second.ID)
		if err != nil {
			t.Fatalf("GetLoopRun(second) error = %v", err)
		}
		if storedFirst.NetworkSpecSnapshot() != firstSpec || storedSecond.NetworkSpecSnapshot() != secondSpec {
			t.Fatalf("stored specs = %#v / %#v, want immutable start snapshots", storedFirst, storedSecond)
		}
		observations := resolver.snapshot()
		if got, want := len(observations), 2; got != want {
			t.Fatalf("resolved observations after committed starts = %d, want %d", got, want)
		}
		if observations[0].Owner.ID != string(first.ID) || observations[0].Spec != firstSpec ||
			observations[1].Owner.ID != string(second.ID) || observations[1].Spec != secondSpec {
			t.Fatalf("resolved observations = %#v, want committed loop-run snapshots", observations)
		}
	})
}

func TestServiceIntegrationConfigureShouldClampAtWriteTime(t *testing.T) {
	t.Parallel()

	t.Run("Should clamp loop config overrides before persisting", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)

		err := svc.Configure(ctx, "ws-1", "valid-loop", loop.LoopConfig{
			FanOutWidth:      new(loop.LoopMaxFanoutWidth + 100),
			NoProgressWindow: new(loop.LoopMaxNoProgressWindow + 100),
			GateMaxRevisions: new(loop.LoopMaxGateRevisions + 100),
		})
		if err != nil {
			t.Fatalf("Configure() error = %v", err)
		}

		cfg, err := globalDB.GetLoopConfig(ctx, "ws-1", "valid-loop")
		if err != nil {
			t.Fatalf("GetLoopConfig() error = %v", err)
		}
		if cfg.FanOutWidth == nil || *cfg.FanOutWidth != loop.LoopMaxFanoutWidth {
			t.Fatalf("stored FanOutWidth = %#v, want %d", cfg.FanOutWidth, loop.LoopMaxFanoutWidth)
		}
		if cfg.NoProgressWindow == nil || *cfg.NoProgressWindow != loop.LoopMaxNoProgressWindow {
			t.Fatalf(
				"stored NoProgressWindow = %#v, want %d",
				cfg.NoProgressWindow,
				loop.LoopMaxNoProgressWindow,
			)
		}
		if cfg.GateMaxRevisions == nil || *cfg.GateMaxRevisions != loop.LoopMaxGateRevisions {
			t.Fatalf(
				"stored GateMaxRevisions = %#v, want %d",
				cfg.GateMaxRevisions,
				loop.LoopMaxGateRevisions,
			)
		}
	})
}

func openLoopServiceGlobalDB(t *testing.T) *globaldb.GlobalDB {
	t.Helper()

	globalDB, err := globaldb.OpenGlobalDB(
		testutil.Context(t),
		filepath.Join(t.TempDir(), store.GlobalDatabaseName),
	)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := globalDB.Close(testutil.Context(t)); closeErr != nil {
			t.Errorf("Close() error = %v", closeErr)
		}
	})
	return globalDB
}

func newIntegrationService(
	t *testing.T,
	loopStore loop.Store,
	def dsl.Definition,
	opts ...loop.Option,
) loop.Service {
	t.Helper()

	resolved := compileDefinition(t, def)
	options := []loop.Option{
		loop.WithClock(func() time.Time {
			return time.Date(2026, 7, 4, 15, 0, 0, 0, time.UTC)
		}),
	}
	options = append(options, opts...)
	svc, err := loop.NewService(
		loopStore,
		loop.DefinitionResolverFunc(func(
			context.Context,
			loop.WorkspaceID,
			string,
		) (*loop.ResolvedDefinition, error) {
			return resolved, nil
		}),
		loop.GoalRunPolicyResolverFunc(func(
			context.Context,
			loop.WorkspaceID,
		) (*loop.GoalRunPolicy, error) {
			return &loop.GoalRunPolicy{ContextNudgeRatio: 0.8}, nil
		}),
		options...,
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

func countRows(ctx context.Context, t *testing.T, globalDB *globaldb.GlobalDB, table string) int {
	t.Helper()

	var count int
	query := "SELECT COUNT(*) FROM " + table
	if err := globalDB.DB().QueryRowContext(ctx, query).Scan(&count); err != nil {
		t.Fatalf("count %s rows error = %v", table, err)
	}
	return count
}

func insertLoopServiceWorkspace(t *testing.T, globalDB *globaldb.GlobalDB, workspaceID string) {
	t.Helper()
	now := time.Date(2026, 7, 4, 14, 59, 0, 0, time.UTC)
	if err := globalDB.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        workspaceID,
		RootDir:   t.TempDir(),
		Name:      workspaceID,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace(%q) error = %v", workspaceID, err)
	}
}
