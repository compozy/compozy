package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// archiveStoppedThreadBlocks classifies every listed thread by its first blocking rule.
func archiveStoppedThreadBlocks(members []store.ArchiveStoppedMember) map[string]store.ArchiveStoppedBlock {
	byRoot := map[string][]store.ArchiveStoppedMember{}
	for _, member := range members {
		byRoot[member.RootID] = append(byRoot[member.RootID], member)
	}
	blocks := make(map[string]store.ArchiveStoppedBlock, len(byRoot))
	for root, thread := range byRoot {
		blocks[root] = store.ArchiveStoppedThreadBlock(thread)
	}
	return blocks
}

func execArchiveStoppedFixture(ctx context.Context, t *testing.T, db *GlobalDB, statements ...string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := db.db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

func insertArchiveStoppedInput(ctx context.Context, t *testing.T, db *GlobalDB, id, sessionID, status string) {
	t.Helper()
	now := store.FormatTimestamp(time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	if _, err := db.db.ExecContext(ctx, `INSERT INTO session_input_queue (
		id, session_id, status, mode, text, enqueued_at, updated_at
	) VALUES (?, ?, ?, ?, 'hello', ?, ?)`, id, sessionID, status, store.SessionInputQueueModeQueue, now, now); err != nil {
		t.Fatal(err)
	}
}

func insertArchiveStoppedInteraction(ctx context.Context, t *testing.T, db *GlobalDB, id, sessionID, status string) {
	t.Helper()
	if _, err := db.db.ExecContext(ctx, `INSERT INTO session_pending_interactions (
		interaction_id, session_id, kind, provider_request_id, status, created_at
	) VALUES (?, ?, 'permission', ?, ?, ?)`, id, sessionID, id, status,
		store.FormatTimestamp(time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveStoppedMembers(t *testing.T) {
	t.Parallel()

	// Invariant: a thread is a candidate only when no unarchived member breaks a
	// rule, and it is classified by the first rule in skip-reason order.
	t.Run("Should classify each thread by its first blocking fact", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, _, now := subagentFixture(t)
		for _, root := range []string{
			"clean", "running", "pending", "orphaned", "queued", "dispatching", "pinned", "later", "main", "mixed",
		} {
			registerSubagentSession(t, db, workspace, root, "", "", now)
			registerSubagentSession(t, db, workspace, root+"-child", root, "worker", now)
		}
		registerSubagentSession(t, db, workspace, "archived", "", "", now)
		registerSubagentSession(t, db, workspace, "clean-old-child", "clean", "worker", now)
		execArchiveStoppedFixture(
			ctx,
			t,
			db,
			`UPDATE sessions SET state='active' WHERE id='running-child'`,
			`UPDATE sessions SET pinned_at='2026-11-21T12:00:00Z' WHERE id='pinned-child'`,
			`UPDATE sessions SET parked_at='2026-11-21T12:00:00Z' WHERE id='later-child'`,
			`UPDATE sessions SET archived_at='2026-11-20T12:00:00Z' WHERE id IN ('archived','clean-old-child')`,
			// Pinned and later together classify as pinned, the earlier reason.
			`UPDATE sessions SET pinned_at='2026-11-21T12:00:00Z', parked_at='2026-11-21T12:00:00Z' WHERE id='mixed-child'`,
			`INSERT INTO agent_instances (id, profile_id, agent_name, workspace_id, state, created_by_kind,
				created_by_id, created_at, updated_at, main_session_id) VALUES ('inst', '`+store.DefaultProfileID+`',
				'writer', '`+workspace+`', 'active', 'human', 'operator', '2026-11-21T12:00:00Z',
				'2026-11-21T12:00:00Z', 'main')`,
		)
		insertArchiveStoppedInteraction(ctx, t, db, "perm-pending", "pending-child", "pending")
		insertArchiveStoppedInteraction(ctx, t, db, "perm-orphaned", "orphaned-child", "orphaned")
		insertArchiveStoppedInteraction(ctx, t, db, "perm-resolved", "clean-child", "resolved")
		insertArchiveStoppedInput(ctx, t, db, "in-queued", "queued-child", store.SessionInputQueueStatusQueued)
		insertArchiveStoppedInput(ctx, t, db, "in-dispatching", "dispatching-child", "dispatching")
		insertArchiveStoppedInput(ctx, t, db, "in-sent", "clean-child", "sent")

		members, err := db.ListArchiveStoppedMembers(ctx, store.ArchiveStoppedQuery{
			ProfileID: store.DefaultProfileID, WorkspaceID: workspace,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := archiveStoppedThreadBlocks(members)
		want := map[string]store.ArchiveStoppedBlock{
			"parent":      store.ArchiveStoppedBlockNone,
			"clean":       store.ArchiveStoppedBlockNone,
			"running":     store.ArchiveStoppedBlockRunning,
			"pending":     store.ArchiveStoppedBlockWaitingOnYou,
			"orphaned":    store.ArchiveStoppedBlockWaitingOnYou,
			"queued":      store.ArchiveStoppedBlockQueuedInput,
			"dispatching": store.ArchiveStoppedBlockQueuedInput,
			"pinned":      store.ArchiveStoppedBlockPinned,
			"later":       store.ArchiveStoppedBlockLater,
			"main":        store.ArchiveStoppedBlockInstanceMain,
			"mixed":       store.ArchiveStoppedBlockPinned,
		}
		if len(got) != len(want) {
			t.Fatalf("thread blocks = %v, want %v", got, want)
		}
		for root, block := range want {
			if got[root] != block {
				t.Fatalf("thread %s block = %q, want %q (all: %v)", root, got[root], block, got)
			}
		}
		var clean []string
		for _, member := range members {
			if member.RootID == "clean" {
				clean = append(clean, member.SessionID)
			}
		}
		if want := []string{"clean", "clean-child"}; !slices.Equal(clean, want) {
			t.Fatalf("clean thread members = %v, want %v root first without archived members", clean, want)
		}
	})

	t.Run("Should scope all workspaces to unarchived workspaces of one profile", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, _, now := subagentFixture(t)
		other := registerWorkspaceForGlobalTests(t, db, "archive-stopped-other", filepath.Join(t.TempDir(), "o"))
		closed := registerWorkspaceForGlobalTests(t, db, "archive-stopped-closed", filepath.Join(t.TempDir(), "c"))
		registerSubagentSession(t, db, other, "other-root", "", "", now)
		registerSubagentSession(t, db, closed, "closed-root", "", "", now)
		theirs := store.SessionInfo{
			ID: "theirs", ProfileID: insertLayoutTestProfile(ctx, t, db), AgentName: "coder", Provider: "claude",
			WorkspaceID: workspace, RuntimeStatus: store.SessionRuntimeUnbound, SessionType: defaultSessionType,
			State: "stopped", CreatedAt: now, UpdatedAt: now,
		}
		if err := db.RegisterSession(ctx, &theirs); err != nil {
			t.Fatal(err)
		}
		execArchiveStoppedFixture(ctx, t, db,
			`UPDATE workspaces SET archived_at='2026-11-21T12:00:00Z' WHERE id='`+closed+`'`)
		roots := func(query store.ArchiveStoppedQuery) []string {
			t.Helper()
			members, err := db.ListArchiveStoppedMembers(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]string, 0, len(members))
			for _, member := range members {
				ids = append(ids, member.RootID)
			}
			return ids
		}
		if got := roots(store.ArchiveStoppedQuery{ProfileID: store.DefaultProfileID}); !slices.Equal(
			got, []string{"other-root", "parent"},
		) {
			t.Fatalf("all workspaces roots = %v, want other-root and parent only", got)
		}
		if got := roots(store.ArchiveStoppedQuery{
			ProfileID: store.DefaultProfileID, WorkspaceID: workspace,
		}); !slices.Equal(got, []string{"parent"}) {
			t.Fatalf("workspace roots = %v, want parent only", got)
		}
	})
}

func TestArchiveStoppedSessionTree(t *testing.T) {
	t.Parallel()

	// Invariant: the archive transaction re-checks every rule, so a thread that
	// changed after the candidate read is refused whole.
	t.Run("Should refuse a thread that gained queued input after the read", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		registerSubagentSession(t, db, workspace, "child", parent, "worker", now)
		members, err := db.ListArchiveStoppedMembers(ctx, store.ArchiveStoppedQuery{
			ProfileID: store.DefaultProfileID, WorkspaceID: workspace,
		})
		if err != nil || store.ArchiveStoppedThreadBlock(members) != store.ArchiveStoppedBlockNone {
			t.Fatalf("candidate read = %v, %v, want an eligible thread", members, err)
		}
		insertArchiveStoppedInput(ctx, t, db, "in-late", "child", store.SessionInputQueueStatusQueued)

		_, err = db.ArchiveStoppedSessionTree(ctx, workspace, parent)
		ineligible, ok := errors.AsType[*store.ArchiveStoppedIneligibleError](err)
		if !ok || ineligible.Block != store.ArchiveStoppedBlockQueuedInput {
			t.Fatalf("ArchiveStoppedSessionTree() error = %v, want queued_input refusal", err)
		}
		for _, id := range []string{parent, "child"} {
			if at, err := db.SessionArchivedAt(ctx, workspace, id); err != nil || at != nil {
				t.Fatalf("member %s archived_at = %v, %v, want unarchived", id, at, err)
			}
		}
	})

	t.Run("Should return only the members this archive archived", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db, workspace, parent, now := subagentFixture(t)
		registerSubagentSession(t, db, workspace, "child", parent, "worker", now)
		registerSubagentSession(t, db, workspace, "earlier", parent, "worker", now)
		execArchiveStoppedFixture(ctx, t, db,
			`UPDATE sessions SET archived_at='2026-11-20T12:00:00Z' WHERE id='earlier'`)

		tree, err := db.ArchiveStoppedSessionTree(ctx, workspace, parent)
		if err != nil || tree.Root.ArchivedAt == nil || len(tree.Descendants) != 1 ||
			tree.Descendants[0].ID != "child" {
			t.Fatalf("ArchiveStoppedSessionTree() = %#v, %v, want parent and child only", tree, err)
		}
		if _, err := db.ArchiveStoppedSessionTree(ctx, workspace, parent); !errors.Is(
			err, store.ErrArchiveStoppedNotCandidate,
		) {
			t.Fatalf("second archive error = %v, want not a candidate", err)
		}
	})
}
