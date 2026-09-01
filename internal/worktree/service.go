package worktree

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
)

type Workspace struct {
	ID            string
	Name          string
	Root          string
	Worktrees     config.WorktreesConfig
	WorktreesRoot string
}

type WorkspaceResolver interface {
	ResolveWorktreeWorkspace(context.Context, string) (Workspace, error)
	ListWorktreeWorkspaces(context.Context) ([]Workspace, error)
}

type SessionGuard interface {
	HasActiveSession(context.Context, string, string) (bool, error)
}

type Service struct {
	store      Store
	runner     GitRunner
	capability *CapabilityGate
	resolver   WorkspaceResolver
	sessions   SessionGuard
	locks      *RepositoryLocks
	usage      *worktreeUsageLocks
	hooks      HookDispatcher
	events     EventSink
	forge      ForgeProvider
	config     config.WorktreesConfig
	root       string
	now        func() time.Time
	newID      func(string) (string, error)
	getenv     func(string) string
	environ    func() []string
	logger     *slog.Logger
	configMu   sync.RWMutex
	cacheMu    sync.Mutex
	discovery  map[string]discoveryCacheEntry
	cacheEpoch uint64
	catalogMu  sync.Mutex
	catalog    *catalogBroadcaster
	createMu   sync.Mutex
	creates    map[string]*createOperation
	exitMu     sync.Mutex
	exits      map[string]*exitOperationControl
}

type Option func(*Service)

func NewService(store Store, runner GitRunner, opts ...Option) *Service {
	s := &Service{
		store: store, runner: runner, capability: NewCapabilityGate(runner),
		locks: NewRepositoryLocks(8), now: time.Now, newID: generateID, getenv: os.Getenv, environ: os.Environ,
		usage:  newWorktreeUsageLocks(),
		logger: slog.Default(), discovery: make(map[string]discoveryCacheEntry),
		catalog: newCatalogBroadcaster(), creates: make(map[string]*createOperation),
		exits: make(map[string]*exitOperationControl),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func WithWorkspaceResolver(resolver WorkspaceResolver) Option {
	return func(s *Service) { s.resolver = resolver }
}

func WithSessionGuard(guard SessionGuard) Option {
	return func(s *Service) { s.sessions = guard }
}

func WithConfig(value config.WorktreesConfig, resolvedRoot string) Option {
	return func(s *Service) { s.config, s.root = value, resolvedRoot }
}

// Reconfigure applies global worktree settings to future operations. Existing
// registry rows retain their persisted paths and lifecycle state.
func (s *Service) Reconfigure(value config.WorktreesConfig, resolvedRoot string) {
	s.configMu.Lock()
	s.config, s.root = value, resolvedRoot
	s.configMu.Unlock()
	s.cacheMu.Lock()
	clear(s.discovery)
	s.cacheEpoch++
	s.cacheMu.Unlock()
}

func WithHooks(dispatcher HookDispatcher) Option {
	return func(s *Service) { s.hooks = dispatcher }
}

func WithEvents(sink EventSink) Option {
	return func(s *Service) { s.events = sink }
}

func WithForge(provider ForgeProvider) Option {
	return func(s *Service) { s.forge = provider }
}

func WithLogger(logger *slog.Logger) Option {
	return func(s *Service) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

func WithIDGenerator(generator func(string) (string, error)) Option {
	return func(s *Service) {
		if generator != nil {
			s.newID = generator
		}
	}
}

func WithCapabilityGate(gate *CapabilityGate) Option {
	return func(s *Service) {
		if gate != nil {
			s.capability = gate
		}
	}
}

func WithRepositoryLocks(locks *RepositoryLocks) Option {
	return func(s *Service) {
		if locks != nil {
			s.locks = locks
		}
	}
}

func (s *Service) resolveWorkspace(ctx context.Context, workspaceID string) (Workspace, error) {
	if s.resolver == nil {
		return Workspace{}, refusal(ErrNotFound, "workspace is unavailable")
	}
	return s.resolver.ResolveWorktreeWorkspace(ctx, workspaceID)
}

// Capability reports whether the Git dependency required by worktree mutations is ready.
func (s *Service) Capability(ctx context.Context) (CapabilityDiagnostic, error) {
	if s == nil || s.capability == nil {
		return CapabilityDiagnostic{Code: ErrGitUnavailable.Error()}, ErrGitUnavailable
	}
	return s.capability.Check(ctx)
}

func (s *Service) worktreeSettings(workspace Workspace) (config.WorktreesConfig, string) {
	s.configMu.RLock()
	value, root := s.config, s.root
	s.configMu.RUnlock()
	if workspace.Worktrees.RunBranchNamespace != "" {
		value = workspace.Worktrees
	}
	if workspace.WorktreesRoot != "" {
		root = workspace.WorktreesRoot
	}
	return value, root
}
