package extensions

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

// OpenRunScopeOptions controls whether executable extensions should be
// initialized as part of the early run bootstrap.
type OpenRunScopeOptions = model.OpenRunScopeOptions

// RunScope owns the run resources that must exist before planning starts.
type RunScope struct {
	Artifacts         model.RunArtifacts
	Journal           *journal.Journal
	EventBus          *events.Bus[events.Event]
	ExtensionsEnabled bool
	Manager           *Manager
}

var _ model.RunScope = (*RunScope)(nil)

// Manager owns the extension runtime state for one run. Task 07 only constructs
// the manager and prepares its registry/host API surface; subprocess startup
// remains part of task 08.
type Manager struct {
	runID         string
	workspaceRoot string
	journal       *journal.Journal
	eventBus      *events.Bus[events.Event]
	registry      *Registry
	dispatcher    *HookDispatcher
	hostAPI       *HostAPIRouter
	audit         *AuditLogger

	shutdownOnce sync.Once
	shutdownErr  error
	shutdownHook func(context.Context) error
}

var _ model.RuntimeManager = (*Manager)(nil)

var discoverRunScopeExtensions = func(ctx context.Context, cfg *model.RuntimeConfig) (DiscoveryResult, error) {
	return Discovery{WorkspaceRoot: cfg.WorkspaceRoot}.Discover(ctx)
}

var (
	closeRunScopeJournal = func(ctx context.Context, runJournal *journal.Journal) error {
		if runJournal == nil {
			return nil
		}
		return runJournal.Close(ctx)
	}
	closeRunScopeBus = func(ctx context.Context, bus *events.Bus[events.Event]) error {
		if bus == nil {
			return nil
		}
		return bus.Close(ctx)
	}
)

func init() {
	model.RegisterOpenRunScopeFactory(func(
		ctx context.Context,
		cfg *model.RuntimeConfig,
		opts model.OpenRunScopeOptions,
	) (model.RunScope, error) {
		return OpenRunScope(ctx, cfg, opts)
	})
}

// OpenRunScope allocates run artifacts, opens the journal, constructs the
// event bus, and optionally constructs the extension manager before planning.
func OpenRunScope(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	opts OpenRunScopeOptions,
) (*RunScope, error) {
	baseScope, err := model.OpenBaseRunScope(ctx, cfg)
	if err != nil {
		return nil, err
	}

	scope := &RunScope{
		Artifacts:         baseScope.Artifacts,
		Journal:           baseScope.Journal,
		EventBus:          baseScope.EventBus,
		ExtensionsEnabled: opts.EnableExecutableExtensions,
	}
	if !opts.EnableExecutableExtensions {
		return scope, nil
	}

	manager, err := newRunScopeManager(ctx, cfg, scope)
	if err != nil {
		if closeErr := scope.Close(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, err
	}
	scope.Manager = manager
	return scope, nil
}

// RunArtifacts reports the run artifact paths owned by the scope.
func (s *RunScope) RunArtifacts() model.RunArtifacts {
	if s == nil {
		return model.RunArtifacts{}
	}
	return s.Artifacts
}

// RunJournal reports the run journal owned by the scope.
func (s *RunScope) RunJournal() *journal.Journal {
	if s == nil {
		return nil
	}
	return s.Journal
}

// RunEventBus reports the run-scoped event bus.
func (s *RunScope) RunEventBus() *events.Bus[events.Event] {
	if s == nil {
		return nil
	}
	return s.EventBus
}

// RunManager reports the optional extension manager bound to the scope.
func (s *RunScope) RunManager() model.RuntimeManager {
	if s == nil || s.Manager == nil {
		return nil
	}
	return s.Manager
}

// Close tears down the optional manager, then flushes the journal, then closes
// the event bus.
func (s *RunScope) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}

	var closeErr error
	if s.Manager != nil {
		if err := s.Manager.Shutdown(ctx); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}

	cleanupCtx := contextWithoutCancel(ctx)
	if s.Journal != nil {
		if err := closeRunScopeJournal(cleanupCtx, s.Journal); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	if s.EventBus != nil {
		if err := closeRunScopeBus(cleanupCtx, s.EventBus); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}

	return closeErr
}

