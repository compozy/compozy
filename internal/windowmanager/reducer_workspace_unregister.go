package windowmanager

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// filesRoutePrefix roots a Files app window at one project: `/files/<workspace_id>`.
const filesRoutePrefix = "/files/"

func routeBindsWorkspace(route RouteIntent, binding string) bool {
	var value string
	return json.Unmarshal(route.Search["workspace"], &value) == nil && value == binding
}

// routeOpensWorkspaceFiles reports a Files app route whose window exists only for the project.
func routeOpensWorkspaceFiles(route RouteIntent, binding string) bool {
	rest, ok := strings.CutPrefix(route.Pathname, filesRoutePrefix)
	if !ok {
		return false
	}
	segment, _, _ := strings.Cut(rest, "/")
	workspaceID, err := url.PathUnescape(segment)
	return err == nil && workspaceID == binding
}

func routeReferencesWorkspace(route RouteIntent, binding string) bool {
	return routeBindsWorkspace(route, binding) || routeOpensWorkspaceFiles(route, binding)
}

func windowReferencesWorkspace(window Window, binding string) bool {
	return routeReferencesWorkspace(window.Route, binding) ||
		slices.ContainsFunc(
			window.NavStack,
			func(route RouteIntent) bool { return routeReferencesWorkspace(route, binding) },
		)
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
		if routeOpensWorkspaceFiles(window.Route, binding) {
			// A project's Files window has no catalog landing: it leaves with its project.
			if err := r.removeRetiredWindow(snapshot, id); err != nil {
				return false, err
			}
			changed = true
			continue
		}
		if routeBindsWorkspace(window.Route, binding) {
			window.Route = catalogRoute(window)
			window.InstanceKey = nil
		}
		window.NavStack = slices.DeleteFunc(
			window.NavStack,
			func(route RouteIntent) bool { return routeReferencesWorkspace(route, binding) },
		)
		snapshot.Windows[id] = window
		r.changes.window(id)
		changed = true
	}
	for i := range snapshot.ClosedEntries {
		entry := &snapshot.ClosedEntries[i]
		before := len(entry.Windows)
		entry.Windows = slices.DeleteFunc(
			entry.Windows,
			func(window Window) bool { return windowReferencesWorkspace(window, binding) },
		)
		if before != len(entry.Windows) {
			changed = true

		}
		*entry = pruneClosedEntry(*entry)
	}
	snapshot.ClosedEntries = slices.DeleteFunc(
		snapshot.ClosedEntries,
		func(entry ClosedEntry) bool { return len(entry.Windows) == 0 },
	)
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

func (r *reducer) removeRetiredWindow(snapshot *Snapshot, windowID WindowID) error {
	window := snapshot.Windows[windowID]
	if !removeWindow(snapshot, windowID) {
		return fmt.Errorf("window %q has no placement: %w", windowID, ErrInvalidTopology)
	}
	delete(snapshot.Windows, windowID)
	r.changes.window(windowID)
	r.changes.desktop(window.DesktopID)

	return nil
}
