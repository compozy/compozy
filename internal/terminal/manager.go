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
	maxTombstones                  = 256
	defaultReaperPeriod            = time.Minute
	defaultJournalShutdownTimeout  = 5 * time.Second
	defaultTerminalShutdownTimeout = 30 * time.Second
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
	events          *Notifier
	journal         Journal
	logger          *slog.Logger
	now             func() time.Time
	entropy         io.Reader
	inputRequestTTL time.Duration

	mu                 sync.RWMutex
	terminals          map[terminalKey]*session
	tombstones         map[terminalKey]tombstone
	tombstoneOrder     []terminalKey
	pendingByScope     map[terminalScope]int
	pendingDaemon      int
	workspaceProducers map[string]int
	producerChanged    chan struct{}
	sealedWorkspaces   map[string]struct{}
	closing            bool
	reaperStop         chan struct{}
	reaperDone         chan struct{}
	shutdownDone       chan struct{}
	shutdownErr        error
	inputs             *inputRegistry
	tickets            *attachTicketRegistry
}

var _ Manager = (*Service)(nil)

func NewManager(options ...Option) (*Service, error) {
	service := &Service{
		terminals:          make(map[terminalKey]*session),
		tombstones:         make(map[terminalKey]tombstone),
		pendingByScope:     make(map[terminalScope]int),
		workspaceProducers: make(map[string]int),
		producerChanged:    make(chan struct{}),
		sealedWorkspaces:   make(map[string]struct{}),
		reaperDone:         make(chan struct{}),
		shutdownDone:       make(chan struct{}),
		inputs:             newInputRegistry(),
		tickets:            newAttachTicketRegistry(),
	}
	defaultServiceOptions(service)
	for _, option := range options {
		if option != nil {
			if err := option(service); err != nil {
				return nil, err
			}
		}
	}
	if service.journal == nil {
		return nil, errors.New("terminal: journal is required")
	}
	service.events.Observe(service.resolveInputRequestsOnClose)
	service.events.Observe(service.invalidateAttachTicketsOnClose)
	return service, nil
}

func (m *Service) Start(ctx context.Context) error {
	if ctx == nil {
		return errors.New("terminal: lifecycle context is required")
	}
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return fmt.Errorf("%s: %w", errorMessageShuttingDown, ErrShuttingDown)
	}
	if m.reaperStop != nil {
		m.mu.Unlock()
		return nil
	}
	m.reaperStop = make(chan struct{})
	reaperStop := m.reaperStop
	m.reaperDone = make(chan struct{})
	m.mu.Unlock()
	go m.reaper(ctx, reaperStop, defaultReaperPeriod)
	return nil
}

