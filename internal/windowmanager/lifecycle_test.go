package windowmanager

// Suite: window-manager lifecycle reducers
// Invariant: desktop and window lifecycle commands preserve every unaffected member and restore deterministic structure.
// Boundary IN: desktop/window/layout reducers plus client-view coordinator.
// Boundary OUT: pointer gesture interpretation and transport payload parsing.

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

func TestWindowLifecycleReflow(t *testing.T) {
	t.Run(
		"Should minimize, restore through an anchor, and close without displacing another window",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
			arranged := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"w1", "w2"},
					Arrangement: ArrangementHorizontal,
					Frame:       fullRect(),
					GroupID:     "group-main",
				},
			)
			if len(arranged.Snapshot.Desktops[0].Groups) != 1 {
				t.Fatalf(
					"arranged groups=%+v windows=%+v",
					arranged.Snapshot.Desktops[0].Groups,
					arranged.Snapshot.Windows,
				)
			}
			minimized := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CloseWindowCommand{WindowID: "w1", Minimize: true},
			)
			if !minimized.Snapshot.Windows["w1"].Minimized || minimized.Snapshot.Windows["w1"].ReturnAnchor == nil {
				t.Fatalf("minimized window=%+v", minimized.Snapshot.Windows["w1"])
			}
			if len(minimized.Snapshot.Desktops[0].Groups) != 1 {
				t.Fatalf(
					"reflow groups=%+v windows=%+v",
					minimized.Snapshot.Desktops[0].Groups,
					minimized.Snapshot.Windows,
				)
			}
			if root := minimized.Snapshot.Desktops[0].Groups[0].Root; root.Kind != NodeKindLeaf ||
				valueOrZero(root.WindowID) != "w2" {
				t.Fatalf("reflow root=%+v", root)
			}
			restored := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				OpenWindowCommand{RestoreWindowID: new(WindowID("w1"))},
			)
			if restored.Snapshot.Windows["w1"].Minimized ||
				restored.Snapshot.Windows["w1"].Placement != WindowPlacementTiled {
				t.Fatalf("restored window=%+v", restored.Snapshot.Windows["w1"])
			}
			closed := executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w1"})
			if _, exists := closed.Snapshot.Windows["w1"]; exists {
				t.Fatal("closed window still exists")
			}
			if _, exists := closed.Snapshot.Windows["w2"]; !exists {
				t.Fatal("unaffected window was removed")
			}
			requireValidSnapshot(t, closed.Snapshot)
		},
	)
	t.Run("Should reject minimizing a window whose desktop is missing", func(t *testing.T) {
		t.Parallel()
		snapshot := validThreeWindowSnapshot()
		window := snapshot.Windows["w1"]
		window.DesktopID = "desktop-missing"
		snapshot.Windows["w1"] = window
		reducer := reducer{config: DefaultConfig()}

		_, err := reducer.minimizeWindow(&snapshot, "w1")
		if !errors.Is(err, ErrInvalidTopology) {
			t.Fatalf("minimizeWindow() error = %v, want ErrInvalidTopology", err)
		}
	})
}

