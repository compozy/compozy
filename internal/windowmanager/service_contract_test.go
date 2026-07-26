package windowmanager

// Suite: window-manager service contracts
// Invariant: public service helpers preserve clone isolation, transient-client locality, lifecycle teardown, and repository CAS.
// Boundary IN: manager service, in-memory adapters, subscriptions, layout documents, and client registration.
// Boundary OUT: HTTP/UDS serialization and durable bbolt encoding.

import (
	"context"
	"errors"
	"testing"
)

func TestLayoutDocumentContract(t *testing.T) {
	t.Run(
		"Should export immutable documents, validate diagnostics, and replace through normal revision CAS",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			document, err := environment.manager.ExportLayout(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("ExportLayout() error = %v", err)
			}
			validation, err := environment.manager.ValidateLayout(t.Context(), "workspace-a", document)
			if err != nil || !validation.Valid || len(validation.Diagnostics) != 0 {
				t.Fatalf("ValidateLayout(valid) = %+v, error = %v", validation, err)
			}

			document.Desktops[0].Name = "Tampered"
			document.Windows["w1"].Route.Search["tampered"] = []byte(`true`)
			document.Windows["w1"] = Window{}
			current, err := environment.manager.Snapshot(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("Snapshot() error = %v", err)
			}
			if current.Desktops[0].Name == "Tampered" || current.Windows["w1"].ID == "" ||
				current.Windows["w1"].Route.Search["tampered"] != nil {
				t.Fatal("exported document aliases the durable snapshot")
			}

			clean, err := environment.manager.ExportLayout(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("second ExportLayout() error = %v", err)
			}
			noOp, err := environment.manager.ReplaceLayout(
				t.Context(),
				ReplaceLayoutRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: opened.Snapshot.Revision,
					Document:         clean,
				},
			)
			if err != nil || noOp.Applied {
				t.Fatalf("ReplaceLayout(no-op) applied = %v, error = %v", noOp.Applied, err)
			}
			clean.Desktops[0].Name = "Primary"
			replaced, err := environment.manager.ReplaceLayout(
				t.Context(),
				ReplaceLayoutRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: opened.Snapshot.Revision,
					Document:         clean,
				},
			)
			if err != nil || !replaced.Applied || replaced.Snapshot.Desktops[0].Name != "Primary" {
				t.Fatalf("ReplaceLayout(changed) = %+v, error = %v", replaced, err)
			}

			mismatched := clean
			mismatched.WorkspaceID = "workspace-b"
			validation, err = environment.manager.ValidateLayout(t.Context(), "workspace-a", mismatched)
			if err != nil || validation.Valid || len(validation.Diagnostics) != 1 ||
				validation.Diagnostics[0].Code != "topology.workspace_mismatch" {
				t.Fatalf("ValidateLayout(mismatch) = %+v, error = %v", validation, err)
			}
			invalidConfig := clean
			historyLimit := 0
			invalidConfig.Overrides.HistoryLimit = &historyLimit
			validation, err = environment.manager.ValidateLayout(t.Context(), "workspace-a", invalidConfig)
			if err != nil || validation.Valid || len(validation.Diagnostics) != 1 ||
				validation.Diagnostics[0].Code != "config.invalid" {
				t.Fatalf("ValidateLayout(config) = %+v, error = %v", validation, err)
			}
			invalidTopology := clean
			invalidTopology.Desktops = nil
			invalidTopology.Windows = map[WindowID]Window{}
			validation, err = environment.manager.ValidateLayout(t.Context(), "workspace-a", invalidTopology)
			if err != nil || validation.Valid || len(validation.Diagnostics) == 0 {
				t.Fatalf("ValidateLayout(topology) = %+v, error = %v", validation, err)
			}
		},
	)

	t.Run(
		"Should preserve current routes for survivors and accept document routes for new windows",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
			navigated := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				NavigateWindowCommand{
					WindowID: "w1",
					Route:    RouteIntent{Pathname: "/current", Search: RouteSearch{"tab": []byte(`"one"`)}},
				},
			)
			document, err := environment.manager.ExportLayout(t.Context(), "workspace-a")
			if err != nil {
				t.Fatalf("ExportLayout() error = %v", err)
			}
			existing := document.Windows["w1"]
			existing.Route = testRoute("/document-stale")
			document.Windows["w1"] = existing
			document.Windows["w2"] = Window{
				ID: "w2", App: "New", Route: testRoute("/document-new"),
				Placement: WindowPlacementFloating, DesktopID: "desktop-default", FloatingRect: fullRect(),
			}
			document.Desktops[0].Floating = append(document.Desktops[0].Floating, "w2")
			document.Desktops[0].Name = "Applied"
			replaced, err := environment.manager.ReplaceLayout(
				t.Context(),
				ReplaceLayoutRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: navigated.Snapshot.Revision,
					Document:         document,
				},
			)
			if err != nil {
				t.Fatalf("ReplaceLayout() error = %v", err)
			}
			if replaced.Snapshot.Windows["w1"].Route.Pathname != "/current" ||
				replaced.Snapshot.Windows["w2"].Route.Pathname != "/document-new" {
				t.Fatalf("replaced routes = %+v", replaced.Snapshot.Windows)
			}
			if len(replaced.Snapshot.History.Undo) != len(opened.Snapshot.History.Undo)+1 {
				t.Fatalf("replace history = %+v", replaced.Snapshot.History)
			}
		})

	t.Run("Should preserve current routes while applying a declarative layout resource", func(t *testing.T) {
		t.Parallel()
		registry := &testLayoutRegistry{}
		manager := newTestManagerWithRegistry(t, registry)
		openTestWindow(t, manager, "workspace-a", nil, "w1", "desktop-default")
		navigated := executeTestCommand(
			t,
			manager,
			"workspace-a",
			nil,
			NavigateWindowCommand{WindowID: "w1", Route: testRoute("/current")},
		)
		document, err := manager.ExportLayout(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("ExportLayout() error = %v", err)
		}
		window := document.Windows["w1"]
		window.Route = testRoute("/resource-stale")
		document.Windows["w1"] = window
		document.Desktops[0].Name = "Resource"
		registry.resources = []LayoutResource{{ID: "resource", Document: document}}
		applied, err := manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: navigated.Snapshot.Revision,
				Payload:          ArrangeLayoutCommand{ResourceID: "resource"},
			},
		)
		if err != nil {
			t.Fatalf("Execute(resource) error = %v", err)
		}
		if applied.Snapshot.Windows["w1"].Route.Pathname != "/current" {
			t.Fatalf("resource-applied route = %+v", applied.Snapshot.Windows["w1"].Route)
		}
	})

	t.Run("Should clone layout resources and surface registry failures", func(t *testing.T) {
		t.Parallel()
		document := LayoutDocument{
			Version:     SnapshotVersion,
			WorkspaceID: "workspace-a",
			Desktops: []Desktop{
				{
					ID:       "desktop-default",
					Name:     defaultDesktopName,
					Purpose:  DesktopPurposeStandard,
					Groups:   []LayoutGroup{},
					Floating: []WindowID{},
				},
			},
			Windows: map[WindowID]Window{},
		}
		registry := &testLayoutRegistry{
			resources: []LayoutResource{
				{ID: "focus", DisplayName: "Focus", ParticipantSlots: []WindowID{"slot-1"}, Document: document},
			},
		}
		manager := newTestManagerWithRegistry(t, registry)
		resources, err := manager.LayoutResources(t.Context(), "workspace-a")
		if err != nil || len(resources) != 1 {
			t.Fatalf("LayoutResources() = %+v, error = %v", resources, err)
		}
		resources[0].Document.Desktops[0].Name = "Tampered"
		resources[0].Document.Windows["injected"] = Window{ID: "injected"}
		resources[0].ParticipantSlots[0] = "tampered"
		if registry.resources[0].Document.Desktops[0].Name == "Tampered" ||
			len(registry.resources[0].Document.Windows) != 0 ||
			registry.resources[0].ParticipantSlots[0] != "slot-1" {
			t.Fatal("layout resources alias registry-owned data")
		}

		registryError := errors.New("registry unavailable")
		registry.listErr = registryError
		if _, err := manager.LayoutResources(t.Context(), "workspace-a"); !errors.Is(err, registryError) {
			t.Fatalf("LayoutResources() error = %v, want registry error identity", err)
		}
	})

	t.Run("Should apply valid resources and reject invalid or mixed resources without writes", func(t *testing.T) {
		t.Parallel()
		valid := LayoutDocument{
			Version: SnapshotVersion, WorkspaceID: "workspace-a",
			Desktops: []Desktop{{
				ID: "desktop-default", Name: "Resource Desktop", Purpose: DesktopPurposeStandard,
				Groups: []LayoutGroup{}, Floating: []WindowID{},
			}},
			Windows: map[WindowID]Window{},
		}
		invalid := cloneLayoutDocument(valid)
		invalid.Desktops = nil
		registry := &testLayoutRegistry{resources: []LayoutResource{
			{ID: "focus", Document: valid},
			{ID: "invalid", Document: invalid},
		}}
		manager := newTestManagerWithRegistry(t, registry)

		preview, err := manager.Preview(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", Payload: ArrangeLayoutCommand{ResourceID: "focus"},
		})
		if err != nil || !preview.Changed || preview.Snapshot.Desktops[0].Name != "Resource Desktop" {
			t.Fatalf("Preview(resource) = %+v, error = %v", preview, err)
		}
		unchanged, err := manager.Snapshot(t.Context(), "workspace-a")
		if err != nil || unchanged.Revision != 0 || unchanged.Desktops[0].Name != defaultDesktopName {
			t.Fatalf("Snapshot(after preview) = %+v, error = %v", unchanged, err)
		}

		applied, err := manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", Payload: ArrangeLayoutCommand{ResourceID: "focus"},
		})
		if err != nil || !applied.Applied || applied.Snapshot.Revision != 1 ||
			applied.Snapshot.Desktops[0].Name != "Resource Desktop" ||
			len(applied.Snapshot.History.Undo) != 1 ||
			applied.Snapshot.History.Undo[0].CommandID != CommandLayoutArrange {
			t.Fatalf("Execute(resource) = %+v, error = %v", applied, err)
		}

		invalidCommands := []struct {
			name         string
			payload      ArrangeLayoutCommand
			wantSentinel error
			wantTopology bool
		}{
			{
				name: "invalid topology", payload: ArrangeLayoutCommand{ResourceID: "invalid"},
				wantSentinel: ErrInvalidTopology, wantTopology: true,
			},
			{
				name: "mixed fields",
				payload: ArrangeLayoutCommand{
					ResourceID: "focus", DesktopID: "desktop-default",
				},
				wantSentinel: ErrInvalidCommand,
			},
		}
		for _, test := range invalidCommands {
			t.Run(test.name, func(t *testing.T) {
				_, executeErr := manager.Execute(t.Context(), CommandRequest{
					WorkspaceID: "workspace-a", ExpectedRevision: 1, Payload: test.payload,
				})
				if !errors.Is(executeErr, test.wantSentinel) {
					t.Fatalf("Execute(resource) error = %v, want %v", executeErr, test.wantSentinel)
				}
				var topologyErr *TopologyError
				if errors.As(executeErr, &topologyErr) != test.wantTopology {
					t.Fatalf("Execute(resource) topology error = %v, want %v", topologyErr, test.wantTopology)
				}
			})
		}
		afterFailure, err := manager.Snapshot(t.Context(), "workspace-a")
		if err != nil || afterFailure.Revision != 1 || len(afterFailure.History.Undo) != 1 {
			t.Fatalf("Snapshot(after invalid resources) = %+v, error = %v", afterFailure, err)
		}
	})
}