// Shutdown transitions the manager to draining, waits for observer hooks, and
// closes the audit log. More advanced process lifecycle work lands in task 08.
func (m *Manager) Shutdown(ctx context.Context) error {
	if m == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.shutdownOnce.Do(func() {
		m.setAllStates(ExtensionStateDraining)
		defer m.setAllStates(ExtensionStateStopped)

		if m.shutdownHook != nil {
			m.shutdownErr = m.shutdownHook(ctx)
			return
		}

		if err := m.waitForObservers(ctx); err != nil {
			m.shutdownErr = err
			if closeErr := m.closeAudit(contextWithoutCancel(ctx)); closeErr != nil {
				m.shutdownErr = errors.Join(m.shutdownErr, closeErr)
			}
			return
		}
		m.shutdownErr = m.closeAudit(ctx)
	})

	return m.shutdownErr
}

func newRunScopeManager(
	ctx context.Context,
	cfg *model.RuntimeConfig,
	scope *RunScope,
) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("open run scope manager: missing runtime config")
	}
	if scope == nil {
		return nil, fmt.Errorf("open run scope manager: missing run scope")
	}

	discovered, err := discoverRunScopeExtensions(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("discover extensions: %w", err)
	}

	registeredExtensions := make([]*RuntimeExtension, 0, len(discovered.Extensions))
	for i := range discovered.Extensions {
		runtimeExtension, err := runtimeExtensionFromDiscovered(discovered.Extensions[i])
		if err != nil {
			return nil, err
		}
		registeredExtensions = append(registeredExtensions, runtimeExtension)
	}

	registry, err := NewRegistry(registeredExtensions...)
	if err != nil {
		return nil, fmt.Errorf("build runtime registry: %w", err)
	}

	audit := &AuditLogger{}
	if err := audit.Open(scope.Artifacts.RunDir); err != nil {
		return nil, fmt.Errorf("open extension audit log: %w", err)
	}

	dispatcher := NewHookDispatcher(registry, audit)
	hostAPI := NewHostAPIRouter(registry, audit)
	ops, err := NewDefaultKernelOps(DefaultKernelOpsConfig{
		WorkspaceRoot: cfg.WorkspaceRoot,
		RunID:         scope.Artifacts.RunID,
		ParentRunID:   cfg.ParentRunID,
		EventBus:      scope.EventBus,
		Journal:       scope.Journal,
	})
	if err != nil {
		if closeErr := audit.Close(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("build host services: %w", err)
	}
	if err := RegisterHostServices(hostAPI, ops); err != nil {
		if closeErr := audit.Close(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("register host services: %w", err)
	}

	return &Manager{
		runID:         scope.Artifacts.RunID,
		workspaceRoot: cfg.WorkspaceRoot,
		journal:       scope.Journal,
		eventBus:      scope.EventBus,
		registry:      registry,
		dispatcher:    dispatcher,
		hostAPI:       hostAPI,
		audit:         audit,
	}, nil
}

func runtimeExtensionFromDiscovered(discovered DiscoveredExtension) (*RuntimeExtension, error) {
	if discovered.Manifest == nil {
		return nil, fmt.Errorf("register runtime extension %q: missing manifest", discovered.Ref.Name)
	}

	extension := &RuntimeExtension{
		Name:         discovered.Manifest.Extension.Name,
		Ref:          discovered.Ref,
		Manifest:     discovered.Manifest,
		ManifestPath: discovered.ManifestPath,
		ExtensionDir: discovered.ExtensionDir,
		Capabilities: NewCapabilityChecker(discovered.Manifest.Security.Capabilities),
	}
	extension.SetState(ExtensionStateLoaded)
	if discovered.Manifest.Subprocess != nil {
		extension.SetShutdownDeadline(discovered.Manifest.Subprocess.ShutdownTimeout)
	}
	return extension, nil
}

func (m *Manager) waitForObservers(ctx context.Context) error {
	if m == nil || m.dispatcher == nil {
		return nil
	}

	done := make(chan struct{})
	go func() {
		m.dispatcher.waitForObservers()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) closeAudit(ctx context.Context) error {
	if m == nil || m.audit == nil {
		return nil
	}
	return m.audit.Close(ctx)
}

func (m *Manager) setAllStates(state ExtensionState) {
	if m == nil || m.registry == nil {
		return
	}
	for _, extension := range m.registry.Extensions() {
		extension.SetState(state)
	}
}

func contextWithoutCancel(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
