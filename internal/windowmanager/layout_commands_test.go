package windowmanager

// Suite: window-manager structural commands
// Invariant: every arrangement, drop, swap, insertion, resize, and balance command produces one valid deterministic topology.
// Boundary IN: structural reducers through the transactional manager.
// Boundary OUT: pointer hit-testing and viewport pixel projection.

import (
	"errors"
	"math"
	"testing"
)

func TestArrangementModes(t *testing.T) {
	tests := []struct {
		name        string
		arrangement Arrangement
		windowIDs   []WindowID
		rootKind    NodeKind
		rootAxis    Axis
		childKind   NodeKind
		childAxis   Axis
		nodeCount   int
		placement   WindowPlacement
	}{
		{
			name:        "Should arrange one participant as a leaf",
			arrangement: ArrangementHorizontal,
			windowIDs:   []WindowID{"w1"},
			rootKind:    NodeKindLeaf,
			nodeCount:   1,
			placement:   WindowPlacementTiled,
		},
		{
			name:        "Should arrange horizontal participants as one split",
			arrangement: ArrangementHorizontal,
			windowIDs:   []WindowID{"w1", "w2", "w3"},
			rootKind:    NodeKindSplit,
			rootAxis:    AxisHorizontal,
			nodeCount:   4,
			placement:   WindowPlacementTiled,
		},
		{
			name:        "Should arrange vertical participants as one split",
			arrangement: ArrangementVertical,
			windowIDs:   []WindowID{"w1", "w2", "w3"},
			rootKind:    NodeKindSplit,
			rootAxis:    AxisVertical,
			nodeCount:   4,
			placement:   WindowPlacementTiled,
		},
		{
			name:        "Should arrange grid participants as vertical rows",
			arrangement: ArrangementGrid,
			windowIDs:   []WindowID{"w1", "w2", "w3", "w4"},
			rootKind:    NodeKindSplit,
			rootAxis:    AxisVertical,
			childKind:   NodeKindSplit,
			childAxis:   AxisHorizontal,
			nodeCount:   7,
			placement:   WindowPlacementTiled,
		},
		{
			name:        "Should arrange stack participants into one active stack",
			arrangement: ArrangementStack,
			windowIDs:   []WindowID{"w1", "w2", "w3"},
			rootKind:    NodeKindStack,
			nodeCount:   1,
			placement:   WindowPlacementStacked,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			for _, windowID := range test.windowIDs {
				openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			}
			result := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   test.windowIDs,
					Arrangement: test.arrangement,
					GroupID:     "group-main",
				},
			)
			root := result.Snapshot.Desktops[0].Groups[0].Root
			if root.Kind != test.rootKind || (test.rootAxis != "" && valueOrZero(root.Axis) != test.rootAxis) {
				t.Fatalf("arranged root = %+v", root)
			}
			if members := nodeWindowIDs(root); len(members) != len(test.windowIDs) {
				t.Fatalf("arranged members = %v", members)
			}
			if len(result.Changes.NodeIDs) != test.nodeCount {
				t.Fatalf("arranged node changes = %v, want %d nodes", result.Changes.NodeIDs, test.nodeCount)
			}
			if test.childKind != "" {
				if len(root.Children) != 2 {
					t.Fatalf("arranged grid rows = %+v, want 2 rows", root.Children)
				}
				for _, child := range root.Children {
					if child.Kind != test.childKind || valueOrZero(child.Axis) != test.childAxis {
						t.Fatalf("arranged grid row = %+v, want %s/%s", child, test.childKind, test.childAxis)
					}
				}
			}
			for _, windowID := range test.windowIDs {
				if result.Snapshot.Windows[windowID].Placement != test.placement {
					t.Fatalf("window %q placement = %q", windowID, result.Snapshot.Windows[windowID].Placement)
				}
			}
			requireValidSnapshot(t, result.Snapshot)
		})
	}
}