func TestClientLifecycle(t *testing.T) {
	t.Run(
		"Should keep preview local, order clients deterministically, and unregister transient state",
		func(t *testing.T) {
			t.Parallel()
			environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
			created := executeTestCommand(
				t,
				environment.manager,
				"workspace-a",
				nil,
				CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			)
			generated, err := environment.manager.RegisterClient(
				t.Context(),
				ClientRegistration{WorkspaceID: "workspace-a", ActiveDesktopID: "d2"},
			)
			if err != nil || generated.ClientID == "" || generated.ActiveDesktopID != "d2" ||
				generated.PresentationRevision != 1 {
				t.Fatalf("RegisterClient(generated) = %+v, error = %v", generated, err)
			}
			explicit := registerTestClient(t, environment.manager, "workspace-a", "client-b")
			if explicit.ActiveDesktopID != "desktop-default" || explicit.PresentationRevision != 1 {
				t.Fatalf("explicit client = %+v", explicit)
			}
			if _, err := environment.manager.RegisterClient(
				t.Context(),
				ClientRegistration{WorkspaceID: "workspace-a", ClientID: "bad", ActiveDesktopID: "missing"},
			); !errors.Is(
				err,
				ErrDesktopNotFound,
			) {
				t.Fatalf("RegisterClient(missing desktop) error = %v", err)
			}

			clientID := explicit.ClientID
			preview, err := environment.manager.Preview(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: created.Snapshot.Revision,
					ClientID:         &clientID,
					Payload:          SwitchDesktopCommand{DesktopID: "d2"},
				},
			)
			if err != nil || !preview.Changed || preview.Client == nil || preview.Client.ActiveDesktopID != "d2" ||
				preview.Snapshot.Revision != created.Snapshot.Revision {
				t.Fatalf("Preview(switch) = %+v, error = %v", preview, err)
			}
			clients, err := environment.manager.Clients(t.Context(), "workspace-a")
			if err != nil || len(clients) != 2 || clients[0].ClientID >= clients[1].ClientID {
				t.Fatalf("Clients() = %+v, error = %v", clients, err)
			}
			for _, client := range clients {
				if client.ClientID == clientID && client.ActiveDesktopID != "desktop-default" {
					t.Fatal("presentation preview mutated live client state")
				}
			}
			if _, err := environment.manager.Execute(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: created.Snapshot.Revision,
					Payload:          SwitchDesktopCommand{DesktopID: "d2"},
				},
			); !errors.Is(
				err,
				ErrClientNotFound,
			) {
				t.Fatalf("Execute(switch without client) error = %v", err)
			}
			missingClient := ClientID("missing")
			if _, err := environment.manager.Preview(
				t.Context(),
				CommandRequest{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: created.Snapshot.Revision,
					ClientID:         &missingClient,
					Payload:          SwitchDesktopCommand{DesktopID: "d2"},
				},
			); !errors.Is(
				err,
				ErrClientNotFound,
			) {
				t.Fatalf("Preview(missing client) error = %v", err)
			}

			subscription, err := environment.manager.Subscribe(
				t.Context(),
				SubscriptionRequest{
					WorkspaceID:   "workspace-a",
					AfterRevision: created.Snapshot.Revision,
					ClientID:      &clientID,
				},
			)
			if err != nil {
				t.Fatalf("Subscribe(client) error = %v", err)
			}
			subscription = trackSubscription(t, subscription)
			fence := subscription.Fence()
			if fence.Client == nil || fence.Client.ClientID != clientID ||
				fence.Client.PresentationRevision != 1 {
				t.Fatalf("client fence = %+v", fence.Client)
			}
			refreshed, err := environment.manager.RegisterClient(
				t.Context(),
				ClientRegistration{
					WorkspaceID:     "workspace-a",
					ClientID:        clientID,
					ActiveDesktopID: "d2",
				},
			)
			if err != nil || refreshed.ActiveDesktopID != "d2" ||
				refreshed.PresentationRevision != 2 {
				t.Fatalf("RegisterClient(refresh) = %+v, error = %v", refreshed, err)
			}
			refreshUpdate := <-subscription.Updates()
			if refreshUpdate.Client == nil || refreshUpdate.Event != nil ||
				refreshUpdate.Client.PresentationRevision != 2 {
				t.Fatalf("refresh update = %+v", refreshUpdate)
			}
			noOpRefresh, err := environment.manager.RegisterClient(
				t.Context(),
				ClientRegistration{
					WorkspaceID: "workspace-a",
					ClientID:    clientID,
				},
			)
			if err != nil || noOpRefresh.ActiveDesktopID != "d2" ||
				noOpRefresh.PresentationRevision != 2 {
				t.Fatalf("RegisterClient(no-op refresh) = %+v, error = %v", noOpRefresh, err)
			}
			select {
			case update := <-subscription.Updates():
				t.Fatalf("no-op refresh update = %+v", update)
			default:
			}
			if err := environment.manager.UnregisterClient(t.Context(), "workspace-a", clientID); err != nil {
				t.Fatalf("UnregisterClient() error = %v", err)
			}
			if _, open := <-subscription.Updates(); open ||
				!errors.Is(subscription.Err(), ErrClientNotFound) {
				t.Fatalf("unregistered subscription open = %v, error = %v", open, subscription.Err())
			}
			if err := environment.manager.UnregisterClient(
				t.Context(),
				"workspace-a",
				clientID,
			); !errors.Is(
				err,
				ErrClientNotFound,
			) {
				t.Fatalf("second UnregisterClient() error = %v", err)
			}
		},
	)

	t.Run("Should reject changes once a presentation revision reaches the wire maximum", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		created := executeTestCommand(
			t,
			environment.manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		)
		clientID := ClientID("client-max")
		if _, err := environment.manager.RegisterClient(
			t.Context(),
			ClientRegistration{
				WorkspaceID: "workspace-a", ClientID: clientID, ActiveDesktopID: "d2",
			},
		); err != nil {
			t.Fatalf("RegisterClient() error = %v", err)
		}
		environment.manager.mu.Lock()
		view := environment.manager.clients["workspace-a"][clientID]
		view.PresentationRevision = MaxWireRevision
		environment.manager.clients["workspace-a"][clientID] = view
		environment.manager.mu.Unlock()
		subscription, err := environment.manager.Subscribe(t.Context(), SubscriptionRequest{
			WorkspaceID: "workspace-a", AfterRevision: created.Snapshot.Revision, ClientID: &clientID,
		})
		if err != nil {
			t.Fatalf("Subscribe() error = %v", err)
		}
		subscription = trackSubscription(t, subscription)
		if subscription.Fence().Client == nil ||
			subscription.Fence().Client.PresentationRevision != MaxWireRevision {
			t.Fatalf("maximum client fence = %+v", subscription.Fence().Client)
		}

		if _, err := environment.manager.RegisterClient(
			t.Context(),
			ClientRegistration{
				WorkspaceID:     "workspace-a",
				ClientID:        clientID,
				ActiveDesktopID: "desktop-default",
			},
		); !errors.Is(err, ErrPresentationRevisionExhausted) {
			t.Fatalf("RegisterClient(exhausted) error = %v", err)
		}
		if _, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: created.Snapshot.Revision,
			ClientID: &clientID,
			Payload:  SwitchDesktopCommand{DesktopID: "desktop-default"},
		}); !errors.Is(err, ErrPresentationRevisionExhausted) {
			t.Fatalf("Execute(desktop.switch exhausted) error = %v", err)
		}
		if _, err := environment.manager.Execute(t.Context(), CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: created.Snapshot.Revision,
			Payload: DeleteDesktopCommand{DesktopID: "d2"},
		}); !errors.Is(err, ErrPresentationRevisionExhausted) {
			t.Fatalf("Execute(desktop.delete exhausted) error = %v", err)
		}
		requireNoSubscriptionUpdate(t, subscription, "exhausted presentation revision")
		snapshot, err := environment.manager.Snapshot(t.Context(), "workspace-a")
		if err != nil {
			t.Fatalf("Snapshot() error = %v", err)
		}
		if snapshot.Revision != created.Snapshot.Revision ||
			len(environment.repository.Commits("workspace-a")) != 1 {
			t.Fatalf(
				"exhausted revision wrote topology: revision=%d commits=%d",
				snapshot.Revision,
				len(environment.repository.Commits("workspace-a")),
			)
		}
		if _, exists := desktopIndexByID(&snapshot, "d2"); !exists {
			t.Fatal("exhausted revision deleted desktop")
		}
	})
}

