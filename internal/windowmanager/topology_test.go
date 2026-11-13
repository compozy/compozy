package windowmanager

// Suite: topology validation. Invariant: every window has one leaf in one tab,
// desktops retain content, and normalization preserves valid split geometry.
import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestInitialSnapshot(t *testing.T) {
	t.Run("Should create one desktop with a New Tab", func(t *testing.T) {
		t.Parallel()
		s := initialSnapshot("workspace-a")
		requireValidSnapshot(t, s)
		if len(s.Desktops) != 1 || len(s.Desktops[0].Tabs) != 1 || len(s.Windows) != 1 {
			t.Fatalf("default = %+v", s)
		}
		for _, w := range s.Windows {
			if w.App != "new-tab" {
				t.Fatalf("default window = %+v", w)
			}
		}
	})
}

func TestValidateSnapshot(t *testing.T) {
	cases := []struct {
		name   string
		code   string
		mutate func(*Snapshot)
	}{
		{"duplicate window", "topology.window_membership", func(s *Snapshot) {
			s.Desktops[0].Tabs = append(
				s.Desktops[0].Tabs,
				Tab{
					ID:   "duplicate",
					Root: LayoutNode{ID: "duplicate-leaf", Kind: NodeKindLeaf, WindowID: new(WindowID("w1"))},
				},
			)
		}},
		{
			"empty tab",
			"topology.leaf_window_required",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root = LayoutNode{ID: "empty", Kind: NodeKindLeaf} },
		},
		{
			"leaf children", "topology.leaf_shape",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Children[0].Children = []LayoutNode{{ID: "hidden"}} },
		},
		{
			"split window",
			"topology.split_shape",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.WindowID = new(WindowID("w1")) },
		},
		{"empty desktop", "topology.tab_required", func(s *Snapshot) { s.Desktops[0].Tabs = nil }},
		{"missing active", "topology.active_tab", func(s *Snapshot) { s.Desktops[0].ActiveTabID = "missing" }},
		{"pin prefix", "topology.tab_pinned_prefix", func(s *Snapshot) {
			d := &s.Desktops[0]
			root := d.Tabs[0].Root
			d.Tabs[0].Root = root.Children[0]
			d.Tabs = append(d.Tabs, Tab{ID: "pinned", Pinned: true, Root: root.Children[1]})
			for _, id := range []WindowID{"w2", "w3"} {
				w := s.Windows[id]
				w.TabID = "pinned"
				s.Windows[id] = w
			}
		}},
		{
			"duplicate node",
			"topology.node_cycle_or_duplicate",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Children[1].ID = "leaf-1" },
		},
		{
			"invalid weights",
			"topology.split_weight",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Weights[0] = math.NaN() },
		},
		{"missing membership", "topology.window_unknown", func(s *Snapshot) { delete(s.Windows, "w1") }},
		{
			"foreign tab",
			"topology.window_tab",
			func(s *Snapshot) { w := s.Windows["w1"]; w.TabID = "foreign"; s.Windows[w.ID] = w },
		},
	}
	for _, c := range cases {
		t.Run("Should reject "+c.name+" (UT-064)", func(t *testing.T) {
			t.Parallel()
			s := validThreeWindowSnapshot()
			requireValidSnapshot(t, s)
			c.mutate(&s)
			err := ValidateSnapshot(s)
			topology, ok := errors.AsType[*TopologyError](err)
			if !ok || !errors.Is(err, ErrInvalidTopology) ||
				!slices.ContainsFunc(topology.Diagnostics, func(d Diagnostic) bool { return d.Code == c.code }) {
				t.Fatalf("validation = %v, want %s", err, c.code)
			}
			if c.name == "pin prefix" && len(topology.Diagnostics) != 1 {
				t.Fatalf("confounded pin-prefix rejection = %+v", topology.Diagnostics)
			}
		})
	}
}

func TestNormalizeSnapshot(t *testing.T) {
	t.Run("Should preserve valid geometry and be idempotent", func(t *testing.T) {
		t.Parallel()
		s := validThreeWindowSnapshot()
		s.Desktops[0].Tabs[0].Root.Weights = []float64{.7, .3}
		n := NormalizeSnapshot(s)
		requireValidSnapshot(t, n)
		if !reflect.DeepEqual(n, NormalizeSnapshot(n)) ||
			!reflect.DeepEqual(s.Desktops[0].Tabs[0].Root.Weights, n.Desktops[0].Tabs[0].Root.Weights) {
			t.Fatal("normalization changed valid weights or is not idempotent")
		}
	})
}
