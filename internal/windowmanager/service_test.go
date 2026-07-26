package windowmanager

// Suite: window-manager service transactions
// Invariant: one accepted durable command creates one isolated revision, history record, commit, and ordered event.
// Boundary IN: manager, repository CAS, history, revision guards, and subscription hub.
// Boundary OUT: transport authorization and bbolt persistence adapter.

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestCommandTransaction(t *testing.T) {
	t.Run(
		"Should commit one revision and history entry while preview, no-op, and rejection write nothing",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			preview, err := environment.manager.Preview(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: 0,
					Payload:          CreateDesktopCommand{Name: "Preview"},
				},
			)
			if err != nil {
				t.Fatalf("Preview() error = %v", err)
			}
			if !preview.Changed || len(environment.repository.Commits("workspace-a")) != 0 {
				t.Fatalf(
					"preview changed=%v commits=%d",
					preview.Changed,
					len(environment.repository.Commits("workspace-a")),
				)
			}
			result := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			commits := environment.repository.Commits("workspace-a")
			if result.Snapshot.Revision != 1 || len(commits) != 1 || len(result.Snapshot.History.Undo) != 1 {
				t.Fatalf(
					"revision=%d commits=%d history=%d",
					result.Snapshot.Revision,
					len(commits),
					len(result.Snapshot.History.Undo),
				)
			}
			if commits[0].Event.CommandID != CommandDesktopCreate || commits[0].Event.Revision != 1 {
				t.Fatalf("event = %+v", commits[0].Event)
			}
			noOp := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				UpdateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			if noOp.Applied || len(environment.repository.Commits("workspace-a")) != 1 {
				t.Fatalf(
					"no-op applied=%v commits=%d",
					noOp.Applied,
					len(environment.repository.Commits("workspace-a")),
				)
			}
			_, err = environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: 0,
					Payload:          UpdateDesktopCommand{DesktopID: "d2", Name: "Stale"},
				},
			)
			if !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("stale Execute() error = %v", err)
			}
			if len(environment.repository.Commits("workspace-a")) != 1 {
				t.Fatalf("rejection commits=%d", len(environment.repository.Commits("workspace-a")))
			}
		},
	)

	t.Run("Should stop changed topology at the wire revision limit while allowing no-op", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		atLimit := initialSnapshot("workspace-a")
		atLimit.Revision = Revision(MaxWireRevision)
		environment.repository.mu.Lock()
		environment.repository.snapshots["workspace-a"] = cloneSnapshot(atLimit)
		environment.repository.mu.Unlock()

		noOpRequest := CommandRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: atLimit.Revision,
			Payload: UpdateDesktopCommand{
				DesktopID: atLimit.Desktops[0].ID,
				Name:      atLimit.Desktops[0].Name,
			},
		}
		preview, err := environment.manager.Preview(t.Context(), noOpRequest)
		if err != nil || preview.Changed || preview.Snapshot.Revision != atLimit.Revision {
			t.Fatalf("Preview(no-op at limit) = %+v, error = %v", preview, err)
		}
		result, err := environment.manager.Execute(t.Context(), noOpRequest)
		if err != nil || result.Applied || result.Snapshot.Revision != atLimit.Revision {
			t.Fatalf("Execute(no-op at limit) = %+v, error = %v", result, err)
		}

		changedRequest := CommandRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: atLimit.Revision,
			Payload:          CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		}
		if _, err := environment.manager.Preview(
			t.Context(),
			changedRequest,
		); !errors.Is(err, ErrTopologyRevisionExhausted) {
			t.Fatalf("Preview(changed at limit) error = %v", err)
		}
		if _, err := environment.manager.Execute(
			t.Context(),
			changedRequest,
		); !errors.Is(err, ErrTopologyRevisionExhausted) {
			t.Fatalf("Execute(changed at limit) error = %v", err)
		}
		if commits := environment.repository.Commits("workspace-a"); len(commits) != 0 {
			t.Fatalf("revision-limit commits = %d, want 0", len(commits))
		}
		stored, err := environment.repository.Load(t.Context(), "workspace-a")
		if err != nil || stored.Revision != atLimit.Revision {
			t.Fatalf("Load(after revision-limit rejection) = %+v, error = %v", stored, err)
		}
	})

	t.Run("Should observe only selected durable commits with isolated event values", func(t *testing.T) {
		t.Parallel()
		observed := make([]Event, 0, 5)
		environment := newTestEnvironmentWithOptions(
			t,
			DefaultConfig(),
			[]WorkspaceID{"workspace-a"},
			WithEventObserver(func(_ context.Context, event Event) {
				observed = append(observed, cloneEvent(event))
				if len(event.Changes.DesktopIDs) > 0 {
					event.Changes.DesktopIDs[0] = "observer-mutated"
				}
			}),
		)

		if _, err := environment.manager.Preview(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a",
			Payload:     CreateDesktopCommand{DesktopID: "preview", Name: "Preview"},
		}); err != nil {
			t.Fatalf("Preview(desktop.create) error = %v", err)
		}
		created, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a",
			Payload:     CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			Actor:       Actor{Kind: "agent", ID: "agent-a"},
			Origin:      "native:desktop.create",
		})
		if err != nil {
			t.Fatalf("Execute(desktop.create) error = %v", err)
		}
		if len(observed) != 1 ||
			observed[0].CommandID != CommandDesktopCreate ||
			observed[0].Actor.ID != "agent-a" ||
			observed[0].Origin != "native:desktop.create" {
			t.Fatalf("desktop.create observations = %#v", observed)
		}
		if len(created.Changes.DesktopIDs) != 1 {
			t.Fatalf("result desktop changes = %v, want one ID", created.Changes.DesktopIDs)
		}
		if got := created.Changes.DesktopIDs[0]; got != "d2" {
			t.Fatalf("result desktop change = %q, want d2", got)
		}
		commits := environment.repository.Commits("workspace-a")
		if len(commits) != 1 || len(commits[0].Event.Changes.DesktopIDs) != 1 {
			t.Fatalf("repository commits = %+v, want one event with one desktop ID", commits)
		}
		if got := commits[0].Event.Changes.DesktopIDs[0]; got != "d2" {
			t.Fatalf("repository event desktop change = %q, want d2", got)
		}

		noOp := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			UpdateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		if noOp.Applied {
			t.Fatal("desktop.update no-op applied = true")
		}
		if _, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: 0,
			Payload:          UpdateDesktopCommand{DesktopID: "d2", Name: "Stale"},
		}); !errors.Is(err, ErrRevisionConflict) {
			t.Fatalf("stale desktop.update error = %v", err)
		}

		client := registerTestClient(t, environment.manager, "workspace-a", "browser-a")
		clientID := client.ClientID
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		openTestWindow(t, environment.manager, "workspace-a", &clientID, "w1", "desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "desktop-default"},
		)
		windowID := WindowID("w1")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			FocusWindowCommand{WindowID: &windowID},
		)
		if len(observed) != 1 {
			t.Fatalf("non-observable commands produced %d observations, want 1", len(observed))
		}

		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			MoveWindowCommand{
				WindowID:             "w1",
				DestinationDesktopID: "d2",
				Placement:            DropFloating,
			},
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			ArrangeLayoutCommand{
				DesktopID:   "d2",
				WindowIDs:   []WindowID{"w1"},
				Arrangement: ArrangementStack,
				Frame:       fullRect(),
				GroupID:     "group-hooks",
			},
		)
		document, err := environment.manager.ExportLayout(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("ExportLayout() error = %v", err)
		}
		document.Desktops[0].Name = "Primary"
		if _, err := environment.manager.ReplaceLayout(t.Context(), ReplaceLayoutRequest{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: documentRevision(t, environment.manager, "workspace-a"),
			Document:         document,
		}); err != nil {
			t.Fatalf("ReplaceLayout() error = %v", err)
		}
		destinationID := DesktopID("desktop-default")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			DeleteDesktopCommand{DesktopID: "d2", DestinationID: &destinationID},
		)

		wantCommands := []CommandID{
			CommandDesktopCreate,
			CommandWindowMove,
			CommandLayoutArrange,
			CommandLayoutReplace,
			CommandDesktopDelete,
		}
		if len(observed) != len(wantCommands) {
			t.Fatalf("observed commands = %#v, want %v", observed, wantCommands)
		}
		for index, want := range wantCommands {
			if observed[index].CommandID != want {
				t.Fatalf("observed[%d].CommandID = %q, want %q", index, observed[index].CommandID, want)
			}
		}
	})

	t.Run("Should observe resource-backed layout application only after commit", func(t *testing.T) {
		t.Parallel()
		document := LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops: []Desktop{{
				ID:       "desktop-default",
				Name:     "Resource Desktop",
				Purpose:  DesktopPurposeStandard,
				Groups:   []LayoutGroup{},
				Floating: []WindowID{},
			}},
			Windows: map[WindowID]Window{},
		}
		registry := &testLayoutRegistry{resources: []LayoutResource{{ID: "focus", Document: document}}}
		repository := NewMemoryRepository()
		observed := make([]Event, 0, 1)
		manager, err := NewService(
			repository,
			NewMemoryWorkspaceResolver("workspace-a"),
			registry,
			DefaultConfig(),
			WithEventObserver(func(_ context.Context, event Event) {
				observed = append(observed, cloneEvent(event))
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v", closeErr)
			}
		})

		if _, err := manager.Preview(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a",
			Payload:     ArrangeLayoutCommand{ResourceID: "focus"},
		}); err != nil {
			t.Fatalf("Preview(resource) error = %v", err)
		}
		if len(observed) != 0 {
			t.Fatalf("preview observations = %d, want 0", len(observed))
		}
		result, err := manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a",
			Payload:     ArrangeLayoutCommand{ResourceID: "focus"},
		})
		if err != nil || !result.Applied {
			t.Fatalf("Execute(resource) = %#v, error = %v", result, err)
		}
		if len(observed) != 1 ||
			observed[0].CommandID != CommandLayoutArrange ||
			observed[0].Revision != 1 {
			t.Fatalf("resource observations = %#v", observed)
		}
	})

	t.Run("Should serialize concurrent commands so exactly one wins the revision", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		start := make(chan struct{})
		errorsChannel := make(chan error, 2)
		var waitGroup sync.WaitGroup
		for _, desktopID := range []DesktopID{"d2", "d3"} {
			waitGroup.Go(func() {
				<-start
				_, executeErr := environment.manager.Execute(
					t.Context(),
					CommandRequest{
						WorkspaceID:      "workspace-a",
						ExpectedRevision: 0,
						Payload:          CreateDesktopCommand{DesktopID: desktopID, Name: string(desktopID)},
					},
				)
				errorsChannel <- executeErr
			})
		}
		close(start)
		waitGroup.Wait()
		close(errorsChannel)
		successes, conflicts := 0, 0
		for executeErr := range errorsChannel {
			switch {
			case executeErr == nil:
				successes++
			case errors.Is(executeErr, ErrRevisionConflict):
				conflicts++
			default:
				t.Fatalf("concurrent Execute() error = %v", executeErr)
			}
		}
		if successes != 1 || conflicts != 1 || len(environment.repository.Commits("workspace-a")) != 1 {
			t.Fatalf(
				"successes=%d conflicts=%d commits=%d",
				successes,
				conflicts,
				len(environment.repository.Commits("workspace-a")),
			)
		}
	})
}