func TestWindowTabGroupingV3(t *testing.T) {
	t.Run("Should group floating windows and activate the newly added member [UT-010]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		stack := result.Snapshot.Desktops[0].FloatingStacks[0]
		if !slices.Equal(stack.WindowIDs, []WindowID{"w1", "w2"}) || valueOrZero(stack.ActiveID) != "w2" {
			t.Fatalf("floating stack = %+v", stack)
		}
		requireValidSnapshot(t, result.Snapshot)
	})

	t.Run("Should move a floating tab frame as one unit with move_group", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		rect := NormalizedRect{X: 0.4, Y: 0.3, Width: 0.5, Height: 0.6}
		moved := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w2", DestinationDesktopID: "desktop-default",
			Placement: DropFloating, FloatingRect: &rect, MoveGroup: true,
		})
		stack := moved.Snapshot.Desktops[0].FloatingStacks[0]
		if stack.Rect != rect || !slices.Equal(stack.WindowIDs, []WindowID{"w1", "w2"}) {
			t.Fatalf("moved frame = %+v", stack)
		}
		if moved.Snapshot.Windows["w1"].Placement != WindowPlacementStacked {
			t.Fatalf("member left the frame: %+v", moved.Snapshot.Windows["w1"])
		}
		repeat, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: moved.Snapshot.Revision,
			Payload: MoveWindowCommand{
				WindowID: "w2", DestinationDesktopID: "desktop-default",
				Placement: DropFloating, FloatingRect: &rect, MoveGroup: true,
			},
		})
		if err != nil || repeat.Applied {
			t.Fatalf("same-rect frame move applied = %v, err = %v", repeat.Applied, err)
		}
		created := executeTestCommand(t, environment.manager, "workspace-a", nil, CreateDesktopCommand{
			DesktopID: "desktop-b", Name: "B",
		})
		crossed := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w1", DestinationDesktopID: "desktop-b", MoveGroup: true,
		})
		destinationIndex, exists := desktopIndexByID(&crossed.Snapshot, "desktop-b")
		if !exists || len(crossed.Snapshot.Desktops[destinationIndex].FloatingStacks) != 1 {
			t.Fatalf("cross-desktop frame move = %+v", crossed.Snapshot.Desktops)
		}
		for _, windowID := range []WindowID{"w1", "w2"} {
			if crossed.Snapshot.Windows[windowID].DesktopID != "desktop-b" {
				t.Fatalf("member %q desktop = %+v", windowID, crossed.Snapshot.Windows[windowID])
			}
		}
		_ = created
		requireValidSnapshot(t, crossed.Snapshot)
	})

	t.Run("Should splice a floating tab frame beside a tiled target as one stack node", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "board"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID:   "desktop-default",
			WindowIDs:   []WindowID{"board"},
			Arrangement: ArrangementHorizontal,
			Frame:       fullRect(),
			GroupID:     "group-board",
		})
		moved := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w2", DestinationDesktopID: "desktop-default",
			TargetWindowID: new(WindowID("board")), Placement: DropRight, MoveGroup: true,
		})
		desktop := moved.Snapshot.Desktops[0]
		if len(desktop.FloatingStacks) != 0 {
			t.Fatalf("floating stack survived the structural move: %+v", desktop.FloatingStacks)
		}
		root := desktop.Groups[0].Root
		if root.Kind != NodeKindSplit || len(root.Children) != 2 {
			t.Fatalf("split root = %+v", root)
		}
		stackNode := root.Children[1]
		if stackNode.Kind != NodeKindStack || !slices.Equal(stackNode.WindowIDs, []WindowID{"w1", "w2"}) {
			t.Fatalf("stack node = %+v", stackNode)
		}
		for _, windowID := range []WindowID{"w1", "w2"} {
			if moved.Snapshot.Windows[windowID].Placement != WindowPlacementStacked {
				t.Fatalf("member %q placement = %+v", windowID, moved.Snapshot.Windows[windowID])
			}
		}
		requireValidSnapshot(t, moved.Snapshot)
	})

	t.Run("Should float a tiled tab frame as one unit when a group move carries a rect", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "board"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID:   "desktop-default",
			WindowIDs:   []WindowID{"board"},
			Arrangement: ArrangementHorizontal,
			Frame:       fullRect(),
			GroupID:     "group-board",
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w2", DestinationDesktopID: "desktop-default",
			TargetWindowID: new(WindowID("board")), Placement: DropRight, MoveGroup: true,
		})
		rect := NormalizedRect{X: 0.2, Y: 0.25, Width: 0.5, Height: 0.5}
		floated := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w2", DestinationDesktopID: "desktop-default",
			Placement: DropFloating, FloatingRect: &rect, MoveGroup: true,
		})
		desktop := floated.Snapshot.Desktops[0]
		if len(desktop.FloatingStacks) != 1 {
			t.Fatalf("floating stacks = %+v", desktop.FloatingStacks)
		}
		stack := desktop.FloatingStacks[0]
		if stack.Rect != rect || !slices.Equal(stack.WindowIDs, []WindowID{"w1", "w2"}) ||
			valueOrZero(stack.ActiveID) != "w2" {
			t.Fatalf("floated frame = %+v", stack)
		}
		for _, windowID := range []WindowID{"w1", "w2"} {
			if floated.Snapshot.Windows[windowID].Placement != WindowPlacementStacked {
				t.Fatalf("member %q placement = %+v", windowID, floated.Snapshot.Windows[windowID])
			}
		}
		for _, group := range desktop.Groups {
			for _, windowID := range nodeWindowIDs(group.Root) {
				if windowID == "w1" || windowID == "w2" {
					t.Fatalf("member %q survived in the tiled tree: %+v", windowID, group.Root)
				}
			}
		}
		if !slices.Contains(floated.Changes.GroupIDs, GroupID("group-board")) {
			t.Fatalf("float changes = %+v, want source group %q", floated.Changes, "group-board")
		}
		requireValidSnapshot(t, floated.Snapshot)
	})

	t.Run("Should fold a center group drop into the target stack", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "board"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID:   "desktop-default",
			WindowIDs:   []WindowID{"board"},
			Arrangement: ArrangementHorizontal,
			Frame:       fullRect(),
			GroupID:     "group-board",
		})
		folded := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID: "w2", DestinationDesktopID: "desktop-default",
			TargetWindowID: new(WindowID("board")), Placement: DropCenter, MoveGroup: true,
		})
		root := folded.Snapshot.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindStack || !slices.Equal(root.WindowIDs, []WindowID{"board", "w1", "w2"}) {
			t.Fatalf("folded stack = %+v", root)
		}
		if len(folded.Snapshot.Desktops[0].FloatingStacks) != 0 {
			t.Fatalf("floating stack survived the fold: %+v", folded.Snapshot.Desktops[0].FloatingStacks)
		}
		requireValidSnapshot(t, folded.Snapshot)
	})

	t.Run("Should reject a structural group move onto a floating target", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "float"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		before, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		_, err = environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: before.Revision,
			Payload: MoveWindowCommand{
				WindowID: "w2", DestinationDesktopID: "desktop-default",
				TargetWindowID: new(WindowID("float")), Placement: DropRight, MoveGroup: true,
			},
		})
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("floating-target group move error = %v, want ErrInvalidCommand", err)
		}
	})

	t.Run("Should keep rejecting placement fields on tiled group moves", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID:   "desktop-default",
			WindowIDs:   []WindowID{"w1", "w2"},
			Arrangement: ArrangementHorizontal,
			Frame:       fullRect(),
			GroupID:     "group",
		})
		before, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		rect := NormalizedRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5}
		_, err = environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: before.Revision,
			Payload: MoveWindowCommand{
				WindowID: "w1", DestinationDesktopID: "desktop-default",
				FloatingRect: &rect, MoveGroup: true,
			},
		})
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("tiled group move with rect error = %v, want ErrInvalidCommand", err)
		}
	})

	t.Run("Should reject empty duplicate self and missing group members atomically [UT-011]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		cases := []struct {
			command GroupWindowsCommand
			want    error
		}{
			{command: GroupWindowsCommand{TargetWindowID: "w1"}, want: ErrInvalidCommand},
			{
				command: GroupWindowsCommand{TargetWindowID: "w1", WindowIDs: []WindowID{"w1"}},
				want:    ErrInvalidCommand,
			},
			{
				command: GroupWindowsCommand{TargetWindowID: "w1", WindowIDs: []WindowID{"w2", "w2"}},
				want:    ErrInvalidCommand,
			},
			{
				command: GroupWindowsCommand{TargetWindowID: "missing", WindowIDs: []WindowID{"w2"}},
				want:    ErrWindowNotFound,
			},
			{
				command: GroupWindowsCommand{TargetWindowID: "w1", WindowIDs: []WindowID{"missing"}},
				want:    ErrWindowNotFound,
			},
		}
		before, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		for _, testCase := range cases {
			_, err := environment.manager.Execute(t.Context(), CommandRequest{
				WorkspaceID: "workspace-a", ExpectedRevision: before.Revision, Payload: testCase.command,
			})
			if !errors.Is(err, testCase.want) {
				t.Fatalf("Execute(%+v) error = %v, want identity %v", testCase.command, err, testCase.want)
			}
		}
		after, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot(after errors) error = %v", err)
		}
		if after.Revision != before.Revision {
			t.Fatalf("revision after rejected groups = %d, want %d", after.Revision, before.Revision)
		}
	})

	t.Run("Should grow a tiled leaf stack and restore a minimized member [UT-012]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, ToggleFloatingCommand{WindowID: "w1"})
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "w2", Minimize: true},
		)
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		root := result.Snapshot.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindStack || !slices.Equal(root.WindowIDs, []WindowID{"w1", "w2"}) ||
			result.Snapshot.Windows["w2"].Minimized {
			t.Fatalf("grouped root = %+v, w2 = %+v", root, result.Snapshot.Windows["w2"])
		}
	})

	t.Run(
		"Should fold all requested source-stack members into the target in payload order [UT-013]",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			for _, windowID := range []WindowID{"w1", "w2", "w3"} {
				openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			}
			source := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				GroupWindowsCommand{TargetWindowID: "w2", WindowIDs: []WindowID{"w3"}},
			)
			sourceLocation, found := findStackByWindow(&source.Snapshot, "w2")
			if !found {
				t.Fatal("source stack not found")
			}
			result := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				GroupWindowsCommand{TargetWindowID: "w1", WindowIDs: []WindowID{"w2", "w3"}},
			)
			members := stackMembersForWindow(t, result.Snapshot, "w1")
			if !slices.Equal(members, []WindowID{"w1", "w2", "w3"}) {
				t.Fatalf("grouped members = %v", members)
			}
			if !slices.Contains(result.Changes.StackUngrouped, sourceLocation.id()) {
				t.Fatalf("group changes = %+v, want source stack %q ungrouped", result.Changes, sourceLocation.id())
			}
		},
	)

	t.Run("Should migrate grouped members to the targets desktop [UT-014]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "target", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "member", "d2")
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			GroupWindowsCommand{TargetWindowID: "target", WindowIDs: []WindowID{"member"}},
		)
		if result.Snapshot.Windows["member"].DesktopID != "desktop-default" ||
			len(result.Snapshot.Desktops[1].Floating) != 0 {
			t.Fatalf("cross-desktop result = %+v", result.Snapshot)
		}
	})

	t.Run("Should reorder DropCenter within one stack without duplicate membership [UT-015]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		created := createFloatingStack(t, environment.manager, []WindowID{"w1", "w2"})
		before, found := findStackByWindow(&created.Snapshot, "w1")
		if !found {
			t.Fatal("stack before DropCenter not found")
		}
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, MoveWindowCommand{
			WindowID:             "w1",
			DestinationDesktopID: "desktop-default",
			TargetWindowID:       new(WindowID("w2")),
			Placement:            DropCenter,
		})
		members := stackMembersForWindow(t, result.Snapshot, "w1")
		if !slices.Equal(members, []WindowID{"w2", "w1"}) {
			t.Fatalf("DropCenter members = %v", members)
		}
		after, found := findStackByWindow(&result.Snapshot, "w1")
		if !found {
			t.Fatal("stack after DropCenter not found")
		}
		if after.id() != before.id() {
			t.Fatalf("DropCenter stack ID = %q, want %q", after.id(), before.id())
		}
		requireValidSnapshot(t, result.Snapshot)
	})

	t.Run("Should merge pinned and unpinned regions with stable relative order [UT-016]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"p1", "u1", "u2", "p2"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "p1", Pinned: true})
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "p2", Pinned: true})
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			GroupWindowsCommand{TargetWindowID: "p1", WindowIDs: []WindowID{"u1"}},
		)
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			GroupWindowsCommand{TargetWindowID: "p1", WindowIDs: []WindowID{"u2", "p2"}},
		)
		members := stackMembersForWindow(t, result.Snapshot, "p1")
		if !slices.Equal(members, []WindowID{"p1", "p2", "u1", "u2"}) {
			t.Fatalf("merged members = %v", members)
		}
	})

	t.Run("Should place an unpinned member at the start of the unpinned region [UT-017]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"w1", "w2", "w3"})
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "w2", Pinned: true})
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			PinWindowCommand{WindowID: "w2", Pinned: false},
		)
		if members := stackMembersForWindow(
			t,
			result.Snapshot,
			"w2",
		); !slices.Equal(
			members,
			[]WindowID{"w2", "w1", "w3"},
		) {
			t.Fatalf("members after unpin = %v", members)
		}
	})

	t.Run("Should ungroup a pinned member without clearing the pin [UT-018]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"w1", "w2"})
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "w1", Pinned: true})
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, ToggleFloatingCommand{WindowID: "w1"})
		if !result.Snapshot.Windows["w1"].Pinned || result.Snapshot.Windows["w1"].Placement != WindowPlacementFloating {
			t.Fatalf("ungrouped pinned window = %+v", result.Snapshot.Windows["w1"])
		}
	})

	t.Run("Should clamp reorder within the pin region and reject non-stacked windows [UT-019]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"p1", "p2", "u1", "u2"})
		for _, windowID := range []WindowID{"p1", "p2"} {
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				PinWindowCommand{WindowID: windowID, Pinned: true},
			)
		}
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ReorderStackCommand{WindowID: "u2", Index: 0},
		)
		if members := stackMembersForWindow(
			t,
			result.Snapshot,
			"u2",
		); !slices.Equal(
			members,
			[]WindowID{"p1", "p2", "u2", "u1"},
		) {
			t.Fatalf("clamped members = %v", members)
		}
		openTestWindow(t, environment.manager, "workspace-a", nil, "solo", "desktop-default")
		snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: snapshot.Revision,
				Payload:          ReorderStackCommand{WindowID: "solo", Index: 0},
			},
		)
		if !errors.Is(err, ErrNotStacked) {
			t.Fatalf("ReorderStack(solo) error = %v, want ErrNotStacked", err)
		}
	})

	t.Run("Should splice and clamp group InsertIndex within legal pin regions [UT-179]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"p1", "u1"})
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "p1", Pinned: true})
		for _, windowID := range []WindowID{"p2", "u2", "u3"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, PinWindowCommand{WindowID: "p2", Pinned: true})
		zero := 0
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			GroupWindowsCommand{TargetWindowID: "p1", WindowIDs: []WindowID{"u2", "p2"}, InsertIndex: &zero},
		)
		if members := stackMembersForWindow(
			t,
			result.Snapshot,
			"p1",
		); !slices.Equal(
			members,
			[]WindowID{"p2", "p1", "u2", "u1"},
		) {
			t.Fatalf("indexed members = %v", members)
		}
		result = executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			GroupWindowsCommand{TargetWindowID: "p1", WindowIDs: []WindowID{"u3"}},
		)
		if members := stackMembersForWindow(
			t,
			result.Snapshot,
			"p1",
		); !slices.Equal(
			members,
			[]WindowID{"p2", "p1", "u2", "u1", "u3"},
		) {
			t.Fatalf("nil-index members = %v", members)
		}
	})
}