func TestStructuralDropPlacements(t *testing.T) {
	tests := []struct {
		name      string
		placement DropPlacement
		axis      Axis
		first     WindowID
		kind      NodeKind
	}{
		{
			name:      "Should insert before a target",
			placement: DropBefore,
			axis:      AxisHorizontal,
			first:     "moving",
			kind:      NodeKindSplit,
		},
		{
			name:      "Should insert after a target",
			placement: DropAfter,
			axis:      AxisHorizontal,
			first:     "target",
			kind:      NodeKindSplit,
		},
		{
			name:      "Should insert left of a target",
			placement: DropLeft,
			axis:      AxisHorizontal,
			first:     "moving",
			kind:      NodeKindSplit,
		},
		{
			name:      "Should insert right of a target",
			placement: DropRight,
			axis:      AxisHorizontal,
			first:     "target",
			kind:      NodeKindSplit,
		},
		{
			name:      "Should insert above a target",
			placement: DropTop,
			axis:      AxisVertical,
			first:     "moving",
			kind:      NodeKindSplit,
		},
		{
			name:      "Should insert below a target",
			placement: DropBottom,
			axis:      AxisVertical,
			first:     "target",
			kind:      NodeKindSplit,
		},
		{name: "Should insert at center as a stack", placement: DropCenter, first: "target", kind: NodeKindStack},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			openTestWindow(t, environment.manager, "workspace-a", nil, "target", "desktop-default")
			openTestWindow(t, environment.manager, "workspace-a", nil, "moving", "desktop-default")
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"target"},
					Arrangement: ArrangementHorizontal,
					GroupID:     "group-main",
				},
			)
			result := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				MoveWindowCommand{
					WindowID:             "moving",
					DestinationDesktopID: "desktop-default",
					TargetWindowID:       new(WindowID("target")),
					Placement:            test.placement,
				},
			)
			root := result.Snapshot.Desktops[0].Groups[0].Root
			members := nodeWindowIDs(root)
			if root.Kind != test.kind || len(members) != 2 || members[0] != test.first {
				t.Fatalf("drop root = %+v, members = %v", root, members)
			}
			if test.axis != "" && valueOrZero(root.Axis) != test.axis {
				t.Fatalf("drop axis = %q, want %q", valueOrZero(root.Axis), test.axis)
			}
			requireValidSnapshot(t, result.Snapshot)
		})
	}
}

func TestSwapWindows(t *testing.T) {
	t.Run("Should exchange floating and tiled placements without corrupting membership metadata", func(t *testing.T) {
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
				WindowIDs:   []WindowID{"w1", "w2"},
				Arrangement: ArrangementHorizontal,
				GroupID:     "group-main",
			},
		)
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			SwapWindowsCommand{FirstWindowID: "w1", SecondWindowID: "w3"},
		)
		w1Placement, w1Found := findWindowPlacement(&result.Snapshot, "w1")
		w3Placement, w3Found := findWindowPlacement(&result.Snapshot, "w3")
		if !w1Found || !w3Found || w1Placement.placement != WindowPlacementFloating ||
			w3Placement.placement != WindowPlacementTiled {
			t.Fatalf("swap placements: w1=%+v found=%v, w3=%+v found=%v", w1Placement, w1Found, w3Placement, w3Found)
		}
		if result.Snapshot.Windows["w1"].Placement != WindowPlacementFloating ||
			result.Snapshot.Windows["w3"].Placement != WindowPlacementTiled {
			t.Fatalf("swap windows = %+v", result.Snapshot.Windows)
		}
		requireValidSnapshot(t, result.Snapshot)

		noOp := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			SwapWindowsCommand{FirstWindowID: "w1", SecondWindowID: "w1"},
		)
		if noOp.Applied {
			t.Fatal("self swap was applied")
		}
		if _, err := environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: noOp.Snapshot.Revision,
				Payload:          SwapWindowsCommand{FirstWindowID: "w1", SecondWindowID: "missing"},
			},
		); !errors.Is(
			err,
			ErrWindowNotFound,
		) {
			t.Fatalf("swap missing window error = %v", err)
		}
	})

	t.Run("Should exchange distinct members within the same tiled group", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"w1", "w2"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		arranged := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID: "desktop-default", WindowIDs: []WindowID{"w1", "w2"},
				Arrangement: ArrangementHorizontal, GroupID: "group-main",
			},
		)

		swapped := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			SwapWindowsCommand{FirstWindowID: "w1", SecondWindowID: "w2"},
		)
		members := nodeWindowIDs(swapped.Snapshot.Desktops[0].Groups[0].Root)
		if !swapped.Applied || swapped.Snapshot.Revision != arranged.Snapshot.Revision+1 ||
			len(members) != 2 || members[0] != "w2" || members[1] != "w1" {
			t.Fatalf("same-group swap = %+v, members = %v", swapped, members)
		}
		requireValidSnapshot(t, swapped.Snapshot)
	})
}