func documentRevision(t *testing.T, manager *Manager, workspaceID WorkspaceID) Revision {
	t.Helper()
	snapshot, err := manager.Snapshot(t.Context(), workspaceID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	return snapshot.Revision
}

func TestHistory(t *testing.T) {
	t.Run(
		"Should undo and redo complete state while clearing redo after a new write and honoring the cap",
		func(t *testing.T) {
			t.Parallel()
			config := DefaultConfig()
			config.HistoryLimit = 2
			environment := newTestEnvironment(t, config, "workspace-a")
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
			third := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d4", Name: "Four"},
			)
			if len(third.Snapshot.History.Undo) != 2 {
				t.Fatalf("undo history length=%d", len(third.Snapshot.History.Undo))
			}
			undone := executeTestCommand(t, environment.manager, "workspace-a", nil, UndoLayoutCommand{})
			if _, exists := desktopIndexByID(&undone.Snapshot, "d4"); exists || len(undone.Snapshot.History.Redo) != 1 {
				t.Fatalf("undo snapshot=%+v", undone.Snapshot.Desktops)
			}
			redone := executeTestCommand(t, environment.manager, "workspace-a", nil, RedoLayoutCommand{})
			if _, exists := desktopIndexByID(&redone.Snapshot, "d4"); !exists {
				t.Fatal("redo did not restore d4")
			}
			executeTestCommand(t, environment.manager, "workspace-a", nil, UndoLayoutCommand{})
			written := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				UpdateDesktopCommand{DesktopID: "d3", Name: "Renamed"},
			)
			if len(written.Snapshot.History.Redo) != 0 {
				t.Fatalf("redo history length=%d", len(written.Snapshot.History.Redo))
			}
			_, err := environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: written.Snapshot.Revision,
					Payload:          RedoLayoutCommand{},
				},
			)
			if !errors.Is(err, ErrHistoryBoundary) {
				t.Fatalf("Redo() error=%v", err)
			}
		},
	)

	t.Run("Should preserve current routes across undo and redo without changing redo history", func(t *testing.T) {
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
				DesktopID: "desktop-default", WindowIDs: []WindowID{"w1", "w2"},
				Arrangement: ArrangementHorizontal, Frame: fullRect(), GroupID: "group-main",
			},
		)
		navigated := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/after-arrange")},
		)
		if len(navigated.Snapshot.History.Undo) != len(arranged.Snapshot.History.Undo) ||
			len(navigated.Snapshot.History.Redo) != len(arranged.Snapshot.History.Redo) {
			t.Fatalf("navigation history = %+v, want %+v", navigated.Snapshot.History, arranged.Snapshot.History)
		}
		undone := executeTestCommand(t, environment.manager, "workspace-a", nil, UndoLayoutCommand{})
		if undone.Snapshot.Windows["w1"].Route.Pathname != "/after-arrange" ||
			len(undone.Snapshot.History.Redo) != 1 {
			t.Fatalf("undo route/history = %+v / %+v", undone.Snapshot.Windows["w1"].Route, undone.Snapshot.History)
		}
		renavigated := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/before-redo")},
		)
		if len(renavigated.Snapshot.History.Redo) != 1 {
			t.Fatalf("navigation cleared redo = %+v", renavigated.Snapshot.History)
		}
		redone := executeTestCommand(t, environment.manager, "workspace-a", nil, RedoLayoutCommand{})
		if redone.Snapshot.Windows["w1"].Route.Pathname != "/before-redo" {
			t.Fatalf("redo route = %+v", redone.Snapshot.Windows["w1"].Route)
		}
	})
}

