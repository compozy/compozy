//go:build integration

package globaldb

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/goal"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGoalSessionOutboxIntegration(t *testing.T) {
	t.Run("Should retain orphan projections without starving deliverable sessions", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		globalDB := openLoopTestGlobalDB(t, "ws-goal-outbox")
		now := time.Date(2026, 7, 10, 19, 0, 0, 0, time.UTC)
		for _, id := range []string{"session-deleted", "session-stopped", "session-bound"} {
			info := goalSessionInfoForTest(id, "ws-goal-outbox", now)
			info.State = "stopped"
			if err := globalDB.RegisterSession(ctx, &(info)); err != nil {
				t.Fatalf("RegisterSession(%q) error = %v", id, err)
			}
		}
		for _, origin := range []string{"session-deleted", "session-stopped"} {
			runID := "run-" + origin
			insertGoalSchemaLoopRun(t, globalDB, runID, "ws-goal-outbox", "session", &origin)
			if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, goal.EnqueueSessionOutboxRequest{
				EventID: "event-" + origin, WorkspaceID: "ws-goal-outbox", OriginSessionID: origin,
				LoopRunID: looppkg.RunID(runID), BoundSessionID: new("session-bound"),
				Cause: goal.SessionOutboxCauseStatus, CreatedAt: now,
			}); err != nil {
				t.Fatalf("EnqueueGoalSessionOutbox(%q) error = %v", origin, err)
			}
		}
		for _, id := range []string{"session-deleted", "session-bound"} {
			if err := globalDB.DeleteSession(ctx, id); err != nil {
				t.Fatalf("DeleteSession(%q) error = %v", id, err)
			}
		}
		path := globalDB.Path()
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		globalDB = openGlobalDBForTest(t, path)

		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 1)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(after reopen) error = %v", err)
		}
		if len(pending) != 1 || pending[0].EventID != "event-session-stopped" {
			t.Fatalf("pending = %#v, want stopped origin despite deleted bound session", pending)
		}
		if err := globalDB.AcknowledgeGoalSessionOutbox(ctx, pending[0].EventID, now); err != nil {
			t.Fatalf("AcknowledgeGoalSessionOutbox() error = %v", err)
		}
		pending, err = globalDB.ClaimGoalSessionOutbox(ctx, 1)
		if err != nil || len(pending) != 0 {
			t.Fatalf("ClaimGoalSessionOutbox(orphan only) = %#v, %v, want empty", pending, err)
		}
		retained, err := loadGoalSessionOutboxByEventID(ctx, globalDB.db, "event-session-deleted")
		if err != nil {
			t.Fatalf("loadGoalSessionOutboxByEventID(deleted origin) error = %v", err)
		}
		if retained.DeliveredAt != nil {
			t.Fatalf("orphan delivered_at = %v, want undelivered", retained.DeliveredAt)
		}
		if err := globalDB.RegisterSession(
			ctx,
			new(goalSessionInfoForTest("session-deleted", "ws-goal-outbox", now)),
		); err != nil {
			t.Fatalf("RegisterSession(restored origin) error = %v", err)
		}
		pending, err = globalDB.ClaimGoalSessionOutbox(ctx, 1)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(restored origin) error = %v", err)
		}
		if len(pending) != 1 || pending[0].EventID != retained.EventID || pending[0].ID != retained.ID {
			t.Fatalf("pending = %#v, want retained event %#v after catalog restoration", pending, retained)
		}
	})

	t.Run("Should claim retry and acknowledge deterministic events exactly once", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-outbox")
		now := time.Date(2026, 7, 10, 19, 0, 0, 0, time.UTC)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-origin", "ws-goal-outbox", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-origin",
				PolicySpecDigest:   "policy-origin",
				CreationDigest:     "creation-origin",
			},
		)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-bound", "ws-goal-outbox", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-bound",
				PolicySpecDigest:   "policy-bound",
				CreationDigest:     "creation-bound",
			},
		)
		originSessionID := "session-origin"
		boundSessionID := "session-bound"
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-goal-outbox",
			"ws-goal-outbox",
			"session",
			&originSessionID,
		)

		ctx := testutil.Context(t)
		firstRequest := goal.EnqueueSessionOutboxRequest{
			EventID:         "goal-snapshot:session-origin:run-goal-outbox:start:1",
			WorkspaceID:     "ws-goal-outbox",
			OriginSessionID: "session-origin",
			LoopRunID:       "run-goal-outbox",
			BoundSessionID:  &boundSessionID,
			Cause:           goal.SessionOutboxCauseStart,
			CreatedAt:       now,
		}
		first, err := globalDB.EnqueueGoalSessionOutbox(ctx, firstRequest)
		if err != nil {
			t.Fatalf("EnqueueGoalSessionOutbox(first) error = %v", err)
		}
		if first.ID < 1 || first.DeliveredAt != nil {
			t.Fatalf("first event = %#v, want positive ID and pending delivery", first)
		}
		repeated, err := globalDB.EnqueueGoalSessionOutbox(ctx, firstRequest)
		if err != nil {
			t.Fatalf("EnqueueGoalSessionOutbox(repeated) error = %v", err)
		}
		if repeated.ID != first.ID || repeated.EventID != first.EventID ||
			repeated.BoundSessionID == nil || first.BoundSessionID == nil ||
			*repeated.BoundSessionID != *first.BoundSessionID ||
			!repeated.CreatedAt.Equal(first.CreatedAt) {
			t.Fatalf("repeated event = %#v, want %#v", repeated, first)
		}

		collision := firstRequest
		collision.Cause = goal.SessionOutboxCauseStatus
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, collision); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("EnqueueGoalSessionOutbox(collision) error = %v, want %v", err, looppkg.ErrTransitionConflict)
		}

		secondRequest := firstRequest
		secondRequest.EventID = "goal-snapshot:session-origin:run-goal-outbox:status:2"
		secondRequest.Cause = goal.SessionOutboxCauseStatus
		secondRequest.CreatedAt = now.Add(time.Minute)
		second, err := globalDB.EnqueueGoalSessionOutbox(ctx, secondRequest)
		if err != nil {
			t.Fatalf("EnqueueGoalSessionOutbox(second) error = %v", err)
		}

		claimed, err := globalDB.ClaimGoalSessionOutbox(ctx, 1)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(first) error = %v", err)
		}
		if len(claimed) != 1 || claimed[0].EventID != first.EventID {
			t.Fatalf("first claim = %#v, want only %q", claimed, first.EventID)
		}
		retried, err := globalDB.ClaimGoalSessionOutbox(ctx, 1)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(retry) error = %v", err)
		}
		if len(retried) != 1 || retried[0].EventID != first.EventID {
			t.Fatalf("retry claim = %#v, want only %q", retried, first.EventID)
		}

		deliveredAt := now.Add(2 * time.Minute)
		if err := globalDB.AcknowledgeGoalSessionOutbox(ctx, first.EventID, deliveredAt); err != nil {
			t.Fatalf("AcknowledgeGoalSessionOutbox(first) error = %v", err)
		}
		if err := globalDB.AcknowledgeGoalSessionOutbox(ctx, first.EventID, deliveredAt.Add(time.Minute)); err != nil {
			t.Fatalf("AcknowledgeGoalSessionOutbox(repeated) error = %v", err)
		}

		pending, err := globalDB.ClaimGoalSessionOutbox(ctx, 10)
		if err != nil {
			t.Fatalf("ClaimGoalSessionOutbox(after ack) error = %v", err)
		}
		if len(pending) != 1 || pending[0].EventID != second.EventID {
			t.Fatalf("pending after ack = %#v, want only %q", pending, second.EventID)
		}

		if err := globalDB.AcknowledgeGoalSessionOutbox(
			ctx,
			"missing-event",
			deliveredAt,
		); !errors.Is(
			err,
			goal.ErrOutboxEventNotFound,
		) {
			t.Fatalf("AcknowledgeGoalSessionOutbox(missing) error = %v, want %v", err, goal.ErrOutboxEventNotFound)
		}
	})

	t.Run("Should preserve a null bound session for clear projections", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-outbox-clear")
		now := time.Date(2026, 7, 10, 19, 30, 0, 0, time.UTC)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-origin-clear", "ws-goal-outbox-clear", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-origin-clear",
				PolicySpecDigest:   "policy-origin-clear",
				CreationDigest:     "creation-origin-clear",
			},
		)
		originSessionID := "session-origin-clear"
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-goal-outbox-clear",
			"ws-goal-outbox-clear",
			"session",
			&originSessionID,
		)

		event, err := globalDB.EnqueueGoalSessionOutbox(testutil.Context(t), goal.EnqueueSessionOutboxRequest{
			EventID:         "goal-snapshot:session-origin-clear:run-goal-outbox-clear:clear:2",
			WorkspaceID:     "ws-goal-outbox-clear",
			OriginSessionID: originSessionID,
			LoopRunID:       "run-goal-outbox-clear",
			Cause:           goal.SessionOutboxCauseClear,
			CreatedAt:       now,
		})
		if err != nil {
			t.Fatalf("EnqueueGoalSessionOutbox(clear) error = %v", err)
		}
		if event.BoundSessionID != nil {
			t.Fatalf("clear bound session = %v, want nil", event.BoundSessionID)
		}
	})

	t.Run("Should reject cross workspace or mismatched session projections without a write", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-outbox", "ws-foreign")
		now := time.Date(2026, 7, 10, 20, 0, 0, 0, time.UTC)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-origin", "ws-goal-outbox", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-origin",
				PolicySpecDigest:   "policy-origin",
				CreationDigest:     "creation-origin",
			},
		)
		originSessionID := "session-origin"
		insertGoalSchemaLoopRun(
			t,
			globalDB,
			"run-goal-outbox-rejected",
			"ws-goal-outbox",
			"session",
			&originSessionID,
		)

		ctx := testutil.Context(t)
		boundSessionID := "session-origin"
		request := goal.EnqueueSessionOutboxRequest{
			EventID:         "goal-snapshot:session-origin:run-goal-outbox-rejected:start:1",
			WorkspaceID:     "ws-foreign",
			OriginSessionID: "session-origin",
			LoopRunID:       "run-goal-outbox-rejected",
			BoundSessionID:  &boundSessionID,
			Cause:           goal.SessionOutboxCauseStart,
			CreatedAt:       now,
		}
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, request); !errors.Is(err, looppkg.ErrRunNotFound) {
			t.Fatalf("EnqueueGoalSessionOutbox(foreign workspace) error = %v, want %v", err, looppkg.ErrRunNotFound)
		}

		request.WorkspaceID = "ws-goal-outbox"
		request.OriginSessionID = "session-other"
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, request); err == nil {
			t.Fatal("EnqueueGoalSessionOutbox(mismatched origin) error = nil")
		}

		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-foreign-bound", "ws-foreign", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-foreign",
				PolicySpecDigest:   "policy-foreign",
				CreationDigest:     "creation-foreign",
			},
		)
		foreignBoundSessionID := "session-foreign-bound"
		request.OriginSessionID = originSessionID
		request.BoundSessionID = &foreignBoundSessionID
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, request); !errors.Is(err, store.ErrSessionNotFound) {
			t.Fatalf(
				"EnqueueGoalSessionOutbox(foreign bound session) error = %v, want %v",
				err,
				store.ErrSessionNotFound,
			)
		}

		var count int
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM loop_goal_session_outbox WHERE loop_run_id = ?`,
			"run-goal-outbox-rejected",
		).Scan(&count); err != nil {
			t.Fatalf("count rejected outbox rows error = %v", err)
		}
		if count != 0 {
			t.Fatalf("rejected outbox rows = %d, want 0", count)
		}
	})
	t.Run("Should signal the Goal relay only after an enqueue commits", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-goal-signal")
		now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
		registerGoalSessionIdentityForTest(
			t,
			globalDB,
			new(goalSessionInfoForTest("session-signal", "ws-goal-signal", now)),
			store.SessionCreationIdentity{
				CreationProfileRef: "profile-signal",
				PolicySpecDigest:   "policy-signal",
				CreationDigest:     "creation-signal",
			},
		)
		originSessionID := "session-signal"
		insertGoalSchemaLoopRun(t, globalDB, "run-goal-signal", "ws-goal-signal", "session", &originSessionID)
		var goalSignals, effectSignals atomic.Int32
		globalDB.SetGoalSessionOutboxCommitSignal(func() { goalSignals.Add(1) })
		globalDB.SetLoopEffectCommitSignal(func() { effectSignals.Add(1) })

		ctx := testutil.Context(t)
		request := goal.EnqueueSessionOutboxRequest{
			EventID:     "goal-snapshot:session-signal:run-goal-signal:clear:1",
			WorkspaceID: "ws-goal-signal", OriginSessionID: "session-signal",
			LoopRunID: "run-goal-signal", Cause: goal.SessionOutboxCauseClear, CreatedAt: now,
		}
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, request); err != nil {
			t.Fatalf("EnqueueGoalSessionOutbox() error = %v", err)
		}
		if got := goalSignals.Load(); got != 1 {
			t.Fatalf("Goal commit signals = %d, want 1", got)
		}
		collision := request
		collision.CreatedAt = now.Add(time.Minute)
		if _, err := globalDB.EnqueueGoalSessionOutbox(ctx, collision); !errors.Is(err, looppkg.ErrTransitionConflict) {
			t.Fatalf("EnqueueGoalSessionOutbox(collision) error = %v, want %v", err, looppkg.ErrTransitionConflict)
		}
		if got := goalSignals.Load(); got != 1 {
			t.Fatalf("Goal commit signals after rollback = %d, want no signal for a rolled-back enqueue", got)
		}
		if got := effectSignals.Load(); got != 0 {
			t.Fatalf("effect commit signals = %d, want 0 for a Goal-only transaction", got)
		}
	})
}
