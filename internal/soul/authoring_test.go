package soul_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/soul"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestManagedSoulAuthoringServicePutValidateAndCAS(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve context cancellation across validation parsing", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		ctx := &cancelAfterFirstErrContext{Context: fixture.ctx}
		body := validSoulBody("coder", "Cancellation must not become validation failure.")

		_, err := fixture.service.Validate(ctx, soul.ValidateRequest{
			Target: fixture.target,
			Body:   &body,
		})
		if err == nil {
			t.Fatal("Validate(canceled) error = nil, want context cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Validate(canceled) error = %v, want context.Canceled", err)
		}
		if errors.Is(err, soul.ErrInvalid) {
			t.Fatalf("Validate(canceled) error = %v, must not be classified as ErrInvalid", err)
		}
	})

	t.Run("Should preserve parser configuration errors across validation", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		body := validSoulBody("coder", "Configuration failures must not become validation failures.")
		target := fixture.target
		target.Config = aghconfig.SoulConfig{
			Enabled:                true,
			MaxBodyBytes:           4096,
			ContextProjectionBytes: 0,
		}

		_, err := fixture.service.Validate(fixture.ctx, soul.ValidateRequest{
			Target: target,
			Body:   &body,
		})
		if err == nil {
			t.Fatal("Validate(invalid config) error = nil, want config validation error")
		}
		if !strings.Contains(err.Error(), "agents.soul.context_projection_bytes") {
			t.Fatalf("Validate(invalid config) error = %v, want config validation", err)
		}
		if errors.Is(err, soul.ErrInvalid) {
			t.Fatalf("Validate(invalid config) error = %v, must not be classified as ErrInvalid", err)
		}
	})

	t.Run("Should reject nil authoring store", func(t *testing.T) {
		t.Parallel()

		if _, err := soul.NewManagedSoulAuthoringService(nil); err == nil ||
			!strings.Contains(err.Error(), "soul: authoring store is required") {
			t.Fatalf("NewManagedSoulAuthoringService(nil) error = %v", err)
		}
	})

	t.Run("Should purge only revisions owned by the effective definition source", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		if err := fixture.db.DeleteSoulAgentHistory(
			fixture.ctx,
			"",
			"coder",
			"agents/coder/SOUL.md",
		); err == nil || !strings.Contains(err.Error(), "requires workspace id, agent name, and source path") {
			t.Fatalf("DeleteSoulAgentHistory(empty workspace) error = %v", err)
		}
		created, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   validSoulBody("coder", "Workspace source."),
			Actor:  soul.AuthoringIdentity{Kind: "user", Ref: "tester"},
			Origin: soul.AuthoringIdentity{Kind: "api", Ref: "test"},
		})
		if err != nil {
			t.Fatalf("Put(workspace source) error = %v", err)
		}
		survivor := created.Revision
		survivor.ID = "srev-global-survivor"
		survivor.SourcePath = "agents/coder/SOUL.md"
		survivor.CreatedAt = survivor.CreatedAt.Add(time.Minute)
		if _, err := fixture.db.AppendSoulRevision(fixture.ctx, survivor); err != nil {
			t.Fatalf("AppendSoulRevision(global survivor) error = %v", err)
		}
		if err := fixture.service.PurgeAgentHistory(
			fixture.ctx,
			soul.WorkspaceRef{WorkspaceID: fixture.workspaceID},
			"coder",
			fixture.agentPath,
		); err != nil {
			t.Fatalf("PurgeAgentHistory() error = %v", err)
		}
		revisions, err := fixture.db.ListSoulRevisions(fixture.ctx, soul.RevisionListQuery{
			WorkspaceID: fixture.workspaceID,
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListSoulRevisions() error = %v", err)
		}
		if len(revisions) != 1 || revisions[0].ID != survivor.ID || revisions[0].SourcePath != survivor.SourcePath {
			t.Fatalf("surviving revisions = %#v, want only global source", revisions)
		}
	})

	t.Run("Should reject incomplete or untrusted purge targets", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		tests := []struct {
			name       string
			service    *soul.ManagedSoulAuthoringService
			ctx        context.Context
			workspace  string
			agentName  string
			sourcePath string
			wantError  string
		}{
			{
				name: "nil context", service: fixture.service,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "context is required",
			},
			{
				name: "nil service", ctx: fixture.ctx,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "store is required",
			},
			{
				name: "missing workspace", service: fixture.service, ctx: fixture.ctx,
				agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "workspace id is required",
			},
			{
				name: "missing agent", service: fixture.service, ctx: fixture.ctx,
				workspace: fixture.workspaceID, sourcePath: fixture.agentPath,
				wantError: "agent name is required",
			},
			{
				name: "non-agent source", service: fixture.service, ctx: fixture.ctx,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.soulPath,
				wantError: "requires an AGENT.md source path",
			},
			{
				name: "outside agents root", service: fixture.service, ctx: fixture.ctx,
				workspace: fixture.workspaceID, agentName: "coder",
				sourcePath: filepath.Join(t.TempDir(), "coder", aghconfig.AgentDefinitionFileName),
				wantError:  "outside an agents root",
			},
		}
		for _, tc := range tests {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()
				err := tc.service.PurgeAgentHistory(
					tc.ctx,
					soul.WorkspaceRef{WorkspaceID: tc.workspace},
					tc.agentName,
					tc.sourcePath,
				)
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("PurgeAgentHistory(invalid target) error = %v, want %q", err, tc.wantError)
				}
			})
		}
	})

	t.Run("Should validate and write SOUL content with snapshots and revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		body := validSoulBody("coder", "Keep changes tight.")
		validation, err := fixture.service.Validate(fixture.ctx, soul.ValidateRequest{
			Target: fixture.target,
			Body:   &body,
		})
		if err != nil {
			t.Fatalf("Validate(body) error = %v", err)
		}
		if !validation.Soul.Valid || validation.Soul.Digest == "" {
			t.Fatalf("Validate(body).Soul = %#v, want valid digest", validation.Soul)
		}

		first, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   body,
			Actor:  soul.AuthoringIdentity{Kind: "human", Ref: "tester"},
			Origin: soul.AuthoringIdentity{Kind: "cli", Ref: "agh agent soul write"},
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}
		if !first.Soul.Valid || !first.Soul.Present || first.Soul.Digest == "" {
			t.Fatalf("Put(create).Soul = %#v, want present valid digest", first.Soul)
		}
		if first.Snapshot.ID == "" || first.Snapshot.Digest != first.Soul.Digest {
			t.Fatalf("Put(create).Snapshot = %#v, want persisted current digest", first.Snapshot)
		}
		if first.Revision.Action != soul.RevisionActionPut ||
			first.Revision.PreviousDigest != "" ||
			first.Revision.NewDigest != first.Soul.Digest ||
			first.Revision.Body != body {
			t.Fatalf("Put(create).Revision = %#v, want create put revision", first.Revision)
		}
		assertFileContent(t, fixture.soulPath, body)

		updatedBody := validSoulBody("reviewer", "Review with receipts.")
		second, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target:         fixture.target,
			Body:           updatedBody,
			ExpectedDigest: first.Soul.Digest,
			Actor:          soul.AuthoringIdentity{Kind: "human", Ref: "tester"},
			Origin:         soul.AuthoringIdentity{Kind: "cli", Ref: "agh agent soul write"},
		})
		if err != nil {
			t.Fatalf("Put(update) error = %v", err)
		}
		if second.Soul.Digest == first.Soul.Digest {
			t.Fatalf("Put(update).Soul.Digest = %q, want new digest", second.Soul.Digest)
		}
		if second.Revision.PreviousDigest != first.Soul.Digest ||
			second.Revision.NewDigest != second.Soul.Digest {
			t.Fatalf("Put(update).Revision = %#v, want digest transition", second.Revision)
		}
		assertFileContent(t, fixture.soulPath, updatedBody)

		currentValidation, err := fixture.service.Validate(fixture.ctx, soul.ValidateRequest{
			Target: fixture.target,
		})
		if err != nil {
			t.Fatalf("Validate(current) error = %v", err)
		}
		if currentValidation.Soul.Digest != second.Soul.Digest {
			t.Fatalf(
				"Validate(current).Soul.Digest = %q, want %q",
				currentValidation.Soul.Digest,
				second.Soul.Digest,
			)
		}

		history, err := fixture.service.History(fixture.ctx, soul.HistoryRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if got, want := len(history.Revisions), 2; got != want {
			t.Fatalf("len(History().Revisions) = %d, want %d", got, want)
		}
		if history.Revisions[0].NewDigest != second.Soul.Digest ||
			history.Revisions[1].NewDigest != first.Soul.Digest {
			t.Fatalf("History().Revisions = %#v, want newest-first digest order", history.Revisions)
		}
	})

	t.Run("Should reject invalid content without modifying file or appending revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		original := validSoulBody("coder", "Keep the current file.")
		created, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   original,
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		invalid := "---\ntools:\n  - bash\n---\nDo operational work.\n"
		result, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target:         fixture.target,
			Body:           invalid,
			ExpectedDigest: created.Soul.Digest,
		})
		if !errors.Is(err, soul.ErrInvalid) {
			t.Fatalf("Put(invalid) error = %v, want ErrInvalid", err)
		}
		authoringErr := requireAuthoringCode(t, err, "soul_invalid")
		if len(authoringErr.Diagnostics) != 1 || authoringErr.Diagnostics[0].Code != "forbidden_field" {
			t.Fatalf("invalid diagnostics = %#v, want forbidden_field", authoringErr.Diagnostics)
		}
		if result.Soul.Valid {
			t.Fatalf("Put(invalid).Soul.Valid = true, want false")
		}
		assertFileContent(t, fixture.soulPath, original)
		assertRevisionCount(t, fixture, 1)
	})

	t.Run("Should reject stale expected digest without modifying file or appending revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		original := validSoulBody("coder", "Keep this digest.")
		if _, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   original,
		}); err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		_, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target:         fixture.target,
			Body:           validSoulBody("coder", "Stale update."),
			ExpectedDigest: "sha256:stale",
		})
		if !errors.Is(err, soul.ErrAuthoringConflict) {
			t.Fatalf("Put(stale) error = %v, want ErrAuthoringConflict", err)
		}
		requireAuthoringCode(t, err, "soul_conflict")
		assertFileContent(t, fixture.soulPath, original)
		assertRevisionCount(t, fixture, 1)
	})
}