func createFloatingStack(t *testing.T, manager *Manager, windowIDs []WindowID) Result {
	t.Helper()
	for _, windowID := range windowIDs {
		openTestWindow(t, manager, "workspace-a", nil, windowID, "desktop-default")
	}
	return executeTestCommand(t, manager, "workspace-a", nil, GroupWindowsCommand{
		TargetWindowID: windowIDs[0], WindowIDs: append([]WindowID(nil), windowIDs[1:]...),
	})
}

func stackMembersForWindow(t *testing.T, snapshot Snapshot, windowID WindowID) []WindowID {
	t.Helper()
	location, found := findStackByWindow(&snapshot, windowID)
	if !found {
		t.Fatalf("window %q is not stacked in %+v", windowID, snapshot)
	}
	return append([]WindowID(nil), location.members()...)
}

func TestWindowTabCloseAndReopenV3(t *testing.T) {
	t.Run("Should close one tab into a complete entry and activate the right neighbor [UT-020]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"w1", "w2", "w3"})
		executeTestCommand(t, environment.manager, "workspace-a", nil, SetStackActiveCommand{WindowID: "w2"})
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w2"})
		if len(result.Snapshot.ClosedEntries) != 1 {
			t.Fatalf("closed entries = %+v", result.Snapshot.ClosedEntries)
		}
		entry := result.Snapshot.ClosedEntries[0]
		if !slices.Equal(closedEntryWindowIDs(entry), []WindowID{"w2"}) || entry.StackID == nil ||
			valueOrZero(entry.ActiveID) != "w2" {
			t.Fatalf("closed entry = %+v", entry)
		}
		location, found := findStackByWindow(&result.Snapshot, "w1")
		if !found || valueOrZero(location.activeID()) != "w3" {
			t.Fatalf("surviving stack = %+v, found = %v", location, found)
		}
	})

	t.Run("Should close a whole frame as one ordered entry [UT-021]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		stacked := createFloatingStack(t, environment.manager, []WindowID{"w1", "w2", "w3"})
		stack := stacked.Snapshot.Desktops[0].FloatingStacks[0]
		executeTestCommand(t, environment.manager, "workspace-a", nil, SetStackActiveCommand{WindowID: "w2"})
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "w1", Scope: CloseScopeGroup},
		)
		entry := result.Snapshot.ClosedEntries[0]
		if !slices.Equal(closedEntryWindowIDs(entry), []WindowID{"w1", "w2", "w3"}) ||
			valueOrZero(entry.ActiveID) != "w2" || valueOrZero(entry.StackID) != stack.ID || entry.Rect != stack.Rect {
			t.Fatalf("group closed entry = %+v", entry)
		}
		if len(result.Snapshot.Windows) != 0 || result.Snapshot.Revision != stacked.Snapshot.Revision+2 {
			t.Fatalf("group close result = %+v", result)
		}
	})

	t.Run("Should leave a valid empty desktop after closing the last window [UT-022]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "solo", "desktop-default")
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "solo"})
		if len(result.Snapshot.Windows) != 0 || len(result.Snapshot.Desktops[0].Floating) != 0 {
			t.Fatalf("empty desktop result = %+v", result.Snapshot)
		}
		requireValidSnapshot(t, result.Snapshot)
	})

	t.Run(
		"Should protect a pinned tab but allow scoped frame close and reject scoped minimize [UT-023]",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			createFloatingStack(t, environment.manager, []WindowID{"pinned", "other"})
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				PinWindowCommand{WindowID: "pinned", Pinned: true},
			)
			snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}
			for _, testCase := range []struct {
				command CloseWindowCommand
				want    error
			}{
				{command: CloseWindowCommand{WindowID: "pinned"}, want: ErrWindowPinned},
				{
					command: CloseWindowCommand{WindowID: "pinned", Minimize: true, Scope: CloseScopeGroup},
					want:    ErrInvalidCommand,
				},
			} {
				_, executeErr := environment.manager.Execute(
					t.Context(),
					CommandRequest{
						WorkspaceID:      "workspace-a",
						ExpectedRevision: snapshot.Revision,
						Payload:          testCase.command,
					},
				)
				if !errors.Is(executeErr, testCase.want) {
					t.Fatalf(
						"Execute(%+v) error = %v, want identity %v",
						testCase.command,
						executeErr,
						testCase.want,
					)
				}
			}
			result := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CloseWindowCommand{WindowID: "pinned", Scope: CloseScopeGroup},
			)
			if len(result.Snapshot.Windows) != 0 || len(result.Snapshot.ClosedEntries) != 1 {
				t.Fatalf("pinned group close = %+v", result.Snapshot)
			}
		},
	)

	t.Run("Should open directly into floating and tiled target stacks [UT-024]", func(t *testing.T) {
		for _, tiled := range []bool{false, true} {
			t.Run(fmt.Sprintf("Should join a tiled target equal %v", tiled), func(t *testing.T) {
				t.Parallel()
				environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
				openTestWindow(t, environment.manager, "workspace-a", nil, "target", "desktop-default")
				if tiled {
					executeTestCommand(
						t,
						environment.manager,
						"workspace-a",
						nil,
						ToggleFloatingCommand{WindowID: "target"},
					)
				}
				targetID := WindowID("target")
				result := executeTestCommand(
					t,
					environment.manager,
					"workspace-a",
					nil,
					OpenWindowCommand{Window: WindowSpec{
						ID:                  "new",
						App:                 "New",
						Route:               testRoute("/new"),
						FloatingRect:        fullRect(),
						StackTargetWindowID: &targetID,
					}},
				)
				if members := stackMembersForWindow(
					t,
					result.Snapshot,
					"new",
				); !slices.Equal(
					members,
					[]WindowID{"target", "new"},
				) {
					t.Fatalf("open-as-tab members = %v", members)
				}
			})
		}
	})

	t.Run("Should reopen into a living stack with route nav and pin state preserved [UT-025]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"w1", "w2", "w3"})
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w3", Route: testRoute("/next"), Mode: NavigatePush},
		)
		executeTestCommand(t, environment.manager, "workspace-a", nil, SetStackActiveCommand{WindowID: "w3"})
		closed := executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w3"})
		want := closed.Snapshot.ClosedEntries[0].Windows[0]
		reopened := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		got := reopened.Snapshot.Windows["w3"]
		if got.Pinned != want.Pinned || !routeIntentsEqual(got.Route, want.Route) ||
			len(got.NavStack) != len(want.NavStack) {
			t.Fatalf("reopened window = %+v, want state = %+v", got, want)
		}
		location, found := findStackByWindow(&reopened.Snapshot, "w3")
		if !found || valueOrZero(location.activeID()) != "w3" || len(reopened.Snapshot.ClosedEntries) != 0 {
			t.Fatalf("reopened stack = %+v found=%v", location, found)
		}
		if !slices.Contains(reopened.Changes.NodeIDs, location.id()) {
			t.Fatalf("reopen changes = %+v, want stack node %q", reopened.Changes, location.id())
		}
	})

	t.Run("Should rebuild a dead frame or degrade a singleton to floating [UT-026]", func(t *testing.T) {
		t.Run("Should rebuild a multi-window frame", func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			createFloatingStack(t, environment.manager, []WindowID{"w1", "w2"})
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CloseWindowCommand{WindowID: "w1", Scope: CloseScopeGroup},
			)
			result := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
			if len(result.Snapshot.Desktops[0].FloatingStacks) != 1 ||
				!slices.Equal(stackMembersForWindow(t, result.Snapshot, "w1"), []WindowID{"w1", "w2"}) {
				t.Fatalf("rebuilt frame = %+v", result.Snapshot.Desktops[0])
			}
		})
		t.Run("Should reopen one window as floating", func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "solo", "desktop-default")
			executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "solo"})
			result := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
			if result.Snapshot.Windows["solo"].Placement != WindowPlacementFloating ||
				!slices.Contains(result.Snapshot.Desktops[0].Floating, WindowID("solo")) {
				t.Fatalf("reopened singleton = %+v", result.Snapshot)
			}
		})
		t.Run("Should fall back to the requesting client's active desktop", func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d3", Name: "Three"},
			)
			openTestWindow(t, environment.manager, "workspace-a", nil, "solo", "d2")
			executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "solo"})
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				DeleteDesktopCommand{DesktopID: "d2", DestinationID: new(DesktopID("d3"))},
			)
			clientID := ClientID("client-a")
			if _, err := environment.manager.RegisterClient(t.Context(), ClientRegistration{
				WorkspaceID: "workspace-a", ClientID: clientID, ActiveDesktopID: "d3",
			}); err != nil {
				t.Fatalf("RegisterClient() error = %v", err)
			}
			result := executeTestCommand(t, environment.manager, "workspace-a", &clientID, ReopenCommand{})
			if result.Snapshot.Windows["solo"].DesktopID != "d3" {
				t.Fatalf("reopened desktop = %q, want d3", result.Snapshot.Windows["solo"].DesktopID)
			}
		})
	})

	t.Run("Should treat reopen with empty history as a no-op without revision [UT-027]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		before, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		if result.Applied || result.Snapshot.Revision != before.Revision {
			t.Fatalf("empty reopen result = %+v", result)
		}
	})

	t.Run("Should reopen a fully pinned frame with every pin intact [UT-028]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"p1", "p2"})
		for _, windowID := range []WindowID{"p1", "p2"} {
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				PinWindowCommand{WindowID: windowID, Pinned: true},
			)
		}
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "p1", Scope: CloseScopeGroup},
		)
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		if !result.Snapshot.Windows["p1"].Pinned || !result.Snapshot.Windows["p2"].Pinned {
			t.Fatalf("reopened pins = %+v", result.Snapshot.Windows)
		}
	})

	t.Run("Should consume closed entries newest-first then become a no-op [UT-029]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: windowID})
		}
		first := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		if _, exists := first.Snapshot.Windows["w2"]; !exists {
			t.Fatalf("first reopen windows = %+v", first.Snapshot.Windows)
		}
		second := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		if _, exists := second.Snapshot.Windows["w1"]; !exists {
			t.Fatalf("second reopen windows = %+v", second.Snapshot.Windows)
		}
		third := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
		if third.Applied {
			t.Fatalf("third reopen = %+v", third)
		}
	})

	t.Run("Should close others and right as one unpinned batch [UT-172]", func(t *testing.T) {
		for _, scope := range []CloseScope{CloseScopeOthers, CloseScopeRight} {
			t.Run("Should apply scope "+string(scope), func(t *testing.T) {
				t.Parallel()
				environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
				created := createFloatingStack(t, environment.manager, []WindowID{"p", "w1", "w2", "w3"})
				executeTestCommand(
					t,
					environment.manager,
					"workspace-a",
					nil,
					PinWindowCommand{WindowID: "p", Pinned: true},
				)
				before, err := environment.manager.Snapshot(t.Context(), "workspace-a")
				if err != nil {
					t.Fatalf("Snapshot() error = %v", err)
				}
				result := executeTestCommand(
					t,
					environment.manager,
					"workspace-a",
					nil,
					CloseWindowCommand{WindowID: "w1", Scope: scope},
				)
				if result.Snapshot.Revision != before.Revision+1 || len(result.Snapshot.ClosedEntries) != 1 ||
					!slices.Equal(closedEntryWindowIDs(result.Snapshot.ClosedEntries[0]), []WindowID{"w2", "w3"}) {
					t.Fatalf("scoped close result = %+v, created revision = %d", result, created.Snapshot.Revision)
				}
				if _, pinnedLives := result.Snapshot.Windows["p"]; !pinnedLives {
					t.Fatal("pinned peer was closed")
				}
			})
		}
	})

	t.Run("Should reserve closed IDs through generation and release them on eviction [UT-173]", func(t *testing.T) {
		t.Parallel()
		windowID, err := randomID("window")
		if err != nil {
			t.Fatalf("randomID(window) error = %v", err)
		}
		if !strings.HasPrefix(windowID, "w-") || len(windowID) != 28 {
			t.Fatalf("randomID(window) = %q, want w-<26-char-random>", windowID)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(windowID, "w-")); err != nil {
			t.Fatalf("randomID(window) random suffix is not hexadecimal: %v", err)
		}
		config := DefaultConfig()
		config.ClosedEntryLimit = 1
		var generated atomic.Int64
		environment := newTestEnvironmentWithOptions(
			t,
			config,
			[]WorkspaceID{"workspace-a"},
			WithIDGenerator(func(kind string) (string, error) {
				if kind == "window" {
					if generated.Add(1) == 1 {
						return "w1", nil
					}
					return "w-new", nil
				}
				return kind + "-generated", nil
			}),
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w1"})
		snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: snapshot.Revision,
				Payload: OpenWindowCommand{
					Window: WindowSpec{
						ID:           "w1",
						App:          "Reserved",
						Route:        testRoute("/reserved"),
						FloatingRect: fullRect(),
					},
				},
			},
		)
		if err == nil {
			t.Fatal("OpenWindow(reserved caller ID) error = nil")
		}
		generatedResult := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			OpenWindowCommand{
				Window: WindowSpec{App: "Generated", Route: testRoute("/generated"), FloatingRect: fullRect()},
			},
		)
		if _, exists := generatedResult.Snapshot.Windows["w-new"]; !exists {
			t.Fatalf("generated windows = %+v", generatedResult.Snapshot.Windows)
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w-new"})
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			OpenWindowCommand{
				Window: WindowSpec{ID: "w1", App: "Released", Route: testRoute("/released"), FloatingRect: fullRect()},
			},
		)
		if _, exists := result.Snapshot.Windows["w1"]; !exists {
			t.Fatalf("released ID windows = %+v", result.Snapshot.Windows)
		}
	})
}

