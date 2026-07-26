package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestWindowManagerHookBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should map committed events into detached async hook payloads", func(t *testing.T) {
		t.Parallel()
		events := []struct {
			command windowmanager.CommandID
			hook    hookspkg.HookEvent
		}{
			{command: windowmanager.CommandLayoutArrange, hook: hookspkg.HookWindowManagerLayoutApplied},
			{command: windowmanager.CommandDesktopCreate, hook: hookspkg.HookWindowManagerDesktopCreated},
			{command: windowmanager.CommandDesktopDelete, hook: hookspkg.HookWindowManagerDesktopDeleted},
			{command: windowmanager.CommandWindowMove, hook: hookspkg.HookWindowManagerWindowMoved},
		}
		seen := make(chan hookspkg.WindowManagerPayload, len(events))
		declarations := make([]hookspkg.HookDecl, 0, len(events))
		executors := make(map[string]hookspkg.Executor, len(events))
		for _, item := range events {
			name := item.hook.String()
			declarations = append(declarations, hookspkg.HookDecl{
				Name:         name,
				Event:        item.hook,
				Mode:         hookspkg.HookModeAsync,
				Matcher:      hookspkg.HookMatcher{WorkspaceID: "workspace-a"},
				ExecutorKind: hookspkg.HookExecutorNative,
			})
			executors[name] = hookspkg.NewTypedNativeExecutor(
				func(
					_ context.Context,
					_ hookspkg.RegisteredHook,
					payload hookspkg.WindowManagerPayload,
				) (hookspkg.WindowManagerObservationPatch, error) {
					seen <- payload
					return hookspkg.WindowManagerObservationPatch{}, nil
				},
			)
		}
		dispatcher := hookspkg.NewHooks(
			hookspkg.WithLogger(discardLogger()),
			hookspkg.WithNativeDeclarations(declarations),
			hookspkg.WithExecutorResolver(func(declaration hookspkg.HookDecl) (hookspkg.Executor, error) {
				return executors[declaration.Name], nil
			}),
			hookspkg.WithAsyncWorkerCount(1),
			hookspkg.WithAsyncQueueCapacity(len(events)),
		)
		t.Cleanup(dispatcher.Close)
		if err := dispatcher.Rebuild(t.Context()); err != nil {
			t.Fatalf("Rebuild() error = %v", err)
		}
		state := &bootState{logger: discardLogger(), hookDispatcher: dispatcher}
		observer := newWindowManagerHookObserver(state)
		canceled, cancel := context.WithCancel(t.Context())
		cancel()

		for _, item := range events {
			observer(canceled, windowmanager.Event{
				WorkspaceID: "workspace-a",
				Revision:    7,
				CommandID:   item.command,
				Changes: windowmanager.ChangeSet{
					DesktopIDs: []windowmanager.DesktopID{"desktop-a"},
					WindowIDs:  []windowmanager.WindowID{"window-a"},
					GroupIDs:   []windowmanager.GroupID{"group-a"},
					NodeIDs:    []windowmanager.NodeID{"node-a"},
					ClientIDs:  []windowmanager.ClientID{"client-a"},
				},
				Actor:      windowmanager.Actor{Kind: "agent", ID: "agent-a"},
				Origin:     "native:agh__window_manager",
				OccurredAt: time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC),
			})
			select {
			case payload := <-seen:
				if payload.Event != item.hook ||
					payload.WorkspaceID != "workspace-a" ||
					payload.Revision != 7 ||
					payload.CommandID != string(item.command) ||
					payload.Changes.DesktopIDs[0] != "desktop-a" ||
					payload.Changes.WindowIDs[0] != "window-a" ||
					payload.Changes.GroupIDs[0] != "group-a" ||
					payload.Changes.NodeIDs[0] != "node-a" ||
					payload.Changes.ClientIDs[0] != "client-a" ||
					payload.Actor.Kind != "agent" ||
					payload.Actor.ID != "agent-a" ||
					payload.Origin != "native:agh__window_manager" ||
					!payload.Timestamp.Equal(time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)) {
					t.Fatalf("hook payload = %#v", payload)
				}
			case <-time.After(time.Second):
				t.Fatalf("timed out waiting for %q", item.hook)
			}
		}

		observer(t.Context(), windowmanager.Event{
			WorkspaceID: "workspace-a",
			CommandID:   windowmanager.CommandWindowFocus,
		})
		select {
		case payload := <-seen:
			t.Fatalf("presentation command produced hook payload %#v", payload)
		case <-time.After(20 * time.Millisecond):
		}
	})

	t.Run("Should tolerate an unavailable late-bound dispatcher", func(t *testing.T) {
		t.Parallel()
		observer := newWindowManagerHookObserver(&bootState{logger: discardLogger()})
		observer(t.Context(), windowmanager.Event{
			WorkspaceID: "workspace-a",
			CommandID:   windowmanager.CommandDesktopCreate,
		})
	})
}