func TestManagedSoulAuthoringServiceDeleteRollbackAndHistory(t *testing.T) {
	t.Parallel()

	t.Run("Should delete only the managed SOUL file and append a delete revision", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		created, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   validSoulBody("coder", "Delete me through the service."),
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		deleted, err := fixture.service.Delete(fixture.ctx, soul.DeleteRequest{
			Target:         fixture.target,
			ExpectedDigest: created.Soul.Digest,
			Actor:          soul.AuthoringIdentity{Kind: "human", Ref: "tester"},
			Origin:         soul.AuthoringIdentity{Kind: "http", Ref: "DELETE /soul"},
		})
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.Soul.Present || deleted.Soul.Digest != "" {
			t.Fatalf("Delete().Soul = %#v, want missing state", deleted.Soul)
		}
		if deleted.Revision.Action != soul.RevisionActionDelete ||
			deleted.Revision.PreviousDigest != created.Soul.Digest ||
			deleted.Revision.NewDigest != "" ||
			deleted.Revision.Body != "" {
			t.Fatalf("Delete().Revision = %#v, want delete transition", deleted.Revision)
		}
		if _, err := os.Stat(fixture.soulPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(SOUL.md) error = %v, want %v", err, os.ErrNotExist)
		}
		if _, err := os.Stat(fixture.agentPath); err != nil {
			t.Fatalf("Stat(AGENT.md) error = %v, want managed delete to leave agent file", err)
		}
	})

	t.Run("Should reject deleting absent SOUL content deterministically", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		_, err := fixture.service.Delete(fixture.ctx, soul.DeleteRequest{
			Target: fixture.target,
		})
		if !errors.Is(err, soul.ErrAuthoringMissing) {
			t.Fatalf("Delete(absent) error = %v, want ErrAuthoringMissing", err)
		}
		requireAuthoringCode(t, err, "soul_missing")
		assertRevisionCount(t, fixture, 0)
	})

	t.Run("Should restore a prior revision through validation CAS and append rollback history", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		firstBody := validSoulBody("first", "First persona body.")
		first, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   firstBody,
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		secondBody := validSoulBody("second", "Second persona body.")
		second, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target:         fixture.target,
			Body:           secondBody,
			ExpectedDigest: first.Soul.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}

		rolledBack, err := fixture.service.Rollback(fixture.ctx, soul.RollbackRequest{
			Target:         fixture.target,
			RevisionID:     first.Revision.ID,
			ExpectedDigest: second.Soul.Digest,
			Actor:          soul.AuthoringIdentity{Kind: "human", Ref: "tester"},
			Origin:         soul.AuthoringIdentity{Kind: "uds", Ref: "agent.soul.rollback"},
		})
		if err != nil {
			t.Fatalf("Rollback() error = %v", err)
		}
		if rolledBack.Soul.Digest != first.Soul.Digest {
			t.Fatalf("Rollback().Soul.Digest = %q, want %q", rolledBack.Soul.Digest, first.Soul.Digest)
		}
		if rolledBack.Revision.Action != soul.RevisionActionRollback ||
			rolledBack.Revision.PreviousDigest != second.Soul.Digest ||
			rolledBack.Revision.NewDigest != first.Soul.Digest ||
			rolledBack.Revision.Body != firstBody {
			t.Fatalf("Rollback().Revision = %#v, want rollback to first body", rolledBack.Revision)
		}
		assertFileContent(t, fixture.soulPath, firstBody)
		assertRevisionCount(t, fixture, 3)
	})

	t.Run("Should reject missing rollback revisions without mutating history", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		body := validSoulBody("coder", "Rollback must select a real revision.")
		created, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   body,
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		_, err = fixture.service.Rollback(fixture.ctx, soul.RollbackRequest{
			Target:         fixture.target,
			RevisionID:     "srev-missing",
			ExpectedDigest: created.Soul.Digest,
		})
		if !errors.Is(err, soul.ErrRevisionNotFound) {
			t.Fatalf("Rollback(missing revision) error = %v, want ErrRevisionNotFound", err)
		}
		requireAuthoringCode(t, err, "revision_not_found")
		assertFileContent(t, fixture.soulPath, body)
		assertRevisionCount(t, fixture, 1)
	})

	t.Run("Should persist revision history across database reopen", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), "agh.db")
		cloneSoulTestStoreSeed(t, dbPath)
		firstFixture := newAuthoringFixtureWithDBPath(t, dbPath)
		first, err := firstFixture.service.Put(firstFixture.ctx, soul.PutRequest{
			Target: firstFixture.target,
			Body:   validSoulBody("first", "Persist first."),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		second, err := firstFixture.service.Put(firstFixture.ctx, soul.PutRequest{
			Target:         firstFixture.target,
			Body:           validSoulBody("second", "Persist second."),
			ExpectedDigest: first.Soul.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		if err := firstFixture.db.Close(firstFixture.ctx); err != nil {
			t.Fatalf("Close(first db) error = %v", err)
		}

		reopened, err := globaldb.OpenGlobalDB(firstFixture.ctx, dbPath)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(reopened db) error = %v", err)
			}
		})
		service, err := soul.NewManagedSoulAuthoringService(reopened)
		if err != nil {
			t.Fatalf("NewManagedSoulAuthoringService(reopen) error = %v", err)
		}
		history, err := service.History(firstFixture.ctx, soul.HistoryRequest{Target: firstFixture.target})
		if err != nil {
			t.Fatalf("History(reopen) error = %v", err)
		}
		if got, want := len(history.Revisions), 2; got != want {
			t.Fatalf("len(History(reopen).Revisions) = %d, want %d", got, want)
		}
		if history.Revisions[0].NewDigest != second.Soul.Digest ||
			history.Revisions[1].NewDigest != first.Soul.Digest {
			t.Fatalf("History(reopen).Revisions = %#v, want persisted newest-first order", history.Revisions)
		}
	})
}

