package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	bolt "go.etcd.io/bbolt"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/compozy/compozy/internal/windowmanager/statekeys"
)

func TestWindowManagerRepository(t *testing.T) {
	t.Parallel()

	t.Run("Should persist deprecated command apps and warn once per id [IT-014]", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		var logs bytes.Buffer
		observer := windowManagerAppDeprecationLogger(slog.New(slog.NewJSONHandler(&logs, nil)), &sync.Map{})
		manager, err := windowmanager.NewService(
			fixture.repository,
			windowManagerWorkspaceAuthorizer{resolver: fixture.storeResolver},
			nil,
			windowmanager.DefaultConfig(),
			windowmanager.WithAppDeprecationObserver(observer),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Close(); err != nil {
				t.Error(err)
			}
		})
		for i, app := range []string{"jobs", "triggers", "jobs", "triggers"} {
			id := windowmanager.WindowID(fmt.Sprintf("retired-%d", i))
			source := "command"
			if app == "triggers" {
				source = "cli"
			}
			result, err := manager.Execute(
				t.Context(),
				windowmanager.CommandRequest{
					WorkspaceID:      windowmanager.WorkspaceID(fixture.workspace.ID),
					ExpectedRevision: windowmanager.Revision(i),
					Actor:            windowmanager.Actor{Kind: source},
					Payload: windowmanager.OpenWindowCommand{
						Window: windowmanager.WindowSpec{
							ID:  id,
							App: app,
							Route: windowmanager.RouteIntent{
								Pathname: "/" + app + "/example",
								Search:   windowmanager.RouteSearch{},
							},
							FloatingRect: windowmanager.NormalizedRect{Width: 0.5, Height: 0.5},
						},
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.Snapshot.Windows[id].App != "automations" ||
				result.Snapshot.Windows[id].Route.Pathname != "/automations/"+app+"/example" {
				t.Fatalf("window=%+v", result.Snapshot.Windows[id])
			}
		}
		if strings.Count(logs.String(), `"event":"windowmanager.app_id_deprecated"`) != 2 ||
			!strings.Contains(logs.String(), `"source":"cli"`) ||
			!strings.Contains(logs.String(), `"source":"command"`) ||
			!strings.Contains(logs.String(), `"removal":"v0.5.0"`) {
			t.Fatalf("warnings=%s", logs.String())
		}
		loaded, err := fixture.repository.Load(t.Context(), windowmanager.WorkspaceID(fixture.workspace.ID))
		if err != nil {
			t.Fatal(err)
		}
		if len(loaded.Windows) != 5 {
			t.Fatalf("persisted windows=%d", len(loaded.Windows))
		}
	})

	t.Run("Should reject a nil commit as invalid topology", func(t *testing.T) {
		t.Parallel()
		repository := &windowManagerRepository{}
		err := repository.Commit(testutil.Context(t), nil)
		if !errors.Is(err, windowmanager.ErrInvalidTopology) {
			t.Fatalf("Commit(nil) error = %v, want ErrInvalidTopology", err)
		}
	})

	t.Run("Should persist one full v6 snapshot exactly across store reopen [IT-001]", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		want := daemonWindowManagerSnapshot(workspaceID, 1, "Primary")
		decorateDaemonV3Snapshot(&want)
		if err := fixture.repository.Commit(ctx, daemonWindowManagerCommit(want, 0)); err != nil {
			t.Fatalf("Commit() error = %v", err)
		}
		closeDaemonWindowManagerEngine(t, fixture.engine)

		reopened, err := clientstate.Open(
			ctx,
			fixture.storePath,
			fixture.storeResolver,
			clientstate.Limits{
				MaxValueBytes:       windowManagerMaxSnapshotBytes,
				MaxKeysPerWorkspace: clientstate.DefaultLimits().MaxKeysPerWorkspace,
			},
			clientstate.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatalf("clientstate.Open(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(); err != nil {
				t.Errorf("Engine.Close(reopened) error = %v", err)
			}
		})
		repository, err := newWindowManagerRepository(reopened, testWindowManagerProfileID)
		if err != nil {
			t.Fatalf("newWindowManagerRepository(reopened, testWindowManagerProfileID) error = %v", err)
		}
		got, err := repository.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() = %#v, want %#v", got, want)
		}
	})

	t.Run("Should key desktop arrangements by workspace and profile [IT-057]", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const (
			dev       = "01JQPROFILEDEV0000000000000"
			marketing = "01JQPROFILEMARKETING000000"
			fresh     = "01JQPROFILEFRESH0000000000"
		)

		devManager, err := fixture.registry.For(dev)
		if err != nil {
			t.Fatalf("For(dev) error = %v", err)
		}
		marketingManager, err := fixture.registry.For(marketing)
		if err != nil {
			t.Fatalf("For(marketing) error = %v", err)
		}
		devResult := executeDaemonDesktopCreate(t, devManager, workspaceID, "desktop-dev", "Dev")
		marketingResult := executeDaemonDesktopCreate(
			t, marketingManager, workspaceID, "desktop-marketing", "Marketing",
		)

		// Isolated: each profile's arrangement holds only its own desktops, and the
		// revision each one counts is its own.
		assertDaemonDesktopNames(t, devResult.Snapshot, []string{"Desktop 1", "Dev"})
		assertDaemonDesktopNames(t, marketingResult.Snapshot, []string{"Desktop 1", "Marketing"})

		// Restored on switch: re-entering a profile returns exactly what it left,
		// which is what the shell rebinding to another partition then back does.
		restored, err := devManager.Snapshot(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Snapshot(dev) error = %v", err)
		}
		assertDaemonDesktopNames(t, restored, []string{"Desktop 1", "Dev"})

		// New profile clean: an untouched profile starts on the seeded default desk
		// rather than inheriting a neighbour's arrangement.
		freshManager, err := fixture.registry.For(fresh)
		if err != nil {
			t.Fatalf("For(fresh) error = %v", err)
		}
		freshSnapshot, err := freshManager.Snapshot(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Snapshot(fresh) error = %v", err)
		}
		assertDaemonDesktopNames(t, freshSnapshot, []string{"Desktop 1"})

		// Retained: archiving removes no window state, so the stored partition
		// survives everything short of deletion — including a daemon restart.
		if err := fixture.registry.Close(); err != nil {
			t.Fatalf("windowManagerRegistry.Close() error = %v", err)
		}
		reopened, err := newWindowManagerRepository(fixture.engine, marketing)
		if err != nil {
			t.Fatalf("newWindowManagerRepository(marketing) error = %v", err)
		}
		retained, err := reopened.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Load(marketing) error = %v", err)
		}
		assertDaemonDesktopNames(t, retained, []string{"Desktop 1", "Marketing"})
	})

	t.Run("Should count and purge only the deleted profile's desktops [IT-038]", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		const (
			doomed   = "01JQPROFILEDOOMED000000000"
			survivor = "01JQPROFILESURVIVOR000000"
		)
		for _, profileID := range []string{doomed, survivor} {
			manager, err := fixture.registry.For(profileID)
			if err != nil {
				t.Fatalf("For(%s) error = %v", profileID, err)
			}
			executeDaemonDesktopCreate(t, manager, workspaceID, "desktop-second", "Second")
		}

		count, err := fixture.registry.CountDesktopPartitions(ctx, doomed)
		if err != nil {
			t.Fatalf("CountDesktopPartitions() error = %v", err)
		}
		if count != 1 {
			t.Fatalf("CountDesktopPartitions() = %d, want 1", count)
		}
		if err := fixture.registry.PurgeDesktopPartitions(ctx, doomed); err != nil {
			t.Fatalf("PurgeDesktopPartitions() error = %v", err)
		}
		if err := fixture.registry.PurgeDesktopPartitions(ctx, doomed); err != nil {
			t.Fatalf("PurgeDesktopPartitions(repeat) error = %v", err)
		}
		remaining, err := fixture.registry.CountDesktopPartitions(ctx, doomed)
		if err != nil {
			t.Fatalf("CountDesktopPartitions(after purge) error = %v", err)
		}
		if remaining != 0 {
			t.Fatalf("CountDesktopPartitions(after purge) = %d, want 0", remaining)
		}
		survivorCount, err := fixture.registry.CountDesktopPartitions(ctx, survivor)
		if err != nil {
			t.Fatalf("CountDesktopPartitions(survivor) error = %v", err)
		}
		if survivorCount != 1 {
			t.Fatalf("CountDesktopPartitions(survivor) = %d, want 1", survivorCount)
		}
	})

	t.Run("Should reject a wrapped commit without replacing the revision-limit snapshot", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		atLimit := daemonWindowManagerSnapshot(workspaceID, 1, "Limit")
		atLimit.Revision = windowmanager.Revision(windowmanager.MaxWireRevision)
		encoded, err := json.Marshal(atLimit)
		if err != nil {
			t.Fatalf("json.Marshal(limit snapshot) error = %v", err)
		}
		if _, err := fixture.engine.Apply(
			ctx,
			clientstate.WorkspaceID(workspaceID),
			windowManagerStateDomain,
			[]clientstate.Op{{
				Kind:  clientstate.OpPut,
				Key:   windowManagerSnapshotKey(testWindowManagerProfileID),
				Value: encoded,
			}},
			clientstate.ApplyOptions{},
		); err != nil {
			t.Fatalf("Apply(limit snapshot) error = %v", err)
		}

		wrapped := daemonWindowManagerSnapshot(workspaceID, 0, "Wrapped")
		if err := fixture.repository.Commit(
			ctx,
			daemonWindowManagerCommit(wrapped, atLimit.Revision),
		); !errors.Is(err, windowmanager.ErrTopologyRevisionExhausted) {
			t.Fatalf("Commit(wrapped revision) error = %v", err)
		}
		stored, err := fixture.repository.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Load(after wrapped revision) error = %v", err)
		}
		if stored.Revision != atLimit.Revision || stored.Desktops[0].Name != "Limit" {
			t.Fatalf("stored snapshot after wrapped revision = %+v", stored)
		}
	})

	t.Run("Should allow exactly one concurrent compare-and-swap writer", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := contextWithoutCancellation(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		initial := daemonWindowManagerSnapshot(workspaceID, 1, "Initial")
		if err := fixture.repository.Commit(ctx, daemonWindowManagerCommit(initial, 0)); err != nil {
			t.Fatalf("Commit(initial) error = %v", err)
		}

		results := make(chan error, 2)
		var writers sync.WaitGroup
		for _, name := range []string{"First", "Second"} {
			writers.Go(func() {
				next := daemonWindowManagerSnapshot(workspaceID, 2, name)
				results <- fixture.repository.Commit(ctx, daemonWindowManagerCommit(next, 1))
			})
		}
		writers.Wait()
		close(results)

		successes := 0
		conflicts := 0
		for err := range results {
			switch {
			case err == nil:
				successes++
			case errors.Is(err, windowmanager.ErrRevisionConflict):
				conflicts++
			default:
				t.Fatalf("Commit(concurrent) error = %v", err)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("concurrent results = successes %d conflicts %d, want 1/1", successes, conflicts)
		}
	})
}

