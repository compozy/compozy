package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	runtimeevents "github.com/compozy/compozy/pkg/compozy/events"
)

var runScopeTestHomeMu sync.Mutex

func TestOpenRunScopeDisabledReturnsArtifactsJournalAndBusWithoutManager(t *testing.T) {
	cfg := runtimeConfigForTest(t)

	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	defer closeRunScopeForTest(t, scope)

	if scope.ExtensionsEnabled {
		t.Fatal("expected executable extensions to remain disabled")
	}
	if scope.Manager != nil {
		t.Fatalf("scope.Manager = %#v, want nil", scope.Manager)
	}
	if scope.Journal == nil {
		t.Fatal("expected scope journal")
	}
	if scope.EventBus == nil {
		t.Fatal("expected scope event bus")
	}
	if scope.Artifacts.RunID == "" {
		t.Fatal("expected run id to be allocated")
	}
	if _, err := os.Stat(scope.Artifacts.JobsDir); err != nil {
		t.Fatalf("expected jobs dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(scope.Artifacts.RunDir, AuditLogFileName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected disabled scope to skip audit log, got %v", err)
	}
}

func TestOpenRunScopeEnabledWithoutExtensionsCreatesEmptyManagerAndAuditLog(t *testing.T) {
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{})
	defer restoreDiscovery()

	cfg := runtimeConfigForTest(t)
	scope, err := OpenRunScope(context.Background(), cfg, OpenRunScopeOptions{
		EnableExecutableExtensions: true,
	})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	defer closeRunScopeForTest(t, scope)

	if !scope.ExtensionsEnabled {
		t.Fatal("expected executable extensions to be enabled")
	}
	if scope.Manager == nil {
		t.Fatal("expected manager to be constructed")
	}
	if got := scope.Manager.registry.Extensions(); len(got) != 0 {
		t.Fatalf("registered extensions = %d, want 0", len(got))
	}
	if _, err := os.Stat(filepath.Join(scope.Artifacts.RunDir, AuditLogFileName)); err != nil {
		t.Fatalf("expected audit log to exist: %v", err)
	}
}

func TestOpenRunScopeEnabledRegistersDiscoveredExtensionsWithoutStartingThem(t *testing.T) {
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{
		Extensions: []DiscoveredExtension{
			discoveredExtensionForTest("alpha", CapabilityPlanMutate),
			discoveredExtensionForTest("beta", CapabilityPromptMutate),
		},
	})
	defer restoreDiscovery()

	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{
		EnableExecutableExtensions: true,
	})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	defer closeRunScopeForTest(t, scope)

	registered := scope.Manager.registry.Extensions()
	if len(registered) != 2 {
		t.Fatalf("registered extensions = %d, want 2", len(registered))
	}
	gotNames := []string{registered[0].Name, registered[1].Name}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(gotNames, want) {
		t.Fatalf("registered names = %#v, want %#v", gotNames, want)
	}
	for _, extension := range registered {
		if extension.State() != ExtensionStateLoaded {
			t.Fatalf("extension %q state = %q, want %q", extension.Name, extension.State(), ExtensionStateLoaded)
		}
		if extension.Caller != nil {
			t.Fatalf("extension %q caller = %#v, want nil before task 08", extension.Name, extension.Caller)
		}
	}
}

func TestOpenRunScopeReturnsManagerConstructionError(t *testing.T) {
	restoreDiscovery := stubRunScopeDiscovery(t, DiscoveryResult{
		Extensions: []DiscoveredExtension{{Ref: Ref{Name: "broken", Source: SourceWorkspace}}},
	})
	defer restoreDiscovery()

	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{
		EnableExecutableExtensions: true,
	})
	if err == nil {
		t.Fatal("expected OpenRunScope() to fail")
	}
	if scope != nil {
		t.Fatalf("scope = %#v, want nil on error", scope)
	}
	if !strings.Contains(err.Error(), "missing manifest") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunScopeAccessorsReturnUnderlyingResources(t *testing.T) {
	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}
	defer closeRunScopeForTest(t, scope)

	if got := scope.RunArtifacts(); got != scope.Artifacts {
		t.Fatalf("RunArtifacts() = %#v, want %#v", got, scope.Artifacts)
	}
	if got := scope.RunJournal(); got != scope.Journal {
		t.Fatalf("RunJournal() = %p, want %p", got, scope.Journal)
	}
	if got := scope.RunEventBus(); got != scope.EventBus {
		t.Fatalf("RunEventBus() = %p, want %p", got, scope.EventBus)
	}
	if got := scope.RunManager(); got != nil {
		t.Fatalf("RunManager() = %#v, want nil", got)
	}
}

