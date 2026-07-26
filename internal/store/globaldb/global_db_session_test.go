package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/store"
	globalschema "github.com/compozy/agh/internal/store/globaldb/schema"
	"github.com/compozy/agh/internal/testutil"
)

func TestScanSessionInfoReadsStopFields(t *testing.T) {
	t.Parallel()

	t.Run("Should read stop fields and Soul provenance", func(t *testing.T) {
		t.Parallel()

		db := openScanSessionInfoDB(t)
		subprocessStartedAt := time.Date(2026, 4, 3, 12, 3, 0, 0, time.UTC)
		lastUpdateAt := time.Date(2026, 4, 3, 12, 4, 0, 0, time.UTC)
		liveSpec := participation.Spec{
			Version:         participation.SpecVersion,
			Mode:            participation.ModeLive,
			WorkspaceID:     "ws-1",
			ChannelStrategy: participation.StrategyNamed,
			ChannelID:       "builders",
			Source:          participation.SourceExplicitRequest,
			Bounds: participation.Bounds{
				MaxWakes:         1,
				MaxWakeWallTime:  "1m",
				MaxTotalWallTime: "5m",
				MaxInputTokens:   1024,
				MaxOutputTokens:  512,
				MaxWakeDepth:     1,
				CoalesceWindow:   "500ms",
			},
		}
		if err := participation.ValidateSpec(liveSpec); err != nil {
			t.Fatalf("ValidateSpec() error = %v", err)
		}
		liveJSON, err := json.Marshal(liveSpec)
		if err != nil {
			t.Fatalf("json.Marshal(live spec) error = %v", err)
		}
		row := db.QueryRowContext(context.Background(), `
		SELECT
			'sess-scan',
			'Demo',
			'coder',
			'claude',
			'ws-1',
			?,
			'live',
			'builders',
			'explicit_request',
			'user',
			NULL,
			NULL,
			0,
			NULL,
			NULL,
			false,
			'{}',
			'{}',
			'stopped',
			'acp-123',
			'timeout',
			'deadline exceeded',
			'process_exit',
			'redacted summary',
			'/tmp/crash.json',
			42,
			?,
			?,
			'stalled',
			'activity_timeout',
			'',
			'',
			NULL,
			0,
			'snap-scan',
			'sha256:scan',
			'sha256:parent',
			'env-scan',
			'local',
			'local',
			'instance-scan',
			'prepared',
			'{"local":true}',
			?,
			'sync failed',
			?,
			?`,
			string(liveJSON),
			formatTimestamp(subprocessStartedAt),
			formatTimestamp(lastUpdateAt),
			formatTimestamp(time.Date(2026, 4, 3, 12, 4, 30, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)),
		)

		info, err := scanSessionInfo(row)
		if err != nil {
			t.Fatalf("scanSessionInfo() error = %v", err)
		}
		if got, want := info.StopReason, store.StopTimeout; got != want {
			t.Fatalf("info.StopReason = %q, want %q", got, want)
		}
		if got, want := info.StopDetail, "deadline exceeded"; got != want {
			t.Fatalf("info.StopDetail = %q, want %q", got, want)
		}
		if info.Failure == nil {
			t.Fatal("info.Failure = nil, want failure")
		}
		if got, want := info.Failure.Kind, store.FailureProcess; got != want {
			t.Fatalf("info.Failure.Kind = %q, want %q", got, want)
		}
		if got, want := info.Failure.Summary, "redacted summary"; got != want {
			t.Fatalf("info.Failure.Summary = %q, want %q", got, want)
		}
		if got, want := info.Provider, "claude"; got != want {
			t.Fatalf("info.Provider = %q, want %q", got, want)
		}
		if got, want := info.NetworkSpecSnapshot(), liveSpec; got != want {
			t.Fatalf("info Network participation = %#v, want %#v", got, want)
		}
		if info.ACPSessionID == nil || *info.ACPSessionID != "acp-123" {
			t.Fatalf("info.ACPSessionID = %#v, want acp-123", info.ACPSessionID)
		}
		if got, want := info.SoulSnapshotID, "snap-scan"; got != want {
			t.Fatalf("info.SoulSnapshotID = %q, want %q", got, want)
		}
		if got, want := info.SoulDigest, "sha256:scan"; got != want {
			t.Fatalf("info.SoulDigest = %q, want %q", got, want)
		}
		if got, want := info.ParentSoulDigest, "sha256:parent"; got != want {
			t.Fatalf("info.ParentSoulDigest = %q, want %q", got, want)
		}
		if info.Sandbox == nil {
			t.Fatal("info.Sandbox = nil, want sandbox metadata")
		}
		if info.Liveness == nil {
			t.Fatal("info.Liveness = nil, want liveness metadata")
		}
		if got, want := info.Liveness.SubprocessPID, 42; got != want {
			t.Fatalf("info.Liveness.SubprocessPID = %d, want %d", got, want)
		}
		if info.Liveness.SubprocessStartedAt == nil || !info.Liveness.SubprocessStartedAt.Equal(subprocessStartedAt) {
			t.Fatalf(
				"info.Liveness.SubprocessStartedAt = %#v, want %s",
				info.Liveness.SubprocessStartedAt,
				subprocessStartedAt,
			)
		}
		if info.Liveness.LastUpdateAt == nil || !info.Liveness.LastUpdateAt.Equal(lastUpdateAt) {
			t.Fatalf("info.Liveness.LastUpdateAt = %#v, want %s", info.Liveness.LastUpdateAt, lastUpdateAt)
		}
		if got, want := info.Liveness.StallState, "stalled"; got != want {
			t.Fatalf("info.Liveness.StallState = %q, want %q", got, want)
		}
		if got, want := info.Liveness.StallReason, "activity_timeout"; got != want {
			t.Fatalf("info.Liveness.StallReason = %q, want %q", got, want)
		}
		if got, want := info.Sandbox.SandboxID, "env-scan"; got != want {
			t.Fatalf("info.Sandbox.SandboxID = %q, want %q", got, want)
		}
		if got, want := info.Sandbox.LastSyncError, "sync failed"; got != want {
			t.Fatalf("info.Sandbox.LastSyncError = %q, want %q", got, want)
		}
	})
}

