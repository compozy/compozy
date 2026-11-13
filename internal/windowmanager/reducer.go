package windowmanager

import (
	"fmt"
	"time"
)

type reducer struct {
	generate      idGenerator
	config        Config
	focusedWindow *WindowID
	activeDesktop *DesktopID
	activeTabs    map[DesktopID]TabID
	now           func() time.Time
	changes       changeBuilder
}

type reduction struct {
	changed bool
	applied bool
	changes ChangeSet
}

func (r *reducer) reduce(s *Snapshot, command Command) (reduction, error) {
	beforeMembers := tabMemberCounts(s)
	var changed bool
	var err error
	switch c := command.(type) {
	case reuseWindowCommand:
		if _, ok := s.Windows[c.id]; !ok {
			return reduction{}, ErrWindowNotFound
		}
		return reduction{}, nil
	case noTopologyCommand:
		return reduction{}, nil
	case ReorderTabCommand:
		changed, err = r.reorderTab(s, c)
	case ActivateTabCommand:
		changed, err = r.activateTab(s, c)
	case PinWindowCommand:
		changed, err = r.pinWindow(s, c)
	case adjacentTabsCommand:
		changed, err = r.placeAdjacentTabs(s, c)
	case ArrangeLayoutCommand:
		changed, err = r.arrange(s, c)
	case ResizeLayoutCommand:
		changed, err = r.resize(s, c)
	case BalanceLayoutCommand:
		changed, err = r.balance(s, c)
	case UndoLayoutCommand:
		changed, err = r.undo(s)
	case RedoLayoutCommand:
		changed, err = r.redo(s)
	case ReplaceLayoutCommand:
		changed, err = r.replace(s, c)
	default:
		changed, err = r.reduceLifecycle(s, command)
	}
	if err != nil {
		return reduction{}, err
	}
	if changed {
		if err := r.ensureDesktopTabs(s); err != nil {
			return reduction{}, err
		}
	}
	r.recordTabTransitions(beforeMembers, tabMemberCounts(s))
	return reduction{changed: changed, applied: changed, changes: r.changes.result()}, nil
}

func (r *reducer) reduceLifecycle(s *Snapshot, command Command) (bool, error) {
	switch c := command.(type) {
	case CreateDesktopCommand:
		return r.createDesktop(s, c)
	case UpdateDesktopCommand:
		return r.updateDesktop(s, c)
	case ReorderDesktopCommand:
		return r.reorderDesktop(s, c)
	case DeleteDesktopCommand:
		return r.deleteDesktop(s, c)

	case OpenWindowCommand:
		return r.openWindow(s, c)
	case NavigateWindowCommand:
		return r.navigateWindow(s, c)
	case CloseWindowCommand:
		return r.closeWindow(s, c)
	case MoveWindowCommand:
		return r.moveWindow(s, c)
	case SwapWindowsCommand:
		return r.swapWindows(s, c)

	case ReopenCommand:
		return r.reopenWindow(s)

	case terminalRetirementCommand:
		return r.retireTerminalWindows(s, c.ids), nil
	case workspaceUnregisteredCommand:
		return r.retireUnregisteredWorkspaceWindows(s, c.binding)
	case sessionDeletionCommand:
		return r.retireDeletedSessionWindows(s, c.sessionID)
	default:
		return false, fmt.Errorf("unsupported durable command %T: %w", command, ErrInvalidCommand)
	}
}

type changeBuilder struct {
	desktops map[DesktopID]struct{}
	windows  map[WindowID]struct{}
	tabs     map[TabID]struct{}
	nodes    map[NodeID]struct{}
	split    map[TabID]struct{}
	unsplit  map[TabID]struct{}
}

func addChange[T comparable](values *map[T]struct{}, id T) {
	if *values == nil {
		*values = make(map[T]struct{})
	}
	(*values)[id] = struct{}{}
}
func (b *changeBuilder) desktop(id DesktopID) { addChange(&b.desktops, id) }
func (b *changeBuilder) window(id WindowID)   { addChange(&b.windows, id) }
func (b *changeBuilder) tab(id TabID)         { addChange(&b.tabs, id) }
func (b *changeBuilder) node(id NodeID)       { addChange(&b.nodes, id) }
func (b *changeBuilder) tabSplit(id TabID)    { addChange(&b.split, id) }
func (b *changeBuilder) tabUnsplit(id TabID)  { addChange(&b.unsplit, id) }
func (b *changeBuilder) result() ChangeSet {
	c := ChangeSet{}
	for id := range b.desktops {
		c.DesktopIDs = append(c.DesktopIDs, id)
	}
	for id := range b.windows {
		c.WindowIDs = append(c.WindowIDs, id)
	}
	for id := range b.tabs {
		c.TabIDs = append(c.TabIDs, id)
	}
	for id := range b.nodes {
		c.NodeIDs = append(c.NodeIDs, id)
	}
	for id := range b.split {
		c.TabSplit = append(c.TabSplit, id)
	}
	for id := range b.unsplit {
		c.TabUnsplit = append(c.TabUnsplit, id)
	}
	sortChangeSet(&c)
	return c
}
