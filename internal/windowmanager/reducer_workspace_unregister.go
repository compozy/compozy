package windowmanager

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

func routeBindsWorkspace(route RouteIntent, binding string) bool {
	var value string
	return json.Unmarshal(route.Search["workspace"], &value) == nil && value == binding
}

func windowReferencesWorkspace(window Window, binding string) bool {
	return routeBindsWorkspace(window.Route, binding) || slices.ContainsFunc(window.NavStack, func(route RouteIntent) bool { return routeBindsWorkspace(route, binding) })
}

func catalogRoute(window Window) RouteIntent {
	path := "/" + strings.Split(strings.TrimPrefix(window.Route.Pathname, "/"), "/")[0]
	if window.App == "session" {
		path = sessionEmptyRoutePath
	}
	return RouteIntent{Pathname: path, Search: RouteSearch{}}
}

func (r *reducer) retireUnregisteredWorkspaceWindows(snapshot *Snapshot, binding string) (bool, error) {
	if binding == "" {
		return false, fmt.Errorf("workspace binding is required: %w", ErrInvalidCommand)
	}
	changed := false
	for id, window := range snapshot.Windows {
		if !windowReferencesWorkspace(window, binding) {
			continue
		}
		if routeBindsWorkspace(window.Route, binding) {
			window.Route = catalogRoute(window)
			window.InstanceKey = nil
		}
		window.NavStack = slices.DeleteFunc(window.NavStack, func(route RouteIntent) bool { return routeBindsWorkspace(route, binding) })
		snapshot.Windows[id] = window
		r.changes.window(id)
		changed = true
	}
	for i := range snapshot.ClosedEntries {
		entry := &snapshot.ClosedEntries[i]
		before := len(entry.Windows)
		entry.Windows = slices.DeleteFunc(entry.Windows, func(window Window) bool { return windowReferencesWorkspace(window, binding) })
		if before != len(entry.Windows) {
			changed = true
			if len(entry.Windows) > 0 && entry.ActiveID != nil && !slices.ContainsFunc(entry.Windows, func(window Window) bool { return window.ID == *entry.ActiveID }) {
				entry.ActiveID = new(entry.Windows[0].ID)
			}
		}
	}
	snapshot.ClosedEntries = slices.DeleteFunc(snapshot.ClosedEntries, func(entry ClosedEntry) bool { return len(entry.Windows) == 0 })
	for _, entries := range [][]HistoryEntry{snapshot.History.Undo, snapshot.History.Redo} {
		for _, entry := range entries {
			for _, state := range []State{entry.Before, entry.After} {
				for _, window := range state.Windows {
					if windowReferencesWorkspace(window, binding) {
						changed = true
					}
				}
			}
		}
	}
	if changed {
		snapshot.History.Undo = []HistoryEntry{}
		snapshot.History.Redo = []HistoryEntry{}
	}
	return changed, nil
}