func TestScanSessionInfoHandlesNullStopReason(t *testing.T) {
	t.Parallel()

	t.Run("Should handle null stop reason and empty Soul provenance", func(t *testing.T) {
		t.Parallel()

		db := openScanSessionInfoDB(t)
		row := db.QueryRowContext(context.Background(), `
		SELECT
			'sess-null',
			NULL,
			'coder',
			'',
			'ws-1',
			'{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
			'local',
			NULL,
			'built_in_local',
			'user',
			NULL,
			NULL,
			0,
			NULL,
			NULL,
			false,
			'{}',
			'{}',
			'active',
			NULL,
			NULL,
			NULL,
			NULL,
			'',
			'',
				0,
				NULL,
				NULL,
				'',
				'',
				'',
				'',
				NULL,
				0,
				NULL,
				'',
				'',
				'',
				'',
				'',
				'',
				'',
				'',
				NULL,
				'',
			?,
			?`,
			formatTimestamp(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)),
		)

		info, err := scanSessionInfo(row)
		if err != nil {
			t.Fatalf("scanSessionInfo() error = %v", err)
		}
		if info.StopReason != "" {
			t.Fatalf("info.StopReason = %q, want empty", info.StopReason)
		}
		if info.StopDetail != "" {
			t.Fatalf("info.StopDetail = %q, want empty", info.StopDetail)
		}
		if info.Provider != "" {
			t.Fatalf("info.Provider = %q, want empty", info.Provider)
		}
		if channel := info.NetworkSpecSnapshot().ChannelID; channel != "" {
			t.Fatalf("info Network channel = %q, want empty", channel)
		}
		if info.ACPSessionID != nil {
			t.Fatalf("info.ACPSessionID = %#v, want nil", info.ACPSessionID)
		}
		if info.SoulSnapshotID != "" || info.SoulDigest != "" || info.ParentSoulDigest != "" {
			t.Fatalf(
				"Soul provenance = %#v/%q/%q, want empty",
				info.SoulSnapshotID,
				info.SoulDigest,
				info.ParentSoulDigest,
			)
		}
	})
}