func decorateDaemonV3Snapshot(snapshot *windowmanager.Snapshot) {
	snapshot.Windows = map[windowmanager.WindowID]windowmanager.Window{}
	snapshot.Desktops[0].Tabs = nil
	for _, id := range []windowmanager.WindowID{"w1", "w2"} {
		tab := daemonTestTab(id)
		snapshot.Desktops[0].Tabs = append(snapshot.Desktops[0].Tabs, tab)
		snapshot.Windows[id] = windowmanager.Window{
			ID:        id,
			App:       "settings",
			Route:     windowmanager.RouteIntent{Pathname: "/settings", Search: windowmanager.RouteSearch{}},
			NavStack:  []windowmanager.RouteIntent{},
			DesktopID: "desktop-default",
			TabID:     tab.ID,
		}
	}
	snapshot.Desktops[0].ActiveTabID = snapshot.Desktops[0].Tabs[1].ID
}

func daemonTestTab(id windowmanager.WindowID) windowmanager.Tab {
	return windowmanager.Tab{
		ID: windowmanager.TabID("tab-" + string(id)),
		Root: windowmanager.LayoutNode{
			ID:       windowmanager.NodeID("leaf-" + string(id)),
			Kind:     windowmanager.NodeKindLeaf,
			WindowID: new(id),
		},
	}
}