func TestWindowManagerWorkspaceDeletionGate(t *testing.T) {
	t.Parallel()

	t.Run("Should purge topology clients and subscriptions before same-id recreation", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		sessionPreparation := &windowManagerSessionRemovalPreparation{}
		sessions := &windowManagerRemovalSessionManager{
			fakeSessionManager: &fakeSessionManager{},
			workspaceID:        fixture.workspace.ID,
			preparation:        sessionPreparation,
		}
		state := &bootState{
			windowManagerBootState: windowManagerBootState{
				windowManagerStoreResolver: fixture.storeResolver,
				windowManagerStore:         fixture.engine,
				windowManagerRepository:    fixture.repository,
				windowManager:              fixture.manager,
			},
			workspaceResolver: fixture.resolver,
		}
		if err := installWorkspaceRemovalPreparer(state, sessions); err != nil {
			t.Fatalf("installWorkspaceRemovalPreparer() error = %v", err)
		}

		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		clientID := windowmanager.ClientID("browser-a")
		if _, err := fixture.manager.RegisterClient(ctx, windowmanager.ClientRegistration{
			WorkspaceID: workspaceID, ClientID: clientID,
		}); err != nil {
			t.Fatalf("RegisterClient() error = %v", err)
		}
		if _, err := fixture.manager.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID: workspaceID, ExpectedRevision: 0, ClientID: &clientID,
			Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
				ID: "window-a", App: "Terminal", DesktopID: "desktop-default",
				Route: windowmanager.RouteIntent{
					Pathname: "/terminal", Search: windowmanager.RouteSearch{},
				},
				FloatingRect: windowmanager.NormalizedRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5},
			}},
		}); err != nil {
			t.Fatalf("Execute(window.open) error = %v", err)
		}
		subscription, err := fixture.manager.Subscribe(
			ctx,
			windowmanager.SubscriptionRequest{WorkspaceID: workspaceID, AfterRevision: 1},
		)
		if err != nil {
			t.Fatalf("Subscribe() error = %v", err)
		}
		t.Cleanup(func() {
			if err := subscription.Close(); err != nil {
				t.Errorf("Subscription.Close() error = %v", err)
			}
		})

		if err := fixture.resolver.Unregister(ctx, fixture.workspace.ID); err != nil {
			t.Fatalf("Unregister() error = %v", err)
		}
		if sessionPreparation.commits != 1 || sessionPreparation.rollbacks != 0 {
			t.Fatalf(
				"session preparation = (commits=%d rollbacks=%d), want (1 0)",
				sessionPreparation.commits,
				sessionPreparation.rollbacks,
			)
		}
		select {
		case _, open := <-subscription.Updates():
			if open {
				t.Fatal("subscription remained open after workspace purge")
			}
		case <-time.After(time.Second):
			t.Fatal("workspace purge did not close the subscription")
		}
		if !errors.Is(subscription.Err(), windowmanager.ErrWorkspaceNotFound) {
			t.Fatalf("Subscription.Err() = %v, want ErrWorkspaceNotFound", subscription.Err())
		}

		if err := fixture.database.InsertWorkspace(ctx, fixture.workspace); err != nil {
			t.Fatalf("InsertWorkspace(same id) error = %v", err)
		}
		fixture.resolver.Invalidate(fixture.workspace.ID)
		snapshot, err := fixture.manager.Snapshot(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Snapshot(recreated) error = %v", err)
		}
		if snapshot.Revision != 0 || len(snapshot.Windows) != 0 {
			t.Fatalf("Snapshot(recreated) = %#v, want fresh revision zero", snapshot)
		}
		clients, err := fixture.manager.Clients(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Clients(recreated) error = %v", err)
		}
		if len(clients) != 0 {
			t.Fatalf("Clients(recreated) = %#v, want no stale client views", clients)
		}
	})

	t.Run("Should restore topology and transient clients when deletion rolls back", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		clientID := windowmanager.ClientID("browser-a")
		if _, err := fixture.manager.RegisterClient(ctx, windowmanager.ClientRegistration{
			WorkspaceID: workspaceID, ClientID: clientID,
		}); err != nil {
			t.Fatalf("RegisterClient() error = %v", err)
		}
		want := daemonWindowManagerSnapshot(workspaceID, 1, "Before rollback")
		if err := fixture.repository.Commit(ctx, daemonWindowManagerCommit(want, 0)); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		preparation, err := fixture.storeResolver.prepareRemoval(
			fixture.workspace,
			fixture.engine,
			fixture.manager,
			fixture.repository,
		)
		if err != nil {
			t.Fatalf("prepareRemoval() error = %v", err)
		}
		if err := preparation.BeforeDelete(ctx); err != nil {
			t.Fatalf("BeforeDelete() error = %v", err)
		}
		if _, err := fixture.manager.Snapshot(ctx, workspaceID); !errors.Is(err, windowmanager.ErrWorkspaceNotFound) {
			t.Fatalf("Snapshot(deleting) error = %v, want ErrWorkspaceNotFound", err)
		}
		if err := preparation.Rollback(ctx); err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}
		got, err := fixture.manager.Snapshot(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Snapshot(after rollback) error = %v", err)
		}
		if got.Revision != want.Revision || got.Desktops[0].Name != want.Desktops[0].Name {
			t.Fatalf("Snapshot(after rollback) = %#v, want preserved revision and desktop", got)
		}
		clients, err := fixture.manager.Clients(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Clients(after rollback) error = %v", err)
		}
		if len(clients) != 1 || clients[0].ClientID != clientID {
			t.Fatalf("Clients(after rollback) = %#v, want browser-a", clients)
		}
	})
}

