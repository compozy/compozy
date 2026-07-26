package heartbeat_test

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
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/heartbeat"
	aghstore "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	aghworkspace "github.com/compozy/agh/internal/workspace"
)

func TestManagedHeartbeatAuthoringServicePutValidateAndCAS(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil authoring store", func(t *testing.T) {
		t.Parallel()

		service, err := heartbeat.NewManagedHeartbeatAuthoringService(nil)
		if err == nil || !strings.Contains(err.Error(), "heartbeat: authoring store is required") {
			t.Fatalf("NewManagedHeartbeatAuthoringService(nil) = %#v, error = %v", service, err)
		}
	})

	t.Run("Should purge only revisions owned by the effective definition source", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		if err := fixture.db.DeleteHeartbeatAgentHistory(
			fixture.ctx,
			"",
			"coder",
			"agents/coder/HEARTBEAT.md",
		); err == nil || !strings.Contains(err.Error(), "requires workspace id, agent name, and source path") {
			t.Fatalf("DeleteHeartbeatAgentHistory(empty workspace) error = %v", err)
		}
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body: validHeartbeatBody(
				"Inspect workspace source",
				"Inspect the workspace source before waking.",
			),
			Actor: heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindUser), Ref: "tester"},
		})
		if err != nil {
			t.Fatalf("Put(workspace source) error = %v", err)
		}
		survivor := created.Revision
		survivor.ID = "hrev-global-survivor"
		survivor.SourcePath = "agents/coder/HEARTBEAT.md"
		survivor.CreatedAt = survivor.CreatedAt.Add(time.Minute)
		if _, err := fixture.db.AppendHeartbeatRevision(fixture.ctx, survivor); err != nil {
			t.Fatalf("AppendHeartbeatRevision(global survivor) error = %v", err)
		}
		if err := fixture.authoring.PurgeAgentHistory(
			fixture.ctx,
			heartbeat.WorkspaceRef{WorkspaceID: fixture.workspaceID},
			"coder",
			fixture.agentPath,
		); err != nil {
			t.Fatalf("PurgeAgentHistory() error = %v", err)
		}
		revisions, err := fixture.db.ListHeartbeatRevisions(fixture.ctx, heartbeat.RevisionListQuery{
			WorkspaceID: fixture.workspaceID,
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListHeartbeatRevisions() error = %v", err)
		}
		if len(revisions) != 1 || revisions[0].ID != survivor.ID || revisions[0].SourcePath != survivor.SourcePath {
			t.Fatalf("surviving revisions = %#v, want only global source", revisions)
		}
	})

	t.Run("Should reject incomplete or untrusted purge targets", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		tests := []struct {
			name       string
			service    *heartbeat.ManagedHeartbeatAuthoringService
			ctx        context.Context
			workspace  string
			agentName  string
			sourcePath string
			wantError  string
		}{
			{
				name: "nil context", service: fixture.authoring,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "context is required",
			},
			{
				name: "nil service", ctx: fixture.ctx,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "store is required",
			},
			{
				name: "missing workspace", service: fixture.authoring, ctx: fixture.ctx,
				agentName: "coder", sourcePath: fixture.agentPath,
				wantError: "workspace id is required",
			},
			{
				name: "missing agent", service: fixture.authoring, ctx: fixture.ctx,
				workspace: fixture.workspaceID, sourcePath: fixture.agentPath,
				wantError: "agent name is required",
			},
			{
				name: "non-agent source", service: fixture.authoring, ctx: fixture.ctx,
				workspace: fixture.workspaceID, agentName: "coder", sourcePath: fixture.heartbeatPath,
				wantError: "requires an AGENT.md source path",
			},
			{
				name: "outside agents root", service: fixture.authoring, ctx: fixture.ctx,
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
					heartbeat.WorkspaceRef{WorkspaceID: tc.workspace},
					tc.agentName,
					tc.sourcePath,
				)
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("PurgeAgentHistory(invalid target) error = %v, want %q", err, tc.wantError)
				}
			})
		}
	})

	t.Run("Should validate and write HEARTBEAT content with snapshots and revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		body := validHeartbeatBody(
			"Inspect before waking",
			"Inspect context, then send at most one synthetic wake prompt.",
		)
		validation, err := fixture.authoring.Validate(fixture.ctx, heartbeat.ValidateRequest{
			Target: fixture.target,
			Body:   &body,
		})
		if err != nil {
			t.Fatalf("Validate(body) error = %v", err)
		}
		if !validation.Policy.Valid || validation.Policy.Digest == "" {
			t.Fatalf("Validate(body).Policy = %#v, want valid digest", validation.Policy)
		}

		first, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   body,
			Actor:  heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindAgent), Ref: "coder"},
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}
		if !first.Policy.Valid || !first.Policy.Present || first.Policy.Digest == "" {
			t.Fatalf("Put(create).Policy = %#v, want present valid digest", first.Policy)
		}
		if first.Snapshot.ID == "" || first.Snapshot.Digest != first.Policy.Digest {
			t.Fatalf("Put(create).Snapshot = %#v, want persisted current digest", first.Snapshot)
		}
		if first.Revision.Operation != heartbeat.RevisionOperationWrite ||
			first.Revision.PreviousDigest != "" ||
			first.Revision.NewDigest != first.Policy.Digest ||
			first.Revision.NewSnapshotID != first.Snapshot.ID ||
			first.Revision.Body != body {
			t.Fatalf("Put(create).Revision = %#v, want create write revision", first.Revision)
		}
		assertHeartbeatFileContent(t, fixture.heartbeatPath, body)

		updatedBody := validHeartbeatBody(
			"Review health before waking",
			"Check session health, then send a single synthetic wake prompt if the session is idle.",
		)
		second, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target:         fixture.target,
			Body:           updatedBody,
			ExpectedDigest: first.Policy.Digest,
			Actor:          heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindUser), Ref: "tester"},
		})
		if err != nil {
			t.Fatalf("Put(update) error = %v", err)
		}
		if second.Policy.Digest == first.Policy.Digest {
			t.Fatalf("Put(update).Policy.Digest = %q, want new digest", second.Policy.Digest)
		}
		if second.Revision.PreviousDigest != first.Policy.Digest ||
			second.Revision.NewDigest != second.Policy.Digest {
			t.Fatalf("Put(update).Revision = %#v, want digest transition", second.Revision)
		}
		assertHeartbeatFileContent(t, fixture.heartbeatPath, updatedBody)

		currentValidation, err := fixture.authoring.Validate(fixture.ctx, heartbeat.ValidateRequest{
			Target: fixture.target,
		})
		if err != nil {
			t.Fatalf("Validate(current) error = %v", err)
		}
		if currentValidation.Policy.Digest != second.Policy.Digest {
			t.Fatalf(
				"Validate(current).Policy.Digest = %q, want %q",
				currentValidation.Policy.Digest,
				second.Policy.Digest,
			)
		}

		history, err := fixture.authoring.History(fixture.ctx, heartbeat.HistoryRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("History() error = %v", err)
		}
		if got, want := len(history.Revisions), 2; got != want {
			t.Fatalf("len(History().Revisions) = %d, want %d", got, want)
		}
		if history.Revisions[0].NewDigest != second.Policy.Digest ||
			history.Revisions[1].NewDigest != first.Policy.Digest {
			t.Fatalf("History().Revisions = %#v, want newest-first digest order", history.Revisions)
		}
	})

	t.Run("Should reject invalid content without modifying file or appending revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		original := validHeartbeatBody("Keep current policy", "Inspect state before sending a wake prompt.")
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   original,
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		invalid := "---\nversion: 2\n---\nWake gently.\n"
		result, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target:         fixture.target,
			Body:           invalid,
			ExpectedDigest: created.Policy.Digest,
		})
		if !errors.Is(err, heartbeat.ErrInvalid) {
			t.Fatalf("Put(invalid) error = %v, want ErrInvalid", err)
		}
		authoringErr := requireHeartbeatAuthoringCode(t, err, "heartbeat_invalid")
		assertHeartbeatDiagnosticCode(t, authoringErr.Diagnostics, "heartbeat_invalid_field_type")
		if result.Policy.Valid {
			t.Fatalf("Put(invalid).Policy.Valid = true, want false")
		}
		assertHeartbeatFileContent(t, fixture.heartbeatPath, original)
		assertHeartbeatRevisionCount(t, fixture, 1)
		assertHeartbeatSnapshotCount(t, fixture, 1)
	})

	t.Run("Should repair invalid current HEARTBEAT content without an expected digest", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		invalid := "---\nversion: 2\n---\nWake gently.\n"
		if err := os.WriteFile(fixture.heartbeatPath, []byte(invalid), 0o644); err != nil {
			t.Fatalf("WriteFile(invalid HEARTBEAT.md) error = %v", err)
		}
		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Status(invalid policy) error = %v", err)
		}
		if !status.Present || status.Valid || status.Digest != "" {
			t.Fatalf("Status(invalid policy) = %#v, want present invalid policy without digest", status)
		}

		repairedBody := validHeartbeatBody(
			"Repair invalid policy",
			"Repair this invalid policy through the managed authoring boundary.",
		)
		repaired, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   repairedBody,
			Actor:  heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindAgent), Ref: "coder"},
		})
		if err != nil {
			t.Fatalf("Put(repair invalid current) error = %v", err)
		}
		if !repaired.Policy.Valid || !repaired.Policy.Present || repaired.Policy.Digest == "" {
			t.Fatalf("Put(repair invalid current).Policy = %#v, want valid present policy", repaired.Policy)
		}
		if repaired.Revision.Operation != heartbeat.RevisionOperationWrite ||
			repaired.Revision.PreviousDigest != "" ||
			repaired.Revision.NewDigest != repaired.Policy.Digest ||
			repaired.Revision.Body != repairedBody {
			t.Fatalf("Put(repair invalid current).Revision = %#v, want repair write revision", repaired.Revision)
		}
		assertHeartbeatFileContent(t, fixture.heartbeatPath, repairedBody)
		assertHeartbeatRevisionCount(t, fixture, 1)
		assertHeartbeatSnapshotCount(t, fixture, 1)
	})

	t.Run("Should reject missing and stale expected digests without appending revisions", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		original := validHeartbeatBody("Keep this digest", "Do not accept stale authoring input.")
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   original,
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		_, err = fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Missing CAS", "Missing expected digest must not update the policy."),
		})
		if !errors.Is(err, heartbeat.ErrAuthoringConflict) {
			t.Fatalf("Put(missing expected digest) error = %v, want ErrAuthoringConflict", err)
		}
		requireHeartbeatAuthoringCode(t, err, "heartbeat_conflict")

		_, err = fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target:         fixture.target,
			Body:           validHeartbeatBody("Stale CAS", "Stale expected digest must not update the policy."),
			ExpectedDigest: "sha256:stale",
		})
		if !errors.Is(err, heartbeat.ErrAuthoringConflict) {
			t.Fatalf("Put(stale) error = %v, want ErrAuthoringConflict", err)
		}
		requireHeartbeatAuthoringCode(t, err, "heartbeat_conflict")
		assertHeartbeatFileContent(t, fixture.heartbeatPath, original)
		assertHeartbeatRevisionCount(t, fixture, 1)

		current, err := fixture.authoring.Validate(fixture.ctx, heartbeat.ValidateRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Validate(current) error = %v", err)
		}
		if current.Policy.Digest != created.Policy.Digest {
			t.Fatalf("current digest = %q, want unchanged %q", current.Policy.Digest, created.Policy.Digest)
		}
	})
}