func TestBalanceAndResize(t *testing.T) {
	t.Run("Should recursively balance a group and balance one split independently", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		snapshot := validThreeWindowSnapshot()
		document := LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops:    snapshot.Desktops,
			Windows:     snapshot.Windows,
		}
		replaced, err := environment.manager.ReplaceLayout(
			t.Context(),
			ReplaceLayoutRequest{WorkspaceID: "workspace-a", ExpectedRevision: 0, Document: document},
		)
		if err != nil {
			t.Fatalf("ReplaceLayout() error = %v", err)
		}
		rootResized := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 0, Delta: 0.1},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ResizeLayoutCommand{SplitID: "right", BoundaryIndex: 0, Delta: 0.1},
		)
		groupID := GroupID("group-1")
		balanced := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			BalanceLayoutCommand{GroupID: &groupID},
		)
		for _, splitID := range []NodeID{"root", "right"} {
			node, found := findNodeInSnapshot(&balanced.Snapshot, splitID)
			if !found || !weightsEqual(node.Weights) {
				t.Fatalf("balanced split %q = %+v", splitID, node)
			}
		}
		noOp := executeTestCommand(t, environment.manager, "workspace-a", nil, BalanceLayoutCommand{GroupID: &groupID})
		if noOp.Applied {
			t.Fatal("already-balanced group was applied")
		}

		resizedAgain := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 0, Delta: -0.1},
		)
		siblingResized := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ResizeLayoutCommand{SplitID: "right", BoundaryIndex: 0, Delta: -0.1},
		)
		siblingBefore, found := findNodeInSnapshot(&siblingResized.Snapshot, "right")
		if !found {
			t.Fatal("sibling split missing before targeted balance")
		}
		splitID := NodeID("root")
		oneSplit := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			BalanceLayoutCommand{SplitID: &splitID},
		)
		node, found := findNodeInSnapshot(&oneSplit.Snapshot, splitID)
		if !found || !weightsEqual(node.Weights) || oneSplit.Snapshot.Revision != siblingResized.Snapshot.Revision+1 {
			t.Fatalf("single split balance = %+v", oneSplit)
		}
		siblingAfter, found := findNodeInSnapshot(&oneSplit.Snapshot, "right")
		if !found || len(siblingAfter.Weights) != len(siblingBefore.Weights) {
			t.Fatalf("sibling split after targeted balance = %+v", siblingAfter)
		}
		for index := range siblingBefore.Weights {
			if siblingAfter.Weights[index] != siblingBefore.Weights[index] {
				t.Fatalf("sibling weights changed from %v to %v", siblingBefore.Weights, siblingAfter.Weights)
			}
		}
		if resizedAgain.Snapshot.Revision+1 != siblingResized.Snapshot.Revision {
			t.Fatalf("sibling resize revision = %d", siblingResized.Snapshot.Revision)
		}
		if rootResized.Snapshot.Revision != replaced.Snapshot.Revision+1 {
			t.Fatalf("first resize revision = %d", rootResized.Snapshot.Revision)
		}
	})

	t.Run("Should reject invalid resize and balance targets without committing", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		snapshot := validThreeWindowSnapshot()
		document := LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops:    snapshot.Desktops,
			Windows:     snapshot.Windows,
		}
		replaced, err := environment.manager.ReplaceLayout(
			t.Context(),
			ReplaceLayoutRequest{WorkspaceID: "workspace-a", ExpectedRevision: 0, Document: document},
		)
		if err != nil {
			t.Fatalf("ReplaceLayout() error = %v", err)
		}
		requests := []Command{
			ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 0, Delta: math.NaN()},
			ResizeLayoutCommand{SplitID: "missing", BoundaryIndex: 0, Delta: 0.1},
			ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 4, Delta: 0.1},
			ResizeLayoutCommand{SplitID: "root", BoundaryIndex: 0, Delta: 0.9},
			BalanceLayoutCommand{},
		}
		for _, command := range requests {
			_, executeErr := environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: replaced.Snapshot.Revision,
					Payload:          command,
				},
			)
			if !errors.Is(executeErr, ErrInvalidCommand) {
				t.Fatalf("Execute(%T) error = %v", command, executeErr)
			}
		}
		if len(environment.repository.Commits("workspace-a")) != 1 {
			t.Fatalf("rejected commands wrote commits = %d", len(environment.repository.Commits("workspace-a")))
		}
	})
}

