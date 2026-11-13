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
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

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
				Arrangement: ArrangementHorizontal,
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
		result := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{WindowID: "w1", DestinationDesktopID: "desktop-default", Placement: DropTab},
		)
		root := result.Snapshot.Desktops[0].Tabs[1].Root
		if root.Kind != NodeKindLeaf || valueOrZero(root.WindowID) != "w2" ||
			len(result.Snapshot.Windows["w1"].NavStack) != 1 {
			t.Fatalf("drag-out result = %+v", result.Snapshot)
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
					(client.ActiveDesktopID != "desktop-default" || valueOrZero(client.FocusedWindowID) != "win-newtab-desktop-default") {
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

func TestWorkspaceBindingRetirement(t *testing.T) {
	t.Run(
		"Should retire bound windows and prevent navigation history and reopen from restoring them",
		func(t *testing.T) {
			t.Parallel()
			env := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			opened := openTestWindow(t, env.manager, "workspace-a", nil, "bound", "desktop-default")
			route := testRoute("/loops/runs/run-one")
			route.Search["workspace"] = json.RawMessage(`"workspace-b"`)
			executeTestCommand(
				t,
				env.manager,
				"workspace-a",
				nil,
				NavigateWindowCommand{WindowID: "bound", Route: route},
			)
			openTestWindow(t, env.manager, "workspace-a", nil, "closed", "desktop-default")
			executeTestCommand(
				t,
				env.manager,
				"workspace-a",
				nil,
				NavigateWindowCommand{WindowID: "closed", Route: route},
			)
			executeTestCommand(t, env.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "closed"})
			before := mustSnapshot(t, env.manager, "workspace-a")
			sub, err := env.manager.Subscribe(
				t.Context(),
				SubscriptionRequest{WorkspaceID: "workspace-a", AfterRevision: before.Revision},
			)
			if err != nil {
				t.Fatal(err)
			}
			sub = trackSubscription(t, sub)
			if err := env.manager.ReconcileUnregisteredWorkspace(
				t.Context(),
				"workspace-a",
				"workspace-b",
			); err != nil {
				t.Fatal(err)
			}
			snapshot, err := env.manager.Snapshot(t.Context(), "workspace-a")
			if err != nil {
				t.Fatal(err)
			}
			update := <-sub.Updates()
			if update.Event == nil || update.Event.CommandID != CommandWindowNavigate {
				t.Fatalf("event = %#v", update)
			}
			if len(snapshot.ClosedEntries) != 0 {
				t.Fatalf("retained closed entries = %#v", snapshot.ClosedEntries)
			}
			for _, retained := range snapshot.Windows {
				if windowReferencesWorkspace(retained, "workspace-b") {
					t.Fatalf("retained binding = %#v", retained)
				}
			}
			window := snapshot.Windows["bound"]
			if window.Route.Pathname != "/loops" || len(window.Route.Search) != 0 || len(snapshot.History.Undo) != 0 ||
				len(snapshot.History.Redo) != 0 {
				t.Fatalf("retirement = %#v (opened revision %d)", snapshot, opened.Snapshot.Revision)
			}
		},
	)
}

// Invariant: terminal retirement repairs client presentation without reversible close history.

// TestTabsAndSplits owns tab lifecycle invariants at the semantic-command layer.
func TestTabsAndSplits(t *testing.T) {
	t.Run("Should open beside caller active tab (UT-001)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		client := ClientID("caller")
		registerTestClient(t, e.manager, "workspace-a", client)
		executeTestCommand(t, e.manager, "workspace-a", &client, ActivateTabCommand{WindowID: "a"})
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			&client,
			OpenWindowCommand{Window: WindowSpec{App: "Test", Route: testRoute("/new")}},
		)
		ids := r.Snapshot.Desktops[0].Tabs
		if len(ids) != 4 || valueOrZero(ids[0].Root.WindowID) != "a" || valueOrZero(ids[2].Root.WindowID) != "b" ||
			r.Client == nil ||
			r.Client.ActiveTab["desktop-default"] != ids[1].ID ||
			valueOrZero(r.Client.FocusedWindowID) != valueOrZero(ids[1].Root.WindowID) {
			t.Fatalf("open=%+v", r)
		}
	})
	t.Run("Should explicit and policy split (UT-002)", func(t *testing.T) {
		t.Parallel()
		for _, explicit := range []bool{false, true} {
			config := besideFocusConfig()
			if explicit {
				config = DefaultConfig()
			}
			e := newTestEnvironment(t, config, "workspace-a")
			client := ClientID("caller")
			registerTestClient(t, e.manager, "workspace-a", client)
			r := executeTestCommand(
				t,
				e.manager,
				"workspace-a",
				&client,
				OpenWindowCommand{
					Window: WindowSpec{ID: "a", App: "Test", Route: testRoute("/new"), InsertTiled: explicit},
				},
			)
			root := tabFor(t, r.Snapshot, "a").Root
			if valueOrZero(root.Axis) != AxisHorizontal || valueOrZero(root.Children[1].WindowID) != "a" ||
				!slices.Equal(root.Weights, []float64{.5, .5}) {
				t.Fatalf("split open=%+v", root)
			}
		}
	})
	t.Run("Should reuse single instance (UT-003)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		client := ClientID("caller")
		registerTestClient(t, e.manager, "workspace-a", client)
		before := mustSnapshot(t, e.manager, "workspace-a")
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			&client,
			OpenWindowCommand{RestoreWindowID: new(WindowID("a"))},
		)
		if r.Applied || r.Snapshot.Revision != before.Revision ||
			r.Client.ActiveTab["desktop-default"] != before.Windows["a"].TabID {
			t.Fatalf("reuse=%+v", r)
		}
	})
	t.Run("Should open without client (UT-004)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, ActivateTabCommand{WindowID: "b"})
		client := registerTestClient(t, e.manager, "workspace-a", "peer")
		r := openTestWindow(t, e.manager, "workspace-a", nil, "new", "desktop-default")
		ids := nodeWindowIDs(r.Snapshot.Desktops[0].Tabs[2].Root)
		peers, _ := e.manager.Clients(t.Context(), "workspace-a")
		if !slices.Equal(ids, []WindowID{"new"}) || !reflect.DeepEqual(client.ActiveTab, peers[0].ActiveTab) {
			t.Fatalf("open=%+v clients=%+v", r, peers)
		}
	})
	t.Run("Should right then left activation (UT-005)", func(t *testing.T) {
		t.Parallel()
		for _, id := range []WindowID{"b", "c"} {
			e := threeTabs(t)
			client := ClientID("caller")
			registerTestClient(t, e.manager, "workspace-a", client)
			executeTestCommand(t, e.manager, "workspace-a", &client, ActivateTabCommand{WindowID: id})
			r := executeTestCommand(t, e.manager, "workspace-a", &client, CloseWindowCommand{WindowID: id})
			want := WindowID("c")
			if id == "c" {
				want = "b"
			}
			if valueOrZero(r.Client.FocusedWindowID) != want {
				t.Fatalf("close %s focus=%v", id, r.Client.FocusedWindowID)
			}
		}
	})
	t.Run("Should close full split tab (UT-006)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b")
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
		before := tabFor(t, mustSnapshot(t, e.manager, "workspace-a"), "a")
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			CloseWindowCommand{WindowID: "a", Scope: CloseScopeGroup},
		)
		entry := r.Snapshot.ClosedEntries[len(r.Snapshot.ClosedEntries)-1]
		if len(entry.Tabs) != 1 || len(entry.Windows) != 2 || !entry.Tabs[0].Pinned ||
			!reflect.DeepEqual(entry.Tabs[0].Root, before.Root) {
			t.Fatalf("entry=%+v", entry)
		}
	})
	t.Run("Should scoped close skips pins (UT-007)", func(t *testing.T) {
		t.Parallel()
		for _, scope := range []CloseScope{CloseScopeOthers, CloseScopeRight, CloseScopeLeft, CloseScopeAll} {
			e := threeTabs(t)
			executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
			before := mustSnapshot(t, e.manager, "workspace-a")
			r := executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b", Scope: scope})
			if _, ok := r.Snapshot.Windows["a"]; !ok {
				t.Fatal("pinned tab closed")
			}
			want := []WindowID{"c"}
			switch scope {
			case CloseScopeLeft:
				want = nil
			case CloseScopeAll:
				want = []WindowID{"b", "c"}
			}
			if len(want) == 0 {
				if r.Applied {
					t.Fatal("empty close applied")
				}
				continue
			}
			entry := r.Snapshot.ClosedEntries[len(r.Snapshot.ClosedEntries)-1]
			if len(r.Snapshot.ClosedEntries) != len(before.ClosedEntries)+1 || len(entry.Windows) != len(want) {
				t.Fatalf("scope %s entry=%+v", scope, entry)
			}
			for _, id := range want {
				if _, ok := r.Snapshot.Windows[id]; ok {
					t.Fatalf("%s remains", id)
				}
			}
		}
	})
	t.Run("Should pinned solo refuses member allowed (UT-008)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
		s := mustSnapshot(t, e.manager, "workspace-a")
		_, err := e.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: s.Revision,
				Payload:          CloseWindowCommand{WindowID: "a"},
			},
		)
		if !errors.Is(err, ErrWindowPinned) {
			t.Fatalf("pinned close=%v", err)
		}
		moveTestWindow(t, e, "b", "a", DropRight)
		r := executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b"})
		if !tabFor(t, r.Snapshot, "a").Pinned {
			t.Fatal("pin lost")
		}
	})
	t.Run("Should activation outside history and alias (UT-010)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		before := mustSnapshot(t, e.manager, "workspace-a")
		for _, command := range []Command{ActivateTabCommand{WindowID: "a"}, SetStackActiveCommand{WindowID: "b"}} {
			r := executeTestCommand(t, e.manager, "workspace-a", nil, command)
			if r.Snapshot.Desktops[0].ActiveTabID != r.Snapshot.Windows[map[bool]WindowID{true: "a", false: "b"}[command.CommandID() == CommandWindowTabActivate]].TabID ||
				len(r.Snapshot.History.Undo) != len(before.History.Undo) {
				t.Fatalf("activation=%+v", r)
			}
		}
	})
	t.Run("Should reorder clamps pin ranges and alias (UT-011)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "b", Pinned: true})
		for _, command := range []Command{ReorderTabCommand{WindowID: "c", Index: -100}, ReorderStackCommand{WindowID: "a", Index: 100}} {
			r := executeTestCommand(t, e.manager, "workspace-a", nil, command)
			if !r.Snapshot.Desktops[0].Tabs[0].Pinned || !r.Snapshot.Desktops[0].Tabs[1].Pinned ||
				r.Snapshot.Desktops[0].Tabs[2].Pinned {
				t.Fatalf("order=%+v", r.Snapshot.Desktops[0].Tabs)
			}
		}
	})
	t.Run("Should pin prefix (UT-012)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "b", Pinned: true})
		r := executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "c", Pinned: true})
		if valueOrZero(r.Snapshot.Desktops[0].Tabs[1].Root.WindowID) != "c" {
			t.Fatal("pin not at prefix end")
		}
		r = executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "b", Pinned: false})
		if valueOrZero(r.Snapshot.Desktops[0].Tabs[1].Root.WindowID) != "b" {
			t.Fatal("unpin not first regular")
		}
	})
	t.Run("Should restore index and append (UT-013)", func(t *testing.T) {
		t.Parallel()
		for _, id := range []WindowID{"b", "c"} {
			e := threeTabs(t)
			before := mustSnapshot(t, e.manager, "workspace-a")
			executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: id})
			r := executeTestCommand(t, e.manager, "workspace-a", nil, ReopenCommand{})
			if !reflect.DeepEqual(r.Snapshot.Desktops[0].Tabs, before.Desktops[0].Tabs) ||
				r.Snapshot.Desktops[0].ActiveTabID != r.Snapshot.Windows[id].TabID {
				t.Fatalf("reopen=%+v", r.Snapshot.Desktops)
			}
		}
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b"})
		s := mustSnapshot(t, e.manager, "workspace-a")
		s.ClosedEntries[len(s.ClosedEntries)-1].Tabs[0].Index = 100
		seedTestSnapshot(t, e, s)
		r := executeTestCommand(t, e.manager, "workspace-a", nil, ReopenCommand{})
		tabs := r.Snapshot.Desktops[0].Tabs
		if valueOrZero(tabs[len(tabs)-1].Root.WindowID) != "b" {
			t.Fatalf("clamped reopen = %+v", tabs)
		}
	})
	t.Run("Should restore split geometry and pin (UT-014)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b")
		executeTestCommand(t, e.manager, "workspace-a", nil, PinWindowCommand{WindowID: "a", Pinned: true})
		before := tabFor(t, mustSnapshot(t, e.manager, "workspace-a"), "a")
		executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "a", Scope: CloseScopeGroup})
		r := executeTestCommand(t, e.manager, "workspace-a", nil, ReopenCommand{})
		if !reflect.DeepEqual(tabFor(t, r.Snapshot, "a"), before) {
			t.Fatal("split tab was not restored exactly")
		}
	})
	t.Run("Should member anchors and fallbacks (UT-015)", func(t *testing.T) {
		t.Parallel()
		for _, mode := range []string{"parent", "neighbor", "tab", "gone"} {
			e := threeTabs(t)
			ids := []WindowID{"a", "b"}
			if mode == "parent" {
				ids = append(ids, "c")
			}
			splitTestWindows(t, e, ids...)
			executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b"})
			if mode == "tab" || mode == "gone" {
				executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "a"})
				s := mustSnapshot(t, e.manager, "workspace-a")
				s.ClosedEntries = s.ClosedEntries[:len(s.ClosedEntries)-1]
				if mode == "tab" {
					d := &s.Desktops[0]
					tab := Tab{
						ID:   s.ClosedEntries[len(s.ClosedEntries)-1].Member.TabID,
						Root: LayoutNode{ID: "replacement", Kind: NodeKindLeaf, WindowID: new(WindowID("c"))},
					}
					removeWindow(&s, "c")
					insertTab(d, tab, 0)
					assignTabWindows(&s, d, tab)
				}
				seedTestSnapshot(t, e, s)
			}
			r := executeTestCommand(t, e.manager, "workspace-a", nil, ReopenCommand{})
			requireValidSnapshot(t, r.Snapshot)
			if _, ok := r.Snapshot.Windows["b"]; !ok {
				t.Fatal("member lost")
			}
			if mode == "parent" || mode == "neighbor" {
				tab := tabFor(t, r.Snapshot, "a")
				if !slices.Equal(nodeWindowIDs(tab.Root), ids) {
					t.Fatalf("restored=%+v", tab.Root)
				}
			}
		}
	})
	t.Run("Should missing desktop fallback (UT-016)", func(t *testing.T) {
		t.Parallel()
		for _, withClient := range []bool{false, true} {
			e := threeTabs(t)
			executeTestCommand(t, e.manager, "workspace-a", nil, CreateDesktopCommand{DesktopID: "d2", Name: "Two"})
			var client *ClientID
			want := DesktopID("desktop-default")
			if withClient {
				client = new(ClientID("caller"))
				registerTestClient(t, e.manager, "workspace-a", *client)
				executeTestCommand(t, e.manager, "workspace-a", client, SwitchDesktopCommand{DesktopID: "d2"})
				want = "d2"
			}
			executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "b"})
			s := mustSnapshot(t, e.manager, "workspace-a")
			s.ClosedEntries[len(s.ClosedEntries)-1].DesktopID = "gone"
			s.ClosedEntries[len(s.ClosedEntries)-1].Windows[0].DesktopID = "gone"
			seedTestSnapshot(t, e, s)
			r := executeTestCommand(t, e.manager, "workspace-a", client, ReopenCommand{})
			if r.Snapshot.Windows["b"].DesktopID != want {
				t.Fatalf("fallback desktop = %s, want %s", r.Snapshot.Windows["b"].DesktopID, want)
			}
		}
	})
	t.Run("Should collapse records neighbor (UT-027)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b")
		r := executeTestCommand(t, e.manager, "workspace-a", nil, CloseWindowCommand{WindowID: "a"})
		entry := r.Snapshot.ClosedEntries[len(r.Snapshot.ClosedEntries)-1]
		if entry.Member == nil || entry.Member.NeighborID != "b" || entry.Member.NeighborEdge != Edge(DropLeft) ||
			tabFor(t, r.Snapshot, "b").Root.Kind != NodeKindLeaf {
			t.Fatalf("member=%+v", entry)
		}
	})
	t.Run("Should move whole tab across desktops (UT-038)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		splitTestWindows(t, e, "a", "b", "c")
		executeTestCommand(t, e.manager, "workspace-a", nil, CreateDesktopCommand{DesktopID: "d2", Name: "Two"})
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{WindowID: "a", DestinationDesktopID: "d2", MoveGroup: true, Placement: DropTab},
		)
		if len(r.Snapshot.Desktops[0].Tabs) != 1 {
			t.Fatal("empty source has no New Tab")
		}
		for _, id := range []WindowID{"a", "b", "c"} {
			if r.Snapshot.Windows[id].DesktopID != "d2" {
				t.Fatal("member not moved")
			}
		}
	})
	t.Run("Should desktop deletion (UT-039)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		executeTestCommand(t, e.manager, "workspace-a", nil, CreateDesktopCommand{DesktopID: "d2", Name: "Two"})
		s := mustSnapshot(t, e.manager, "workspace-a")
		_, err := e.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: s.Revision,
				Payload:          DeleteDesktopCommand{DesktopID: "desktop-default"},
			},
		)
		if !errors.Is(err, ErrDestinationRequired) {
			t.Fatalf("delete=%v", err)
		}
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			DeleteDesktopCommand{DesktopID: "desktop-default", DestinationID: new(DesktopID("d2"))},
		)
		if len(r.Snapshot.Desktops[0].Tabs) != 4 {
			t.Fatal("destination did not append tabs")
		}
		_, err = e.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: r.Snapshot.Revision,
				Payload:          DeleteDesktopCommand{DesktopID: "d2"},
			},
		)
		if !errors.Is(err, ErrFinalDesktop) {
			t.Fatalf("final desktop=%v", err)
		}
	})
}