func TestScanSessionInfoRejectsInvalidSandboxLastSyncAt(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid sandbox last sync timestamps", func(t *testing.T) {
		t.Parallel()

		db := openScanSessionInfoDB(t)
		row := db.QueryRowContext(context.Background(), `
		SELECT
			'sess-invalid-last-sync',
			'Demo',
			'coder',
			'claude',
			'ws-1',
			'{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
			'local',
			NULL,
			'built_in_local',
			'user',
			NULL,
			NULL,
			0,
			NULL,
			NULL,
			false,
			'{}',
			'{}',
			'active',
			NULL,
			NULL,
			NULL,
			NULL,
			'',
			'',
				0,
				NULL,
				NULL,
				'',
				'',
				'',
				'',
				NULL,
				0,
				NULL,
				'',
				'',
				'env-invalid',
				'local',
			'local',
			'instance-invalid',
			'prepared',
			'{"local":true}',
			'not-a-timestamp',
			'',
			?,
			?`,
			formatTimestamp(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)),
		)

		_, err := scanSessionInfo(row)
		if err == nil {
			t.Fatal("scanSessionInfo() error = nil, want invalid sandbox_last_sync_at failure")
		}
		if got, want := err.Error(), `store: parse timestamp "not-a-timestamp"`; !strings.Contains(got, want) {
			t.Fatalf("scanSessionInfo() error = %v, want substring %q", err, want)
		}
	})
}

func TestScanSessionInfoRejectsStallStateWithoutReason(t *testing.T) {
	t.Parallel()

	t.Run("Should reject stall state without a reason", func(t *testing.T) {
		t.Parallel()

		db := openScanSessionInfoDB(t)
		row := db.QueryRowContext(context.Background(), `
		SELECT
			'sess-invalid-stall',
			'Demo',
			'coder',
			'claude',
			'ws-1',
			'{"version":"network-participation/v1","mode":"local","source":"built_in_local"}',
			'local',
			NULL,
			'built_in_local',
			'user',
			NULL,
			NULL,
			0,
			NULL,
			NULL,
			false,
			'{}',
			'{}',
			'active',
			NULL,
			NULL,
			NULL,
			NULL,
			'',
			'',
			42,
			?,
			?,
			'stalled',
			'',
			'',
			'',
			NULL,
			0,
			NULL,
			'',
			'',
			'',
			'local',
			'',
			'',
			'',
			'',
			NULL,
			'',
			?,
			?`,
			formatTimestamp(time.Date(2026, 4, 3, 12, 3, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 4, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)),
			formatTimestamp(time.Date(2026, 4, 3, 12, 5, 0, 0, time.UTC)),
		)

		_, err := scanSessionInfo(row)
		if err == nil {
			t.Fatal("scanSessionInfo() error = nil, want invalid stall reason failure")
		}
		if got, want := err.Error(), "store: session stall reason required when stall state is set"; !strings.Contains(
			got,
			want,
		) {
			t.Fatalf("scanSessionInfo() error = %v, want substring %q", err, want)
		}
	})
}

func TestGlobalDBAttachSessionRejectsStalledSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should exclude stalled sessions from resumable attach paths", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"stalled-attach-workspace",
			filepath.Join(t.TempDir(), "stalled-attach-workspace"),
		)
		now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
		if err := globalDB.RegisterSession(ctx, store.SessionInfo{
			ID:          "sess-stalled-attach",
			Name:        "Stalled Attach",
			AgentName:   "coder",
			Provider:    "claude",
			WorkspaceID: workspaceID,
			SessionType: defaultSessionType,
			State:       globalDBSessionStateActive,
			Liveness: &store.SessionLivenessMeta{
				SubprocessPID: 77,
				StallState:    store.SessionStallStateDetected,
				StallReason:   store.SessionStallReasonActivityTimeout,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		resumable, err := globalDB.ListSessions(ctx, store.SessionListQuery{Resumable: true})
		if err != nil {
			t.Fatalf("ListSessions(resumable) error = %v", err)
		}
		if len(resumable) != 0 {
			t.Fatalf("resumable sessions = %#v, want none for stalled session", resumable)
		}
		_, err = globalDB.AttachSession(ctx, store.SessionAttachRequest{
			SessionID:  "sess-stalled-attach",
			AttachedTo: "operator",
			Now:        now,
			TTL:        time.Minute,
		})
		if !errors.Is(err, store.ErrSessionNotAttachable) {
			t.Fatalf("AttachSession(stalled) error = %v, want ErrSessionNotAttachable", err)
		}
	})
}