func TestServiceLifecycle(t *testing.T) {
	t.Run("Should reject invalid context and command envelopes and close idempotently", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := environment.manager.Snapshot(canceled, "workspace-a"); !errors.Is(err, context.Canceled) {
			t.Fatalf("Snapshot(canceled) error = %v", err)
		}
		if _, err := environment.manager.Snapshot(t.Context(), " "); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("Snapshot(empty workspace) error = %v", err)
		}
		if _, err := environment.manager.Execute(
			t.Context(),
			CommandRequest{WorkspaceID: "workspace-a"},
		); !errors.Is(
			err,
			ErrInvalidCommand,
		) {
			t.Fatalf("Execute(nil payload) error = %v", err)
		}
		if _, err := environment.manager.Execute(
			t.Context(),
			CommandRequest{
				WorkspaceID: "workspace-a",
				CommandID:   CommandDesktopDelete,
				Payload:     CreateDesktopCommand{DesktopID: "d2"},
			},
		); !errors.Is(
			err,
			ErrInvalidCommand,
		) {
			t.Fatalf("Execute(mismatched command ID) error = %v", err)
		}
		if err := environment.manager.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if err := environment.manager.Close(); err != nil {
			t.Fatalf("second Close() error = %v", err)
		}
		if _, err := environment.manager.Snapshot(t.Context(), "workspace-a"); !errors.Is(err, ErrClosed) {
			t.Fatalf("Snapshot(closed) error = %v", err)
		}
	})

	t.Run("Should fence subscriptions and delete durable and transient workspace state", func(t *testing.T) {
		t.Parallel()
		environment := newTestEnvironment(t, DefaultConfig(), "workspace-a")
		opened := openTestWindow(t, environment.manager, "workspace-a", nil, "w1", "desktop-default")
		registerTestClient(t, environment.manager, "workspace-a", "client-a")
		if _, err := environment.manager.Subscribe(
			t.Context(),
			SubscriptionRequest{
				WorkspaceID:   "workspace-a",
				AfterRevision: opened.Snapshot.Revision + 1,
			},
		); !errors.Is(
			err,
			ErrRevisionConflict,
		) {
			t.Fatalf("Subscribe(ahead) error = %v", err)
		}
		subscription, err := environment.manager.Subscribe(
			t.Context(),
			SubscriptionRequest{
				WorkspaceID:   "workspace-a",
				AfterRevision: opened.Snapshot.Revision,
			},
		)
		if err != nil {
			t.Fatalf("Subscribe() error = %v", err)
		}
		fence := subscription.Fence().Snapshot
		fence.Desktops[0].Name = "Tampered"
		if subscription.Fence().Snapshot.Desktops[0].Name == "Tampered" {
			t.Fatal("subscription snapshot aliases its fence")
		}
		if err := environment.manager.DeleteWorkspace(t.Context(), "workspace-a"); err != nil {
			t.Fatalf("DeleteWorkspace() error = %v", err)
		}
		if _, open := <-subscription.Updates(); open || !errors.Is(subscription.Err(), ErrWorkspaceNotFound) {
			t.Fatalf("deleted subscription open = %v, error = %v", open, subscription.Err())
		}
		if _, err := environment.repository.Load(t.Context(), "workspace-a"); !errors.Is(err, ErrSnapshotNotFound) {
			t.Fatalf("repository Load() after delete error = %v", err)
		}
		clients, err := environment.manager.Clients(t.Context(), "workspace-a")
		if err != nil || len(clients) != 0 {
			t.Fatalf("Clients() after delete = %+v, error = %v", clients, err)
		}
	})
}