func seedTestSnapshot(t *testing.T, e testEnvironment, s Snapshot) {
	t.Helper()
	requireValidSnapshot(t, s)
	e.repository.mu.Lock()
	e.repository.snapshots[s.WorkspaceID] = cloneSnapshot(s)
	e.repository.mu.Unlock()
}

func TestLifecycleTabRepair(t *testing.T) {
	t.Run("Should repair empty tabs and collapse splits for lifecycle removals (UT-009)", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []string{"session", "terminal", "workspace"} {
			for _, split := range []bool{false, true} {
				e := newTestEnvironment(t, DefaultConfig(), "workspace-a")
				s := DefaultSnapshot("workspace-a")
				id := WindowID("bound")
				tab := s.Desktops[0].Tabs[0]
				old := *tab.Root.WindowID
				delete(s.Windows, old)
				tab.Root.WindowID = new(id)
				app, path := "session", "/sessions/gone"
				if kind == "terminal" {
					app, path = "terminal", "/terminal/gone"
				}
				if kind == "workspace" {
					app, path = "files", "/files/gone"
				}
				s.Windows[id] = Window{
					ID:        id,
					App:       app,
					Route:     testRoute(path),
					NavStack:  []RouteIntent{},
					DesktopID: "desktop-default",
					TabID:     tab.ID,
				}
				s.Desktops[0].Tabs[0] = tab
				seedTestSnapshot(t, e, s)
				if split {
					executeTestCommand(
						t,
						e.manager,
						"workspace-a",
						nil,
						OpenWindowCommand{
							Window: WindowSpec{ID: "other", App: "Test", Route: testRoute("/other"), InsertTiled: true},
						},
					)
				}
				var err error
				switch kind {
				case "session":
					err = e.manager.ReconcileDeletedSession(t.Context(), "workspace-a", "gone")
				case "terminal":
					err = e.manager.RetireTerminals(t.Context(), "workspace-a", map[string]bool{"gone": true})
				case "workspace":
					err = e.manager.ReconcileUnregisteredWorkspace(t.Context(), "workspace-a", "gone")
				}
				if err != nil {
					t.Fatal(err)
				}
				got := mustSnapshot(t, e.manager, "workspace-a")
				requireValidSnapshot(t, got)
				if _, ok := got.Windows[id]; ok {
					t.Fatal("retired window retained")
				}
				if len(got.Desktops[0].Tabs) != 1 || got.Desktops[0].Tabs[0].Root.Kind != NodeKindLeaf {
					t.Fatalf("repair=%+v", got)
				}
				if !split {
					for _, w := range got.Windows {
						if w.App != "new-tab" {
							t.Fatalf("default=%+v", w)
						}
					}
				}
			}
		}
	})
}