func TestManagedHeartbeatAuthoringServiceDeleteRollbackHistoryAndPersistence(t *testing.T) {
	t.Parallel()

	t.Run("Should delete only the managed HEARTBEAT file and append a delete revision", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Delete me", "Delete this advisory wake policy through the service."),
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		deleted, err := fixture.authoring.Delete(fixture.ctx, heartbeat.DeleteRequest{
			Target:         fixture.target,
			ExpectedDigest: created.Policy.Digest,
			Actor:          heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindUser), Ref: "tester"},
		})
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if deleted.Policy.Present || deleted.Policy.Digest != "" || !deleted.Policy.Valid {
			t.Fatalf("Delete().Policy = %#v, want missing valid state", deleted.Policy)
		}
		if deleted.Snapshot.ID == "" ||
			deleted.Snapshot.Digest == "" ||
			deleted.Snapshot.Digest == created.Policy.Digest {
			t.Fatalf("Delete().Snapshot = %#v, want persisted absent-state snapshot", deleted.Snapshot)
		}
		if deleted.Revision.Operation != heartbeat.RevisionOperationDelete ||
			deleted.Revision.PreviousDigest != created.Policy.Digest ||
			deleted.Revision.NewDigest != "" ||
			deleted.Revision.NewSnapshotID != deleted.Snapshot.ID ||
			deleted.Revision.Body != "" {
			t.Fatalf("Delete().Revision = %#v, want delete transition", deleted.Revision)
		}
		if stat, err := os.Stat(fixture.heartbeatPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(HEARTBEAT.md) = %#v, error = %v, want %v", stat, err, os.ErrNotExist)
		}
		if stat, err := os.Stat(fixture.agentPath); err != nil {
			t.Fatalf("Stat(AGENT.md) = %#v, error = %v, want managed delete to leave agent file", stat, err)
		}
		assertHeartbeatRevisionCount(t, fixture, 2)
		assertHeartbeatSnapshotCount(t, fixture, 2)
	})

	t.Run("Should reject deleting absent HEARTBEAT content deterministically", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		_, err := fixture.authoring.Delete(fixture.ctx, heartbeat.DeleteRequest{
			Target: fixture.target,
		})
		if !errors.Is(err, heartbeat.ErrAuthoringNoPolicy) {
			t.Fatalf("Delete(absent) error = %v, want ErrAuthoringNoPolicy", err)
		}
		requireHeartbeatAuthoringCode(t, err, "heartbeat_no_policy")
		assertHeartbeatRevisionCount(t, fixture, 0)
	})

	t.Run("Should delete invalid current HEARTBEAT content without an expected digest", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		invalid := "---\nversion: 2\n---\nWake gently.\n"
		if err := os.WriteFile(fixture.heartbeatPath, []byte(invalid), 0o644); err != nil {
			t.Fatalf("WriteFile(invalid HEARTBEAT.md) error = %v", err)
		}
		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Status(invalid policy) error = %v", err)
		}
		if !status.Present || status.Valid || status.Digest != "" {
			t.Fatalf("Status(invalid policy) = %#v, want present invalid policy without digest", status)
		}

		deleted, err := fixture.authoring.Delete(fixture.ctx, heartbeat.DeleteRequest{
			Target: fixture.target,
			Actor:  heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindUser), Ref: "tester"},
		})
		if err != nil {
			t.Fatalf("Delete(invalid current) error = %v", err)
		}
		if deleted.Policy.Present || deleted.Policy.Digest != "" || !deleted.Policy.Valid {
			t.Fatalf("Delete(invalid current).Policy = %#v, want missing valid state", deleted.Policy)
		}
		if deleted.Revision.Operation != heartbeat.RevisionOperationDelete ||
			deleted.Revision.PreviousDigest != "" ||
			deleted.Revision.NewDigest != "" ||
			deleted.Revision.NewSnapshotID != deleted.Snapshot.ID {
			t.Fatalf("Delete(invalid current).Revision = %#v, want invalid delete transition", deleted.Revision)
		}
		if stat, err := os.Stat(fixture.heartbeatPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(HEARTBEAT.md) = %#v, error = %v, want %v", stat, err, os.ErrNotExist)
		}
		assertHeartbeatRevisionCount(t, fixture, 1)
		assertHeartbeatSnapshotCount(t, fixture, 1)
	})

	t.Run("Should stop wake prompts after managed delete", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Delete wake policy", "This policy must not survive deletion."),
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}
		if _, err := fixture.authoring.Delete(fixture.ctx, heartbeat.DeleteRequest{
			Target:         fixture.target,
			ExpectedDigest: created.Policy.Digest,
		}); err != nil {
			t.Fatalf("Delete() error = %v", err)
		}
		if err := fixture.db.RegisterSession(fixture.ctx, aghstore.SessionInfo{
			ID:          "sess-delete",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: fixture.workspaceID,
			State:       string(heartbeat.SessionHealthStateIdle),
			CreatedAt:   fixture.now,
			UpdatedAt:   fixture.now,
		}); err != nil {
			t.Fatalf("RegisterSession(sess-delete) error = %v", err)
		}

		prompter := &recordingHeartbeatPrompter{}
		wakeService, err := heartbeat.NewManagedWakeService(
			fixture.db,
			managedHeartbeatHealthReader{health: eligibleManagedWakeHealth(fixture, "sess-delete")},
			prompter,
			aghconfig.DefaultHeartbeatConfig(),
			heartbeat.WithWakeClock(deterministicHeartbeatClock(fixture.now)),
		)
		if err != nil {
			t.Fatalf("NewManagedWakeService() error = %v", err)
		}

		decision, err := wakeService.Wake(fixture.ctx, heartbeat.WakeRequest{
			WorkspaceID: fixture.workspaceID,
			AgentName:   "coder",
			SessionID:   "sess-delete",
			Source:      heartbeat.WakeSourceManual,
		})
		if err != nil {
			t.Fatalf("Wake(after delete) error = %v", err)
		}
		if decision.Result != heartbeat.WakeResultSkipped ||
			decision.Reason != heartbeat.WakeReasonHeartbeatNoPolicy {
			t.Fatalf("Wake(after delete) = %#v, want no-policy skip", decision)
		}
		if got := len(prompter.requestsSnapshot()); got != 0 {
			t.Fatalf("wake prompts = %d, want 0 after managed delete", got)
		}
	})

	t.Run("Should rollback to delete revisions as absent policy", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		first, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Delete rollback first", "This policy will be deleted."),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		deleted, err := fixture.authoring.Delete(fixture.ctx, heartbeat.DeleteRequest{
			Target:         fixture.target,
			ExpectedDigest: first.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Delete(first) error = %v", err)
		}
		second, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Delete rollback second", "Rollback should remove this policy."),
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}

		rolledBack, err := fixture.authoring.Rollback(fixture.ctx, heartbeat.RollbackRequest{
			Target:         fixture.target,
			RevisionID:     deleted.Revision.ID,
			ExpectedDigest: second.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Rollback(delete revision) error = %v", err)
		}
		if rolledBack.Policy.Present || rolledBack.Policy.Digest != "" || !rolledBack.Policy.Valid {
			t.Fatalf("Rollback(delete revision).Policy = %#v, want absent valid policy", rolledBack.Policy)
		}
		if rolledBack.Revision.Operation != heartbeat.RevisionOperationRollback ||
			rolledBack.Revision.PreviousDigest != second.Policy.Digest ||
			rolledBack.Revision.NewDigest != "" ||
			rolledBack.Revision.Body != "" {
			t.Fatalf("Rollback(delete revision).Revision = %#v, want rollback to absence", rolledBack.Revision)
		}
		if stat, err := os.Stat(fixture.heartbeatPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(HEARTBEAT.md) = %#v, error = %v, want %v", stat, err, os.ErrNotExist)
		}
	})

	t.Run("Should restore prior policy bodies through validation CAS and append rollback history", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		firstBody := validHeartbeatBodyWithWindows("First policy", "Review the initial runtime state before waking.")
		first, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   firstBody,
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		secondBody := validHeartbeatBody("Second policy", "Review the newer runtime state before waking.")
		second, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target:         fixture.target,
			Body:           secondBody,
			ExpectedDigest: first.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}

		rolledBack, err := fixture.authoring.Rollback(fixture.ctx, heartbeat.RollbackRequest{
			Target:         fixture.target,
			RevisionID:     first.Revision.ID,
			ExpectedDigest: second.Policy.Digest,
			Actor:          heartbeat.AuthoringIdentity{Kind: string(heartbeat.ActorKindAgent), Ref: "coder"},
		})
		if err != nil {
			t.Fatalf("Rollback(revision) error = %v", err)
		}
		if rolledBack.Policy.Digest != first.Policy.Digest {
			t.Fatalf("Rollback(revision).Policy.Digest = %q, want %q", rolledBack.Policy.Digest, first.Policy.Digest)
		}
		if rolledBack.Revision.Operation != heartbeat.RevisionOperationRollback ||
			rolledBack.Revision.PreviousDigest != second.Policy.Digest ||
			rolledBack.Revision.NewDigest != first.Policy.Digest ||
			rolledBack.Revision.Body != firstBody {
			t.Fatalf("Rollback(revision).Revision = %#v, want rollback to first body", rolledBack.Revision)
		}
		assertHeartbeatFileContent(t, fixture.heartbeatPath, firstBody)

		secondAgain, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target:         fixture.target,
			Body:           secondBody,
			ExpectedDigest: rolledBack.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second again) error = %v", err)
		}
		rolledBackByDigest, err := fixture.authoring.Rollback(fixture.ctx, heartbeat.RollbackRequest{
			Target:         fixture.target,
			TargetDigest:   first.Policy.Digest,
			ExpectedDigest: secondAgain.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Rollback(target digest) error = %v", err)
		}
		if rolledBackByDigest.Policy.Digest != first.Policy.Digest {
			t.Fatalf(
				"Rollback(target digest).Policy.Digest = %q, want %q",
				rolledBackByDigest.Policy.Digest,
				first.Policy.Digest,
			)
		}
		assertHeartbeatRevisionCount(t, fixture, 5)
	})

	t.Run("Should reject missing rollback revisions without mutating history", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		body := validHeartbeatBody("Rollback guard", "Rollback must select a real revision or snapshot.")
		created, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   body,
		})
		if err != nil {
			t.Fatalf("Put(create) error = %v", err)
		}

		_, err = fixture.authoring.Rollback(fixture.ctx, heartbeat.RollbackRequest{
			Target:         fixture.target,
			RevisionID:     "hrev-missing",
			ExpectedDigest: created.Policy.Digest,
		})
		if !errors.Is(err, heartbeat.ErrRevisionNotFound) {
			t.Fatalf("Rollback(missing revision) error = %v, want ErrRevisionNotFound", err)
		}
		requireHeartbeatAuthoringCode(t, err, "revision_not_found")
		assertHeartbeatFileContent(t, fixture.heartbeatPath, body)
		assertHeartbeatRevisionCount(t, fixture, 1)
	})

	t.Run("Should persist revision history and snapshots across database reopen", func(t *testing.T) {
		t.Parallel()

		dbPath := filepath.Join(t.TempDir(), aghstore.GlobalDatabaseName)
		firstFixture := newHeartbeatFixtureWithDBPath(t, dbPath, aghconfig.DefaultHeartbeatConfig())
		first, err := firstFixture.authoring.Put(firstFixture.ctx, heartbeat.PutRequest{
			Target: firstFixture.target,
			Body:   validHeartbeatBody("Persist first", "Persist the first policy snapshot."),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		second, err := firstFixture.authoring.Put(firstFixture.ctx, heartbeat.PutRequest{
			Target:         firstFixture.target,
			Body:           validHeartbeatBody("Persist second", "Persist the second policy snapshot."),
			ExpectedDigest: first.Policy.Digest,
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
		authoring, err := heartbeat.NewManagedHeartbeatAuthoringService(reopened)
		if err != nil {
			t.Fatalf("NewManagedHeartbeatAuthoringService(reopen) error = %v", err)
		}
		history, err := authoring.History(firstFixture.ctx, heartbeat.HistoryRequest{Target: firstFixture.target})
		if err != nil {
			t.Fatalf("History(reopen) error = %v", err)
		}
		if got, want := len(history.Revisions), 2; got != want {
			t.Fatalf("len(History(reopen).Revisions) = %d, want %d", got, want)
		}
		if history.Revisions[0].NewDigest != second.Policy.Digest ||
			history.Revisions[1].NewDigest != first.Policy.Digest {
			t.Fatalf("History(reopen).Revisions = %#v, want persisted newest-first order", history.Revisions)
		}
		snapshot, found, err := reopened.FindHeartbeatSnapshotByDigest(
			firstFixture.ctx,
			firstFixture.workspaceID,
			"coder",
			second.Policy.Digest,
		)
		if err != nil {
			t.Fatalf("FindHeartbeatSnapshotByDigest(reopen) error = %v", err)
		}
		if !found || snapshot.Digest != second.Policy.Digest {
			t.Fatalf("FindHeartbeatSnapshotByDigest(reopen) = %#v found=%v, want second digest", snapshot, found)
		}
	})
}

func TestManagedHeartbeatAuthoringServiceSafetyBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should reject path escapes, symlink targets, and missing agents deterministically", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		outsidePath := filepath.Join(filepath.Dir(fixture.root), "outside", "AGENT.md")
		_, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: withHeartbeatAgentPath(fixture.target, outsidePath),
			Body:   validHeartbeatBody("Traversal attempt", "Traversal attempts must not mutate files."),
		})
		if !errors.Is(err, heartbeat.ErrAuthoringPathRejected) {
			t.Fatalf("Put(traversal) error = %v, want ErrAuthoringPathRejected", err)
		}
		requireHeartbeatAuthoringCode(t, err, "heartbeat_path_escape")

		if runtime.GOOS == "windows" {
			t.Skip("symlink creation requires elevated privileges on windows")
		}
		linkedTarget := filepath.Join(fixture.root, "linked-heartbeat.md")
		if err := os.WriteFile(linkedTarget, []byte("linked target"), 0o644); err != nil {
			t.Fatalf("WriteFile(linked target) error = %v", err)
		}
		if err := os.Symlink(linkedTarget, fixture.heartbeatPath); err != nil {
			t.Fatalf("Symlink(HEARTBEAT.md) error = %v", err)
		}
		_, err = fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Symlink attempt", "Symlink attempts must not mutate files."),
		})
		if !errors.Is(err, heartbeat.ErrAuthoringPathRejected) {
			t.Fatalf("Put(symlink) error = %v, want ErrAuthoringPathRejected", err)
		}
		requireHeartbeatAuthoringCode(t, err, "heartbeat_path_escape")
		assertHeartbeatFileContent(t, linkedTarget, "linked target")

		missingTarget := fixture.target
		missingTarget.AgentName = "missing"
		_, err = fixture.authoring.History(fixture.ctx, heartbeat.HistoryRequest{Target: missingTarget})
		if !errors.Is(err, heartbeat.ErrAuthoringAgentNotFound) {
			t.Fatalf("History(missing agent) error = %v, want ErrAuthoringAgentNotFound", err)
		}
		requireHeartbeatAuthoringCode(t, err, "agent_not_found")
	})

	t.Run("Should preserve session health, wake audit, and task leases across authoring writes", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		session := aghstore.SessionInfo{
			ID:          "sess-authoring",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: fixture.workspaceID,
			State:       "active",
			CreatedAt:   fixture.now,
			UpdatedAt:   fixture.now,
		}
		if err := fixture.db.RegisterSession(fixture.ctx, session); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}
		healthBefore, err := fixture.db.UpsertSessionHealth(
			fixture.ctx,
			managedHeartbeatSessionHealth(fixture.workspaceID, session.ID, fixture.now.Add(time.Minute)),
		)
		if err != nil {
			t.Fatalf("UpsertSessionHealth(before) error = %v", err)
		}

		claimedBy := taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: session.ID}
		origin := taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "test"}
		taskRecord := taskpkg.Task{
			ID:          "task-authoring",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: fixture.workspaceID,
			Title:       "Heartbeat authoring safety",
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

		first, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Initial safety policy", "Initial policy does not touch runtime state."),
		})
		if err != nil {
			t.Fatalf("Put(first) error = %v", err)
		}
		second, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body: validHeartbeatBody(
				"Updated safety policy",
				"Updated policy remains advisory and does not touch runtime state.",
			),
			ExpectedDigest: first.Policy.Digest,
		})
		if err != nil {
			t.Fatalf("Put(second) error = %v", err)
		}
		if second.Policy.Digest == first.Policy.Digest {
			t.Fatalf("Put(second).Policy.Digest = %q, want changed digest", second.Policy.Digest)
		}

		healthAfter, err := fixture.db.GetSessionHealth(fixture.ctx, session.ID)
		if err != nil {
			t.Fatalf("GetSessionHealth(after) error = %v", err)
		}
		if healthAfter.Health != healthBefore.Health ||
			healthAfter.State != healthBefore.State ||
			healthAfter.ActivePrompt != healthBefore.ActivePrompt ||
			healthAfter.EligibleForWake != healthBefore.EligibleForWake ||
			!healthAfter.UpdatedAt.Equal(healthBefore.UpdatedAt) {
			t.Fatalf("session health after authoring = %#v, want unchanged %#v", healthAfter, healthBefore)
		}
		states, err := fixture.db.ListHeartbeatWakeState(fixture.ctx, heartbeat.WakeStateListQuery{
			WorkspaceID: fixture.workspaceID,
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeState() error = %v", err)
		}
		if got := len(states); got != 0 {
			t.Fatalf("len(ListHeartbeatWakeState()) = %d, want 0", got)
		}
		events, err := fixture.db.ListHeartbeatWakeEvents(fixture.ctx, heartbeat.WakeEventListQuery{
			WorkspaceID: fixture.workspaceID,
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeEvents() error = %v", err)
		}
		if got := len(events); got != 0 {
			t.Fatalf("len(ListHeartbeatWakeEvents()) = %d, want 0", got)
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

func TestManagedHeartbeatStatusService(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil status store", func(t *testing.T) {
		t.Parallel()

		service, err := heartbeat.NewManagedHeartbeatStatusService(nil)
		if err == nil {
			t.Fatalf("NewManagedHeartbeatStatusService(nil) = %#v, want error", service)
		}
	})

	t.Run("Should compose policy, config, wake state, and session health", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		written, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Compose status", "Compose advisory policy with runtime health."),
		})
		if err != nil {
			t.Fatalf("Put(policy) error = %v", err)
		}
		registerManagedHeartbeatSession(t, fixture, "sess-status")
		health, err := fixture.db.UpsertSessionHealth(
			fixture.ctx,
			managedHeartbeatStaleSessionHealth(fixture.workspaceID, "sess-status", fixture.now.Add(time.Minute)),
		)
		if err != nil {
			t.Fatalf("UpsertSessionHealth() error = %v", err)
		}
		wakeState, err := fixture.db.UpsertHeartbeatWakeState(fixture.ctx, heartbeat.WakeState{
			WorkspaceID:      fixture.workspaceID,
			AgentName:        "coder",
			SessionID:        "sess-status",
			PolicySnapshotID: written.Snapshot.ID,
			LastWakeAt:       fixture.now.Add(2 * time.Minute),
			NextAllowedAt:    fixture.now.Add(time.Hour),
			CoalescedCount:   2,
			LastResult:       heartbeat.WakeResultCoalesced,
			LastReason:       heartbeat.WakeReasonCoalesced,
			UpdatedAt:        fixture.now.Add(3 * time.Minute),
		})
		if err != nil {
			t.Fatalf("UpsertHeartbeatWakeState() error = %v", err)
		}

		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:               fixture.target,
			SessionID:            "sess-status",
			IncludeSessionHealth: true,
		})
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if !status.Valid || !status.Present || !status.Active ||
			status.Digest != written.Policy.Digest ||
			status.SnapshotID != written.Snapshot.ID ||
			status.ConfigDigest != written.Policy.ConfigDigest {
			t.Fatalf("Status() = %#v, want current policy snapshot and config digest", status)
		}
		if status.WakeState == nil || status.WakeState.SessionID != wakeState.SessionID ||
			status.WakeState.LastReason != heartbeat.WakeReasonCoalesced {
			t.Fatalf("Status().WakeState = %#v, want composed wake state %#v", status.WakeState, wakeState)
		}
		if status.SessionHealth == nil || status.SessionHealth.SessionID != health.SessionID ||
			status.SessionHealth.Health != heartbeat.SessionHealthStale ||
			status.SessionHealth.EligibleForWake {
			t.Fatalf(
				"Status().SessionHealth = %#v, want stale wake-ineligible health %#v",
				status.SessionHealth,
				health,
			)
		}
		if len(status.Diagnostics) != 0 {
			t.Fatalf("Status().Diagnostics = %#v, want none", status.Diagnostics)
		}
	})

	t.Run("Should skip session health reads when session health is not requested", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		written, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Wake-only status", "Wake-state reads do not require session health."),
		})
		if err != nil {
			t.Fatalf("Put(policy) error = %v", err)
		}
		registerManagedHeartbeatSession(t, fixture, "sess-wake-only")
		wakeState, err := fixture.db.UpsertHeartbeatWakeState(fixture.ctx, heartbeat.WakeState{
			WorkspaceID:      fixture.workspaceID,
			AgentName:        "coder",
			SessionID:        "sess-wake-only",
			PolicySnapshotID: written.Snapshot.ID,
			LastWakeAt:       fixture.now.Add(2 * time.Minute),
			NextAllowedAt:    fixture.now.Add(time.Hour),
			CoalescedCount:   1,
			LastResult:       heartbeat.WakeResultSent,
			LastReason:       heartbeat.WakeReasonSent,
			UpdatedAt:        fixture.now.Add(3 * time.Minute),
		})
		if err != nil {
			t.Fatalf("UpsertHeartbeatWakeState() error = %v", err)
		}
		statusWithoutReader, err := heartbeat.NewManagedHeartbeatStatusService(fixture.db)
		if err != nil {
			t.Fatalf("NewManagedHeartbeatStatusService(no health reader) error = %v", err)
		}

		status, err := statusWithoutReader.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:    fixture.target,
			SessionID: "sess-wake-only",
		})
		if err != nil {
			t.Fatalf("Status(wake-only) error = %v", err)
		}
		if status.SessionHealth != nil {
			t.Fatalf("Status(wake-only).SessionHealth = %#v, want nil", status.SessionHealth)
		}
		if status.WakeState == nil || status.WakeState.SessionID != wakeState.SessionID ||
			status.WakeState.LastReason != heartbeat.WakeReasonSent {
			t.Fatalf("Status(wake-only).WakeState = %#v, want wake state %#v", status.WakeState, wakeState)
		}
	})

	t.Run("Should inspect missing policy with config provenance and no snapshot", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Status(missing policy) error = %v", err)
		}
		if status.Present || status.Active || !status.Valid ||
			status.Digest != "" ||
			status.SnapshotID != "" ||
			status.ConfigDigest == "" ||
			status.ConfigProvenance.Digest != status.ConfigDigest {
			t.Fatalf("Status(missing policy) = %#v, want valid missing policy with config provenance", status)
		}
	})

	t.Run("Should inspect invalid policy as closed diagnostics without raw data leak", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		secret := "hb-secret-do-not-leak"
		cleanup := diagnostics.RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)
		invalid := `---
summary: "` + secret + `
---
Wake gently.
`
		if err := os.WriteFile(fixture.heartbeatPath, []byte(invalid), 0o644); err != nil {
			t.Fatalf("WriteFile(invalid HEARTBEAT.md) error = %v", err)
		}

		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Status(invalid policy) error = %v", err)
		}
		if !status.Present || status.Valid || status.Active || status.SnapshotID != "" {
			t.Fatalf("Status(invalid policy) = %#v, want present invalid inactive policy without snapshot", status)
		}
		assertHeartbeatDiagnosticCode(t, status.Diagnostics, "heartbeat_malformed_frontmatter")
		if heartbeatDiagnosticsContain(status.Diagnostics, secret) {
			t.Fatalf("Status(invalid policy).Diagnostics leaked registered secret: %#v", status.Diagnostics)
		}
	})

	t.Run("Should report disabled config while keeping authored policy valid", func(t *testing.T) {
		t.Parallel()

		cfg := aghconfig.DefaultHeartbeatConfig()
		cfg.Enabled = false
		fixture := newHeartbeatFixtureWithDBPath(t, filepath.Join(t.TempDir(), aghstore.GlobalDatabaseName), cfg)
		written, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Disabled config", "The policy remains authored but inactive while disabled."),
		})
		if err != nil {
			t.Fatalf("Put(disabled config) error = %v", err)
		}
		status, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{Target: fixture.target})
		if err != nil {
			t.Fatalf("Status(disabled config) error = %v", err)
		}
		if status.Enabled || status.Active || !status.Valid || !status.Present ||
			status.Digest != written.Policy.Digest ||
			status.ConfigDigest != written.Policy.ConfigDigest {
			t.Fatalf("Status(disabled config) = %#v, want present valid inactive policy", status)
		}
	})

	t.Run("Should reject missing and unsupported session health with closed status errors", func(t *testing.T) {
		t.Parallel()

		fixture := newHeartbeatFixture(t)
		if _, err := fixture.authoring.Put(fixture.ctx, heartbeat.PutRequest{
			Target: fixture.target,
			Body:   validHeartbeatBody("Health error policy", "Session health errors stay closed."),
		}); err != nil {
			t.Fatalf("Put(policy) error = %v", err)
		}
		_, err := fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:               fixture.target,
			IncludeSessionHealth: true,
		})
		if !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			t.Fatalf("Status(health without session id) error = %v, want ErrSessionHealthNotFound", err)
		}
		requireHeartbeatStatusCode(t, err, "session_not_found")

		statusWithoutReader, err := heartbeat.NewManagedHeartbeatStatusService(fixture.db)
		if err != nil {
			t.Fatalf("NewManagedHeartbeatStatusService(no health reader) error = %v", err)
		}
		_, err = statusWithoutReader.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:               fixture.target,
			SessionID:            "sess-missing",
			IncludeSessionHealth: true,
		})
		if !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			t.Fatalf("Status(no health reader) error = %v, want ErrSessionHealthNotFound", err)
		}
		requireHeartbeatStatusCode(t, err, "session_not_found")

		_, err = fixture.status.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:               fixture.target,
			SessionID:            "sess-missing",
			IncludeSessionHealth: true,
		})
		if !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			t.Fatalf("Status(missing health) error = %v, want ErrSessionHealthNotFound", err)
		}
		requireHeartbeatStatusCode(t, err, "session_not_found")

		reader := managedHeartbeatHealthReader{
			health: heartbeat.SessionHealth{
				SessionID:   "sess-unsupported",
				WorkspaceID: fixture.workspaceID,
				AgentName:   "coder",
				State:       heartbeat.SessionHealthState("unsupported"),
				Health:      heartbeat.SessionHealthHealthy,
				UpdatedAt:   fixture.now,
			},
		}
		statusService, err := heartbeat.NewManagedHeartbeatStatusService(
			fixture.db,
			heartbeat.WithHeartbeatStatusSessionHealthReader(reader),
		)
		if err != nil {
			t.Fatalf("NewManagedHeartbeatStatusService(custom reader) error = %v", err)
		}
		_, err = statusService.Status(fixture.ctx, heartbeat.StatusRequest{
			Target:               fixture.target,
			SessionID:            "sess-unsupported",
			IncludeSessionHealth: true,
		})
		if !errors.Is(err, heartbeat.ErrInvalidSessionHealth) {
			t.Fatalf("Status(unsupported health) error = %v, want ErrInvalidSessionHealth", err)
		}
		requireHeartbeatStatusCode(t, err, "session_health_unsupported")
	})
}

