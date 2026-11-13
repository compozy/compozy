package windowmanager

// Suite: stored snapshot conversion. Invariant: frozen v3/v4/v5 input upgrades
// deterministically without losing content, IDs, routes, pins or split geometry.
import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func legacyFixture(ids ...WindowID) SnapshotV5 {
	s := SnapshotV5{
		Version:     5,
		WorkspaceID: "workspace-a",
		Desktops:    []DesktopV5{{ID: "d1", Name: "One"}},
		Windows:     map[WindowID]WindowV5{},
	}
	for _, id := range ids {
		s.Windows[id] = WindowV5{
			ID:           id,
			App:          "Test",
			DesktopID:    "d1",
			Route:        testRoute("/" + string(id)),
			NavStack:     []RouteIntent{testRoute("/previous")},
			Placement:    WindowPlacementTiled,
			FloatingRect: fullRect(),
		}
	}
	return s
}
func oldLeaf(id WindowID) LayoutNodeV5 {
	return LayoutNodeV5{ID: NodeID("node-" + string(id)), Kind: NodeKindLeaf, WindowID: new(id)}
}
func oldSplit(axis Axis, children ...LayoutNodeV5) LayoutNodeV5 {
	return LayoutNodeV5{
		ID:       "split",
		Kind:     NodeKindSplit,
		Axis:     new(axis),
		Children: children,
		Weights:  equalWeights(len(children)),
	}
}
func oldGroup(id GroupID, root LayoutNodeV5, frame NormalizedRect) LayoutGroupV5 {
	return LayoutGroupV5{ID: id, Root: root, Frame: frame}
}
func convertFixture(t *testing.T, s SnapshotV5) Snapshot {
	t.Helper()
	out, _, err := ConvertSnapshotV5(s)
	if err != nil {
		t.Fatal(err)
	}
	requireValidSnapshot(t, out)
	return out
}
func encodeFixture(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMigrateSnapshotV5(t *testing.T) {
	t.Run("Should translate retired overrides at every stored snapshot decoder", func(t *testing.T) {
		t.Parallel()
		legacy := legacyFixture("a")
		legacy.Desktops[0].Groups = []LayoutGroupV5{oldGroup("g", oldLeaf("a"), fullRect())}
		current := convertFixture(t, legacy)
		for _, version := range []uint32{3, 4, 5, SnapshotVersion} {
			var raw map[string]json.RawMessage
			encoded := encodeFixture(t, legacy)
			if version == SnapshotVersion {
				encoded = encodeFixture(t, current)
			}
			if err := json.Unmarshal(encoded, &raw); err != nil {
				t.Fatal(err)
			}
			raw["version"] = encodeFixture(t, version)
			raw["overrides"] = json.RawMessage(`{"new_window_policy":"floating","history_limit":25,` +
				`"raise_on_focus":[],"snap":{"edge_band":false},"bindings":{"top_center":42}}`)
			got, _, err := DecodeSnapshot(encodeFixture(t, raw))
			if err != nil {
				t.Fatalf("version %d: %v", version, err)
			}
			if got.Overrides.NewWindowPolicy == nil || *got.Overrides.NewWindowPolicy != NewWindowTab ||
				got.Overrides.HistoryLimit == nil || *got.Overrides.HistoryLimit != 25 ||
				got.Overrides.RaiseOnFocus != nil || got.Overrides.Snap != nil || got.Overrides.Bindings != nil ||
				len(got.Windows) != 1 {
				t.Fatalf("version %d: snapshot = %+v", version, got)
			}
		}
	})

	t.Run("Should root stack order and active (UT-040)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c")
		s.Desktops[0].Groups = []LayoutGroupV5{
			oldGroup(
				"g",
				LayoutNodeV5{
					ID:        "stack",
					Kind:      "stack",
					WindowIDs: []WindowID{"a", "b", "c"},
					ActiveID:  new(WindowID("b")),
				},
				fullRect(),
			),
		}
		out := convertFixture(t, s)
		d := out.Desktops[0]
		if len(d.Tabs) != 3 || d.ActiveTabID != out.Windows["b"].TabID {
			t.Fatalf("stack=%+v", d)
		}
		for i, id := range []WindowID{"a", "b", "c"} {
			if valueOrZero(d.Tabs[i].Root.WindowID) != id {
				t.Fatal("stack order changed")
			}
		}
	})
	t.Run("Should split preserves IDs and weights (UT-041)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b")
		root := oldSplit(AxisHorizontal, oldLeaf("a"), oldLeaf("b"))
		root.Weights = []float64{.7, .3}
		s.Desktops[0].Groups = []LayoutGroupV5{oldGroup("g", root, fullRect())}
		out := convertFixture(t, s)
		tab := out.Desktops[0].Tabs[0]
		if tab.ID != "g" || tab.Root.ID != root.ID || !slices.Equal(tab.Root.Weights, root.Weights) ||
			tab.Root.Children[0].ID != "node-a" {
			t.Fatalf("split=%+v", tab)
		}
	})
	t.Run("Should nested stack extras (UT-042)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c")
		s.Desktops[0].Groups = []LayoutGroupV5{
			oldGroup(
				"g",
				oldSplit(
					AxisHorizontal,
					LayoutNodeV5{
						ID:        "stack",
						Kind:      "stack",
						WindowIDs: []WindowID{"a", "b"},
						ActiveID:  new(WindowID("a")),
					},
					oldLeaf("c"),
				),
				fullRect(),
			),
		}
		out := convertFixture(t, s)
		if !slices.Equal(nodeWindowIDs(out.Desktops[0].Tabs[0].Root), []WindowID{"a", "c"}) ||
			valueOrZero(out.Desktops[0].Tabs[1].Root.WindowID) != "b" {
			t.Fatalf("nested=%+v", out)
		}
	})
	t.Run("Should retain the active stack member in an inseparable island (UT-042)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c")
		s.Desktops[0].Groups = []LayoutGroupV5{
			oldGroup(
				"g1",
				LayoutNodeV5{ID: "stack", Kind: "stack", WindowIDs: []WindowID{"a", "b"}, ActiveID: new(WindowID("b"))},
				fullRect(),
			),
			oldGroup("g2", oldLeaf("c"), fullRect()),
		}
		out := convertFixture(t, s)
		if len(out.Desktops[0].Tabs) != 3 || out.Desktops[0].Tabs[0].ID != "g1" ||
			valueOrZero(out.Desktops[0].Tabs[0].Root.WindowID) != "b" ||
			valueOrZero(out.Desktops[0].Tabs[1].Root.WindowID) != "a" {
			t.Fatalf("island stack conversion = %+v", out.Desktops[0])
		}
	})

	for _, three := range []bool{false, true} {
		id := "UT-043"
		if three {
			id = "UT-044"
		}
		t.Run("Should merge island cuts ("+id+")", func(t *testing.T) {
			t.Parallel()
			s := legacyFixture("a", "b")
			s.Desktops[0].Groups = []LayoutGroupV5{
				oldGroup("left", oldLeaf("a"), NormalizedRect{Width: .5, Height: 1}),
				oldGroup("right", oldLeaf("b"), NormalizedRect{X: .5, Width: .5, Height: 1}),
			}
			if three {
				s.Windows["c"] = WindowV5{ID: "c", App: "Test", DesktopID: "d1", Route: testRoute("/c")}
				s.Desktops[0].Groups[0].Frame.Width = .3
				s.Desktops[0].Groups[1].Frame = NormalizedRect{X: .3, Width: .7, Height: .4}
				s.Desktops[0].Groups = append(
					s.Desktops[0].Groups,
					oldGroup("bottom", oldLeaf("c"), NormalizedRect{X: .3, Y: .4, Width: .7, Height: .6}),
				)
			}
			out := convertFixture(t, s)
			root := out.Desktops[0].Tabs[0].Root
			expectedWeights := []float64{.5, .5}
			if three {
				expectedWeights = []float64{.3, .7}
			}
			if len(out.Desktops[0].Tabs) != 1 || valueOrZero(root.Axis) != AxisHorizontal ||
				!slices.Equal(root.Weights, expectedWeights) {
				t.Fatalf("islands=%+v", out)
			}
			if three &&
				(valueOrZero(root.Children[1].Axis) != AxisVertical || !slices.Equal(root.Children[1].Weights, []float64{.4, .6})) {
				t.Fatalf("cut=%+v", root)
			}
		})
	}
	t.Run("Should inseparable islands (UT-045)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c", "d")
		frames := []NormalizedRect{
			{Width: .7, Height: .3},
			{X: .7, Width: .3, Height: .7},
			{X: .3, Y: .7, Width: .7, Height: .3},
			{Y: .3, Width: .3, Height: .7},
		}
		for i, id := range []WindowID{"a", "b", "c", "d"} {
			s.Desktops[0].Groups = append(s.Desktops[0].Groups, oldGroup(GroupID(id), oldLeaf(id), frames[i]))
		}
		out := convertFixture(t, s)
		if len(out.Desktops[0].Tabs) != 4 || !slices.Equal(tabIDs(out.Desktops[0].Tabs), []TabID{"a", "d", "c", "b"}) {
			t.Fatalf("pinwheel=%+v", out)
		}
	})
	t.Run("Should floaters minimized and floating stack (UT-046)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c", "d")
		s.Desktops[0].FloatingStacks = []FloatingStackV5{
			{ID: "stack", WindowIDs: []WindowID{"a", "b"}, Rect: fullRect()},
		}
		s.Desktops[0].Floating = []WindowID{"c", "d"}
		w := s.Windows["d"]
		w.Minimized = true
		s.Windows[w.ID] = w
		out, summary, err := ConvertSnapshotV5(s)
		if err != nil {
			t.Fatal(err)
		}
		if summary.FloatingToTabs != 3 || summary.MinimizedToTabs != 1 {
			t.Fatalf("summary=%+v", summary)
		}
		for i, id := range []WindowID{"a", "b", "c", "d"} {
			if valueOrZero(out.Desktops[0].Tabs[i].Root.WindowID) != id {
				t.Fatal("floating order changed")
			}
		}
	})
	t.Run("Should empty desktop (UT-047)", func(t *testing.T) {
		t.Parallel()
		out := convertFixture(t, legacyFixture())
		if _, ok := out.Windows["win-newtab-d1"]; !ok {
			t.Fatal("New Tab absent")
		}
	})
	t.Run("Should pins (UT-048)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c")
		s.Desktops[0].Groups = []LayoutGroupV5{
			oldGroup("split-group", oldSplit(AxisHorizontal, oldLeaf("a"), oldLeaf("b")), fullRect()),
			oldGroup("solo-group", oldLeaf("c"), fullRect()),
		}
		for _, id := range []WindowID{"a", "c"} {
			w := s.Windows[id]
			w.Pinned = true
			s.Windows[id] = w
		}
		out, summary, err := ConvertSnapshotV5(s)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Desktops[0].Tabs[0].Pinned || out.Desktops[0].Tabs[0].ID != "solo-group" ||
			out.Desktops[0].Tabs[1].Pinned ||
			summary.PinsDropped != 1 {
			t.Fatalf("pins=%+v summary=%+v", out, summary)
		}
	})
	t.Run("Should zoom returns and island merge (UT-049)", func(t *testing.T) {
		t.Parallel()
		for _, missing := range []bool{false, true} {
			s := legacyFixture("a", "b", "c", "floater")
			s.Desktops[0].Floating = []WindowID{"floater"}
			s.Desktops[0].Groups = []LayoutGroupV5{
				oldGroup("left", oldLeaf("a"), NormalizedRect{Width: .5, Height: 1}),
				oldGroup("right", oldLeaf("b"), NormalizedRect{X: .5, Width: .5, Height: 1}),
			}
			s.Desktops = append(
				s.Desktops,
				DesktopV5{
					ID:     "zoom",
					Name:   "Zoom",
					Order:  1,
					Groups: []LayoutGroupV5{oldGroup("zoom-group", oldLeaf("c"), fullRect())},
				},
			)
			target := DesktopID("d1")
			if missing {
				target = "gone"
			}
			w := s.Windows["c"]
			w.DesktopID = "zoom"
			w.Zoomed = true
			w.ReturnAnchor = &ReturnAnchorV5{DesktopID: target}
			s.Windows[w.ID] = w
			out := convertFixture(t, s)
			want := 1
			if missing {
				want = 2
			}
			if len(out.Desktops) != want {
				t.Fatalf("zoom=%+v", out)
			}
			if !missing &&
				(len(out.Desktops[0].Tabs) != 3 || out.Desktops[0].Tabs[1].ID != "zoom-group" || out.Desktops[0].ActiveTabID != "zoom-group") {
				t.Fatalf("zoom tab=%+v", out)
			}
		}
	})
	t.Run("Should arbitrary child count and small weight (UT-050)", func(t *testing.T) {
		t.Parallel()
		var ids []WindowID
		var nodes []LayoutNodeV5
		for i := range 11 {
			id := WindowID(string(rune('a' + i)))
			ids = append(ids, id)
			nodes = append(nodes, oldLeaf(id))
		}
		s := legacyFixture(ids...)
		root := oldSplit(AxisHorizontal, nodes...)
		s.Desktops[0].Groups = []LayoutGroupV5{oldGroup("g", root, fullRect())}
		out := convertFixture(t, s)
		if !slices.Equal(out.Desktops[0].Tabs[0].Root.Weights, root.Weights) {
			t.Fatal("weights changed")
		}
		root.Weights[0] = .02
		for i := 1; i < len(root.Weights); i++ {
			root.Weights[i] = .98 / 10
		}
		s.Desktops[0].Groups[0].Root = root
		out = convertFixture(t, s)
		if !slices.Equal(out.Desktops[0].Tabs[0].Root.Weights, root.Weights) {
			t.Fatal("small weight changed")
		}
	})
	t.Run("Should closed windows become tabs (UT-051)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture()
		entry := ClosedEntryV5{DesktopID: "d1"}
		for _, id := range []WindowID{"a", "b", "c"} {
			entry.Windows = append(
				entry.Windows,
				WindowV5{ID: id, App: "Test", DesktopID: "d1", Route: testRoute("/closed"), Pinned: id == "b"},
			)
		}
		s.ClosedEntries = []ClosedEntryV5{entry}
		out := convertFixture(t, s)
		got := out.ClosedEntries[0]
		if len(got.Tabs) != 3 || !got.Tabs[1].Pinned || got.Member != nil {
			t.Fatalf("closed=%+v", got)
		}
		for i, id := range []WindowID{"a", "b", "c"} {
			tab := got.Tabs[i]
			if tab.Index != len(out.Desktops[0].Tabs)+i || tab.Pinned != (id == "b") ||
				valueOrZero(tab.Root.WindowID) != id ||
				got.Windows[i].ID != id {
				t.Fatalf("closed index %d = %+v", i, got)
			}
		}
	})
	t.Run("Should content and override preservation (UT-052)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a", "b", "c")
		s.Desktops[0].Groups = []LayoutGroupV5{oldGroup("g", oldLeaf("a"), fullRect())}
		s.Desktops = append(s.Desktops, DesktopV5{ID: "d2", Name: "Two", Order: 1, Floating: []WindowID{"c"}})
		s.Desktops[0].Floating = []WindowID{"b"}
		for _, id := range []WindowID{"a", "b", "c"} {
			w := s.Windows[id]
			if id == "c" {
				w.DesktopID = "d2"
			}
			w.InstanceKey = new("instance-" + string(id))
			w.Route.Search = RouteSearch{"q": json.RawMessage(`"a value"`), "enabled": json.RawMessage(`true`)}
			w.NavStack = []RouteIntent{
				testRoute("/before/" + string(id)),
				{Pathname: "/search", Search: RouteSearch{"q": json.RawMessage(`"previous"`)}},
			}
			s.Windows[id] = w
		}
		s.History = json.RawMessage(`{"unknown_old_history":true}`)
		s.Overrides = WorkspaceConfigV5{
			RaiseOnFocus:    new(true),
			NewWindowPolicy: new(NewWindowPolicy("floating")),
			HistoryLimit:    new(12),
		}
		before := encodeFixture(t, s)
		out := convertFixture(t, s)
		for id, beforeWindow := range s.Windows {
			after := out.Windows[id]
			if after.ID != id || after.App != beforeWindow.App || after.DesktopID != beforeWindow.DesktopID ||
				!reflect.DeepEqual(after.InstanceKey, beforeWindow.InstanceKey) ||
				!routeIntentsEqual(after.Route, beforeWindow.Route) ||
				!reflect.DeepEqual(after.NavStack, beforeWindow.NavStack) {
				t.Fatalf("window %s changed: %+v want=%+v", id, after, beforeWindow)
			}
		}
		if len(out.Windows) != len(s.Windows) ||
			len(out.History.Undo) != 0 ||
			out.Overrides.RaiseOnFocus != nil ||
			valueOrZero(out.Overrides.NewWindowPolicy) != NewWindowTab ||
			valueOrZero(out.Overrides.HistoryLimit) != 12 ||
			!bytes.Equal(before, encodeFixture(t, s)) {
			t.Fatalf("conversion=%+v", out)
		}
	})
	t.Run("Should deterministic and v6 identity (UT-053)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a")
		s.Desktops[0].Floating = []WindowID{"a"}
		raw := encodeFixture(t, s)
		a, _, err := DecodeSnapshot(raw)
		if err != nil {
			t.Fatal(err)
		}
		b, _, err := DecodeSnapshot(raw)
		if err != nil || !bytes.Equal(encodeFixture(t, a), encodeFixture(t, b)) {
			t.Fatalf("determinism=%v", err)
		}
		b, _, err = DecodeSnapshot(encodeFixture(t, a))
		if err != nil || !reflect.DeepEqual(a, b) {
			t.Fatalf("v6 identity=%v", err)
		}
	})
	t.Run("Should decode actual v3 focus and v4 bytes through the frozen chain (UT-054)", func(t *testing.T) {
		t.Parallel()
		for _, anchor := range []bool{false, true} {
			raw := legacyFocusSnapshotJSON(t, anchor)
			var legacy legacySnapshotV3
			if err := json.Unmarshal(raw, &legacy); err != nil {
				t.Fatal(err)
			}
			equivalent := SnapshotV5{
				Version:     5,
				WorkspaceID: legacy.WorkspaceID,
				Revision:    legacy.Revision + 1,
				Windows:     legacy.Windows,
				Overrides:   legacy.Overrides,
				UpdatedAt:   legacy.UpdatedAt,
			}
			for _, d := range legacy.Desktops {
				equivalent.Desktops = append(
					equivalent.Desktops,
					DesktopV5{
						ID:             d.ID,
						Name:           d.Name,
						Order:          d.Order,
						Groups:         d.Groups,
						Floating:       d.Floating,
						FloatingStacks: d.FloatingStacks,
					},
				)
			}
			owner := equivalent.Windows["tasks"]
			owner.Zoomed = true
			equivalent.Windows["tasks"] = owner
			want := convertFixture(t, equivalent)
			got, _, err := DecodeSnapshot(raw)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("v3 anchor=%v differs: %+v want=%+v error=%v", anchor, got, want, err)
			}
			v4 := strings.NewReplacer(`"version":3`, `"version":4`, `"revision":4`, `"revision":5`, `"purpose":"standard",`, "", `"purpose":"focus","focus_owner":"tasks",`, "", `"app":"tasks"`, `"app":"tasks","zoomed":true`).
				Replace(string(raw))
			got, _, err = DecodeSnapshot([]byte(v4))
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("v4 anchor=%v differs: %+v error=%v", anchor, got, err)
			}
		}
	})
	t.Run("Should missing membership fails closed (UT-055)", func(t *testing.T) {
		t.Parallel()
		out, _, err := ConvertSnapshotV5(legacyFixture("orphan"))
		if err == nil || out.Version != 0 {
			t.Fatalf("orphan=%+v error=%v", out, err)
		}
	})
	t.Run("Should generated collisions (UT-065)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("w", "win-newtab-d1")
		s.Desktops = append(
			s.Desktops,
			DesktopV5{
				ID:       "d2",
				Name:     "Two",
				Order:    1,
				Floating: []WindowID{"w"},
				Groups: []LayoutGroupV5{
					oldGroup(
						"tab-w",
						LayoutNodeV5{ID: "merge-d1-1", Kind: NodeKindLeaf, WindowID: new(WindowID("win-newtab-d1"))},
						fullRect(),
					),
				},
			},
		)
		s.Desktops[0].Groups = nil
		for id, w := range s.Windows {
			w.DesktopID = "d2"
			s.Windows[id] = w
		}
		out := convertFixture(t, s)
		if _, ok := out.Windows["win-newtab-d1-2"]; !ok {
			t.Fatal("window collision not avoided")
		}
		if out.Windows["w"].TabID != "tab-w-2" || out.Desktops[1].Tabs[0].ID != "tab-w" ||
			out.Desktops[1].Tabs[0].Root.ID != "merge-d1-1" {
			t.Fatalf("IDs=%+v", out)
		}
		merged := legacyFixture("a", "b")
		merged.Desktops[0].Groups = []LayoutGroupV5{
			oldGroup(
				"g1",
				LayoutNodeV5{ID: "merge-d1-1", Kind: NodeKindLeaf, WindowID: new(WindowID("a"))},
				NormalizedRect{Width: .5, Height: 1},
			),
			oldGroup("g2", oldLeaf("b"), NormalizedRect{X: .5, Width: .5, Height: 1}),
		}
		out = convertFixture(t, merged)
		if out.Desktops[0].Tabs[0].Root.ID != "merge-d1-1-2" ||
			out.Desktops[0].Tabs[0].Root.Children[0].ID != "merge-d1-1" {
			t.Fatalf("merge node collision = %+v", out.Desktops[0])
		}
		closed := legacyFixture()
		closed.ClosedEntries = []ClosedEntryV5{{DesktopID: "d1", Windows: []WindowV5{{
			ID: "win-newtab-d1", App: "Test", DesktopID: "d1", Route: testRoute("/closed"),
		}}}}
		out = convertFixture(t, closed)
		if out.Windows["win-newtab-d1-2"].ID == "" || out.ClosedEntries[0].Windows[0].ID != "win-newtab-d1" {
			t.Fatalf("closed window collision = %+v", out)
		}
	})
}

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
	t.Run("Should drop an empty ownerless legacy focus desktop", func(t *testing.T) {
		t.Parallel()
		raw := legacyFocusSnapshotJSON(t, false)
		var legacy legacySnapshotV3
		if err := json.Unmarshal(raw, &legacy); err != nil {
			t.Fatal(err)
		}
		delete(legacy.Windows, "tasks")
		legacy.Desktops[1].Groups = nil
		got, _, err := DecodeSnapshot(encodeFixture(t, legacy))
		if err != nil || len(got.Desktops) != 1 || got.Desktops[0].ID != "desktop-default" {
			t.Fatalf("empty focus=%+v error=%v", got, err)
		}
	})
	t.Run("Should reject non-v3 bytes at the v3 boundary", func(t *testing.T) {
		t.Parallel()
		if _, err := MigrateLegacySnapshotV3(
			[]byte(`{"version":4}`),
		); err == nil ||
			!strings.Contains(err.Error(), "version 4") {
			t.Fatalf("wrong-version error=%v", err)
		}
	})
}

