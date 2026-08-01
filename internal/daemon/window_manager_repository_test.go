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
		repository, err := newWindowManagerRepository(reopened)
		if err != nil {
			t.Fatalf("newWindowManagerRepository(reopened) error = %v", err)
		}
		got, err := repository.Load(ctx, workspaceID)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Load() = %#v, want %#v", got, want)
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
				Key:   windowManagerSnapshotKey,
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
					[]clientstate.Op{{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey, Value: value}},
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
					windowManagerSnapshotKey,
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
				[]clientstate.Op{{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey, Value: legacy}},
				clientstate.ApplyOptions{},
			); err != nil {
				t.Fatalf("Apply(v2 snapshot) error = %v", err)
			}
			result, err := fixture.manager.Execute(ctx, windowmanager.CommandRequest{
				WorkspaceID: windowmanager.WorkspaceID(workspaceID), ExpectedRevision: 0,
				Payload: windowmanager.CreateDesktopCommand{DesktopID: "d2", Name: "Two"},
			})
			if err != nil || result.Snapshot.Revision != 1 || result.Snapshot.Version != 3 {
				t.Fatalf("Execute(after discard) = %+v, error = %v", result, err)
			}
		},
	)

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
						[]clientstate.Op{{Kind: clientstate.OpPut, Key: windowManagerSnapshotKey, Value: value}},
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
						windowManagerSnapshotKey,
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