func TestDeletedSessionWindowReconciliation(t *testing.T) {
	t.Run(
		"Should retire matching windows to the session empty route without disturbing another workspace",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a", "workspace-b")
			const deletedSession = "session-deleted"

			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "desktop-two", Name: "Desktop 2"},
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-floating",
				deletedSession,
				"desktop-default",
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-tiled",
				deletedSession,
				"desktop-default",
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ToggleFloatingCommand{WindowID: "deleted-tiled"},
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-stack",
				deletedSession,
				"desktop-default",
			)
			openTestWindow(t, environment.manager, "workspace-a", nil, "keep-stack", "desktop-default")
			executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
				TargetWindowID: "deleted-stack",
				WindowIDs:      []WindowID{"keep-stack"},
			})
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-pinned",
				deletedSession,
				"desktop-default",
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				PinWindowCommand{WindowID: "deleted-pinned", Pinned: true},
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-minimized",
				deletedSession,
				"desktop-default",
			)
			executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{
				WindowID: "deleted-minimized",
				Minimize: true,
			})
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-other-desktop",
				deletedSession,
				"desktop-two",
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"deleted-closed",
				deletedSession,
				"desktop-default",
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CloseWindowCommand{WindowID: "deleted-closed"},
			)
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-a",
				"keep-session",
				"session-keep",
				"desktop-default",
			)
			openTestWindow(t, environment.manager, "workspace-a", nil, "keep-latest", "desktop-default")
			openSessionTestWindow(
				t,
				environment.manager,
				"workspace-b",
				"other-workspace",
				deletedSession,
				"desktop-default",
			)

			clientID := ClientID("client-a")
			registerTestClient(t, environment.manager, "workspace-a", clientID)
			deletedFocusWindow := WindowID("deleted-floating")
			executeTestCommand(t, environment.manager, "workspace-a", &clientID, FocusWindowCommand{
				WindowID: &deletedFocusWindow,
			})
			focusWindow := WindowID("keep-latest")
			executeTestCommand(t, environment.manager, "workspace-a", &clientID, FocusWindowCommand{
				WindowID: &focusWindow,
			})
			before := mustSnapshot(t, environment.manager, "workspace-a")
			subscription, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
				WorkspaceID:   "workspace-a",
				AfterRevision: before.Revision,
			})
			if err != nil {
				t.Fatalf("Subscribe() error = %v", err)
			}
			subscription = trackSubscription(t, subscription)

			if err := environment.manager.ReconcileDeletedSession(
				t.Context(),
				"workspace-a",
				deletedSession,
			); err != nil {
				t.Fatalf("ReconcileDeletedSession() error = %v", err)
			}
			update := <-subscription.Updates()
			if update.Event == nil || update.Event.CommandID != CommandWindowNavigate ||
				!slices.Equal(update.Event.Changes.WindowIDs, []WindowID{
					"deleted-floating",
					"deleted-minimized",
					"deleted-other-desktop",
					"deleted-pinned",
					"deleted-stack",
					"deleted-tiled",
				}) {
				t.Fatalf("deleted-session event = %+v", update)
			}

			after := mustSnapshot(t, environment.manager, "workspace-a")
			if after.Revision != before.Revision+1 || len(after.ClosedEntries) != 0 ||
				len(after.History.Undo) != 0 || len(after.History.Redo) != 0 {
				t.Fatalf(
					"deleted-session snapshot revision=%d closed=%+v history=(%d,%d), want revision %d and no restore history",
					after.Revision,
					after.ClosedEntries,
					len(after.History.Undo),
					len(after.History.Redo),
					before.Revision+1,
				)
			}
			if _, exists := after.Windows["deleted-closed"]; exists {
				t.Fatal("already-closed deleted session window returned to the snapshot")
			}
			for _, windowID := range []WindowID{
				"deleted-floating",
				"deleted-tiled",
				"deleted-stack",
				"deleted-pinned",
				"deleted-minimized",
				"deleted-other-desktop",
			} {
				assertRetiredSessionWindow(t, after, windowID)
			}
			if !after.Windows["deleted-pinned"].Pinned {
				t.Fatal("retired pinned window lost its pin")
			}
			if !after.Windows["deleted-minimized"].Minimized {
				t.Fatal("retired minimized window was restored")
			}
			for _, windowID := range []WindowID{"keep-stack", "keep-session", "keep-latest"} {
				if _, exists := after.Windows[windowID]; !exists {
					t.Fatalf("unrelated window %q was removed", windowID)
				}
			}
			if windowBelongsToSession(after.Windows["keep-session"], deletedSession) {
				t.Fatal("unrelated session window was retargeted")
			}
			desktopTwoIndex, desktopTwoExists := desktopIndexByID(&after, "desktop-two")
			if !desktopTwoExists {
				t.Fatal("deleted-only desktop was removed")
			}
			desktopTwo := after.Desktops[desktopTwoIndex]
			if len(desktopTwo.Floating) != 1 || desktopTwo.Floating[0] != "deleted-other-desktop" {
				t.Fatalf("retired window left its desktop: %+v", desktopTwo)
			}
			requireValidSnapshot(t, after)

			clients, err := environment.manager.Clients(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Clients() error = %v", err)
			}
			if len(clients) != 1 || valueOrZero(clients[0].FocusedWindowID) != "keep-latest" ||
				clients[0].ActiveDesktopID != "desktop-default" {
				t.Fatalf("repaired client view = %+v", clients)
			}

			otherWorkspace := mustSnapshot(t, environment.manager, "workspace-b")
			if _, exists := otherWorkspace.Windows["other-workspace"]; !exists {
				t.Fatal("window in another workspace was removed")
			}

			commitsBeforeNoOp := len(environment.repository.Commits("workspace-a"))
			if err := environment.manager.ReconcileDeletedSession(
				t.Context(),
				"workspace-a",
				deletedSession,
			); err != nil {
				t.Fatalf("ReconcileDeletedSession(no-op) error = %v", err)
			}
			noOp := mustSnapshot(t, environment.manager, "workspace-a")
			if noOp.Revision != after.Revision ||
				len(environment.repository.Commits("workspace-a")) != commitsBeforeNoOp {
				t.Fatalf(
					"repeated reconciliation revision=%d commits=%d, want revision %d commits %d",
					noOp.Revision,
					len(environment.repository.Commits("workspace-a")),
					after.Revision,
					commitsBeforeNoOp,
				)
			}
			reopened := executeTestCommand(t, environment.manager, "workspace-a", nil, ReopenCommand{})
			if reopened.Applied || reopened.Snapshot.Revision != after.Revision {
				t.Fatalf("reopen after deleted-session reconciliation = %+v", reopened)
			}
		},
	)

	t.Run("Should clear redo history that only contains the deleted session", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		const deletedSession = "session-redo-only"
		openSessionTestWindow(
			t,
			environment.manager,
			"workspace-a",
			"deleted-redo",
			deletedSession,
			"desktop-default",
		)
		undone := executeTestCommand(t, environment.manager, "workspace-a", nil, UndoLayoutCommand{})
		if _, exists := undone.Snapshot.Windows["deleted-redo"]; exists || len(undone.Snapshot.History.Redo) == 0 {
			t.Fatalf("undo snapshot = %+v, want deleted window only in redo history", undone.Snapshot)
		}

		before := undone.Snapshot
		if err := environment.manager.ReconcileDeletedSession(
			t.Context(),
			"workspace-a",
			deletedSession,
		); err != nil {
			t.Fatalf("ReconcileDeletedSession() error = %v", err)
		}
		after := mustSnapshot(t, environment.manager, "workspace-a")
		if after.Revision != before.Revision+1 || len(after.History.Undo) != 0 || len(after.History.Redo) != 0 {
			t.Fatalf("history-only reconciliation snapshot = %+v, want one revision and empty history", after)
		}
		_, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: after.Revision,
			Payload:          RedoLayoutCommand{},
		})
		if !errors.Is(err, ErrHistoryBoundary) {
			t.Fatalf("RedoLayoutCommand() error = %v, want history boundary", err)
		}
		if _, exists := mustSnapshot(t, environment.manager, "workspace-a").Windows["deleted-redo"]; exists {
			t.Fatal("redo resurrected the deleted session window")
		}
	})

	t.Run("Should clear undo history after the deleted close entry is evicted", func(t *testing.T) {
		t.Parallel()
		config := DefaultConfig()
		config.ClosedEntryLimit = 1
		environment := newTestEnvironment(t, config, "workspace-a")
		const deletedSession = "session-evicted-close"
		openSessionTestWindow(
			t,
			environment.manager,
			"workspace-a",
			"deleted-evicted",
			deletedSession,
			"desktop-default",
		)
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "deleted-evicted"})
		openTestWindow(t, environment.manager, "workspace-a", nil, "unrelated-closed", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "unrelated-closed"})

		before := mustSnapshot(t, environment.manager, "workspace-a")
		for _, entry := range before.ClosedEntries {
			for _, window := range entry.Windows {
				if windowBelongsToSession(window, deletedSession) {
					t.Fatalf("deleted session still has a closed entry after eviction: %+v", before.ClosedEntries)
				}
			}
		}
		if !historyContainsDeletedSession(before.History, deletedSession) {
			t.Fatal("test setup did not retain the deleted session in history")
		}

		if err := environment.manager.ReconcileDeletedSession(t.Context(), "workspace-a", deletedSession); err != nil {
			t.Fatalf("ReconcileDeletedSession() error = %v", err)
		}
		after := mustSnapshot(t, environment.manager, "workspace-a")
		if after.Revision != before.Revision+1 || len(after.History.Undo) != 0 || len(after.History.Redo) != 0 {
			t.Fatalf("evicted-entry reconciliation snapshot = %+v, want one revision and empty history", after)
		}
	})
}

