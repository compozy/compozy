package network

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
)

const (
	// RuntimePeerID is the reserved first-party AGH runtime network peer.
	RuntimePeerID        = "agh.runtime"
	runtimePeerSessionID = "runtime:agh.runtime"
)

const (
	// StatusDisabled reports that the network runtime is intentionally disabled.
	StatusDisabled = "disabled"
	// StatusReady reports that Network is available with no Live participants.
	StatusReady = "ready"
	// StatusActive reports that Network is available with at least one Live participant.
	StatusActive = "active"
)

// Status is the manager-facing diagnostics snapshot.
type Status struct {
	Enabled              bool
	Status               string
	LocalPeers           int
	Channels             int
	MessagesSent         int64
	MessagesReceived     int64
	MessagesRejected     int64
	MessagesDelivered    int64
	WorkflowTaggedEvents int64
	HandoffTaggedEvents  int64
	OpenThreads          int64
	OpenDirectRooms      int64
	OpenWorkItems        int64
	ConversationMessages int64
	WorkTransitions      int64
	DirectResolves       int64
	KindMetrics          []KindMetric
	Metrics              []MetricSample
}

// WakeNotifier receives post-commit wake notifications. The durable task run
// remains authoritative if notification is delayed or lost.
type WakeNotifier interface {
	NotifyNetworkWake(store.CommittedNetworkNotification)
}

// ManagerOption customizes network manager construction.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	logger        *slog.Logger
	now           func() time.Time
	auditor       AuditWriter
	conversations store.NetworkConversationStore
	acceptance    store.NetworkAcceptanceStore
	inbox         store.NetworkInboxStore
	causation     store.NetworkCausationStore
	wakeNotifier  WakeNotifier
	hooks         HookDispatcher
}

type managedSession struct {
	sessionID            string
	peerID               string
	workspaceID          string
	channel              string
	ownerKey             string
	networkParticipation participation.Spec
}

// Manager owns live membership and durable-commit-then-notify dispatch.
type Manager struct {
	config       aghconfig.NetworkConfig
	logger       *slog.Logger
	now          func() time.Time
	lifecycleCtx context.Context
	cancel       context.CancelFunc

	peers         *PeerRegistry
	router        *Router
	auditor       AuditWriter
	conversations store.NetworkConversationStore
	acceptance    store.NetworkAcceptanceStore
	inbox         store.NetworkInboxStore
	causation     store.NetworkCausationStore
	wakeNotifier  WakeNotifier
	hooks         HookDispatcher
	stats         *runtimeStats

	mu       sync.Mutex
	sessions map[string]*managedSession
	waiters  map[string]map[chan struct{}]struct{}
	enabled  bool
	closed   bool
}

// WithManagerLogger overrides the logger used by the network manager.
func WithManagerLogger(logger *slog.Logger) ManagerOption {
	return func(opts *managerOptions) { opts.logger = logger }
}

// WithManagerClock overrides the manager clock, primarily for tests.
func WithManagerClock(now func() time.Time) ManagerOption {
	return func(opts *managerOptions) { opts.now = now }
}

// WithManagerAuditWriter injects a custom audit sink, primarily for tests.
func WithManagerAuditWriter(auditor AuditWriter) ManagerOption {
	return func(opts *managerOptions) { opts.auditor = auditor }
}

// WithManagerConversationStore injects the durable conversation repository.
func WithManagerConversationStore(conversations store.NetworkConversationStore) ManagerOption {
	return func(opts *managerOptions) { opts.conversations = conversations }
}

// WithManagerAcceptanceStore injects the atomic acceptance repository.
func WithManagerAcceptanceStore(acceptance store.NetworkAcceptanceStore) ManagerOption {
	return func(opts *managerOptions) { opts.acceptance = acceptance }
}

// WithManagerWakeNotifier injects the daemon-owned post-commit wake signal.
func WithManagerWakeNotifier(notifier WakeNotifier) ManagerOption {
	return func(opts *managerOptions) { opts.wakeNotifier = notifier }
}

