package windowmanager

// Suite: window-manager lifecycle reducers
// Invariant: desktop and window lifecycle commands preserve every unaffected member and restore deterministic structure.
// Boundary IN: desktop/window/layout reducers plus client-view coordinator.
// Boundary OUT: pointer gesture interpretation and transport payload parsing.

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
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
			var typeError *json.UnmarshalTypeError
			if !errors.As(err, &typeError) || typeError.Field != "search" {
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

	t.Run("Should reuse a persistent focus desktop and restore the source slot", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		arranged := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"w1"},
				Arrangement: ArrangementHorizontal,
				Frame:       fullRect(),
				GroupID:     "group-main",
			},
		)
		sourceGroup := arranged.Snapshot.Desktops[0].Groups[0]
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("w1"))},
		)
		zoomed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "w1"},
		)
		focusDesktopID := zoomed.Snapshot.Windows["w1"].DesktopID
		focusIndex, exists := desktopIndexByID(&zoomed.Snapshot, focusDesktopID)
		if !exists || zoomed.Snapshot.Desktops[focusIndex].Purpose != DesktopPurposeFocus || zoomed.Client == nil ||
			zoomed.Client.ActiveDesktopID != focusDesktopID {
			t.Fatalf("zoom result=%+v", zoomed)
		}
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "w1"},
		)
		if restored.Snapshot.Windows["w1"].DesktopID != "desktop-default" {
			t.Fatalf("restored window=%+v", restored.Snapshot.Windows["w1"])
		}
		requireExactGroup(t, restored.Snapshot, "desktop-default", sourceGroup)
		focusIndex, exists = desktopIndexByID(&restored.Snapshot, focusDesktopID)
		if !exists || len(restored.Snapshot.Desktops[focusIndex].Groups) != 0 {
			t.Fatalf("persistent focus desktop=%+v", restored.Snapshot.Desktops)
		}
		zoomedAgain := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "w1"},
		)
		if zoomedAgain.Snapshot.Windows["w1"].DesktopID != focusDesktopID {
			t.Fatalf(
				"focus desktop was not reused: got=%q want=%q",
				zoomedAgain.Snapshot.Windows["w1"].DesktopID,
				focusDesktopID,
			)
		}
	})

	t.Run("Should restore exact split IDs order and weights when the source is unchanged", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"tasks", "settings"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		arranged := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"tasks", "settings"},
				Arrangement: ArrangementHorizontal,
				Frame:       NormalizedRect{X: 0.1, Y: 0.2, Width: 0.8, Height: 0.6},
				GroupID:     "group-main",
			},
		)
		sourceGroup := arranged.Snapshot.Desktops[0].Groups[0]
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("tasks"))},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)

		requireExactGroup(t, restored.Snapshot, "desktop-default", sourceGroup)
	})

	t.Run(
		"Should restore exact stack identity order and active member when the source is unchanged",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			for _, windowID := range []WindowID{"tasks", "settings", "jobs"} {
				openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
			}
			arranged := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				ArrangeLayoutCommand{
					DesktopID:   "desktop-default",
					WindowIDs:   []WindowID{"tasks", "settings", "jobs"},
					Arrangement: ArrangementStack,
					Frame:       fullRect(),
					GroupID:     "group-main",
				},
			)
			sourceGroup := arranged.Snapshot.Desktops[0].Groups[0]
			clientID := ClientID("client-a")
			registerTestClient(t, environment.manager, "workspace-a", clientID)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				FocusWindowCommand{WindowID: new(WindowID("settings"))},
			)
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				ZoomWindowCommand{WindowID: "settings"},
			)
			restored := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				&clientID,
				ZoomWindowCommand{WindowID: "settings"},
			)

			requireExactGroup(t, restored.Snapshot, "desktop-default", sourceGroup)
			if restored.Snapshot.Windows["settings"].Placement != WindowPlacementStacked {
				t.Fatalf("restored stack window=%+v", restored.Snapshot.Windows["settings"])
			}
		},
	)

	t.Run("Should use deterministic neighbor fallback after the source group changes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		for _, windowID := range []WindowID{"tasks", "settings"} {
			openTestWindow(t, environment.manager, "workspace-a", nil, windowID, "desktop-default")
		}
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			ArrangeLayoutCommand{
				DesktopID:   "desktop-default",
				WindowIDs:   []WindowID{"tasks", "settings"},
				Arrangement: ArrangementHorizontal,
				Frame:       fullRect(),
				GroupID:     "group-main",
			},
		)
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("tasks"))},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "jobs", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{
				WindowID:             "jobs",
				DestinationDesktopID: "desktop-default",
				TargetWindowID:       new(WindowID("settings")),
				Placement:            DropAfter,
			},
		)
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		desktopIndex, exists := desktopIndexByID(&restored.Snapshot, "desktop-default")
		if !exists || len(restored.Snapshot.Desktops[desktopIndex].Groups) != 1 {
			t.Fatalf("restored source desktop=%+v", restored.Snapshot.Desktops)
		}
		members := nodeWindowIDs(restored.Snapshot.Desktops[desktopIndex].Groups[0].Root)
		if !reflect.DeepEqual(members, []WindowID{"settings", "tasks", "jobs"}) {
			t.Fatalf("fallback members=%v", members)
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

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

	t.Run("Should rejoin the source stack after the stack changed while zoomed", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arrangeFocusAndZoom(
			t, environment, "workspace-a", clientID,
			[]WindowID{"tasks", "settings", "jobs"}, ArrangementStack, "settings",
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "docs", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{
				WindowID:             "docs",
				DestinationDesktopID: "desktop-default",
				TargetWindowID:       new(WindowID("tasks")),
				Placement:            DropCenter,
			},
		)
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "settings"},
		)
		desktopIndex, exists := desktopIndexByID(&restored.Snapshot, "desktop-default")
		if !exists || len(restored.Snapshot.Desktops[desktopIndex].Groups) != 1 {
			t.Fatalf("restored source desktop=%+v", restored.Snapshot.Desktops)
		}
		root := restored.Snapshot.Desktops[desktopIndex].Groups[0].Root
		if root.Kind != NodeKindStack || !containsWindowID(root.WindowIDs, "settings") ||
			valueOrZero(root.ActiveID) != "settings" {
			t.Fatalf("zoomed stack member did not rejoin its stack: %+v", root)
		}
		if restored.Snapshot.Windows["settings"].Placement != WindowPlacementStacked {
			t.Fatalf("restored placement=%+v", restored.Snapshot.Windows["settings"])
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run("Should keep windows opened during zoom and graduate the focus desktop", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		arranged, zoomed := arrangeFocusAndZoom(
			t, environment, "workspace-a", clientID,
			[]WindowID{"tasks", "settings"}, ArrangementHorizontal, "tasks",
		)
		sourceGroup := arranged.Snapshot.Desktops[0].Groups[0]
		focusDesktopID := zoomed.Snapshot.Windows["tasks"].DesktopID
		openTestWindow(t, environment.manager, "workspace-a", &clientID, "docs", focusDesktopID)
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		requireExactGroup(t, restored.Snapshot, "desktop-default", sourceGroup)
		focusIndex, exists := desktopIndexByID(&restored.Snapshot, focusDesktopID)
		if !exists {
			t.Fatalf("graduated desktop was deleted: %+v", restored.Snapshot.Desktops)
		}
		graduated := restored.Snapshot.Desktops[focusIndex]
		if graduated.Purpose != DesktopPurposeStandard || graduated.FocusOwner != nil ||
			!containsWindowID(graduated.Floating, "docs") {
			t.Fatalf("focus desktop did not graduate with its windows: %+v", graduated)
		}
		if restored.Snapshot.Windows["docs"].DesktopID != focusDesktopID {
			t.Fatalf("co-resident window moved: %+v", restored.Snapshot.Windows["docs"])
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run("Should return a zoomed window to its source before minimizing and follow it", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeFocusAndZoom(
			t, environment, "workspace-a", clientID,
			[]WindowID{"tasks", "settings"}, ArrangementHorizontal, "tasks",
		)
		focusDesktopID := zoomed.Snapshot.Windows["tasks"].DesktopID
		minimized := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			CloseWindowCommand{WindowID: "tasks", Minimize: true},
		)
		window := minimized.Snapshot.Windows["tasks"]
		if !window.Minimized || window.DesktopID != "desktop-default" {
			t.Fatalf("minimized zoomed window=%+v", window)
		}
		if minimized.Client == nil || minimized.Client.ActiveDesktopID != "desktop-default" {
			t.Fatalf("client stayed on the focus desktop: %+v", minimized.Client)
		}
		focusIndex, exists := desktopIndexByID(&minimized.Snapshot, focusDesktopID)
		if !exists || minimized.Snapshot.Desktops[focusIndex].FocusOwner != nil ||
			len(minimized.Snapshot.Desktops[focusIndex].Groups) != 0 {
			t.Fatalf("focus desktop not reusable after minimize: %+v", minimized.Snapshot.Desktops)
		}
		restored := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			OpenWindowCommand{
				Window: WindowSpec{
					ID:           "tasks",
					App:          "Test",
					Route:        testRoute("/test"),
					DesktopID:    "desktop-default",
					FloatingRect: NormalizedRect{X: 0.2, Y: 0.2, Width: 0.5, Height: 0.5},
				},
				RestoreWindowID: new(WindowID("tasks")),
			},
		)
		desktopIndex, exists := desktopIndexByID(&restored.Snapshot, "desktop-default")
		if !exists || len(restored.Snapshot.Desktops[desktopIndex].Groups) != 1 {
			t.Fatalf("restored desktop=%+v", restored.Snapshot.Desktops)
		}
		members := nodeWindowIDs(restored.Snapshot.Desktops[desktopIndex].Groups[0].Root)
		if restored.Snapshot.Windows["tasks"].Minimized || !containsWindowID(members, "tasks") {
			t.Fatalf("restore did not rejoin the tiled source: members=%v window=%+v",
				members, restored.Snapshot.Windows["tasks"])
		}
		requireValidSnapshot(t, restored.Snapshot)
	})

	t.Run("Should return to the zoom source desktop when closing the zoomed window", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "tasks", "d2")
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: new(WindowID("tasks"))},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ZoomWindowCommand{WindowID: "tasks"},
		)
		closed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			CloseWindowCommand{WindowID: "tasks"},
		)
		if closed.Client == nil || closed.Client.ActiveDesktopID != "d2" {
			t.Fatalf("client did not return to the zoom source desktop: %+v", closed.Client)
		}
		requireValidSnapshot(t, closed.Snapshot)
	})

	t.Run("Should graduate the focus desktop when closing a zoomed window with co-residents", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		clientID := ClientID("client-a")
		_, zoomed := arrangeFocusAndZoom(
			t, environment, "workspace-a", clientID,
			[]WindowID{"tasks", "settings"}, ArrangementHorizontal, "tasks",
		)
		focusDesktopID := zoomed.Snapshot.Windows["tasks"].DesktopID
		openTestWindow(t, environment.manager, "workspace-a", &clientID, "docs", focusDesktopID)
		closed := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			CloseWindowCommand{WindowID: "tasks"},
		)
		if _, exists := closed.Snapshot.Windows["tasks"]; exists {
			t.Fatalf("closed zoomed window still present: %+v", closed.Snapshot.Windows)
		}
		focusIndex, exists := desktopIndexByID(&closed.Snapshot, focusDesktopID)
		if !exists {
			t.Fatalf("focus desktop with co-residents was deleted: %+v", closed.Snapshot.Desktops)
		}
		graduated := closed.Snapshot.Desktops[focusIndex]
		if graduated.Purpose != DesktopPurposeStandard || graduated.FocusOwner != nil ||
			!containsWindowID(graduated.Floating, "docs") {
			t.Fatalf("focus desktop did not graduate with its co-resident: %+v", graduated)
		}
		if closed.Snapshot.Windows["docs"].DesktopID != focusDesktopID {
			t.Fatalf("co-resident window moved: %+v", closed.Snapshot.Windows["docs"])
		}
		if closed.Client == nil || closed.Client.ActiveDesktopID != "desktop-default" {
			t.Fatalf("client did not return to the zoom source desktop: %+v", closed.Client)
		}
		requireValidSnapshot(t, closed.Snapshot)
	})
}

