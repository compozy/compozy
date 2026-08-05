package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb"
)

const (
	defaultLifecycleTimeout = 5 * time.Second
	defaultPromptBufferSize = 128
)

var (
	// ErrSessionNotFound reports that the requested active session does not exist.
	ErrSessionNotFound = errors.New("session: session not found")
	// ErrSessionNotActive reports that a known session cannot accept live approvals or prompts.
	ErrSessionNotActive = errors.New("session: session is not active")
	// ErrPendingPermissionNotFound reports that no waiting permission matched the approval request.
	ErrPendingPermissionNotFound = errors.New("session: pending permission not found")
	// ErrPendingPermissionConflict reports that the approval request matched multiple pending permissions.
	ErrPendingPermissionConflict = errors.New("session: pending permission lookup is ambiguous")
	// ErrInvalidPermissionDecision reports an approval decision unsupported by the pending provider request.
	ErrInvalidPermissionDecision = errors.New("session: invalid permission decision")
	// ErrInvalidRuntimeOverride reports that a session runtime override is invalid.
	ErrInvalidRuntimeOverride = errors.New("session: invalid runtime override")
	// ErrValidation reports structurally invalid session creation input.
	ErrValidation = errors.New("session: validation failed")
)

// NewManager constructs a session manager with sensible defaults.
func NewManager(opts ...Option) (*Manager, error) {
	homePaths, err := compozyconfig.ResolveHomePaths()
	if err != nil {
		return nil, fmt.Errorf("session: resolve home paths: %w", err)
	}

	manager := &Manager{
		sessions:             make(map[string]*Session),
		pending:              make(map[string]sessionReservation),
		finalizing:           make(map[string]*sessionFinalization),
		clearFinalizing:      make(map[string]chan struct{}),
		promptDrains:         make(map[chan struct{}]struct{}),
		managedInputLeases:   make(map[string]managedInputLease),
		promptAdmissionLocks: make(map[string]*promptAdmissionLock),
		compactionLifecycle: sessionCompactionLifecycle{
			runs: make(map[string]*sessionCompactionState),
		},
		startLifecycle: sessionStartLifecycle{
			runs: make(map[string]*sessionStartRun),
		},
		resumeLifecycle: sessionResumeLifecycle{
			runs: make(map[string]*sessionResumeRun),
		},
		syntheticQueues:       make(map[string][]queuedSyntheticPrompt),
		syntheticDispatching:  make(map[string]bool),
		soulLocks:             make(map[string]chan struct{}),
		sessionHealthHookLast: make(map[string]time.Time),
		streamEvents:          newSessionEventBroadcaster(),
		catalogEvents:         newSessionCatalogBroadcaster(),
		logger:                slog.Default(),
		driver:                NewACPDriverAdapter(acp.New()),
		homePaths:             homePaths,
		readSessionMeta:       store.ReadSessionMeta,
		openStore: func(ctx context.Context, owner store.SessionDBOwner, path string) (EventRecorder, error) {
			return sessiondb.OpenSessionDB(ctx, owner, path)
		},
		supervision:                  compozyconfig.DefaultSessionSupervisionConfig(),
		busyInput:                    compozyconfig.DefaultSessionBusyInputConfig(),
		compaction:                   compozyconfig.DefaultSessionCompactionConfig(),
		sessionHealthStaleAfter:      compozyconfig.DefaultHeartbeatConfig().SessionHealthStaleAfter,
		sessionHealthHookMinInterval: compozyconfig.DefaultHeartbeatConfig().SessionHealthHookMinInterval,
		now: func() time.Time {
			return time.Now().UTC()
		},
		newSessionID:                newIDGenerator("sess"),
		newSandboxID:                newIDGenerator("env"),
		newTurnID:                   newIDGenerator("turn"),
		newRepairEventID:            newIDGenerator("ev"),
		acquireSessionDBFamilyLease: sessiondb.AcquireFamilyLease,
		promptBufSize:               defaultPromptBufferSize,
		soulRefreshTimeout:          defaultLifecycleTimeout,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(manager)
		}
	}

	if err := manager.applyRuntimeDefaults(); err != nil {
		return nil, err
	}
	if err := compozyconfig.EnsureHomeLayout(manager.homePaths); err != nil {
		return nil, fmt.Errorf("session: ensure home layout: %w", err)
	}
	if err := manager.startRuntimeOwners(); err != nil {
		return nil, err
	}
	manager.cleanupDeleteTombstones()

	return manager, nil
}

