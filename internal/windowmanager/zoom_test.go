package windowmanager

// Suite: window zoom
// Invariant: a zoomed unit is the only full-frame island of a desktop. It zooms in place when its
// desktop shows nothing else and lifts to a fresh desktop otherwise; unzoom returns it to the slot
// it left and a desktop zoom created disappears once the unit has left it empty.
// Boundary IN: window.zoom, window.close, window.open restore, window.move, layout.arrange.
// Boundary OUT: durable snapshot and the issuing client's presentation view.

import (
	"slices"
	"testing"
)

func TestZoom(t *testing.T) {
	t.Run("Should zoom a solo window in place as the full island and float it back on unzoom", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		rect := opened.Snapshot.Windows["w1"].FloatingRect
		zoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		if len(zoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("zoom on an otherwise empty desktop created a desktop: %+v", zoomed.Snapshot.Desktops)
		}
		requireZoomedIsland(t, zoomed.Snapshot, "w1", "desktop-default")
		w1 := zoomed.Snapshot.Windows["w1"]
		if w1.ReturnAnchor == nil || w1.ReturnAnchor.DesktopID != "desktop-default" || w1.ReturnAnchor.Zoomed {
			t.Fatalf("zoom anchor = %+v", w1.ReturnAnchor)
		}
		unzoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		w1 = unzoomed.Snapshot.Windows["w1"]
		if w1.Zoomed || w1.Placement != WindowPlacementFloating || w1.FloatingRect != rect || w1.ReturnAnchor != nil {
			t.Fatalf("unzoomed window = %+v", w1)
		}
		if len(unzoomed.Snapshot.Desktops[0].Groups) != 0 {
			t.Fatalf("unzoom left an island behind: %+v", unzoomed.Snapshot.Desktops[0].Groups)
		}
		requireValidSnapshot(t, unzoomed.Snapshot)
	})

	t.Run("Should zoom in place over minimized windows", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "desktop-default")
		executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{
			WindowID: "w2", Minimize: true,
		})
		zoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		if len(zoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("a minimized window counted as occupying the desktop: %+v", zoomed.Snapshot.Desktops)
		}
		requireZoomedIsland(t, zoomed.Snapshot, "w1", "desktop-default")
		requireValidSnapshot(t, zoomed.Snapshot)
	})

	t.Run("Should lift a zoomed window to a fresh desktop when its desktop shows another window", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arranged, zoomed := arrangeAndZoom(
			t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks",
		)
		if len(zoomed.Snapshot.Desktops) != 2 || zoomed.Snapshot.Desktops[0].ID != "desktop-default" {
			t.Fatalf("zoom over a tiled neighbor did not add a desktop right after: %+v", zoomed.Snapshot.Desktops)
		}
		lifted := zoomed.Snapshot.Desktops[1]
		requireZoomedIsland(t, zoomed.Snapshot, "tasks", lifted.ID)
		tasks := zoomed.Snapshot.Windows["tasks"]
		if tasks.ReturnAnchor == nil || tasks.ReturnAnchor.DesktopID != "desktop-default" {
			t.Fatalf("lifted window lost its origin: %+v", tasks.ReturnAnchor)
		}
		settings := zoomed.Snapshot.Windows["settings"]
		if settings.DesktopID != "desktop-default" || settings.Placement != WindowPlacementTiled || settings.Zoomed {
			t.Fatalf("neighbor moved with the zoom: %+v", settings)
		}
		if zoomed.Client == nil || zoomed.Client.ActiveDesktopID != lifted.ID ||
			valueOrZero(zoomed.Client.FocusedWindowID) != "tasks" {
			t.Fatalf("client did not follow the zoom: %+v", zoomed.Client)
		}
		unzoomed := executeTestCommand(
			t, environment.manager, "workspace-a", &clientID, ZoomWindowCommand{WindowID: "tasks"},
		)
		if len(unzoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("unzoom left the lifted desktop behind: %+v", unzoomed.Snapshot.Desktops)
		}
		requireExactGroup(t, unzoomed.Snapshot, "desktop-default", arranged.Snapshot.Desktops[0].Groups[0])
		tasks = unzoomed.Snapshot.Windows["tasks"]
		if tasks.Zoomed || tasks.ReturnAnchor != nil || tasks.DesktopID != "desktop-default" {
			t.Fatalf("returned window = %+v", tasks)
		}
		if unzoomed.Client == nil || unzoomed.Client.ActiveDesktopID != "desktop-default" {
			t.Fatalf("client did not follow the unzoom home: %+v", unzoomed.Client)
		}
		requireValidSnapshot(t, unzoomed.Snapshot)
	})

	t.Run("Should zoom each unit on its own desktop when both zoom", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		both := executeTestCommand(
			t, environment.manager, "workspace-a", &clientID, ZoomWindowCommand{WindowID: "settings"},
		)
		if len(both.Snapshot.Desktops) != 2 {
			t.Fatalf("zooming the window left alone created a desktop: %+v", both.Snapshot.Desktops)
		}
		requireZoomedIsland(t, both.Snapshot, "settings", "desktop-default")
		requireZoomedIsland(t, both.Snapshot, "tasks", zoomed.Snapshot.Desktops[1].ID)
		requireValidSnapshot(t, both.Snapshot)
	})

	t.Run("Should split the zoomed island and keep its desktop when a window drops beside it", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeAndZoom(
			t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks",
		)
		liftedID := zoomed.Snapshot.Desktops[1].ID
		openTestWindow(t, environment.manager, "workspace-a", &clientID, "docs", "desktop-default")
		target := WindowID("tasks")
		split := executeTestCommand(t, environment.manager, "workspace-a", &clientID, MoveWindowCommand{
			WindowID: "docs", DestinationDesktopID: liftedID, TargetWindowID: &target, Placement: DropLeft,
		})
		tasks := split.Snapshot.Windows["tasks"]
		if tasks.Zoomed || tasks.ReturnAnchor != nil {
			t.Fatalf("structural insert left the unit zoomed: %+v", tasks)
		}
		if split.Snapshot.Windows["docs"].Placement != WindowPlacementTiled {
			t.Fatalf("inserted window = %+v", split.Snapshot.Windows["docs"])
		}
		if len(split.Snapshot.Desktops) != 2 || split.Snapshot.Desktops[1].ID != liftedID {
			t.Fatalf("a desktop the user filled disappeared: %+v", split.Snapshot.Desktops)
		}
		requireValidSnapshot(t, split.Snapshot)
	})

	t.Run("Should zoom the whole tab frame in place and float it back on unzoom", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		stacked := createFloatingStack(t, environment.manager, []WindowID{"w1", "w2"})
		rect := stacked.Snapshot.Desktops[0].FloatingStacks[0].Rect
		zoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		if len(zoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("frame zoom created a desktop: %+v", zoomed.Snapshot.Desktops)
		}
		island := requireZoomedIsland(t, zoomed.Snapshot, "w1", "desktop-default")
		if island.Root.Kind != NodeKindStack || !slices.Equal(island.Root.WindowIDs, []WindowID{"w1", "w2"}) {
			t.Fatalf("zoomed frame root = %+v", island.Root)
		}
		if zoomed.Snapshot.Windows["w2"].Zoomed {
			t.Fatal("a non-owner member carried the zoom flag")
		}
		unzoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w2"})
		if unitZoomed(&unzoomed.Snapshot, []WindowID{"w1", "w2"}) {
			t.Fatalf("unzoom through a member left the frame zoomed: %+v", unzoomed.Snapshot.Windows)
		}
		stacks := unzoomed.Snapshot.Desktops[0].FloatingStacks
		if len(stacks) != 1 || stacks[0].Rect != rect || !slices.Equal(stacks[0].WindowIDs, []WindowID{"w1", "w2"}) {
			t.Fatalf("frame did not float back where it was: %+v", stacks)
		}
		requireValidSnapshot(t, unzoomed.Snapshot)
	})

	t.Run("Should group another window into the zoomed frame and return the frame to the split", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		targetID := WindowID("tasks")
		grouped := executeTestCommand(t, environment.manager, "workspace-a", &clientID, OpenWindowCommand{
			Window: WindowSpec{
				ID: "docs", App: "Test", Route: testRoute("/test"), FloatingRect: fullRect(),
				StackTargetWindowID: &targetID,
			},
		})
		members := stackMembersForWindow(t, grouped.Snapshot, "tasks")
		if !slices.Equal(members, []WindowID{"tasks", "docs"}) {
			t.Fatalf("grouped members = %v", members)
		}
		if !unitZoomed(&grouped.Snapshot, members) || grouped.Snapshot.Windows["docs"].Zoomed {
			t.Fatalf("zoom did not survive grouping: %+v", grouped.Snapshot.Windows)
		}
		stackID := stackLocationForWindow(t, grouped.Snapshot, "tasks").id()
		unzoomed := executeTestCommand(
			t, environment.manager, "workspace-a", &clientID, ZoomWindowCommand{WindowID: "docs"},
		)
		if len(unzoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("unzoom left the lifted desktop behind: %+v", unzoomed.Snapshot.Desktops)
		}
		root := unzoomed.Snapshot.Desktops[0].Groups[0].Root
		if root.Kind != NodeKindSplit || len(root.Children) != 2 || root.Children[0].ID != stackID ||
			!slices.Equal(root.Children[0].WindowIDs, []WindowID{"tasks", "docs"}) ||
			valueOrZero(root.Children[1].WindowID) != "settings" {
			t.Fatalf("frame did not return to the tasks slot: %+v", root)
		}
		for _, windowID := range members {
			if window := unzoomed.Snapshot.Windows[windowID]; window.Zoomed || window.DesktopID != "desktop-default" {
				t.Fatalf("returned member %q = %+v", windowID, window)
			}
		}
		requireValidSnapshot(t, unzoomed.Snapshot)
	})

	t.Run("Should end the zoom and keep the desktop when the zoomed window floats", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		floated := executeTestCommand(
			t, environment.manager, "workspace-a", &clientID, ToggleFloatingCommand{WindowID: "tasks"},
		)
		tasks := floated.Snapshot.Windows["tasks"]
		if tasks.Zoomed || tasks.Placement != WindowPlacementFloating ||
			tasks.DesktopID != zoomed.Snapshot.Desktops[1].ID {
			t.Fatalf("floated window = %+v", tasks)
		}
		if len(floated.Snapshot.Desktops) != 2 {
			t.Fatalf("floating on the lifted desktop removed it: %+v", floated.Snapshot.Desktops)
		}
		requireValidSnapshot(t, floated.Snapshot)
	})

	t.Run("Should release the lifted desktop when its zoomed window closes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		closed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			CloseWindowCommand{WindowID: "tasks"},
		)
		if len(closed.Snapshot.Desktops) != 1 || closed.Snapshot.Desktops[0].ID != "desktop-default" {
			t.Fatalf("closing the zoomed window orphaned its desktop: %+v", closed.Snapshot.Desktops)
		}
		if closed.Client == nil || closed.Client.ActiveDesktopID != "desktop-default" {
			t.Fatalf("client stayed on the released desktop: %+v", closed.Client)
		}
		requireValidSnapshot(t, closed.Snapshot)
	})

	t.Run("Should release the lifted desktop when its minimized zoomed window closes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		minimized := executeTestCommand(t, environment.manager, "workspace-a", &clientID, CloseWindowCommand{
			WindowID: "tasks", Minimize: true,
		})
		if len(minimized.Snapshot.Desktops) != 2 {
			t.Fatalf("minimizing released the desktop the window still lives on: %+v", minimized.Snapshot.Desktops)
		}
		closed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			CloseWindowCommand{WindowID: "tasks"},
		)
		if len(closed.Snapshot.Desktops) != 1 {
			t.Fatalf("closing the minimized zoomed window orphaned its desktop: %+v", closed.Snapshot.Desktops)
		}
		requireValidSnapshot(t, closed.Snapshot)
	})

	t.Run(
		"Should re-zoom a minimized lifted window on its desktop and still take it home on unzoom",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			clientID := ClientID("client-a")
			arranged, zoomed := arrangeAndZoom(
				t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks",
			)
			liftedID := zoomed.Snapshot.Desktops[1].ID
			minimized := executeTestCommand(t, environment.manager, "workspace-a", &clientID, CloseWindowCommand{
				WindowID: "tasks", Minimize: true,
			})
			tasks := minimized.Snapshot.Windows["tasks"]
			if !tasks.Minimized || tasks.Zoomed || tasks.DesktopID != liftedID || tasks.ReturnAnchor == nil ||
				!tasks.ReturnAnchor.Zoomed || tasks.ReturnAnchor.DesktopID != "desktop-default" {
				t.Fatalf("minimized zoomed window = %+v", tasks)
			}
			restored := executeTestCommand(t, environment.manager, "workspace-a", &clientID, OpenWindowCommand{
				RestoreWindowID: new(WindowID("tasks")),
			})
			requireZoomedIsland(t, restored.Snapshot, "tasks", liftedID)
			tasks = restored.Snapshot.Windows["tasks"]
			if tasks.ReturnAnchor == nil || tasks.ReturnAnchor.Zoomed ||
				tasks.ReturnAnchor.DesktopID != "desktop-default" {
				t.Fatalf("restore forgot the zoom origin: %+v", tasks.ReturnAnchor)
			}
			if restored.Client == nil || restored.Client.ActiveDesktopID != liftedID ||
				valueOrZero(restored.Client.FocusedWindowID) != "tasks" {
				t.Fatalf("restore client = %+v", restored.Client)
			}
			unzoomed := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				ZoomWindowCommand{WindowID: "tasks"},
			)
			if len(unzoomed.Snapshot.Desktops) != 1 {
				t.Fatalf("unzoom after restore left the lifted desktop behind: %+v", unzoomed.Snapshot.Desktops)
			}
			requireExactGroup(t, unzoomed.Snapshot, "desktop-default", arranged.Snapshot.Desktops[0].Groups[0])
			requireValidSnapshot(t, unzoomed.Snapshot)
		},
	)

	t.Run("Should restore a minimized zoom onto a fresh desktop after its prior desktop fills", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeAndZoom(
			t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks",
		)
		priorDesktopID := zoomed.Snapshot.Windows["tasks"].DesktopID
		executeTestCommand(t, environment.manager, "workspace-a", &clientID, CloseWindowCommand{
			WindowID: "tasks", Minimize: true,
		})
		openTestWindow(t, environment.manager, "workspace-a", nil, "docs", priorDesktopID)
		executeTestCommand(t, environment.manager, "workspace-a", nil, ArrangeLayoutCommand{
			DesktopID: priorDesktopID, WindowIDs: []WindowID{"docs"},
			Arrangement: ArrangementHorizontal, Frame: fullRect(), GroupID: "group-docs",
		})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "docs"})

		restored := executeTestCommand(t, environment.manager, "workspace-a", &clientID, OpenWindowCommand{
			RestoreWindowID: new(WindowID("tasks")),
		})
		tasks := restored.Snapshot.Windows["tasks"]
		if tasks.DesktopID == priorDesktopID {
			t.Fatalf("restored zoom covered the prior desktop: %+v", tasks)
		}
		requireZoomedIsland(t, restored.Snapshot, "tasks", tasks.DesktopID)
		docs := restored.Snapshot.Windows["docs"]
		if docs.Zoomed || docs.DesktopID != priorDesktopID {
			t.Fatalf("prior zoom did not return before restore placement: %+v", docs)
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run("Should float the returning window when its origin neighbors are no longer tiled", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arrangeAndZoom(t, environment, "workspace-a", clientID, []WindowID{"tasks", "settings"}, "tasks")
		executeTestCommand(t, environment.manager, "workspace-a", nil, ToggleFloatingCommand{WindowID: "settings"})
		unzoomed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		tasks := unzoomed.Snapshot.Windows["tasks"]
		if tasks.Zoomed || tasks.DesktopID != "desktop-default" || tasks.Placement != WindowPlacementFloating {
			t.Fatalf("returned window = %+v", tasks)
		}
		if len(unzoomed.Snapshot.Desktops) != 1 {
			t.Fatalf("unzoom left the lifted desktop behind: %+v", unzoomed.Snapshot.Desktops)
		}
		requireValidSnapshot(t, unzoomed.Snapshot)
	})

	t.Run("Should keep a zoomed frame zoomed when its zoomed tab closes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		createFloatingStack(t, environment.manager, []WindowID{"w1", "w2", "w3"})
		executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		closed := executeTestCommand(t, environment.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "w1"})
		if _, exists := closed.Snapshot.Windows["w1"]; exists {
			t.Fatal("closed window survived")
		}
		ownerID, zoomed := zoomOwner(&closed.Snapshot, []WindowID{"w2", "w3"})
		if !zoomed {
			t.Fatalf("frame lost its zoom with the closed tab: %+v", closed.Snapshot.Windows)
		}
		if closed.Snapshot.Windows[ownerID].ReturnAnchor == nil {
			t.Fatalf("heir %q did not inherit the return slot", ownerID)
		}
		requireValidSnapshot(t, closed.Snapshot)
	})

	t.Run("Should zoom without a client so agents can zoom from the CLI", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		zoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		if !zoomed.Applied || !zoomed.Snapshot.Windows["w1"].Zoomed {
			t.Fatalf("clientless zoom = %+v", zoomed.Snapshot.Windows["w1"])
		}
	})

	t.Run("Should restore and zoom a minimized window in one zoom command", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "w1", Minimize: true},
		)
		zoomed := executeTestCommand(t, environment.manager, "workspace-a", nil, ZoomWindowCommand{WindowID: "w1"})
		w1 := zoomed.Snapshot.Windows["w1"]
		if w1.Minimized || !w1.Zoomed {
			t.Fatalf("zoom of a minimized window = %+v", w1)
		}
		requireZoomedIsland(t, zoomed.Snapshot, "w1", "desktop-default")
		requireValidSnapshot(t, zoomed.Snapshot)
	})
}

