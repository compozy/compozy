package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/windowmanager"
)

// WindowManagerSnapshotFromDomain converts the authoritative domain snapshot to its stable wire form.
func WindowManagerSnapshotFromDomain(snapshot windowmanager.Snapshot) (WindowManagerSnapshot, error) {
	revision, err := windowManagerRevision(snapshot.Revision)
	if err != nil {
		return WindowManagerSnapshot{}, err
	}
	windows := windowManagerWindowsFromDomain(snapshot.Windows)
	return WindowManagerSnapshot{
		Version: snapshot.Version, WorkspaceID: snapshot.WorkspaceID, Revision: revision,
		Desktops: windowManagerDesktopsFromDomain(snapshot.Desktops), Windows: windows,
		ClosedEntryCount: len(snapshot.ClosedEntries), Migration: snapshot.Migration,
		Overrides: cloneWindowManagerWorkspaceConfig(snapshot.Overrides), UpdatedAt: snapshot.UpdatedAt,
	}, nil
}

// WindowManagerLayoutFromDomain converts a domain layout to its stable wire form.
func WindowManagerLayoutFromDomain(document windowmanager.LayoutDocument) (WindowManagerLayoutDocument, error) {
	windows := windowManagerWindowsFromDomain(document.Windows)
	return WindowManagerLayoutDocument{
		InputDeprecations: slices.Clone(document.InputDeprecations),
		Version:           document.Version, WorkspaceID: document.WorkspaceID,
		Desktops: windowManagerDesktopsFromDomain(document.Desktops),
		Windows:  windows, Overrides: cloneWindowManagerWorkspaceConfig(document.Overrides),
	}, nil
}

// Domain converts a stable wire layout to the authoritative domain document.
func (document WindowManagerLayoutDocument) Domain() windowmanager.LayoutDocument {
	return windowmanager.LayoutDocument{
		InputDeprecations: slices.Clone(document.InputDeprecations),
		Version:           document.Version, WorkspaceID: document.WorkspaceID,
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
		StackActive:     map[string]string{}, ActiveTab: windowManagerActiveTabFromDomain(client.ActiveTab),
		PaletteContext: WindowManagerPaletteContext{
			WindowFocused:  client.PaletteContext.WindowFocused,
			WindowFloating: client.PaletteContext.WindowFloating,
			WindowStacked:  client.PaletteContext.WindowStacked, WindowInSplit: client.PaletteContext.WindowInSplit,
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
	return new(cloneWindowManagerRoute(*route))
}

func windowManagerActiveTabFromDomain(values map[windowmanager.DesktopID]windowmanager.TabID) map[string]string {
	result := map[string]string{}
	for d, t := range values {
		result[string(d)] = string(t)
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
		Snapshot: snapshot,
		Applied:  result.Applied,
		Changes:  result.Changes,
		Deprecations: append(
			[]windowmanager.Deprecation{},
			result.Deprecations...),
		Diagnostics: result.Diagnostics,
		Client:      client,
		RebasedFrom: rebased,
	}, nil
}

// WindowManagerPreviewFromDomain converts a command preview.
func WindowManagerPreviewFromDomain(preview *windowmanager.Preview) (WindowManagerPreview, error) {
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
		Snapshot: snapshot,
		Changed:  preview.Changed,
		Changes:  preview.Changes,
		Deprecations: append(
			[]windowmanager.Deprecation{},
			preview.Deprecations...),
		Diagnostics: preview.Diagnostics,
		Client:      client,
	}, nil
}

func windowManagerWindowsFromDomain(
	windows map[windowmanager.WindowID]windowmanager.Window,
) map[string]WindowManagerWindow {
	result := make(map[string]WindowManagerWindow, len(windows))
	for id, w := range windows {
		result[string(id)] = WindowManagerWindow{
			ID:          w.ID,
			App:         w.App,
			InstanceKey: cloneWindowManagerPointer(w.InstanceKey),
			Route:       cloneWindowManagerRoute(w.Route),
			NavStack:    cloneWindowManagerRoutes(w.NavStack),
			DesktopID:   w.DesktopID,
			TabID:       w.TabID,
		}
	}
	return result
}

func windowManagerWindowsToDomain(
	windows map[string]WindowManagerWindow,
) map[windowmanager.WindowID]windowmanager.Window {
	result := make(map[windowmanager.WindowID]windowmanager.Window, len(windows))
	for id, w := range windows {
		result[windowmanager.WindowID(id)] = windowmanager.Window{
			ID:          w.ID,
			App:         w.App,
			InstanceKey: cloneWindowManagerPointer(w.InstanceKey),
			Route:       cloneWindowManagerRoute(w.Route),
			NavStack:    cloneWindowManagerRoutes(w.NavStack),
			DesktopID:   w.DesktopID,
			TabID:       w.TabID,
		}
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

func (document *WindowManagerLayoutDocument) UnmarshalJSON(raw []byte) error {
	var domain windowmanager.LayoutDocument
	if err := json.Unmarshal(raw, &domain); err != nil {
		return err
	}
	converted, err := WindowManagerLayoutFromDomain(domain)
	if err != nil {
		return err
	}
	*document = converted
	if len(domain.InputDeprecations) > 0 {
		var input struct {
			Overrides json.RawMessage `json:"overrides"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return err
		}
		document.inputOverrides = input.Overrides
	}
	return nil
}

// MarshalJSON keeps retired input overrides until the receiving daemon can observe their use.
// Documents exported from the domain have no input bytes and always encode the effective config.
func (document WindowManagerLayoutDocument) MarshalJSON() ([]byte, error) {
	type wire WindowManagerLayoutDocument
	if len(document.inputOverrides) == 0 {
		return json.Marshal(wire(document))
	}
	input, _, err := windowmanager.DecodeWorkspaceConfigJSON(document.inputOverrides)
	if err != nil {
		return nil, err
	}
	normalized, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	current, err := json.Marshal(document.Overrides)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(normalized, current) {
		return json.Marshal(wire(document))
	}
	return json.Marshal(struct {
		wire
		Overrides json.RawMessage `json:"overrides"`
	}{wire: wire(document), Overrides: document.inputOverrides})
}