func TestGlobalDBRegisterSessionPreservesTranscriptEpoch(t *testing.T) {
	t.Parallel()

	t.Run("Should not reset a persisted transcript epoch from a reconstructed session row", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"transcript-epoch-workspace",
			filepath.Join(t.TempDir(), "transcript-epoch-workspace"),
		)
		now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
		session := store.SessionInfo{
			ID:              "sess-transcript-epoch",
			Name:            "Transcript Epoch",
			AgentName:       "coder",
			Provider:        "claude",
			WorkspaceID:     workspaceID,
			SessionType:     defaultSessionType,
			State:           globalDBSessionStateActive,
			TranscriptEpoch: 3,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := globalDB.RegisterSession(ctx, session); err != nil {
			t.Fatalf("RegisterSession(initial) error = %v", err)
		}

		session.TranscriptEpoch = 0
		session.State = globalDBSessionStateStopped
		session.UpdatedAt = now.Add(time.Minute)
		if err := globalDB.RegisterSession(ctx, session); err != nil {
			t.Fatalf("RegisterSession(reconstructed) error = %v", err)
		}

		epoch, err := globalDB.SessionTranscriptEpoch(ctx, session.ID)
		if err != nil {
			t.Fatalf("SessionTranscriptEpoch() error = %v", err)
		}
		if epoch != 3 {
			t.Fatalf("SessionTranscriptEpoch() = %d, want 3", epoch)
		}

		sessions, err := globalDB.ListSessions(ctx, store.SessionListQuery{ID: session.ID})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("len(sessions) = %d, want 1", len(sessions))
		}
		if sessions[0].TranscriptEpoch != 3 {
			t.Fatalf("sessions[0].TranscriptEpoch = %d, want 3", sessions[0].TranscriptEpoch)
		}
	})
}

func TestGlobalDBRegisterSessionRejectsParticipationSnapshotChange(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	globalDB := openTestGlobalDB(t)
	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"immutable-participation-workspace",
		filepath.Join(t.TempDir(), "immutable-participation-workspace"),
	)
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	session := store.SessionInfo{
		ID:          "sess-immutable-participation",
		Name:        "Immutable Participation",
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		SessionType: defaultSessionType,
		State:       globalDBSessionStateActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	session.SetNetworkSpec(participation.LocalSpec())
	if err := globalDB.RegisterSession(ctx, session); err != nil {
		t.Fatalf("RegisterSession(Local) error = %v", err)
	}

	session.SetNetworkSpec(participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       "immutable-builders",
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
	})
	session.State = globalDBSessionStateStopped
	session.UpdatedAt = now.Add(time.Minute)
	err := globalDB.RegisterSession(ctx, session)
	if !errors.Is(err, store.ErrSessionParticipationMismatch) {
		t.Fatalf("RegisterSession(Live refresh) error = %v, want participation mismatch", err)
	}

	stored, err := globalDB.ListSessions(ctx, store.SessionListQuery{ID: session.ID})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("len(stored) = %d, want 1", len(stored))
	}
	if got, want := stored[0].NetworkSpecSnapshot(), participation.LocalSpec(); got != want {
		t.Fatalf("stored participation = %#v, want %#v", got, want)
	}
	if got, want := stored[0].State, globalDBSessionStateActive; got != want {
		t.Fatalf("stored state = %q, want unchanged %q", got, want)
	}
}

func TestGlobalDBDeleteSession(t *testing.T) {
	t.Parallel()

	t.Run("Should remove only the target session and its dependent rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		sessionID := "sess-delete"
		survivorID := "sess-delete-survivor"
		registerSessionForGlobalTests(t, globalDB, sessionID)
		registerSessionForGlobalTests(t, globalDB, survivorID)
		writeSessionDeleteDependents(t, globalDB, sessionID)
		writeSessionDeleteDependents(t, globalDB, survivorID)

		if err := globalDB.DeleteSession(testutil.Context(t), sessionID); err != nil {
			t.Fatalf("DeleteSession() error = %v", err)
		}

		assertSessionDeleteRowCounts(t, globalDB, sessionID, 0, 0, 0)
		assertSessionDeleteRowCounts(t, globalDB, survivorID, 1, 1, 1)
	})

	t.Run("Should return session not found when the catalog row is absent", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		err := globalDB.DeleteSession(testutil.Context(t), "sess-missing-delete")
		if !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("DeleteSession(missing) error = %v, want ErrSessionNotFound", err)
		}
	})

	t.Run("Should roll back dependent deletes when the session row cannot be deleted", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		sessionID := "sess-delete-rollback"
		registerSessionForGlobalTests(t, globalDB, sessionID)
		writeSessionDeleteDependents(t, globalDB, sessionID)
		if _, err := globalDB.db.ExecContext(testutil.Context(t), `
			CREATE TRIGGER prevent_session_delete
			BEFORE DELETE ON sessions
			WHEN OLD.id = 'sess-delete-rollback'
			BEGIN
				SELECT RAISE(ABORT, 'forced session delete failure');
			END`); err != nil {
			t.Fatalf("create delete failure trigger error = %v", err)
		}

		err := globalDB.DeleteSession(testutil.Context(t), sessionID)
		if err == nil || !strings.Contains(err.Error(), "forced session delete failure") {
			t.Fatalf("DeleteSession() error = %v, want forced delete failure", err)
		}

		assertSessionDeleteRowCounts(t, globalDB, sessionID, 1, 1, 1)
	})
}

