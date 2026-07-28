package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const defaultDesktopName = "Desktop 1"

// Manager coordinates semantic commands, transient clients, and subscriptions.
type Manager struct {
	repository         Repository
	workspaces         WorkspaceResolver
	layouts            LayoutResourceRegistry
	defaults           Config
	now                func() time.Time
	generate           idGenerator
	subscriptionBuffer int
	eventObserver      EventObserver
	workspaceConfig    WorkspaceConfigResolver
	defaultsMu         sync.RWMutex

	mu             sync.Mutex
	closed         bool
	workspaceLocks map[WorkspaceID]*sync.Mutex
	clients        map[WorkspaceID]map[ClientID]ClientView
	hubs           map[WorkspaceID]*subscriptionHub
}

// UpdateDefaults atomically replaces validated runtime defaults for future commands.
func (m *Manager) UpdateDefaults(defaults Config) error {
	canonical, err := canonicalConfig(defaults)
	if err != nil {
		return fmt.Errorf("validate window manager defaults: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return ErrClosed
	}
	m.defaultsMu.Lock()
	m.defaults = canonical
	m.defaultsMu.Unlock()
	return nil
}

func (m *Manager) defaultConfig() Config {
	m.defaultsMu.RLock()
	defer m.defaultsMu.RUnlock()
	return cloneConfig(m.defaults)
}

var _ Service = (*Manager)(nil)

// NewService constructs the daemon-authoritative window manager.
func NewService(
	repository Repository,
	workspaces WorkspaceResolver,
	layouts LayoutResourceRegistry,
	defaults Config,
	options ...Option,
) (*Manager, error) {
	if repository == nil {
		return nil, errors.New("window manager repository is required")
	}
	if workspaces == nil {
		return nil, errors.New("window manager workspace resolver is required")
	}
	if layouts == nil {
		layouts = emptyLayoutRegistry{}
	}
	canonical, err := canonicalConfig(defaults)
	if err != nil {
		return nil, fmt.Errorf("validate window manager defaults: %w", err)
	}
	resolved, err := resolveOptions(options)
	if err != nil {
		return nil, err
	}
	return &Manager{
		repository:         repository,
		workspaces:         workspaces,
		layouts:            layouts,
		defaults:           canonical,
		now:                resolved.now,
		generate:           resolved.generate,
		subscriptionBuffer: resolved.subscriptionBuffer,
		eventObserver:      resolved.eventObserver,
		workspaceConfig:    resolved.workspaceConfig,
		workspaceLocks:     make(map[WorkspaceID]*sync.Mutex),
		clients:            make(map[WorkspaceID]map[ClientID]ClientView),
		hubs:               make(map[WorkspaceID]*subscriptionHub),
	}, nil
}

// Snapshot returns a validated workspace aggregate.
func (m *Manager) Snapshot(ctx context.Context, workspaceID WorkspaceID) (Snapshot, error) {
	if err := m.resolveWorkspace(ctx, workspaceID); err != nil {
		return Snapshot{}, err
	}
	lock, err := m.lockFor(workspaceID)
	if err != nil {
		return Snapshot{}, err
	}
	lock.Lock()
	defer lock.Unlock()
	return m.loadSnapshot(ctx, workspaceID)
}

// Execute applies one semantic command through one compare-and-swap commit.
func (m *Manager) Execute(ctx context.Context, request CommandRequest) (Result, error) {
	if err := m.resolveWorkspace(ctx, request.WorkspaceID); err != nil {
		return Result{}, err
	}
	commandID, err := validateCommandRequest(request)
	if err != nil {
		return Result{}, err
	}
	lock, err := m.lockFor(request.WorkspaceID)
	if err != nil {
		return Result{}, err
	}
	lock.Lock()
	result, executeErr := func() (Result, error) {
		snapshot, loadErr := m.loadSnapshot(ctx, request.WorkspaceID)
		if loadErr != nil {
			return Result{}, loadErr
		}
		rebasedFrom, revisionErr := resolveExpectedRevision(&snapshot, request)
		if revisionErr != nil {
			return Result{}, revisionErr
		}
		if commandID == CommandDesktopSwitch || commandID == CommandWindowFocus {
			return m.executePresentation(snapshot, request, rebasedFrom)
		}
		return m.executeDurable(ctx, snapshot, request, commandID, rebasedFrom)
	}()
	lock.Unlock()
	if executeErr != nil {
		return Result{}, executeErr
	}
	if result.Applied {
		m.observeCommittedEvent(
			ctx,
			newCommandEvent(result.Snapshot, request, commandID, result.Changes),
		)
	}
	return result, nil
}

// Preview reduces and validates without persisting, publishing, or advancing history.
func (m *Manager) Preview(ctx context.Context, request CommandRequest) (Preview, error) {
	if err := m.resolveWorkspace(ctx, request.WorkspaceID); err != nil {
		return Preview{}, err
	}
	commandID, err := validateCommandRequest(request)
	if err != nil {
		return Preview{}, err
	}
	lock, err := m.lockFor(request.WorkspaceID)
	if err != nil {
		return Preview{}, err
	}
	lock.Lock()
	defer lock.Unlock()
	snapshot, err := m.loadSnapshot(ctx, request.WorkspaceID)
	if err != nil {
		return Preview{}, err
	}
	if _, err := resolveExpectedRevision(&snapshot, request); err != nil {
		return Preview{}, err
	}
	if commandID == CommandDesktopSwitch || commandID == CommandWindowFocus {
		return m.previewPresentation(snapshot, request)
	}
	client, err := m.clientForRequest(request, commandID)
	if err != nil {
		return Preview{}, err
	}
	workspaceDefaults, err := m.resolveWorkspaceDefaults(ctx, request.WorkspaceID)
	if err != nil {
		return Preview{}, err
	}
	payload, err := m.resolveLayoutResourceCommand(
		ctx,
		request.WorkspaceID,
		request.Payload,
		workspaceDefaults,
	)
	if err != nil {
		return Preview{}, err
	}
	config, err := effectiveConfig(workspaceDefaults, snapshot.Overrides)
	if err != nil {
		return Preview{}, err
	}
	working := cloneSnapshot(snapshot)
	var focused *WindowID
	if client != nil {
		focused = clonePointer(client.FocusedWindowID)
	}
	reduced, err := (&reducer{generate: m.generate, config: config, focusedWindow: focused}).reduce(
		&working,
		payload,
	)
	if err != nil {
		return Preview{}, err
	}
	var projectedClient *ClientView
	if reduced.changed {
		nextRevision, revisionErr := NextTopologyRevision(snapshot.Revision)
		if revisionErr != nil {
			return Preview{}, revisionErr
		}
		working = NormalizeSnapshot(working)
		if err := ValidateSnapshot(working); err != nil {
			return Preview{}, err
		}
		working.Revision = nextRevision
		projectedClient, err = m.previewDurableClient(snapshot, working, request)
		if err != nil {
			return Preview{}, err
		}
	}
	return Preview{
		Snapshot: cloneSnapshot(working), Changed: reduced.changed, Changes: reduced.changes,
		Client: projectedClient,
	}, nil
}

func (m *Manager) loadSnapshot(ctx context.Context, workspaceID WorkspaceID) (Snapshot, error) {
	snapshot, err := m.repository.Load(ctx, workspaceID)
	if errors.Is(err, ErrSnapshotNotFound) {
		return initialSnapshot(workspaceID), nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("load window snapshot: %w", err)
	}
	if snapshot.WorkspaceID != workspaceID {
		return Snapshot{}, fmt.Errorf("snapshot workspace mismatch: %w", ErrInvalidTopology)
	}
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, err
	}
	return cloneSnapshot(snapshot), nil
}

func initialSnapshot(workspaceID WorkspaceID) Snapshot {
	return Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: workspaceID,
		Desktops: []Desktop{
			{
				ID:       DesktopID("desktop-default"),
				Name:     defaultDesktopName,
				Purpose:  DesktopPurposeStandard,
				Groups:   []LayoutGroup{},
				Floating: []WindowID{},
			},
		},
		Windows: map[WindowID]Window{},
		History: History{Undo: []HistoryEntry{}, Redo: []HistoryEntry{}},
	}
}
