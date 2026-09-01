package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/windowmanager"
)

func TestWindowManagerRepository(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a nil commit as invalid topology", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		err := fixture.repository.Commit(testutil.Context(t), nil)
		if !errors.Is(err, windowmanager.ErrInvalidTopology) {
			t.Fatalf("Commit(nil) error = %v, want ErrInvalidTopology", err)
		}
	})

	t.Run("Should persist one full v3 snapshot exactly across store reopen [IT-001]", func(t *testing.T) {
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

	t.Run("Should discard legacy unknown and malformed snapshot documents", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			name  string
			value string
		}{
			{
				name: "legacy v1",
				value: `{"version":1,"workspace_id":"WORKSPACE_ID","revision":1,` +
					`"desktops":[{"id":"desktop-default","name":"One","order":0,"purpose":"standard","groups":[],"floating":[]}],` +
					`"windows":{},"history":{"undo":[],"redo":[]},"overrides":{},"updated_at":"2026-07-22T12:00:00Z"}`,
			},
			{
				name: "unknown field",
				value: `{"version":2,"workspace_id":"WORKSPACE_ID","revision":1,` +
					`"desktops":[{"id":"desktop-default","name":"One","order":0,"purpose":"standard","groups":[],"floating":[]}],` +
					`"windows":{},"history":{"undo":[],"redo":[]},"overrides":{},"updated_at":"2026-07-22T12:00:00Z","legacy":true}`,
			},
			{
				name: "malformed topology",
				value: `{"version":2,"workspace_id":"WORKSPACE_ID","revision":1,"desktops":[],` +
					`"windows":{},"history":{"undo":[],"redo":[]},"overrides":{},"updated_at":"2026-07-22T12:00:00Z"}`,
			},
		}
		for _, testCase := range cases {
			t.Run("Should discard "+testCase.name, func(t *testing.T) {
				t.Parallel()
				fixture := newDaemonWindowManagerFixture(t)
				ctx := testutil.Context(t)
				workspaceID := clientstate.WorkspaceID(fixture.workspace.ID)
				value := []byte(strings.ReplaceAll(
					testCase.value,
					"WORKSPACE_ID",
					fixture.workspace.ID,
				))
				if _, err := fixture.engine.Apply(
					ctx,
					workspaceID,
					windowManagerStateDomain,
					[]clientstate.Op{
						{
							Kind:  clientstate.OpPut,
							Key:   windowManagerSnapshotKey(testWindowManagerProfileID),
							Value: value,
						},
					},
					clientstate.ApplyOptions{},
				); err != nil {
					t.Fatalf("Apply(raw document) error = %v", err)
				}
				if _, err := fixture.repository.Load(
					ctx,
					windowmanager.WorkspaceID(workspaceID),
				); !errors.Is(err, windowmanager.ErrSnapshotNotFound) {
					t.Fatalf("Load() error = %v, want ErrSnapshotNotFound", err)
				}
				if _, err := fixture.engine.Get(
					ctx,
					workspaceID,
					windowManagerStateDomain,
					windowManagerSnapshotKey(testWindowManagerProfileID),
				); !errors.Is(err, clientstate.ErrNotFound) {
					t.Fatalf("Get(after discard) error = %v, want ErrNotFound", err)
				}
			})
		}
	})

	t.Run(
		"Should reinitialize after a discard and commit the next command at revision one [UT-062]",
		func(t *testing.T) {
			t.Parallel()
			fixture := newDaemonWindowManagerFixture(t)
			ctx := testutil.Context(t)
			workspaceID := clientstate.WorkspaceID(fixture.workspace.ID)
			legacy := []byte(strings.ReplaceAll(
				`{"version":2,"workspace_id":"WORKSPACE_ID","revision":7,"desktops":[],"windows":{},"history":{"undo":[],"redo":[]},"overrides":{},"updated_at":"2026-07-22T12:00:00Z"}`,
				"WORKSPACE_ID",
				fixture.workspace.ID,
			))
			if _, err := fixture.engine.Apply(
				ctx,
				workspaceID,
				windowManagerStateDomain,
				[]clientstate.Op{
					{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey(testWindowManagerProfileID), Value: legacy},
				},
				clientstate.ApplyOptions{},
			); err != nil {
				t.Fatalf("Apply(v2 snapshot) error = %v", err)
			}
			result, err := fixture.manager.Execute(ctx, windowmanager.CommandRequest{
				WorkspaceID: windowmanager.WorkspaceID(workspaceID), ExpectedRevision: 0,
				Payload: windowmanager.CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			})
			if err != nil || result.Snapshot.Revision != 1 || result.Snapshot.Version != windowmanager.SnapshotVersion {
				t.Fatalf("Execute(after discard) = %+v, error = %v", result, err)
			}
		},
	)

	t.Run("Should migrate a version 3 arrangement and keep its focus owner zoomed on its desktop", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := clientstate.WorkspaceID(fixture.workspace.ID)
		legacy := []byte(strings.ReplaceAll(`{"version":3,"workspace_id":"WORKSPACE_ID","revision":5,
			"desktops":[
				{"id":"desktop-default","name":"Desktop 1","order":0,"purpose":"standard",
					"groups":[],"floating":["settings"],"floating_stacks":[]},
				{"id":"desktop-focus","name":"Focus — Tasks","order":1,"purpose":"focus","focus_owner":"tasks",
					"groups":[{"id":"group-focus","frame":{"x":0,"y":0,"width":1,"height":1},
						"root":{"id":"leaf-focus","kind":"leaf","window_id":"tasks"}}],
					"floating":[],"floating_stacks":[]}],
			"windows":{
				"tasks":{"id":"tasks","app":"tasks","route":{"pathname":"/tasks","search":{}},"nav_stack":[],
					"pinned":false,"placement":"tiled","desktop_id":"desktop-focus",
					"floating_rect":{"x":0.1,"y":0.1,"width":0.5,"height":0.5},"minimized":false,
					"return_anchor":{"desktop_id":"desktop-default","source_revision":4}},
				"settings":{"id":"settings","app":"settings","route":{"pathname":"/settings","search":{}},"nav_stack":[],
					"pinned":false,"placement":"floating","desktop_id":"desktop-default",
					"floating_rect":{"x":0.2,"y":0.2,"width":0.5,"height":0.5},"minimized":false}},
			"history":{"undo":[],"redo":[]},"overrides":{},"updated_at":"2026-08-30T10:00:00Z"}`,
			"WORKSPACE_ID", fixture.workspace.ID))
		if _, err := fixture.engine.Apply(
			ctx,
			workspaceID,
			windowManagerStateDomain,
			[]clientstate.Op{
				{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey(testWindowManagerProfileID), Value: legacy},
			},
			clientstate.ApplyOptions{},
		); err != nil {
			t.Fatalf("Apply(v3 snapshot) error = %v", err)
		}
		loaded, err := fixture.repository.Load(ctx, windowmanager.WorkspaceID(workspaceID))
		if err != nil {
			t.Fatalf("Load(v3 snapshot) error = %v", err)
		}
		if loaded.Version != windowmanager.SnapshotVersion || loaded.Revision != 6 || len(loaded.Desktops) != 2 {
			t.Fatalf("migrated snapshot = %+v", loaded)
		}
		stored, err := fixture.engine.Get(
			ctx, workspaceID, windowManagerStateDomain, windowManagerSnapshotKey(testWindowManagerProfileID),
		)
		if err != nil {
			t.Fatalf("Get(after migration) error = %v", err)
		}
		var persisted windowmanager.Snapshot
		if err := json.Unmarshal(stored.Value, &persisted); err != nil {
			t.Fatalf("json.Unmarshal(persisted migration) error = %v", err)
		}
		if persisted.Version != windowmanager.SnapshotVersion || persisted.Revision != 6 {
			t.Fatalf(
				"load did not persist the migrated snapshot: version %d revision %d",
				persisted.Version,
				persisted.Revision,
			)
		}
		tasks := loaded.Windows["tasks"]
		if !tasks.Zoomed || tasks.DesktopID != "desktop-focus" ||
			tasks.Placement != windowmanager.WindowPlacementTiled || tasks.ReturnAnchor == nil ||
			tasks.ReturnAnchor.DesktopID != "desktop-default" {
			t.Fatalf("migrated focus owner = %+v", tasks)
		}
		result, err := fixture.manager.Execute(ctx, windowmanager.CommandRequest{
			WorkspaceID: windowmanager.WorkspaceID(workspaceID), ExpectedRevision: 6,
			Payload: windowmanager.CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
		})
		if err != nil || result.Snapshot.Revision != 7 || result.Snapshot.Version != windowmanager.SnapshotVersion {
			t.Fatalf("Execute(after migration) = %+v, error = %v", result, err)
		}
		reloaded, err := fixture.repository.Load(ctx, windowmanager.WorkspaceID(workspaceID))
		if err != nil || reloaded.Version != windowmanager.SnapshotVersion || !reloaded.Windows["tasks"].Zoomed {
			t.Fatalf("Load(after migration commit) = %+v, error = %v", reloaded, err)
		}
	})

	t.Run(
		"Should discard only decode and version classes while preserving forensic blobs [IT-002]",
		func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name        string
				value       func(t *testing.T, workspaceID string) []byte
				wantError   error
				wantDeleted bool
			}{
				{
					name: "Should preserve a workspace mismatch for forensics",
					value: func(t *testing.T, _ string) []byte {
						t.Helper()
						snapshot := daemonWindowManagerSnapshot("another-workspace", 1, "Mismatch")
						encoded, err := json.Marshal(snapshot)
						if err != nil {
							t.Fatalf("json.Marshal(mismatch) error = %v", err)
						}
						return encoded
					},
					wantError: windowmanager.ErrInvalidTopology,
				},
				{
					name: "Should preserve an invalid current topology for forensics",
					value: func(t *testing.T, workspaceID string) []byte {
						t.Helper()
						snapshot := daemonWindowManagerSnapshot(windowmanager.WorkspaceID(workspaceID), 1, "Invalid")
						snapshot.Desktops = nil
						encoded, err := json.Marshal(snapshot)
						if err != nil {
							t.Fatalf("json.Marshal(invalid v3) error = %v", err)
						}
						return encoded
					},
					wantError: windowmanager.ErrInvalidTopology,
				},
				{
					name: "Should delete an unsupported snapshot version",
					value: func(_ *testing.T, workspaceID string) []byte {
						return fmt.Appendf(nil, `{"version":2,"workspace_id":%q}`, workspaceID)
					},
					wantError:   windowmanager.ErrSnapshotNotFound,
					wantDeleted: true,
				},
				{
					name: "Should delete a snapshot with an unknown field",
					value: func(_ *testing.T, workspaceID string) []byte {
						return fmt.Appendf(nil, `{"version":3,"workspace_id":%q,"unknown":true}`, workspaceID)
					},
					wantError:   windowmanager.ErrSnapshotNotFound,
					wantDeleted: true,
				},
			}
			for _, testCase := range cases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					fixture := newDaemonWindowManagerFixture(t)
					ctx := testutil.Context(t)
					workspaceID := clientstate.WorkspaceID(fixture.workspace.ID)
					value := testCase.value(t, fixture.workspace.ID)
					if _, err := fixture.engine.Apply(
						ctx,
						workspaceID,
						windowManagerStateDomain,
						[]clientstate.Op{
							{
								Kind:  clientstate.OpPut,
								Key:   windowManagerSnapshotKey(testWindowManagerProfileID),
								Value: value,
							},
						},
						clientstate.ApplyOptions{},
					); err != nil {
						t.Fatalf("Apply(raw snapshot) error = %v", err)
					}
					_, loadErr := fixture.repository.Load(ctx, windowmanager.WorkspaceID(workspaceID))
					if !errors.Is(loadErr, testCase.wantError) {
						t.Fatalf("Load() error = %v, want %v", loadErr, testCase.wantError)
					}
					entry, getErr := fixture.engine.Get(
						ctx,
						workspaceID,
						windowManagerStateDomain,
						windowManagerSnapshotKey(testWindowManagerProfileID),
					)
					if testCase.wantDeleted {
						if !errors.Is(getErr, clientstate.ErrNotFound) {
							t.Fatalf("Get(after discard) error = %v", getErr)
						}
					} else if getErr != nil || !reflect.DeepEqual(entry.Value, value) {
						t.Fatalf("preserved entry = %+v, error = %v, want bytes %s", entry, getErr, value)
					}
				})
			}
		},
	)

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
	w1, w2, w3 := windowmanager.WindowID("w1"), windowmanager.WindowID("w2"), windowmanager.WindowID("w3")
	active := w2
	route := func(path string) windowmanager.RouteIntent {
		return windowmanager.RouteIntent{Pathname: path, Search: windowmanager.RouteSearch{}}
	}
	snapshot.Windows = map[windowmanager.WindowID]windowmanager.Window{
		w1: {
			ID:           w1,
			App:          "One",
			Route:        route("/one"),
			NavStack:     []windowmanager.RouteIntent{route("/root")},
			Pinned:       true,
			Placement:    windowmanager.WindowPlacementStacked,
			DesktopID:    "desktop-default",
			FloatingRect: windowmanager.NormalizedRect{Width: 1, Height: 1},
		},
		w2: {
			ID:           w2,
			App:          "Two",
			Route:        route("/two"),
			Placement:    windowmanager.WindowPlacementStacked,
			DesktopID:    "desktop-default",
			FloatingRect: windowmanager.NormalizedRect{Width: 1, Height: 1},
		},
	}
	snapshot.Desktops[0].FloatingStacks = []windowmanager.FloatingStack{{
		ID: "stack", WindowIDs: []windowmanager.WindowID{w1, w2}, ActiveID: &active,
		Rect: windowmanager.NormalizedRect{X: 0.1, Y: 0.1, Width: 0.7, Height: 0.7},
	}}
	snapshot.ClosedEntries = []windowmanager.ClosedEntry{
		{
			Windows: []windowmanager.Window{
				{
					ID:           w3,
					App:          "Three",
					Route:        route("/three"),
					DesktopID:    "desktop-default",
					Placement:    windowmanager.WindowPlacementFloating,
					FloatingRect: windowmanager.NormalizedRect{Width: 1, Height: 1},
				},
			},
			DesktopID: "desktop-default",
			Rect:      windowmanager.NormalizedRect{Width: 1, Height: 1},
		},
	}
}
