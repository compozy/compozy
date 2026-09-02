package windowmanager

import "maps"

func cloneSnapshot(snapshot Snapshot) Snapshot {
	snapshot.Desktops = cloneDesktops(snapshot.Desktops)
	snapshot.Windows = cloneWindows(snapshot.Windows)
	snapshot.ClosedEntries = cloneClosedEntries(snapshot.ClosedEntries)
	snapshot.History = cloneHistory(snapshot.History)
	snapshot.Overrides = cloneWorkspaceConfig(snapshot.Overrides)
	return snapshot
}

func cloneDesktops(desktops []Desktop) []Desktop {
	cloned := make([]Desktop, len(desktops))
	for index, desktop := range desktops {
		cloned[index] = cloneDesktop(desktop)
	}
	return cloned
}

func cloneDesktop(desktop Desktop) Desktop {
	sourceGroups := desktop.Groups
	desktop.Groups = make([]LayoutGroup, len(sourceGroups))
	for index, group := range sourceGroups {
		desktop.Groups[index] = cloneLayoutGroup(group)
	}
	desktop.Floating = append([]WindowID(nil), desktop.Floating...)
	desktop.FloatingStacks = cloneFloatingStacks(desktop.FloatingStacks)
	return desktop
}

func cloneLayoutGroup(group LayoutGroup) LayoutGroup {
	group.Root = cloneNode(group.Root)
	return group
}

func cloneNode(node LayoutNode) LayoutNode {
	node.WindowID = clonePointer(node.WindowID)
	node.Axis = clonePointer(node.Axis)
	sourceChildren := node.Children
	node.Children = make([]LayoutNode, len(sourceChildren))
	for index, child := range sourceChildren {
		node.Children[index] = cloneNode(child)
	}
	node.Weights = append([]float64(nil), node.Weights...)
	node.WindowIDs = append([]WindowID(nil), node.WindowIDs...)
	node.ActiveID = clonePointer(node.ActiveID)
	return node
}

func cloneWindows(windows map[WindowID]Window) map[WindowID]Window {
	cloned := make(map[WindowID]Window, len(windows))
	for id, window := range windows {
		cloned[id] = cloneWindow(window)
	}
	return cloned
}

func cloneWindow(window Window) Window {
	window.InstanceKey = clonePointer(window.InstanceKey)
	window.Route = cloneRouteIntent(window.Route)
	window.NavStack = cloneRouteIntents(window.NavStack)
	window.ReturnAnchor = cloneReturnAnchor(window.ReturnAnchor)
	return window
}

func cloneRouteIntents(routes []RouteIntent) []RouteIntent {
	cloned := make([]RouteIntent, len(routes))
	for index, route := range routes {
		cloned[index] = cloneRouteIntent(route)
	}
	return cloned
}

func cloneFloatingStack(stack FloatingStack) FloatingStack {
	stack.WindowIDs = append([]WindowID(nil), stack.WindowIDs...)
	stack.ActiveID = clonePointer(stack.ActiveID)
	return stack
}

func cloneFloatingStacks(stacks []FloatingStack) []FloatingStack {
	cloned := make([]FloatingStack, len(stacks))
	for index, stack := range stacks {
		cloned[index] = cloneFloatingStack(stack)
	}
	return cloned
}

func cloneClosedEntries(entries []ClosedEntry) []ClosedEntry {
	cloned := make([]ClosedEntry, len(entries))
	for index, entry := range entries {
		entry.Windows = make([]Window, len(entries[index].Windows))
		for windowIndex, window := range entries[index].Windows {
			entry.Windows[windowIndex] = cloneWindow(window)
		}
		entry.ActiveID = clonePointer(entry.ActiveID)
		entry.StackID = clonePointer(entry.StackID)
		cloned[index] = entry
	}
	return cloned
}

func cloneRouteIntent(route RouteIntent) RouteIntent {
	route.Search = cloneRouteSearch(route.Search)
	return route
}

func cloneRouteSearch(search RouteSearch) RouteSearch {
	if search == nil {
		return nil
	}
	cloned := make(RouteSearch, len(search))
	for key, value := range search {
		cloned[key] = append([]byte(nil), value...)
	}
	return cloned
}

func cloneReturnAnchor(anchor *ReturnAnchor) *ReturnAnchor {
	if anchor == nil {
		return nil
	}
	cloned := *anchor
	cloned.GroupID = clonePointer(anchor.GroupID)
	cloned.ParentSplitID = clonePointer(anchor.ParentSplitID)
	cloned.ChildIndex = clonePointer(anchor.ChildIndex)
	cloned.Weight = clonePointer(anchor.Weight)
	cloned.NeighborIDs = append([]WindowID(nil), anchor.NeighborIDs...)
	if anchor.SourceGroup != nil {
		sourceGroup := cloneLayoutGroup(*anchor.SourceGroup)
		cloned.SourceGroup = &sourceGroup
	}
	if anchor.SourceStack != nil {
		sourceStack := cloneFloatingStack(*anchor.SourceStack)
		cloned.SourceStack = &sourceStack
	}
	return &cloned
}