// Get returns the active in-memory session by id.
func (m *Manager) Get(id string) (*Session, bool) {
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[target]
	return session, ok
}

func (m *Manager) isPending(id string) bool {
	target := strings.TrimSpace(id)
	if target == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.pending[target]; ok {
		return true
	}
	_, ok := m.clearFinalizing[target]
	return ok
}

func (m *Manager) beginClearFinalization(id string) error {
	target := strings.TrimSpace(id)
	if target == "" {
		return errors.New("session: clear finalization session id is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.clearFinalizing[target]; exists {
		return fmt.Errorf("%w: %s", ErrSessionNotActive, target)
	}
	if m.clearFinalizing == nil {
		m.clearFinalizing = make(map[string]chan struct{})
	}
	m.clearFinalizing[target] = make(chan struct{})
	return nil
}

func (m *Manager) endClearFinalization(id string) {
	target := strings.TrimSpace(id)
	m.mu.Lock()
	done, exists := m.clearFinalizing[target]
	if exists {
		delete(m.clearFinalizing, target)
		close(done)
	}
	m.mu.Unlock()
}

func (m *Manager) waitForClearFinalization(ctx context.Context, id string) error {
	target := strings.TrimSpace(id)
	m.mu.RLock()
	done := m.clearFinalizing[target]
	m.mu.RUnlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("session: wait for clear finalization for %q: %w", target, ctx.Err())
	}
}

// SetNetworkPeerLifecycle installs the late-bound network join/leave callbacks
// used after session activation and before final stop cleanup.
func (m *Manager) SetNetworkPeerLifecycle(lifecycle NetworkPeerLifecycle) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.networkPeers = lifecycle
}

// SetTurnEndNotifier installs a post-construction callback invoked after each
// prompt turn finishes.
func (m *Manager) SetTurnEndNotifier(fn TurnEndNotifier) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.turnEndNotifier = fn
}

// IsPrompting reports whether the target session currently has an in-flight
// prompt setup or active turn.
func (m *Manager) IsPrompting(id string) bool {
	session, ok := m.Get(id)
	if !ok {
		return false
	}
	return session.IsPrompting()
}

func (m *Manager) currentNetworkPeerLifecycle() NetworkPeerLifecycle {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.networkPeers
}

func (m *Manager) currentTurnEndNotifier() TurnEndNotifier {
	if m == nil {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.turnEndNotifier
}

// List returns active in-memory sessions in stable order.
func (m *Manager) List() []*Info {
	m.mu.RLock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	infos := make([]*Info, 0, len(sessions))
	for _, session := range sessions {
		infos = append(infos, m.sessionInfoForRead(session))
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].CreatedAt.Equal(infos[j].CreatedAt) {
			return infos[i].ID < infos[j].ID
		}
		return infos[i].CreatedAt.Before(infos[j].CreatedAt)
	})

	return infos
}

func (m *Manager) lookup(id string) (*Session, error) {
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, errors.New("session: session id is required")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[target]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, target)
	}
	return session, nil
}

