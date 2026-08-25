package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/store"
	terminalpty "github.com/compozy/compozy/internal/terminal/pty"
	"github.com/compozy/compozy/internal/toolruntime"
)

const (
	maxTombstones       = 256
	idleTombstoneTTL    = time.Hour
	defaultReaperPeriod = time.Minute
)

type terminalKey struct {
	workspaceID string
	profileID   string
	id          ID
}

type terminalScope struct {
	workspaceID string
	profileID   string
}

type tombstone struct {
	key       terminalKey
	expiresAt time.Time
}

// Service is the daemon-owned terminal domain authority.
type Service struct {
	pty             terminalpty.PTY
	workspaces      WorkspaceResolver
	profileNames    ProfileNameResolver
	settings        SettingsProvider
	profiles        ProfileGuard
	typingGrants    TypingGrantAuthorizer
	execApprovals   ExecAuthorizer
	registerProcess processRegister
	events          *EventBus
	journal         Journal
	markers         MarkerConsumer
	logger          *slog.Logger
	now             func() time.Time
	entropy         io.Reader
	inputRequestTTL time.Duration

	mu             sync.RWMutex
	terminals      map[terminalKey]*session
	tombstones     map[terminalKey]tombstone
	tombstoneOrder []terminalKey
	pendingByScope map[terminalScope]int
	pendingDaemon  int
	closing        bool
	reaperCancel   context.CancelFunc
	reaperDone     chan struct{}
	shutdownDone   chan struct{}
	shutdownErr    error
	inputs         *inputRegistry
}

func NewManager(options ...Option) (*Service, error) {
	service := &Service{
		terminals:      make(map[terminalKey]*session),
		tombstones:     make(map[terminalKey]tombstone),
		pendingByScope: make(map[terminalScope]int),
		reaperDone:     make(chan struct{}),
		shutdownDone:   make(chan struct{}),
		inputs:         newInputRegistry(),
	}
	defaultServiceOptions(service)
	for _, option := range options {
		if option != nil {
			if err := option(service); err != nil {
				return nil, err
			}
		}
	}
	service.events.Observe(service.resolveInputRequestsOnClose)
	return service, nil
}

func (m *Service) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("terminal: lifecycle context is required")
	}
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return &Error{Code: "terminal_shutting_down", Message: "terminal manager is shutting down", Err: ErrShuttingDown}
	}
	if m.reaperCancel != nil {
		m.mu.Unlock()
		return nil
	}
	reaperCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	m.reaperCancel = cancel
	m.reaperDone = make(chan struct{})
	m.mu.Unlock()
	go m.reaper(reaperCtx, defaultReaperPeriod)
	return nil
}

func (m *Service) Handle(_ context.Context, workspaceID, profileID string, id ID) (Handle, error) {
	session, err := m.lookup(terminalKey{workspaceID: workspaceID, profileID: profileID, id: id})
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (m *Service) Get(ctx context.Context, workspaceID, profileID string, id ID) (*Info, error) {
	handle, err := m.Handle(ctx, workspaceID, profileID, id)
	if err != nil {
		return nil, err
	}
	info := handle.Info()
	return &info, nil
}

func (m *Service) List(_ context.Context, workspaceID string, scope store.ReadScope) ([]Info, error) {
	if err := scope.Validate(); err != nil {
		return []Info{}, nil
	}
	m.mu.RLock()
	items := make([]Info, 0)
	for key, session := range m.terminals {
		if key.workspaceID == workspaceID && scope.Matches(key.profileID) {
			items = append(items, session.Info())
		}
	}
	m.mu.RUnlock()
	slices.SortFunc(items, func(left, right Info) int {
		if left.CreatedAt.Equal(right.CreatedAt) {
			return strings.Compare(string(left.ID), string(right.ID))
		}
		if left.CreatedAt.Before(right.CreatedAt) {
			return -1
		}
		return 1
	})
	return items, nil
}

func (m *Service) Close(
	ctx context.Context,
	workspaceID string,
	id ID,
	actor Actor,
	signal Signal,
) (*Exit, error) {
	key := terminalKey{workspaceID: workspaceID, profileID: actor.ProfileID, id: id}
	session, err := m.lookup(key)
	if err != nil {
		return nil, err
	}
	if err := session.authorizeClose(actor); err != nil {
		return nil, err
	}
	return session.close(ctx, signal, "operator_close", actor)
}

func (m *Service) Journal() Journal { return m.journal }

func (m *Service) TerminalFor(string) (Manager, error) { return m, nil }

func (m *Service) Observe(fn func(context.Context, TerminalEvent)) { m.events.Observe(fn) }

func (m *Service) ArchiveProfile(ctx context.Context, profileID string) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("terminal: profile id is required")
	}
	return m.archiveTerminals(ctx, "profile_archived", "profile-lifecycle", func(key terminalKey) bool {
		return key.profileID == profileID
	})
}

func (m *Service) ArchiveWorkspace(ctx context.Context, workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("terminal: workspace id is required")
	}
	archiveErr := m.archiveTerminals(ctx, "workspace_deleted", "workspace-lifecycle", func(key terminalKey) bool {
		return key.workspaceID == workspaceID
	})
	var removeErr error
	if lifecycle, ok := m.journal.(interface {
		RemoveWorkspace(context.Context, string) error
	}); ok {
		removeErr = lifecycle.RemoveWorkspace(ctx, workspaceID)
	}
	return errors.Join(archiveErr, removeErr)
}

