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
		root := tabFor(t, r.Snapshot, "a").Root
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
	})
	t.Run("Should balance scope (UT-025)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		r := splitTestWindows(t, e, "a", "b", "c")
		root := tabFor(t, r.Snapshot, "a").Root
		r = executeTestCommand(t, e.manager, "workspace-a", nil, ResizeLayoutCommand{SplitID: root.ID, Delta: .1})
		for _, kind := range []string{"split", "tab", "group"} {
			var command BalanceLayoutCommand
			switch kind {
			case "split":
				command.SplitID = new(root.ID)
			case "tab":
				command.TabID = new(tabFor(t, r.Snapshot, "a").ID)
			case "group":
				command.GroupID = new(GroupID(tabFor(t, r.Snapshot, "a").ID))
			}
			r = executeTestCommand(t, e.manager, "workspace-a", nil, command)
			if !slices.Equal(tabFor(t, r.Snapshot, "a").Root.Weights, equalWeights(3)) {
				t.Fatalf("%s balance=%+v", kind, r)
			}
			r = executeTestCommand(t, e.manager, "workspace-a", nil, ResizeLayoutCommand{SplitID: root.ID, Delta: .1})
		}
	})
}
func TestArrangementModes(t *testing.T) {
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
		if len(r.Snapshot.Desktops[0].Tabs) != 1 || index != 0 ||
			!reflect.DeepEqual(nodeWindowIDs(r.Snapshot.Desktops[0].Tabs[0].Root), []WindowID{"a", "b", "c"}) {
			t.Fatalf("grid=%+v", r)
		}
	})
}