func assertRetiredSessionWindow(t *testing.T, snapshot Snapshot, windowID WindowID) {
	t.Helper()
	window, exists := snapshot.Windows[windowID]
	if !exists {
		t.Fatalf("retired window %q missing", windowID)
	}
	if window.Route.Pathname != sessionEmptyRoutePath || window.InstanceKey != nil ||
		len(window.NavStack) != 0 {
		t.Fatalf(
			"retired window %q route=%q instance=%v nav=%d, want %s with cleared instance and stack",
			windowID,
			window.Route.Pathname,
			window.InstanceKey,
			len(window.NavStack),
			sessionEmptyRoutePath,
		)
	}
}

func openSessionTestWindow(
	t *testing.T,
	manager *Manager,
	workspaceID WorkspaceID,
	windowID WindowID,
	sessionID string,
	desktopID DesktopID,
) Result {
	t.Helper()
	key := sessionID
	return executeTestCommand(t, manager, workspaceID, nil, OpenWindowCommand{Window: WindowSpec{
		ID:           windowID,
		App:          "session",
		InstanceKey:  &key,
		Route:        testRoute(fmt.Sprintf("/session/%s", sessionID)),
		DesktopID:    desktopID,
		FloatingRect: fullRect(),
	}})
}

func TestWindowTabNavigationV3(t *testing.T) {
	t.Run(
		"Should cap navigation at the effective limit while retaining newest ancestors [UT-008][UT-031]",
		func(t *testing.T) {
			t.Parallel()
			config := DefaultConfig()
			config.NavStackLimit = 2
			environment := newTestEnvironment(t, config, "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			for _, path := range []string{"/one", "/two", "/three"} {
				executeTestCommand(
					t,
					environment.manager,
					"workspace-a",
					nil,
					NavigateWindowCommand{WindowID: "w1", Route: testRoute(path), Mode: NavigatePush},
				)
			}
			window := mustSnapshot(t, environment.manager, "workspace-a").Windows["w1"]
			if len(window.NavStack) != 2 || window.NavStack[0].Pathname != "/one" ||
				window.NavStack[1].Pathname != "/two" {
				t.Fatalf("capped navigation = %+v", window)
			}
		},
	)

	t.Run("Should apply a lowered closed-entry limit only on the next close [UT-009]", func(t *testing.T) {
		t.Parallel()
		config := DefaultConfig()
		config.ClosedEntryLimit = 2
		environment := newTestEnvironment(t, config, "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: windowID})
		}
		if got := len(mustSnapshot(t, environment.manager, "workspace-a").ClosedEntries); got != 2 {
			t.Fatalf("closed entries before lowering = %d", got)
		}
		config.ClosedEntryLimit = 1
		if err := environment.manager.UpdateDefaults(config); err != nil {
			t.Fatalf("UpdateDefaults() error = %v", err)
		}
		if got := len(mustSnapshot(t, environment.manager, "workspace-a").ClosedEntries); got != 2 {
			t.Fatalf("closed entries changed retroactively = %d", got)
		}
		openTestWindow(t, environment.manager, "workspace-a", nil, "w3", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w3"})
		if got := len(mustSnapshot(t, environment.manager, "workspace-a").ClosedEntries); got != 1 {
			t.Fatalf("closed entries after next close = %d", got)
		}
	})

	t.Run("Should push the prior route and pop it back [UT-030]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		pushed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/child"), Mode: NavigatePush},
		)
		if pushed.Snapshot.Windows["w1"].Route.Pathname != "/child" ||
			len(pushed.Snapshot.Windows["w1"].NavStack) != 1 {
			t.Fatalf("pushed window = %+v", pushed.Snapshot.Windows["w1"])
		}
		popped := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Mode: NavigatePop},
		)
		if popped.Snapshot.Windows["w1"].Route.Pathname != "/test" || len(popped.Snapshot.Windows["w1"].NavStack) != 0 {
			t.Fatalf("popped window = %+v", popped.Snapshot.Windows["w1"])
		}
	})

	t.Run("Should treat pop at the root as a successful no-op [UT-032]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		before := mustSnapshot(t, environment.manager, "workspace-a")
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Mode: NavigatePop},
		)
		if result.Applied || result.Snapshot.Revision != before.Revision {
			t.Fatalf("root pop = %+v", result)
		}
	})

	t.Run("Should preserve navigation while dragging a tab out and collapsing two to one [UT-033]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"w1", "w2"},
				Arrangement: ArrangementStack,
				Frame:       fullRect(),
				GroupID:     "group",
			},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/child"), Mode: NavigatePush},
		)
		result := executeTestCommand(t, environment.manager, "workspace-a", nil, ToggleFloatingCommand{WindowID: "w1"})
		root := result.Snapshot.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindLeaf || valueOrZero(root.WindowID) != "w2" ||
			len(result.Snapshot.Windows["w1"].NavStack) != 1 {
			t.Fatalf("drag-out result = %+v", result.Snapshot)
		}
	})

	t.Run("Should keep arrange stack producing one three-member node stack [UT-035]", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "w3"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"w1", "w2", "w3"},
				Arrangement: ArrangementStack,
				Frame:       fullRect(),
				GroupID:     "group",
			},
		)
		root := result.Snapshot.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindStack || !slices.Equal(root.WindowIDs, []WindowID{"w1", "w2", "w3"}) {
			t.Fatalf("arranged stack = %+v", root)
		}
	})
}