type heartbeatFixture struct {
	ctx           context.Context
	db            *globaldb.GlobalDB
	authoring     *heartbeat.ManagedHeartbeatAuthoringService
	status        *heartbeat.ManagedHeartbeatStatusService
	root          string
	workspaceID   string
	agentPath     string
	heartbeatPath string
	target        heartbeat.AuthoringTarget
	now           time.Time
}

type managedHeartbeatHealthReader struct {
	health heartbeat.SessionHealth
	err    error
}

type recordingHeartbeatPrompter struct {
	mu       sync.Mutex
	requests []heartbeat.SyntheticWakePromptRequest
}

func (r managedHeartbeatHealthReader) GetSessionHealth(
	context.Context,
	string,
) (heartbeat.SessionHealth, error) {
	if r.err != nil {
		return heartbeat.SessionHealth{}, r.err
	}
	return r.health, nil
}

func (p *recordingHeartbeatPrompter) PromptHeartbeatWake(
	_ context.Context,
	req heartbeat.SyntheticWakePromptRequest,
) (heartbeat.SyntheticWakePromptResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, req)
	return heartbeat.SyntheticWakePromptResult{SyntheticPromptID: req.TurnID}, nil
}

func (p *recordingHeartbeatPrompter) requestsSnapshot() []heartbeat.SyntheticWakePromptRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]heartbeat.SyntheticWakePromptRequest(nil), p.requests...)
}