// TestGlobalDBSessionCascadeMigration is serial because its race-instrumented
// v2-to-head replay is the package's machine-sized historical fixture.
func TestGlobalDBSessionCascadeMigration(t *testing.T) {
	t.Run("Should upgrade the immutable bridge prefix before cascading session history", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, sessionCascadeMigrationPrefix(t))
		if err != nil {
			t.Fatalf("OpenSQLiteDatabase(v2 prefix) error = %v", err)
		}
		prefixGlobalDB := &GlobalDB{
			db:   prefixDB,
			path: path,
			now: func() time.Time {
				return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
			},
		}
		prefixGlobalDB.initializeRepositories(openConfig{})
		targetID := "sess-v2-upgrade-target"
		survivorID := "sess-v2-upgrade-survivor"
		seedSessionDeletePrefixRows(t, prefixGlobalDB, targetID)
		seedSessionDeletePrefixRows(t, prefixGlobalDB, survivorID)
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}

		globalDB, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(full-stream upgrade) error = %v", err)
		}
		ctx := testutil.Context(t)
		t.Cleanup(func() {
			if err := globalDB.Close(testutil.Context(t)); err != nil {
				t.Errorf("Close(upgraded global DB) error = %v", err)
			}
		})

		assertSessionDeleteRowCounts(t, globalDB, targetID, 1, 1, 1)
		assertSessionDeleteRowCounts(t, globalDB, survivorID, 1, 1, 1)
		assertSessionDeleteForeignKeysCascade(t, globalDB.db)
		status, err := store.Status(ctx, globalDB.db, MigrationStream())
		if err != nil {
			t.Fatalf("Status(upgraded global DB) error = %v", err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())

		if err := globalDB.DeleteSession(ctx, targetID); err != nil {
			t.Fatalf("DeleteSession(upgraded target) error = %v", err)
		}
		assertSessionDeleteRowCounts(t, globalDB, targetID, 0, 0, 0)
		assertSessionDeleteRowCounts(t, globalDB, survivorID, 1, 1, 1)
	})
}

func TestGlobalDBEnsureSessionTranscriptEpoch(t *testing.T) {
	t.Parallel()

	t.Run("Should raise below-minimum epoch and return current epoch when already high enough", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"ensure-transcript-epoch-workspace",
			filepath.Join(t.TempDir(), "ensure-transcript-epoch-workspace"),
		)
		now := time.Date(2026, 7, 7, 12, 30, 0, 0, time.UTC)
		session := store.SessionInfo{
			ID:              "sess-ensure-transcript-epoch",
			Name:            "Ensure Transcript Epoch",
			AgentName:       "coder",
			Provider:        "claude",
			WorkspaceID:     workspaceID,
			SessionType:     defaultSessionType,
			State:           globalDBSessionStateActive,
			TranscriptEpoch: 1,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := globalDB.RegisterSession(ctx, session); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		epoch, err := globalDB.EnsureSessionTranscriptEpoch(ctx, store.SessionTranscriptEpochUpdate{
			SessionID: session.ID,
			Minimum:   5,
		})
		if err != nil {
			t.Fatalf("EnsureSessionTranscriptEpoch(raise) error = %v", err)
		}
		if epoch != 5 {
			t.Fatalf("EnsureSessionTranscriptEpoch(raise) = %d, want 5", epoch)
		}

		epoch, err = globalDB.EnsureSessionTranscriptEpoch(ctx, store.SessionTranscriptEpochUpdate{
			SessionID: session.ID,
			Minimum:   3,
		})
		if err != nil {
			t.Fatalf("EnsureSessionTranscriptEpoch(already high) error = %v", err)
		}
		if epoch != 5 {
			t.Fatalf("EnsureSessionTranscriptEpoch(already high) = %d, want 5", epoch)
		}
	})

	t.Run("Should return not found when no session can be raised or read", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		_, err := globalDB.EnsureSessionTranscriptEpoch(testutil.Context(t), store.SessionTranscriptEpochUpdate{
			SessionID: "sess-missing-transcript-epoch",
			Minimum:   2,
		})
		if !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf("EnsureSessionTranscriptEpoch(missing) error = %v, want ErrSessionNotFound", err)
		}
	})
}