func cloneClientView(view ClientView) ClientView {
	view.FocusedWindowID = clonePointer(view.FocusedWindowID)
	view.FocusOrder = append([]WindowID(nil), view.FocusOrder...)
	view.StackActive = cloneStackActive(view.StackActive)
	view.GlobalShortcuts = CloneGlobalShortcutRegistrations(view.GlobalShortcuts)
	if view.PaletteContext.DestinationIntent != nil {
		destination := cloneRouteIntent(*view.PaletteContext.DestinationIntent)
		view.PaletteContext.DestinationIntent = &destination
	}
	return view
}

func cloneStackActive(values map[NodeID]WindowID) map[NodeID]WindowID {
	if values == nil {
		return nil
	}
	cloned := make(map[NodeID]WindowID, len(values))
	maps.Copy(cloned, values)
	return cloned
}

func cloneHistory(history History) History {
	history.Undo = cloneHistoryEntries(history.Undo)
	history.Redo = cloneHistoryEntries(history.Redo)
	return history
}

func cloneHistoryEntries(entries []HistoryEntry) []HistoryEntry {
	cloned := make([]HistoryEntry, len(entries))
	for index, entry := range entries {
		cloned[index] = cloneHistoryEntry(entry)
	}
	return cloned
}

func cloneHistoryEntry(entry HistoryEntry) HistoryEntry {
	entry.Before = cloneState(entry.Before)
	entry.After = cloneState(entry.After)
	return entry
}

func cloneState(state State) State {
	state.Desktops = cloneDesktops(state.Desktops)
	state.Windows = cloneWindows(state.Windows)
	state.Overrides = cloneWorkspaceConfig(state.Overrides)
	return state
}

func cloneWorkspaceConfig(config WorkspaceConfig) WorkspaceConfig {
	config.NewWindowPolicy = clonePointer(config.NewWindowPolicy)
	config.SmallViewportPolicy = clonePointer(config.SmallViewportPolicy)
	config.FocusPolicy = clonePointer(config.FocusPolicy)
	config.FocusWrap = clonePointer(config.FocusWrap)
	config.FocusFollowsPointer = clonePointer(config.FocusFollowsPointer)
	config.RaiseOnFocus = clonePointer(config.RaiseOnFocus)
	config.DragAwayPolicy = clonePointer(config.DragAwayPolicy)
	config.GroupMoveModifier = clonePointer(config.GroupMoveModifier)
	config.SwapModifier = clonePointer(config.SwapModifier)
	config.HistoryLimit = clonePointer(config.HistoryLimit)
	config.NavStackLimit = clonePointer(config.NavStackLimit)
	config.ClosedEntryLimit = clonePointer(config.ClosedEntryLimit)
	config.DesktopTransition = clonePointer(config.DesktopTransition)
	config.Gaps = clonePointer(config.Gaps)
	if config.Snap != nil {
		cloned := cloneSnapConfig(*config.Snap)
		config.Snap = &cloned
	}
	config.Bindings = clonePointer(config.Bindings)
	config.Shortcuts = CloneShortcutMap(config.Shortcuts)
	return config
}

func cloneConfig(config Config) Config {
	config.Snap = cloneSnapConfig(config.Snap)
	config.Shortcuts = CloneShortcutMap(config.Shortcuts)
	return config
}

func cloneSnapConfig(config SnapConfig) SnapConfig {
	config.RepeatRatios = append([]float64(nil), config.RepeatRatios...)
	return config
}

func cloneChangeSet(changes ChangeSet) ChangeSet {
	changes.DesktopIDs = append([]DesktopID(nil), changes.DesktopIDs...)
	changes.WindowIDs = append([]WindowID(nil), changes.WindowIDs...)
	changes.GroupIDs = append([]GroupID(nil), changes.GroupIDs...)
	changes.NodeIDs = append([]NodeID(nil), changes.NodeIDs...)
	changes.ClientIDs = append([]ClientID(nil), changes.ClientIDs...)
	changes.StackGrouped = append([]NodeID(nil), changes.StackGrouped...)
	changes.StackUngrouped = append([]NodeID(nil), changes.StackUngrouped...)
	return changes
}

func cloneEvent(event Event) Event {
	event.Changes = cloneChangeSet(event.Changes)
	return event
}

func cloneSubscriptionFence(fence SubscriptionFence) SubscriptionFence {
	fence.Snapshot = cloneSnapshot(fence.Snapshot)
	if fence.Client != nil {
		client := cloneClientView(*fence.Client)
		fence.Client = &client
	}
	return fence
}

func cloneSubscriptionUpdate(update SubscriptionUpdate) SubscriptionUpdate {
	if update.Event != nil {
		event := cloneEvent(*update.Event)
		update.Event = &event
	}
	if update.Client != nil {
		client := cloneClientView(*update.Client)
		update.Client = &client
	}
	return update
}

func cloneLayoutDocument(document LayoutDocument) LayoutDocument {
	document.Desktops = cloneDesktops(document.Desktops)
	document.Windows = cloneWindows(document.Windows)
	document.Overrides = cloneWorkspaceConfig(document.Overrides)
	return document
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
