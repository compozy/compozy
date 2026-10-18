package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspace"
)

// Suite: daemon session work sources
// Invariant: only hierarchy children count as a session's active-child work; a session
// continued or forked from it names it as provenance and never defers its stop.
func TestSessionWorkSourcesChildren(t *testing.T) {
	t.Parallel()

	t.Run("Should not count a session continued from the parent as its child work", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		registry, err := openDaemonTestGlobalDBAtPath(ctx, filepath.Join(t.TempDir(), "compozy.db"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := registry.Close(context.Background()); err != nil {
				t.Errorf("close work store: %v", err)
			}
		})
		now := time.Now().UTC()
		if err := registry.InsertWorkspace(ctx, workspace.Workspace{
			ID: "ws-work", Name: "ws-work", RootDir: t.TempDir(), CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("InsertWorkspace() error = %v", err)
		}
		homePaths := testHomePaths(t)
		if err := store.WriteSessionMeta(
			store.SessionMetaFile(filepath.Join(homePaths.SessionsDir, "sess-source")),
			&store.SessionMeta{
				ID: "sess-source", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-work",
				AgentName: "coder", Provider: "mock", State: string(session.StateStopped),
				RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now,
			},
		); err != nil {
			t.Fatal(err)
		}
		// The continued session has no readable state: counting it would have to read it.
		if err := registry.RegisterSession(ctx, store.SessionInfo{
			ProfileID: store.DefaultProfileID, ID: "sess-continued", AgentName: "reviewer",
			WorkspaceID: "ws-work", SessionType: string(session.SessionTypeUser),
			State: string(session.StateActive), RuntimeStatus: store.SessionRuntimeUnbound,
			Lineage: &store.SessionLineage{
				ParentSessionID: "sess-source", RootSessionID: "sess-source", SpawnDepth: 1,
				Kind: store.LineageKindContinue, OriginAgentName: "coder",
			},
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		manager, err := session.NewManager(session.WithHomePaths(homePaths), session.WithLogger(discardLogger()))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Errorf("shutdown work manager: %v", err)
			}
		})
		sources := sessionWorkSources{state: &bootState{registry: registry}, manager: manager, now: time.Now}

		work, err := sources.children(ctx, "sess-source")
		if err != nil {
			t.Fatalf("children() error = %v, want the continued session ignored", err)
		}
		if len(work) != 0 {
			t.Fatalf("children() = %#v, want no child work", work)
		}
	})
}
