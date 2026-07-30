package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sync"
	"time"
)

const stackActiveQuietPeriod = 2 * time.Second

type activeCoalescer struct {
	mu      sync.Mutex
	pending map[WorkspaceID]map[NodeID]WindowID
	timers  map[WorkspaceID]*time.Timer
	cancels map[WorkspaceID]chan struct{}
	locks   map[WorkspaceID]*sync.Mutex
	closed  bool
	wg      sync.WaitGroup
	manager *Manager
	ctx     context.Context
}

func newActiveCoalescer(ctx context.Context, manager *Manager) *activeCoalescer {
	return &activeCoalescer{
		pending: make(map[WorkspaceID]map[NodeID]WindowID),
		timers:  make(map[WorkspaceID]*time.Timer),
		cancels: make(map[WorkspaceID]chan struct{}),
		locks:   make(map[WorkspaceID]*sync.Mutex),
		manager: manager,
		ctx:     ctx,
	}
}

func (c *activeCoalescer) bindWorkspace(workspaceID WorkspaceID, lock *sync.Mutex) {
	c.mu.Lock()
	if !c.closed {
		c.locks[workspaceID] = lock
	}
	c.mu.Unlock()
}

func (m *Manager) noteStackActive(workspaceID WorkspaceID, stackID NodeID, windowID WindowID) {
	m.coalescer.note(workspaceID, stackID, windowID)
}

func (c *activeCoalescer) note(workspaceID WorkspaceID, stackID NodeID, windowID WindowID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	workspacePending := c.pending[workspaceID]
	if workspacePending == nil {
		workspacePending = make(map[NodeID]WindowID)
		c.pending[workspaceID] = workspacePending
	}
	workspacePending[stackID] = windowID
	c.stopTimerLocked(workspaceID)
	lock := c.locks[workspaceID]
	if lock == nil {
		return
	}
	timer := time.NewTimer(stackActiveQuietPeriod)
	cancel := make(chan struct{})
	c.timers[workspaceID] = timer
	c.cancels[workspaceID] = cancel
	c.wg.Add(1)
	go c.runTimer(workspaceID, lock, timer, cancel)
}

func (c *activeCoalescer) runTimer(
	workspaceID WorkspaceID,
	lock *sync.Mutex,
	timer *time.Timer,
	cancel <-chan struct{},
) {
	defer c.wg.Done()
	triggered := false
	select {
	case <-timer.C:
		triggered = true
	case <-cancel:
	}
	if triggered {
		lock.Lock()
		err := c.manager.flushStackActiveLocked(c.ctx, workspaceID)
		lock.Unlock()
		if err != nil {
			slog.Warn("window manager stack activation flush failed", "workspace_id", workspaceID, "error", err)
		}
	}
	c.mu.Lock()
	if c.timers[workspaceID] == timer {
		delete(c.timers, workspaceID)
		delete(c.cancels, workspaceID)
	}
	c.mu.Unlock()
}

func (c *activeCoalescer) stopTimerLocked(workspaceID WorkspaceID) {
	if timer := c.timers[workspaceID]; timer != nil {
		timer.Stop()
		close(c.cancels[workspaceID])
		delete(c.timers, workspaceID)
		delete(c.cancels, workspaceID)
	}
}

func (c *activeCoalescer) hasPending(workspaceID WorkspaceID) bool {
	c.mu.Lock()
	hasPending := len(c.pending[workspaceID]) > 0
	c.mu.Unlock()
	return hasPending
}

func (m *Manager) flushStackActiveLocked(ctx context.Context, workspaceID WorkspaceID) error {
	return m.coalescer.flushLocked(ctx, workspaceID)
}

