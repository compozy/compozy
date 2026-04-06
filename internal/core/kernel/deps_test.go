package kernel

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type executeCall struct {
	prep *model.SolvePreparation
	cfg  *model.RuntimeConfig
}

type fakeOperations struct {
	validateCalls []*model.RuntimeConfig
	prepareCalls  []*model.RuntimeConfig
	executeCalls  []executeCall
	execCalls     []*model.RuntimeConfig
	fetchCalls    []core.Config
	migrateCalls  []core.MigrationConfig
	syncCalls     []core.SyncConfig
	archiveCalls  []core.ArchiveConfig

	validateErr error
	prepareErr  error
	executeErr  error
	execErr     error
	fetchErr    error
	migrateErr  error
	syncErr     error
	archiveErr  error

	prepareResult *model.SolvePreparation
	fetchResult   *core.FetchResult
	migrateResult *core.MigrationResult
	syncResult    *core.SyncResult
	archiveResult *core.ArchiveResult
}

func (f *fakeOperations) ValidateRuntimeConfig(cfg *model.RuntimeConfig) error {
	f.validateCalls = append(f.validateCalls, cloneRuntimeConfig(cfg))
	return f.validateErr
}

func (f *fakeOperations) Prepare(_ context.Context, cfg *model.RuntimeConfig) (*model.SolvePreparation, error) {
	f.prepareCalls = append(f.prepareCalls, cloneRuntimeConfig(cfg))
	if f.prepareErr != nil {
		return nil, f.prepareErr
	}
	if f.prepareResult == nil {
		return &model.SolvePreparation{}, nil
	}
	return f.prepareResult, nil
}

func (f *fakeOperations) Execute(
	_ context.Context,
	prep *model.SolvePreparation,
	cfg *model.RuntimeConfig,
) error {
	f.executeCalls = append(f.executeCalls, executeCall{
		prep: cloneSolvePreparation(prep),
		cfg:  cloneRuntimeConfig(cfg),
	})
	return f.executeErr
}

func (f *fakeOperations) ExecuteExec(_ context.Context, cfg *model.RuntimeConfig) error {
	f.execCalls = append(f.execCalls, cloneRuntimeConfig(cfg))
	return f.execErr
}

func (f *fakeOperations) FetchReviews(_ context.Context, cfg core.Config) (*core.FetchResult, error) {
	f.fetchCalls = append(f.fetchCalls, cfg)
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	if f.fetchResult == nil {
		return &core.FetchResult{}, nil
	}
	return f.fetchResult, nil
}

func (f *fakeOperations) Migrate(
	_ context.Context,
	cfg core.MigrationConfig,
) (*core.MigrationResult, error) {
	f.migrateCalls = append(f.migrateCalls, cfg)
	if f.migrateErr != nil {
		return nil, f.migrateErr
	}
	if f.migrateResult == nil {
		return &core.MigrationResult{}, nil
	}
	return f.migrateResult, nil
}

func (f *fakeOperations) Sync(_ context.Context, cfg core.SyncConfig) (*core.SyncResult, error) {
	f.syncCalls = append(f.syncCalls, cfg)
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	if f.syncResult == nil {
		return &core.SyncResult{}, nil
	}
	return f.syncResult, nil
}

func (f *fakeOperations) Archive(
	_ context.Context,
	cfg core.ArchiveConfig,
) (*core.ArchiveResult, error) {
	f.archiveCalls = append(f.archiveCalls, cfg)
	if f.archiveErr != nil {
		return nil, f.archiveErr
	}
	if f.archiveResult == nil {
		return &core.ArchiveResult{}, nil
	}
	return f.archiveResult, nil
}

func TestBuildDefaultRegistersAllPhaseACommands(t *testing.T) {
	t.Parallel()

	dispatcher := BuildDefault(testKernelDeps(&fakeOperations{}))
	if err := selfTestDefaultRegistry(dispatcher); err != nil {
		t.Fatalf("self test: %v", err)
	}
}

func TestKernelDepsResolveOperationsUsesDefaultRegistryBackedOps(t *testing.T) {
	t.Parallel()

	deps := KernelDeps{}
	if _, ok := deps.resolveOperations().(realOperations); !ok {
		t.Fatalf("expected realOperations, got %T", deps.resolveOperations())
	}
}