func TestGlobalDBListSessionsSweepsExpiredAttachLocks(t *testing.T) {
	t.Parallel()

	t.Run("Should clear expired attach holder fields before returning sessions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			globalDB,
			"expired-attach-workspace",
			filepath.Join(t.TempDir(), "expired-attach-workspace"),
		)
		now := time.Date(2026, 5, 22, 11, 0, 0, 0, time.UTC)
		expiredAt := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := globalDB.RegisterSession(ctx, store.SessionInfo{
			ID:              "sess-expired-attach",
			Name:            "Expired Attach",
			AgentName:       "coder",
			Provider:        "claude",
			WorkspaceID:     workspaceID,
			SessionType:     defaultSessionType,
			State:           globalDBSessionStateActive,
			AttachedTo:      "operator-old",
			AttachExpiresAt: &expiredAt,
			CreatedAt:       now.Add(-time.Hour),
			UpdatedAt:       now.Add(-time.Hour),
		}); err != nil {
			t.Fatalf("RegisterSession() error = %v", err)
		}

		sessions, err := globalDB.ListSessions(ctx, store.SessionListQuery{ID: "sess-expired-attach"})
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}
		if len(sessions) != 1 {
			t.Fatalf("len(sessions) = %d, want 1", len(sessions))
		}
		if sessions[0].AttachedTo != "" || sessions[0].AttachExpiresAt != nil {
			t.Fatalf(
				"session attach fields = %#v/%#v, want cleared",
				sessions[0].AttachedTo,
				sessions[0].AttachExpiresAt,
			)
		}
		cleared, err := globalDB.SweepExpiredSessionAttachLocks(ctx, now)
		if err != nil {
			t.Fatalf("SweepExpiredSessionAttachLocks(after list) error = %v", err)
		}
		if cleared != 0 {
			t.Fatalf("SweepExpiredSessionAttachLocks(after list) = %d, want 0", cleared)
		}
	})
}

func openScanSessionInfoDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open(sqliteDriverName, sqliteDSN(t.TempDir()+"/scan.db"))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("Close(scan db) error = %v", err)
		}
	})
	return db
}

func writeSessionDeleteDependents(t *testing.T, globalDB *GlobalDB, sessionID string) {
	t.Helper()

	inputTokens := int64(7)
	if err := globalDB.UpdateTokenStats(testutil.Context(t), store.TokenStatsUpdate{
		SessionID:   sessionID,
		AgentName:   "coder",
		InputTokens: &inputTokens,
		CostStatus:  "unknown",
		CostSource:  "none",
		Turns:       1,
	}); err != nil {
		t.Fatalf("UpdateTokenStats() error = %v", err)
	}
	writeSessionDeletePermissionLog(t, globalDB, sessionID)
}