func TestManagedSoulAuthoringServiceSafetyBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should reject traversal symlink and missing agent targets deterministically", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		outsidePath := filepath.Join(filepath.Dir(fixture.root), "outside", "AGENT.md")
		_, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: withAgentPath(fixture.target, outsidePath),
			Body:   validSoulBody("coder", "Traversal attempt."),
		})
		if !errors.Is(err, soul.ErrAuthoringPathRejected) {
			t.Fatalf("Put(traversal) error = %v, want ErrAuthoringPathRejected", err)
		}
		requireAuthoringCode(t, err, "path_escape")

		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires elevated privileges on windows")
		}
		linkedTarget := filepath.Join(fixture.root, "linked-soul.md")
		if err := os.WriteFile(linkedTarget, []byte("linked target"), 0o644); err != nil {
			t.Fatalf("WriteFile(linked target) error = %v", err)
		}
		if err := os.Symlink(linkedTarget, fixture.soulPath); err != nil {
			t.Fatalf("Symlink(SOUL.md) error = %v", err)
		}
		_, err = fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   validSoulBody("coder", "Symlink attempt."),
		})
		if !errors.Is(err, soul.ErrAuthoringPathRejected) {
			t.Fatalf("Put(symlink) error = %v, want ErrAuthoringPathRejected", err)
		}
		requireAuthoringCode(t, err, "path_escape")
		assertFileContent(t, linkedTarget, "linked target")

		missingTarget := fixture.target
		missingTarget.AgentName = "missing"
		_, err = fixture.service.History(fixture.ctx, soul.HistoryRequest{Target: missingTarget})
		if !errors.Is(err, soul.ErrAuthoringAgentNotFound) {
			t.Fatalf("History(missing agent) error = %v, want ErrAuthoringAgentNotFound", err)
		}
		requireAuthoringCode(t, err, "agent_not_found")
	})

	t.Run("Should preserve active session and task run ownership metadata across writes", func(t *testing.T) {
		t.Parallel()

		fixture := newAuthoringFixture(t)
		first, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target: fixture.target,
			Body:   validSoulBody("first", "Session starts with this soul."),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		session := store.SessionInfo{
			ID:             "sess-authoring",
			AgentName:      "coder",
			WorkspaceID:    fixture.workspaceID,
			State:          "active",
			SoulSnapshotID: first.Snapshot.ID,
			SoulDigest:     first.Soul.Digest,
		}
		if err := fixture.db.RegisterSession(fixture.ctx, session); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		claimedBy := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: session.ID}
		origin := taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "test"}
		taskRecord := taskpkg.Task{
			ID:          "task-authoring",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: fixture.workspaceID,
			Title:       "Authoring safety",
			Status:      taskpkg.TaskStatusInProgress,
			CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "tester"},
			Origin:      origin,
		}
		if err := fixture.db.CreateTask(fixture.ctx, taskRecord); err != nil {
			t.Fatalf("CreateTask() error = %v", err)
		}
		claimedAt := time.Date(2026, 5, 2, 14, 0, 0, 0, time.UTC)
		taskRun := taskpkg.Run{
			ID:             "run-authoring",
			TaskID:         taskRecord.ID,
			Status:         taskpkg.TaskRunStatusRunning,
			Attempt:        1,
			ClaimedBy:      &claimedBy,
			SessionID:      session.ID,
			Origin:         origin,
			ClaimTokenHash: strings.Repeat("a", 64),
			LeaseUntil:     claimedAt.Add(time.Hour),
			HeartbeatAt:    claimedAt.Add(time.Minute),
			ClaimedAt:      claimedAt,
			StartedAt:      claimedAt,
		}
		if err := fixture.db.CreateTaskRun(fixture.ctx, taskRun); err != nil {
			t.Fatalf("CreateTaskRun() error = %v", err)
		}

		second, err := fixture.service.Put(fixture.ctx, soul.PutRequest{
			Target:         fixture.target,
			Body:           validSoulBody("second", "New sessions can use this later."),
			ExpectedDigest: first.Soul.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		if second.Soul.Digest == first.Soul.Digest {
			t.Fatalf("Put(second).Soul.Digest = %q, want changed digest", second.Soul.Digest)
		}

		sessions, err := fixture.db.ListSessions(fixture.ctx, store.SessionListQuery{AgentName: "coder"})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		gotSession := findSession(t, sessions, session.ID)
		if gotSession.SoulSnapshotID != first.Snapshot.ID || gotSession.SoulDigest != first.Soul.Digest {
			t.Fatalf(
				"session soul provenance = %q/%q, want original %q/%q",
				gotSession.SoulSnapshotID,
				gotSession.SoulDigest,
				first.Snapshot.ID,
				first.Soul.Digest,
			)
		}
		gotRun, err := fixture.db.GetTaskRun(fixture.ctx, taskRun.ID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if gotRun.Status != taskRun.Status ||
			gotRun.SessionID != taskRun.SessionID ||
			gotRun.ClaimTokenHash != taskRun.ClaimTokenHash ||
			!gotRun.LeaseUntil.Equal(taskRun.LeaseUntil) ||
			!gotRun.HeartbeatAt.Equal(taskRun.HeartbeatAt) ||
			gotRun.ClaimedBy == nil ||
			*gotRun.ClaimedBy != claimedBy {
			t.Fatalf("task run after authoring = %#v, want ownership and lease unchanged", gotRun)
		}
	})
}