func TestRunScopeCloseOrdersManagerThenJournalThenBus(t *testing.T) {
	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	order := make([]string, 0, 3)
	restoreCloseHooks := stubRunScopeCloseHooks(t,
		func(ctx context.Context, runJournal *journal.Journal) error {
			order = append(order, "journal")
			return runJournal.Close(ctx)
		},
		func(ctx context.Context, bus *runtimeevents.Bus[runtimeevents.Event]) error {
			order = append(order, "bus")
			return bus.Close(ctx)
		},
	)
	defer restoreCloseHooks()

	scope.Manager = &Manager{
		shutdownHook: func(_ context.Context) error {
			order = append(order, "manager")
			if err := scope.Journal.Submit(
				context.Background(),
				runtimeEventForTest(scope.Artifacts.RunID),
			); err != nil {
				return fmt.Errorf("journal closed before manager shutdown: %w", err)
			}
			_, ch, unsubscribe := scope.EventBus.Subscribe()
			defer unsubscribe()
			select {
			case <-ch:
				return errors.New("bus closed before manager shutdown")
			default:
			}
			return nil
		},
	}

	if err := scope.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if want := []string{"manager", "journal", "bus"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("close order = %#v, want %#v", order, want)
	}
}

func TestRunScopeCloseIsSafeWhenManagerIsNil(t *testing.T) {
	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	if err := scope.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestRunScopeCloseReturnsDeadlineExceededAndEscalatesCleanup(t *testing.T) {
	scope, err := OpenRunScope(context.Background(), runtimeConfigForTest(t), OpenRunScopeOptions{})
	if err != nil {
		t.Fatalf("OpenRunScope() error = %v", err)
	}

	order := make([]string, 0, 3)
	restoreCloseHooks := stubRunScopeCloseHooks(t,
		func(ctx context.Context, runJournal *journal.Journal) error {
			order = append(order, "journal")
			return runJournal.Close(ctx)
		},
		func(ctx context.Context, bus *runtimeevents.Bus[runtimeevents.Event]) error {
			order = append(order, "bus")
			return bus.Close(ctx)
		},
	)
	defer restoreCloseHooks()

	scope.Manager = &Manager{
		shutdownHook: func(ctx context.Context) error {
			order = append(order, "manager")
			<-ctx.Done()
			return ctx.Err()
		},
	}

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = scope.Close(closeCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close() error = %v, want context deadline exceeded", err)
	}
	if want := []string{"manager", "journal", "bus"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("close order = %#v, want %#v", order, want)
	}
	if err := scope.Journal.Submit(
		context.Background(),
		runtimeEventForTest(scope.Artifacts.RunID),
	); !errors.Is(
		err,
		journal.ErrClosed,
	) {
		t.Fatalf("journal submit error = %v, want journal.ErrClosed", err)
	}
	_, ch, _ := scope.EventBus.Subscribe()
	select {
	case <-ch:
	default:
		t.Fatal("expected bus subscribe channel to be closed after Close")
	}
}

func TestManagerShutdownClosesAuditAndTransitionsStates(t *testing.T) {
	runtimeExtension := &RuntimeExtension{
		Name:         "alpha",
		Capabilities: NewCapabilityChecker(nil),
	}
	runtimeExtension.SetState(ExtensionStateLoaded)
	registry, err := NewRegistry(runtimeExtension)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	audit := &AuditLogger{}
	if err := audit.Open(t.TempDir()); err != nil {
		t.Fatalf("audit.Open() error = %v", err)
	}
	manager := &Manager{
		registry: registry,
		audit:    audit,
	}

	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if got := runtimeExtension.State(); got != ExtensionStateStopped {
		t.Fatalf("extension state = %q, want %q", got, ExtensionStateStopped)
	}
	if err := audit.Record(AuditEntry{
		Timestamp:  time.Now().UTC(),
		Extension:  "alpha",
		Direction:  AuditDirectionHostToExt,
		Method:     executeHookMethod,
		Capability: CapabilityPromptMutate,
		Result:     AuditResultOK,
	}); !errors.Is(err, ErrAuditLoggerClosed) {
		t.Fatalf("audit.Record() error = %v, want ErrAuditLoggerClosed", err)
	}
}

func TestRuntimeHelpersHandleNilAndInvalidInputs(t *testing.T) {
	var scope *RunScope
	if got := scope.RunArtifacts(); got != (model.RunArtifacts{}) {
		t.Fatalf("RunArtifacts() = %#v, want zero value", got)
	}
	if scope.RunJournal() != nil {
		t.Fatal("expected nil run journal")
	}
	if scope.RunEventBus() != nil {
		t.Fatal("expected nil run event bus")
	}
	if scope.RunManager() != nil {
		t.Fatal("expected nil run manager")
	}
	if err := scope.Close(context.Background()); err != nil {
		t.Fatalf("Close(background) error = %v", err)
	}

	var manager *Manager
	if err := manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown(background) error = %v", err)
	}
	if err := manager.closeAudit(context.Background()); err != nil {
		t.Fatalf("closeAudit(nil manager) error = %v", err)
	}
	if got := contextWithoutCancel(context.Background()); got == nil {
		t.Fatal("expected background context")
	}

	if scope, err := OpenRunScope(context.Background(), nil, OpenRunScopeOptions{}); err == nil {
		t.Fatalf("expected nil config to fail, got scope %#v", scope)
	}
	if manager, err := newRunScopeManager(context.Background(), nil, &RunScope{}); err == nil {
		t.Fatalf("expected nil config to fail, got manager %#v", manager)
	}
	if manager, err := newRunScopeManager(context.Background(), runtimeConfigForTest(t), nil); err == nil {
		t.Fatalf("expected nil scope to fail, got manager %#v", manager)
	}
}

