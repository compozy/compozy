package contract

import (
	"fmt"

	"github.com/compozy/compozy/internal/windowmanager"
)

// WindowManagerSnapshotFromDomain converts the authoritative domain snapshot to its stable wire form.
func WindowManagerSnapshotFromDomain(snapshot windowmanager.Snapshot) (WindowManagerSnapshot, error) {
	revision, err := windowManagerRevision(snapshot.Revision)
	if err != nil {
		return WindowManagerSnapshot{}, err
	}
	windows, err := windowManagerWindowsFromDomain(snapshot.Windows)
	if err != nil {
		return WindowManagerSnapshot{}, err
	}
	return WindowManagerSnapshot{
		Version: snapshot.Version, WorkspaceID: snapshot.WorkspaceID, Revision: revision,
		Desktops: windowManagerDesktopsFromDomain(snapshot.Desktops), Windows: windows,
		ClosedEntryCount: len(snapshot.ClosedEntries),
		Overrides:        cloneWindowManagerWorkspaceConfig(snapshot.Overrides), UpdatedAt: snapshot.UpdatedAt,
	}, nil
}

// WindowManagerLayoutFromDomain converts a domain layout to its stable wire form.
func WindowManagerLayoutFromDomain(document windowmanager.LayoutDocument) (WindowManagerLayoutDocument, error) {
	windows, err := windowManagerWindowsFromDomain(document.Windows)
	if err != nil {
		return WindowManagerLayoutDocument{}, err
	}
	return WindowManagerLayoutDocument{
		Version: document.Version, WorkspaceID: document.WorkspaceID,
		Desktops: windowManagerDesktopsFromDomain(document.Desktops),
		Windows:  windows, Overrides: cloneWindowManagerWorkspaceConfig(document.Overrides),
	}, nil
}

// Domain converts a stable wire layout to the authoritative domain document.
func (document WindowManagerLayoutDocument) Domain() windowmanager.LayoutDocument {
	return windowmanager.LayoutDocument{
		Version: document.Version, WorkspaceID: document.WorkspaceID,
		Desktops:  windowManagerDesktopsToDomain(document.Desktops),
		Windows:   windowManagerWindowsToDomain(document.Windows),
		Overrides: cloneWindowManagerWorkspaceConfig(document.Overrides),
	}
}

// WindowManagerClientFromDomain converts transient client presentation state.
func WindowManagerClientFromDomain(client windowmanager.ClientView) (WindowManagerClientView, error) {
	revision, err := windowManagerRevision(windowmanager.Revision(client.PresentationRevision))
	if err != nil {
		return WindowManagerClientView{}, err
	}
	contextRevision, err := windowManagerRevision(windowmanager.Revision(client.ContextRevision))
	if err != nil {
		return WindowManagerClientView{}, err
	}
	return WindowManagerClientView{
		WorkspaceID: client.WorkspaceID, ClientID: client.ClientID, Kind: client.Kind,
		ActiveDesktopID: client.ActiveDesktopID, PresentationRevision: revision,
		ContextRevision: contextRevision,
		FocusedWindowID: cloneWindowManagerPointer(client.FocusedWindowID),
		FocusOrder:      append([]windowmanager.WindowID{}, client.FocusOrder...),
		StackActive:     windowManagerStackActiveFromDomain(client.StackActive),
		PaletteContext: WindowManagerPaletteContext{
			WindowFocused:       client.PaletteContext.WindowFocused,
			WindowFloating:      client.PaletteContext.WindowFloating,
			WindowStacked:       client.PaletteContext.WindowStacked,
			DesktopWindowCount:  client.PaletteContext.DesktopWindowCount,
			ScopeGlobal:         client.PaletteContext.ScopeGlobal,
			ShellDesktop:        client.PaletteContext.ShellDesktop,
			FocusedSessionState: client.PaletteContext.FocusedSessionState,
			WorkspaceTrusted:    client.PaletteContext.WorkspaceTrusted,
			DestinationIntent:   cloneWindowManagerRoutePointer(client.PaletteContext.DestinationIntent),
		},
		GlobalShortcuts: append(
			[]windowmanager.GlobalShortcutRegistration{},
			client.GlobalShortcuts...,
		),
		ConnectedAt:     client.ConnectedAt,
		AttachmentToken: client.AttachmentToken,
	}, nil
}

func cloneWindowManagerRoutePointer(route *windowmanager.RouteIntent) *windowmanager.RouteIntent {
	if route == nil {
		return nil
	}
	cloned := cloneWindowManagerRoute(*route)
	return &cloned
}