func eligibleManagedWakeHealth(fixture heartbeatFixture, sessionID string) heartbeat.SessionHealth {
	return heartbeat.SessionHealth{
		SessionID:       sessionID,
		WorkspaceID:     fixture.workspaceID,
		AgentName:       "coder",
		State:           heartbeat.SessionHealthStateIdle,
		Health:          heartbeat.SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		LastActivityAt:  fixture.now.Add(-2 * time.Minute),
		LastPresenceAt:  fixture.now.Add(-time.Minute),
		UpdatedAt:       fixture.now,
	}
}

func newHeartbeatFixture(t *testing.T) heartbeatFixture {
	t.Helper()

	return newHeartbeatFixtureWithDBPath(
		t,
		filepath.Join(t.TempDir(), aghstore.GlobalDatabaseName),
		aghconfig.DefaultHeartbeatConfig(),
	)
}

func newHeartbeatFixtureWithDBPath(
	t *testing.T,
	dbPath string,
	cfg aghconfig.HeartbeatConfig,
) heartbeatFixture {
	t.Helper()

	ctx := testutil.Context(t)
	root := t.TempDir()
	agentPath := writeHeartbeatAgentDefinition(t, root, "coder")
	globalDB, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := globalDB.Close(testutil.Context(t)); err != nil {
			t.Errorf("Close(GlobalDB) error = %v", err)
		}
	})
	workspaceID := "ws-heartbeat-authoring"
	if err := globalDB.InsertWorkspace(ctx, aghworkspace.Workspace{
		ID:      workspaceID,
		RootDir: root,
		Name:    "heartbeat-authoring",
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	authoring, err := heartbeat.NewManagedHeartbeatAuthoringService(
		globalDB,
		heartbeat.WithHeartbeatAuthoringClock(deterministicHeartbeatClock(now)),
		heartbeat.WithHeartbeatAuthoringIDGenerator(deterministicHeartbeatIDGenerator()),
	)
	if err != nil {
		t.Fatalf("NewManagedHeartbeatAuthoringService() error = %v", err)
	}
	status, err := heartbeat.NewManagedHeartbeatStatusService(
		globalDB,
		heartbeat.WithHeartbeatStatusSessionHealthReader(globalDB),
	)
	if err != nil {
		t.Fatalf("NewManagedHeartbeatStatusService() error = %v", err)
	}
	target := heartbeat.AuthoringTarget{
		WorkspaceID:   workspaceID,
		WorkspaceRoot: root,
		AgentName:     "coder",
		AgentPath:     agentPath,
		Config:        cfg,
	}
	return heartbeatFixture{
		ctx:           ctx,
		db:            globalDB,
		authoring:     authoring,
		status:        status,
		root:          root,
		workspaceID:   workspaceID,
		agentPath:     agentPath,
		heartbeatPath: filepath.Join(filepath.Dir(agentPath), heartbeat.FileName),
		target:        target,
		now:           now,
	}
}

func writeHeartbeatAgentDefinition(t *testing.T, root string, agentName string) string {
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

func validHeartbeatBody(summary string, body string) string {
	return fmt.Sprintf(`---
version: "1"
enabled: true
summary: "%s"
preferences:
  min_interval: "30m"
context:
  include:
    - self
    - session_health
    - task
---
%s
`, summary, body)
}

func validHeartbeatBodyWithWindows(summary string, body string) string {
	return fmt.Sprintf(`---
version: "1"
enabled: true
summary: "%s"
preferences:
  min_interval: "30m"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "20:00"
  quiet_windows:
    - timezone: "America/Sao_Paulo"
      start: "22:00"
      end: "08:00"
context:
  include:
    - self
    - session_health
    - task
---
%s
`, summary, body)
}

func assertHeartbeatFileContent(t *testing.T, path string, want string) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, string(got), want)
	}
}