func TestDefaultRegistrySelfTestFailsWhenCommandIsMissing(t *testing.T) {
	t.Parallel()

	deps := testKernelDeps(&fakeOperations{})
	ops := deps.ops

	dispatcher := NewDispatcher()
	Register(dispatcher, newRunStartHandler(deps, ops))
	Register(dispatcher, newWorkflowPrepareHandler(deps, ops))
	Register(dispatcher, newWorkflowSyncHandler(deps, ops))
	Register(dispatcher, newWorkflowArchiveHandler(deps, ops))
	Register(dispatcher, newWorkspaceMigrateHandler(deps, ops))

	err := selfTestDefaultRegistry(dispatcher)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "commands.ReviewsFetchCommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunStartExecPathDelegatesToExecuteExec(t *testing.T) {
	t.Parallel()

	fake := &fakeOperations{}
	dispatcher := BuildDefault(testKernelDeps(fake))

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModeExec,
				IDE:           model.IDECodex,
				RunID:         "exec-001",
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch exec run start: %v", err)
	}
	if len(fake.execCalls) != 1 {
		t.Fatalf("expected 1 execute-exec call, got %d", len(fake.execCalls))
	}
	if result.RunID != "exec-001" {
		t.Fatalf("unexpected run id: %q", result.RunID)
	}
}

func TestBuildDefaultDispatchesRunStartAndDelegatesToPrepareAndExecute(t *testing.T) {
	fake := &fakeOperations{
		prepareResult: &model.SolvePreparation{
			Jobs: []model.Job{{
				CodeFiles:     []string{"task_001.md"},
				SafeName:      "task-001",
				Prompt:        []byte("prompt"),
				OutPromptPath: "/tmp/task-001.prompt.md",
				OutLog:        "/tmp/task-001.out.log",
				ErrLog:        "/tmp/task-001.err.log",
			}},
			RunArtifacts: model.NewRunArtifacts("/workspace", "run-123"),
			InputDir:     "demo",
			InputDirPath: "/workspace/.compozy/tasks/demo",
		},
	}
	dispatcher := BuildDefault(testKernelDeps(fake))

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				Model:         "gpt-5.4",
				BatchSize:     1,
				Timeout:       time.Minute,
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start: %v", err)
	}

	if len(fake.validateCalls) != 1 {
		t.Fatalf("expected 1 validate call, got %d", len(fake.validateCalls))
	}
	if len(fake.prepareCalls) != 1 {
		t.Fatalf("expected 1 prepare call, got %d", len(fake.prepareCalls))
	}
	if len(fake.executeCalls) != 1 {
		t.Fatalf("expected 1 execute call, got %d", len(fake.executeCalls))
	}

	gotCfg := fake.prepareCalls[0]
	if gotCfg.Name != "demo" {
		t.Fatalf("unexpected prepare name: %q", gotCfg.Name)
	}
	if gotCfg.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("unexpected prepare mode: %q", gotCfg.Mode)
	}
	if gotCfg.IDE != model.IDECodex {
		t.Fatalf("unexpected prepare ide: %q", gotCfg.IDE)
	}
	if gotCfg.Model != "gpt-5.4" {
		t.Fatalf("unexpected prepare model: %q", gotCfg.Model)
	}

	gotExec := fake.executeCalls[0]
	if gotExec.prep == nil {
		t.Fatal("expected execute preparation")
	}
	if len(gotExec.prep.Jobs) != 1 {
		t.Fatalf("unexpected execute job count: %d", len(gotExec.prep.Jobs))
	}
	if gotExec.prep.RunArtifacts.RunID != "run-123" {
		t.Fatalf("unexpected run id: %q", gotExec.prep.RunArtifacts.RunID)
	}

	if result.RunID != "run-123" {
		t.Fatalf("unexpected result run id: %q", result.RunID)
	}
	expectedArtifactsDir := "/workspace/.compozy/runs/run-123"
	if result.ArtifactsDir != expectedArtifactsDir {
		t.Fatalf("unexpected artifacts dir: %q", result.ArtifactsDir)
	}
	if result.Status != runStartStatusSucceeded {
		t.Fatalf("unexpected status: %q", result.Status)
	}
}

func TestRunStartNoWorkReturnsTypedStatusWithoutError(t *testing.T) {
	t.Parallel()

	fake := &fakeOperations{prepareErr: core.ErrNoWork}
	dispatcher := BuildDefault(testKernelDeps(fake))

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start: %v", err)
	}
	if result.Status != runStartStatusNoWork {
		t.Fatalf("unexpected status: %q", result.Status)
	}
}