// Invariant: a load reconciles retired app state durably exactly once, with one observable log.
// Owning layer: daemon repository/clientstate SQLite; canonical suite: window_manager_repository_test.go.
func TestWindowManagerRepositoryReconcile(t *testing.T) {
	t.Parallel()
	t.Run("Should persist and log reconciliation once across repeated loads [IT-009]", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := t.Context()
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		snapshot := daemonWindowManagerSnapshot(workspaceID, 1, "Primary")
		for _, app := range []string{"knowledge", "session"} {
			id := windowmanager.WindowID(app)
			snapshot.Windows[id] = windowmanager.Window{
				ID:        id,
				App:       app,
				DesktopID: "desktop-default",
				Route:     windowmanager.RouteIntent{Pathname: "/" + app, Search: windowmanager.RouteSearch{}},
			}
			tab := daemonTestTab(id)
			snapshot.Desktops[0].Tabs = append(snapshot.Desktops[0].Tabs, tab)
			w := snapshot.Windows[id]
			w.TabID = tab.ID
			snapshot.Windows[id] = w
		}
		settingsID := windowmanager.WindowID("settings")
		snapshot.Windows[settingsID] = windowmanager.Window{
			ID:        settingsID,
			App:       "settings",
			DesktopID: "desktop-default",
			Route:     windowmanager.RouteIntent{Pathname: "/settings/memory", Search: windowmanager.RouteSearch{}},
			NavStack: []windowmanager.RouteIntent{
				{Pathname: "/settings/memory", Search: windowmanager.RouteSearch{}},
			},
		}
		tab := daemonTestTab(settingsID)
		snapshot.Desktops[0].Tabs = append(snapshot.Desktops[0].Tabs, tab)
		w := snapshot.Windows[settingsID]
		w.TabID = tab.ID
		snapshot.Windows[settingsID] = w
		if err := fixture.repository.Commit(ctx, daemonWindowManagerCommit(snapshot, 0)); err != nil {
			t.Fatalf("Commit() = %v", err)
		}
		before, err := fixture.engine.Get(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain,
			windowManagerSnapshotKey(testWindowManagerProfileID))
		if err != nil {
			t.Fatalf("Get(before) = %v", err)
		}
		var logs bytes.Buffer
		repository, err := newWindowManagerRepository(fixture.engine, testWindowManagerProfileID,
			withWindowManagerRepositoryLogger(slog.New(slog.NewJSONHandler(&logs, nil))))
		if err != nil {
			t.Fatalf("newWindowManagerRepository() = %v", err)
		}
		got, err := repository.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if len(got.Windows) != 3 || got.Windows["session"].App != "session" || got.Revision != 2 ||
			len(got.Desktops[0].Tabs) != 3 ||
			got.Windows[settingsID].Route.Pathname != "/settings" ||
			got.Windows[settingsID].NavStack[0].Pathname != "/settings" {
			t.Fatalf("reconciled snapshot = %+v", got)
		}
		after, err := fixture.engine.Get(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain,
			windowManagerSnapshotKey(testWindowManagerProfileID))
		if err != nil {
			t.Fatalf("Get(after) = %v", err)
		}
		if after.Rev != before.Rev+1 {
			t.Fatalf("store revision = %d, want %d", after.Rev, before.Rev+1)
		}
		var record struct {
			ProfileID       string   `json:"profile_id"`
			DroppedApps     []string `json:"dropped_apps"`
			RewrittenRoutes int      `json:"rewritten_routes"`
		}
		if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
			t.Fatalf("decode reconcile audit: %v", err)
		}
		if record.ProfileID != string(testWindowManagerProfileID) || len(record.DroppedApps) != 1 ||
			record.DroppedApps[0] != "knowledge" || record.RewrittenRoutes != 2 {
			t.Fatalf("reconcile audit = %+v", record)
		}
		again, err := repository.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("second Load() = %+v, error = %v", again, err)
		}
		encoded, err := json.Marshal(again)
		if err != nil {
			t.Fatalf("Marshal(second load) = %v", err)
		}
		if !bytes.Equal(encoded, after.Value) {
			t.Fatalf("second load changed durable snapshot: %s", encoded)
		}
		last, err := fixture.engine.Get(ctx, clientstate.WorkspaceID(workspaceID), windowManagerStateDomain,
			windowManagerSnapshotKey(testWindowManagerProfileID))
		if err != nil {
			t.Fatalf("Get(last) = %v", err)
		}
		if last.Rev != after.Rev || strings.Count(logs.String(), "windowmanager.snapshot_reconciled") != 1 {
			t.Fatalf("second load revision = %d, logs = %s", last.Rev, logs.String())
		}
	})
}

