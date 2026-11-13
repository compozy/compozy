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
	case CreateDesktopCommand:
		changed, err = r.createDesktop(s, c)
	case UpdateDesktopCommand:
		changed, err = r.updateDesktop(s, c)
	case ReorderDesktopCommand:
		changed, err = r.reorderDesktop(s, c)
	case DeleteDesktopCommand:
		changed, err = r.deleteDesktop(s, c)
	case OpenWindowCommand:
		changed, err = r.openWindow(s, c)
	case NavigateWindowCommand:
		changed, err = r.navigateWindow(s, c)
	case CloseWindowCommand:
		changed, err = r.closeWindow(s, c)
	case MoveWindowCommand:
		changed, err = r.moveWindow(s, c)
	case SwapWindowsCommand:
		changed, err = r.swapWindows(s, c)
	case ReorderTabCommand:
		changed, err = r.reorderTab(s, c)
	case ActivateTabCommand:
		changed, err = r.activateTab(s, c)
	case PinWindowCommand:
		changed, err = r.pinWindow(s, c)
	case ReopenCommand:
		changed, err = r.reopenWindow(s)
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
	case terminalRetirementCommand:
		changed = r.retireTerminalWindows(s, c.ids)
	case workspaceUnregisteredCommand:
		changed, err = r.retireUnregisteredWorkspaceWindows(s, c.binding)
	case sessionDeletionCommand:
		changed, err = r.retireDeletedSessionWindows(s, c.sessionID)
	default:
		return reduction{}, fmt.Errorf("unsupported durable command %T: %w", command, ErrInvalidCommand)
	}
	if err != nil {
		return reduction{}, err
	}
	if changed {
		if err := r.ensureDesktopTabs(s); err != nil {
			return reduction{}, err
		}
	}
	return reduction{changed: changed, applied: changed, changes: r.changes.result()}, nil
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
