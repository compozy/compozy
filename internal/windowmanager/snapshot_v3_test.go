package windowmanager

// Suite: legacy aggregate migration
// Invariant: a stored version 3 aggregate loads as a valid version 4 aggregate whose former focus
// desktops are regular desktops hosting their owner as a lifted zoom that unzoom takes home.
// Boundary IN: raw stored JSON.
// Boundary OUT: repository storage and transport decoding.

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func legacyFocusSnapshotJSON(t *testing.T, includeAnchor bool) []byte {
	t.Helper()
	anchor := ""
	if includeAnchor {
		anchor = `,"return_anchor":{"desktop_id":"desktop-default","group_id":"group-main","parent_split_id":"split-main",
			"child_index":0,"weight":0.5,"neighbor_ids":["settings"],"source_revision":3,
			"source_group":{"id":"group-main","frame":{"x":0,"y":0,"width":1,"height":1},
				"root":{"id":"split-main","kind":"split","axis":"horizontal","weights":[0.5,0.5],
					"children":[{"id":"leaf-tasks","kind":"leaf","window_id":"tasks"},
						{"id":"leaf-settings","kind":"leaf","window_id":"settings"}]}}}`
	}
	document := `{
		"version":3,"workspace_id":"workspace-a","revision":4,
		"desktops":[
			{"id":"desktop-default","name":"Desktop 1","order":0,"purpose":"standard",
				"groups":[{"id":"group-main","frame":{"x":0,"y":0,"width":1,"height":1},
					"root":{"id":"leaf-settings","kind":"leaf","window_id":"settings"}}],
				"floating":[],"floating_stacks":[]},
			{"id":"desktop-focus","name":"Focus — Tasks","order":1,"purpose":"focus","focus_owner":"tasks",
				"groups":[{"id":"group-focus","frame":{"x":0,"y":0,"width":1,"height":1},
					"root":{"id":"leaf-focus","kind":"leaf","window_id":"tasks"}}],
				"floating":[],"floating_stacks":[]}
		],
		"windows":{
			"tasks":{"id":"tasks","app":"tasks","route":{"pathname":"/tasks","search":{}},"nav_stack":[],
				"pinned":false,"placement":"tiled","desktop_id":"desktop-focus",
				"floating_rect":{"x":0.1,"y":0.1,"width":0.5,"height":0.5},"minimized":false` + anchor + `},
			"settings":{"id":"settings","app":"settings","route":{"pathname":"/settings","search":{}},"nav_stack":[],
				"pinned":false,"placement":"tiled","desktop_id":"desktop-default",
				"floating_rect":{"x":0.2,"y":0.2,"width":0.5,"height":0.5},"minimized":false}
		},
		"history":{"undo":[{"command_id":"window.zoom","before":{"desktops":[],"windows":{}},
			"after":{"desktops":[],"windows":{}}}],"redo":[]},
		"overrides":{},
		"updated_at":"2026-08-30T10:00:00Z"
	}`
	var compact json.RawMessage
	if err := json.Unmarshal([]byte(document), &compact); err != nil {
		t.Fatalf("legacy fixture is not valid JSON: %v", err)
	}
	return []byte(document)
}