func TestRetiredWindowCommands(t *testing.T) {
	t.Run("Should open a floating request as a tab under beside focus policy (UT-058)", func(t *testing.T) {
		t.Parallel()
		e := newTestEnvironment(t, besideFocusConfig(), "workspace-a")
		client := ClientID("caller")
		registerTestClient(t, e.manager, "workspace-a", client)
		r := executeTestCommand(t, e.manager, "workspace-a", &client, OpenWindowCommand{Window: WindowSpec{
			ID: "solo", App: "Test", Route: testRoute("/solo"), Floating: true, InsertTiled: true,
		}})
		if len(r.Snapshot.Desktops[0].Tabs) != 2 || tabFor(t, r.Snapshot, "solo").Root.Kind != NodeKindLeaf {
			t.Fatalf("floating request did not open a tab: %+v", r.Snapshot)
		}
	})

	t.Run("Should accept retired commands and fields at the boundary (UT-058)", func(t *testing.T) {
		t.Parallel()
		e := threeTabs(t)
		for _, command := range []Command{ToggleFloatingCommand{WindowID: "a"}, ZoomWindowCommand{WindowID: "a"}, ResizeWindowCommand{WindowID: "a", Frame: fullRect()}, FrameResizeLayoutCommand{}, CloseWindowCommand{WindowID: "a", Minimize: true}} {
			before := mustSnapshot(t, e.manager, "workspace-a")
			r := executeTestCommand(t, e.manager, "workspace-a", nil, command)
			if r.Applied || len(r.Deprecations) != 1 || !reflect.DeepEqual(r.Snapshot, before) {
				t.Fatalf("retired %s=%+v", command.CommandID(), r)
			}
		}
		r := executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			OpenWindowCommand{Window: WindowSpec{ID: "d", App: "Test", Route: testRoute("/d"), Floating: true}},
		)
		if tabFor(t, r.Snapshot, "d").Root.Kind != NodeKindLeaf || len(r.Deprecations) != 1 {
			t.Fatalf("floating=%+v", r)
		}
		moveTestWindow(t, e, "b", "a", DropRight)
		r = executeTestCommand(
			t,
			e.manager,
			"workspace-a",
			nil,
			MoveWindowCommand{WindowID: "b", DestinationDesktopID: "desktop-default", Placement: DropFloating},
		)
		if tabFor(t, r.Snapshot, "b").Root.Kind != NodeKindLeaf || len(r.Deprecations) != 1 {
			t.Fatalf("legacy move=%+v", r)
		}
		for _, command := range []Command{ArrangeLayoutCommand{DesktopID: "desktop-default", WindowIDs: []WindowID{"a", "b"}, Arrangement: ArrangementStack}, GroupWindowsCommand{TargetWindowID: "a", WindowIDs: []WindowID{"c", "b"}}} {
			r = executeTestCommand(t, e.manager, "workspace-a", nil, command)
			if len(r.Deprecations) != 1 {
				t.Fatalf("adjacent tabs=%+v", r)
			}
			d := r.Snapshot.Desktops[0]
			a := tabHoldingID(d.Tabs, r.Snapshot.Windows["a"].TabID)
			want := []WindowID{"b"}
			if _, ok := command.(GroupWindowsCommand); ok {
				want = []WindowID{"c", "b"}
			}
			for i, id := range want {
				if valueOrZero(d.Tabs[a+i+1].Root.WindowID) != id {
					t.Fatalf("adjacent=%+v", d)
				}
			}
		}
	})
}
