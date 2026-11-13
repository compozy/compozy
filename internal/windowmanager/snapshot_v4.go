package windowmanager

import (
	"encoding/json"
	"fmt"
	"maps"
	"strings"
)

// RetiredApp remains part of the lossless stored-snapshot migration.
func RetiredApp(app string) (string, bool) {
	switch app {
	case "jobs", "triggers":
		return "automations", true
	default:
		return app, false
	}
}

func RewriteRetiredAppRoute(app string, route RouteIntent) RouteIntent {
	if _, retired := RetiredApp(app); !retired {
		return route
	}
	prefix := "/" + app
	if route.Pathname == prefix || route.Pathname == prefix+"/" {
		route.Pathname = "/automations"
		route.Search = maps.Clone(route.Search)
		if route.Search == nil {
			route.Search = RouteSearch{}
		}
		start := `"schedule"`
		if app == "triggers" {
			start = `"event"`
			if event, exists := route.Search["event"]; exists {
				if _, hasQuery := route.Search["q"]; !hasQuery {
					route.Search["q"] = event
				}
				delete(route.Search, "event")
			}
		}
		route.Search["start"] = json.RawMessage(start)
	} else if id, ok := strings.CutPrefix(route.Pathname, prefix+"/"); ok {
		id = strings.TrimSuffix(id, "/")
		if id != "" && !strings.Contains(id, "/") {
			route.Pathname = "/automations" + prefix + "/" + id
		}
	}
	return route
}

func MigrateSnapshotV4(encoded []byte) (SnapshotV5, error) {
	var snapshot SnapshotV5
	if err := decodeSnapshotJSON(encoded, &snapshot); err != nil {
		return SnapshotV5{}, fmt.Errorf("decode v4 snapshot: %w", err)
	}
	if snapshot.Version != 4 {
		return SnapshotV5{}, fmt.Errorf("snapshot version %d is not 4", snapshot.Version)
	}
	snapshot.Version = 5
	return snapshot, nil
}

func rewriteRetiredWindow(window Window) Window {
	if replacement, retired := RetiredApp(window.App); retired {
		window.Route = RewriteRetiredAppRoute(window.App, window.Route)
		for i, route := range window.NavStack {
			window.NavStack[i] = RewriteRetiredAppRoute(window.App, route)
		}
		window.App = replacement
	}
	return window
}

func rewriteSnapshotRetiredApps(snapshot *Snapshot) {
	rewriteRetiredWindows(snapshot.Windows)
	for i := range snapshot.ClosedEntries {
		for j, window := range snapshot.ClosedEntries[i].Windows {
			snapshot.ClosedEntries[i].Windows[j] = rewriteRetiredWindow(window)
		}
	}
	for _, entries := range [][]HistoryEntry{snapshot.History.Undo, snapshot.History.Redo} {
		for i := range entries {
			rewriteRetiredWindows(entries[i].Before.Windows)
			rewriteRetiredWindows(entries[i].After.Windows)
		}
	}
}

func rewriteRetiredWindows(windows map[WindowID]Window) {
	for id, window := range windows {
		windows[id] = rewriteRetiredWindow(window)
	}
}
