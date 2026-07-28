package windowmanager

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type testLayoutRegistry struct {
	resources []LayoutResource
	listErr   error
}

func (r *testLayoutRegistry) Resolve(_ context.Context, _ WorkspaceID, resourceID string) (LayoutDocument, error) {
	for _, resource := range r.resources {
		if resource.ID == resourceID {
			return resource.Document, nil
		}
	}
	return LayoutDocument{}, ErrLayoutResourceNotFound
}

func (r *testLayoutRegistry) List(context.Context, WorkspaceID) ([]LayoutResource, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.resources, nil
}

var _ LayoutResourceRegistry = (*testLayoutRegistry)(nil)

type testEnvironment struct {
	manager    *Manager
	repository *MemoryRepository
}

func newTestEnvironment(t *testing.T, config Config, workspaces ...WorkspaceID) testEnvironment {
	t.Helper()
	return newTestEnvironmentWithOptions(t, config, workspaces)
}

func newTestManagerWithRegistry(t *testing.T, registry LayoutResourceRegistry) *Manager {
	t.Helper()
	manager, err := NewService(
		NewMemoryRepository(), NewMemoryWorkspaceResolver("workspace-a"), registry, DefaultConfig(),
		WithClock(func() time.Time { return time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC) }),
		WithIDGenerator(func(kind string) (string, error) { return kind + "-generated", nil }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("Manager.Close() error = %v", closeErr)
		}
	})
	return manager
}

func trackSubscription(t *testing.T, subscription Subscription) Subscription {
	t.Helper()
	t.Cleanup(func() {
		if closeErr := subscription.Close(); closeErr != nil {
			t.Errorf("Subscription.Close() error = %v", closeErr)
		}
	})
	return subscription
}

func newTestEnvironmentWithOptions(
	t *testing.T,
	config Config,
	workspaces []WorkspaceID,
	options ...Option,
) testEnvironment {
	t.Helper()
	repository := NewMemoryRepository()
	resolver := NewMemoryWorkspaceResolver(workspaces...)
	var sequence atomic.Int64
	baseOptions := []Option{
		WithClock(func() time.Time { return time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC) }),
		WithIDGenerator(
			func(kind string) (string, error) { return fmt.Sprintf("%s-%03d", kind, sequence.Add(1)), nil },
		),
	}
	baseOptions = append(baseOptions, options...)
	manager, err := NewService(repository, resolver, nil, config, baseOptions...)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("Manager.Close() error = %v", closeErr)
		}
	})
	return testEnvironment{manager: manager, repository: repository}
}

func executeTestCommand(
	t *testing.T,
	manager *Manager,
	workspaceID WorkspaceID,
	clientID *ClientID,
	command Command,
) Result {
	t.Helper()
	snapshot, err := manager.Snapshot(t.Context(), workspaceID)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	result, err := manager.Execute(
		t.Context(),
		CommandRequest{
			WorkspaceID:      workspaceID,
			CommandID:        command.CommandID(),
			ExpectedRevision: snapshot.Revision,
			ClientID:         clonePointer(clientID),
			Payload:          command,
		},
	)
	if err != nil {
		t.Fatalf("Execute(%s) error = %v", command.CommandID(), err)
	}
	return result
}

func openTestWindow(
	t *testing.T,
	manager *Manager,
	workspaceID WorkspaceID,
	clientID *ClientID,
	windowID WindowID,
	desktopID DesktopID,
) Result {
	t.Helper()
	return executeTestCommand(
		t,
		manager,
		workspaceID,
		clientID,
		OpenWindowCommand{
			Window: WindowSpec{
				ID:           windowID,
				App:          "Test",
				Route:        testRoute("/test"),
				DesktopID:    desktopID,
				FloatingRect: NormalizedRect{X: 0.2, Y: 0.2, Width: 0.5, Height: 0.5},
			},
		},
	)
}

func registerTestClient(t *testing.T, manager *Manager, workspaceID WorkspaceID, clientID ClientID) ClientView {
	t.Helper()
	view, err := manager.RegisterClient(t.Context(), ClientRegistration{WorkspaceID: workspaceID, ClientID: clientID})
	if err != nil {
		t.Fatalf("RegisterClient() error = %v", err)
	}
	return view
}

func requireValidSnapshot(t *testing.T, snapshot Snapshot) {
	t.Helper()
	if err := ValidateSnapshot(snapshot); err != nil {
		t.Fatalf("ValidateSnapshot() error = %v", err)
	}
}

func validThreeWindowSnapshot() Snapshot {
	w1, w2, w3 := WindowID("w1"), WindowID("w2"), WindowID("w3")
	horizontal, vertical := AxisHorizontal, AxisVertical
	return Snapshot{
		Version:     SnapshotVersion,
		WorkspaceID: "workspace-a",
		Desktops: []Desktop{
			{
				ID:      "desktop-default",
				Name:    defaultDesktopName,
				Purpose: DesktopPurposeStandard,
				Groups: []LayoutGroup{
					{
						ID:    "group-1",
						Frame: fullRect(),
						Root: LayoutNode{
							ID:      "root",
							Kind:    NodeKindSplit,
							Axis:    &horizontal,
							Weights: []float64{0.5, 0.5},
							Children: []LayoutNode{
								{ID: "leaf-1", Kind: NodeKindLeaf, WindowID: &w1},
								{
									ID:      "right",
									Kind:    NodeKindSplit,
									Axis:    &vertical,
									Weights: []float64{0.5, 0.5},
									Children: []LayoutNode{
										{ID: "leaf-2", Kind: NodeKindLeaf, WindowID: &w2},
										{ID: "leaf-3", Kind: NodeKindLeaf, WindowID: &w3},
									},
								},
							},
						},
					},
				},
				Floating: []WindowID{},
			},
		},
		Windows: map[WindowID]Window{
			w1: {
				ID:           w1,
				App:          "One",
				DesktopID:    "desktop-default",
				Route:        testRoute("/one"),
				Placement:    WindowPlacementTiled,
				FloatingRect: NormalizedRect{Width: 0.5, Height: 0.5},
			},
			w2: {
				ID:           w2,
				App:          "Two",
				DesktopID:    "desktop-default",
				Route:        testRoute("/two"),
				Placement:    WindowPlacementTiled,
				FloatingRect: NormalizedRect{Width: 0.5, Height: 0.5},
			},
			w3: {
				ID:           w3,
				App:          "Three",
				DesktopID:    "desktop-default",
				Route:        testRoute("/three"),
				Placement:    WindowPlacementTiled,
				FloatingRect: NormalizedRect{Width: 0.5, Height: 0.5},
			},
		},
		History: History{Undo: []HistoryEntry{}, Redo: []HistoryEntry{}},
	}
}

func testRoute(pathname string) RouteIntent {
	return RouteIntent{Pathname: pathname, Search: RouteSearch{}}
}
