package kernel

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type managerSpy struct {
	startCalls    int
	shutdownCalls int
	startErr      error
}

func (m *managerSpy) Start(context.Context) error {
	m.startCalls++
	return m.startErr
}

func (*managerSpy) DispatchMutableHook(_ context.Context, _ string, input any) (any, error) {
	return input, nil
}

func (*managerSpy) DispatchObserverHook(context.Context, string, any) {}

func (m *managerSpy) Shutdown(context.Context) error {
	m.shutdownCalls++
	return nil
}

type scopeSpy struct {
	artifacts   model.RunArtifacts
	manager     model.RuntimeManager
	closeCalls  int
	closeCtxErr error
	closeErr    error
}

func (s *scopeSpy) RunArtifacts() model.RunArtifacts {
	return s.artifacts
}

func (*scopeSpy) RunJournal() *journal.Journal {
	return nil
}

func (*scopeSpy) RunEventBus() *events.Bus[events.Event] {
	return nil
}

func (s *scopeSpy) RunManager() model.RuntimeManager {
	return s.manager
}

func (s *scopeSpy) Close(ctx context.Context) error {
	s.closeCalls++
	if ctx != nil {
		s.closeCtxErr = ctx.Err()
	}
	return s.closeErr
}

func TestRunStartHandlerPassesExecutableExtensionOptionForWorkflowModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode model.ExecutionMode
	}{
		{name: "start", mode: model.ExecutionModePRDTasks},
		{name: "fix_reviews", mode: model.ExecutionModePRReview},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeOperations{
				prepareResult: &model.SolvePreparation{
					RunArtifacts: model.NewRunArtifacts("/workspace", "run-123"),
				},
			}
			dispatcher := BuildDefault(testKernelDeps(fake))

			_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
				context.Background(),
				dispatcher,
				commands.RunStartCommand{
					Runtime: model.RuntimeConfig{
						WorkspaceRoot:              "/workspace",
						Name:                       "demo",
						Mode:                       tt.mode,
						IDE:                        model.IDECodex,
						EnableExecutableExtensions: true,
					},
				},
			)
			if err != nil {
				t.Fatalf("dispatch run start: %v", err)
			}
			if len(fake.openOptions) != 1 {
				t.Fatalf("expected one open options call, got %d", len(fake.openOptions))
			}
			if !fake.openOptions[0].EnableExecutableExtensions {
				t.Fatal("expected executable extensions to be enabled for workflow mode")
			}
		})
	}
}

func TestRunStartHandlerStartsRuntimeManagerBeforePrepare(t *testing.T) {
	t.Parallel()

	manager := &managerSpy{}
	scope := &scopeSpy{
		artifacts: model.NewRunArtifacts("/workspace", "run-123"),
		manager:   manager,
	}
	fake := &fakeOperations{
		openResult: scope,
		prepareResult: &model.SolvePreparation{
			RunArtifacts: scope.artifacts,
		},
		prepareHook: func(model.RunScope) {
			if manager.startCalls != 1 {
				t.Fatalf("expected manager to start before prepare, got %d calls", manager.startCalls)
			}
		},
	}
	dispatcher := BuildDefault(testKernelDeps(fake))

	_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot:              "/workspace",
				Name:                       "demo",
				Mode:                       model.ExecutionModePRDTasks,
				IDE:                        model.IDECodex,
				EnableExecutableExtensions: true,
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start: %v", err)
	}
	if manager.startCalls != 1 {
		t.Fatalf("expected one manager start call, got %d", manager.startCalls)
	}
}

func TestRunStartExecPathWithoutExtensionsSkipsRunScopeOpen(t *testing.T) {
	t.Parallel()

	fake := &fakeOperations{}
	dispatcher := BuildDefault(testKernelDeps(fake))

	_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot: "/workspace",
				Mode:          model.ExecutionModeExec,
				IDE:           model.IDECodex,
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch exec run start: %v", err)
	}
	if len(fake.openCalls) != 0 {
		t.Fatalf("expected exec fast path to skip open run scope, got %d open calls", len(fake.openCalls))
	}
	if len(fake.execCalls) != 1 {
		t.Fatalf("expected one exec call, got %d", len(fake.execCalls))
	}
	if fake.execCalls[0].scope != nil {
		t.Fatalf("expected exec fast path to pass nil scope, got %T", fake.execCalls[0].scope)
	}
}

func TestRunStartExecPathWithExtensionsOpensScopeAndPassesItToExec(t *testing.T) {
	t.Parallel()

	scope := &scopeSpy{artifacts: model.NewRunArtifacts("/workspace", "exec-extensions")}
	fake := &fakeOperations{openResult: scope}
	dispatcher := BuildDefault(testKernelDeps(fake))

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot:              "/workspace",
				Mode:                       model.ExecutionModeExec,
				IDE:                        model.IDECodex,
				EnableExecutableExtensions: true,
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch exec run start: %v", err)
	}
	if len(fake.openOptions) != 1 || !fake.openOptions[0].EnableExecutableExtensions {
		t.Fatalf("unexpected open options: %#v", fake.openOptions)
	}
	if len(fake.execCalls) != 1 {
		t.Fatalf("expected one exec call, got %d", len(fake.execCalls))
	}
	if fake.execCalls[0].scope != scope {
		t.Fatalf("expected scoped exec call, got %T", fake.execCalls[0].scope)
	}
	if result.RunID != scope.artifacts.RunID {
		t.Fatalf("unexpected run id: %q", result.RunID)
	}
}

func TestRunStartHandlerClosesRunScopeOnSuccessPrepareErrorAndCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cancelContext bool
		prepareErr    error
		executeErr    error
	}{
		{name: "success"},
		{name: "prepare_error", prepareErr: errors.New("prepare boom")},
		{name: "cancellation", cancelContext: true, executeErr: context.Canceled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scope := &scopeSpy{artifacts: model.NewRunArtifacts("/workspace", "run-123")}
			fake := &fakeOperations{
				openResult: scope,
				prepareErr: tt.prepareErr,
				executeErr: tt.executeErr,
				prepareResult: &model.SolvePreparation{
					RunArtifacts: scope.artifacts,
				},
			}
			dispatcher := BuildDefault(testKernelDeps(fake))

			ctx := context.Background()
			if tt.cancelContext {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
				ctx,
				dispatcher,
				commands.RunStartCommand{
					Runtime: model.RuntimeConfig{
						WorkspaceRoot: "/workspace",
						Name:          "demo",
						Mode:          model.ExecutionModePRDTasks,
						IDE:           model.IDECodex,
					},
				},
			)
			if tt.prepareErr == nil && tt.executeErr == nil && err != nil {
				t.Fatalf("dispatch run start: %v", err)
			}
			if tt.prepareErr != nil && !errors.Is(err, tt.prepareErr) {
				t.Fatalf("expected prepare error %v, got %v", tt.prepareErr, err)
			}
			if tt.executeErr != nil && !errors.Is(err, tt.executeErr) {
				t.Fatalf("expected execute error %v, got %v", tt.executeErr, err)
			}
			if scope.closeCalls != 1 {
				t.Fatalf("expected one scope close call, got %d", scope.closeCalls)
			}
			if tt.cancelContext && !errors.Is(scope.closeCtxErr, context.Canceled) {
				t.Fatalf("expected canceled close context, got %v", scope.closeCtxErr)
			}
		})
	}
}