func TestMigrateSnapshotV4(t *testing.T) {
	t.Run("Should retain the retired app route fixture through v4 and v5 decoding", func(t *testing.T) {
		t.Parallel()
		raw, err := os.ReadFile("testdata/retired_app_routes.json")
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			App      string       `json:"app"`
			Input    RouteIntent  `json:"input"`
			Expected *RouteIntent `json:"expected"`
		}
		if err := json.Unmarshal(raw, &cases); err != nil {
			t.Fatal(err)
		}
		for _, c := range cases {
			for _, version := range []uint32{4, 5} {
				legacy := legacyFixture("retired")
				legacy.Version = version
				w := legacy.Windows["retired"]
				w.App, w.Route, w.NavStack = c.App, c.Input, []RouteIntent{c.Input}
				legacy.Windows[w.ID] = w
				legacy.Desktops[0].Floating = []WindowID{w.ID}
				got, _, err := DecodeSnapshot(encodeFixture(t, legacy))
				if err != nil {
					t.Fatalf("v%d %s %s: %v", version, c.App, c.Input.Pathname, err)
				}
				want := c.Input
				if c.Expected != nil {
					want = *c.Expected
				}
				actual := got.Windows[w.ID]
				if actual.App != "automations" || !routeIntentsEqual(actual.Route, want) || len(actual.NavStack) != 1 ||
					!routeIntentsEqual(actual.NavStack[0], want) {
					t.Fatalf("v%d migrated=%+v want=%+v", version, actual, want)
				}
			}
		}
	})
	t.Run("Should reject non-v4 bytes at the v4 boundary", func(t *testing.T) {
		t.Parallel()
		if _, err := MigrateSnapshotV4([]byte(`{"version":5}`)); err == nil {
			t.Fatal("v4 boundary accepted v5 bytes")
		}
	})
}