func (c *activeCoalescer) flushLocked(ctx context.Context, workspaceID WorkspaceID) error {
	c.mu.Lock()
	pending := clonePendingStackActive(c.pending[workspaceID])
	c.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	snapshot, err := c.manager.repository.Load(ctx, workspaceID)
	if errors.Is(err, ErrSnapshotNotFound) {
		c.clearMatching(workspaceID, pending)
		return nil
	}
	if err != nil {
		return fmt.Errorf("load stack activation snapshot: %w", err)
	}
	working := cloneSnapshot(snapshot)
	changes := ChangeSet{}
	for stackID, windowID := range pending {
		location, found := findStackByID(&working, stackID)
		if !found || !containsWindowID(location.members(), windowID) {
			continue
		}
		if location.activeID() != nil && *location.activeID() == windowID {
			continue
		}
		location.setActive(windowID)
		changes.NodeIDs = append(changes.NodeIDs, stackID)
		changes.WindowIDs = append(changes.WindowIDs, windowID)
	}
	if len(changes.NodeIDs) == 0 {
		c.clearMatching(workspaceID, pending)
		return nil
	}
	working = NormalizeSnapshot(working)
	if err := ValidateSnapshot(working); err != nil {
		return fmt.Errorf("validate stack activation snapshot: %w", err)
	}
	nextRevision, err := NextTopologyRevision(snapshot.Revision)
	if err != nil {
		return err
	}
	working.Revision = nextRevision
	working.UpdatedAt = c.manager.now().UTC()
	sortChangeSet(&changes)
	request := CommandRequest{
		WorkspaceID: workspaceID,
		CommandID:   CommandWindowStackSetActive,
		Actor:       Actor{Kind: "system", ID: "active_coalescer"},
		Origin:      "active_coalescer",
		Payload:     SetStackActiveCommand{},
	}
	event := newCommandEvent(working, request, CommandWindowStackSetActive, changes)
	if err := c.manager.repository.Commit(ctx, &Commit{
		WorkspaceID: workspaceID, ExpectedRevision: snapshot.Revision, Snapshot: working, Event: event,
	}); err != nil {
		return fmt.Errorf("commit stack activation: %w", err)
	}
	c.clearMatching(workspaceID, pending)
	c.manager.publishEvent(event)
	c.manager.observeCommittedEvent(ctx, event)
	return nil
}

func clonePendingStackActive(source map[NodeID]WindowID) map[NodeID]WindowID {
	cloned := make(map[NodeID]WindowID, len(source))
	maps.Copy(cloned, source)
	return cloned
}

func (c *activeCoalescer) clearMatching(workspaceID WorkspaceID, flushed map[NodeID]WindowID) {
	c.mu.Lock()
	current := c.pending[workspaceID]
	for stackID, windowID := range flushed {
		if current[stackID] == windowID {
			delete(current, stackID)
		}
	}
	if len(current) == 0 {
		delete(c.pending, workspaceID)
	}
	c.mu.Unlock()
}

func (c *activeCoalescer) discardWorkspaceLocked(workspaceID WorkspaceID) {
	c.mu.Lock()
	c.stopTimerLocked(workspaceID)
	delete(c.pending, workspaceID)
	c.mu.Unlock()
}

func (c *activeCoalescer) close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	workspaceIDs := make([]WorkspaceID, 0, len(c.pending))
	for workspaceID := range c.pending {
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	for workspaceID := range c.timers {
		c.stopTimerLocked(workspaceID)
	}
	locks := make(map[WorkspaceID]*sync.Mutex, len(c.locks))
	maps.Copy(locks, c.locks)
	c.mu.Unlock()
	slices.Sort(workspaceIDs)
	var flushErrors []error
	for _, workspaceID := range workspaceIDs {
		lock := locks[workspaceID]
		if lock == nil {
			continue
		}
		lock.Lock()
		err := c.flushLocked(c.ctx, workspaceID)
		lock.Unlock()
		if err != nil {
			flushErrors = append(flushErrors, fmt.Errorf("flush workspace %q: %w", workspaceID, err))
		}
	}
	c.wg.Wait()
	return errors.Join(flushErrors...)
}
