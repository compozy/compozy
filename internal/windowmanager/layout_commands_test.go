package windowmanager

// Suite: structural commands. Invariant: moves and seam operations preserve
// unique membership, destination pin state, and every unaffected split weight.
import (
	"errors"
	"math"
	"reflect"
	"slices"
	"testing"
)

func tabFor(t *testing.T, s Snapshot, id WindowID) Tab {
	t.Helper()
	d, i, ok := findTab(&s, s.Windows[id].TabID)
	if !ok {
		t.Fatalf("missing tab for %s", id)
	}
	return d.Tabs[i]
}
func splitTestWindows(t *testing.T, e testEnvironment, ids ...WindowID) Result {
	t.Helper()
	return executeTestCommand(
		t,
		e.manager,
		"workspace-a",
		nil,
		ArrangeLayoutCommand{DesktopID: "desktop-default", WindowIDs: ids, Arrangement: ArrangementHorizontal},
	)
}
func moveTestWindow(t *testing.T, e testEnvironment, id, target WindowID, p DropPlacement) Result {
	t.Helper()
	return executeTestCommand(
		t,
		e.manager,
		"workspace-a",
		nil,
		MoveWindowCommand{
			WindowID:             id,
			DestinationDesktopID: "desktop-default",
			TargetWindowID:       new(target),
			Placement:            p,
		},
	)
}
func threeTabs(t *testing.T) testEnvironment {
	t.Helper()
	e := newTestEnvironment(t, DefaultConfig(), "workspace-a")
	for _, id := range []WindowID{"a", "b", "c"} {
		openTestWindow(t, e.manager, "workspace-a", nil, id, "desktop-default")
	}
	executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "win-newtab-desktop-default"})
	return e
}

func TestStructuralDropPlacements(t *testing.T) {
	t.Run("Should place a solo tab after an explicit center target", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := moveTestWindow(t, e, "c", "a", DropCenter)
		d := r.Snapshot.Desktops[0]
		if !r.Applied || valueOrZero(d.Tabs[1].Root.WindowID) != "c" || valueOrZero(d.Tabs[2].Root.WindowID) != "b" {
			t.Fatalf("center placement = %+v", r)
		}
	})

	t.Run("Should split tabs (UT-017)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		s := mustSnapshot(t, e.manager, "workspace-a")
		target := s.Windows["a"].TabID
		r := moveTestWindow(t, e, "b", "a", DropRight)
		tab := tabFor(t, r.Snapshot, "a")
		if len(r.Snapshot.Desktops[0].Tabs) != 2 || !slices.Equal(nodeWindowIDs(tab.Root), []WindowID{"a", "b"}) ||
			!slices.Equal(r.Changes.TabSplit, []TabID{target}) {
			t.Fatalf("split=%+v", r)
		}
	})
	t.Run("Should extend and nest (UT-018)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		moveTestWindow(t, e, "b", "a", DropRight)
		r := moveTestWindow(t, e, "c", "b", DropRight)
		root := tabFor(t, r.Snapshot, "a").Root
		if len(root.Children) != 3 || !slices.Equal(root.Weights, equalWeights(3)) {
			t.Fatalf("extend=%+v", root)
		}
		openTestWindow(t, e.manager, "workspace-a", nil, "d", "desktop-default")
		r = moveTestWindow(t, e, "d", "b", DropBottom)
		root = tabFor(t, r.Snapshot, "a").Root
		if root.Children[1].Kind != NodeKindSplit || valueOrZero(root.Children[1].Axis) != AxisVertical {
			t.Fatalf("nest=%+v", root)
		}
	})
	t.Run("Should root edge (UT-019)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := moveTestWindow(t, e, "b", "a", DropBottom)
		tab := tabFor(t, r.Snapshot, "a")
		r = executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{
				WindowID:             "c",
				DestinationDesktopID: "desktop-default",
				TargetTabID:          new(tab.ID),
				Placement:            DropLeft,
			},
		)
		root := tabFor(t, r.Snapshot, "a").Root
		if valueOrZero(root.Axis) != AxisHorizontal || valueOrZero(root.Children[0].WindowID) != "c" ||
			root.Children[1].ID != tab.Root.ID {
			t.Fatalf("root edge=%+v", root)
		}
	})
	t.Run("Should destination pin and unsplit (UT-022)", func(t *testing.T) {
		t.Parallel()
		for _, pin := range []bool{false, true} {
			e := threeTabs(t)
			executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "b", Pinned: true})
			if pin {
				executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
			}
			r := moveTestWindow(t, e, "b", "a", DropRight)
			if tabFor(t, r.Snapshot, "a").Pinned != pin {
				t.Fatal("destination pin changed")
			}
			r = executeTestCommand(
				t,
				e.manager,
				"workspace-a",
				nil,
				MoveWindowCommand{WindowID: "b", DestinationDesktopID: "desktop-default", Placement: DropTab},
			)
			if tabFor(t, r.Snapshot, "b").Pinned {
				t.Fatal("unsplit tab pinned")
			}
		}
	})
	t.Run("Should unsplit and solo noop (UT-026)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := moveTestWindow(t, e, "b", "a", DropRight)
		tab := tabFor(t, r.Snapshot, "a")
		r = executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{WindowID: "b", DestinationDesktopID: "desktop-default", Placement: DropTab},
		)
		d := r.Snapshot.Desktops[0]
		i := tabHoldingID(d.Tabs, tab.ID)
		if valueOrZero(d.Tabs[i+1].Root.WindowID) != "b" || !slices.Equal(r.Changes.TabUnsplit, []TabID{tab.ID}) {
			t.Fatalf("unsplit=%+v", r)
		}
		r = executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{WindowID: "b", DestinationDesktopID: "desktop-default", Placement: DropTab},
		)
		if r.Applied {
			t.Fatal("solo move applied")
		}
	})
}