func writeSessionDeletePermissionLog(t *testing.T, globalDB *GlobalDB, sessionID string) {
	t.Helper()

	if err := globalDB.WritePermissionLog(testutil.Context(t), store.PermissionLogEntry{
		ID:         "perm-" + sessionID,
		SessionID:  sessionID,
		AgentName:  "coder",
		Action:     "fs/read",
		Resource:   "README.md",
		Decision:   "allow",
		PolicyUsed: "approve-reads",
		Timestamp:  time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WritePermissionLog() error = %v", err)
	}
}

func seedSessionDeletePrefixRows(t *testing.T, globalDB *GlobalDB, sessionID string) {
	t.Helper()

	ctx := testutil.Context(t)
	now := store.FormatTimestamp(time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC))
	workspaceID := sessionID + "-workspace"
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO workspaces (id, root_dir, name, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		workspaceID,
		filepath.Join(t.TempDir(), sessionID),
		workspaceID,
		now,
		now,
	); err != nil {
		t.Fatalf("seed v2 workspace %q: %v", workspaceID, err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO sessions (
			id, agent_name, provider, workspace_id, state, created_at, updated_at
		 ) VALUES (?, 'coder', 'claude', ?, 'active', ?, ?)`,
		sessionID,
		workspaceID,
		now,
		now,
	); err != nil {
		t.Fatalf("seed v2 session %q: %v", sessionID, err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO token_stats (
			id, session_id, agent_name, input_tokens, turn_count, updated_at
		 ) VALUES (?, ?, 'coder', 7, 1, ?)`,
		"token-"+sessionID,
		sessionID,
		now,
	); err != nil {
		t.Fatalf("seed v2 token stats for %q: %v", sessionID, err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO permission_log (
			id, session_id, agent_name, action, resource, decision, policy_used, timestamp
		 ) VALUES (?, ?, 'coder', 'fs/read', 'README.md', 'allow', 'approve-reads', ?)`,
		"perm-"+sessionID,
		sessionID,
		now,
	); err != nil {
		t.Fatalf("seed v2 permission log for %q: %v", sessionID, err)
	}
}

func assertSessionDeleteRowCounts(
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	wantSessions int,
	wantTokenStats int,
	wantPermissionLogs int,
) {
	t.Helper()

	sessions, err := globalDB.ListSessions(testutil.Context(t), store.SessionListQuery{ID: sessionID})
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != wantSessions {
		t.Fatalf("len(sessions) = %d, want %d", len(sessions), wantSessions)
	}
	stats, err := globalDB.ListTokenStats(testutil.Context(t), store.TokenStatsQuery{SessionID: sessionID})
	if err != nil {
		t.Fatalf("ListTokenStats() error = %v", err)
	}
	if len(stats) != wantTokenStats {
		t.Fatalf("len(token stats) = %d, want %d", len(stats), wantTokenStats)
	}
	entries, err := globalDB.ListPermissionLog(
		testutil.Context(t),
		store.PermissionLogQuery{SessionID: sessionID},
	)
	if err != nil {
		t.Fatalf("ListPermissionLog() error = %v", err)
	}
	if len(entries) != wantPermissionLogs {
		t.Fatalf("len(permission logs) = %d, want %d", len(entries), wantPermissionLogs)
	}
}

func sessionCascadeMigrationPrefix(t *testing.T) store.MigrationStream {
	t.Helper()

	const v2AtlasSum = "h1:/3U/DcBvQv8MKlUUznXHiKwlNS7ZWCwZR0CNPIs6UHw=\n" +
		"00001_baseline.sql h1:S/Hl9w8PyqMpsW5NP7g/7u/VnQI84pVFJZUUBi0MMy0=\n" +
		"00002_schema.sql h1:PBK6FCjEVDmfxcoY4whqvHExe4llHojr9bZjNymgtHw=\n"
	files := fstest.MapFS{
		"atlas.sum": &fstest.MapFile{Data: []byte(v2AtlasSum)},
	}
	for _, name := range []string{"00001_baseline.sql", "00002_schema.sql"} {
		data, err := fs.ReadFile(globalschema.Files, "migrations/"+name)
		if err != nil {
			t.Fatalf("read global migration prefix %q: %v", name, err)
		}
		files[name] = &fstest.MapFile{Data: data}
	}
	stream := MigrationStream()
	stream.FS = files
	stream.Dir = "."
	return stream
}

func assertSessionDeleteForeignKeysCascade(t *testing.T, db *sql.DB) {
	t.Helper()

	var foreignKeysEnabled int
	if err := db.QueryRowContext(testutil.Context(t), `PRAGMA foreign_keys`).Scan(&foreignKeysEnabled); err != nil {
		t.Fatalf("query foreign_keys error = %v", err)
	}
	if foreignKeysEnabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeysEnabled)
	}
	for _, table := range []string{"permission_log", "token_stats"} {
		var onDelete string
		if err := db.QueryRowContext(
			testutil.Context(t),
			`SELECT on_delete
			 FROM pragma_foreign_key_list(?)
			 WHERE "table" = 'sessions' AND "from" = 'session_id'`,
			table,
		).Scan(&onDelete); err != nil {
			t.Fatalf("query %s session foreign key error = %v", table, err)
		}
		if onDelete != "CASCADE" {
			t.Fatalf("%s session foreign key on_delete = %q, want CASCADE", table, onDelete)
		}
	}
}