func (m *Service) Handle(ctx context.Context, workspaceID, profileID string, id ID) (Handle, error) {
	if err := requestContextError(ctx, "handle"); err != nil {
		return nil, err
	}
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

func (m *Service) List(ctx context.Context, workspaceID string, scope store.ReadScope) ([]Info, error) {
	if err := requestContextError(ctx, "list"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("terminal: validate list scope: %w", err)
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

func (m *Service) ActiveRecordings(
	ctx context.Context,
	workspaceID string,
	scope store.ReadScope,
) ([]RecordingRef, error) {
	if err := requestContextError(ctx, "list active recordings"); err != nil {
		return nil, err
	}
	if err := scope.Validate(); err != nil {
		return nil, fmt.Errorf("terminal: validate active recording scope: %w", err)
	}
	m.mu.RLock()
	items := make([]RecordingRef, 0)
	for key, session := range m.terminals {
		if key.workspaceID != workspaceID || !scope.Matches(key.profileID) {
			continue
		}
		if recording, ok := session.activeRecording(); ok {
			items = append(items, recording)
		}
	}
	m.mu.RUnlock()
	slices.SortFunc(items, func(left, right RecordingRef) int {
		if left.StartedAt.Equal(right.StartedAt) {
			return strings.Compare(string(left.TerminalID), string(right.TerminalID))
		}
		if left.StartedAt.Before(right.StartedAt) {
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

func (m *Service) Observe(fn func(context.Context, Event)) { m.events.Observe(fn) }

func (m *Service) ArchiveProfile(ctx context.Context, profileID string) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("terminal: profile id is required")
	}
	return m.archiveTerminals(ctx, "profile_archived", "profile-lifecycle", func(key terminalKey) bool {
		return key.profileID == profileID
	})
}

func (m *Service) ArchiveWorkspace(ctx context.Context, workspaceID string) error {
	preparation, err := m.PrepareWorkspaceRemoval(ctx, workspaceID)
	if err != nil {
		return err
	}
	if err := preparation.BeforeDelete(ctx); err != nil {
		return errors.Join(err, preparation.Rollback(context.WithoutCancel(ctx)))
	}
	return preparation.Commit(ctx)
}

func (m *Service) archiveTerminals(
	ctx context.Context,
	reason, actorID string,
	matches func(terminalKey) bool,
) error {
	m.mu.RLock()
	targets := make([]terminalLifecycleTarget, 0)
	for key, item := range m.terminals {
		if matches(key) {
			targets = append(targets, terminalLifecycleTarget{key: key, item: item})
		}
	}
	m.mu.RUnlock()
	return m.closeAndArchiveTerminals(ctx, targets, reason, actorID)
}

type terminalLifecycleTarget struct {
	key  terminalKey
	item *session
}

func (m *Service) closeAndArchiveTerminals(
	ctx context.Context,
	targets []terminalLifecycleTarget,
	reason, actorID string,
) error {
	slices.SortFunc(targets, func(left, right terminalLifecycleTarget) int {
		return compareTerminalKeys(left.key, right.key)
	})
	var closeErrors []error
	for _, target := range targets {
		actor := Actor{Kind: ActorKindSystem, ID: actorID, ProfileID: target.key.profileID}
		if _, err := target.item.close(ctx, SignalHUP, reason, actor); err != nil && !errors.Is(err, ErrExited) {
			closeErrors = append(closeErrors, err)
			continue
		}
		m.removeWithTombstone(target.key, target.item, m.tombstoneExpiry(target.item))
	}
	return errors.Join(closeErrors...)
}

func compareTerminalKeys(left, right terminalKey) int {
	return strings.Compare(
		left.workspaceID+"\x00"+left.profileID+"\x00"+string(left.id),
		right.workspaceID+"\x00"+right.profileID+"\x00"+string(right.id),
	)
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
	reaperStop := m.reaperStop
	done := m.shutdownDone
	m.mu.Unlock()
	drainCtx, cancelDrain := boundedCleanupContext(ctx, defaultTerminalShutdownTimeout)
	go func() {
		defer cancelDrain()
		m.drain(drainCtx, reaperStop)
	}()
	return m.waitForShutdown(ctx, done)
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

func (m *Service) drain(ctx context.Context, reaperStop chan struct{}) {
	if reaperStop != nil {
		close(reaperStop)
		<-m.reaperDone
	}
	m.mu.RLock()
	targets := make([]terminalLifecycleTarget, 0, len(m.terminals))
	for key, item := range m.terminals {
		targets = append(targets, terminalLifecycleTarget{key: key, item: item})
	}
	m.mu.RUnlock()
	closeErr := m.closeAndArchiveTerminals(ctx, targets, "shutdown", "daemon-shutdown")
	closeErrors := []error{closeErr}
	if closeErr == nil {
		closeErrors = append(closeErrors, m.waitAllProducers(ctx))
	}
	if errors.Join(closeErrors...) == nil {
		journalCtx, cancelJournal := context.WithTimeout(ctx, defaultJournalShutdownTimeout)
		if err := m.journal.Shutdown(journalCtx); err != nil {
			closeErrors = append(closeErrors, err)
		}
		cancelJournal()
	}
	m.mu.Lock()
	m.shutdownErr = errors.Join(closeErrors...)
	close(m.shutdownDone)
	m.mu.Unlock()
}

func (m *Service) tombstoneExpiry(item *session) time.Time {
	item.mu.RLock()
	retention := item.policy.ExitRetention
	item.mu.RUnlock()
	return m.now().Add(retention)
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
		return nil, &Error{Code: ErrorCodeExpired, Message: errorMessageExpired, Err: ErrExpired}
	}
	return nil, &Error{Code: ErrorCodeNotFound, Message: errorMessageNotFound, Err: ErrNotFound}
}

func (m *Service) admit(ctx context.Context, workspaceID string, actor Actor) error {
	if strings.TrimSpace(workspaceID) == "" {
		return &Error{
			Code:    ErrorCodeRequiresWorkspace,
			Message: "terminal operations require a workspace",
			Err:     ErrRequiresWorkspace,
		}
	}
	if strings.TrimSpace(actor.ProfileID) == "" {
		return errors.New("terminal: actor profile is required")
	}
	if actor.Kind == ActorKindAgent && !validRunActor(actor) {
		return fmt.Errorf("terminal agent run identity is incomplete: %w", ErrRunIdentityIncomplete)
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
	handle, err := m.registerProcess(ctx, toolruntime.RegisterConfig{
		Source: toolruntime.ProcessSourceTerminal,
		Owner: toolruntime.ProcessOwner{
			SessionID:  info.ControllerSessionID(),
			RunID:      info.ControllerRunID(),
			Generation: info.ControllerGeneration(),
			TerminalID: string(info.ID),
		},
		PID:            item.proc.PID(),
		ProcessGroupID: item.proc.ProcessGroupID(),
		Command:        spec.Argv[0],
		Args:           append([]string(nil), spec.Argv[1:]...),
		Cwd:            spec.Cwd,
		StartedAt:      item.proc.StartedAt(),
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

func (i Info) ControllerGeneration() int64 {
	if i.Controller == nil {
		return 0
	}
	return i.Controller.Generation
}
