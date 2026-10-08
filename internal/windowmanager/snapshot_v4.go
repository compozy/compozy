package windowmanager

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if route.Pathname == prefix {
		route.Pathname = "/automations"
		route.Search = maps.Clone(route.Search)
		if route.Search == nil {
			route.Search = RouteSearch{}
		}
		start := `"schedule"`
		if app == "triggers" {
			start = `"event"`
			if event, exists := route.Search["event"]; exists {
				route.Search["q"] = event
				delete(route.Search, "event")
			}
		}
		route.Search["start"] = json.RawMessage(start)
	} else if strings.HasPrefix(route.Pathname, prefix+"/") {
		route.Pathname = "/automations" + route.Pathname
	}
	return route
}

func MigrateSnapshotV4(encoded []byte) (Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode v4 window-manager snapshot: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("decode v4 window-manager snapshot: trailing data")
	}
	if snapshot.Version != PreviousSnapshotVersion {
		return Snapshot{}, fmt.Errorf(
			"window-manager snapshot version %d is not %d", snapshot.Version, PreviousSnapshotVersion,
		)
	}
	revision, err := NextTopologyRevision(snapshot.Revision)
	if err != nil {
		return Snapshot{}, fmt.Errorf("v4 window-manager snapshot revision: %w", err)
	}
	snapshot.Version = SnapshotVersion
	snapshot.Revision = revision
	rewriteSnapshotRetiredApps(&snapshot)
	if err := ValidateSnapshot(snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("migrated window-manager snapshot: %w", err)
	}
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