// The aggregate owns reconcile/rebind across current, closed and history references.
func retainedReferenceSnapshot() Snapshot {
	s := validThreeWindowSnapshot()
	window := cloneWindow(s.Windows["w1"])
	window.ID, window.TabID = "closed", "closed-tab"
	s.ClosedEntries = []ClosedEntry{
		{
			DesktopID: "desktop-default",
			Windows:   []Window{window},
			Tabs: []ClosedTab{
				{
					ID:    window.TabID,
					Root:  LayoutNode{ID: "closed-leaf", Kind: NodeKindLeaf, WindowID: new(window.ID)},
					Index: 1,
				},
			},
		},
	}
	return s
}

func TestReconcileRegisteredApps(t *testing.T) {
	t.Run("Should prune unknown apps in live closed and undo/redo state without mutating input", func(t *testing.T) {
		t.Parallel()
		s := retainedReferenceSnapshot()
		unknown := s.Windows["w2"]
		unknown.App = "retired"
		s.Windows[unknown.ID] = unknown
		settings := s.Windows["w3"]
		settings.App = "settings"
		settings.Route = testRoute("/settings/memory")
		settings.NavStack = []RouteIntent{testRoute("/settings/memory")}
		s.Windows[settings.ID] = settings
		s.ClosedEntries[0].Windows[0].App = "retired"
		s.History = History{
			Undo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
			Redo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
		}
		before := encodeFixture(t, s)
		registered := func(app string) bool { return app == "One" || app == "settings" || app == newTabApp }
		got, changed := ReconcileRegisteredApps(s, registered)
		if !changed || len(got.Windows) != 2 || len(got.ClosedEntries) != 0 ||
			!bytes.Equal(before, encodeFixture(t, s)) {
			t.Fatalf("reconcile=%+v changed=%v", got, changed)
		}
		requireValidSnapshot(t, got)
		states := []State{
			snapshotState(got),
			got.History.Undo[0].Before,
			got.History.Undo[0].After,
			got.History.Redo[0].Before,
			got.History.Redo[0].After,
		}
		for _, state := range states {
			if len(state.Windows) != 2 || len(state.ClosedEntries) != 0 ||
				state.Windows["w3"].Route.Pathname != "/settings" ||
				state.Windows["w3"].NavStack[0].Pathname != "/settings" {
				t.Fatalf("retained state=%+v", state)
			}
		}
		again, changed := ReconcileRegisteredApps(got, registered)
		if changed || !reflect.DeepEqual(got, again) {
			t.Fatal("reconcile is not idempotent")
		}
	})
}