func windowManagerStackActiveFromDomain(values map[windowmanager.NodeID]windowmanager.WindowID) map[string]string {
	result := make(map[string]string, len(values))
	for stackID, windowID := range values {
		result[string(stackID)] = string(windowID)
	}
	return result
}

// WindowManagerResultFromDomain converts a semantic command result.
func WindowManagerResultFromDomain(result *windowmanager.Result) (WindowManagerResult, error) {
	snapshot, err := WindowManagerSnapshotFromDomain(result.Snapshot)
	if err != nil {
		return WindowManagerResult{}, err
	}
	rebased, err := optionalWindowManagerRevision(result.RebasedFrom)
	if err != nil {
		return WindowManagerResult{}, err
	}
	var client *WindowManagerClientView
	if result.Client != nil {
		converted, err := WindowManagerClientFromDomain(*result.Client)
		if err != nil {
			return WindowManagerResult{}, fmt.Errorf("convert result client: %w", err)
		}
		client = &converted
	}
	return WindowManagerResult{
		Snapshot: snapshot, Applied: result.Applied, Changes: result.Changes,
		Diagnostics: result.Diagnostics, Client: client, RebasedFrom: rebased,
	}, nil
}

// WindowManagerPreviewFromDomain converts a command preview.
func WindowManagerPreviewFromDomain(preview windowmanager.Preview) (WindowManagerPreview, error) {
	snapshot, err := WindowManagerSnapshotFromDomain(preview.Snapshot)
	if err != nil {
		return WindowManagerPreview{}, err
	}
	var client *WindowManagerClientView
	if preview.Client != nil {
		converted, err := WindowManagerClientFromDomain(*preview.Client)
		if err != nil {
			return WindowManagerPreview{}, fmt.Errorf("convert preview client: %w", err)
		}
		client = &converted
	}
	return WindowManagerPreview{
		Snapshot: snapshot, Changed: preview.Changed, Changes: preview.Changes,
		Diagnostics: preview.Diagnostics, Client: client,
	}, nil
}

func windowManagerWindowsFromDomain(
	windows map[windowmanager.WindowID]windowmanager.Window,
) (map[string]WindowManagerWindow, error) {
	result := make(map[string]WindowManagerWindow, len(windows))
	for id, window := range windows {
		converted := WindowManagerWindow{
			ID: window.ID, App: window.App,
			InstanceKey: cloneWindowManagerPointer(window.InstanceKey),
			Route:       cloneWindowManagerRoute(window.Route), NavStack: cloneWindowManagerRoutes(window.NavStack),
			Pinned: window.Pinned, Placement: window.Placement,
			DesktopID: window.DesktopID, FloatingRect: window.FloatingRect, Minimized: window.Minimized,
			Zoomed: window.Zoomed,
		}
		if window.ReturnAnchor != nil {
			revision, err := windowManagerRevision(window.ReturnAnchor.SourceRevision)
			if err != nil {
				return nil, fmt.Errorf("convert return anchor for window %q: %w", id, err)
			}
			converted.ReturnAnchor = &WindowManagerReturnAnchor{
				DesktopID:      window.ReturnAnchor.DesktopID,
				GroupID:        cloneWindowManagerPointer(window.ReturnAnchor.GroupID),
				ParentSplitID:  cloneWindowManagerPointer(window.ReturnAnchor.ParentSplitID),
				ChildIndex:     cloneWindowManagerPointer(window.ReturnAnchor.ChildIndex),
				Weight:         cloneWindowManagerPointer(window.ReturnAnchor.Weight),
				NeighborIDs:    append([]windowmanager.WindowID(nil), window.ReturnAnchor.NeighborIDs...),
				SourceRevision: revision,
				SourceGroup:    optionalWindowManagerLayoutGroupFromDomain(window.ReturnAnchor.SourceGroup),
				Zoomed:         window.ReturnAnchor.Zoomed,
			}
		}
		result[string(id)] = converted
	}
	return result, nil
}