func TestMigrateLegacySnapshotV3(t *testing.T) {
	t.Run("Should keep a focus owner zoomed on its former focus desktop with its return anchor", func(t *testing.T) {
		t.Parallel()
		migrated, err := MigrateLegacySnapshotV3(legacyFocusSnapshotJSON(t, true))
		if err != nil {
			t.Fatalf("MigrateLegacySnapshotV3() error = %v", err)
		}
		if migrated.Version != SnapshotVersion || migrated.Revision != 5 {
			t.Fatalf(
				"migrated header = version %d revision %d, want version 4 revision 5",
				migrated.Version,
				migrated.Revision,
			)
		}
		if len(migrated.Desktops) != 2 || migrated.Desktops[1].ID != "desktop-focus" {
			t.Fatalf("former focus desktop did not survive as a regular desktop: %+v", migrated.Desktops)
		}
		tasks := migrated.Windows["tasks"]
		if !tasks.Zoomed || tasks.DesktopID != "desktop-focus" || tasks.Placement != WindowPlacementTiled ||
			tasks.ReturnAnchor == nil || tasks.ReturnAnchor.DesktopID != "desktop-default" || tasks.ReturnAnchor.Zoomed {
			t.Fatalf("migrated owner = %+v", tasks)
		}
		if len(migrated.History.Undo) != 0 || len(migrated.History.Redo) != 0 {
			t.Fatalf("legacy history survived migration: %+v", migrated.History)
		}
		requireValidSnapshot(t, migrated)
	})

	t.Run("Should take a migrated focus owner home and drop its desktop on unzoom", func(t *testing.T) {
		t.Parallel()
		migrated, err := MigrateLegacySnapshotV3(legacyFocusSnapshotJSON(t, true))
		if err != nil {
			t.Fatalf("MigrateLegacySnapshotV3() error = %v", err)
		}
		reducer := &reducer{generate: func(kind string) (string, error) { return kind + "-generated", nil }}
		reduced, err := reducer.reduce(&migrated, ZoomWindowCommand{WindowID: "tasks"})
		if err != nil || !reduced.changed {
			t.Fatalf("reduce(window.zoom) = %+v, error = %v", reduced, err)
		}
		unzoomed := NormalizeSnapshot(migrated)
		if len(unzoomed.Desktops) != 1 || unzoomed.Desktops[0].ID != "desktop-default" {
			t.Fatalf("former focus desktop survived unzoom: %+v", unzoomed.Desktops)
		}
		tasks := unzoomed.Windows["tasks"]
		if tasks.Zoomed || tasks.DesktopID != "desktop-default" || tasks.Placement != WindowPlacementTiled ||
			tasks.ReturnAnchor != nil {
			t.Fatalf("returned owner = %+v", tasks)
		}
		root := unzoomed.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindSplit || len(root.Children) != 2 ||
			valueOrZero(root.Children[0].WindowID) != "tasks" || valueOrZero(root.Children[1].WindowID) != "settings" {
			t.Fatalf("source group was not restored exactly: %+v", root)
		}
		requireValidSnapshot(t, unzoomed)
	})

	t.Run("Should keep an anchorless focus owner zoomed on its desktop", func(t *testing.T) {
		t.Parallel()
		migrated, err := MigrateLegacySnapshotV3(legacyFocusSnapshotJSON(t, false))
		if err != nil {
			t.Fatalf("MigrateLegacySnapshotV3() error = %v", err)
		}
		if len(migrated.Desktops) != 2 {
			t.Fatalf("owner without an anchor must stay on its desktop: %+v", migrated.Desktops)
		}
		tasks := migrated.Windows["tasks"]
		if !tasks.Zoomed || tasks.DesktopID != "desktop-focus" || tasks.Placement != WindowPlacementTiled ||
			tasks.ReturnAnchor != nil {
			t.Fatalf("anchorless owner = %+v", tasks)
		}
		requireValidSnapshot(t, migrated)
	})

	t.Run("Should reject a document that is not version 3", func(t *testing.T) {
		t.Parallel()
		_, err := MigrateLegacySnapshotV3([]byte(`{"version":2,"workspace_id":"workspace-a"}`))
		if err == nil {
			t.Fatal("MigrateLegacySnapshotV3() accepted a version 2 document")
		}
		if _, isSyntax := errors.AsType[*json.SyntaxError](err); isSyntax {
			t.Fatalf("version mismatch reported as syntax error: %v", err)
		}
		if !strings.Contains(err.Error(), "version 2") {
			t.Fatalf("version mismatch error = %v, want version 2", err)
		}
	})
}