func TestRevisionRebase(t *testing.T) {
	t.Run(
		"Should reject stale commands unless explicit stable identities make the rebase unambiguous",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			staleRevision := opened.Snapshot.Revision
			executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			_, err := environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: staleRevision,
					Payload:          ToggleFloatingCommand{WindowID: "w1"},
				},
			)
			if !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("unguarded stale error=%v", err)
			}
			windowID := WindowID("w1")
			rebased, err := environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: staleRevision,
					Rebase:           &RebaseGuard{WindowID: &windowID},
					Payload:          ToggleFloatingCommand{WindowID: "w1"},
				},
			)
			if err != nil {
				t.Fatalf("guarded Execute() error=%v", err)
			}
			if rebased.RebasedFrom == nil || *rebased.RebasedFrom != staleRevision || !rebased.Applied {
				t.Fatalf("rebase result=%+v", rebased)
			}
			wrongNode := NodeID("missing")
			_, err = environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: staleRevision,
					Rebase:           &RebaseGuard{WindowID: &windowID, SourceNodeID: &wrongNode},
					Payload:          ToggleFloatingCommand{WindowID: "w1"},
				},
			)
			var conflict *RevisionConflictError
			if !errors.As(err, &conflict) || len(conflict.Details) == 0 {
				t.Fatalf("ambiguous rebase error=%v", err)
			}
		},
	)
}