// NewManager constructs the broker-free network runtime. Prompt execution is
// owned by the daemon NetworkWakeRunner.
func NewManager(
	ctx context.Context,
	cfg aghconfig.NetworkConfig,
	auditPath string,
	auditStore AuditStore,
	opts ...ManagerOption,
) (*Manager, error) {
	if ctx == nil {
		return nil, errors.New("network: manager context is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("network: validate manager config: %w", err)
	}
	options := resolveManagerOptions(opts...)
	lifecycleCtx, cancel := context.WithCancel(ctx)
	manager := &Manager{
		config: cfg, logger: options.logger, now: options.now,
		lifecycleCtx: lifecycleCtx, cancel: cancel,
		auditor: options.auditor, conversations: options.conversations,
		acceptance: options.acceptance, inbox: options.inbox, causation: options.causation,
		wakeNotifier: options.wakeNotifier, hooks: options.hooks,
		stats: newRuntimeStats(), sessions: make(map[string]*managedSession), enabled: cfg.Enabled,
		waiters: make(map[string]map[chan struct{}]struct{}),
	}
	if err := manager.initialize(cfg, auditPath, auditStore); err != nil {
		cancel()
		return nil, err
	}
	manager.logger.Info("network.started", "dispatch", "durable_in_process")
	return manager, nil
}

func resolveManagerOptions(opts ...ManagerOption) managerOptions {
	options := managerOptions{
		logger: slog.Default(),
		now:    func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	if options.logger == nil {
		options.logger = slog.Default()
	}
	if options.now == nil {
		options.now = func() time.Time { return time.Now().UTC() }
	}
	return options
}

func (m *Manager) initialize(cfg aghconfig.NetworkConfig, auditPath string, auditStore AuditStore) error {
	peers, err := NewPeerRegistry(WithPeerRegistryClock(m.now))
	if err != nil {
		return err
	}
	m.peers = peers
	m.resolveStoreDependencies(auditStore)
	var participantResolver ThreadParticipantResolver
	if resolver, ok := m.conversations.(ThreadParticipantResolver); ok {
		participantResolver = resolver
	}
	m.router, err = NewRouter(
		peers,
		cfg.MaxReplayAgeDuration(),
		WithRouterClock(m.now),
		WithRouterThreadParticipantResolver(participantResolver),
		WithRouterLogger(m.logger),
	)
	if err != nil {
		return err
	}
	if m.auditor == nil {
		m.auditor, err = NewAuditWriter(auditPath, auditStore)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) resolveStoreDependencies(candidate AuditStore) {
	if m.conversations == nil {
		if conversations, ok := candidate.(store.NetworkConversationStore); ok {
			m.conversations = conversations
		}
	}
	if m.acceptance == nil {
		if acceptance, ok := candidate.(store.NetworkAcceptanceStore); ok {
			m.acceptance = acceptance
		}
	}
	if m.inbox == nil {
		if inbox, ok := candidate.(store.NetworkInboxStore); ok {
			m.inbox = inbox
		}
	}
	if m.causation == nil {
		if causation, ok := candidate.(store.NetworkCausationStore); ok {
			m.causation = causation
		}
	}
}

// Status returns a diagnostics snapshot without transport credentials.
func (m *Manager) Status(ctx context.Context) (*Status, error) {
	if ctx == nil {
		return nil, errors.New("network: status context is required")
	}
	if m == nil {
		return nil, errors.New("network: manager is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	peers, err := m.ListPeers(ctx, "", "")
	if err != nil {
		return nil, err
	}
	channels, err := m.ListChannels(ctx, "")
	if err != nil {
		return nil, err
	}
	localPeers := 0
	for _, peer := range peers {
		if peer.Local {
			localPeers++
		}
	}
	stats := m.stats.snapshot()
	enabled := m.Enabled()
	status := StatusDisabled
	if enabled {
		status = StatusReady
		if localPeers > 0 {
			status = StatusActive
		}
	}
	return &Status{
		Enabled: enabled, Status: status, LocalPeers: localPeers,
		Channels:     len(channels),
		MessagesSent: stats.MessagesSent, MessagesReceived: stats.MessagesReceived,
		MessagesRejected: stats.MessagesRejected, MessagesDelivered: stats.MessagesDelivered,
		WorkflowTaggedEvents: stats.WorkflowTaggedEvents, HandoffTaggedEvents: stats.HandoffTaggedEvents,
		OpenThreads: stats.OpenThreads, OpenDirectRooms: stats.OpenDirectRooms,
		OpenWorkItems: stats.OpenWorkItems, ConversationMessages: stats.ConversationMessages,
		WorkTransitions: stats.WorkTransitions, DirectResolves: stats.DirectResolves,
		KindMetrics: stats.KindMetrics, Metrics: append([]MetricSample(nil), stats.Metrics...),
	}, nil
}

// SetEnabled updates runtime availability after the durable config transition commits.
func (m *Manager) SetEnabled(enabled bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.enabled = enabled
	m.mu.Unlock()
}

// Enabled reports the current runtime availability switch.
func (m *Manager) Enabled() bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// OnTurnEnd is retained for the current daemon interface. Wake execution is
// lease-driven and does not maintain an in-memory delivery queue.
func (m *Manager) OnTurnEnd(string) {}

// Shutdown stops live membership and releases all waiter goroutines.
func (m *Manager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		return errors.New("network: shutdown context is required")
	}
	if m == nil {
		return nil
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	// Cancel managed context before Leave so waiters observe shutdown first.
	m.cancel()
	sessionIDs := make([]string, 0, len(m.sessions))
	for sessionID := range m.sessions {
		sessionIDs = append(sessionIDs, sessionID)
	}
	m.sessions = make(map[string]*managedSession)
	for _, waiters := range m.waiters {
		for waiter := range waiters {
			close(waiter)
		}
	}
	m.waiters = make(map[string]map[chan struct{}]struct{})
	m.mu.Unlock()
	for _, sessionID := range sessionIDs {
		m.router.Leave(sessionID)
	}
	m.logger.Info("network.stopped", "sessions", len(sessionIDs))
	return nil
}

func (m *Manager) sessionSnapshot(sessionID string) (managedSession, bool) {
	if m == nil {
		return managedSession{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	runtime, ok := m.sessions[strings.TrimSpace(sessionID)]
	if !ok || runtime == nil {
		return managedSession{}, false
	}
	return *runtime, true
}

func (m *Manager) removeSessionRuntime(sessionID string) (managedSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target := strings.TrimSpace(sessionID)
	runtime, ok := m.sessions[target]
	if !ok || runtime == nil {
		return managedSession{}, false
	}
	delete(m.sessions, target)
	return *runtime, true
}
