package extensions

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/subprocess"
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

// Manager owns the extension runtime state for one run.
type Manager struct {
	runID           string
	parentRunID     string
	workspaceRoot   string
	invokingCommand string
	journal         *journal.Journal
	eventBus        *events.Bus[events.Event]
	registry        *Registry
	dispatcher      *HookDispatcher
	hostAPI         *HostAPIRouter
	audit           *AuditLogger

	mu              sync.RWMutex
	backgroundCtx   context.Context
	backgroundStop  context.CancelFunc
	eventSubID      events.SubID
	eventUnsub      func()
	subprocs        map[string]*subprocess.Process
	sessions        map[string]*extensionSession
	started         bool
	startErr        error
	backgroundGroup sync.WaitGroup

	shutdownOnce sync.Once
	shutdownErr  error
	shutdownHook func(context.Context) error
}

var _ model.RuntimeManager = (*Manager)(nil)

type managerConfig struct {
	RunID           string
	ParentRunID     string
	WorkspaceRoot   string
	InvokingCommand string
	Journal         *journal.Journal
	EventBus        *events.Bus[events.Event]
	AuditDir        string
}

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

	return newManagerForExtensions(managerConfig{
		RunID:           scope.Artifacts.RunID,
		ParentRunID:     cfg.ParentRunID,
		WorkspaceRoot:   cfg.WorkspaceRoot,
		InvokingCommand: invokingCommandForMode(cfg.Mode),
		Journal:         scope.Journal,
		EventBus:        scope.EventBus,
		AuditDir:        scope.Artifacts.RunDir,
	}, registeredExtensions)
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

func runtimeExtensionFromDeclaredProvider(provider DeclaredProvider) (*RuntimeExtension, error) {
	if provider.Manifest == nil {
		return nil, fmt.Errorf("register runtime extension %q: missing manifest", provider.Extension.Name)
	}

	extension := &RuntimeExtension{
		Name:         strings.TrimSpace(provider.Manifest.Extension.Name),
		Ref:          provider.Extension,
		Manifest:     provider.Manifest,
		ManifestPath: provider.ManifestPath,
		ExtensionDir: provider.ExtensionDir,
		Capabilities: NewCapabilityChecker(provider.Manifest.Security.Capabilities),
	}
	extension.SetState(ExtensionStateLoaded)
	if provider.Manifest.Subprocess != nil {
		extension.SetShutdownDeadline(provider.Manifest.Subprocess.ShutdownTimeout)
	}
	return extension, nil
}

func cloneRuntimeExtension(extension *RuntimeExtension) (*RuntimeExtension, error) {
	if extension == nil {
		return nil, fmt.Errorf("clone runtime extension: missing runtime extension")
	}
	if extension.Manifest == nil {
		return nil, fmt.Errorf("clone runtime extension %q: missing manifest", extension.normalizedName())
	}

	cloned := &RuntimeExtension{
		Name:         extension.normalizedName(),
		Ref:          extension.Ref,
		Manifest:     extension.Manifest,
		ManifestPath: extension.ManifestPath,
		ExtensionDir: extension.ExtensionDir,
		Capabilities: NewCapabilityChecker(extension.Manifest.Security.Capabilities),
	}
	cloned.SetState(ExtensionStateLoaded)
	if extension.Manifest.Subprocess != nil {
		cloned.SetShutdownDeadline(extension.Manifest.Subprocess.ShutdownTimeout)
	}
	return cloned, nil
}

func newManagerForExtensions(cfg managerConfig, extensions []*RuntimeExtension) (*Manager, error) {
	registry, err := NewRegistry(extensions...)
	if err != nil {
		return nil, fmt.Errorf("build runtime registry: %w", err)
	}

	var audit *AuditLogger
	if auditDir := strings.TrimSpace(cfg.AuditDir); auditDir != "" {
		audit = &AuditLogger{}
		if err := audit.Open(auditDir); err != nil {
			return nil, fmt.Errorf("open extension audit log: %w", err)
		}
	}

	dispatcher := NewHookDispatcher(registry, audit)
	hostAPI := NewHostAPIRouter(registry, audit)
	manager := &Manager{
		runID:           strings.TrimSpace(cfg.RunID),
		parentRunID:     strings.TrimSpace(cfg.ParentRunID),
		workspaceRoot:   strings.TrimSpace(cfg.WorkspaceRoot),
		invokingCommand: strings.TrimSpace(cfg.InvokingCommand),
		journal:         cfg.Journal,
		eventBus:        cfg.EventBus,
		registry:        registry,
		dispatcher:      dispatcher,
		hostAPI:         hostAPI,
		audit:           audit,
		subprocs:        make(map[string]*subprocess.Process),
		sessions:        make(map[string]*extensionSession),
	}

	ops, err := NewDefaultKernelOps(DefaultKernelOpsConfig{
		WorkspaceRoot:  cfg.WorkspaceRoot,
		RunID:          cfg.RunID,
		ParentRunID:    cfg.ParentRunID,
		EventBus:       cfg.EventBus,
		Journal:        cfg.Journal,
		RuntimeManager: manager,
	})
	if err != nil {
		if closeErr := manager.closeAudit(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("build host services: %w", err)
	}
	if err := RegisterHostServices(hostAPI, ops); err != nil {
		if closeErr := manager.closeAudit(context.Background()); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
		return nil, fmt.Errorf("register host services: %w", err)
	}

	return manager, nil
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