func TestBuildDefaultDispatchesAllPhaseACommandsSequentially(t *testing.T) {
	fake := &fakeOperations{
		prepareResult: &model.SolvePreparation{
			Jobs:         []model.Job{{SafeName: "task-001"}},
			RunArtifacts: model.NewRunArtifacts("/workspace", "run-200"),
		},
		fetchResult:   &core.FetchResult{Name: "demo", Round: 2},
		migrateResult: &core.MigrationResult{Target: "/workspace/.compozy/tasks/demo"},
		syncResult:    &core.SyncResult{Target: "/workspace/.compozy/tasks/demo"},
		archiveResult: &core.ArchiveResult{Target: "/workspace/.compozy/tasks/demo"},
	}
	dispatcher := BuildDefault(testKernelDeps(fake))

	if _, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
		},
	); err != nil {
		t.Fatalf("run start: %v", err)
	}

	prepareResult, err := Dispatch[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
		context.Background(),
		dispatcher,
		commands.WorkflowPrepareCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
		},
	)
	if err != nil {
		t.Fatalf("workflow prepare: %v", err)
	}
	if prepareResult.Preparation == nil {
		t.Fatal("expected preparation result")
	}

	fetchResult, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
		context.Background(),
		dispatcher,
		commands.ReviewsFetchCommand{
			WorkspaceRoot: "/workspace",
			Name:          "demo",
			Round:         2,
			Provider:      "coderabbit",
			PR:            "259",
		},
	)
	if err != nil {
		t.Fatalf("reviews fetch: %v", err)
	}
	if fetchResult.Result == nil || fetchResult.Result.Round != 2 {
		t.Fatalf("unexpected fetch result: %#v", fetchResult.Result)
	}

	migrateResult, err := Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
		context.Background(),
		dispatcher,
		commands.WorkspaceMigrateCommand{
			WorkspaceRoot: "/workspace",
			Name:          "demo",
			DryRun:        true,
		},
	)
	if err != nil {
		t.Fatalf("workspace migrate: %v", err)
	}
	if migrateResult.Result == nil || migrateResult.Result.Target == "" {
		t.Fatalf("unexpected migrate result: %#v", migrateResult.Result)
	}

	syncResult, err := Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
		context.Background(),
		dispatcher,
		commands.WorkflowSyncCommand{
			WorkspaceRoot: "/workspace",
			Name:          "demo",
			TasksDir:      "/workspace/.compozy/tasks/demo",
			DryRun:        true,
		},
	)
	if err != nil {
		t.Fatalf("workflow sync: %v", err)
	}
	if syncResult.Result == nil || syncResult.Result.Target == "" {
		t.Fatalf("unexpected sync result: %#v", syncResult.Result)
	}

	archiveResult, err := Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
		context.Background(),
		dispatcher,
		commands.WorkflowArchiveCommand{
			WorkspaceRoot: "/workspace",
			Name:          "demo",
			TasksDir:      "/workspace/.compozy/tasks/demo",
		},
	)
	if err != nil {
		t.Fatalf("workflow archive: %v", err)
	}
	if archiveResult.Result == nil || archiveResult.Result.Target == "" {
		t.Fatalf("unexpected archive result: %#v", archiveResult.Result)
	}
}