// arrangeAndZoom tiles the windows on the default desktop, registers a client
// focused on the target, and zooms the target through that client.
func arrangeAndZoom(
	t *testing.T,
	environment testEnvironment,
	workspaceID WorkspaceID,
	clientID ClientID,
	windowIDs []WindowID,
	target WindowID,
) (Result, Result) {
	t.Helper()
	for _, windowID := range windowIDs {
		openTestWindow(t, environment.manager, workspaceID, nil, windowID, "desktop-default")
	}
	arranged := executeTestCommand(t, environment.manager, workspaceID, nil, ArrangeLayoutCommand{
		DesktopID: "desktop-default", WindowIDs: windowIDs, Arrangement: ArrangementHorizontal,
		Frame: fullRect(), GroupID: "group-main",
	})
	registerTestClient(t, environment.manager, workspaceID, clientID)
	executeTestCommand(t, environment.manager, workspaceID, &clientID, FocusWindowCommand{WindowID: &target})
	zoomed := executeTestCommand(t, environment.manager, workspaceID, &clientID, ZoomWindowCommand{WindowID: target})
	return arranged, zoomed
}

// requireZoomedIsland asserts the window is zoomed as the only full-frame
// island of the desktop and returns that island.
func requireZoomedIsland(t *testing.T, snapshot Snapshot, windowID WindowID, desktopID DesktopID) LayoutGroup {
	t.Helper()
	window := snapshot.Windows[windowID]
	if !window.Zoomed || window.Minimized || window.DesktopID != desktopID {
		t.Fatalf("window %q is not zoomed on %q: %+v", windowID, desktopID, window)
	}
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		t.Fatalf("desktop %q missing: %+v", desktopID, snapshot.Desktops)
	}
	groups := snapshot.Desktops[desktopIndex].Groups
	if len(groups) != 1 || groups[0].Frame != fullRect() || !containsWindowID(nodeWindowIDs(groups[0].Root), windowID) {
		t.Fatalf("zoomed unit is not the only full island of %q: %+v", desktopID, groups)
	}
	return groups[0]
}

func stackLocationForWindow(t *testing.T, snapshot Snapshot, windowID WindowID) stackLocation {
	t.Helper()
	location, found := findStackByWindow(&snapshot, windowID)
	if !found {
		t.Fatalf("window %q is not stacked in %+v", windowID, snapshot)
	}
	return location
}