func runtimeConfigForTest(t *testing.T) *model.RuntimeConfig {
	t.Helper()
	isolateRunScopeHome(t)

	return &model.RuntimeConfig{
		WorkspaceRoot: t.TempDir(),
		Name:          "demo",
		Mode:          model.ExecutionModePRDTasks,
		IDE:           model.IDECodex,
		DryRun:        true,
	}
}

func isolateRunScopeHome(t testing.TB) {
	t.Helper()

	homeDir, err := os.MkdirTemp("", "cmp-ext-home-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	runScopeTestHomeMu.Lock()
	previousHome, hadPreviousHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", homeDir); err != nil {
		runScopeTestHomeMu.Unlock()
		_ = os.RemoveAll(homeDir)
		t.Fatalf("Setenv(HOME) error = %v", err)
	}
	t.Cleanup(func() {
		if hadPreviousHome {
			_ = os.Setenv("HOME", previousHome)
		} else {
			_ = os.Unsetenv("HOME")
		}
		runScopeTestHomeMu.Unlock()
		_ = os.RemoveAll(homeDir)
	})
}

func discoveredExtensionForTest(name string, capabilities ...Capability) DiscoveredExtension {
	root := filepath.Join(string(os.PathSeparator), "tmp", name)
	manifest := &Manifest{
		Extension: ExtensionInfo{
			Name:              name,
			Version:           "1.0.0",
			MinCompozyVersion: "0.0.0",
		},
		Security: SecurityConfig{Capabilities: capabilities},
	}
	return DiscoveredExtension{
		Ref:          Ref{Name: name, Source: SourceWorkspace},
		Manifest:     manifest,
		ExtensionDir: root,
		ManifestPath: filepath.Join(root, "compozy.toml"),
		Enabled:      true,
	}
}

func runtimeEventForTest(runID string) runtimeevents.Event {
	return runtimeevents.Event{
		SchemaVersion: runtimeevents.SchemaVersion,
		RunID:         runID,
		Timestamp:     time.Now().UTC(),
		Kind:          runtimeevents.EventKindExtensionEvent,
		Payload:       json.RawMessage(`{}`),
	}
}

func closeRunScopeForTest(t *testing.T, scope *RunScope) {
	t.Helper()

	if scope == nil {
		return
	}
	if err := scope.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func stubRunScopeDiscovery(t *testing.T, result DiscoveryResult) func() {
	t.Helper()

	original := discoverRunScopeExtensions
	discoverRunScopeExtensions = func(context.Context, *model.RuntimeConfig) (DiscoveryResult, error) {
		return result, nil
	}
	return func() {
		discoverRunScopeExtensions = original
	}
}

func stubRunScopeCloseHooks(
	t *testing.T,
	closeJournal func(context.Context, *journal.Journal) error,
	closeBus func(context.Context, *runtimeevents.Bus[runtimeevents.Event]) error,
) func() {
	t.Helper()

	originalJournal := closeRunScopeJournal
	originalBus := closeRunScopeBus
	closeRunScopeJournal = closeJournal
	closeRunScopeBus = closeBus
	return func() {
		closeRunScopeJournal = originalJournal
		closeRunScopeBus = originalBus
	}
}