func mustSnapshot(t *testing.T, manager *Manager, workspaceID WorkspaceID) Snapshot {
	t.Helper()
	snapshot, err := manager.Snapshot(t.Context(), workspaceID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	return snapshot
}

func TestWindowNavigation(t *testing.T) {
	t.Run(
		"Should persist one canonical route event without history and project explicit client focus",
		func(t *testing.T) {
			t.Parallel()
			var observed atomic.Int64
			environment := newTestEnvironmentWithOptions(
				t,
				DefaultConfig(),
				[]WorkspaceID{"workspace-a"},
				WithEventObserver(func(_ context.Context, _ Event) { observed.Add(1) }),
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "d2")
			clientID := ClientID("client-a")
			registerTestClient(t, environment.manager, "workspace-a", clientID)
			registerTestClient(t, environment.manager, "workspace-a", "client-b")
			observed.Store(0)
			subscription, err := environment.manager.Subscribe(
				t.Context(),
				SubscriptionRequest{
					WorkspaceID:   "workspace-a",
					AfterRevision: opened.Snapshot.Revision,
					ClientID:      &clientID,
				},
			)
			if err != nil {
				t.Fatalf("Subscribe() error = %v", err)
			}
			subscription = trackSubscription(t, subscription)
			route := RouteIntent{
				Pathname: "/settings",
				Search: RouteSearch{
					"filter": json.RawMessage(` { "z": 1.0, "a": [true, null] } `),
				},
			}
			preview, err := environment.manager.Preview(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: opened.Snapshot.Revision,
					ClientID:         &clientID,
					Payload:          NavigateWindowCommand{WindowID: "w1", Route: route},
				},
			)
			if err != nil || !preview.Changed || preview.Client == nil ||
				preview.Client.ActiveDesktopID != "d2" ||
				valueOrZero(preview.Client.FocusedWindowID) != "w1" ||
				string(preview.Snapshot.Windows["w1"].Route.Search["filter"]) != `{"a":[true,null],"z":1}` {
				t.Fatalf("Preview(window.navigate) = %+v, error = %v", preview, err)
			}
			clients, err := environment.manager.Clients(t.Context(), "workspace-a")
			if err != nil || clients[0].ActiveDesktopID != "desktop-default" {
				t.Fatalf("Clients(after preview) = %+v, error = %v", clients, err)
			}

			historyBefore := cloneHistory(opened.Snapshot.History)
			commitsBefore := len(environment.repository.Commits("workspace-a"))
			navigated := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				NavigateWindowCommand{WindowID: "w1", Route: route},
			)
			if !navigated.Applied || navigated.Snapshot.Revision != opened.Snapshot.Revision+1 ||
				navigated.Client == nil || navigated.Client.ActiveDesktopID != "d2" ||
				valueOrZero(navigated.Client.FocusedWindowID) != "w1" ||
				len(navigated.Snapshot.History.Undo) != len(historyBefore.Undo) ||
				len(navigated.Snapshot.History.Redo) != len(historyBefore.Redo) ||
				len(environment.repository.Commits("workspace-a")) != commitsBefore+1 {
				t.Fatalf("Execute(window.navigate) = %+v", navigated)
			}
			eventUpdate := <-subscription.Updates()
			if eventUpdate.Event == nil ||
				eventUpdate.Event.CommandID != CommandWindowNavigate ||
				eventUpdate.Event.Revision != navigated.Snapshot.Revision ||
				len(eventUpdate.Event.Changes.WindowIDs) != 1 ||
				eventUpdate.Event.Changes.WindowIDs[0] != "w1" {
				t.Fatalf("navigation event update = %+v", eventUpdate)
			}
			clientUpdate := <-subscription.Updates()
			if clientUpdate.Client == nil || clientUpdate.Event != nil ||
				clientUpdate.Client.ClientID != clientID ||
				clientUpdate.Client.ActiveDesktopID != "d2" ||
				valueOrZero(clientUpdate.Client.FocusedWindowID) != "w1" ||
				clientUpdate.Client.PresentationRevision != 2 {
				t.Fatalf("navigation client update = %+v", clientUpdate)
			}
			if observed.Load() != 0 {
				t.Fatalf("navigation observer calls = %d, want 0", observed.Load())
			}
			loaded, err := environment.repository.Load(t.Context(), "workspace-a")
			if err != nil || loaded.Windows["w1"].Route.Pathname != "/settings" {
				t.Fatalf("Repository.Load() route = %+v, error = %v", loaded.Windows["w1"].Route, err)
			}

			nextRoute := RouteIntent{Pathname: "/settings/advanced", Search: RouteSearch{}}
			withoutClient := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				NavigateWindowCommand{WindowID: "w1", Route: nextRoute},
			)
			<-subscription.Updates()
			clients, err = environment.manager.Clients(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Clients(after clientless navigate) error = %v", err)
			}
			for _, client := range clients {
				if client.ClientID == "client-b" &&
					(client.ActiveDesktopID != "desktop-default" || client.FocusedWindowID != nil) {
					t.Fatalf("clientless navigation changed client-b = %+v", client)
				}
			}
			noOp := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				NavigateWindowCommand{WindowID: "w1", Route: nextRoute},
			)
			if noOp.Applied || noOp.Snapshot.Revision != withoutClient.Snapshot.Revision ||
				len(environment.repository.Commits("workspace-a")) != commitsBefore+2 {
				t.Fatalf("equal navigation = %+v", noOp)
			}
			select {
			case unexpected := <-subscription.Updates():
				t.Fatalf("equal navigation published event = %+v", unexpected)
			default:
			}
		},
	)

	t.Run("Should retarget the window instance on replace navigation and reset the nav stack", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/sessions/sess-a"), Mode: NavigatePush},
		)

		targetKey := "sess-b"
		retargeted := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{
				WindowID:    "w1",
				Route:       testRoute("/sessions/sess-b"),
				InstanceKey: &targetKey,
				Mode:        NavigateReplace,
			},
		)
		window := retargeted.Snapshot.Windows["w1"]
		if !retargeted.Applied ||
			window.InstanceKey == nil || *window.InstanceKey != targetKey ||
			window.Route.Pathname != "/sessions/sess-b" ||
			len(window.NavStack) != 0 {
			t.Fatalf("Execute(retarget) window = %+v applied=%v, want re-keyed window with empty nav stack",
				window, retargeted.Applied)
		}

		sameRouteKey := "sess-c"
		rekeyedOnly := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{
				WindowID:    "w1",
				Route:       testRoute("/sessions/sess-b"),
				InstanceKey: &sameRouteKey,
				Mode:        NavigateReplace,
			},
		)
		window = rekeyedOnly.Snapshot.Windows["w1"]
		if !rekeyedOnly.Applied || window.InstanceKey == nil || *window.InstanceKey != sameRouteKey {
			t.Fatalf("Execute(same-route retarget) window = %+v applied=%v, want instance key change applied",
				window, rekeyedOnly.Applied)
		}

		emptyKey := ""
		cleared := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{
				WindowID:    "w1",
				Route:       testRoute(sessionEmptyRoutePath),
				InstanceKey: &emptyKey,
				Mode:        NavigateReplace,
			},
		)
		window = cleared.Snapshot.Windows["w1"]
		if !cleared.Applied || window.InstanceKey != nil || window.Route.Pathname != sessionEmptyRoutePath {
			t.Fatalf("Execute(empty-key retarget) window = %+v applied=%v, want cleared instance key",
				window, cleared.Applied)
		}

		snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		pushKey := "sess-d"
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				CommandID:        CommandWindowNavigate,
				ExpectedRevision: snapshot.Revision,
				Payload: NavigateWindowCommand{
					WindowID:    "w1",
					Route:       testRoute("/sessions/sess-d"),
					InstanceKey: &pushKey,
					Mode:        NavigatePush,
				},
			},
		)
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("Execute(push retarget) error = %v, want %v", err, ErrInvalidCommand)
		}

		popKey := "sess-e"
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				CommandID:        CommandWindowNavigate,
				ExpectedRevision: snapshot.Revision,
				Payload: NavigateWindowCommand{
					WindowID:    "w1",
					InstanceKey: &popKey,
					Mode:        NavigatePop,
				},
			},
		)
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("Execute(pop retarget) error = %v, want %v", err, ErrInvalidCommand)
		}
	})
}