func TestNewWindowInsertionPolicy(t *testing.T) {
	t.Run("Should tile beside live focus and fall back to floating when no focus target exists", func(t *testing.T) {
		t.Parallel()
		config := DefaultConfig()
		config.NewWindowPolicy = NewWindowInsert
		environment := newTestEnvironment(t, config, "workspace-a")
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		first := openTestWindow(t, environment.manager, "workspace-a", &clientID, "w1", "desktop-default")
		if first.Snapshot.Windows["w1"].Placement != WindowPlacementTiled ||
			len(first.Snapshot.Desktops[0].Groups) != 1 {
			t.Fatalf("first inserted window = %+v", first.Snapshot)
		}
		second := openTestWindow(t, environment.manager, "workspace-a", &clientID, "w2", "desktop-default")
		if second.Snapshot.Windows["w2"].Placement != WindowPlacementTiled ||
			len(nodeWindowIDs(second.Snapshot.Desktops[0].Groups[0].Root)) != 2 {
			t.Fatalf("focused insertion = %+v", second.Snapshot)
		}
		third := openTestWindow(t, environment.manager, "workspace-a", nil, "w3", "desktop-default")
		if third.Snapshot.Windows["w3"].Placement != WindowPlacementFloating {
			t.Fatalf("unfocused insertion placement = %q", third.Snapshot.Windows["w3"].Placement)
		}

		generated := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			OpenWindowCommand{
				Window: WindowSpec{
					App: "  Generated  ", Route: testRoute("/generated"),
					FloatingRect: NormalizedRect{Width: -1, Height: math.NaN()},
				},
			},
		)
		if len(generated.Changes.WindowIDs) != 1 {
			t.Fatalf("generated window changes = %+v", generated.Changes)
		}
		window := generated.Snapshot.Windows[generated.Changes.WindowIDs[0]]
		if window.ID == "" || window.App != "Generated" || !validRect(window.FloatingRect) {
			t.Fatalf("generated window = %+v", window)
		}

		beforeCommits := len(environment.repository.Commits("workspace-a"))
		invalidCommands := []struct {
			command OpenWindowCommand
			want    error
		}{
			{
				command: OpenWindowCommand{
					Window: WindowSpec{ID: "bad-app", App: " ", Route: testRoute("/bad-app")},
				},
				want: ErrInvalidCommand,
			},
			{
				command: OpenWindowCommand{
					Window: WindowSpec{ID: "w1", App: "Duplicate", Route: testRoute("/duplicate")},
				},
				want: ErrInvalidCommand,
			},
			{command: OpenWindowCommand{Window: WindowSpec{
				ID: "bad-desktop", App: "Bad", Route: testRoute("/bad-desktop"), DesktopID: "missing",
			}}, want: ErrDesktopNotFound},
		}
		for _, test := range invalidCommands {
			snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}
			if _, err := environment.manager.Execute(
				t.Context(),
				CommandRequest{WorkspaceID: "workspace-a", ExpectedRevision: snapshot.Revision, Payload: test.command},
			); !errors.Is(err, test.want) {
				t.Fatalf("Execute(%+v) error = %v, want %v", test.command, err, test.want)
			}
		}
		if len(environment.repository.Commits("workspace-a")) != beforeCommits {
			t.Fatal("invalid open command wrote a commit")
		}
	})
}