func TestWorkspaceIsolation(t *testing.T) {
	t.Run("Should keep workspace snapshots, history, and authorization partitions independent", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a", "workspace-b")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "private", Name: "Private"},
		)
		workspaceB, err := environment.manager.Snapshot(t.Context(), "workspace-b")
		if err != nil {
			t.Fatalf("Snapshot(B) error=%v", err)
		}
		if _, exists := desktopIndexByID(&workspaceB, "private"); exists || workspaceB.Revision != 0 {
			t.Fatalf("workspace B leaked state=%+v", workspaceB)
		}
		_, err = environment.manager.Snapshot(t.Context(), "workspace-missing")
		if !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("missing workspace error=%v", err)
		}
		if len(environment.repository.Commits("workspace-b")) != 0 {
			t.Fatalf("workspace B commits=%d", len(environment.repository.Commits("workspace-b")))
		}
	})
}

func TestSubscription(t *testing.T) {
	t.Run("Should atomically fence a client across a concurrent registration refresh", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		created := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		start := make(chan struct{})
		type subscriptionResult struct {
			subscription Subscription
			err          error
		}
		subscriptionResults := make(chan subscriptionResult, 1)
		registrationResults := make(chan error, 1)
		go func() {
			<-start
			subscription, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
				WorkspaceID:   "workspace-a",
				AfterRevision: created.Snapshot.Revision,
				ClientID:      &clientID,
			})
			subscriptionResults <- subscriptionResult{subscription: subscription, err: err}
		}()
		go func() {
			<-start
			_, err := environment.manager.RegisterClient(t.Context(), ClientRegistration{
				WorkspaceID: "workspace-a", ClientID: clientID, ActiveDesktopID: "d2",
			})
			registrationResults <- err
		}()
		close(start)
		subscribed := <-subscriptionResults
		if subscribed.err != nil {
			t.Fatalf("Subscribe() error = %v", subscribed.err)
		}
		if err := <-registrationResults; err != nil {
			t.Fatalf("RegisterClient(refresh) error = %v", err)
		}
		subscribed.subscription = trackSubscription(t, subscribed.subscription)
		fenced := subscribed.subscription.Fence().Client
		if fenced == nil {
			t.Fatal("client fence = nil")
		}
		switch fenced.PresentationRevision {
		case 1:
			update := <-subscribed.subscription.Updates()
			if update.Client == nil || update.Event != nil ||
				update.Client.ActiveDesktopID != "d2" ||
				update.Client.PresentationRevision != 2 {
				t.Fatalf("post-fence refresh update = %+v", update)
			}
		case 2:
			if fenced.ActiveDesktopID != "d2" {
				t.Fatalf("refreshed fence = %+v", fenced)
			}
			requireNoSubscriptionUpdate(t, subscribed.subscription, "pre-fence refresh")
		default:
			t.Fatalf("client fence = %+v", fenced)
		}
	})

	t.Run("Should stream monotonic presentation changes exactly once to only the bound client", func(t *testing.T) {
		t.Parallel()
		observed := 0
		environment := newTestEnvironmentWithOptions(
			t,
			DefaultConfig(),
			[]WorkspaceID{"workspace-a"},
			WithEventObserver(func(context.Context, Event) { observed++ }),
		)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		openTestWindow(t, environment.manager, "workspace-a", nil, "default-window", "desktop-default")
		openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "d2")
		opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w2", "d2")
		observed = 0
		commitsBefore := len(environment.repository.Commits("workspace-a"))
		historyBefore := cloneHistory(opened.Snapshot.History)
		clientA := ClientID("client-a")
		clientB := ClientID("client-b")
		registerTestClient(t, environment.manager, "workspace-a", clientA)
		registerTestClient(t, environment.manager, "workspace-a", clientB)
		boundA, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: opened.Snapshot.Revision, ClientID: &clientA,
		})
		if err != nil {
			t.Fatalf("Subscribe(client-a) error = %v", err)
		}
		boundB, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: opened.Snapshot.Revision, ClientID: &clientB,
		})
		if err != nil {
			t.Fatalf("Subscribe(client-b) error = %v", err)
		}
		topologyOnly, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: opened.Snapshot.Revision,
		})
		if err != nil {
			t.Fatalf("Subscribe(topology) error = %v", err)
		}
		boundA = trackSubscription(t, boundA)
		boundB = trackSubscription(t, boundB)
		topologyOnly = trackSubscription(t, topologyOnly)
		fence := boundA.Fence()
		if fence.Snapshot.Revision != opened.Snapshot.Revision || fence.Client == nil ||
			fence.Client.ClientID != clientA || fence.Client.PresentationRevision != 1 {
			t.Fatalf("client-a fence = %+v", fence)
		}
		if topologyOnly.Fence().Client != nil {
			t.Fatalf("topology-only fence client = %+v", topologyOnly.Fence().Client)
		}

		switched := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientA,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		if !switched.Applied || switched.Snapshot.Revision != opened.Snapshot.Revision ||
			switched.Client == nil || switched.Client.PresentationRevision != 2 {
			t.Fatalf("desktop.switch result = %+v", switched)
		}
		if len(switched.Client.FocusOrder) != 1 || switched.Client.FocusOrder[0] != "w1" ||
			valueOrZero(switched.Client.FocusedWindowID) != "w1" {
			t.Fatalf("desktop.switch client view = %+v, want d2-only MRU with w1 focused", switched.Client)
		}
		switchUpdate := <-boundA.Updates()
		if switchUpdate.Event != nil || switchUpdate.Client == nil ||
			switchUpdate.Client.ClientID != clientA ||
			switchUpdate.Client.ActiveDesktopID != "d2" ||
			switchUpdate.Client.PresentationRevision != 2 {
			t.Fatalf("desktop.switch update = %+v", switchUpdate)
		}
		requireNoSubscriptionUpdate(t, boundA, "duplicate desktop.switch")
		requireNoSubscriptionUpdate(t, boundB, "cross-client desktop.switch")
		requireNoSubscriptionUpdate(t, topologyOnly, "topology-only desktop.switch")

		windowID := WindowID("w2")
		focused := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientA,
			FocusWindowCommand{WindowID: &windowID},
		)
		if !focused.Applied || focused.Snapshot.Revision != opened.Snapshot.Revision ||
			focused.Client == nil || focused.Client.PresentationRevision != 3 {
			t.Fatalf("window.focus result = %+v", focused)
		}
		focusUpdate := <-boundA.Updates()
		if focusUpdate.Event != nil || focusUpdate.Client == nil ||
			valueOrZero(focusUpdate.Client.FocusedWindowID) != windowID ||
			focusUpdate.Client.PresentationRevision != 3 {
			t.Fatalf("window.focus update = %+v", focusUpdate)
		}
		requireNoSubscriptionUpdate(t, boundA, "duplicate window.focus")
		requireNoSubscriptionUpdate(t, boundB, "cross-client window.focus")
		requireNoSubscriptionUpdate(t, topologyOnly, "topology-only window.focus")

		noOp := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientA,
			FocusWindowCommand{WindowID: &windowID},
		)
		if noOp.Applied || noOp.Client == nil || noOp.Client.PresentationRevision != 3 {
			t.Fatalf("no-op window.focus result = %+v", noOp)
		}
		requireNoSubscriptionUpdate(t, boundA, "no-op window.focus")
		if len(environment.repository.Commits("workspace-a")) != commitsBefore ||
			len(noOp.Snapshot.History.Undo) != len(historyBefore.Undo) ||
			len(noOp.Snapshot.History.Redo) != len(historyBefore.Redo) ||
			observed != 0 {
			t.Fatalf(
				"presentation side effects: commits=%d history=%+v observed=%d",
				len(environment.repository.Commits("workspace-a")),
				noOp.Snapshot.History,
				observed,
			)
		}
	})

	t.Run("Should order one topology event before every actually changed client view", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		created := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		clientA := ClientID("client-a")
		clientB := ClientID("client-b")
		clientC := ClientID("client-c")
		for _, clientID := range []ClientID{clientA, clientB} {
			view, err := environment.manager.RegisterClient(
				t.Context(),
				ClientRegistration{
					WorkspaceID:     "workspace-a",
					ClientID:        clientID,
					ActiveDesktopID: "d2",
				},
			)
			if err != nil || view.PresentationRevision != 1 {
				t.Fatalf("RegisterClient(%s) = %+v, error = %v", clientID, view, err)
			}
		}
		registerTestClient(t, environment.manager, "workspace-a", clientC)
		subscriptions := make(map[ClientID]Subscription, 3)
		for _, clientID := range []ClientID{clientA, clientB, clientC} {
			subscription, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
				WorkspaceID: "workspace-a", AfterRevision: created.Snapshot.Revision, ClientID: &clientID,
			})
			if err != nil {
				t.Fatalf("Subscribe(%s) error = %v", clientID, err)
			}
			subscriptions[clientID] = trackSubscription(t, subscription)
		}
		topologyOnly, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: created.Snapshot.Revision,
		})
		if err != nil {
			t.Fatalf("Subscribe(topology) error = %v", err)
		}
		topologyOnly = trackSubscription(t, topologyOnly)

		deleted := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			DeleteDesktopCommand{DesktopID: "d2"},
		)
		for _, clientID := range []ClientID{clientA, clientB, clientC} {
			eventUpdate := <-subscriptions[clientID].Updates()
			if eventUpdate.Event == nil || eventUpdate.Client != nil ||
				eventUpdate.Event.Revision != deleted.Snapshot.Revision {
				t.Fatalf("%s first update = %+v", clientID, eventUpdate)
			}
		}
		topologyUpdate := <-topologyOnly.Updates()
		if topologyUpdate.Event == nil || topologyUpdate.Client != nil ||
			topologyUpdate.Event.Revision != deleted.Snapshot.Revision {
			t.Fatalf("topology update = %+v", topologyUpdate)
		}
		for _, clientID := range []ClientID{clientA, clientB} {
			clientUpdate := <-subscriptions[clientID].Updates()
			if clientUpdate.Event != nil || clientUpdate.Client == nil ||
				clientUpdate.Client.ClientID != clientID ||
				clientUpdate.Client.ActiveDesktopID != "desktop-default" ||
				clientUpdate.Client.PresentationRevision != 2 {
				t.Fatalf("%s client update = %+v", clientID, clientUpdate)
			}
		}
		requireNoSubscriptionUpdate(t, subscriptions[clientC], "unchanged client")
		requireNoSubscriptionUpdate(t, topologyOnly, "topology-only client repair")
	})

	t.Run("Should publish after commit and evict a slow subscriber without blocking later writes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironmentWithOptions(
			t,
			DefaultConfig(),
			[]WorkspaceID{"workspace-a"},
			WithSubscriptionBuffer(1),
		)
		subscription, err := environment.manager.Subscribe(
			t.Context(),
			SubscriptionRequest{WorkspaceID: "workspace-a"},
		)
		if err != nil {
			t.Fatalf("Subscribe() error=%v", err)
		}
		subscription = trackSubscription(t, subscription)
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
		first, open := <-subscription.Updates()
		if !open || first.Event == nil || first.Event.Revision != 1 {
			t.Fatalf("first event=%+v open=%v", first, open)
		}
		_, open = <-subscription.Updates()
		if open {
			t.Fatal("slow subscription remained open")
		}
		if !errors.Is(subscription.Err(), ErrSlowConsumer) {
			t.Fatalf("subscription error=%v", subscription.Err())
		}
		third := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d4", Name: "Four"},
		)
		if third.Snapshot.Revision != 3 {
			t.Fatalf("writer revision=%d", third.Snapshot.Revision)
		}
	})

	t.Run("Should evict a slow client-bound subscriber without blocking presentation writes", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironmentWithOptions(
			t,
			DefaultConfig(),
			[]WorkspaceID{"workspace-a"},
			WithSubscriptionBuffer(1),
		)
		created := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		clientID := ClientID("client-a")
		registerTestClient(t, environment.manager, "workspace-a", clientID)
		subscription, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: created.Snapshot.Revision, ClientID: &clientID,
		})
		if err != nil {
			t.Fatalf("Subscribe() error = %v", err)
		}
		subscription = trackSubscription(t, subscription)
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		second := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "desktop-default"},
		)
		if second.Client == nil || second.Client.PresentationRevision != 3 ||
			second.Snapshot.Revision != created.Snapshot.Revision {
			t.Fatalf("second presentation result = %+v", second)
		}
		first, open := <-subscription.Updates()
		if !open || first.Client == nil || first.Client.PresentationRevision != 2 {
			t.Fatalf("first presentation update = %+v, open = %v", first, open)
		}
		if _, open = <-subscription.Updates(); open || !errors.Is(subscription.Err(), ErrSlowConsumer) {
			t.Fatalf("slow subscription open = %v, error = %v", open, subscription.Err())
		}
		third := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			&clientID,
			SwitchDesktopCommand{DesktopID: "d2"},
		)
		if third.Client == nil || third.Client.PresentationRevision != 4 ||
			third.Snapshot.Revision != created.Snapshot.Revision {
			t.Fatalf("third presentation result = %+v", third)
		}
	})

	t.Run("Should forget transient workspace state idempotently without deleting repository data", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		registerTestClient(t, environment.manager, "workspace-a", "client-a")
		subscription, err := environment.manager.Subscribe(
			t.Context(),
			SubscriptionRequest{WorkspaceID: "workspace-a", AfterRevision: 1},
		)
		if err != nil {
			t.Fatalf("Subscribe() error=%v", err)
		}
		environment.manager.ForgetWorkspace("workspace-a")
		environment.manager.ForgetWorkspace("workspace-a")
		_, open := <-subscription.Updates()
		if open || !errors.Is(subscription.Err(), ErrWorkspaceNotFound) {
			t.Fatalf("subscription open=%v error=%v", open, subscription.Err())
		}
		if len(environment.repository.Commits("workspace-a")) != 1 {
			t.Fatal("ForgetWorkspace deleted repository state")
		}
		clients, err := environment.manager.Clients(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Clients() error=%v", err)
		}
		if len(clients) != 0 {
			t.Fatalf("forgotten clients=%+v", clients)
		}
	})
}

func requireNoSubscriptionUpdate(t *testing.T, subscription Subscription, operation string) {
	t.Helper()
	select {
	case update, open := <-subscription.Updates():
		t.Fatalf("%s update = %+v, open = %v", operation, update, open)
	default:
	}
}
