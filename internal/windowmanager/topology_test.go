package windowmanager

// Suite: topology validation. Invariant: every window has one leaf in one tab,
// desktops retain content, and normalization preserves valid split geometry.
import (
	"errors"
	"math"
	"reflect"
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
		mutate func(*Snapshot)
	}{
		{"duplicate window", func(s *Snapshot) {
			s.Desktops[0].Tabs = append(
				s.Desktops[0].Tabs,
				Tab{
					ID:   "duplicate",
					Root: LayoutNode{ID: "duplicate-leaf", Kind: NodeKindLeaf, WindowID: new(WindowID("w1"))},
				},
			)
		}},
		{"empty tab", func(s *Snapshot) { s.Desktops[0].Tabs[0].Root = LayoutNode{ID: "empty", Kind: NodeKindLeaf} }},
		{
			"leaf children",
			func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Children[0].Children = []LayoutNode{{ID: "hidden"}} },
		},
		{"split window", func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.WindowID = new(WindowID("w1")) }},
		{"empty desktop", func(s *Snapshot) { s.Desktops[0].Tabs = nil }},
		{"missing active", func(s *Snapshot) { s.Desktops[0].ActiveTabID = "missing" }},
		{"pin prefix", func(s *Snapshot) {
			s.Desktops[0].Tabs = append(
				s.Desktops[0].Tabs,
				Tab{
					ID:     "pinned",
					Pinned: true,
					Root:   LayoutNode{ID: "pin-leaf", Kind: NodeKindLeaf, WindowID: new(WindowID("w1"))},
				},
			)
		}},
		{"duplicate node", func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Children[1].ID = "leaf-1" }},
		{"invalid weights", func(s *Snapshot) { s.Desktops[0].Tabs[0].Root.Weights[0] = math.NaN() }},
		{"missing membership", func(s *Snapshot) { delete(s.Windows, "w1") }},
		{"foreign tab", func(s *Snapshot) { w := s.Windows["w1"]; w.TabID = "foreign"; s.Windows[w.ID] = w }},
	}
	for _, c := range cases {
		t.Run("Should reject "+c.name+" (UT-064)", func(t *testing.T) {
			t.Parallel()
			s := validThreeWindowSnapshot()
			c.mutate(&s)
			if err := ValidateSnapshot(s); !errors.Is(err, ErrInvalidTopology) {
				t.Fatalf("validation = %v", err)
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
