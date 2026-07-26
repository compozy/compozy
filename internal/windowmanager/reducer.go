package windowmanager

import "fmt"

type reducer struct {
	generate      idGenerator
	config        Config
	focusedWindow *WindowID
	changes       changeBuilder
}

type reduction struct {
	changed bool
	changes ChangeSet
}

func (r *reducer) reduce(snapshot *Snapshot, command Command) (reduction, error) {
	var changed bool
	var err error
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
	case MoveWindowCommand:
		changed, err = r.moveWindow(snapshot, payload)
	case SwapWindowsCommand:
		changed, err = r.swapWindows(snapshot, payload)
	case ToggleFloatingCommand:
		changed, err = r.toggleFloating(snapshot, payload)
	case ZoomWindowCommand:
		changed, err = r.zoomWindow(snapshot, payload)
	case ArrangeLayoutCommand:
		changed, err = r.arrange(snapshot, payload)
	case ResizeLayoutCommand:
		changed, err = r.resize(snapshot, payload)
	case BalanceLayoutCommand:
		changed, err = r.balance(snapshot, payload)
	case UndoLayoutCommand:
		changed, err = r.undo(snapshot)
	case RedoLayoutCommand:
		changed, err = r.redo(snapshot)
	case ReplaceLayoutCommand:
		changed, err = r.replace(snapshot, payload)
	case SwitchDesktopCommand, FocusWindowCommand:
		return reduction{}, fmt.Errorf("presentation command reached durable reducer: %w", ErrInvalidCommand)
	default:
		return reduction{}, fmt.Errorf("unsupported command %T: %w", command, ErrInvalidCommand)
	}
	if err != nil {
		return reduction{}, err
	}
	return reduction{changed: changed, changes: r.changes.result()}, nil
}

type changeBuilder struct {
	desktops map[DesktopID]struct{}
	windows  map[WindowID]struct{}
	groups   map[GroupID]struct{}
	nodes    map[NodeID]struct{}
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
	sortChangeSet(&result)
	return result
}