func TestSwapWindows(t *testing.T) {
	t.Run("Should leaves retain weights (UT-023)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := splitTestWindows(t, e, "a", "b", "c")
		seeded := r.Snapshot
		d, ti, _ := findTab(&seeded, seeded.Windows["a"].TabID)
		d.Tabs[ti].Root.Weights = []float64{.2, .3, .5}
		seedTestSnapshot(t, e, seeded)
		root := tabFor(t, seeded, "a").Root
		r = executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			SwapWindowsCommand{FirstWindowID: "a", SecondWindowID: "c"},
		)
		got := tabFor(t, r.Snapshot, "a").Root
		if !slices.Equal(nodeWindowIDs(got), []WindowID{"c", "b", "a"}) || !slices.Equal(got.Weights, root.Weights) {
			t.Fatalf("swap=%+v", got)
		}
	})
}

func TestBalanceAndResize(t *testing.T) {
	t.Run("Should floor and tiny delta (UT-024)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := splitTestWindows(t, e, "a", "b")
		root := tabFor(t, r.Snapshot, "a").Root
		for _, delta := range []float64{.5, -.5, math.Inf(1)} {
			_, err := e.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: r.Snapshot.Revision,
					Payload:          ResizeLayoutCommand{SplitID: root.ID, Delta: delta},
				},
			)
			if !errors.Is(err, ErrInvalidCommand) {
				t.Fatalf("delta=%g error=%v", delta, err)
			}
		}
		for _, delta := range []float64{0, 1e-6, -1e-6} {
			x := executeTestCommand(
				t,
				e.manager,
				"workspace-a",
				nil,
				ResizeLayoutCommand{SplitID: root.ID, Delta: delta},
			)
			if x.Applied {
				t.Fatalf("tiny delta=%g applied", delta)
			}
		}
		for _, delta := range []float64{.490001, -.490001} {
			before := mustSnapshot(t, e.manager, "workspace-a")
			_, err := e.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: before.Revision,
					Payload:          ResizeLayoutCommand{SplitID: root.ID, Delta: delta},
				},
			)
			if !errors.Is(err, ErrInvalidCommand) ||
				!reflect.DeepEqual(before, mustSnapshot(t, e.manager, "workspace-a")) {
				t.Fatalf("below floor delta=%g error=%v", delta, err)
			}
		}
		for _, delta := range []float64{.49, -.49} {
			before := r.Snapshot
			seedTestSnapshot(t, e, before)
			result := executeTestCommand(
				t,
				e.manager,
				"workspace-a",
				nil,
				ResizeLayoutCommand{SplitID: root.ID, Delta: delta},
			)
			weights := tabFor(t, result.Snapshot, "a").Root.Weights
			if math.Abs(min(weights[0], weights[1])-.01) > weightTolerance {
				t.Fatalf("floor weights=%v", weights)
			}
		}
	})
	t.Run("Should balance scope (UT-025)", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []string{"split", "tab", "group"} {
			e := threeTabs(t)
			splitTestWindows(t, e, "a", "b", "c")
			openTestWindow(t, e.manager, "workspace-a", nil, "d", "desktop-default")
			moveTestWindow(t, e, "d", "c", DropBottom)
			seeded := mustSnapshot(t, e.manager, "workspace-a")
			desktop, ti, _ := findTab(&seeded, seeded.Windows["a"].TabID)
			root := &desktop.Tabs[ti].Root
			root.Weights = []float64{.2, .3, .5}
			root.Children[2].Weights = []float64{.3, .7}
			seedTestSnapshot(t, e, seeded)
			command := BalanceLayoutCommand{SplitID: new(root.ID)}
			if kind == "tab" {
				command = BalanceLayoutCommand{TabID: new(desktop.Tabs[ti].ID)}
			}
			if kind == "group" {
				command = BalanceLayoutCommand{GroupID: new(GroupID(desktop.Tabs[ti].ID))}
			}
			r := executeTestCommand(t, e.manager, "workspace-a", nil, command)
			got := tabFor(t, r.Snapshot, "a").Root
			nested := []float64{.3, .7}
			if kind != "split" {
				nested = []float64{.5, .5}
			}
			if !slices.Equal(got.Weights, equalWeights(3)) || !slices.Equal(got.Children[2].Weights, nested) {
				t.Fatalf("%s balanced root=%+v", kind, got)
			}
		}
	})
}
func TestArrangementModes(t *testing.T) {
	t.Run("Should preserve a nonzero anchor relative to surviving tabs (UT-029)", func(t *testing.T) {
		t.Parallel()
		for _, ids := range [][]WindowID{{"c", "a", "e"}, {"c", "e", "a"}, {"c", "d", "e"}} {
			e := threeTabs(t)
			for _, id := range []WindowID{"d", "e", "f"} {
				openTestWindow(t, e.manager, "workspace-a", nil, id, "desktop-default")
			}
			r := executeTestCommand(
				t,
				e.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{DesktopID: "desktop-default", WindowIDs: ids, Arrangement: ArrangementGrid},
			)
			var got []WindowID
			for _, tab := range r.Snapshot.Desktops[0].Tabs {
				got = append(got, nodeWindowIDs(tab.Root)[0])
			}
			want := []WindowID{"b", "c", "d", "f"}
			if ids[1] == "d" {
				want = []WindowID{"a", "b", "c", "f"}
			}
			if !slices.Equal(got, want) || !slices.Equal(nodeWindowIDs(tabFor(t, r.Snapshot, "c").Root), ids) {
				t.Fatalf("arrange %v order=%v want=%v", ids, got, want)
			}
		}
	})
	t.Run("Should reject an invalid one-window arrangement atomically", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		before := mustSnapshot(t, e.manager, "workspace-a")
		_, err := e.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: before.Revision,
				Payload: ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"a"},
					Arrangement: Arrangement("invalid"),
				},
			},
		)
		if !errors.Is(err, ErrInvalidCommand) || !reflect.DeepEqual(before, mustSnapshot(t, e.manager, "workspace-a")) {
			t.Fatalf("invalid arrangement=%v", err)
		}
	})

	t.Run("Should grid gathers three tabs (UT-029)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		s := mustSnapshot(t, e.manager, "workspace-a")
		index := tabHoldingID(s.Desktops[0].Tabs, s.Windows["a"].TabID)
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"a", "b", "c"},
				Arrangement: ArrangementGrid,
			},
		)
		root := r.Snapshot.Desktops[0].Tabs[0].Root
		if len(r.Snapshot.Desktops[0].Tabs) != 1 || index != 0 || root.Kind != NodeKindSplit ||
			valueOrZero(root.Axis) != AxisVertical ||
			valueOrZero(root.Children[0].Axis) != AxisHorizontal ||
			!slices.Equal(root.Weights, []float64{.5, .5}) ||
			!reflect.DeepEqual(nodeWindowIDs(r.Snapshot.Desktops[0].Tabs[0].Root), []WindowID{"a", "b", "c"}) {
			t.Fatalf("grid=%+v", r)
		}
	})
}