type authoringFixture struct {
	ctx         context.Context
	db          *globaldb.GlobalDB
	service     *soul.ManagedSoulAuthoringService
	root        string
	workspaceID string
	agentPath   string
	soulPath    string
	target      soul.AuthoringTarget
}

type cancelAfterFirstErrContext struct {
	context.Context
	mu    sync.Mutex
	calls int
}

func (c *cancelAfterFirstErrContext) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	if c.calls <= 1 {
		return nil
	}
	closed := make(chan struct{})
	close(closed)
	return closed
}

func (c *cancelAfterFirstErrContext) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls > 1 {
		return context.Canceled
	}
	return nil
}

func newAuthoringFixture(t *testing.T) authoringFixture {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "agh.db")
	cloneSoulTestStoreSeed(t, dbPath)
	return newAuthoringFixtureWithDBPath(t, dbPath)
}

func cloneSoulTestStoreSeed(t *testing.T, dbPath string) {
	t.Helper()
	if soulTestStoreSeed == nil {
		t.Fatal("soul test store seed is not initialized")
	}
	if err := soulTestStoreSeed.Clone(dbPath); err != nil {
		t.Fatalf("Clone(store seed) error = %v", err)
	}
}

func newAuthoringFixtureWithDBPath(t *testing.T, dbPath string) authoringFixture {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	agentPath := writeAgentDefinition(t, root, "coder")
	globalDB, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(GlobalDB) error = %v", err)
		}
	})
	workspaceID := "ws-authoring"
	if err := globalDB.InsertWorkspace(ctx, aghworkspace.Workspace{
		ID:      workspaceID,
		RootDir: root,
		Name:    "authoring",
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	service, err := soul.NewManagedSoulAuthoringService(
		globalDB,
		soul.WithSoulAuthoringClock(deterministicClock(time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC))),
		soul.WithSoulAuthoringIDGenerator(deterministicIDGenerator()),
	)
	if err != nil {
		t.Fatalf("NewManagedSoulAuthoringService() error = %v", err)
	}
	target := soul.AuthoringTarget{
		WorkspaceID:   workspaceID,
		WorkspaceRoot: root,
		AgentName:     "coder",
		AgentPath:     agentPath,
		Config:        aghconfig.DefaultSoulConfig(),
		ConfigSource:  "test",
	}
	return authoringFixture{
		ctx:         ctx,
		db:          globalDB,
		service:     service,
		root:        root,
		workspaceID: workspaceID,
		agentPath:   agentPath,
		soulPath:    filepath.Join(filepath.Dir(agentPath), soul.FileName),
		target:      target,
	}
}

