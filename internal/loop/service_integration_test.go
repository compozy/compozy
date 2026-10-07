//go:build integration

package loop_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestServiceIntegrationShouldPersistConfigureAndReflectEffectiveConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should persist loop config and reflect it in later run config", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)

		err := svc.Configure(ctx, "ws-1", store.DefaultProfileID, "valid-loop", loop.LoopConfig{
			BudgetTokens:     new(2222),
			BudgetWallSec:    new(333),
			BudgetOnExceeded: new(dsl.BudgetExceededEscalate),
			FanOutWidth:      new(9),
			NoProgressWindow: new(4),
		})
		if err != nil {
			t.Fatalf("Configure() error = %v", err)
		}

		run, err := svc.Start(ctx, "ws-1", "valid-loop", loop.Inputs{ProfileID: store.DefaultProfileID,
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

		preview, err := svc.DryRun(ctx, "ws-1", "valid-loop", loop.Inputs{ProfileID: store.DefaultProfileID,
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
		preview, err := svc.DryRun(ctx, "ws-1", "valid-loop", loop.Inputs{ProfileID: store.DefaultProfileID,
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

func TestServiceIntegrationExecutedDefinitionSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and hydrate templated parent and child Loop actions", func(t *testing.T) {
		t.Parallel()

		definition := validDefinition()
		definition.Inputs = map[string]dsl.Input{
			"slug": {Type: dsl.InputTypeString, Required: true},
		}
		definition.Graph = dsl.Graph{
			Nodes: []dsl.Node{
				{
					ID: "draft", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunAgent),
					Params: dsl.NodeParams{
						"agent": "codex", "prompt": "Draft {{ .inputs.slug }}",
						"output_schema": map[string]any{"summary": "string"},
					},
				},
				{
					ID: "workspace_child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
					Params: dsl.NodeParams{
						"loop": "workspace-child",
						"inputs": map[string]any{
							"slug": "{{ .inputs.slug }}", "summary": "{{ .nodes.draft.output.summary }}",
						},
					},
				},
				{
					ID: "marketplace_child", Class: dsl.NodeClassAction, Kind: string(dsl.ActionRunLoop),
					Params: dsl.NodeParams{
						"loop": "review-and-fix",
						"inputs": map[string]any{
							"slug": "{{ .inputs.slug }}", "summary": "{{ .nodes.draft.output.summary }}",
						},
					},
				},
			},
			Edges: []dsl.Edge{
				{From: "draft", To: "workspace_child"},
				{From: "workspace_child", To: "marketplace_child"},
			},
		}
		globalDB := openLoopServiceGlobalDB(t)
		insertLoopServiceWorkspace(t, globalDB, "ws-1")
		svc := newIntegrationService(t, globalDB, definition)
		ctx := testutil.Context(t)

		run, err := svc.Start(ctx, "ws-1", "valid-loop", loop.Inputs{ProfileID: store.DefaultProfileID,
			Values: map[string]any{"slug": "issue-313"},
		}, humanActor(t))
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		stored, err := globalDB.GetLoopRun(ctx, "ws-1", run.ID)
		if err != nil {
			t.Fatalf("GetLoopRun() error = %v", err)
		}
		if stored.DefinitionDigest != run.DefinitionDigest || stored.Status != loop.StatusRunning {
			t.Fatalf("stored Run = %#v, want running with digest %q", stored, run.DefinitionDigest)
		}
		snapshot, err := globalDB.GetLoopDefinitionSnapshot(ctx, "ws-1", run.DefinitionDigest)
		if err != nil {
			t.Fatalf("GetLoopDefinitionSnapshot() error = %v", err)
		}
		hydrated, err := loop.LoadExecutedDefinitionSnapshot(snapshot.Definition, snapshot.Digest)
		if err != nil {
			t.Fatalf("LoadExecutedDefinitionSnapshot() error = %v", err)
		}
		for _, key := range []string{
			"nodes.workspace_child.params.inputs.slug",
			"nodes.workspace_child.params.inputs.summary",
			"nodes.marketplace_child.params.inputs.slug",
			"nodes.marketplace_child.params.inputs.summary",
			"nodes.workspace_child.params.mode",
			"nodes.marketplace_child.params.mode",
		} {
			if hydrated.Templates[key] == nil {
				t.Fatalf("hydrated template %q is nil", key)
			}
		}
	})
}

func TestServiceIntegrationConfigureShouldPreserveUnboundedFanOutWidth(t *testing.T) {
	t.Parallel()

	t.Run("Should clamp loop config overrides before persisting", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopServiceGlobalDB(t)
		svc := newIntegrationService(t, globalDB, validDefinition())
		ctx := testutil.Context(t)

		err := svc.Configure(ctx, "ws-1", store.DefaultProfileID, "valid-loop", loop.LoopConfig{
			FanOutWidth:      new(500),
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
		if cfg.FanOutWidth == nil || *cfg.FanOutWidth != 500 {
			t.Fatalf("stored FanOutWidth = %#v, want 500", cfg.FanOutWidth)
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