func TestHandlersPropagateOperationErrors(t *testing.T) {
	t.Parallel()

	validateErr := errors.New("validate failed")
	execErr := errors.New("exec failed")
	executeErr := errors.New("execute failed")
	fetchErr := errors.New("fetch failed")
	migrateErr := errors.New("migrate failed")
	syncErr := errors.New("sync failed")
	archiveErr := errors.New("archive failed")

	tests := []struct {
		name     string
		fake     *fakeOperations
		dispatch func(*Dispatcher) error
		wantErr  error
	}{
		{
			name:    "run start validate",
			wantErr: validateErr,
			fake:    &fakeOperations{validateErr: validateErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
					context.Background(),
					dispatcher,
					commands.RunStartCommand{
						RuntimeFields: commands.RuntimeFields{
							Mode:      model.ExecutionModePRDTasks,
							IDE:       model.IDECodex,
							BatchSize: 1,
						},
					},
				)
				return err
			},
		},
		{
			name:    "run start exec",
			wantErr: execErr,
			fake:    &fakeOperations{execErr: execErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
					context.Background(),
					dispatcher,
					commands.RunStartCommand{
						RuntimeFields: commands.RuntimeFields{
							Mode: model.ExecutionModeExec,
							IDE:  model.IDECodex,
						},
					},
				)
				return err
			},
		},
		{
			name:    "run start execute",
			wantErr: executeErr,
			fake: &fakeOperations{
				prepareResult: &model.SolvePreparation{RunArtifacts: model.NewRunArtifacts("/workspace", "run-123")},
				executeErr:    executeErr,
			},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
					context.Background(),
					dispatcher,
					commands.RunStartCommand{
						RuntimeFields: commands.RuntimeFields{
							WorkspaceRoot: "/workspace",
							Mode:          model.ExecutionModePRDTasks,
							IDE:           model.IDECodex,
							BatchSize:     1,
						},
					},
				)
				return err
			},
		},
		{
			name:    "workflow prepare validate",
			wantErr: validateErr,
			fake:    &fakeOperations{validateErr: validateErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
					context.Background(),
					dispatcher,
					commands.WorkflowPrepareCommand{
						RuntimeFields: commands.RuntimeFields{
							Mode:      model.ExecutionModePRDTasks,
							IDE:       model.IDECodex,
							BatchSize: 1,
						},
					},
				)
				return err
			},
		},
		{
			name:    "reviews fetch",
			wantErr: fetchErr,
			fake:    &fakeOperations{fetchErr: fetchErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.ReviewsFetchCommand, commands.ReviewsFetchResult](
					context.Background(),
					dispatcher,
					commands.ReviewsFetchCommand{},
				)
				return err
			},
		},
		{
			name:    "workspace migrate",
			wantErr: migrateErr,
			fake:    &fakeOperations{migrateErr: migrateErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.WorkspaceMigrateCommand, commands.WorkspaceMigrateResult](
					context.Background(),
					dispatcher,
					commands.WorkspaceMigrateCommand{},
				)
				return err
			},
		},
		{
			name:    "workflow sync",
			wantErr: syncErr,
			fake:    &fakeOperations{syncErr: syncErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.WorkflowSyncCommand, commands.WorkflowSyncResult](
					context.Background(),
					dispatcher,
					commands.WorkflowSyncCommand{},
				)
				return err
			},
		},
		{
			name:    "workflow archive",
			wantErr: archiveErr,
			fake:    &fakeOperations{archiveErr: archiveErr},
			dispatch: func(dispatcher *Dispatcher) error {
				_, err := Dispatch[commands.WorkflowArchiveCommand, commands.WorkflowArchiveResult](
					context.Background(),
					dispatcher,
					commands.WorkflowArchiveCommand{},
				)
				return err
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dispatcher := BuildDefault(testKernelDeps(tt.fake))
			err := tt.dispatch(dispatcher)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowPrepareReturnsCoreNoWork(t *testing.T) {
	t.Parallel()

	fake := &fakeOperations{prepareErr: core.ErrNoWork}
	dispatcher := BuildDefault(testKernelDeps(fake))

	_, err := Dispatch[commands.WorkflowPrepareCommand, commands.WorkflowPrepareResult](
		context.Background(),
		dispatcher,
		commands.WorkflowPrepareCommand{
			RuntimeFields: commands.RuntimeFields{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
		},
	)
	if !errors.Is(err, core.ErrNoWork) {
		t.Fatalf("expected core.ErrNoWork, got %v", err)
	}
}

func TestNewPreparationReturnsNilForNilInput(t *testing.T) {
	t.Parallel()

	if prep := newPreparation(nil); prep != nil {
		t.Fatalf("expected nil preparation, got %#v", prep)
	}
}

func testKernelDeps(fake *fakeOperations) KernelDeps {
	return KernelDeps{
		EventBus:      events.New[events.Event](16),
		Workspace:     workspace.Context{Root: "/workspace"},
		AgentRegistry: agent.DefaultRegistry(),
		ops:           fake,
	}
}

func cloneRuntimeConfig(cfg *model.RuntimeConfig) *model.RuntimeConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.AddDirs = append([]string(nil), cfg.AddDirs...)
	return &cloned
}

func cloneSolvePreparation(prep *model.SolvePreparation) *model.SolvePreparation {
	if prep == nil {
		return nil
	}
	cloned := *prep
	cloned.Jobs = append([]model.Job(nil), prep.Jobs...)
	cloned.JournalHandle = prep.JournalHandle
	return &cloned
}
