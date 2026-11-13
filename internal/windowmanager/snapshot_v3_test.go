package windowmanager

// Suite: stored snapshot conversion. Invariant: frozen v3/v4/v5 input upgrades
// deterministically without losing content, IDs, routes, pins or split geometry.
import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
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
				s.Desktops[0].Groups[1].Frame.Height = .5
				s.Desktops[0].Groups = append(
					s.Desktops[0].Groups,
					oldGroup("bottom", oldLeaf("c"), NormalizedRect{X: .5, Y: .5, Width: .5, Height: .5}),
				)
			}
			out := convertFixture(t, s)
			root := out.Desktops[0].Tabs[0].Root
			if len(out.Desktops[0].Tabs) != 1 || valueOrZero(root.Axis) != AxisHorizontal ||
				!slices.Equal(root.Weights, []float64{.5, .5}) {
				t.Fatalf("islands=%+v", out)
			}
			if three &&
				(valueOrZero(root.Children[1].Axis) != AxisVertical || !slices.Equal(root.Children[1].Weights, []float64{.5, .5})) {
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
	})
	t.Run("Should content and override preservation (UT-052)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a")
		s.Desktops[0].Groups = []LayoutGroupV5{oldGroup("g", oldLeaf("a"), fullRect())}
		s.History = json.RawMessage(`{"unknown_old_history":true}`)
		s.Overrides = WorkspaceConfigV5{
			RaiseOnFocus:    new(true),
			NewWindowPolicy: new(NewWindowFloating),
			HistoryLimit:    new(12),
		}
		before := encodeFixture(t, s)
		out := convertFixture(t, s)
		if out.Windows["a"].Route.Pathname != "/a" || len(out.Windows["a"].NavStack) != 1 ||
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
	t.Run("Should v3 v4 chain (UT-054)", func(t *testing.T) {
		t.Parallel()
		s := legacyFixture("a")
		s.Desktops[0].Floating = []WindowID{"a"}
		want := convertFixture(t, s)
		for _, version := range []uint32{3, 4} {
			s.Version = version
			got, _, err := DecodeSnapshot(encodeFixture(t, s))
			if err != nil {
				t.Fatal(err)
			}
			got.Revision = want.Revision
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("v%d conversion differs", version)
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