// Migration invariants belong to the repository suite over the real client-state store.
func TestWindowManagerSnapshotMigration(t *testing.T) {
	t.Run("Should re-upgrade newer legacy bytes and retain every superseded backup", func(t *testing.T) {
		t.Parallel()
		for _, invalid := range []bool{false, true} {
			f := newDaemonWindowManagerFixture(t)
			ws := windowmanager.WorkspaceID(f.workspace.ID)
			original := daemonLegacySnapshot(t, ws, true)
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), original)
			if _, err := f.repository.Load(t.Context(), ws); err != nil {
				t.Fatal(err)
			}
			newer := bytes.ReplaceAll(daemonLegacySnapshot(t, ws, invalid), []byte(`"old"`), []byte(`"newer"`))
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), newer)
			status, err := f.manager.Migration(t.Context(), ws, "", false)
			want := windowManagerMigrationMigrated
			if invalid {
				want = windowManagerMigrationFailed
			}
			if err != nil || status.State != want || status.Acknowledged || status.Error == nil ||
				!strings.Contains(status.Error.Message, "1 earlier layout backup(s) were set aside") {
				t.Fatalf("re-upgrade invalid=%t status=%+v error=%v", invalid, status, err)
			}
			for _, raw := range [][]byte{original, newer} {
				key := windowManagerLegacyKey(testWindowManagerProfileID)
				if bytes.Equal(raw, original) {
					key = statekeys.SupersededLegacy(testWindowManagerProfileID, raw)
				}
				if !bytes.Equal(getDaemonState(t, &f, key).Value, raw) {
					t.Fatalf("backup %s changed", key)
				}
			}
			ack, err := f.manager.Migration(t.Context(), ws, "ack", false)
			if err != nil || !ack.Acknowledged || ack.State != want {
				t.Fatalf("re-upgrade ack=%+v error=%v", ack, err)
			}
			third := bytes.ReplaceAll(daemonLegacySnapshot(t, ws, true), []byte(`"old"`), []byte(`"third"`))
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), third)
			status, err = f.manager.Migration(t.Context(), ws, "", false)
			if err != nil || status.State != windowManagerMigrationFailed || status.Acknowledged ||
				status.Error == nil || !strings.Contains(status.Error.Message, "2 earlier layout backup(s)") {
				t.Fatalf("second re-upgrade status=%+v error=%v", status, err)
			}
			for _, raw := range [][]byte{original, newer, third} {
				key := statekeys.SupersededLegacy(testWindowManagerProfileID, raw)
				if bytes.Equal(raw, third) {
					key = windowManagerLegacyKey(testWindowManagerProfileID)
				}
				if !bytes.Equal(getDaemonState(t, &f, key).Value, raw) {
					t.Fatalf("second cycle changed backup %s", key)
				}
			}
			// Returning to identical input deduplicates the content-addressed archive.
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), original)
			if _, err := f.repository.Load(t.Context(), ws); err != nil {
				t.Fatal(err)
			}
			archived := getDaemonState(t, &f, statekeys.SupersededLegacy(testWindowManagerProfileID, original))
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), newer)
			if _, err := f.repository.Load(t.Context(), ws); err != nil {
				t.Fatal(err)
			}
			if again := getDaemonState(t, &f, archived.Key); again.Rev != archived.Rev ||
				!bytes.Equal(again.Value, original) {
				t.Fatal("identical superseded input was overwritten")
			}
		}
	})

	t.Run("Should skip corrupt migration records and still expire healthy backups", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), daemonLegacySnapshot(t, ws, false))
		if _, err := f.repository.Load(t.Context(), ws); err != nil {
			t.Fatal(err)
		}
		putDaemonState(t, &f, windowManagerMigrationKey("corrupt-profile"), []byte(`{"state":42}`))
		putDaemonState(t, &f, windowManagerLegacyKey("corrupt-profile"), []byte(`{"original":true}`))
		var logs bytes.Buffer
		f.registry.logger = slog.New(slog.NewJSONHandler(&logs, nil))
		if err := f.registry.expireMigrationBackups(t.Context(), "v0.8.0"); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(logs.String(), "window_manager.migration_retention_skipped") ||
			!bytes.Equal(
				getDaemonState(t, &f, windowManagerLegacyKey("corrupt-profile")).Value,
				[]byte(`{"original":true}`),
			) {
			t.Fatalf("corrupt migration was not retained and diagnosed: %s", logs.String())
		}
		_, err := f.engine.Get(t.Context(), clientstate.WorkspaceID(ws), windowManagerStateDomain,
			windowManagerLegacyKey(testWindowManagerProfileID))
		if !errors.Is(err, clientstate.ErrNotFound) {
			t.Fatalf("healthy backup was not expired: %v", err)
		}
	})
	t.Run(
		"Should stream acknowledgement and both retry outcomes only within the migration partition",
		func(t *testing.T) {
			t.Parallel()
			f := newDaemonWindowManagerFixture(t)
			ws := windowmanager.WorkspaceID(f.workspace.ID)
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID),
				daemonLegacySnapshot(t, ws, true))
			var observed atomic.Int64
			manager, err := windowmanager.NewService(
				f.repository, windowManagerWorkspaceAuthorizer{resolver: f.storeResolver}, nil,
				windowmanager.DefaultConfig(),
				windowmanager.WithEventObserver(func(context.Context, windowmanager.Event) { observed.Add(1) }),
			)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := manager.Close(); err != nil {
					t.Error(err)
				}
			})
			// A nonempty undo history must survive notice-only mutations.
			created := executeDaemonDesktopCreate(t, manager, ws, "extra", "Extra")
			before := created.Snapshot
			if len(before.History.Undo) == 0 {
				t.Fatal("fixture has no undo history")
			}
			hooksBefore := observed.Load()
			subscribe := func(
				owner *windowmanager.Manager,
				workspace windowmanager.WorkspaceID,
				client *windowmanager.ClientID,
			) windowmanager.Subscription {
				t.Helper()
				subscription, err := owner.Subscribe(t.Context(), windowmanager.SubscriptionRequest{
					WorkspaceID: workspace, ClientID: client,
				})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := subscription.Close(); err != nil {
						t.Error(err)
					}
				})
				return subscription
			}
			subscribers := []windowmanager.Subscription{}
			for _, id := range []windowmanager.ClientID{"migration-a", "migration-b"} {
				if _, err := manager.RegisterClient(t.Context(), windowmanager.ClientRegistration{
					WorkspaceID: ws, ClientID: id,
				}); err != nil {
					t.Fatal(err)
				}
				subscribers = append(subscribers, subscribe(manager, ws, &id))
			}
			otherProfile, err := f.registry.For("01JQPROFILEOTHER0000000000")
			if err != nil {
				t.Fatal(err)
			}
			isolated := []windowmanager.Subscription{
				subscribe(manager, windowmanager.GlobalDesktopWorkspaceID, nil),
				subscribe(otherProfile, ws, nil),
			}
			assertIsolated := func() {
				t.Helper()
				for _, subscription := range isolated {
					select {
					case update := <-subscription.Updates():
						t.Fatalf("migration crossed workspace/profile boundary: %+v", update)
					default:
					}
				}
				if observed.Load() != hooksBefore {
					t.Fatal("migration dispatched a command observer and could fire layout/window hooks")
				}
			}
			assertSnapshot := func(
				status *windowmanager.MigrationStatus,
				revision windowmanager.Revision,
				topologyChanged bool,
			) {
				t.Helper()
				for _, subscription := range subscribers {
					if topologyChanged {
						select {
						case update := <-subscription.Updates():
							if update.Event == nil || update.Event.Revision != revision {
								t.Fatalf("retry topology event = %+v", update)
							}
						default:
							t.Fatal("subscriber missed retry topology event")
						}
					}
					select {
					case update := <-subscription.Updates():
						if update.Snapshot == nil || update.Event != nil || update.Client != nil ||
							update.Snapshot.Revision != revision || !reflect.DeepEqual(update.Snapshot.Migration, status) {
							t.Fatalf("migration snapshot = %+v, want status %+v revision %d",
								update, status, revision)
						}
						if !topologyChanged && (!reflect.DeepEqual(update.Snapshot.History, before.History) ||
							!reflect.DeepEqual(update.Snapshot.Desktops, before.Desktops)) {
							t.Fatal("notice changed layout or undo history")
						}
					default:
						t.Fatal("subscriber missed migration snapshot")
					}
				}
				assertIsolated()
			}
			status, err := manager.Migration(t.Context(), ws, "ack", false)
			if err != nil || !status.Acknowledged {
				t.Fatalf("ack = %+v, error = %v", status, err)
			}
			assertSnapshot(nil, before.Revision, false)
			// Retry reopens the notice even when conversion still fails.
			putDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID),
				[]byte(`{"version":5,"windows":{"broken":{}}}`))
			status, err = manager.Migration(t.Context(), ws, "retry", true)
			if err != nil || status.State != "failed" || status.Acknowledged || status.Error == nil ||
				reflect.DeepEqual(status.Error, before.Migration.Error) {
				t.Fatalf("failed retry = %+v, error = %v", status, err)
			}
			assertSnapshot(&status, before.Revision, false)
			putDaemonState(
				t,
				&f,
				windowManagerLegacyKey(testWindowManagerProfileID),
				daemonLegacySnapshot(t, ws, false),
			)
			status, err = manager.Migration(t.Context(), ws, "retry", true)
			if err != nil || status.State != "migrated" || status.Acknowledged || status.Error != nil {
				t.Fatalf("successful retry = %+v, error = %v", status, err)
			}
			assertSnapshot(&status, before.Revision+1, true)
		},
	)

	t.Run("Should expire only successful migration backups starting with v0.8.0", func(t *testing.T) {
		t.Parallel()
		for _, failed := range []bool{false, true} {
			f := newDaemonWindowManagerFixture(t)
			ws := windowmanager.WorkspaceID(f.workspace.ID)
			raw := daemonLegacySnapshot(t, ws, failed)
			putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
			if _, err := f.repository.Load(t.Context(), ws); err != nil {
				t.Fatal(err)
			}
			superseded := daemonLegacySnapshot(t, ws, true)
			archiveKey := statekeys.SupersededLegacy(testWindowManagerProfileID, superseded)
			putDaemonState(t, &f, archiveKey, superseded)
			for _, release := range []string{"dev", "v0.7.0", "v0.8.0-rc.1"} {
				if err := f.registry.expireMigrationBackups(t.Context(), release); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(getDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID)).Value, raw) {
					t.Fatal("backup expired before v0.8.0")
				}
			}
			if err := f.registry.expireMigrationBackups(t.Context(), "v0.8.0"); err != nil {
				t.Fatal(err)
			}
			_, err := f.engine.Get(
				t.Context(),
				clientstate.WorkspaceID(ws),
				windowManagerStateDomain,
				windowManagerLegacyKey(testWindowManagerProfileID),
			)
			if failed && err != nil || !failed && !errors.Is(err, clientstate.ErrNotFound) {
				t.Fatalf("failed=%t backup retention error=%v", failed, err)
			}
			if !bytes.Equal(getDaemonState(t, &f, archiveKey).Value, superseded) {
				t.Fatal("a later successful migration expired superseded failed input")
			}
		}
	})

	t.Run("Should preserve and atomically migrate v5 exactly once (IT-001)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		raw := daemonLegacySnapshot(t, ws, false)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		var logs bytes.Buffer
		f.repository.logger = slog.New(slog.NewJSONHandler(&logs, nil))
		got, err := f.repository.Load(t.Context(), ws)
		if err != nil {
			t.Fatal(err)
		}
		if err := windowmanager.ValidateSnapshot(got); err != nil {
			t.Fatal(err)
		}
		original := getDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID))
		if !bytes.Equal(original.Value, raw) || got.Migration == nil || got.Migration.State != "migrated" ||
			got.Migration.Summary.Windows != 1 {
			t.Fatalf("migration=%+v legacy=%s", got.Migration, original.Value)
		}
		stored := getDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID))
		again, err := f.repository.Load(t.Context(), ws)
		if err != nil {
			t.Fatal(err)
		}
		last := getDaemonState(t, &f, stored.Key)
		if !reflect.DeepEqual(got, again) || stored.Rev != last.Rev ||
			strings.Count(logs.String(), "window_manager.snapshot_migrated") != 1 {
			t.Fatalf("second load changed state: logs=%s", logs.String())
		}
	})
	t.Run("Should retain failed input and normal commits across restart (IT-002)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		raw := daemonLegacySnapshot(t, ws, true)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		assertFailedMigrationSurvivesRestart(t, &f, raw)
	})
	t.Run("Should preserve unparseable bytes and provide a working default (IT-003)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), []byte(`{"placeholder":true}`))
		closeDaemonWindowManagerEngine(t, f.engine)
		raw := []byte("{broken\noriginal bytes\x00")
		db, err := bolt.Open(f.storePath, 0o600, nil)
		if err != nil {
			t.Fatal(err)
		}
		err = db.Update(func(tx *bolt.Tx) error {
			b := tx.Bucket([]byte("ws:" + f.workspace.ID)).Bucket([]byte(windowManagerStateDomain))
			key := []byte(windowManagerSnapshotKey(testWindowManagerProfileID))
			old := b.Get(key)
			encoded := append(append([]byte{}, old[:25]...), raw...)
			return b.Put(key, encoded)
		})
		closeErr := db.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("corrupt fixture: %v close=%v", err, closeErr)
		}
		restartDaemonWindowManagerFixture(t, &f)
		assertFailedMigrationSurvivesRestart(t, &f, raw)
	})
	t.Run("Should acknowledge retry and fence replacement without deleting backup (IT-004)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		raw := daemonLegacySnapshot(t, ws, true)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		if _, err := f.repository.Load(t.Context(), ws); err != nil {
			t.Fatal(err)
		}
		original := raw
		raw = bytes.ReplaceAll(raw, []byte(`"old"`), []byte(`"re-upgraded"`))
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		if _, err := f.manager.Migration(t.Context(), ws, "", false); err != nil {
			t.Fatal(err)
		}
		status, err := f.manager.Migration(t.Context(), ws, "ack", false)
		if err != nil || !status.Acknowledged || status.State != "failed" {
			t.Fatalf("ack=%+v error=%v", status, err)
		}
		s, err := f.repository.Load(t.Context(), ws)
		if err != nil || s.Migration != nil {
			t.Fatalf("ack notice=%+v error=%v", s.Migration, err)
		}
		status, err = f.manager.Migration(t.Context(), ws, "retry", false)
		if err != nil || status.Acknowledged || status.State != "failed" {
			t.Fatalf("failed retry=%+v error=%v", status, err)
		}
		if !bytes.Equal(getDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID)).Value, raw) {
			t.Fatal("failed retry replaced legacy")
		}
		_, err = f.manager.Execute(
			t.Context(),
			windowmanager.CommandRequest{
				WorkspaceID: ws,
				Payload:     windowmanager.CreateDesktopCommand{DesktopID: "extra", Name: "Extra"},
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		before := getDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID))
		_, err = f.manager.Migration(t.Context(), ws, "retry", false)
		if !errors.Is(err, windowmanager.ErrRevisionConflict) {
			t.Fatalf("retry fence=%v", err)
		}
		if !bytes.Equal(getDaemonState(t, &f, before.Key).Value, before.Value) {
			t.Fatal("conflict replaced snapshot")
		}
		// A later converter can repair input that a previous version could not decode.
		valid := bytes.ReplaceAll(daemonLegacySnapshot(t, ws, false), []byte(`"old"`), []byte(`"re-upgraded"`))
		putDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID), valid)
		// A failed converter may have installed a nonzero working revision.
		status.FailureRevision = 1
		if err := f.repository.writeMigrationStatus(t.Context(), ws, status, "retry"); err != nil {
			t.Fatal(err)
		}
		status, err = f.manager.Migration(t.Context(), ws, "retry", true)
		if err != nil || status.State != "migrated" || status.Error == nil || status.Error.Code != "migration.backup_superseded" || status.FailureRevision != 0 {
			t.Fatalf("successful retry=%+v error=%v", status, err)
		}
		if !bytes.Equal(getDaemonState(t, &f, statekeys.SupersededLegacy(testWindowManagerProfileID, original)).Value, original) {
			t.Fatal("retry or acknowledgement changed the superseded backup")
		}
		s, err = f.repository.Load(t.Context(), ws)
		if err != nil || s.Windows["re-upgraded"].ID != "re-upgraded" || s.Windows["old"].ID != "" || s.Revision != 2 ||
			!bytes.Equal(getDaemonState(t, &f, windowManagerLegacyKey(testWindowManagerProfileID)).Value, valid) {
			t.Fatalf("retry snapshot=%+v error=%v", s, err)
		}
	})
	t.Run("Should refuse future state without modifying bytes (IT-005)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		raw := []byte(`{"version":7,"unknown":"future"}`)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		_, err := f.repository.Load(t.Context(), windowmanager.WorkspaceID(f.workspace.ID))
		ahead, ok := errors.AsType[*windowmanager.SnapshotVersionError](err)
		if !errors.Is(err, windowmanager.ErrUnavailable) || !ok || ahead.Diagnostic().Path != "version" {
			t.Fatalf("future=%v", err)
		}
		if !bytes.Equal(raw, getDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID)).Value) {
			t.Fatal("future bytes changed")
		}
	})
	t.Run("Should set aside a different User backup during legacy layout import", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ctx := t.Context()
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		target := clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID)
		original := daemonLegacySnapshot(t, windowmanager.UserDesktopWorkspaceID, true)
		failed, err := json.Marshal(windowmanager.MigrationStatus{State: windowManagerMigrationFailed})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.engine.Apply(ctx, target, windowManagerStateDomain, []clientstate.Op{
			{Kind: clientstate.OpPut, Key: windowManagerLegacyKey(testWindowManagerProfileID), Value: original},
			{Kind: clientstate.OpPut, Key: windowManagerMigrationKey(testWindowManagerProfileID), Value: failed},
		}, clientstate.ApplyOptions{}); err != nil {
			t.Fatal(err)
		}
		raw := daemonLegacySnapshot(t, ws, false)
		putDaemonState(t, &f, windowManagerSnapshotKey(testWindowManagerProfileID), raw)
		if err := migrateUserScopeLayouts(ctx, f.engine, f.workspace.ID, discardLogger()); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{statekeys.SupersededLegacy(testWindowManagerProfileID, original), windowManagerLegacyKey(testWindowManagerProfileID)} {
			entry, err := f.engine.Get(ctx, target, windowManagerStateDomain, key)
			want := raw
			if statekeys.IsSupersededLegacy(key) {
				want = original
			}
			if err != nil || !bytes.Equal(entry.Value, want) {
				t.Fatalf("User backup %s=%s error=%v", key, entry.Value, err)
			}
		}
		status, err := f.repository.Migration(ctx, windowmanager.UserDesktopWorkspaceID, "", false)
		if err != nil || status.State != windowManagerMigrationMigrated || status.Error == nil ||
			status.Error.Code != "migration.backup_superseded" {
			t.Fatalf("User import status=%+v error=%v", status, err)
		}
	})

	t.Run("Should purge all migration keys and move only converted snapshots to User (IT-006)", func(t *testing.T) {
		t.Parallel()
		f := newDaemonWindowManagerFixture(t)
		ws := windowmanager.WorkspaceID(f.workspace.ID)
		raw := daemonLegacySnapshot(t, ws, false)
		for _, key := range []string{windowManagerSnapshotKey(testWindowManagerProfileID), windowManagerLegacyKey(testWindowManagerProfileID), windowManagerMigrationKey(testWindowManagerProfileID)} {
			putDaemonState(t, &f, key, raw)
		}
		if err := migrateUserScopeLayouts(t.Context(), f.engine, f.workspace.ID, discardLogger()); err != nil {
			t.Fatal(err)
		}
		moved, err := f.engine.List(
			t.Context(),
			clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID),
			windowManagerStateDomain,
		)
		if err != nil || len(moved) != 3 {
			t.Fatalf("User=%+v error=%v", moved, err)
		}
		target := clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID)
		stored, err := f.engine.Get(t.Context(), target, windowManagerStateDomain,
			windowManagerSnapshotKey(testWindowManagerProfileID))
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := windowmanager.DecodeSnapshot(stored.Value)
		if err != nil || s.WorkspaceID != windowmanager.UserDesktopWorkspaceID || s.Windows["old"].ID != "old" {
			t.Fatalf("moved=%+v error=%v", s, err)
		}
		status, err := f.repository.Migration(t.Context(), windowmanager.UserDesktopWorkspaceID, "", false)
		if err != nil || status.State != "migrated" || status.Acknowledged || status.FromVersion != 5 ||
			status.Summary == nil || status.Summary.Windows != 1 {
			t.Fatalf("User migration notice=%+v error=%v", status, err)
		}
		backup, err := f.engine.Get(t.Context(), target, windowManagerStateDomain,
			windowManagerLegacyKey(testWindowManagerProfileID))
		if err != nil || !bytes.Equal(backup.Value, raw) {
			t.Fatalf("User backup=%s error=%v", backup.Value, err)
		}
		putDaemonState(t, &f, statekeys.SupersededLegacy(testWindowManagerProfileID, raw), raw)
		if err := f.repository.DeleteWorkspace(t.Context(), ws); err != nil {
			t.Fatal(err)
		}
		assertDaemonMigrationKeysAbsent(t, &f, clientstate.WorkspaceID(ws))
		// An archive without the other three keys still belongs to the deleted profile.
		putDaemonState(t, &f, statekeys.SupersededLegacy(testWindowManagerProfileID, raw), raw)
		if err := f.registry.PurgeDesktopPartitions(t.Context(), testWindowManagerProfileID); err != nil {
			t.Fatal(err)
		}
		assertDaemonMigrationKeysAbsent(t, &f, clientstate.WorkspaceID(ws))
		assertDaemonMigrationKeysAbsent(t, &f, clientstate.WorkspaceID(windowmanager.UserDesktopWorkspaceID))
	})
}