func TestMemoryAdapters(t *testing.T) {
	t.Run(
		"Should enforce repository identity and revision CAS while returning immutable snapshots",
		func(t *testing.T) {
			t.Parallel()
			repository := NewMemoryRepository()
			invalid := initialSnapshot("workspace-a")
			invalid.Revision = 1
			if err := repository.Commit(
				t.Context(),
				&Commit{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: 0,
					Snapshot:         invalid,
					Event:            Event{WorkspaceID: "workspace-b", Revision: 1},
				},
			); !errors.Is(
				err,
				ErrInvalidTopology,
			) {
				t.Fatalf("Commit(mismatched identity) error = %v", err)
			}
			if err := repository.Commit(
				t.Context(),
				&Commit{
					WorkspaceID:      "workspace-a",
					ExpectedRevision: 0,
					Snapshot:         invalid,
					Event:            Event{WorkspaceID: "workspace-a", Revision: 0},
				},
			); !errors.Is(
				err,
				ErrInvalidTopology,
			) {
				t.Fatalf("Commit(non-advancing revision) error = %v", err)
			}
			commit := Commit{
				WorkspaceID:      "workspace-a",
				ExpectedRevision: 0,
				Snapshot:         invalid,
				Event:            Event{WorkspaceID: "workspace-a", Revision: 1},
			}
			if err := repository.Commit(t.Context(), &commit); err != nil {
				t.Fatalf("Commit(valid) error = %v", err)
			}
			commit.Snapshot.Desktops[0].Name = "Tampered"
			loaded, err := repository.Load(t.Context(), "workspace-a")
			if err != nil || loaded.Desktops[0].Name == "Tampered" {
				t.Fatalf("Load() = %+v, error = %v", loaded, err)
			}
			loaded.Desktops[0].Name = "Also tampered"
			again, err := repository.Load(t.Context(), "workspace-a")
			if err != nil || again.Desktops[0].Name == "Also tampered" {
				t.Fatal("repository Load() result aliases stored state")
			}
			if err := repository.Commit(t.Context(), &commit); !errors.Is(err, ErrRevisionConflict) {
				t.Fatalf("Commit(stale) error = %v", err)
			}
			if err := repository.DeleteWorkspace(t.Context(), "workspace-a"); err != nil {
				t.Fatalf("DeleteWorkspace() error = %v", err)
			}
			if _, err := repository.Load(t.Context(), "workspace-a"); !errors.Is(err, ErrSnapshotNotFound) {
				t.Fatalf("Load(deleted) error = %v", err)
			}
		},
	)

	t.Run("Should reject a wrapped commit without replacing the revision-limit snapshot", func(t *testing.T) {
		t.Parallel()
		repository := NewMemoryRepository()
		atLimit := initialSnapshot("workspace-a")
		atLimit.Revision = Revision(MaxWireRevision)
		repository.mu.Lock()
		repository.snapshots["workspace-a"] = cloneSnapshot(atLimit)
		repository.mu.Unlock()

		wrapped := cloneSnapshot(atLimit)
		wrapped.Revision = 0
		commit := Commit{
			WorkspaceID:      "workspace-a",
			ExpectedRevision: atLimit.Revision,
			Snapshot:         wrapped,
			Event:            Event{WorkspaceID: "workspace-a", Revision: wrapped.Revision},
		}
		if err := repository.Commit(
			t.Context(),
			&commit,
		); !errors.Is(err, ErrTopologyRevisionExhausted) {
			t.Fatalf("Commit(wrapped revision) error = %v", err)
		}
		if commits := repository.Commits("workspace-a"); len(commits) != 0 {
			t.Fatalf("wrapped revision commits = %d, want 0", len(commits))
		}
		stored, err := repository.Load(t.Context(), "workspace-a")
		if err != nil || stored.Revision != atLimit.Revision {
			t.Fatalf("Load(after wrapped revision) = %+v, error = %v", stored, err)
		}
	})

	t.Run("Should make workspace registration immediately authoritative", func(t *testing.T) {
		t.Parallel()
		resolver := NewMemoryWorkspaceResolver()
		if err := resolver.ResolveWorkspace(t.Context(), "workspace-a"); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("ResolveWorkspace(unregistered) error = %v", err)
		}
		resolver.Register("workspace-a")
		if err := resolver.ResolveWorkspace(t.Context(), "workspace-a"); err != nil {
			t.Fatalf("ResolveWorkspace(registered) error = %v", err)
		}
		resolver.Unregister("workspace-a")
		if err := resolver.ResolveWorkspace(t.Context(), "workspace-a"); !errors.Is(err, ErrWorkspaceNotFound) {
			t.Fatalf("ResolveWorkspace(unregistered again) error = %v", err)
		}
	})
}