// Invariant: stored apps reconcile without mutating input; topology, client focus and routes remain usable.
// Owning layer: windowmanager durable aggregate; canonical suite: snapshot_v3_test.go.
func TestReconcileRegisteredApps(t *testing.T) {
	t.Parallel()
	registered := func(app string) bool { return app == "session" || app == "settings" }
	t.Run("Should repair a tiled frame after dropping a retired app [UT-012]", func(t *testing.T) {
		t.Parallel()
		snapshot := reconcileSnapshotFixture(t)
		before, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("Marshal(before) = %v", err)
		}
		got, changed := ReconcileRegisteredApps(snapshot, registered)
		if !changed || len(got.Windows) != 2 {
			t.Fatalf("reconcile = %+v, changed = %v", got, changed)
		}
		root := got.Desktops[0].Groups[0].Root
		if len(root.Children) != 2 || valueOrZero(root.Children[0].WindowID) != "session" ||
			valueOrZero(root.Children[1].WindowID) != "settings" ||
			!reflect.DeepEqual(root.Weights, []float64{0.5, 0.5}) {
			t.Fatalf("surviving frame = %+v", root)
		}
		after, err := json.Marshal(snapshot)
		if err != nil {
			t.Fatalf("Marshal(after) = %v", err)
		}
		if string(after) != string(before) {
			t.Fatal("reconciliation mutated its input")
		}
		requireValidSnapshot(t, got)
	})
	t.Run("Should remove an empty frame and repair focus to the next stored window [UT-013]", func(t *testing.T) {
		t.Parallel()
		snapshot := reconcileSnapshotFixture(t)
		snapshot.Desktops[0].Groups = []LayoutGroup{
			{ID: "retired", Frame: NormalizedRect{Width: 0.5, Height: 1},
				Root: LayoutNode{ID: "retired-leaf", Kind: NodeKindLeaf, WindowID: new(WindowID("knowledge"))}},
			{ID: "kept", Frame: NormalizedRect{X: 0.5, Width: 0.5, Height: 1},
				Root: LayoutNode{ID: "kept-leaf", Kind: NodeKindLeaf, WindowID: new(WindowID("session"))}},
		}
		delete(snapshot.Windows, "settings")
		got, changed := ReconcileRegisteredApps(snapshot, registered)
		view := repairClientView(ClientView{ActiveDesktopID: "desktop", FocusedWindowID: new(WindowID("knowledge")),
			FocusOrder: []WindowID{"knowledge", "session"}}, got)
		if !changed || len(got.Desktops[0].Groups) != 1 || valueOrZero(view.FocusedWindowID) != "session" {
			t.Fatalf("reconciled topology = %+v, focus = %+v", got.Desktops, view)
		}
		requireValidSnapshot(t, got)
		delete(snapshot.Windows, "session")
		snapshot = NormalizeSnapshot(snapshot)
		empty, changed := ReconcileRegisteredApps(snapshot, registered)
		view = repairClientView(view, empty)
		if !changed || len(empty.Windows) != 0 || len(empty.Desktops[0].Groups) != 0 || view.FocusedWindowID != nil {
			t.Fatalf("empty desktop = %+v, focus = %+v", empty, view)
		}
		requireValidSnapshot(t, empty)
	})
	t.Run("Should retain usable history and closed windows after retirement", func(t *testing.T) {
		t.Parallel()
		snapshot := reconcileSnapshotFixture(t)
		snapshot.History.Undo = []HistoryEntry{{Before: snapshotState(snapshot), After: snapshotState(snapshot)}}
		snapshot.ClosedEntries = []ClosedEntry{{DesktopID: "desktop", Rect: NormalizedRect{Width: 1, Height: 1},
			ActiveID: new(WindowID("knowledge")), Windows: []Window{snapshot.Windows["knowledge"], snapshot.Windows["settings"]}}}
		got, changed := ReconcileRegisteredApps(snapshot, registered)
		if !changed || len(got.History.Undo) != 1 || len(got.History.Undo[0].Before.Windows) != 2 ||
			len(got.History.Undo[0].After.Windows) != 2 || len(got.ClosedEntries) != 1 ||
			len(got.ClosedEntries[0].Windows) != 1 || got.ClosedEntries[0].Windows[0].App != "settings" ||
			valueOrZero(got.ClosedEntries[0].ActiveID) != "settings" {
			t.Fatalf("reconciled history = %+v, closed windows = %+v", got.History, got.ClosedEntries)
		}
		requireValidSnapshot(t, got)
		again, changed := ReconcileRegisteredApps(got, registered)
		if changed || !reflect.DeepEqual(got, again) {
			t.Fatal("reconciled restore state changed on second pass")
		}
	})
	t.Run("Should rewrite retained settings navigation once [UT-014]", func(t *testing.T) {
		t.Parallel()
		snapshot := reconcileSnapshotFixture(t)
		window := snapshot.Windows["settings"]
		window.Route.Pathname = "/settings/memory"
		window.NavStack = []RouteIntent{{Pathname: "/settings/memory", Search: RouteSearch{}}}
		snapshot.Windows["settings"] = window
		got, changed := ReconcileRegisteredApps(snapshot, registered)
		if !changed || got.Windows["settings"].Route.Pathname != "/settings" ||
			got.Windows["settings"].NavStack[0].Pathname != "/settings" {
			t.Fatalf("reconciled settings = %+v, changed = %v", got.Windows["settings"], changed)
		}
		again, changed := ReconcileRegisteredApps(got, registered)
		if changed || !reflect.DeepEqual(got, again) {
			t.Fatalf("second reconciliation changed snapshot: %+v", again)
		}
		requireValidSnapshot(t, again)
	})
}

func reconcileSnapshotFixture(t *testing.T) Snapshot {
	t.Helper()
	snapshot := Snapshot{Version: SnapshotVersion, WorkspaceID: "workspace-a", Revision: 1,
		Desktops: []Desktop{{ID: "desktop", Name: "Desktop", Groups: []LayoutGroup{{ID: "frame",
			Frame: NormalizedRect{Width: 1, Height: 1}, Root: LayoutNode{ID: "split", Kind: NodeKindSplit,
				Axis: new(AxisHorizontal), Weights: []float64{0.25, 0.5, 0.25}}}}}},
		Windows: map[WindowID]Window{}}
	for _, app := range []string{"session", "knowledge", "settings"} {
		id := WindowID(app)
		snapshot.Windows[id] = Window{ID: id, App: app, DesktopID: "desktop", Placement: WindowPlacementTiled,
			Route: RouteIntent{Pathname: "/" + app, Search: RouteSearch{}}, FloatingRect: NormalizedRect{Width: 0.5, Height: 0.5}}
		snapshot.Desktops[0].Groups[0].Root.Children = append(snapshot.Desktops[0].Groups[0].Root.Children,
			LayoutNode{ID: NodeID("leaf-" + app), Kind: NodeKindLeaf, WindowID: new(id)})
	}
	return snapshot
}