func TestTabTransitionChanges(t *testing.T) {
	// Invariant: split transitions describe before/after member counts, not temporary reducer states.
	t.Run("Should not split or unsplit when rearranging within a two-member tab", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b")
		r := moveTestWindow(t, e, "b", "a", DropBottom)
		if len(r.Changes.TabSplit)+len(r.Changes.TabUnsplit) != 0 {
			t.Fatalf("changes=%+v", r.Changes)
		}
	})
	t.Run("Should not split when reopening into an existing multi-member tab", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b", "c")
		executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b"})
		r := executeTestCommand(t, e.manager, "workspace-a", nil, ReopenCommand{})
		if len(r.Changes.TabSplit)+len(r.Changes.TabUnsplit) != 0 {
			t.Fatalf("changes=%+v", r.Changes)
		}
	})
	t.Run("Should unsplit sources once when arrange extracts members", func(t *testing.T) {
		t.Parallel()
		for _, c := range []struct {
			command Command
			split   bool
		}{
			{ArrangeLayoutCommand{DesktopID: "desktop-default", WindowIDs: []WindowID{"b", "c"}, Arrangement: ArrangementHorizontal}, true},
			{ArrangeLayoutCommand{DesktopID: "desktop-default", WindowIDs: []WindowID{"b", "c"}, Arrangement: ArrangementStack}, false},
			{GroupWindowsCommand{TargetWindowID: "c", WindowIDs: []WindowID{"b"}}, false},
		} {
			e := threeTabs(t)
			splitTestWindows(t, e, "a", "b")
			source := mustSnapshot(t, e.manager, "workspace-a").Windows["a"].TabID
			r := executeTestCommand(t, e.manager, "workspace-a", nil, c.command)
			if !slices.Equal(r.Changes.TabUnsplit, []TabID{source}) {
				t.Fatalf("%s changes=%+v", c.command.CommandID(), r.Changes)
			}
			var split []TabID
			if c.split {
				split = []TabID{r.Snapshot.Windows["b"].TabID}
			}
			if !slices.Equal(r.Changes.TabSplit, split) {
				t.Fatalf("%s changes=%+v", c.command.CommandID(), r.Changes)
			}
		}
	})
}
