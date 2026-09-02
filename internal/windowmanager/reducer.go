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
	now           func() time.Time
	changes       changeBuilder
}

type reduction struct {
	changed bool
	applied bool
	changes ChangeSet
}

func (r *reducer) reduce(snapshot *Snapshot, command Command) (reduction, error) {
	lifted := liftedZoomDesktops(snapshot)
	changed, reportsApplied, handled, err := r.reduceTopologyCommand(snapshot, command)
	if handled && err == nil && changed {
		r.releaseVacatedZoomDesktops(snapshot, lifted)
	}
	if !handled {
		changed, handled, err = r.reduceLayoutCommand(snapshot, command)
		reportsApplied = true
	}
	if err != nil {
		return reduction{}, err
	}
	if !handled {
		switch command.(type) {
		case SwitchDesktopCommand, FocusWindowCommand:
			return reduction{}, fmt.Errorf("presentation command reached durable reducer: %w", ErrInvalidCommand)
		default:
			return reduction{}, fmt.Errorf("unsupported command %T: %w", command, ErrInvalidCommand)
		}
	}
	return r.completeReduction(changed, changed && reportsApplied), nil
}

func (r *reducer) reduceTopologyCommand(
	snapshot *Snapshot,
	command Command,
) (changed bool, reportsApplied bool, handled bool, err error) {
	reportsApplied = true
	switch payload := command.(type) {
	case CreateDesktopCommand:
		changed, err = r.createDesktop(snapshot, payload)
	case UpdateDesktopCommand:
		changed, err = r.updateDesktop(snapshot, payload)
	case ReorderDesktopCommand:
		changed, err = r.reorderDesktop(snapshot, payload)
	case DeleteDesktopCommand:
		changed, err = r.deleteDesktop(snapshot, payload)
	case OpenWindowCommand:
		changed, err = r.openWindow(snapshot, payload)
	case NavigateWindowCommand:
		changed, err = r.navigateWindow(snapshot, payload)
	case CloseWindowCommand:
		changed, err = r.closeWindow(snapshot, payload)
	case sessionDeletionCommand:
		changed, err = r.retireDeletedSessionWindows(snapshot, payload.sessionID)
	case MoveWindowCommand:
		changed, err = r.moveWindow(snapshot, payload)
	case ResizeWindowCommand:
		changed, err = r.resizeWindow(snapshot, payload)
	case SwapWindowsCommand:
		changed, err = r.swapWindows(snapshot, payload)
	case ToggleFloatingCommand:
		changed, err = r.toggleFloating(snapshot, payload)
	case GroupWindowsCommand:
		changed, err = r.groupWindows(snapshot, payload)
	case ReorderStackCommand:
		changed, err = r.reorderStack(snapshot, payload)
	case SetStackActiveCommand:
		changed, err = r.setStackActive(snapshot, payload)
	case PinWindowCommand:
		changed, err = r.pinWindow(snapshot, payload)
	case ReopenCommand:
		changed = r.reopenWindow(snapshot)
		reportsApplied = len(r.changes.windows) > 0
	case ZoomWindowCommand:
		changed, err = r.zoomWindow(snapshot, payload)
	default:
		return false, true, false, nil
	}
	return changed, reportsApplied, true, err
}

func (r *reducer) reduceLayoutCommand(
	snapshot *Snapshot,
	command Command,
) (changed bool, handled bool, err error) {
	switch payload := command.(type) {
	case ArrangeLayoutCommand:
		changed, err = r.arrange(snapshot, payload)
	case ResizeLayoutCommand:
		changed, err = r.resize(snapshot, payload)
	case FrameResizeLayoutCommand:
		changed, err = r.frameResize(snapshot, payload)
	case BalanceLayoutCommand:
		changed, err = r.balance(snapshot, payload)
	case UndoLayoutCommand:
		changed, err = r.undo(snapshot)
	case RedoLayoutCommand:
		changed, err = r.redo(snapshot)
	case ReplaceLayoutCommand:
		changed, err = r.replace(snapshot, payload)
	default:
		return false, false, nil
	}
	return changed, true, err
}

func (r *reducer) completeReduction(changed bool, applied bool) reduction {
	return reduction{changed: changed, applied: applied, changes: r.changes.result()}
}

type changeBuilder struct {
	desktops        map[DesktopID]struct{}
	windows         map[WindowID]struct{}
	groups          map[GroupID]struct{}
	nodes           map[NodeID]struct{}
	groupedStacks   map[NodeID]struct{}
	ungroupedStacks map[NodeID]struct{}
}

func (b *changeBuilder) desktop(id DesktopID) {
	if b.desktops == nil {
		b.desktops = make(map[DesktopID]struct{})
	}
	b.desktops[id] = struct{}{}
}
func (b *changeBuilder) window(id WindowID) {
	if b.windows == nil {
		b.windows = make(map[WindowID]struct{})
	}
	b.windows[id] = struct{}{}
}
func (b *changeBuilder) group(id GroupID) {
	if b.groups == nil {
		b.groups = make(map[GroupID]struct{})
	}
	b.groups[id] = struct{}{}
}
func (b *changeBuilder) node(id NodeID) {
	if b.nodes == nil {
		b.nodes = make(map[NodeID]struct{})
	}
	b.nodes[id] = struct{}{}
}
func (b *changeBuilder) stackGrouped(id NodeID) {
	if b.groupedStacks == nil {
		b.groupedStacks = make(map[NodeID]struct{})
	}
	b.groupedStacks[id] = struct{}{}
}
func (b *changeBuilder) stackUngrouped(id NodeID) {
	if b.ungroupedStacks == nil {
		b.ungroupedStacks = make(map[NodeID]struct{})
	}
	b.ungroupedStacks[id] = struct{}{}
}
func (b *changeBuilder) result() ChangeSet {
	result := ChangeSet{}
	for id := range b.desktops {
		result.DesktopIDs = append(result.DesktopIDs, id)
	}
	for id := range b.windows {
		result.WindowIDs = append(result.WindowIDs, id)
	}
	for id := range b.groups {
		result.GroupIDs = append(result.GroupIDs, id)
	}
	for id := range b.nodes {
		result.NodeIDs = append(result.NodeIDs, id)
	}
	for id := range b.groupedStacks {
		result.StackGrouped = append(result.StackGrouped, id)
	}
	for id := range b.ungroupedStacks {
		result.StackUngrouped = append(result.StackUngrouped, id)
	}
	sortChangeSet(&result)
	return result
}
