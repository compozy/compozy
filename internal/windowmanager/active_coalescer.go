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

// systemActorKind attributes reducer commands the daemon issues on its own.
const systemActorKind = "system"

const tabActiveQuietPeriod = 2 * time.Second

type tabActiveFlushResult struct {
	From    Revision
	To      Revision
	Applied bool
}

type activeCoalescer struct {
	mu      sync.Mutex
	pending map[WorkspaceID]map[DesktopID]TabID
	timers  map[WorkspaceID]*time.Timer
	cancels map[WorkspaceID]chan struct{}
	locks   map[WorkspaceID]*workspaceLock
	closed  bool
	wg      sync.WaitGroup
	manager *Manager
	ctx     context.Context
}

func newActiveCoalescer(ctx context.Context, manager *Manager) *activeCoalescer {
	return &activeCoalescer{
		pending: make(map[WorkspaceID]map[DesktopID]TabID),
		timers:  make(map[WorkspaceID]*time.Timer),
		cancels: make(map[WorkspaceID]chan struct{}),
		locks:   make(map[WorkspaceID]*workspaceLock),
		manager: manager,
		ctx:     ctx,
	}
}

func (c *activeCoalescer) bindWorkspace(workspaceID WorkspaceID, lock *workspaceLock) {
	c.mu.Lock()
	if !c.closed {
		c.locks[workspaceID] = lock
	}
	c.mu.Unlock()
}

func (m *Manager) noteTabActive(workspaceID WorkspaceID, desktopID DesktopID, tabID TabID) {
	m.coalescer.note(workspaceID, desktopID, tabID)
}

func (c *activeCoalescer) note(workspaceID WorkspaceID, desktopID DesktopID, tabID TabID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	workspacePending := c.pending[workspaceID]
	if workspacePending == nil {
		workspacePending = make(map[DesktopID]TabID)
		c.pending[workspaceID] = workspacePending
	}
	workspacePending[desktopID] = tabID
	c.stopTimerLocked(workspaceID)
	lock := c.locks[workspaceID]
	if lock == nil {
		return
	}
	timer := time.NewTimer(tabActiveQuietPeriod)
	cancel := make(chan struct{})
	c.timers[workspaceID] = timer
	c.cancels[workspaceID] = cancel
	c.wg.Go(func() {
		c.runTimer(workspaceID, lock, timer, cancel)
	})
}

func (c *activeCoalescer) runTimer(
	workspaceID WorkspaceID,
	lock *workspaceLock,
	timer *time.Timer,
	cancel <-chan struct{},
) {
	triggered := false
	select {
	case <-timer.C:
		triggered = true
	case <-cancel:
	}
	if triggered {
		lock.Lock()
		_, err := c.manager.flushTabActiveLocked(c.ctx, workspaceID)
		lock.Unlock()
		if err != nil {
			slog.Warn("window manager tab activation flush failed", "workspace_id", workspaceID, "error", err)
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

func (m *Manager) flushTabActiveLocked(
	ctx context.Context,
	workspaceID WorkspaceID,
) (tabActiveFlushResult, error) {
	return m.coalescer.flushLocked(ctx, workspaceID)
}

func (c *activeCoalescer) flushLocked(
	ctx context.Context,
	workspaceID WorkspaceID,
) (tabActiveFlushResult, error) {
	c.mu.Lock()
	pending := clonePendingTabActive(c.pending[workspaceID])
	c.mu.Unlock()
	if len(pending) == 0 {
		return tabActiveFlushResult{}, nil
	}
	snapshot, err := c.manager.repository.Load(ctx, workspaceID)
	if errors.Is(err, ErrSnapshotNotFound) {
		c.clearMatching(workspaceID, pending)
		return tabActiveFlushResult{}, nil
	}
	if err != nil {
		return tabActiveFlushResult{}, fmt.Errorf("load tab activation snapshot: %w", err)
	}
	working := cloneSnapshot(snapshot)
	changes := ChangeSet{}
	for desktopID, tabID := range pending {
		di, ok := desktopIndexByID(&working, desktopID)
		if !ok {
			continue
		}
		d := &working.Desktops[di]
		if tabHoldingID(d.Tabs, tabID) < 0 || d.ActiveTabID == tabID {
			continue
		}
		d.ActiveTabID = tabID
		changes.DesktopIDs = append(changes.DesktopIDs, desktopID)
		changes.TabIDs = append(changes.TabIDs, tabID)
	}
	if len(changes.TabIDs) == 0 {
		c.clearMatching(workspaceID, pending)
		return tabActiveFlushResult{}, nil
	}

	working = NormalizeSnapshot(working)
	if err := ValidateSnapshot(working); err != nil {
		return tabActiveFlushResult{}, fmt.Errorf("validate tab activation snapshot: %w", err)
	}
	nextRevision, err := NextTopologyRevision(snapshot.Revision)
	if err != nil {
		return tabActiveFlushResult{}, err
	}
	working.Revision = nextRevision
	working.UpdatedAt = c.manager.now().UTC()
	sortChangeSet(&changes)
	request := CommandRequest{
		WorkspaceID: workspaceID,
		CommandID:   CommandWindowTabActivate,
		Actor:       Actor{Kind: systemActorKind, ID: "active_coalescer"},
		Origin:      "active_coalescer",
		Payload:     ActivateTabCommand{},
	}
	event := newCommandEvent(working, request, CommandWindowTabActivate, changes)
	if err := c.manager.repository.Commit(ctx, &Commit{
		WorkspaceID: workspaceID, ExpectedRevision: snapshot.Revision, Snapshot: working, Event: event,
	}); err != nil {
		return tabActiveFlushResult{}, fmt.Errorf("commit tab activation: %w", err)
	}
	c.clearMatching(workspaceID, pending)
	c.manager.publishEvent(event)
	c.manager.observeCommittedEvent(ctx, event)
	return tabActiveFlushResult{From: snapshot.Revision, To: working.Revision, Applied: true}, nil
}

func clonePendingTabActive(source map[DesktopID]TabID) map[DesktopID]TabID {
	cloned := make(map[DesktopID]TabID, len(source))
	maps.Copy(cloned, source)
	return cloned
}

func (c *activeCoalescer) clearMatching(workspaceID WorkspaceID, flushed map[DesktopID]TabID) {
	c.mu.Lock()
	current := c.pending[workspaceID]
	for desktopID, tabID := range flushed {
		if current[desktopID] == tabID {
			delete(current, desktopID)
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
	delete(c.locks, workspaceID)
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
	locks := make(map[WorkspaceID]*workspaceLock, len(c.locks))
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
		_, err := c.flushLocked(c.ctx, workspaceID)
		lock.Unlock()
		if err != nil {
			flushErrors = append(flushErrors, fmt.Errorf("flush workspace %q: %w", workspaceID, err))
		}
	}
	c.wg.Wait()
	return errors.Join(flushErrors...)
}