func daemonLegacySnapshot(t *testing.T, ws windowmanager.WorkspaceID, orphan bool) []byte {
	t.Helper()
	s := windowmanager.SnapshotV5{
		Version:     5,
		WorkspaceID: ws,
		Windows: map[windowmanager.WindowID]windowmanager.WindowV5{
			"old": {
				ID:        "old",
				App:       "settings",
				DesktopID: "desktop-default",
				Route:     windowmanager.RouteIntent{Pathname: "/settings", Search: windowmanager.RouteSearch{}},
				NavStack:  []windowmanager.RouteIntent{},
			},
		},
		Desktops: []windowmanager.DesktopV5{
			{ID: "desktop-default", Name: "Primary", Floating: []windowmanager.WindowID{"old"}},
		},
	}
	if orphan {
		s.Desktops[0].Floating = nil
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func putDaemonState(t *testing.T, f *daemonWindowManagerFixture, key string, raw []byte) {
	t.Helper()
	_, err := f.engine.Apply(
		t.Context(),
		clientstate.WorkspaceID(f.workspace.ID),
		windowManagerStateDomain,
		[]clientstate.Op{{Kind: clientstate.OpPut, Key: key, Value: raw}},
		clientstate.ApplyOptions{},
	)
	if err != nil {
		t.Fatal(err)
	}
}
func getDaemonState(t *testing.T, f *daemonWindowManagerFixture, key string) clientstate.Entry {
	t.Helper()
	entry, err := f.engine.Get(t.Context(), clientstate.WorkspaceID(f.workspace.ID), windowManagerStateDomain, key)
	if err != nil {
		t.Fatal(err)
	}
	return entry
}
func assertDaemonMigrationKeysAbsent(t *testing.T, f *daemonWindowManagerFixture, ws clientstate.WorkspaceID) {
	t.Helper()
	entries, err := f.engine.List(t.Context(), ws, windowManagerStateDomain)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if owner, ok := statekeys.Profile(entry.Key); ok && owner == testWindowManagerProfileID {
			t.Fatalf("partition key %s remains", entry.Key)
		}
	}
}
func restartDaemonWindowManagerFixture(t *testing.T, f *daemonWindowManagerFixture) {
	t.Helper()
	if err := f.manager.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.engine.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err := clientstate.Open(
		t.Context(),
		f.storePath,
		f.storeResolver,
		clientstate.Limits{
			MaxValueBytes:       windowManagerMaxSnapshotBytes,
			MaxKeysPerWorkspace: clientstate.DefaultLimits().MaxKeysPerWorkspace,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	f.engine = engine
	t.Cleanup(func() {
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	repository, err := newWindowManagerRepository(engine, testWindowManagerProfileID)
	if err != nil {
		t.Fatal(err)
	}
	f.repository = repository
	manager, err := windowmanager.NewService(
		repository,
		windowManagerWorkspaceAuthorizer{resolver: f.storeResolver},
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatal(err)
	}
	f.manager = manager
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Error(err)
		}
	})
}
func assertFailedMigrationSurvivesRestart(t *testing.T, f *daemonWindowManagerFixture, raw []byte) {
	t.Helper()
	ws := windowmanager.WorkspaceID(f.workspace.ID)
	s, err := f.repository.Load(t.Context(), ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := windowmanager.ValidateSnapshot(s); err != nil {
		t.Fatal(err)
	}
	if s.Migration == nil || s.Migration.State != "failed" || s.Migration.Error == nil {
		t.Fatalf("failure=%+v", s.Migration)
	}
	_, err = f.manager.Execute(
		t.Context(),
		windowmanager.CommandRequest{
			WorkspaceID:      ws,
			ExpectedRevision: s.Revision,
			Payload: windowmanager.OpenWindowCommand{
				Window: windowmanager.WindowSpec{
					ID:    "new",
					App:   "settings",
					Route: windowmanager.RouteIntent{Pathname: "/settings", Search: windowmanager.RouteSearch{}},
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	restartDaemonWindowManagerFixture(t, f)
	s, err = f.repository.Load(t.Context(), ws)
	if err != nil || s.Windows["new"].ID != "new" ||
		!bytes.Equal(getDaemonState(t, f, windowManagerLegacyKey(testWindowManagerProfileID)).Value, raw) {
		t.Fatalf("restart=%+v error=%v", s, err)
	}
}