func (m *Service) archiveTerminals(
	ctx context.Context,
	reason, actorID string,
	matches func(terminalKey) bool,
) error {
	type archiveTarget struct {
		key  terminalKey
		item *session
	}
	m.mu.RLock()
	targets := make([]archiveTarget, 0)
	for key, item := range m.terminals {
		if matches(key) {
			targets = append(targets, archiveTarget{key: key, item: item})
		}
	}
	m.mu.RUnlock()
	var closeErrors []error
	for _, target := range targets {
		actor := Actor{Kind: ActorKindSystem, ID: actorID, ProfileID: target.key.profileID}
		if _, err := target.item.close(ctx, SignalHUP, reason, actor); err != nil && !errors.Is(err, ErrExited) {
			closeErrors = append(closeErrors, err)
			continue
		}
		m.removeWithTombstone(target.key, target.item, m.now().Add(idleTombstoneTTL))
	}
	return errors.Join(closeErrors...)
}

func (m *Service) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("terminal: shutdown context is required")
	}
	m.mu.Lock()
	if m.closing {
		done := m.shutdownDone
		m.mu.Unlock()
		return m.waitForShutdown(ctx, done)
	}
	m.closing = true
	cancel := m.reaperCancel
	targets := make([]shutdownTarget, 0, len(m.terminals))
	for key, item := range m.terminals {
		targets = append(targets, shutdownTarget{key: key, item: item})
	}
	done := m.shutdownDone
	m.mu.Unlock()
	go m.drain(cancel, targets)
	return m.waitForShutdown(ctx, done)
}

type shutdownTarget struct {
	key  terminalKey
	item *session
}

func (m *Service) waitForShutdown(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		m.mu.RLock()
		defer m.mu.RUnlock()
		return m.shutdownErr
	}
}

func (m *Service) drain(cancel context.CancelFunc, targets []shutdownTarget) {
	if cancel != nil {
		cancel()
		<-m.reaperDone
	}
	var closeErrors []error
	for _, target := range targets {
		actor := Actor{Kind: ActorKindSystem, ID: "daemon-shutdown", ProfileID: target.item.Info().ProfileID}
		if _, err := target.item.close(context.Background(), SignalHUP, "shutdown", actor); err != nil && !errors.Is(err, ErrExited) {
			closeErrors = append(closeErrors, err)
		}
	}
	for _, target := range targets {
		<-target.item.done
		m.removeWithTombstone(target.key, target.item, m.now().Add(idleTombstoneTTL))
	}
	if lifecycle, ok := m.journal.(interface {
		Shutdown(context.Context) error
	}); ok {
		if err := lifecycle.Shutdown(context.Background()); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	m.mu.Lock()
	m.shutdownErr = errors.Join(closeErrors...)
	close(m.shutdownDone)
	m.mu.Unlock()
}

func (m *Service) lookup(key terminalKey) (*session, error) {
	m.mu.RLock()
	item := m.terminals[key]
	stone, tombstoned := m.tombstones[key]
	now := m.now()
	m.mu.RUnlock()
	if item != nil {
		return item, nil
	}
	if tombstoned && now.Before(stone.expiresAt) {
		return nil, &Error{Code: "terminal_expired", Message: "terminal has expired", Err: ErrExpired}
	}
	return nil, &Error{Code: "terminal_not_found", Message: "terminal not found", Err: ErrNotFound}
}

func (m *Service) admit(ctx context.Context, workspaceID string, actor Actor) error {
	if strings.TrimSpace(workspaceID) == "" {
		return &Error{Code: "terminal_requires_workspace", Message: "terminal operations require a workspace", Err: ErrRequiresWorkspace}
	}
	if strings.TrimSpace(actor.ProfileID) == "" {
		return &Error{Code: "terminal_not_found", Message: "terminal profile is required", Err: ErrNotFound}
	}
	if m.profiles != nil {
		return m.profiles.EnsureAvailableID(ctx, actor.ProfileID)
	}
	return nil
}

func (m *Service) processRegistration(
	ctx context.Context,
	item *session,
	spec ProcSpec,
) (processCheckpoint, error) {
	if m.registerProcess == nil {
		return nil, nil
	}
	info := item.Info()
	pids := processIDs(item.proc)
	registerCtx := context.WithoutCancel(ctx)
	startedAt := time.Time{}
	if provider, ok := item.proc.(interface{ StartedAt() time.Time }); ok {
		startedAt = provider.StartedAt()
	}
	handle, err := m.registerProcess(registerCtx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceTerminal,
		Owner: toolruntime.ProcessOwner{
			SessionID:  info.ControllerSessionID(),
			TurnID:     info.ControllerRunID(),
			TerminalID: string(info.ID),
		},
		PID:            pids.pid,
		ProcessGroupID: pids.group,
		Command:        spec.Argv[0],
		Args:           append([]string(nil), spec.Argv[1:]...),
		Cwd:            spec.Cwd,
		StartedAt:      startedAt,
		Interrupt: func(interruptCtx context.Context, _ toolruntime.ProcessRecord) error {
			_, interruptErr := item.close(interruptCtx, SignalHUP, "runtime_interrupt", Actor{
				Kind: ActorKindSystem, ID: "toolruntime", ProfileID: info.ProfileID,
			})
			return interruptErr
		},
	})
	if err != nil {
		return nil, fmt.Errorf("terminal: register process %q: %w", info.ID, err)
	}
	return handle, nil
}

type processIDProvider interface {
	PID() int
	ProcessGroupID() int
}

type processIdentity struct{ pid, group int }

func processIDs(proc Proc) processIdentity {
	provider, ok := proc.(processIDProvider)
	if !ok {
		return processIdentity{}
	}
	return processIdentity{pid: provider.PID(), group: provider.ProcessGroupID()}
}

func (i Info) ControllerSessionID() string {
	if i.Controller == nil {
		return ""
	}
	return i.Controller.SessionID
}

func (i Info) ControllerRunID() string {
	if i.Controller == nil {
		return ""
	}
	return i.Controller.RunID
}

var _ Manager = (*Service)(nil)