func TestRouteIntentValidation(t *testing.T) {
	tests := []struct {
		name      string
		route     RouteIntent
		wantError string
	}{
		{
			name:      "Should reject an empty pathname",
			route:     RouteIntent{Search: RouteSearch{}},
			wantError: "route pathname must be a non-empty absolute path",
		},
		{
			name:      "Should reject a relative pathname",
			route:     RouteIntent{Pathname: "settings", Search: RouteSearch{}},
			wantError: "route pathname must be application-internal and absolute",
		},
		{
			name:      "Should reject an external pathname",
			route:     RouteIntent{Pathname: "//example.com", Search: RouteSearch{}},
			wantError: "route pathname must be application-internal and absolute",
		},
		{
			name:      "Should reject a pathname query",
			route:     RouteIntent{Pathname: "/settings?q=1", Search: RouteSearch{}},
			wantError: "route pathname must not contain search or fragment data",
		},
		{
			name:      "Should reject a pathname fragment",
			route:     RouteIntent{Pathname: "/settings#panel", Search: RouteSearch{}},
			wantError: "route pathname must not contain search or fragment data",
		},
		{
			name:      "Should reject a non-object search",
			route:     RouteIntent{Pathname: "/settings"},
			wantError: "route search must be a JSON object",
		},
		{
			name: "Should reject malformed search values",
			route: RouteIntent{
				Pathname: "/settings",
				Search:   RouteSearch{"filter": json.RawMessage(`{"missing":`)},
			},
			wantError: "route search key \"filter\": decode JSON value:",
		},
		{
			name: "Should reject non-finite search values",
			route: RouteIntent{
				Pathname: "/settings",
				Search:   RouteSearch{"ratio": json.RawMessage(`NaN`)},
			},
			wantError: "route search key \"ratio\": decode JSON value:",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := CanonicalRouteIntent(test.route)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("CanonicalRouteIntent(%+v) error = %v, want %q", test.route, err, test.wantError)
			}
		})
	}

	t.Run("Should reject array and scalar search payloads at the JSON boundary", func(t *testing.T) {
		t.Parallel()
		for _, raw := range []string{
			`{"pathname":"/settings","search":[]}`,
			`{"pathname":"/settings","search":"query"}`,
		} {
			var route RouteIntent
			err := json.Unmarshal([]byte(raw), &route)
			typeError, typeErrorMatched := errors.AsType[*json.UnmarshalTypeError](err)
			if !typeErrorMatched || typeError.Field != "search" {
				t.Fatalf("json.Unmarshal(%s) error = %T %v, want search UnmarshalTypeError", raw, err, err)
			}
		}
	})
}

func TestStructuralArrangeAndDrop(t *testing.T) {
	t.Run("Should arrange every explicit participant and make a center drop a stack atomically", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2", "w3"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		arranged := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"w1", "w2", "w3"},
				Arrangement: ArrangementHorizontal,
				Frame:       fullRect(),
				GroupID:     "group-main",
			},
		)
		if members := nodeWindowIDs(arranged.Snapshot.Desktops[0].Groups[0].Root); len(members) != 3 {
			t.Fatalf("arranged members=%v", members)
		}
		dropped := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{
				WindowID:             "w1",
				DestinationDesktopID: "desktop-default",
				TargetWindowID:       new(WindowID("w2")),
				Placement:            DropCenter,
			},
		)
		placement, found := findWindowPlacement(&dropped.Snapshot, "w1")
		if !found || placement.placement != WindowPlacementStacked {
			t.Fatalf("center drop placement=%+v found=%v", placement, found)
		}
		if members := nodeWindowIDs(dropped.Snapshot.Desktops[0].Groups[0].Root); len(members) != 3 {
			t.Fatalf("drop members=%v", members)
		}
		beforeCommits := len(environment.repository.Commits("workspace-a"))
		_, err := environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: dropped.Snapshot.Revision,
				Payload: ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"w1", "w1"},
					Arrangement: ArrangementStack,
					Frame:       fullRect(),
				},
			},
		)
		if !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("duplicate arrange error=%v", err)
		}
		if len(environment.repository.Commits("workspace-a")) != beforeCommits {
			t.Fatal("rejected arrange wrote a commit")
		}
	})
}

func TestFloatingAndGroupTransitions(t *testing.T) {
	t.Run(
		"Should detach one window, restore its tiled slot, and move a whole group only when requested",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"w1", "w2"},
					Arrangement: ArrangementHorizontal,
					Frame:       fullRect(),
					GroupID:     "group-main",
				},
			)
			floatingRect := NormalizedRect{X: 0.9, Y: -1, Width: 0.4, Height: 0.5}
			detached := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ToggleFloatingCommand{WindowID: "w1", FloatingRect: &floatingRect},
			)
			window := detached.Snapshot.Windows["w1"]
			if window.Placement != WindowPlacementFloating || window.FloatingRect.X+window.FloatingRect.Width > 1 ||
				window.FloatingRect.Y < 0 {
				t.Fatalf("detached window=%+v", window)
			}
			restored := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ToggleFloatingCommand{WindowID: "w1"},
			)
			if restored.Snapshot.Windows["w1"].Placement != WindowPlacementTiled {
				t.Fatalf("restored placement=%q", restored.Snapshot.Windows["w1"].Placement)
			}
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			targetWindowID := WindowID("w2")
			groupFloatingRect := fullRect()
			invalidGroupMoves := []MoveWindowCommand{
				{
					WindowID: "w1", DestinationDesktopID: "d2", MoveGroup: true,
					TargetWindowID: &targetWindowID,
				},
				{
					WindowID: "w1", DestinationDesktopID: "d2", MoveGroup: true,
					Placement: DropAfter,
				},
				{
					WindowID: "w1", DestinationDesktopID: "d2", MoveGroup: true,
					FloatingRect: &groupFloatingRect,
				},
			}
			commitsBefore := len(environment.repository.Commits("workspace-a"))
			for _, command := range invalidGroupMoves {
				snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
				if err != nil {
					t.Fatalf("Snapshot() error = %v", err)
				}
				_, err = environment.manager.Execute(t.Context(), CommandRequest{
					WorkspaceID: "workspace-a", ExpectedRevision: snapshot.Revision, Payload: command,
				})
				if !errors.Is(err, ErrInvalidCommand) {
					t.Fatalf("Execute(contradictory group move) error = %v, want ErrInvalidCommand", err)
				}
			}
			if len(environment.repository.Commits("workspace-a")) != commitsBefore {
				t.Fatal("contradictory group move wrote a commit")
			}
			moved := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				MoveWindowCommand{WindowID: "w1", DestinationDesktopID: "d2", MoveGroup: true},
			)
			if moved.Snapshot.Windows["w1"].DesktopID != "d2" || moved.Snapshot.Windows["w2"].DesktopID != "d2" {
				t.Fatalf("group windows=%+v", moved.Snapshot.Windows)
			}
			if len(moved.Snapshot.Desktops[0].Groups) != 0 {
				t.Fatal("source group remained")
			}
			requireValidSnapshot(t, moved.Snapshot)
		},
	)
}

func TestDesktopLifecycle(t *testing.T) {
	t.Run("Should clear a minimized zoom anchor when its desktop is transferred", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeAndZoom(
			t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks",
		)
		liftedID := zoomed.Snapshot.Windows["tasks"].DesktopID
		executeTestCommand(t, environment.manager, "workspace-a", &clientID, CloseWindowCommand{
			WindowID: "tasks", Minimize: true,
		})

		destination := DesktopID("desktop-default")
		transferred := executeTestCommand(t, environment.manager, "workspace-a", nil, DeleteDesktopCommand{
			DesktopID: liftedID, DestinationID: &destination,
		})
		tasks := transferred.Snapshot.Windows["tasks"]
		if tasks.DesktopID != destination || tasks.ReturnAnchor != nil {
			t.Fatalf("transferred minimized window = %+v", tasks)
		}

		restored := executeTestCommand(t, environment.manager, "workspace-a", &clientID, OpenWindowCommand{
			RestoreWindowID: new(WindowID("tasks")),
		})
		tasks = restored.Snapshot.Windows["tasks"]
		if tasks.Zoomed || tasks.ReturnAnchor != nil || tasks.DesktopID != destination {
			t.Fatalf("restored transferred window = %+v", tasks)
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run("Should preserve stable IDs and require an atomic destination for non-empty deletion", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d3", Name: "Three"},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			UpdateDesktopCommand{DesktopID: "d3", Name: "Renamed"},
		)
		reordered := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ReorderDesktopCommand{DesktopID: "d3", Order: 0},
		)
		if reordered.Snapshot.Desktops[0].ID != "d3" || reordered.Snapshot.Desktops[0].Name != "Renamed" {
			t.Fatalf("reordered desktops=%+v", reordered.Snapshot.Desktops)
		}
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		switched := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		if switched.Client == nil || switched.Client.ActiveDesktopID != "d2" ||
			switched.Snapshot.Revision != reordered.Snapshot.Revision {
			t.Fatalf("switch result=%+v", switched)
		}
		openTestWindow(t, environment.manager, "workspace-a", &clientID, "w1", "d2")
		snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error=%v", err)
		}
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: snapshot.Revision,
				Payload:          DeleteDesktopCommand{DesktopID: "d2"},
			},
		)
		if !errors.Is(err, ErrDestinationRequired) {
			t.Fatalf("delete without destination error=%v", err)
		}
		destination := DesktopID("desktop-default")
		deleted := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			DeleteDesktopCommand{DesktopID: "d2", DestinationID: &destination},
		)
		if deleted.Snapshot.Windows["w1"].DesktopID != destination {
			t.Fatalf("transferred window=%+v", deleted.Snapshot.Windows["w1"])
		}
		executeTestCommand(t, environment.manager, "workspace-a", nil, DeleteDesktopCommand{DesktopID: "d3"})
		last, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error=%v", err)
		}
		_, err = environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: last.Revision,
				Payload:          DeleteDesktopCommand{DesktopID: destination},
			},
		)
		if !errors.Is(err, ErrFinalDesktop) {
			t.Fatalf("final delete error=%v", err)
		}
	})

	t.Run("Should transfer tiled islands into an occupied destination without overlap", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "destination-tiled", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "source-tiled", "d2")
		openTestWindow(t, environment.manager, "workspace-a", nil, "source-floating", "d2")
		for _, arrangement := range []ArrangeLayoutCommand{
			{
				DesktopID: "desktop-default", WindowIDs: []WindowID{"destination-tiled"},
				Arrangement: ArrangementHorizontal, GroupID: "destination-group",
			},
			{
				DesktopID: "d2", WindowIDs: []WindowID{"source-tiled"},
				Arrangement: ArrangementHorizontal, GroupID: "source-group",
			},
		} {
			executeTestCommand(t, environment.manager, "workspace-a", nil, arrangement)
		}

		destination := DesktopID("desktop-default")
		deleted := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			DeleteDesktopCommand{DesktopID: "d2", DestinationID: &destination},
		)

		if len(deleted.Snapshot.Desktops) != 1 || len(deleted.Snapshot.Desktops[0].Groups) != 2 {
			t.Fatalf("transferred desktops = %+v, want one desktop with two tiled islands", deleted.Snapshot.Desktops)
		}
		for _, windowID := range []WindowID{"destination-tiled", "source-tiled", "source-floating"} {
			if deleted.Snapshot.Windows[windowID].DesktopID != destination {
				t.Fatalf(
					"window %q = %+v, want destination %q",
					windowID,
					deleted.Snapshot.Windows[windowID],
					destination,
				)
			}
		}
		if len(deleted.Snapshot.Desktops[0].Floating) != 1 ||
			deleted.Snapshot.Desktops[0].Floating[0] != "source-floating" {
			t.Fatalf("transferred floating windows = %v", deleted.Snapshot.Desktops[0].Floating)
		}
		requireValidSnapshot(t, deleted.Snapshot)
	})
}