func writeAgentDefinition(t *testing.T, root string, agentName string) string {
	t.Helper()

	agentDir := filepath.Join(root, aghconfig.DirName, aghconfig.AgentsDirName, agentName)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(agent dir) error = %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	content := fmt.Sprintf(`---
name: %s
provider: codex
---
You are %s.
`, agentName, agentName)
	if err := os.WriteFile(agentPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(AGENT.md) error = %v", err)
	}
	return agentPath
}

func validSoulBody(role string, body string) string {
	return fmt.Sprintf(`---
version: "1"
role: %s
tone:
  - concise
principles:
  - Keep scope tight
---
%s
`, role, body)
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, string(got), want)
	}
}

func assertRevisionCount(t *testing.T, fixture authoringFixture, want int) {
	t.Helper()

	history, err := fixture.service.History(fixture.ctx, soul.HistoryRequest{Target: fixture.target})
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if got := len(history.Revisions); got != want {
		t.Fatalf("len(History().Revisions) = %d, want %d", got, want)
	}
}

func requireAuthoringCode(t *testing.T, err error, code string) *soul.AuthoringError {
	t.Helper()

	var authoringErr *soul.AuthoringError
	if !errors.As(err, &authoringErr) {
		t.Fatalf("error = %T %[1]v, want *soul.AuthoringError", err)
	}
	if authoringErr.Code != code {
		t.Fatalf(
			"AuthoringError.Code = %q, want %q; diagnostics=%#v",
			authoringErr.Code,
			code,
			authoringErr.Diagnostics,
		)
	}
	return authoringErr
}

func withAgentPath(target soul.AuthoringTarget, agentPath string) soul.AuthoringTarget {
	target.AgentPath = agentPath
	return target
}

func deterministicClock(start time.Time) func() time.Time {
	var mu sync.Mutex
	next := start
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		current := next
		next = next.Add(time.Second)
		return current
	}
}

func deterministicIDGenerator() func(prefix string) string {
	var mu sync.Mutex
	counters := make(map[string]int)
	return func(prefix string) string {
		mu.Lock()
		defer mu.Unlock()
		counters[prefix]++
		return fmt.Sprintf("%s-%02d", prefix, counters[prefix])
	}
}

func findSession(t *testing.T, sessions []store.SessionInfo, id string) store.SessionInfo {
	t.Helper()

	for _, session := range sessions {
		if session.ID == id {
			return session
		}
	}
	t.Fatalf("session %q not found in %#v", id, sessions)
	return store.SessionInfo{}
}