func TestReconcileTerminalWindows(t *testing.T) {
	t.Run("Should prune dead terminal references in live closed and undo/redo state", func(t *testing.T) {
		t.Parallel()
		s := retainedReferenceSnapshot()
		for id, key := range map[WindowID]string{"w1": "gone", "w2": "live"} {
			w := s.Windows[id]
			w.App = "terminal"
			w.InstanceKey = new(key)
			w.Route = testRoute("/terminal/" + key)
			s.Windows[id] = w
		}
		closed := &s.ClosedEntries[0].Windows[0]
		closed.App, closed.InstanceKey, closed.Route = "terminal", new("gone"), testRoute("/terminal/gone")
		s.History = History{
			Undo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
			Redo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
		}
		before := encodeFixture(t, s)
		got, changed := ReconcileTerminalWindows(s, func(id string) bool { return id == "live" })
		if !changed || !bytes.Equal(before, encodeFixture(t, s)) {
			t.Fatal("terminal reconcile did not isolate input")
		}
		requireValidSnapshot(t, got)
		for _, state := range []State{snapshotState(got), got.History.Undo[0].Before, got.History.Undo[0].After, got.History.Redo[0].Before, got.History.Redo[0].After} {
			if _, exists := state.Windows["w1"]; exists {
				t.Fatal("dead terminal remains")
			}
			if len(state.Windows) != 2 || len(state.ClosedEntries) != 0 ||
				valueOrZero(state.Windows["w2"].InstanceKey) != "live" {
				t.Fatalf("retained state=%+v", state)
			}
		}
		if _, changed := ReconcileTerminalWindows(got, func(id string) bool { return id == "live" }); changed {
			t.Fatal("terminal reconcile is not idempotent")
		}
	})
}