func TestFocusAndZoom(t *testing.T) {
	t.Run(
		"Should keep clients independent, stop directional focus at edges, and restore focused MRU",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			for _, windowID := range []WindowID{"w1", "w2", "w3"} {
				openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			}
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"w1", "w2", "w3"},
					Arrangement: ArrangementHorizontal,
					Frame:       fullRect(),
					GroupID:     "group-main",
				},
			)
			clientA, clientB := ClientID("client-a"), ClientID("client-b")
			registerTestClient(t, environment.manager, "workspace-a", clientA)
			registerTestClient(t, environment.manager, "workspace-a", clientB)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientA,
				FocusWindowCommand{WindowID: new(WindowID("w2"))},
			)
			right := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientA,
				FocusWindowCommand{Direction: FocusRight},
			)
			if right.Client == nil || valueOrZero(right.Client.FocusedWindowID) != "w3" {
				t.Fatalf("right focus=%+v", right.Client)
			}
			edge := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientA,
				FocusWindowCommand{Direction: FocusRight},
			)
			if edge.Applied {
				t.Fatal("focus wrapped at the right edge")
			}
			clients, err := environment.manager.Clients(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Clients() error=%v", err)
			}
			if len(clients) != 2 || valueOrZero(clients[1].FocusedWindowID) == "w3" {
				t.Fatalf("client views=%+v", clients)
			}
			closed := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientA,
				CloseWindowCommand{WindowID: "w3"},
			)
			if closed.Client == nil || valueOrZero(closed.Client.FocusedWindowID) != "w2" {
				t.Fatalf("MRU after close=%+v", closed.Client)
			}
		},
	)

	t.Run("Should activate the window's desktop when focusing across desktops", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "d2")
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("w1"))},
		)
		focused := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("w2"))},
		)
		if focused.Client == nil || focused.Client.ActiveDesktopID != "d2" ||
			valueOrZero(focused.Client.FocusedWindowID) != "w2" {
			t.Fatalf("cross-desktop focus client=%+v", focused.Client)
		}
		if len(focused.Client.FocusOrder) != 1 || focused.Client.FocusOrder[0] != "w2" {
			t.Fatalf("focus order not repaired to the activated desktop: %+v", focused.Client.FocusOrder)
		}
	})

	t.Run("Should follow a minimized window to its desktop when restoring it", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "d2")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "w2", Minimize: true},
		)
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("w1"))},
		)
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			OpenWindowCommand{
				Window: WindowSpec{
					ID:           "w2",
					App:          "Test",
					Route:        testRoute("/test"),
					DesktopID:    "d2",
					FloatingRect: NormalizedRect{X: 0.2, Y: 0.2, Width: 0.5, Height: 0.5},
				},
				RestoreWindowID: new(WindowID("w2")),
			},
		)
		if restored.Snapshot.Windows["w2"].Minimized {
			t.Fatalf("restored window stayed minimized: %+v", restored.Snapshot.Windows["w2"])
		}
		if restored.Client == nil || restored.Client.ActiveDesktopID != "d2" ||
			valueOrZero(restored.Client.FocusedWindowID) != "w2" {
			t.Fatalf("restore client=%+v", restored.Client)
		}
	})

	t.Run("Should restore a minimized island member into its exact island", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		left := executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID: "desktop-default", WindowIDs: []WindowID{"w1"}, Arrangement: ArrangementHorizontal,
			Frame: NormalizedRect{X: 0, Y: 0, Width: 0.5, Height: 1}, GroupID: "left",
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID: "desktop-default", WindowIDs: []WindowID{"w2"}, Arrangement: ArrangementHorizontal,
			Frame: NormalizedRect{X: 0.5, Y: 0, Width: 0.5, Height: 1}, GroupID: "right",
		})
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "w1", Minimize: true},
		)
		restored := executeTestCommand(t, environment.manager, "workspace-a", nil, OpenWindowCommand{
			RestoreWindowID: new(WindowID("w1")),
		})
		if restored.Snapshot.Windows["w1"].Placement != WindowPlacementTiled {
			t.Fatalf("restored island member = %+v", restored.Snapshot.Windows["w1"])
		}
		requireExactGroup(t, restored.Snapshot, "desktop-default", left.Snapshot.Desktops[0].Groups[0])
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run(
		"Should keep a restored window on its own desktop when the focused window is tiled elsewhere",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
			arranged := executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
				DesktopID: "desktop-default", WindowIDs: []WindowID{"w1", "w2"}, Arrangement: ArrangementHorizontal,
				Frame: fullRect(), GroupID: "group-main",
			})
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			openTestWindow(t, environment.manager, "workspace-a", nil, "w3", "d2")
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CloseWindowCommand{WindowID: "w3", Minimize: true},
			)
			clientID := ClientID("client-a")
			registerTestClient(t, environment.manager, "workspace-a", clientID)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				FocusWindowCommand{WindowID: new(WindowID("w1"))},
			)
			restored := executeTestCommand(t, environment.manager, "workspace-a", &clientID, OpenWindowCommand{
				RestoreWindowID: new(WindowID("w3")),
			})
			w3 := restored.Snapshot.Windows["w3"]
			if w3.DesktopID != "d2" || w3.Placement != WindowPlacementFloating || w3.Minimized {
				t.Fatalf("cross-desktop restore = %+v", w3)
			}
			requireExactGroup(t, restored.Snapshot, "desktop-default", arranged.Snapshot.Desktops[0].Groups[0])
			if restored.Client == nil || restored.Client.ActiveDesktopID != "d2" {
				t.Fatalf("restore client = %+v", restored.Client)
			}
		},
	)

	t.Run("Should count floating frames as desktop content and transfer them on delete", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "d2")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "d2")
		executeTestCommand(t, environment.manager, "workspace-a", nil, GroupWindowsCommand{
			TargetWindowID: "w1", WindowIDs: []WindowID{"w2"},
		})
		snapshot := mustSnapshot(t, environment.manager, "workspace-a")
		_, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", CommandID: CommandDesktopDelete, ExpectedRevision: snapshot.Revision,
			Payload: DeleteDesktopCommand{DesktopID: "d2"},
		})
		if !errors.Is(err, ErrDestinationRequired) {
			t.Fatalf("delete of a desktop holding a floating frame error = %v, want ErrDestinationRequired", err)
		}
		destination := DesktopID("desktop-default")
		deleted := executeTestCommand(t, environment.manager, "workspace-a", nil, DeleteDesktopCommand{
			DesktopID: "d2", DestinationID: &destination,
		})
		if len(deleted.Snapshot.Desktops) != 1 || len(deleted.Snapshot.Desktops[0].FloatingStacks) != 1 {
			t.Fatalf("transfer dropped the floating frame: %+v", deleted.Snapshot.Desktops)
		}
		for _, windowID := range []WindowID{"w1", "w2"} {
			if deleted.Snapshot.Windows[windowID].DesktopID != "desktop-default" {
				t.Fatalf("window %q stayed on the deleted desktop: %+v", windowID, deleted.Snapshot.Windows[windowID])
			}
		}
		requireValidSnapshot(t, deleted.Snapshot)
	})
}

func requireExactGroup(t *testing.T, snapshot Snapshot, desktopID DesktopID, expected LayoutGroup) {
	t.Helper()
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		t.Fatalf("desktop %q missing: %+v", desktopID, snapshot.Desktops)
	}
	for _, group := range snapshot.Desktops[desktopIndex].Groups {
		if group.ID != expected.ID {
			continue
		}
		if !layoutGroupsEqual(group, expected) {
			t.Fatalf("group %q drifted:\n got %+v\nwant %+v", expected.ID, group, expected)
		}
		return
	}
	t.Fatalf("group %q missing on %q: %+v", expected.ID, desktopID, snapshot.Desktops[desktopIndex].Groups)
}