func (m *Manager) reserveStart(ctx context.Context, id string, workspaceID string) error {
	if ctx == nil {
		return errors.New("session: reserve start context is required")
	}
	targetWorkspace := strings.TrimSpace(workspaceID)
	if targetWorkspace == "" {
		return errors.New("session: reserve start workspace id is required")
	}

	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	resolver, err := m.requireWorkspaceResolver()
	if err != nil {
		return err
	}
	resolved, err := resolver.Resolve(ctx, targetWorkspace)
	if err != nil {
		return fmt.Errorf("session: revalidate workspace %q before start: %w", targetWorkspace, err)
	}
	if strings.TrimSpace(resolved.ID) != targetWorkspace {
		return fmt.Errorf(
			"session: revalidated workspace id %q does not match %q",
			strings.TrimSpace(resolved.ID),
			targetWorkspace,
		)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[id]; ok {
		return fmt.Errorf("session: session %q is already active", id)
	}
	if _, ok := m.pending[id]; ok {
		return fmt.Errorf("session: session %q is already pending", id)
	}

	m.pending[id] = sessionReservation{workspaceID: targetWorkspace}
	return nil
}

func (m *Manager) activate(session *Session) error {
	if session == nil {
		return errors.New("session: session is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if current, ok := m.sessions[session.ID]; ok {
		if current == session {
			return nil
		}
		return fmt.Errorf("session: session %q is already active", session.ID)
	}

	reservation, reserved := m.pending[session.ID]
	if !reserved {
		return fmt.Errorf("session: session %q has no start reservation", session.ID)
	}
	if reservation.workspaceID != strings.TrimSpace(session.WorkspaceID) {
		return fmt.Errorf(
			"session: session %q workspace %q does not match reserved workspace %q",
			session.ID,
			strings.TrimSpace(session.WorkspaceID),
			reservation.workspaceID,
		)
	}
	delete(m.pending, session.ID)
	m.sessions[session.ID] = session
	return nil
}

func (m *Manager) registerStarting(session *Session) error {
	if session == nil {
		return errors.New("session: starting session is required")
	}
	if session.Info().State != StateStarting {
		return fmt.Errorf("session: register starting %q: %w", session.ID, ErrInvalidStateTransition)
	}
	return m.activate(session)
}

func (m *Manager) pendingSessionForWorkspace(workspaceID string) (string, bool) {
	targetWorkspace := strings.TrimSpace(workspaceID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for sessionID, reservation := range m.pending {
		if reservation.workspaceID == targetWorkspace {
			return sessionID, true
		}
	}
	return "", false
}

func (m *Manager) releaseReservation(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.pending, id)
}

func (m *Manager) remove(id string) {
	target := strings.TrimSpace(id)
	m.mu.Lock()
	if finalization, ok := m.finalizing[target]; ok {
		close(finalization.done)
	}
	delete(m.sessions, target)
	delete(m.pending, target)
	delete(m.finalizing, target)
	m.mu.Unlock()

	m.soulLocksMu.Lock()
	delete(m.soulLocks, target)
	m.soulLocksMu.Unlock()

	m.emitDroppedSyntheticPrompts(m.takeQueuedSyntheticPrompts(target), ErrSessionNotFound)
}

func (m *Manager) removeActive(id string) {
	target := strings.TrimSpace(id)

	m.mu.Lock()
	delete(m.sessions, target)
	delete(m.pending, target)
	m.mu.Unlock()

	m.soulLocksMu.Lock()
	delete(m.soulLocks, target)
	m.soulLocksMu.Unlock()

	m.emitDroppedSyntheticPrompts(m.takeQueuedSyntheticPrompts(target), ErrSessionNotActive)
}

func (m *Manager) takeQueuedSyntheticPrompts(sessionID string) []queuedSyntheticPrompt {
	if m == nil {
		return nil
	}

	target := strings.TrimSpace(sessionID)
	if target == "" {
		return nil
	}

	m.syntheticMu.Lock()
	defer m.syntheticMu.Unlock()

	queue := append([]queuedSyntheticPrompt(nil), m.syntheticQueues[target]...)
	delete(m.syntheticQueues, target)
	delete(m.syntheticDispatching, target)
	return queue
}

func (m *Manager) emitDroppedSyntheticPrompts(items []queuedSyntheticPrompt, err error) {
	for _, item := range items {
		m.emitQueuedSyntheticDispatchError(item, err)
	}
}
