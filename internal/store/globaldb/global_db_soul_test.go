package globaldb

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBSoulSnapshotStore(t *testing.T) {
	t.Parallel()

	t.Run("Should persist resolver output and reuse an existing snapshot for the same digest", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		root := t.TempDir()
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-resolver-workspace", root)
		sourcePath := filepath.Join(root, "agents", "reviewer", soul.FileName)
		cfg := aghconfig.DefaultSoulConfig()
		content := []byte(`---
version: "1"
role: reviewer
tone:
  - direct
principles:
  - Keep scope tight
---
Guide the work without operational claims.
`)

		resolved, err := soul.Parse(ctx, soul.ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: root,
			Content:       content,
			Config:        cfg,
		})
		if err != nil {
			t.Fatalf("soul.Parse() error = %v", err)
		}
		if !resolved.Valid || resolved.Digest == "" {
			t.Fatalf("resolved soul = %#v, want valid digest", resolved)
		}
		provenance, err := soul.NewConfigProvenance(cfg, "test")
		if err != nil {
			t.Fatalf("NewConfigProvenance() error = %v", err)
		}
		createdAt := time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC)
		snapshot, err := soul.SnapshotFromResolved(
			"snap-resolver",
			workspaceID,
			"reviewer",
			&resolved,
			provenance,
			createdAt,
		)
		if err != nil {
			t.Fatalf("SnapshotFromResolved() error = %v", err)
		}

		saved, err := globalDB.UpsertSoulSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatalf("UpsertSoulSnapshot(first) error = %v", err)
		}
		duplicate := snapshot
		duplicate.ID = "snap-resolver-duplicate"
		reused, err := globalDB.UpsertSoulSnapshot(ctx, duplicate)
		if err != nil {
			t.Fatalf("UpsertSoulSnapshot(duplicate digest) error = %v", err)
		}
		if reused.ID != saved.ID {
			t.Fatalf("reused.ID = %q, want existing snapshot id %q", reused.ID, saved.ID)
		}

		found, ok, err := globalDB.FindSoulSnapshotByDigest(ctx, workspaceID, "reviewer", resolved.Digest)
		if err != nil {
			t.Fatalf("FindSoulSnapshotByDigest() error = %v", err)
		}
		if !ok || found.ID != saved.ID {
			t.Fatalf("FindSoulSnapshotByDigest() = %#v, %v; want saved snapshot", found, ok)
		}
		got, err := globalDB.GetSoulSnapshot(ctx, saved.ID)
		if err != nil {
			t.Fatalf("GetSoulSnapshot() error = %v", err)
		}
		if got.Digest != resolved.Digest || got.Body != resolved.ReadModel.Body || !got.CreatedAt.Equal(createdAt) {
			t.Fatalf("GetSoulSnapshot() = %#v, want resolver digest/body/timestamp", got)
		}
		list, err := globalDB.ListSoulSnapshots(
			ctx,
			soul.SnapshotListQuery{WorkspaceID: workspaceID, AgentName: "reviewer"},
		)
		if err != nil {
			t.Fatalf("ListSoulSnapshots() error = %v", err)
		}
		if got, want := len(list), 1; got != want {
			t.Fatalf("len(ListSoulSnapshots()) = %d, want %d", got, want)
		}

		var profile soul.SnapshotProfile
		if err := json.Unmarshal(saved.ProfileJSON, &profile); err != nil {
			t.Fatalf("Unmarshal(ProfileJSON) error = %v", err)
		}
		if profile.ConfigProvenance.Digest == "" || profile.ConfigProvenance.MaxBodyBytes != cfg.MaxBodyBytes {
			t.Fatalf("ConfigProvenance = %#v, want digest and config limits", profile.ConfigProvenance)
		}
		if profile.ReadModel.Body != resolved.ReadModel.Body || profile.Compact.Role != "reviewer" {
			t.Fatalf("SnapshotProfile = %#v, want full read model and compact projection", profile)
		}
		if strings.Contains(string(saved.ProfileJSON), root) {
			t.Fatalf("ProfileJSON contains absolute workspace root %q: %s", root, string(saved.ProfileJSON))
		}
	})

	t.Run("Should reuse one snapshot for concurrent duplicate digest upserts", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-concurrent-digest", t.TempDir())
		snapshot := soulSnapshotForTest("snap-concurrent-base", workspaceID, "coder", "sha256:concurrent-soul")
		type outcome struct {
			snapshot soul.Snapshot
			err      error
		}
		const upsertCount = 32
		start := make(chan struct{})
		outcomes := make(chan outcome, upsertCount)
		for index := range upsertCount {
			go func() {
				<-start
				candidate := snapshot
				candidate.ID = "snap-concurrent-" + strconv.Itoa(index)
				saved, err := globalDB.UpsertSoulSnapshot(ctx, candidate)
				outcomes <- outcome{snapshot: saved, err: err}
			}()
		}

		close(start)
		winnerID := ""
		for index := range upsertCount {
			got := <-outcomes
			if got.err != nil {
				t.Fatalf("UpsertSoulSnapshot(concurrent duplicate %d) error = %v", index, got.err)
			}
			if got.snapshot.Digest != snapshot.Digest {
				t.Fatalf("snapshot.Digest = %q, want %q", got.snapshot.Digest, snapshot.Digest)
			}
			if winnerID == "" {
				winnerID = got.snapshot.ID
			}
			if got.snapshot.ID != winnerID {
				t.Fatalf("snapshot.ID = %q, want single winner %q", got.snapshot.ID, winnerID)
			}
		}

		var rowCount int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*)
			FROM agent_soul_snapshots
			WHERE workspace_id = ? AND agent_name = ? AND digest = ?`,
			workspaceID,
			"coder",
			snapshot.Digest,
		).Scan(&rowCount); err != nil {
			t.Fatalf("QueryRowContext(agent_soul_snapshots count) error = %v", err)
		}
		if rowCount != 1 {
			t.Fatalf("agent_soul_snapshots count = %d, want 1", rowCount)
		}
	})

	t.Run("Should reject duplicate snapshot ids and malformed profile JSON", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-snapshot-constraints", t.TempDir())

		first := soulSnapshotForTest("snap-constraint", workspaceID, "coder", "sha256:first")
		if _, err := globalDB.UpsertSoulSnapshot(ctx, first); err != nil {
			t.Fatalf("UpsertSoulSnapshot(first) error = %v", err)
		}
		duplicateID := soulSnapshotForTest("snap-constraint", workspaceID, "coder", "sha256:second")
		if _, err := globalDB.UpsertSoulSnapshot(ctx, duplicateID); err == nil {
			t.Fatal("UpsertSoulSnapshot(duplicate id) error = nil, want constraint failure")
		}
		malformed := soulSnapshotForTest("snap-malformed", workspaceID, "coder", "sha256:malformed")
		malformed.ProfileJSON = json.RawMessage(`{`)
		if _, err := globalDB.UpsertSoulSnapshot(ctx, malformed); !errors.Is(err, soul.ErrInvalidSnapshot) {
			t.Fatalf("UpsertSoulSnapshot(malformed) error = %v, want ErrInvalidSnapshot", err)
		}
	})

	t.Run("Should cascade Soul snapshots and revisions when their workspace is deleted", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-cascade", t.TempDir())

		snapshot := soulSnapshotForTest("snap-cascade", workspaceID, "coder", "sha256:cascade")
		if _, err := globalDB.UpsertSoulSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("UpsertSoulSnapshot() error = %v", err)
		}
		revision := soulRevisionForTest(
			"rev-cascade",
			workspaceID,
			"coder",
			soul.RevisionActionPut,
			"",
			"sha256:cascade",
		)
		if _, err := globalDB.AppendSoulRevision(ctx, revision); err != nil {
			t.Fatalf("AppendSoulRevision() error = %v", err)
		}

		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID); err != nil {
			t.Fatalf("DELETE workspaces error = %v", err)
		}
		snapshots, err := globalDB.ListSoulSnapshots(ctx, soul.SnapshotListQuery{WorkspaceID: workspaceID})
		if err != nil {
			t.Fatalf("ListSoulSnapshots(after cascade) error = %v", err)
		}
		if len(snapshots) != 0 {
			t.Fatalf("snapshots after workspace delete = %#v, want none", snapshots)
		}
		revisions, err := globalDB.ListSoulRevisions(ctx, soul.RevisionListQuery{WorkspaceID: workspaceID})
		if err != nil {
			t.Fatalf("ListSoulRevisions(after cascade) error = %v", err)
		}
		if len(revisions) != 0 {
			t.Fatalf("revisions after workspace delete = %#v, want none", revisions)
		}
	})
}

func TestGlobalDBSoulRevisionStore(t *testing.T) {
	t.Parallel()

	t.Run("Should append authoring history and find the requested rollback revision", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-revisions", t.TempDir())

		diagnosticsJSON, err := soul.DiagnosticsJSON([]soul.Diagnostic{{
			Code:       "soul_conflict",
			Message:    "stale expected digest",
			SourcePath: "agents/coder/SOUL.md",
		}})
		if err != nil {
			t.Fatalf("DiagnosticsJSON() error = %v", err)
		}
		first := soulRevisionForTest("rev-first", workspaceID, "coder", soul.RevisionActionPut, "", "sha256:first")
		first.Body = "first body"
		first.DiagnosticsJSON = diagnosticsJSON
		if _, err := globalDB.AppendSoulRevision(ctx, first); err != nil {
			t.Fatalf("AppendSoulRevision(first) error = %v", err)
		}
		second := soulRevisionForTest(
			"rev-second",
			workspaceID,
			"coder",
			soul.RevisionActionPut,
			"sha256:first",
			"sha256:second",
		)
		second.Body = "second body"
		second.CreatedAt = first.CreatedAt.Add(time.Minute)
		if _, err := globalDB.AppendSoulRevision(ctx, second); err != nil {
			t.Fatalf("AppendSoulRevision(second) error = %v", err)
		}
		deleted := soulRevisionForTest(
			"rev-delete",
			workspaceID,
			"coder",
			soul.RevisionActionDelete,
			"sha256:second",
			"",
		)
		deleted.Body = ""
		deleted.CreatedAt = second.CreatedAt.Add(time.Minute)
		if _, err := globalDB.AppendSoulRevision(ctx, deleted); err != nil {
			t.Fatalf("AppendSoulRevision(delete) error = %v", err)
		}

		revisions, err := globalDB.ListSoulRevisions(
			ctx,
			soul.RevisionListQuery{WorkspaceID: workspaceID, AgentName: "coder"},
		)
		if err != nil {
			t.Fatalf("ListSoulRevisions() error = %v", err)
		}
		if got, want := revisionIDs(
			revisions,
		), []string{
			"rev-delete",
			"rev-second",
			"rev-first",
		}; !testutil.EqualStringSlices(
			got,
			want,
		) {
			t.Fatalf("revision ids = %#v, want %#v", got, want)
		}
		rollback, err := globalDB.FindSoulRevisionForRollback(ctx, soul.RollbackLookup{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
			RevisionID:  "rev-first",
		})
		if err != nil {
			t.Fatalf("FindSoulRevisionForRollback(first) error = %v", err)
		}
		if rollback.Body != "first body" || rollback.NewDigest != "sha256:first" {
			t.Fatalf("rollback revision = %#v, want first body and digest", rollback)
		}
		if _, err := globalDB.FindSoulRevisionForRollback(ctx, soul.RollbackLookup{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
			RevisionID:  "rev-delete",
		}); !errors.Is(err, soul.ErrRevisionNotFound) {
			t.Fatalf("FindSoulRevisionForRollback(delete) error = %v, want ErrRevisionNotFound", err)
		}
		got, err := globalDB.GetSoulRevision(ctx, "rev-first")
		if err != nil {
			t.Fatalf("GetSoulRevision(first) error = %v", err)
		}
		var diagnostics []soul.Diagnostic
		if err := json.Unmarshal(got.DiagnosticsJSON, &diagnostics); err != nil {
			t.Fatalf("Unmarshal(DiagnosticsJSON) error = %v", err)
		}
		if len(diagnostics) != 1 || diagnostics[0].Code != "soul_conflict" {
			t.Fatalf("diagnostics = %#v, want persisted redacted diagnostics", diagnostics)
		}
		if _, err := globalDB.AppendSoulRevision(ctx, first); err == nil {
			t.Fatal("AppendSoulRevision(duplicate id) error = nil, want append-only constraint failure")
		}
	})

	t.Run("Should reject invalid revision actions and missing workspace references", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-invalid-revisions", t.TempDir())

		invalid := soulRevisionForTest(
			"rev-invalid",
			workspaceID,
			"coder",
			soul.RevisionAction("rewrite"),
			"",
			"sha256:invalid",
		)
		if _, err := globalDB.AppendSoulRevision(ctx, invalid); !errors.Is(err, soul.ErrInvalidRevision) {
			t.Fatalf("AppendSoulRevision(invalid action) error = %v, want ErrInvalidRevision", err)
		}
		missingWorkspace := soulRevisionForTest(
			"rev-missing-workspace",
			"ws-missing",
			"coder",
			soul.RevisionActionPut,
			"",
			"sha256:missing",
		)
		if _, err := globalDB.AppendSoulRevision(ctx, missingWorkspace); err == nil {
			t.Fatal("AppendSoulRevision(missing workspace) error = nil, want foreign key failure")
		}
	})
}

func TestGlobalDBSoulSessionProvenance(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should persist session Soul references across reopen and clear snapshot id on snapshot delete",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			first, err := OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(first) error = %v", err)
			}
			workspaceID := registerWorkspaceForGlobalTests(t, first, "soul-session-reopen", t.TempDir())
			snapshot := soulSnapshotForTest("snap-session", workspaceID, "coder", "sha256:session")
			saved, err := first.UpsertSoulSnapshot(ctx, snapshot)
			if err != nil {
				t.Fatalf("UpsertSoulSnapshot() error = %v", err)
			}
			now := time.Date(2026, 5, 2, 15, 0, 0, 0, time.UTC)
			if err := first.RegisterSession(ctx, store.SessionInfo{
				ID:               "sess-soul",
				AgentName:        "coder",
				Provider:         "claude",
				WorkspaceID:      workspaceID,
				State:            "active",
				SoulSnapshotID:   saved.ID,
				SoulDigest:       saved.Digest,
				ParentSoulDigest: "sha256:parent",
				CreatedAt:        now,
				UpdatedAt:        now,
			}); err != nil {
				t.Fatalf("RegisterSession() error = %v", err)
			}
			assertSessionSoulProvenance(t, first, "sess-soul", saved.ID, saved.Digest, "sha256:parent")
			if err := first.Close(ctx); err != nil {
				t.Fatalf("Close(first) error = %v", err)
			}

			second, err := OpenGlobalDB(ctx, path)
			if err != nil {
				t.Fatalf("OpenGlobalDB(second) error = %v", err)
			}
			t.Cleanup(func() {
				if err := second.Close(testutil.Context(t)); err != nil {
					t.Fatalf("Close(second) error = %v", err)
				}
			})
			assertSessionSoulProvenance(t, second, "sess-soul", saved.ID, saved.Digest, "sha256:parent")

			if err := second.UpdateSessionSoulSnapshot(ctx, store.SessionSoulSnapshotUpdate{
				ID:               "sess-soul",
				SoulSnapshotID:   saved.ID,
				SoulDigest:       saved.Digest,
				ParentSoulDigest: "sha256:updated-parent",
				UpdatedAt:        now.Add(time.Hour),
			}); err != nil {
				t.Fatalf("UpdateSessionSoulSnapshot() error = %v", err)
			}
			assertSessionSoulProvenance(t, second, "sess-soul", saved.ID, saved.Digest, "sha256:updated-parent")

			if _, err := second.db.ExecContext(
				ctx,
				`DELETE FROM agent_soul_snapshots WHERE id = ?`,
				saved.ID,
			); err != nil {
				t.Fatalf("DELETE agent_soul_snapshots error = %v", err)
			}
			assertSessionSoulProvenance(t, second, "sess-soul", "", saved.Digest, "sha256:updated-parent")
		},
	)
}

func TestGlobalDBSoulStoreDefaultsAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should generate ids timestamps and preserve truncated snapshots", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-store-defaults", t.TempDir())

		snapshot := soulSnapshotForTest("", workspaceID, "coder", "sha256:auto")
		snapshot.Truncated = true
		snapshot.CreatedAt = time.Time{}
		saved, err := globalDB.UpsertSoulSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatalf("UpsertSoulSnapshot(auto) error = %v", err)
		}
		if saved.ID == "" || saved.CreatedAt.IsZero() || !saved.Truncated {
			t.Fatalf("saved snapshot = %#v, want generated id/time and truncated flag", saved)
		}
		got, err := globalDB.GetSoulSnapshot(ctx, saved.ID)
		if err != nil {
			t.Fatalf("GetSoulSnapshot(auto) error = %v", err)
		}
		if !got.Truncated {
			t.Fatalf("GetSoulSnapshot(auto).Truncated = false, want true")
		}

		revision := soulRevisionForTest(
			"",
			workspaceID,
			"coder",
			soul.RevisionActionRollback,
			"sha256:old",
			"sha256:auto",
		)
		revision.CreatedAt = time.Time{}
		appended, err := globalDB.AppendSoulRevision(ctx, revision)
		if err != nil {
			t.Fatalf("AppendSoulRevision(auto) error = %v", err)
		}
		if appended.ID == "" || appended.CreatedAt.IsZero() {
			t.Fatalf("appended revision = %#v, want generated id/time", appended)
		}
	})

	t.Run("Should return typed errors for invalid and missing Soul store lookups", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "soul-store-errors", t.TempDir())

		if _, err := globalDB.GetSoulSnapshot(ctx, ""); !errors.Is(err, soul.ErrInvalidSnapshot) {
			t.Fatalf("GetSoulSnapshot(empty) error = %v, want ErrInvalidSnapshot", err)
		}
		if _, err := globalDB.GetSoulSnapshot(ctx, "snap-missing"); !errors.Is(err, soul.ErrSnapshotNotFound) {
			t.Fatalf("GetSoulSnapshot(missing) error = %v, want ErrSnapshotNotFound", err)
		}
		if _, _, err := globalDB.FindSoulSnapshotByDigest(ctx, "", "coder", "sha256:missing"); !errors.Is(
			err,
			soul.ErrInvalidSnapshot,
		) {
			t.Fatalf("FindSoulSnapshotByDigest(invalid) error = %v, want ErrInvalidSnapshot", err)
		}
		if _, err := globalDB.ListSoulSnapshots(ctx, soul.SnapshotListQuery{Limit: -1}); err == nil {
			t.Fatal("ListSoulSnapshots(negative limit) error = nil, want non-nil")
		}

		if _, err := globalDB.GetSoulRevision(ctx, ""); !errors.Is(err, soul.ErrInvalidRevision) {
			t.Fatalf("GetSoulRevision(empty) error = %v, want ErrInvalidRevision", err)
		}
		if _, err := globalDB.GetSoulRevision(ctx, "rev-missing"); !errors.Is(err, soul.ErrRevisionNotFound) {
			t.Fatalf("GetSoulRevision(missing) error = %v, want ErrRevisionNotFound", err)
		}
		if _, err := globalDB.ListSoulRevisions(ctx, soul.RevisionListQuery{Limit: -1}); err == nil {
			t.Fatal("ListSoulRevisions(negative limit) error = nil, want non-nil")
		}
		if _, err := globalDB.FindSoulRevisionForRollback(ctx, soul.RollbackLookup{}); err == nil {
			t.Fatal("FindSoulRevisionForRollback(invalid) error = nil, want non-nil")
		}

		if err := globalDB.UpdateSessionSoulSnapshot(ctx, store.SessionSoulSnapshotUpdate{}); err == nil {
			t.Fatal("UpdateSessionSoulSnapshot(empty) error = nil, want validation failure")
		}
		if err := globalDB.UpdateSessionSoulSnapshot(ctx, store.SessionSoulSnapshotUpdate{
			ID:             "sess-invalid",
			SoulSnapshotID: "snap-invalid",
		}); err == nil {
			t.Fatal("UpdateSessionSoulSnapshot(snapshot without digest) error = nil, want validation failure")
		}
		if err := globalDB.UpdateSessionSoulSnapshot(ctx, store.SessionSoulSnapshotUpdate{
			ID:               "sess-missing",
			ParentSoulDigest: "sha256:parent",
		}); err == nil {
			t.Fatal("UpdateSessionSoulSnapshot(missing session) error = nil, want not found")
		}

		snapshot := soulSnapshotForTest("snap-errors", workspaceID, "coder", "sha256:errors")
		if _, err := globalDB.UpsertSoulSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("UpsertSoulSnapshot(errors) error = %v", err)
		}
		found, ok, err := globalDB.FindSoulSnapshotByDigest(ctx, workspaceID, "coder", "sha256:absent")
		if err != nil {
			t.Fatalf("FindSoulSnapshotByDigest(absent) error = %v", err)
		}
		if ok || found.ID != "" {
			t.Fatalf("FindSoulSnapshotByDigest(absent) = %#v, %v; want empty false", found, ok)
		}
	})
}

func soulSnapshotForTest(id string, workspaceID string, agentName string, digest string) soul.Snapshot {
	return soul.Snapshot{
		ID:          id,
		WorkspaceID: workspaceID,
		AgentName:   agentName,
		SourcePath:  "agents/" + agentName + "/" + soul.FileName,
		Digest:      digest,
		ProfileJSON: json.RawMessage(`{"schema_version":1,"valid":true}`),
		Body:        "stored body",
		Truncated:   false,
		CreatedAt:   time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC),
	}
}

func soulRevisionForTest(
	id string,
	workspaceID string,
	agentName string,
	action soul.RevisionAction,
	previousDigest string,
	newDigest string,
) soul.Revision {
	return soul.Revision{
		ID:              id,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		SourcePath:      "agents/" + agentName + "/" + soul.FileName,
		Action:          action,
		PreviousDigest:  previousDigest,
		NewDigest:       newDigest,
		Body:            "stored body",
		DiagnosticsJSON: json.RawMessage(`[]`),
		ActorKind:       "agent",
		ActorID:         agentName,
		OriginKind:      "test",
		OriginRef:       "global_db_soul_test",
		CreatedAt:       time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC),
	}
}

func revisionIDs(revisions []soul.Revision) []string {
	ids := make([]string, 0, len(revisions))
	for _, revision := range revisions {
		ids = append(ids, revision.ID)
	}
	return ids
}

func assertSessionSoulProvenance(
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	snapshotID string,
	digest string,
	parentDigest string,
) {
	t.Helper()

	sessions, err := globalDB.ListSessions(testutil.Context(t), store.SessionListQuery{})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	for _, session := range sessions {
		if session.ID != sessionID {
			continue
		}
		if session.SoulSnapshotID != snapshotID ||
			session.SoulDigest != digest ||
			session.ParentSoulDigest != parentDigest {
			t.Fatalf(
				"session Soul provenance = %#v/%q/%q, want %q/%q/%q",
				session.SoulSnapshotID,
				session.SoulDigest,
				session.ParentSoulDigest,
				snapshotID,
				digest,
				parentDigest,
			)
		}
		return
	}
	t.Fatalf("session %q not found in %#v", sessionID, sessions)
}