func windowManagerWindowsToDomain(
	windows map[string]WindowManagerWindow,
) map[windowmanager.WindowID]windowmanager.Window {
	result := make(map[windowmanager.WindowID]windowmanager.Window, len(windows))
	for key, window := range windows {
		converted := windowmanager.Window{
			ID: window.ID, App: window.App,
			InstanceKey: cloneWindowManagerPointer(window.InstanceKey),
			Route:       cloneWindowManagerRoute(window.Route), NavStack: cloneWindowManagerRoutes(window.NavStack),
			Pinned: window.Pinned, Placement: window.Placement,
			DesktopID: window.DesktopID, FloatingRect: window.FloatingRect, Minimized: window.Minimized,
			Zoomed: window.Zoomed,
		}
		if window.ReturnAnchor != nil {
			converted.ReturnAnchor = &windowmanager.ReturnAnchor{
				DesktopID:      window.ReturnAnchor.DesktopID,
				GroupID:        cloneWindowManagerPointer(window.ReturnAnchor.GroupID),
				ParentSplitID:  cloneWindowManagerPointer(window.ReturnAnchor.ParentSplitID),
				ChildIndex:     cloneWindowManagerPointer(window.ReturnAnchor.ChildIndex),
				Weight:         cloneWindowManagerPointer(window.ReturnAnchor.Weight),
				NeighborIDs:    append([]windowmanager.WindowID(nil), window.ReturnAnchor.NeighborIDs...),
				SourceRevision: windowmanager.Revision(window.ReturnAnchor.SourceRevision),
				SourceGroup:    optionalWindowManagerLayoutGroupToDomain(window.ReturnAnchor.SourceGroup),
				Zoomed:         window.ReturnAnchor.Zoomed,
			}
		}
		result[windowmanager.WindowID(key)] = converted
	}
	return result
}

func cloneWindowManagerWorkspaceConfig(config windowmanager.WorkspaceConfig) windowmanager.WorkspaceConfig {
	cloned := config
	cloned.NewWindowPolicy = cloneWindowManagerPointer(config.NewWindowPolicy)
	cloned.SmallViewportPolicy = cloneWindowManagerPointer(config.SmallViewportPolicy)
	cloned.FocusPolicy = cloneWindowManagerPointer(config.FocusPolicy)
	cloned.FocusWrap = cloneWindowManagerPointer(config.FocusWrap)
	cloned.FocusFollowsPointer = cloneWindowManagerPointer(config.FocusFollowsPointer)
	cloned.RaiseOnFocus = cloneWindowManagerPointer(config.RaiseOnFocus)
	cloned.DragAwayPolicy = cloneWindowManagerPointer(config.DragAwayPolicy)
	cloned.GroupMoveModifier = cloneWindowManagerPointer(config.GroupMoveModifier)
	cloned.SwapModifier = cloneWindowManagerPointer(config.SwapModifier)
	cloned.HistoryLimit = cloneWindowManagerPointer(config.HistoryLimit)
	cloned.NavStackLimit = cloneWindowManagerPointer(config.NavStackLimit)
	cloned.ClosedEntryLimit = cloneWindowManagerPointer(config.ClosedEntryLimit)
	cloned.DesktopTransition = cloneWindowManagerPointer(config.DesktopTransition)
	cloned.Gaps = cloneWindowManagerPointer(config.Gaps)
	cloned.Bindings = cloneWindowManagerPointer(config.Bindings)
	cloned.Shortcuts = windowmanager.CloneShortcutMap(config.Shortcuts)
	if config.Snap != nil {
		cloned.Snap = cloneWindowManagerPointer(config.Snap)
		cloned.Snap.RepeatRatios = append([]float64(nil), config.Snap.RepeatRatios...)
	}
	return cloned
}

func cloneWindowManagerRoutes(routes []windowmanager.RouteIntent) []windowmanager.RouteIntent {
	result := make([]windowmanager.RouteIntent, len(routes))
	for index, route := range routes {
		result[index] = cloneWindowManagerRoute(route)
	}
	return result
}

func cloneWindowManagerRoute(route windowmanager.RouteIntent) windowmanager.RouteIntent {
	if route.Search == nil {
		return windowmanager.RouteIntent{Pathname: route.Pathname}
	}
	search := make(windowmanager.RouteSearch, len(route.Search))
	for key, value := range route.Search {
		search[key] = append([]byte(nil), value...)
	}
	return windowmanager.RouteIntent{Pathname: route.Pathname, Search: search}
}

func windowManagerRevision(revision windowmanager.Revision) (WindowManagerRevision, error) {
	if revision > windowmanager.Revision(WindowManagerMaxSafeRevision) {
		return 0, ErrWindowManagerUnsafeRevision
	}
	return WindowManagerRevision(revision), nil
}

func optionalWindowManagerRevision(revision *windowmanager.Revision) (*WindowManagerRevision, error) {
	if revision == nil {
		return nil, nil
	}
	converted, err := windowManagerRevision(*revision)
	if err != nil {
		return nil, err
	}
	return &converted, nil
}