func arrangeFocusAndZoom(
	t *testing.T,
	environment testEnvironment,
	workspaceID WorkspaceID,
	clientID ClientID,
	windowIDs []WindowID,
	arrangement Arrangement,
	focus WindowID,
) (Result, Result) {
	t.Helper()
	for _, windowID := range windowIDs {
		openTestWindow(t, environment.manager, workspaceID, nil, windowID, "desktop-default")
	}
	arranged := executeTestCommand(t, environment.manager, workspaceID, nil, ArrangeLayoutCommand{
		DesktopID:   "desktop-default",
		WindowIDs:   windowIDs,
		Arrangement: arrangement,
		Frame:       fullRect(),
		GroupID:     "group-main",
	})
	registerTestClient(t, environment.manager, workspaceID, clientID)
	executeTestCommand(t, environment.manager, workspaceID, &clientID, FocusWindowCommand{WindowID: new(focus)})
	zoomed := executeTestCommand(t, environment.manager, workspaceID, &clientID, ZoomWindowCommand{WindowID: focus})
	return arranged, zoomed
}

func requireExactGroup(t *testing.T, snapshot Snapshot, desktopID DesktopID, expected LayoutGroup) {
	t.Helper()
	desktopIndex, exists := desktopIndexByID(&snapshot, desktopID)
	if !exists {
		t.Fatalf("desktop %q is missing", desktopID)
	}
	for _, group := range snapshot.Desktops[desktopIndex].Groups {
		if group.ID != expected.ID {
			continue
		}
		if !reflect.DeepEqual(group, expected) {
			t.Fatalf("restored group=%+v want=%+v", group, expected)
		}
		return
	}
	t.Fatalf("group %q is missing", expected.ID)
}