func TestRebindWorkspace(t *testing.T) {
	t.Run(
		"Should rewrite bindings in live closed and undo/redo state and preserve all other fields",
		func(t *testing.T) {
			t.Parallel()
			s := retainedReferenceSnapshot()
			route := RouteIntent{
				Pathname: "/sessions/bound",
				Search:   RouteSearch{"workspace": json.RawMessage(`"old"`), "q": json.RawMessage(`"keep"`)},
			}
			for id, w := range s.Windows {
				w.Route = cloneRouteIntent(route)
				w.NavStack = []RouteIntent{cloneRouteIntent(route), testRoute("/unbound")}
				s.Windows[id] = w
			}
			s.ClosedEntries[0].Windows[0].Route = cloneRouteIntent(route)
			s.ClosedEntries[0].Windows[0].NavStack = []RouteIntent{cloneRouteIntent(route)}
			s.History = History{
				Undo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
				Redo: []HistoryEntry{{Before: snapshotState(s), After: snapshotState(s)}},
			}
			before := encodeFixture(t, s)
			if !RebindWorkspace(&s, "old", "new") {
				t.Fatal("binding did not change")
			}
			want := bytes.ReplaceAll(before, []byte(`"old"`), []byte(`"new"`))
			if !bytes.Equal(encodeFixture(t, s), want) {
				t.Fatalf("rebound state=%s want=%s", encodeFixture(t, s), want)
			}
			requireValidSnapshot(t, s)
			if RebindWorkspace(&s, "old", "new") {
				t.Fatal("rebind is not idempotent")
			}
		},
	)
}
