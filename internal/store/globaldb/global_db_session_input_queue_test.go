package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBSessionInputQueueGeneration(t *testing.T) {
	t.Run("Should fence stale queued input when generation advances", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)

		oldEntry, position, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-old",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "old queued input",
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput(old) error = %v", err)
		}
		if position != 1 {
			t.Fatalf("old queue position = %d, want 1", position)
		}

		generation, err := globalDB.AdvanceSessionInputGeneration(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("AdvanceSessionInputGeneration() error = %v", err)
		}
		if generation != 1 {
			t.Fatalf("generation = %d, want 1", generation)
		}
		canceled, err := globalDB.CancelPendingSessionInputs(ctx, sessionID, generation, now.Add(2*time.Second))
		if err != nil {
			t.Fatalf("CancelPendingSessionInputs() error = %v", err)
		}
		if canceled != 1 {
			t.Fatalf("canceled stale entries = %d, want 1", canceled)
		}

		newEntry, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-new",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "new queued input",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(3 * time.Second),
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput(new) error = %v", err)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(4*time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok {
			t.Fatal("ClaimNextSessionInput() ok = false, want true")
		}
		if claimed.ID != newEntry.ID {
			t.Fatalf("claimed entry = %q, want %q", claimed.ID, newEntry.ID)
		}
		stale, err := getSessionInputQueueEntry(ctx, globalDB.db, sessionID, oldEntry.ID)
		if err != nil {
			t.Fatalf("getSessionInputQueueEntry(old) error = %v", err)
		}
		if stale.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("stale status = %q, want %q", stale.Status, store.SessionInputQueueStatusCanceled)
		}
	})

	t.Run("Should consume the latest current-generation steer once", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 5, 21, 12, 5, 0, 0, time.UTC)

		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID:                "steer-old",
			SessionID:         sessionID,
			Text:              "old steer",
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		}); err != nil {
			t.Fatalf("StageSessionSteer(old) error = %v", err)
		}
		generation, err := globalDB.AdvanceSessionInputGeneration(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("AdvanceSessionInputGeneration() error = %v", err)
		}
		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID:                "steer-new",
			SessionID:         sessionID,
			Text:              "new steer",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("StageSessionSteer(new) error = %v", err)
		}

		consumed, ok, err := globalDB.ConsumeSessionSteer(ctx, sessionID, now.Add(3*time.Second))
		if err != nil {
			t.Fatalf("ConsumeSessionSteer(first) error = %v", err)
		}
		if !ok {
			t.Fatal("ConsumeSessionSteer(first) ok = false, want true")
		}
		if consumed.ID != "steer-new" || consumed.Status != store.SessionInputQueueStatusSent {
			t.Fatalf("consumed = %#v, want sent steer-new", consumed)
		}
		_, ok, err = globalDB.ConsumeSessionSteer(ctx, sessionID, now.Add(4*time.Second))
		if err != nil {
			t.Fatalf("ConsumeSessionSteer(second) error = %v", err)
		}
		if ok {
			t.Fatal("ConsumeSessionSteer(second) ok = true, want false after one-shot consume")
		}
	})

	t.Run("Should summarize only current generation pending input", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 5, 21, 12, 7, 0, 0, time.UTC)

		if _, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-old-summary",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "old queued input",
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		}); err != nil {
			t.Fatalf("EnqueueSessionInput(old) error = %v", err)
		}
		generation, err := globalDB.AdvanceSessionInputGeneration(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("AdvanceSessionInputGeneration() error = %v", err)
		}
		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID:                "steer-current-summary",
			SessionID:         sessionID,
			Text:              "current steer",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("StageSessionSteer(current) error = %v", err)
		}
		if _, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-current-summary",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "current queued input",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(3 * time.Second),
		}); err != nil {
			t.Fatalf("EnqueueSessionInput(current) error = %v", err)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(4*time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok || claimed.SessionGeneration != generation {
			t.Fatalf("ClaimNextSessionInput() = %#v/%v, want current generation %d", claimed, ok, generation)
		}

		summary, err := globalDB.SessionInputQueueSummary(ctx, sessionID)
		if err != nil {
			t.Fatalf("SessionInputQueueSummary() error = %v", err)
		}
		if summary.Generation != generation {
			t.Fatalf("summary.Generation = %d, want %d", summary.Generation, generation)
		}
		if summary.PendingActive != 2 || summary.PendingQueued != 1 ||
			summary.PendingSteer != 1 || summary.PendingLeased != 1 {
			t.Fatalf("SessionInputQueueSummary() = %#v, want active=2 queued=1 steer=1 leased=1", summary)
		}
	})
}

func TestGlobalDBSessionInputQueueCapacity(t *testing.T) {
	t.Run("Should reject queued input when pending entries reach the cap", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 5, 21, 12, 10, 0, 0, time.UTC)

		_, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-one",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "first input",
			SessionGeneration: 0,
			QueueCap:          1,
			Now:               now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput(first) error = %v", err)
		}
		_, _, err = globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-two",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "second input",
			SessionGeneration: 0,
			QueueCap:          1,
			Now:               now.Add(time.Second),
		})
		if !errors.Is(err, store.ErrSessionInputQueueFull) {
			t.Fatalf("EnqueueSessionInput(second) error = %v, want ErrSessionInputQueueFull", err)
		}
	})
}

func registerInputQueueSession(t *testing.T, globalDB *GlobalDB) string {
	t.Helper()

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"input-queue-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)
	sessionID := "sess-input-queue"
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ID:          sessionID,
		Name:        "Input Queue",
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: workspaceID,
		SessionType: defaultSessionType,
		State:       "active",
		CreatedAt:   time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	return sessionID
}