func TestWindowManagerWorkspaceConfigRuntime(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate workspace behavior and follow active hot apply", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceARoot := fixture.workspace.RootDir
		workspaceBRoot := filepath.Join(t.TempDir(), "workspace-b")
		if err := os.MkdirAll(workspaceBRoot, 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", workspaceBRoot, err)
		}
		workspaceB, err := fixture.resolver.Register(ctx, workspacepkg.RegisterOptions{
			RootDir: workspaceBRoot,
			Name:    "window-manager-b",
		})
		if err != nil {
			t.Fatalf("workspace.Register(B) error = %v", err)
		}
		writeDaemonWindowManagerConfig(
			t,
			filepath.Join(workspaceARoot, aghconfig.DirName, aghconfig.ConfigName),
			"[window_manager]\nnew_window_policy = \"beside_focus\"\n",
		)
		writeDaemonWindowManagerConfig(
			t,
			fixture.homePaths.ConfigFile,
			"[window_manager]\nnew_window_policy = \"beside_focus\"\n",
		)
		fixture.resolver.Invalidate(fixture.workspace.ID)
		fixture.resolver.Invalidate(workspaceB.ID)

		open := func(workspaceID string, windowID windowmanager.WindowID) windowmanager.Result {
			t.Helper()
			before, snapshotErr := fixture.manager.Snapshot(ctx, windowmanager.WorkspaceID(workspaceID))
			if snapshotErr != nil {
				t.Fatalf("Snapshot(%s) error = %v", workspaceID, snapshotErr)
			}
			result, executeErr := fixture.manager.Execute(ctx, windowmanager.CommandRequest{
				WorkspaceID:      windowmanager.WorkspaceID(workspaceID),
				ExpectedRevision: before.Revision,
				Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
					ID: windowID, App: "Terminal", DesktopID: "desktop-default",
					Route:        windowmanager.RouteIntent{Pathname: "/terminal", Search: windowmanager.RouteSearch{}},
					FloatingRect: windowmanager.NormalizedRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5},
				}},
			})
			if executeErr != nil {
				t.Fatalf("Execute(window.open %s) error = %v", windowID, executeErr)
			}
			return result
		}

		workspaceAResult := open(fixture.workspace.ID, "window-a")
		if got := workspaceAResult.Snapshot.Windows["window-a"].Placement; got != windowmanager.WindowPlacementTiled {
			t.Fatalf("workspace A placement = %q, want workspace-configured tiled", got)
		}
		workspaceBResult := open(workspaceB.ID, "window-b")
		if got := workspaceBResult.Snapshot.Windows["window-b"].Placement; got != windowmanager.WindowPlacementFloating {
			t.Fatalf("workspace B placement = %q, want active-global floating", got)
		}

		active := windowmanager.DefaultConfig()
		active.NewWindowPolicy = windowmanager.NewWindowInsert
		if err := fixture.manager.UpdateDefaults(active); err != nil {
			t.Fatalf("UpdateDefaults() error = %v", err)
		}
		workspaceBResult = open(workspaceB.ID, "window-b-hot-applied")
		if got := workspaceBResult.Snapshot.Windows["window-b-hot-applied"].Placement; got != windowmanager.WindowPlacementTiled {
			t.Fatalf("workspace B hot-applied placement = %q, want tiled", got)
		}
	})
}

type windowManagerRemovalSessionManager struct {
	*fakeSessionManager
	workspaceID string
	preparation workspacepkg.UnregisterPreparation
}

func (m *windowManagerRemovalSessionManager) PrepareWorkspaceRemoval(
	_ context.Context,
	workspaceID string,
) (workspacepkg.UnregisterPreparation, error) {
	if workspaceID != m.workspaceID {
		return nil, errors.New("unexpected workspace removal id")
	}
	return m.preparation, nil
}

type windowManagerSessionRemovalPreparation struct {
	commits   int
	rollbacks int
}

func (*windowManagerSessionRemovalPreparation) BeforeDelete(context.Context) error {
	return nil
}

func (p *windowManagerSessionRemovalPreparation) Commit(context.Context) error {
	p.commits++
	return nil
}

func (p *windowManagerSessionRemovalPreparation) Rollback(context.Context) error {
	p.rollbacks++
	return nil
}
