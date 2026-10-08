package windowmanager

// Suite: legacy aggregate migration
// Invariant: a stored v3 aggregate passes through the v4 shape into v5; former focus
// desktops are regular desktops hosting their owner as a lifted zoom that unzoom takes home.
// Boundary IN: raw stored JSON.
// Boundary OUT: repository storage and transport decoding.

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
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
				"migrated header = version %d revision %d, want version %d revision 5",
				migrated.Version,
				migrated.Revision,
				SnapshotVersion,
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

// Invariant: app retirement preserves every durable topology and navigation entry; this migration suite owns it.
func TestMigrateSnapshotV4(t *testing.T) {
	t.Parallel()
	t.Run(
		"Should preserve both retired tabs and closed history while migrating once [UT-120 UT-121 UT-122 UT-123]",
		func(t *testing.T) {
			t.Parallel()
			snapshot, err := MigrateLegacySnapshotV3(legacyFocusSnapshotJSON(t, true))
			if err != nil {
				t.Fatal(err)
			}
			snapshot.Version = PreviousSnapshotVersion
			snapshot.Desktops = snapshot.Desktops[:1]
			snapshot.Desktops[0].Groups[0].Root = LayoutNode{
				ID:        "stack",
				Kind:      NodeKindStack,
				WindowIDs: []WindowID{"tasks", "settings"},
				ActiveID:  new(WindowID("settings")),
			}
			for id, window := range snapshot.Windows {
				window.DesktopID = snapshot.Desktops[0].ID
				window.Placement = WindowPlacementStacked
				window.Zoomed = false
				window.ReturnAnchor = nil
				window.App = "jobs"
				window.Route = RouteIntent{Pathname: "/jobs/morning-digest", Search: RouteSearch{}}
				if id == "settings" {
					window.App = "triggers"
					window.Route.Pathname = "/triggers/rerun-delivery"
				}
				window.NavStack = []RouteIntent{
					{Pathname: "/" + window.App, Search: RouteSearch{"event": json.RawMessage(`"session.stopped"`)}},
				}
				snapshot.Windows[id] = window
			}
			snapshot.ClosedEntries = []ClosedEntry{
				{Windows: []Window{snapshot.Windows["settings"]}, DesktopID: snapshot.Desktops[0].ID, Rect: fullRect()},
			}
			snapshot.History.Undo = []HistoryEntry{{Before: snapshotState(snapshot), After: snapshotState(snapshot)}}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			got, err := MigrateSnapshotV4(raw)
			if err != nil {
				t.Fatal(err)
			}
			if got.Version != SnapshotVersion || got.Revision != snapshot.Revision+1 || len(got.Windows) != 2 ||
				!reflect.DeepEqual(got.Desktops, snapshot.Desktops) {
				t.Fatalf("migration changed topology: %+v", got)
			}
			for id, before := range snapshot.Windows {
				after := got.Windows[id]
				if after.App != "automations" || after.Route.Pathname != "/automations"+before.Route.Pathname ||
					after.DesktopID != before.DesktopID ||
					after.Placement != before.Placement ||
					after.FloatingRect != before.FloatingRect {
					t.Fatalf("window lost state: %+v", after)
				}
			}
			closed := got.ClosedEntries[0].Windows[0]
			if len(got.ClosedEntries) != 1 || len(got.ClosedEntries[0].Windows) != 1 || closed.App != "automations" ||
				closed.NavStack[0].Pathname != "/automations" ||
				string(closed.NavStack[0].Search["q"]) != `"session.stopped"` ||
				string(closed.NavStack[0].Search["start"]) != `"event"` {
				t.Fatalf("closed navigation = %+v", closed)
			}
			if got.History.Undo[0].Before.Windows["tasks"].App != "automations" ||
				got.History.Undo[0].After.Windows["settings"].App != "automations" {
				t.Fatal("undo would resurrect retired apps")
			}
			requireValidSnapshot(t, got)
		},
	)
	t.Run("Should chain v3 migration with app retirement and advance one revision [UT-124]", func(t *testing.T) {
		t.Parallel()
		raw := strings.ReplaceAll(string(legacyFocusSnapshotJSON(t, true)), `"app":"tasks"`, `"app":"jobs"`)
		raw = strings.ReplaceAll(raw, `"pathname":"/tasks"`, `"pathname":"/jobs"`)
		got, err := MigrateLegacySnapshotV3([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if got.Version != SnapshotVersion || got.Revision != 5 || got.Windows["tasks"].App != "automations" ||
			got.Windows["tasks"].Route.Pathname != "/automations" {
			t.Fatalf("v3 chain = %+v", got)
		}
	})
	t.Run("Should preserve the shared web redirect vectors without mutating input [UT-126]", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile("testdata/retired_app_routes.json")
		if err != nil {
			t.Fatal(err)
		}
		var vectors []struct {
			App      string       `json:"app"`
			Input    RouteIntent  `json:"input"`
			Expected *RouteIntent `json:"expected"`
		}
		if err := json.Unmarshal(raw, &vectors); err != nil {
			t.Fatal(err)
		}
		for _, vector := range vectors {
			before, err := json.Marshal(vector.Input)
			if err != nil {
				t.Fatal(err)
			}
			got := RewriteRetiredAppRoute(vector.App, vector.Input)
			expected := vector.Input
			if vector.Expected != nil {
				expected = *vector.Expected
			}
			if !reflect.DeepEqual(got, expected) {
				t.Fatalf("%s %+v = %+v, want %+v", vector.App, vector.Input, got, vector.Expected)
			}
			after, err := json.Marshal(vector.Input)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("route rewrite mutated input")
			}
		}
	})
}