func assertHeartbeatRevisionCount(t *testing.T, fixture heartbeatFixture, want int) {
	t.Helper()

	history, err := fixture.authoring.History(fixture.ctx, heartbeat.HistoryRequest{Target: fixture.target})
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if got := len(history.Revisions); got != want {
		t.Fatalf("len(History().Revisions) = %d, want %d", got, want)
	}
}

func assertHeartbeatSnapshotCount(t *testing.T, fixture heartbeatFixture, want int) {
	t.Helper()

	snapshots, err := fixture.db.ListHeartbeatSnapshots(fixture.ctx, heartbeat.SnapshotListQuery{
		WorkspaceID: fixture.workspaceID,
		AgentName:   "coder",
	})
	if err != nil {
		t.Fatalf("ListHeartbeatSnapshots() error = %v", err)
	}
	if got := len(snapshots); got != want {
		t.Fatalf("len(ListHeartbeatSnapshots()) = %d, want %d", got, want)
	}
}

func requireHeartbeatAuthoringCode(t *testing.T, err error, code string) *heartbeat.AuthoringError {
	t.Helper()

	var authoringErr *heartbeat.AuthoringError
	if !errors.As(err, &authoringErr) {
		t.Fatalf("error = %T %[1]v, want *heartbeat.AuthoringError", err)
	}
	if message := authoringErr.Error(); strings.TrimSpace(message) == "" {
		t.Fatal("AuthoringError.Error() = empty, want deterministic message")
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

func requireHeartbeatStatusCode(t *testing.T, err error, code string) *heartbeat.StatusError {
	t.Helper()

	var statusErr *heartbeat.StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error = %T %[1]v, want *heartbeat.StatusError", err)
	}
	if message := statusErr.Error(); strings.TrimSpace(message) == "" {
		t.Fatal("StatusError.Error() = empty, want deterministic message")
	}
	if statusErr.Code != code {
		t.Fatalf(
			"StatusError.Code = %q, want %q; diagnostics=%#v",
			statusErr.Code,
			code,
			statusErr.Diagnostics,
		)
	}
	return statusErr
}

func assertHeartbeatDiagnosticCode(t *testing.T, list []heartbeat.Diagnostic, code string) {
	t.Helper()

	for index := range list {
		if list[index].Code == code {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %q", list, code)
}

func heartbeatDiagnosticsContain(list []heartbeat.Diagnostic, needle string) bool {
	for index := range list {
		if strings.Contains(list[index].Message, needle) ||
			strings.Contains(list[index].Field, needle) ||
			strings.Contains(list[index].Section, needle) ||
			strings.Contains(list[index].SourcePath, needle) {
			return true
		}
	}
	return false
}

func withHeartbeatAgentPath(target heartbeat.AuthoringTarget, agentPath string) heartbeat.AuthoringTarget {
	target.AgentPath = agentPath
	return target
}

func deterministicHeartbeatClock(start time.Time) func() time.Time {
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

func deterministicHeartbeatIDGenerator() func(prefix string) string {
	var mu sync.Mutex
	counters := make(map[string]int)
	return func(prefix string) string {
		mu.Lock()
		defer mu.Unlock()
		counters[prefix]++
		return fmt.Sprintf("%s-%02d", prefix, counters[prefix])
	}
}

func registerManagedHeartbeatSession(t *testing.T, fixture heartbeatFixture, sessionID string) {
	t.Helper()

	if err := fixture.db.RegisterSession(fixture.ctx, aghstore.SessionInfo{
		ID:          sessionID,
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: fixture.workspaceID,
		State:       "active",
		CreatedAt:   fixture.now,
		UpdatedAt:   fixture.now,
	}); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
	}
}

func managedHeartbeatSessionHealth(
	workspaceID string,
	sessionID string,
	updatedAt time.Time,
) heartbeat.SessionHealth {
	return heartbeat.SessionHealth{
		SessionID:       sessionID,
		WorkspaceID:     workspaceID,
		AgentName:       "coder",
		State:           heartbeat.SessionHealthStateIdle,
		Health:          heartbeat.SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		LastActivityAt:  updatedAt.Add(-2 * time.Minute),
		LastPresenceAt:  updatedAt.Add(-time.Minute),
		UpdatedAt:       updatedAt,
	}
}

func managedHeartbeatStaleSessionHealth(
	workspaceID string,
	sessionID string,
	updatedAt time.Time,
) heartbeat.SessionHealth {
	health := managedHeartbeatSessionHealth(workspaceID, sessionID, updatedAt)
	health.Health = heartbeat.SessionHealthStale
	health.EligibleForWake = false
	health.IneligibilityReason = string(heartbeat.SessionHealthReasonStale)
	return health
}
