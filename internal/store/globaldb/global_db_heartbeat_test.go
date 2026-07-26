package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/heartbeat"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBHeartbeatSnapshotAndRevisionStore(t *testing.T) {
	t.Parallel()

	t.Run("Should persist resolver snapshots and authoring revisions across reopen", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
			t,
			first,
			"heartbeat-reopen",
			"sess-heartbeat-reopen",
		)

		snapshot := heartbeatResolvedSnapshotForTest(ctx, t, workspaceID, "coder", "hb-reopen")
		saved, err := first.UpsertHeartbeatSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot() error = %v", err)
		}
		duplicate := snapshot
		duplicate.ID = "hb-reopen-duplicate"
		reused, err := first.UpsertHeartbeatSnapshot(ctx, duplicate)
		if err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot(duplicate digest) error = %v", err)
		}
		if reused.ID != saved.ID {
			t.Fatalf("reused.ID = %q, want existing snapshot id %q", reused.ID, saved.ID)
		}

		revision := heartbeatRevisionForTest(
			"hb-rev-reopen",
			workspaceID,
			"coder",
			heartbeat.RevisionOperationWrite,
			"",
			saved.Digest,
		)
		revision.NewSnapshotID = saved.ID
		revision.Body = saved.Body
		if _, err := first.AppendHeartbeatRevision(ctx, revision); err != nil {
			t.Fatalf("AppendHeartbeatRevision() error = %v", err)
		}
		gotRevision, err := first.GetHeartbeatRevision(ctx, revision.ID)
		if err != nil {
			t.Fatalf("GetHeartbeatRevision() error = %v", err)
		}
		if gotRevision.NewSnapshotID != saved.ID || gotRevision.NewDigest != saved.Digest {
			t.Fatalf("GetHeartbeatRevision() = %#v, want saved snapshot linkage", gotRevision)
		}
		revisions, err := first.ListHeartbeatRevisions(ctx, heartbeat.RevisionListQuery{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
			Operation:   heartbeat.RevisionOperationWrite,
		})
		if err != nil {
			t.Fatalf("ListHeartbeatRevisions() error = %v", err)
		}
		if got, want := heartbeatRevisionIDs(revisions), []string{revision.ID}; !slices.Equal(got, want) {
			t.Fatalf("revision ids = %#v, want %#v", got, want)
		}
		health := heartbeatSessionHealthForTest(sessionID, workspaceID, "coder", saved.CreatedAt.Add(time.Hour))
		if _, err := first.UpsertSessionHealth(ctx, health); err != nil {
			t.Fatalf("UpsertSessionHealth() error = %v", err)
		}
		wakeState := heartbeatWakeStateForTest(
			sessionID,
			workspaceID,
			"coder",
			saved.ID,
			saved.CreatedAt.Add(time.Hour),
		)
		if _, err := first.UpsertHeartbeatWakeState(ctx, wakeState); err != nil {
			t.Fatalf("UpsertHeartbeatWakeState() error = %v", err)
		}
		wakeEvent := heartbeatWakeEventForTest(
			"hb-event-reopen",
			sessionID,
			workspaceID,
			"coder",
			saved.ID,
			saved.CreatedAt.Add(time.Hour),
		)
		if _, err := first.AppendHeartbeatWakeEvent(ctx, wakeEvent); err != nil {
			t.Fatalf("AppendHeartbeatWakeEvent() error = %v", err)
		}
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

		gotSnapshot, err := second.GetHeartbeatSnapshot(ctx, saved.ID)
		if err != nil {
			t.Fatalf("GetHeartbeatSnapshot() error = %v", err)
		}
		envelope, err := gotSnapshot.ResolvedEnvelope()
		if err != nil {
			t.Fatalf("ResolvedEnvelope() error = %v", err)
		}
		if gotSnapshot.Digest != saved.Digest ||
			gotSnapshot.ConfigDigest != saved.ConfigDigest ||
			!envelope.Valid ||
			envelope.ConfigProvenance.Digest != saved.ConfigDigest {
			t.Fatalf("GetHeartbeatSnapshot() = %#v envelope %#v, want saved resolver provenance", gotSnapshot, envelope)
		}
		latest, err := second.GetLatestValidHeartbeatSnapshot(ctx, workspaceID, "coder")
		if err != nil {
			t.Fatalf("GetLatestValidHeartbeatSnapshot() error = %v", err)
		}
		if latest.ID != saved.ID {
			t.Fatalf("latest snapshot id = %q, want %q", latest.ID, saved.ID)
		}
		found, ok, err := second.FindHeartbeatSnapshotByDigest(ctx, workspaceID, "coder", saved.Digest)
		if err != nil {
			t.Fatalf("FindHeartbeatSnapshotByDigest() error = %v", err)
		}
		if !ok || found.ID != saved.ID {
			t.Fatalf("FindHeartbeatSnapshotByDigest() = %#v, %v; want saved snapshot", found, ok)
		}
		gotRevision, err = second.FindHeartbeatRevisionForRollback(ctx, heartbeat.RollbackLookup{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
			RevisionID:  "hb-rev-reopen",
		})
		if err != nil {
			t.Fatalf("FindHeartbeatRevisionForRollback() error = %v", err)
		}
		if gotRevision.Body != saved.Body || gotRevision.NewSnapshotID != saved.ID {
			t.Fatalf("rollback revision = %#v, want saved body and snapshot id", gotRevision)
		}
		gotHealth, err := second.GetSessionHealth(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSessionHealth() error = %v", err)
		}
		if gotHealth.Health != heartbeat.SessionHealthHealthy || !gotHealth.EligibleForWake {
			t.Fatalf("GetSessionHealth() = %#v, want healthy eligible row", gotHealth)
		}
		gotWakeState, err := second.GetHeartbeatWakeState(ctx, workspaceID, "coder", sessionID)
		if err != nil {
			t.Fatalf("GetHeartbeatWakeState() error = %v", err)
		}
		if gotWakeState.PolicySnapshotID != saved.ID || gotWakeState.LastReason != heartbeat.WakeReasonSent {
			t.Fatalf("GetHeartbeatWakeState() = %#v, want saved state", gotWakeState)
		}
		gotWakeEvent, err := second.GetHeartbeatWakeEvent(ctx, "hb-event-reopen")
		if err != nil {
			t.Fatalf("GetHeartbeatWakeEvent() error = %v", err)
		}
		if gotWakeEvent.PolicySnapshotID != saved.ID || gotWakeEvent.SessionID != sessionID {
			t.Fatalf("GetHeartbeatWakeEvent() = %#v, want saved event", gotWakeEvent)
		}
	})

	t.Run("Should reuse one snapshot for concurrent duplicate digest upserts", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "heartbeat-concurrent-digest", t.TempDir())
		snapshot := heartbeatSnapshotForTest("hb-concurrent-base", workspaceID, "coder", "sha256:concurrent-heartbeat")
		type outcome struct {
			snapshot heartbeat.Snapshot
			err      error
		}
		const upsertCount = 32
		start := make(chan struct{})
		outcomes := make(chan outcome, upsertCount)
		for index := range upsertCount {
			go func() {
				<-start
				candidate := snapshot
				candidate.ID = "hb-concurrent-" + strconv.Itoa(index)
				saved, err := globalDB.UpsertHeartbeatSnapshot(ctx, candidate)
				outcomes <- outcome{snapshot: saved, err: err}
			}()
		}

		close(start)
		winnerID := ""
		for index := range upsertCount {
			got := <-outcomes
			if got.err != nil {
				t.Fatalf("UpsertHeartbeatSnapshot(concurrent duplicate %d) error = %v", index, got.err)
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
			FROM agent_heartbeat_snapshots
			WHERE workspace_id = ? AND agent_name = ? AND digest = ?`,
			workspaceID,
			"coder",
			snapshot.Digest,
		).Scan(&rowCount); err != nil {
			t.Fatalf("QueryRowContext(agent_heartbeat_snapshots count) error = %v", err)
		}
		if rowCount != 1 {
			t.Fatalf("agent_heartbeat_snapshots count = %d, want 1", rowCount)
		}
	})

	t.Run("Should reject invalid snapshots revisions and missing workspace references", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(t, globalDB, "heartbeat-constraints", t.TempDir())

		snapshot := heartbeatSnapshotForTest("hb-constraint", workspaceID, "coder", "sha256:heartbeat")
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot(first) error = %v", err)
		}
		duplicateID := heartbeatSnapshotForTest("hb-constraint", workspaceID, "coder", "sha256:other")
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, duplicateID); err == nil {
			t.Fatal("UpsertHeartbeatSnapshot(duplicate id) error = nil, want constraint failure")
		}
		malformed := heartbeatSnapshotForTest("hb-malformed", workspaceID, "coder", "sha256:malformed")
		malformed.ResolvedJSON = json.RawMessage(`{`)
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, malformed); !errors.Is(err, heartbeat.ErrInvalidSnapshot) {
			t.Fatalf("UpsertHeartbeatSnapshot(malformed) error = %v, want ErrInvalidSnapshot", err)
		}
		missingWorkspace := heartbeatSnapshotForTest("hb-missing-workspace", "ws-missing", "coder", "sha256:missing")
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, missingWorkspace); err == nil {
			t.Fatal("UpsertHeartbeatSnapshot(missing workspace) error = nil, want foreign key failure")
		}

		invalidRevision := heartbeatRevisionForTest(
			"hb-rev-invalid",
			workspaceID,
			"coder",
			heartbeat.RevisionOperation("replace"),
			"",
			"sha256:invalid",
		)
		if _, err := globalDB.AppendHeartbeatRevision(ctx, invalidRevision); !errors.Is(
			err,
			heartbeat.ErrInvalidRevision,
		) {
			t.Fatalf("AppendHeartbeatRevision(invalid operation) error = %v, want ErrInvalidRevision", err)
		}
	})
}

func TestGlobalDBSessionHealthStore(t *testing.T) {
	t.Parallel()

	t.Run("Should load one bounded page of health rows by ids", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceOne, sessionOne := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-health-batch-one",
			"sess-health-batch-one",
		)
		workspaceTwo, sessionTwo := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-health-batch-two",
			"sess-health-batch-two",
		)
		baseAt := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
		for _, health := range []heartbeat.SessionHealth{
			heartbeatSessionHealthForTest(sessionOne, workspaceOne, "coder", baseAt),
			heartbeatSessionHealthForTest(sessionTwo, workspaceTwo, "coder", baseAt.Add(time.Minute)),
		} {
			if _, err := globalDB.UpsertSessionHealth(ctx, health); err != nil {
				t.Fatalf("UpsertSessionHealth(%q) error = %v", health.SessionID, err)
			}
		}

		rows, err := globalDB.ListSessionHealthByIDs(ctx, []string{
			" " + sessionOne + " ",
			sessionTwo,
			sessionOne,
			"sess-missing",
		})
		if err != nil {
			t.Fatalf("ListSessionHealthByIDs() error = %v", err)
		}
		gotIDs := make([]string, 0, len(rows))
		for _, row := range rows {
			gotIDs = append(gotIDs, row.SessionID)
		}
		if want := []string{sessionTwo, sessionOne}; !slices.Equal(gotIDs, want) {
			t.Fatalf("ListSessionHealthByIDs() ids = %#v, want %#v", gotIDs, want)
		}
	})

	t.Run("Should upsert read recover and mark stale rows without deleting policy snapshots", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(t, globalDB, "heartbeat-health", "sess-health")
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot := heartbeatSnapshotForTest("hb-health-snapshot", workspaceID, "coder", "sha256:health")
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot() error = %v", err)
		}

		health := heartbeatSessionHealthForTest(sessionID, workspaceID, "coder", baseAt)
		if _, err := globalDB.UpsertSessionHealth(ctx, health); err != nil {
			t.Fatalf("UpsertSessionHealth(first) error = %v", err)
		}
		health.Health = heartbeat.SessionHealthDegraded
		health.EligibleForWake = false
		health.IneligibilityReason = "session_unhealthy"
		health.UpdatedAt = baseAt.Add(time.Minute)
		if _, err := globalDB.UpsertSessionHealth(ctx, health); err != nil {
			t.Fatalf("UpsertSessionHealth(update) error = %v", err)
		}
		got, err := globalDB.GetSessionHealth(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSessionHealth() error = %v", err)
		}
		if got.Health != heartbeat.SessionHealthDegraded ||
			got.EligibleForWake ||
			got.IneligibilityReason != "session_unhealthy" {
			t.Fatalf("GetSessionHealth() = %#v, want updated degraded ineligible row", got)
		}

		recoveryInputs, err := globalDB.ListSessionHealthRecoveryInputs(ctx, 10)
		if err != nil {
			t.Fatalf("ListSessionHealthRecoveryInputs() error = %v", err)
		}
		if got, want := len(recoveryInputs), 1; got != want {
			t.Fatalf("len(recoveryInputs) = %d, want %d", got, want)
		}
		if recoveryInputs[0].SessionID != sessionID {
			t.Fatalf("recoveryInputs[0] = %#v, want session %q", recoveryInputs[0], sessionID)
		}

		marked, err := globalDB.MarkSessionHealthStale(
			baseContext(t),
			baseAt.Add(30*time.Minute),
			baseAt.Add(time.Hour),
		)
		if err != nil {
			t.Fatalf("MarkSessionHealthStale() error = %v", err)
		}
		if got, want := marked, int64(1); got != want {
			t.Fatalf("MarkSessionHealthStale() = %d, want %d", got, want)
		}
		stale, err := globalDB.GetSessionHealth(ctx, sessionID)
		if err != nil {
			t.Fatalf("GetSessionHealth(stale) error = %v", err)
		}
		if stale.Health != heartbeat.SessionHealthStale ||
			stale.EligibleForWake ||
			stale.IneligibilityReason != "session_health_stale" {
			t.Fatalf("stale session health = %#v, want stale and ineligible", stale)
		}
		preserved, err := globalDB.GetHeartbeatSnapshot(ctx, "hb-health-snapshot")
		if err != nil {
			t.Fatalf("GetHeartbeatSnapshot(after stale) error = %v", err)
		}
		if preserved.Digest != snapshot.Digest {
			t.Fatalf("snapshot digest after stale = %q, want %q", preserved.Digest, snapshot.Digest)
		}
	})

	t.Run("Should cascade session health when the session is deleted", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-health-cascade",
			"sess-health-cascade",
		)
		if _, err := globalDB.UpsertSessionHealth(
			ctx,
			heartbeatSessionHealthForTest(
				sessionID,
				workspaceID,
				"coder",
				time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
			),
		); err != nil {
			t.Fatalf("UpsertSessionHealth() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
			t.Fatalf("DELETE sessions error = %v", err)
		}
		if _, err := globalDB.GetSessionHealth(ctx, sessionID); !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
			t.Fatalf("GetSessionHealth(after cascade) error = %v, want ErrSessionHealthNotFound", err)
		}
	})
}

func TestGlobalDBHeartbeatWakeAuditStore(t *testing.T) {
	t.Parallel()

	t.Run("Should persist wake state and events with closed reason constraints", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(t, globalDB, "heartbeat-wake", "sess-wake")
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		snapshot := heartbeatSnapshotForTest("hb-wake-snapshot", workspaceID, "coder", "sha256:wake")
		if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot() error = %v", err)
		}

		state := heartbeatWakeStateForTest(sessionID, workspaceID, "coder", snapshot.ID, baseAt)
		if _, err := globalDB.UpsertHeartbeatWakeState(ctx, state); err != nil {
			t.Fatalf("UpsertHeartbeatWakeState(first) error = %v", err)
		}
		state.CoalescedCount = 2
		state.LastResult = heartbeat.WakeResultCoalesced
		state.LastReason = heartbeat.WakeReasonCoalesced
		state.UpdatedAt = baseAt.Add(time.Minute)
		if _, err := globalDB.UpsertHeartbeatWakeState(ctx, state); err != nil {
			t.Fatalf("UpsertHeartbeatWakeState(update) error = %v", err)
		}
		gotState, err := globalDB.GetHeartbeatWakeState(ctx, workspaceID, "coder", sessionID)
		if err != nil {
			t.Fatalf("GetHeartbeatWakeState() error = %v", err)
		}
		if gotState.CoalescedCount != 2 || gotState.LastReason != heartbeat.WakeReasonCoalesced {
			t.Fatalf("GetHeartbeatWakeState() = %#v, want updated coalesced state", gotState)
		}
		states, err := globalDB.ListHeartbeatWakeState(ctx, heartbeat.WakeStateListQuery{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
		})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeState() error = %v", err)
		}
		if got, want := heartbeatWakeStateSessionIDs(states), []string{sessionID}; !slices.Equal(got, want) {
			t.Fatalf("wake state session ids = %#v, want %#v", got, want)
		}

		event := heartbeatWakeEventForTest("hb-event-sent", sessionID, workspaceID, "coder", snapshot.ID, baseAt)
		if _, err := globalDB.AppendHeartbeatWakeEvent(ctx, event); err != nil {
			t.Fatalf("AppendHeartbeatWakeEvent(sent) error = %v", err)
		}
		skipped := heartbeatWakeEventForTest("hb-event-skipped", sessionID, workspaceID, "coder", snapshot.ID, baseAt)
		skipped.Result = heartbeat.WakeResultSkipped
		skipped.Reason = heartbeat.WakeReasonQuietWindow
		skipped.CreatedAt = baseAt.Add(time.Minute)
		skipped.ExpiresAt = baseAt.Add(24 * time.Hour)
		if _, err := globalDB.AppendHeartbeatWakeEvent(ctx, skipped); err != nil {
			t.Fatalf("AppendHeartbeatWakeEvent(skipped) error = %v", err)
		}
		events, err := globalDB.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{
			WorkspaceID: workspaceID,
			AgentName:   "coder",
			Result:      heartbeat.WakeResultSkipped,
		})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeEvents() error = %v", err)
		}
		if got, want := heartbeatWakeEventIDs(events), []string{"hb-event-skipped"}; !slices.Equal(got, want) {
			t.Fatalf("wake event ids = %#v, want %#v", got, want)
		}

		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO agent_heartbeat_wake_events (
				id, workspace_id, agent_name, session_id, policy_snapshot_id, source, result, reason, created_at, expires_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"hb-event-invalid-reason",
			workspaceID,
			"coder",
			sessionID,
			snapshot.ID,
			string(heartbeat.WakeSourceScheduler),
			string(heartbeat.WakeResultSkipped),
			"claim_owner_assigned",
			store.FormatTimestamp(baseAt),
			store.FormatTimestamp(baseAt.Add(time.Hour)),
		); err == nil {
			t.Fatal("INSERT invalid wake reason error = nil, want CHECK constraint failure")
		}
	})

	t.Run("Should retain only non-expired wake audit rows and respect batch limits", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-retention",
			"sess-retention",
		)
		cutoff := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
		for _, event := range []heartbeat.WakeEvent{
			heartbeatWakeEventForTest(
				"hb-event-old-a",
				sessionID,
				workspaceID,
				"coder",
				"",
				cutoff.Add(-2*time.Hour),
			),
			heartbeatWakeEventForTest(
				"hb-event-old-b",
				sessionID,
				workspaceID,
				"coder",
				"",
				cutoff.Add(-time.Hour),
			),
			heartbeatWakeEventForTest(
				"hb-event-boundary",
				sessionID,
				workspaceID,
				"coder",
				"",
				cutoff,
			),
			heartbeatWakeEventForTest(
				"hb-event-fresh",
				sessionID,
				workspaceID,
				"coder",
				"",
				cutoff.Add(time.Hour),
			),
		} {
			event.ExpiresAt = event.CreatedAt
			if _, err := globalDB.AppendHeartbeatWakeEvent(ctx, event); err != nil {
				t.Fatalf("AppendHeartbeatWakeEvent(%q) error = %v", event.ID, err)
			}
		}

		deleted, err := globalDB.SweepHeartbeatWakeEvents(ctx, cutoff, 1)
		if err != nil {
			t.Fatalf("SweepHeartbeatWakeEvents(first) error = %v", err)
		}
		if got, want := deleted, int64(1); got != want {
			t.Fatalf("SweepHeartbeatWakeEvents(first) = %d, want %d", got, want)
		}
		events, err := globalDB.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{WorkspaceID: workspaceID})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeEvents(after first sweep) error = %v", err)
		}
		if got, want := heartbeatWakeEventIDs(events), []string{
			"hb-event-fresh",
			"hb-event-boundary",
			"hb-event-old-b",
		}; !slices.Equal(got, want) {
			t.Fatalf("wake event ids after first sweep = %#v, want %#v", got, want)
		}

		deleted, err = globalDB.SweepHeartbeatWakeEvents(ctx, cutoff, 10)
		if err != nil {
			t.Fatalf("SweepHeartbeatWakeEvents(second) error = %v", err)
		}
		if got, want := deleted, int64(1); got != want {
			t.Fatalf("SweepHeartbeatWakeEvents(second) = %d, want %d", got, want)
		}
		events, err = globalDB.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{WorkspaceID: workspaceID})
		if err != nil {
			t.Fatalf("ListHeartbeatWakeEvents(after second sweep) error = %v", err)
		}
		if got, want := heartbeatWakeEventIDs(events), []string{"hb-event-fresh", "hb-event-boundary"}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("wake event ids after second sweep = %#v, want %#v", got, want)
		}
	})

	t.Run(
		"Should cascade wake state and null retained wake event references on session and snapshot delete",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			globalDB := openTestGlobalDB(t)
			workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
				t,
				globalDB,
				"heartbeat-wake-cascade",
				"sess-wake-cascade",
			)
			baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
			snapshot := heartbeatSnapshotForTest("hb-wake-cascade", workspaceID, "coder", "sha256:wake-cascade")
			if _, err := globalDB.UpsertHeartbeatSnapshot(ctx, snapshot); err != nil {
				t.Fatalf("UpsertHeartbeatSnapshot() error = %v", err)
			}
			if _, err := globalDB.UpsertHeartbeatWakeState(
				ctx,
				heartbeatWakeStateForTest(sessionID, workspaceID, "coder", snapshot.ID, baseAt),
			); err != nil {
				t.Fatalf("UpsertHeartbeatWakeState() error = %v", err)
			}
			if _, err := globalDB.AppendHeartbeatWakeEvent(
				ctx,
				heartbeatWakeEventForTest("hb-event-cascade", sessionID, workspaceID, "coder", snapshot.ID, baseAt),
			); err != nil {
				t.Fatalf("AppendHeartbeatWakeEvent() error = %v", err)
			}

			if _, err := globalDB.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, sessionID); err != nil {
				t.Fatalf("DELETE sessions error = %v", err)
			}
			if _, err := globalDB.GetHeartbeatWakeState(ctx, workspaceID, "coder", sessionID); !errors.Is(
				err,
				heartbeat.ErrWakeStateNotFound,
			) {
				t.Fatalf("GetHeartbeatWakeState(after session delete) error = %v, want ErrWakeStateNotFound", err)
			}
			event, err := globalDB.GetHeartbeatWakeEvent(ctx, "hb-event-cascade")
			if err != nil {
				t.Fatalf("GetHeartbeatWakeEvent(after session delete) error = %v", err)
			}
			if event.SessionID != "" {
				t.Fatalf("wake event session id after session delete = %q, want null", event.SessionID)
			}
			if _, err := globalDB.db.ExecContext(
				ctx,
				`DELETE FROM agent_heartbeat_snapshots WHERE id = ?`,
				snapshot.ID,
			); err != nil {
				t.Fatalf("DELETE agent_heartbeat_snapshots error = %v", err)
			}
			event, err = globalDB.GetHeartbeatWakeEvent(ctx, "hb-event-cascade")
			if err != nil {
				t.Fatalf("GetHeartbeatWakeEvent(after snapshot delete) error = %v", err)
			}
			if event.PolicySnapshotID != "" {
				t.Fatalf("wake event snapshot id after snapshot delete = %q, want null", event.PolicySnapshotID)
			}
		},
	)
}

func TestGlobalDBHeartbeatStoreDefaultsAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should generate defaults and return typed store errors", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID, sessionID := registerHeartbeatWorkspaceAndSession(
			t,
			globalDB,
			"heartbeat-defaults",
			"sess-heartbeat-defaults",
		)
		baseAt := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)

		snapshot := heartbeatSnapshotForTest("", workspaceID, "coder", "sha256:default")
		snapshot.CreatedAt = time.Time{}
		savedSnapshot, err := globalDB.UpsertHeartbeatSnapshot(ctx, snapshot)
		if err != nil {
			t.Fatalf("UpsertHeartbeatSnapshot(defaults) error = %v", err)
		}
		if savedSnapshot.ID == "" || savedSnapshot.CreatedAt.IsZero() {
			t.Fatalf("saved snapshot = %#v, want generated id and timestamp", savedSnapshot)
		}
		revision := heartbeatRevisionForTest(
			"",
			workspaceID,
			"coder",
			heartbeat.RevisionOperationRollback,
			"sha256:previous",
			savedSnapshot.Digest,
		)
		revision.NewSnapshotID = savedSnapshot.ID
		revision.CreatedAt = time.Time{}
		savedRevision, err := globalDB.AppendHeartbeatRevision(ctx, revision)
		if err != nil {
			t.Fatalf("AppendHeartbeatRevision(defaults) error = %v", err)
		}
		if savedRevision.ID == "" || savedRevision.CreatedAt.IsZero() {
			t.Fatalf("saved revision = %#v, want generated id and timestamp", savedRevision)
		}
		health := heartbeatSessionHealthForTest(sessionID, workspaceID, "coder", baseAt)
		health.UpdatedAt = time.Time{}
		savedHealth, err := globalDB.UpsertSessionHealth(ctx, health)
		if err != nil {
			t.Fatalf("UpsertSessionHealth(defaults) error = %v", err)
		}
		if savedHealth.UpdatedAt.IsZero() {
			t.Fatalf("saved health = %#v, want generated updated_at", savedHealth)
		}
		wakeState := heartbeatWakeStateForTest(sessionID, workspaceID, "coder", savedSnapshot.ID, baseAt)
		wakeState.UpdatedAt = time.Time{}
		savedWakeState, err := globalDB.UpsertHeartbeatWakeState(ctx, wakeState)
		if err != nil {
			t.Fatalf("UpsertHeartbeatWakeState(defaults) error = %v", err)
		}
		if savedWakeState.UpdatedAt.IsZero() {
			t.Fatalf("saved wake state = %#v, want generated updated_at", savedWakeState)
		}
		wakeEvent := heartbeatWakeEventForTest("", sessionID, workspaceID, "coder", savedSnapshot.ID, baseAt)
		wakeEvent.CreatedAt = time.Time{}
		savedWakeEvent, err := globalDB.AppendHeartbeatWakeEvent(ctx, wakeEvent)
		if err != nil {
			t.Fatalf("AppendHeartbeatWakeEvent(defaults) error = %v", err)
		}
		if savedWakeEvent.ID == "" || savedWakeEvent.CreatedAt.IsZero() {
			t.Fatalf("saved wake event = %#v, want generated id and created_at", savedWakeEvent)
		}

		assertHeartbeatStoreErrors(t, globalDB, workspaceID)
	})
}

func TestGlobalDBHeartbeatStorageBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should avoid queue lease claim and network greet columns in Heartbeat storage", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		heartbeatTables := []string{
			"agent_heartbeat_snapshots",
			"agent_heartbeat_revisions",
			"session_health",
			"agent_heartbeat_wake_state",
			"agent_heartbeat_wake_events",
		}
		for _, table := range heartbeatTables {
			assertTableExcludesColumns(t, globalDB.db, table, []string{
				"claim_owner",
				"claim_token",
				"claim_expires_at",
				"lease_owner",
				"lease_expires_at",
				"task_run_id",
				"run_id",
				"queue_state",
			})
		}
		assertTableExcludesColumns(t, globalDB.db, "task_runs", []string{
			"heartbeat_digest",
			"heartbeat_snapshot_id",
			"wake_event_id",
			"wake_reason",
		})
		assertTableExcludesColumns(t, globalDB.db, "network_audit_log", []string{
			"heartbeat_digest",
			"session_health",
			"wake_event_id",
		})
		assertTableExcludesColumns(t, globalDB.db, "network_channels", []string{
			"heartbeat_digest",
			"session_health",
			"wake_event_id",
		})
	})
}

func registerHeartbeatWorkspaceAndSession(
	t *testing.T,
	globalDB *GlobalDB,
	workspaceName string,
	sessionID string,
) (string, string) {
	t.Helper()

	workspaceID := registerWorkspaceForGlobalTests(t, globalDB, workspaceName, t.TempDir())
	now := time.Date(2026, 5, 2, 11, 0, 0, 0, time.UTC)
	if err := globalDB.RegisterSession(testutil.Context(t), store.SessionInfo{
		ID:          sessionID,
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		State:       "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("RegisterSession(%q) error = %v", sessionID, err)
	}
	return workspaceID, sessionID
}

func heartbeatResolvedSnapshotForTest(
	ctx context.Context,
	t *testing.T,
	workspaceID string,
	agentName string,
	id string,
) heartbeat.Snapshot {
	t.Helper()

	root := t.TempDir()
	sourcePath := filepath.Join(root, ".agh", "agents", agentName, heartbeat.FileName)
	resolved, err := heartbeat.Parse(ctx, heartbeat.ParseRequest{
		SourcePath:    sourcePath,
		WorkspaceRoot: root,
		Content: []byte(`---
version: 1
enabled: true
summary: "Review active session state before waking the agent."
preferences:
  min_interval: "30m"
---
Inspect context, then send at most one synthetic wake prompt.
`),
		Config: aghconfig.DefaultHeartbeatConfig(),
	})
	if err != nil {
		t.Fatalf("heartbeat.Parse() error = %v", err)
	}
	snapshot, err := heartbeat.SnapshotFromResolved(
		id,
		workspaceID,
		agentName,
		&resolved,
		time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("SnapshotFromResolved() error = %v", err)
	}
	return snapshot
}

func heartbeatSnapshotForTest(id string, workspaceID string, agentName string, digest string) heartbeat.Snapshot {
	return heartbeat.Snapshot{
		ID:              id,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		SourcePath:      "agents/" + agentName + "/" + heartbeat.FileName,
		SchemaVersion:   1,
		Digest:          digest,
		ConfigDigest:    "sha256:config",
		Body:            "Check state before sending a wake prompt.",
		FrontmatterJSON: json.RawMessage(`{"version":1,"enabled":true}`),
		ResolvedJSON: json.RawMessage(
			`{"schema_version":1,"present":true,"active":true,"valid":true,"config_provenance":{"digest":"sha256:config"}}`,
		),
		DiagnosticsJSON: json.RawMessage(`[]`),
		CreatedAt:       time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	}
}

func heartbeatRevisionForTest(
	id string,
	workspaceID string,
	agentName string,
	operation heartbeat.RevisionOperation,
	previousDigest string,
	newDigest string,
) heartbeat.Revision {
	return heartbeat.Revision{
		ID:             id,
		WorkspaceID:    workspaceID,
		AgentName:      agentName,
		SourcePath:     "agents/" + agentName + "/" + heartbeat.FileName,
		Operation:      operation,
		PreviousDigest: previousDigest,
		NewDigest:      newDigest,
		Body:           "Check state before sending a wake prompt.",
		ActorKind:      heartbeat.ActorKindAgent,
		ActorID:        agentName,
		CreatedAt:      time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC),
	}
}

func heartbeatSessionHealthForTest(
	sessionID string,
	workspaceID string,
	agentName string,
	updatedAt time.Time,
) heartbeat.SessionHealth {
	return heartbeat.SessionHealth{
		SessionID:       sessionID,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		State:           heartbeat.SessionHealthStateIdle,
		Health:          heartbeat.SessionHealthHealthy,
		Attachable:      true,
		EligibleForWake: true,
		LastActivityAt:  updatedAt.Add(-2 * time.Minute),
		LastPresenceAt:  updatedAt.Add(-time.Minute),
		UpdatedAt:       updatedAt,
	}
}

func heartbeatWakeStateForTest(
	sessionID string,
	workspaceID string,
	agentName string,
	snapshotID string,
	updatedAt time.Time,
) heartbeat.WakeState {
	return heartbeat.WakeState{
		WorkspaceID:      workspaceID,
		AgentName:        agentName,
		SessionID:        sessionID,
		PolicySnapshotID: snapshotID,
		LastWakeAt:       updatedAt.Add(-time.Minute),
		NextAllowedAt:    updatedAt.Add(time.Hour),
		CoalescedCount:   1,
		LastResult:       heartbeat.WakeResultSent,
		LastReason:       heartbeat.WakeReasonSent,
		UpdatedAt:        updatedAt,
	}
}

func heartbeatWakeEventForTest(
	id string,
	sessionID string,
	workspaceID string,
	agentName string,
	snapshotID string,
	createdAt time.Time,
) heartbeat.WakeEvent {
	return heartbeat.WakeEvent{
		ID:                id,
		WorkspaceID:       workspaceID,
		AgentName:         agentName,
		SessionID:         sessionID,
		PolicySnapshotID:  snapshotID,
		Source:            heartbeat.WakeSourceScheduler,
		Result:            heartbeat.WakeResultSent,
		Reason:            heartbeat.WakeReasonSent,
		SyntheticPromptID: "prompt-" + id,
		CreatedAt:         createdAt,
		ExpiresAt:         createdAt.Add(24 * time.Hour),
	}
}

func heartbeatWakeEventIDs(events []heartbeat.WakeEvent) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}

func heartbeatRevisionIDs(revisions []heartbeat.Revision) []string {
	ids := make([]string, 0, len(revisions))
	for _, revision := range revisions {
		ids = append(ids, revision.ID)
	}
	return ids
}

func heartbeatWakeStateSessionIDs(states []heartbeat.WakeState) []string {
	ids := make([]string, 0, len(states))
	for _, state := range states {
		ids = append(ids, state.SessionID)
	}
	return ids
}

func assertHeartbeatStoreErrors(t *testing.T, globalDB *GlobalDB, workspaceID string) {
	t.Helper()

	ctx := testutil.Context(t)
	if _, err := globalDB.GetHeartbeatSnapshot(ctx, ""); !errors.Is(err, heartbeat.ErrInvalidSnapshot) {
		t.Fatalf("GetHeartbeatSnapshot(empty) error = %v, want ErrInvalidSnapshot", err)
	}
	if _, err := globalDB.GetHeartbeatSnapshot(ctx, "hb-missing"); !errors.Is(err, heartbeat.ErrSnapshotNotFound) {
		t.Fatalf("GetHeartbeatSnapshot(missing) error = %v, want ErrSnapshotNotFound", err)
	}
	if _, _, err := globalDB.FindHeartbeatSnapshotByDigest(
		ctx,
		"",
		"coder",
		"sha256:missing",
	); !errors.Is(err, heartbeat.ErrInvalidSnapshot) {
		t.Fatalf("FindHeartbeatSnapshotByDigest(invalid) error = %v, want ErrInvalidSnapshot", err)
	}
	if _, err := globalDB.GetLatestValidHeartbeatSnapshot(
		ctx,
		workspaceID,
		"missing-agent",
	); !errors.Is(err, heartbeat.ErrSnapshotNotFound) {
		t.Fatalf("GetLatestValidHeartbeatSnapshot(missing) error = %v, want ErrSnapshotNotFound", err)
	}
	if _, err := globalDB.ListHeartbeatSnapshots(ctx, heartbeat.SnapshotListQuery{Limit: -1}); !errors.Is(
		err,
		heartbeat.ErrInvalidSnapshot,
	) {
		t.Fatalf("ListHeartbeatSnapshots(invalid) error = %v, want ErrInvalidSnapshot", err)
	}
	if _, err := globalDB.GetHeartbeatRevision(ctx, ""); !errors.Is(err, heartbeat.ErrInvalidRevision) {
		t.Fatalf("GetHeartbeatRevision(empty) error = %v, want ErrInvalidRevision", err)
	}
	if _, err := globalDB.GetHeartbeatRevision(ctx, "hb-rev-missing"); !errors.Is(err, heartbeat.ErrRevisionNotFound) {
		t.Fatalf("GetHeartbeatRevision(missing) error = %v, want ErrRevisionNotFound", err)
	}
	if _, err := globalDB.ListHeartbeatRevisions(ctx, heartbeat.RevisionListQuery{Limit: -1}); !errors.Is(
		err,
		heartbeat.ErrInvalidRevision,
	) {
		t.Fatalf("ListHeartbeatRevisions(invalid) error = %v, want ErrInvalidRevision", err)
	}
	if _, err := globalDB.FindHeartbeatRevisionForRollback(ctx, heartbeat.RollbackLookup{}); !errors.Is(
		err,
		heartbeat.ErrInvalidRevision,
	) {
		t.Fatalf("FindHeartbeatRevisionForRollback(invalid) error = %v, want ErrInvalidRevision", err)
	}
	if _, err := globalDB.GetSessionHealth(ctx, ""); !errors.Is(err, heartbeat.ErrInvalidSessionHealth) {
		t.Fatalf("GetSessionHealth(empty) error = %v, want ErrInvalidSessionHealth", err)
	}
	if _, err := globalDB.GetSessionHealth(ctx, "sess-missing"); !errors.Is(err, heartbeat.ErrSessionHealthNotFound) {
		t.Fatalf("GetSessionHealth(missing) error = %v, want ErrSessionHealthNotFound", err)
	}
	if _, err := globalDB.ListSessionHealth(ctx, heartbeat.SessionHealthListQuery{Limit: -1}); !errors.Is(
		err,
		heartbeat.ErrInvalidSessionHealth,
	) {
		t.Fatalf("ListSessionHealth(invalid) error = %v, want ErrInvalidSessionHealth", err)
	}
	if _, err := globalDB.ListSessionHealthRecoveryInputs(ctx, -1); !errors.Is(
		err,
		heartbeat.ErrInvalidSessionHealth,
	) {
		t.Fatalf("ListSessionHealthRecoveryInputs(invalid) error = %v, want ErrInvalidSessionHealth", err)
	}
	if _, err := globalDB.MarkSessionHealthStale(ctx, time.Time{}, time.Time{}); !errors.Is(
		err,
		heartbeat.ErrInvalidSessionHealth,
	) {
		t.Fatalf("MarkSessionHealthStale(invalid) error = %v, want ErrInvalidSessionHealth", err)
	}
	if _, err := globalDB.GetHeartbeatWakeState(ctx, "", "coder", "sess"); !errors.Is(
		err,
		heartbeat.ErrInvalidWakeState,
	) {
		t.Fatalf("GetHeartbeatWakeState(invalid) error = %v, want ErrInvalidWakeState", err)
	}
	if _, err := globalDB.GetHeartbeatWakeState(
		ctx,
		workspaceID,
		"coder",
		"sess-missing",
	); !errors.Is(err, heartbeat.ErrWakeStateNotFound) {
		t.Fatalf("GetHeartbeatWakeState(missing) error = %v, want ErrWakeStateNotFound", err)
	}
	if _, err := globalDB.ListHeartbeatWakeState(ctx, heartbeat.WakeStateListQuery{Limit: -1}); !errors.Is(
		err,
		heartbeat.ErrInvalidWakeState,
	) {
		t.Fatalf("ListHeartbeatWakeState(invalid) error = %v, want ErrInvalidWakeState", err)
	}
	if _, err := globalDB.GetHeartbeatWakeEvent(ctx, ""); !errors.Is(err, heartbeat.ErrInvalidWakeEvent) {
		t.Fatalf("GetHeartbeatWakeEvent(empty) error = %v, want ErrInvalidWakeEvent", err)
	}
	if _, err := globalDB.GetHeartbeatWakeEvent(ctx, "hb-event-missing"); !errors.Is(
		err,
		heartbeat.ErrWakeEventNotFound,
	) {
		t.Fatalf("GetHeartbeatWakeEvent(missing) error = %v, want ErrWakeEventNotFound", err)
	}
	if _, err := globalDB.ListHeartbeatWakeEvents(ctx, heartbeat.WakeEventListQuery{Limit: -1}); !errors.Is(
		err,
		heartbeat.ErrInvalidWakeEvent,
	) {
		t.Fatalf("ListHeartbeatWakeEvents(invalid) error = %v, want ErrInvalidWakeEvent", err)
	}
	if _, err := globalDB.SweepHeartbeatWakeEvents(ctx, time.Time{}, 10); !errors.Is(
		err,
		heartbeat.ErrInvalidWakeEvent,
	) {
		t.Fatalf("SweepHeartbeatWakeEvents(empty cutoff) error = %v, want ErrInvalidWakeEvent", err)
	}
	if _, err := globalDB.SweepHeartbeatWakeEvents(ctx, time.Now().UTC(), 0); !errors.Is(
		err,
		heartbeat.ErrInvalidWakeEvent,
	) {
		t.Fatalf("SweepHeartbeatWakeEvents(invalid limit) error = %v, want ErrInvalidWakeEvent", err)
	}
}

func assertTableExcludesColumns(t *testing.T, db *sql.DB, table string, forbidden []string) {
	t.Helper()

	columns, err := tableColumns(testutil.Context(t), db, table)
	if err != nil {
		t.Fatalf("tableColumns(%q) error = %v", table, err)
	}
	for _, column := range forbidden {
		if _, ok := columns[column]; ok {
			t.Fatalf("table %q includes forbidden column %q in %#v", table, column, columns)
		}
	}
}

func baseContext(t *testing.T) context.Context {
	t.Helper()
	return testutil.Context(t)
}
