package daemon

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/agh/internal/clientstate"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/windowmanager"
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

	t.Run("Should persist one typed snapshot exactly across store reopen", func(t *testing.T) {
		t.Parallel()
		fixture := newDaemonWindowManagerFixture(t)
		ctx := testutil.Context(t)
		workspaceID := windowmanager.WorkspaceID(fixture.workspace.ID)
		want := daemonWindowManagerSnapshot(workspaceID, 1, "Primary")
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

	t.Run("Should reject legacy unknown and malformed topology documents", func(t *testing.T) {
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
			t.Run("Should reject "+testCase.name, func(t *testing.T) {
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
				); !errors.Is(err, windowmanager.ErrInvalidTopology) {
					t.Fatalf("Load() error = %v, want ErrInvalidTopology", err)
				}
			})
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
