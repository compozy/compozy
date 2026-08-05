package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBSessionInputQueueGeneration(t *testing.T) {
	t.Run("Should preserve the runtime snapshot through queue leasing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		wantRuntime := store.SessionInputRuntime{
			Provider:        "openai",
			Model:           "gpt-5.6",
			ReasoningEffort: "high",
			Speed:           "fast",
		}

		enqueued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-runtime-snapshot",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "queued input",
			Runtime:           wantRuntime,
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		if got, want := enqueued.Runtime, wantRuntime; got != want {
			t.Fatalf("enqueued runtime = %#v, want %#v", got, want)
		}

		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok {
			t.Fatal("ClaimNextSessionInput() ok = false, want true")
		}
		if got, want := claimed.Runtime, wantRuntime; got != want {
			t.Fatalf("claimed runtime = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve exact skill invocation refs through queue leasing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
		want := []commandpkg.Invocation{{
			Ref: commandpkg.SkillRef{
				CommandID: "skill:extension:ops:review",
				Name:      "review",
				Source: commandpkg.Source{
					Kind: "extension", ID: "ops", Key: "ops", Scope: "workspace",
				},
			},
			Token: "/ops:review", Start: 7, End: 18,
		}}
		enqueued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-skill-ref", SessionID: sessionID,
			Mode: store.SessionInputQueueModeQueue, Text: "Please /ops:review",
			SkillInvocations: want, SessionGeneration: 0, QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		if !slices.Equal(enqueued.SkillInvocations, want) {
			t.Fatalf("enqueued skill refs = %#v, want %#v", enqueued.SkillInvocations, want)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil || !ok {
			t.Fatalf("ClaimNextSessionInput() = ok %v, error %v", ok, err)
		}
		if !slices.Equal(claimed.SkillInvocations, want) {
			t.Fatalf("claimed skill refs = %#v, want %#v", claimed.SkillInvocations, want)
		}
	})

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

	t.Run("Should dispatch current-generation steer before queued input", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 5, 21, 12, 5, 0, 0, time.UTC)

		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID:                "steer-old",
			SessionID:         sessionID,
			TargetTurnID:      "turn-old",
			Delivery:          store.SessionInputDeliveryInterruptThenPrompt,
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
			TargetTurnID:      "turn-active",
			Delivery:          store.SessionInputDeliveryInterruptThenPrompt,
			Text:              "new steer",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("StageSessionSteer(new) error = %v", err)
		}
		queued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "queue-after-steer",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "queued input",
			SessionGeneration: generation,
			QueueCap:          10,
			Now:               now.Add(2500 * time.Millisecond),
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput(queue) error = %v", err)
		}

		consumed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(3*time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput(first) error = %v", err)
		}
		if !ok {
			t.Fatal("ClaimNextSessionInput(first) ok = false, want true")
		}
		if consumed.ID != "steer-new" || consumed.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("consumed = %#v, want dispatching steer-new", consumed)
		}
		if err := globalDB.MarkSessionInputSent(
			ctx,
			sessionID,
			consumed.ID,
			now.Add(3500*time.Millisecond),
		); err != nil {
			t.Fatalf("MarkSessionInputSent(steer) error = %v", err)
		}
		consumed, ok, err = globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(4*time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput(second) error = %v", err)
		}
		if !ok || consumed.ID != queued.ID {
			t.Fatalf("ClaimNextSessionInput(second) = %#v/%v, want queued input %q", consumed, ok, queued.ID)
		}
	})

	t.Run("Should preserve a dispatching steer when a newer steer is staged", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 5, 0, 0, time.UTC)

		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID: "steer-dispatching", SessionID: sessionID, Text: "already dispatching",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}); err != nil {
			t.Fatalf("StageSessionSteer(dispatching) error = %v", err)
		}
		leased, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok || leased.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("ClaimNextSessionInput() = %#v/%v, want dispatching lease", leased, ok)
		}

		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID: "steer-newer", SessionID: sessionID, Text: "newer staged steer",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now.Add(2 * time.Second),
		}); err != nil {
			t.Fatalf("StageSessionSteer(newer) error = %v", err)
		}

		preserved, err := getSessionInputQueueEntry(ctx, globalDB.db, sessionID, leased.ID)
		if err != nil {
			t.Fatalf("getSessionInputQueueEntry(dispatching) error = %v", err)
		}
		if preserved.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf(
				"dispatching steer status = %q, want %q",
				preserved.Status,
				store.SessionInputQueueStatusDispatching,
			)
		}
		if err := globalDB.MarkSessionInputSent(ctx, sessionID, leased.ID, now.Add(3*time.Second)); err != nil {
			t.Fatalf("MarkSessionInputSent(dispatching) error = %v", err)
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
			TargetTurnID:      "turn-active",
			Delivery:          store.SessionInputDeliveryInterruptThenPrompt,
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

	t.Run("Should reject cancel after dispatch ownership has transferred", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 3, 16, 55, 0, 0, time.UTC)
		queued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-dispatch-owned", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "dispatch-owned input", SessionGeneration: 0, QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil || !ok || claimed.ID != queued.ID {
			t.Fatalf("ClaimNextSessionInput() = %#v/%t/%v", claimed, ok, err)
		}

		_, err = globalDB.CancelSessionInput(ctx, sessionID, queued.ID, now.Add(2*time.Second))
		if !errors.Is(err, store.ErrSessionInputQueueEntryNotQueued) {
			t.Fatalf("CancelSessionInput(dispatching) error = %v, want ErrSessionInputQueueEntryNotQueued", err)
		}
		preserved, err := globalDB.GetSessionInputQueueEntry(ctx, sessionID, queued.ID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry() error = %v", err)
		}
		if preserved.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("dispatch-owned status = %q, want dispatching", preserved.Status)
		}
	})

	t.Run("Should atomically replace one queued input without losing its position owner", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 3, 17, 0, 0, 0, time.UTC)
		original, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-replace-original", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "original input", SessionGeneration: 0, QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}

		replacementRequest := store.SessionInputQueueInsert{
			ID: "inq-replace-new", Text: "replacement input", MessageID: "message-replace",
			IdempotencyKey: "idem-replace", QueueCap: 10, Now: now.Add(time.Second),
		}
		replacement, created, err := globalDB.ReplaceSessionInput(ctx, sessionID, original.ID, replacementRequest)
		if err != nil {
			t.Fatalf("ReplaceSessionInput() error = %v", err)
		}
		if !created {
			t.Fatal("ReplaceSessionInput() created = false, want true")
		}
		if replacement.ID != "inq-replace-new" || replacement.Mode != store.SessionInputQueueModeQueue ||
			replacement.Delivery != store.SessionInputDeliveryAfterTurn || replacement.Text != "replacement input" {
			t.Fatalf("replacement = %#v, want new queued input", replacement)
		}
		canceled, err := globalDB.GetSessionInputQueueEntry(ctx, sessionID, original.ID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry(original) error = %v", err)
		}
		if canceled.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("original status = %q, want canceled", canceled.Status)
		}
		pending, err := globalDB.ListPendingSessionInputs(ctx, sessionID)
		if err != nil {
			t.Fatalf("ListPendingSessionInputs() error = %v", err)
		}
		if len(pending) != 1 || pending[0].ID != replacement.ID {
			t.Fatalf("pending inputs = %#v, want only replacement", pending)
		}
		replayed, replayCreated, err := globalDB.ReplaceSessionInput(
			ctx,
			sessionID,
			original.ID,
			replacementRequest,
		)
		if err != nil {
			t.Fatalf("ReplaceSessionInput(replay) error = %v", err)
		}
		if replayCreated || replayed.ID != replacement.ID {
			t.Fatalf(
				"ReplaceSessionInput(replay) = %#v/%t, want same identity without creation",
				replayed,
				replayCreated,
			)
		}
		conflictingRequest := replacementRequest
		conflictingRequest.Text = "different replacement"
		_, _, err = globalDB.ReplaceSessionInput(ctx, sessionID, original.ID, conflictingRequest)
		if !errors.Is(err, store.ErrSessionInputMutationConflict) {
			t.Fatalf("ReplaceSessionInput(conflict) error = %v, want ErrSessionInputMutationConflict", err)
		}
	})

	t.Run("Should leave the original queued when replacement validation fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 3, 17, 5, 0, 0, time.UTC)
		original, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-invalid-replace-original", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "preserve me", SessionGeneration: 0, QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		_, _, err = globalDB.ReplaceSessionInput(ctx, sessionID, original.ID, store.SessionInputQueueInsert{
			ID: "inq-invalid-replace-new", Text: "", QueueCap: 10, Now: now.Add(time.Second),
		})
		if err == nil {
			t.Fatal("ReplaceSessionInput() error = nil, want validation failure")
		}
		preserved, err := globalDB.GetSessionInputQueueEntry(ctx, sessionID, original.ID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry(original) error = %v", err)
		}
		if preserved.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("original status = %q, want queued", preserved.Status)
		}
	})

	t.Run("Should atomically promote queued input and supersede older steering", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 3, 17, 10, 0, 0, time.UTC)
		if _, err := globalDB.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID: "inq-prior-steer", SessionID: sessionID, TargetTurnID: "turn-live",
			Delivery: store.SessionInputDeliveryInterruptThenPrompt, Text: "older steer",
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}); err != nil {
			t.Fatalf("StageSessionSteer() error = %v", err)
		}
		queued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-promote-original", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "promote me", SessionGeneration: 0, QueueCap: 10, Now: now.Add(time.Second),
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		promoted, created, err := globalDB.PromoteSessionInputToSteer(
			ctx,
			sessionID,
			queued.ID,
			store.SessionInputQueueInsert{
				ID: "inq-promoted-steer", TargetTurnID: "turn-live", Text: "promoted steer",
				MessageID: "message-promote", IdempotencyKey: "idem-promote", QueueCap: 10,
				Now: now.Add(2 * time.Second),
			},
		)
		if err != nil {
			t.Fatalf("PromoteSessionInputToSteer() error = %v", err)
		}
		if !created {
			t.Fatal("PromoteSessionInputToSteer() created = false, want true")
		}
		if promoted.Mode != store.SessionInputQueueModeSteer ||
			promoted.Delivery != store.SessionInputDeliveryInterruptThenPrompt ||
			promoted.TargetTurnID != "turn-live" {
			t.Fatalf("promoted input = %#v, want interrupt-then-prompt steer", promoted)
		}
		pending, err := globalDB.ListPendingSessionInputs(ctx, sessionID)
		if err != nil {
			t.Fatalf("ListPendingSessionInputs() error = %v", err)
		}
		if len(pending) != 1 || pending[0].ID != promoted.ID {
			t.Fatalf("pending inputs = %#v, want only promoted steer", pending)
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

func TestGlobalDBSessionPromptAdmissionMigration(t *testing.T) {
	t.Run("Should create the prompt admission schema on a fresh database", func(t *testing.T) {
		globalDB := openFreshTestGlobalDB(t)
		assertTablesPresent(t, globalDB.db, "session_prompt_admissions", "session_input_queue")
		assertTableHasColumns(t, globalDB.db, "session_input_queue", []string{
			"prompt_admission_id",
			"message_id",
			"idempotency_key",
			"turn_id",
			"event_id",
		})
		assertIndexesPresent(
			t,
			globalDB.db,
			"session_input_queue",
			"uq_session_input_queue_prompt_admission",
		)
		assertIndexesPresent(
			t,
			globalDB.db,
			"session_prompt_admissions",
			"idx_session_prompt_admissions_state",
		)
	})

	t.Run("Should require queue admissions to exist and clear deleted receipts", func(t *testing.T) {
		ctx := testutil.Context(t)
		globalDB := openFreshTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "foreign-key", now)
		admission, created, err := globalDB.ClaimSessionPromptAdmission(ctx, admissionReq)
		if err != nil {
			t.Fatalf("ClaimSessionPromptAdmission() error = %v", err)
		}
		if !created {
			t.Fatal("ClaimSessionPromptAdmission() created = false, want true")
		}

		_, err = globalDB.db.ExecContext(
			ctx,
			`INSERT INTO session_input_queue (
				id, session_id, prompt_admission_id, status, mode, text, enqueued_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"inq-missing-admission",
			sessionID,
			"admission-missing",
			store.SessionInputQueueStatusQueued,
			store.SessionInputQueueModeQueue,
			"missing admission",
			store.FormatTimestamp(now),
			store.FormatTimestamp(now),
		)
		requireSQLiteConstraintError(t, err)

		if _, err := globalDB.db.ExecContext(
			ctx,
			`INSERT INTO session_input_queue (
				id, session_id, prompt_admission_id, status, mode, text, enqueued_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"inq-deleted-admission",
			sessionID,
			admission.ID,
			store.SessionInputQueueStatusQueued,
			store.SessionInputQueueModeQueue,
			"retained queue history",
			store.FormatTimestamp(now),
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatalf("insert queue admission reference error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`DELETE FROM session_prompt_admissions WHERE id = ?`,
			admission.ID,
		); err != nil {
			t.Fatalf("delete session prompt admission error = %v", err)
		}

		var promptAdmissionID sql.NullString
		if err := globalDB.db.QueryRowContext(
			ctx,
			`SELECT prompt_admission_id FROM session_input_queue WHERE id = ?`,
			"inq-deleted-admission",
		).Scan(&promptAdmissionID); err != nil {
			t.Fatalf("query queue admission reference after receipt delete error = %v", err)
		}
		if promptAdmissionID.Valid {
			t.Fatalf("prompt_admission_id after receipt delete = %q, want NULL", promptAdmissionID.String)
		}
	})

	t.Run("Should upgrade the 00033 queue to an admission-aware schema", func(t *testing.T) {
		fixture := openSessionPromptAdmissionMigrationFixture(t)

		t.Run("Should preserve legacy queue values and clear orphaned receipt links", func(t *testing.T) {
			legacy, err := fixture.database.GetSessionInputQueueEntry(
				fixture.ctx,
				fixture.sessionID,
				"inq-legacy-before-00034",
			)
			if err != nil {
				t.Fatalf("GetSessionInputQueueEntry(legacy) error = %v", err)
			}
			if legacy.PromptAdmissionID != "" || legacy.MessageID != "" || legacy.IdempotencyKey != "" ||
				legacy.TurnID != "" || legacy.EventID != "" {
				t.Fatalf("legacy queue identity = %#v, want empty 00034 defaults", legacy)
			}
			if got, want := legacy.Runtime, (store.SessionInputRuntime{
				Provider: "claude", Model: "sonnet", ReasoningEffort: "high", Speed: "fast",
			}); got != want {
				t.Fatalf("legacy queue runtime = %#v, want %#v", got, want)
			}
			orphaned, err := fixture.database.GetSessionInputQueueEntry(
				fixture.ctx,
				fixture.sessionID,
				"inq-orphaned-admission-before-00036",
			)
			if err != nil {
				t.Fatalf("GetSessionInputQueueEntry(orphaned) error = %v", err)
			}
			if orphaned.PromptAdmissionID != "" {
				t.Fatalf("orphaned prompt admission after 00036 = %q, want NULL", orphaned.PromptAdmissionID)
			}
		})

		t.Run("Should report the migration stream as complete after reopening", func(t *testing.T) {
			status, err := store.Status(fixture.ctx, fixture.database.db, MigrationStream())
			if err != nil {
				t.Fatalf("Status(global) error = %v", err)
			}
			assertCompleteMigrationStream(t, status, MigrationStream())
		})

		t.Run("Should enforce receipt constraints for migrated queue entries", func(t *testing.T) {
			admissionReq := promptAdmissionRequest(
				"ws-input-queue-workspace",
				fixture.sessionID,
				"migration",
				fixture.now,
			)
			admission, created, err := fixture.database.ClaimSessionPromptAdmission(fixture.ctx, admissionReq)
			if err != nil {
				t.Fatalf("ClaimSessionPromptAdmission() error = %v", err)
			}
			if !created {
				t.Fatal("ClaimSessionPromptAdmission() created = false, want true")
			}
			_, err = fixture.database.db.ExecContext(
				fixture.ctx,
				`INSERT INTO session_prompt_admissions (
					id, workspace_id, session_id, message_id, idempotency_key, operation,
					fingerprint_version, request_fingerprint, state, mode, authored_text,
					turn_id, event_id, created_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				"admission-migration-duplicate",
				admissionReq.WorkspaceID,
				fixture.sessionID,
				"message-migration-duplicate",
				admissionReq.IdempotencyKey,
				store.SessionPromptOperationPrompt,
				admissionReq.FingerprintVersion,
				"sha256:migration-duplicate",
				store.SessionPromptAdmissionReserved,
				store.SessionInputQueueModeQueue,
				"duplicate idempotency key",
				"turn-migration-duplicate",
				"event-migration-duplicate",
				store.FormatTimestamp(fixture.now),
				store.FormatTimestamp(fixture.now),
			)
			if !isSQLiteUniqueConstraint(err) {
				t.Fatalf("duplicate session prompt admission error = %v, want SQLite unique constraint", err)
			}

			queueReq := store.SessionInputQueueInsert{
				ID:                "inq-admitted-migration",
				SessionID:         fixture.sessionID,
				Text:              "admitted after migration",
				SessionGeneration: 0,
				QueueCap:          10,
				Now:               fixture.now,
			}
			_, entry, _, created, err := fixture.database.EnqueueAdmittedSessionInput(
				fixture.ctx,
				admissionReq,
				queueReq,
			)
			if err != nil {
				t.Fatalf("EnqueueAdmittedSessionInput() error = %v", err)
			}
			if !created || entry.PromptAdmissionID != admission.ID {
				t.Fatalf("admitted queue entry = %#v, created=%v, want admission %q", entry, created, admission.ID)
			}
			_, err = fixture.database.db.ExecContext(
				fixture.ctx,
				`INSERT INTO session_input_queue (
					id, session_id, prompt_admission_id, status, mode, text, enqueued_at, updated_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				"inq-admitted-migration-duplicate",
				fixture.sessionID,
				admission.ID,
				store.SessionInputQueueStatusQueued,
				store.SessionInputQueueModeQueue,
				"duplicate admission binding",
				store.FormatTimestamp(fixture.now),
				store.FormatTimestamp(fixture.now),
			)
			if !isSQLiteUniqueConstraint(err) {
				t.Fatalf("duplicate session input queue error = %v, want SQLite unique constraint", err)
			}
			if _, err := fixture.database.db.ExecContext(
				fixture.ctx,
				`DELETE FROM session_prompt_admissions WHERE id = ?`,
				admission.ID,
			); err != nil {
				t.Fatalf("delete migrated session prompt admission error = %v", err)
			}
			migratedEntry, err := fixture.database.GetSessionInputQueueEntry(
				fixture.ctx,
				fixture.sessionID,
				entry.ID,
			)
			if err != nil {
				t.Fatalf("GetSessionInputQueueEntry(migrated) error = %v", err)
			}
			if migratedEntry.PromptAdmissionID != "" {
				t.Fatalf(
					"migrated prompt admission after receipt delete = %q, want NULL",
					migratedEntry.PromptAdmissionID,
				)
			}
		})
	})
}

type sessionPromptAdmissionMigrationFixture struct {
	ctx       context.Context
	database  *GlobalDB
	sessionID string
	now       time.Time
}

func openSessionPromptAdmissionMigrationFixture(t *testing.T) sessionPromptAdmissionMigrationFixture {
	t.Helper()

	path := filepath.Join(t.TempDir(), GlobalDatabaseName)
	prefixDB, err := openGlobalMigrationPrefixDatabase(
		t,
		path,
		globalMigrationPrefixBefore(t, "00034_schema.sql"),
	)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase(00033 prefix) error = %v", err)
	}
	prefixClosed := false
	defer func() {
		if prefixClosed {
			return
		}
		if closeErr := prefixDB.Close(); closeErr != nil {
			t.Errorf("prefixDB.Close() error = %v", closeErr)
		}
	}()

	ctx := testutil.Context(t)
	now := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	prefixGlobalDB := &GlobalDB{
		db:   prefixDB,
		path: path,
		now: func() time.Time {
			return now
		},
	}
	prefixGlobalDB.initializeRepositories(openConfig{})
	sessionID := registerInputQueueMigrationPrefixSession(t, prefixGlobalDB, now)
	if _, err := prefixDB.ExecContext(
		ctx,
		`INSERT INTO session_input_queue (
			id, session_id, status, mode, text,
			runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
			session_generation, enqueued_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"inq-legacy-before-00034",
		sessionID,
		store.SessionInputQueueStatusQueued,
		store.SessionInputQueueModeQueue,
		"legacy queued input",
		"claude",
		"sonnet",
		"high",
		"fast",
		0,
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed 00033 session_input_queue row error = %v", err)
	}
	if err := applyGlobalMigrationPrefix(
		t,
		prefixDB,
		globalMigrationPrefixBefore(t, "00036_schema.sql"),
	); err != nil {
		t.Fatalf("Apply(global through 00035) error = %v", err)
	}
	if _, err := prefixDB.ExecContext(
		ctx,
		`INSERT INTO session_input_queue (
			id, session_id, prompt_admission_id, status, mode, text, enqueued_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"inq-orphaned-admission-before-00036",
		sessionID,
		"admission-orphaned-before-00036",
		store.SessionInputQueueStatusQueued,
		store.SessionInputQueueModeQueue,
		"orphaned receipt reference",
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed orphaned 00035 admission reference error = %v", err)
	}
	if err := prefixDB.Close(); err != nil {
		t.Fatalf("prefixDB.Close() error = %v", err)
	}
	prefixClosed = true

	upgraded, err := openGlobalMigrationUpgrade(t, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(00036 upgrade) error = %v", err)
	}
	upgradedClosed := false
	defer func() {
		if upgradedClosed {
			return
		}
		if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
			t.Errorf("GlobalDB.Close(upgrade) error = %v", closeErr)
		}
	}()
	if err := upgraded.Close(ctx); err != nil {
		t.Fatalf("GlobalDB.Close(upgrade) error = %v", err)
	}
	upgradedClosed = true

	reopened, err := OpenGlobalDB(ctx, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := reopened.Close(testutil.Context(t)); closeErr != nil {
			t.Errorf("GlobalDB.Close(reopen) error = %v", closeErr)
		}
	})
	return sessionPromptAdmissionMigrationFixture{
		ctx:       ctx,
		database:  reopened,
		sessionID: sessionID,
		now:       now,
	}
}

func TestGlobalDBSessionPromptAdmission(t *testing.T) {
	t.Run("Should preserve the legacy queue terminal path without an admission receipt", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 1, 8, 55, 0, 0, time.UTC)
		enqueued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-legacy-terminal",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "legacy queue input",
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		if _, ok, claimErr := globalDB.ClaimNextSessionInput(
			ctx,
			sessionID,
			now.Add(time.Second),
		); claimErr != nil || !ok {
			t.Fatalf("ClaimNextSessionInput() = ok %v, error %v", ok, claimErr)
		}
		if err := globalDB.MarkSessionInputSent(ctx, sessionID, enqueued.ID, now.Add(2*time.Second)); err != nil {
			t.Fatalf("MarkSessionInputSent() error = %v", err)
		}
		completed, err := globalDB.GetSessionInputQueueEntry(ctx, sessionID, enqueued.ID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry() error = %v", err)
		}
		if completed.PromptAdmissionID != "" || completed.Status != store.SessionInputQueueStatusSent {
			t.Fatalf("completed legacy entry = %#v, want sent with no admission receipt", completed)
		}
	})

	t.Run("Should fence an admitted queue retry for the active dispatch lease", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "queue-lease", now)
		queueReq := store.SessionInputQueueInsert{
			ID: "inq-admitted-lease", SessionID: sessionID, Text: "queued once",
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}
		_, enqueued, _, created, err := globalDB.EnqueueAdmittedSessionInput(ctx, admissionReq, queueReq)
		if err != nil || !created {
			t.Fatalf("EnqueueAdmittedSessionInput() = created %v, error %v", created, err)
		}

		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok || claimed.ID != enqueued.ID || claimed.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("ClaimNextSessionInput() = %#v/%v, want dispatching %q", claimed, ok, enqueued.ID)
		}
		_, _, err = globalDB.ClaimSessionPromptAdmission(ctx, admissionReq)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("ClaimSessionPromptAdmission(during lease) error = %v, want dispatch indeterminate", err)
		}
	})

	t.Run("Should restore the admitted queue replay after send or release", func(t *testing.T) {
		t.Parallel()

		for _, terminal := range []struct {
			name   string
			status string
			apply  func(*testing.T, *GlobalDB, string, string, time.Time) error
		}{
			{
				name:   "Should restore replay after sending the leased input",
				status: store.SessionInputQueueStatusSent,
				apply: func(t *testing.T, db *GlobalDB, sessionID string, entryID string, now time.Time) error {
					return db.MarkSessionInputSent(testutil.Context(t), sessionID, entryID, now)
				},
			},
			{
				name:   "Should restore replay after releasing the leased input",
				status: store.SessionInputQueueStatusQueued,
				apply: func(t *testing.T, db *GlobalDB, sessionID string, entryID string, now time.Time) error {
					return db.ReleaseSessionInput(testutil.Context(t), sessionID, entryID, now)
				},
			},
		} {
			t.Run(terminal.name, func(t *testing.T) {
				t.Parallel()

				ctx := testutil.Context(t)
				db := openTestGlobalDB(t)
				sessionID := registerInputQueueSession(t, db)
				now := time.Date(2026, 8, 1, 9, 5, 0, 0, time.UTC)
				admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, terminal.name, now)
				queueReq := store.SessionInputQueueInsert{
					ID:        "inq-admitted-replay-" + admissionReq.IdempotencyKey,
					SessionID: sessionID, Text: "queued once", SessionGeneration: 0, QueueCap: 10, Now: now,
				}
				_, enqueued, _, _, err := db.EnqueueAdmittedSessionInput(ctx, admissionReq, queueReq)
				if err != nil {
					t.Fatalf("EnqueueAdmittedSessionInput() error = %v", err)
				}
				if _, ok, claimErr := db.ClaimNextSessionInput(
					ctx,
					sessionID,
					now.Add(time.Second),
				); claimErr != nil || !ok {
					t.Fatalf("ClaimNextSessionInput() = ok %v, error %v", ok, claimErr)
				}
				if applyErr := terminal.apply(t, db, sessionID, enqueued.ID, now.Add(2*time.Second)); applyErr != nil {
					t.Fatalf("terminal transition error = %v", applyErr)
				}
				terminalEntry, getErr := db.GetSessionInputQueueEntry(ctx, sessionID, enqueued.ID)
				if getErr != nil {
					t.Fatalf("GetSessionInputQueueEntry(after terminal) error = %v", getErr)
				}
				if terminalEntry.Status != terminal.status {
					t.Fatalf("terminal entry status = %q, want %q", terminalEntry.Status, terminal.status)
				}
				replayed, created, replayErr := db.ClaimSessionPromptAdmission(ctx, admissionReq)
				if replayErr != nil {
					t.Fatalf("ClaimSessionPromptAdmission(after terminal) error = %v", replayErr)
				}
				if created || replayed.State != store.SessionPromptAdmissionCompleted || replayed.Result == nil ||
					replayed.Result.QueueEntryID != enqueued.ID {
					t.Fatalf(
						"replayed admission = %#v, created=%v, want completed receipt for %q",
						replayed,
						created,
						enqueued.ID,
					)
				}
			})
		}
	})

	t.Run("Should permanently fence an admitted queue retry after dispatch failure", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 1, 9, 10, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "queue-failure", now)
		queueReq := store.SessionInputQueueInsert{
			ID: "inq-admitted-failure", SessionID: sessionID, Text: "queued once",
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}
		_, enqueued, _, _, err := globalDB.EnqueueAdmittedSessionInput(ctx, admissionReq, queueReq)
		if err != nil {
			t.Fatalf("EnqueueAdmittedSessionInput() error = %v", err)
		}
		if _, ok, claimErr := globalDB.ClaimNextSessionInput(
			ctx,
			sessionID,
			now.Add(time.Second),
		); claimErr != nil || !ok {
			t.Fatalf("ClaimNextSessionInput() = ok %v, error %v", ok, claimErr)
		}
		if err := globalDB.MarkSessionInputFailed(
			ctx,
			sessionID,
			enqueued.ID,
			"driver failed",
			now.Add(2*time.Second),
		); err != nil {
			t.Fatalf("MarkSessionInputFailed() error = %v", err)
		}
		failed, getErr := globalDB.GetSessionInputQueueEntry(ctx, sessionID, enqueued.ID)
		if getErr != nil {
			t.Fatalf("GetSessionInputQueueEntry(after failure) error = %v", getErr)
		}
		if failed.Status != store.SessionInputQueueStatusFailed || failed.FailureSummary != "driver failed" {
			t.Fatalf("failed entry = %#v, want failed entry with the dispatch reason", failed)
		}
		_, _, err = globalDB.ClaimSessionPromptAdmission(ctx, admissionReq)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("ClaimSessionPromptAdmission(after failure) error = %v, want dispatch indeterminate", err)
		}
	})

	t.Run("Should lease an admitted steer exactly once before sending it", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 1, 9, 15, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "steer-lease", now)
		admissionReq.Mode = store.SessionInputQueueModeSteer
		queueReq := store.SessionInputQueueInsert{
			ID: "steer-admitted-lease", SessionID: sessionID, Text: "steer once",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}
		_, staged, _, err := globalDB.StageAdmittedSessionSteer(ctx, admissionReq, queueReq)
		if err != nil {
			t.Fatalf("StageAdmittedSessionSteer() error = %v", err)
		}
		consumed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput(first) error = %v", err)
		}
		if !ok || consumed.ID != staged.ID || consumed.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("ClaimNextSessionInput(first) = %#v/%v, want dispatching %q", consumed, ok, staged.ID)
		}
		_, ok, err = globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput(second) error = %v", err)
		}
		if ok {
			t.Fatal("ClaimNextSessionInput(second) ok = true, want false while lease is active")
		}
		_, _, err = globalDB.ClaimSessionPromptAdmission(ctx, admissionReq)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("ClaimSessionPromptAdmission(during steer lease) error = %v, want dispatch indeterminate", err)
		}
	})

	t.Run("Should replay one exact admission and reject identity collisions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		workspaceID := "ws-input-queue-workspace"
		now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
		req := promptAdmissionRequest(workspaceID, sessionID, "exact", now)
		req.SkillInvocations = []commandpkg.Invocation{{
			Ref: commandpkg.SkillRef{
				CommandID: "skill:workspace::review", Name: "review",
				Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
			},
			Token: "/review", Start: 8, End: 15,
		}}

		first, created, err := globalDB.ClaimSessionPromptAdmission(ctx, req)
		if err != nil {
			t.Fatalf("ClaimSessionPromptAdmission(first) error = %v", err)
		}
		if !created {
			t.Fatal("ClaimSessionPromptAdmission(first) created = false, want true")
		}
		if got, want := first.State, store.SessionPromptAdmissionReserved; got != want {
			t.Fatalf("first.State = %q, want %q", got, want)
		}

		replayed, created, err := globalDB.ClaimSessionPromptAdmission(ctx, req)
		if err != nil {
			t.Fatalf("ClaimSessionPromptAdmission(replay) error = %v", err)
		}
		if created {
			t.Fatal("ClaimSessionPromptAdmission(replay) created = true, want false")
		}
		if replayed.ID != first.ID || replayed.MessageID != first.MessageID {
			t.Fatalf("replayed admission = %#v, want identity from %#v", replayed, first)
		}
		if !slices.Equal(replayed.SkillInvocations, req.SkillInvocations) {
			t.Fatalf("replayed skill refs = %#v, want %#v", replayed.SkillInvocations, req.SkillInvocations)
		}

		fingerprintCollision := req
		fingerprintCollision.RequestFingerprint = "sha256:different"
		_, _, err = globalDB.ClaimSessionPromptAdmission(ctx, fingerprintCollision)
		if !errors.Is(err, store.ErrSessionPromptIdempotencyConflict) {
			t.Fatalf("ClaimSessionPromptAdmission(fingerprint collision) error = %v", err)
		}

		messageCollision := req
		messageCollision.ID = "admission-message-collision"
		messageCollision.IdempotencyKey = "idem-message-collision"
		messageCollision.RequestFingerprint = "sha256:message-collision"
		_, _, err = globalDB.ClaimSessionPromptAdmission(ctx, messageCollision)
		if !errors.Is(err, store.ErrSessionPromptMessageConflict) {
			t.Fatalf("ClaimSessionPromptAdmission(message collision) error = %v", err)
		}
	})

	t.Run("Should replay an admitted queue entry without consuming capacity twice", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 5, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "queue", now)
		queueReq := store.SessionInputQueueInsert{
			ID: "inq-admitted", SessionID: sessionID, Text: "queued once",
			SessionGeneration: 0, QueueCap: 1, Now: now,
		}

		admission, entry, position, created, err := globalDB.EnqueueAdmittedSessionInput(
			ctx,
			admissionReq,
			queueReq,
		)
		if err != nil {
			t.Fatalf("EnqueueAdmittedSessionInput(first) error = %v", err)
		}
		if !created || position != 1 || admission.State != store.SessionPromptAdmissionCompleted {
			t.Fatalf("first admission = %#v, position=%d created=%v", admission, position, created)
		}
		if entry.MessageID != admissionReq.MessageID || entry.IdempotencyKey != admissionReq.IdempotencyKey ||
			entry.TurnID != admissionReq.TurnID || entry.EventID != admissionReq.EventID {
			t.Fatalf("entry identity = %#v, want admission identity %#v", entry, admissionReq)
		}

		replayed, replayedEntry, replayedPosition, created, err := globalDB.EnqueueAdmittedSessionInput(
			ctx,
			admissionReq,
			queueReq,
		)
		if err != nil {
			t.Fatalf("EnqueueAdmittedSessionInput(replay) error = %v", err)
		}
		if created || replayed.ID != admission.ID || replayedEntry.ID != entry.ID || replayedPosition != position {
			t.Fatalf(
				"replay = admission %#v entry %#v position=%d created=%v",
				replayed,
				replayedEntry,
				replayedPosition,
				created,
			)
		}
		summary, err := globalDB.SessionInputQueueSummary(ctx, sessionID)
		if err != nil {
			t.Fatalf("SessionInputQueueSummary() error = %v", err)
		}
		if summary.PendingQueued != 1 {
			t.Fatalf("summary.PendingQueued = %d, want 1", summary.PendingQueued)
		}
	})

	t.Run("Should not replace the latest staged steer when replaying an older command", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 10, 0, 0, time.UTC)
		firstAdmission := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "steer-a", now)
		firstAdmission.Mode = store.SessionInputQueueModeSteer
		firstQueue := store.SessionInputQueueInsert{
			ID: "steer-admitted-a", SessionID: sessionID, Text: "first steer",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}
		_, firstEntry, created, err := globalDB.StageAdmittedSessionSteer(ctx, firstAdmission, firstQueue)
		if err != nil || !created {
			t.Fatalf("StageAdmittedSessionSteer(first) = created %v, error %v", created, err)
		}

		secondAdmission := promptAdmissionRequest(
			"ws-input-queue-workspace",
			sessionID,
			"steer-b",
			now.Add(time.Second),
		)
		secondAdmission.Mode = store.SessionInputQueueModeSteer
		secondQueue := store.SessionInputQueueInsert{
			ID: "steer-admitted-b", SessionID: sessionID, Text: "second steer",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now.Add(time.Second),
		}
		_, secondEntry, created, err := globalDB.StageAdmittedSessionSteer(ctx, secondAdmission, secondQueue)
		if err != nil || !created {
			t.Fatalf("StageAdmittedSessionSteer(second) = created %v, error %v", created, err)
		}

		_, replayedEntry, created, err := globalDB.StageAdmittedSessionSteer(
			ctx,
			firstAdmission,
			firstQueue,
		)
		if err != nil {
			t.Fatalf("StageAdmittedSessionSteer(replay) error = %v", err)
		}
		if created ||
			replayedEntry.ID != firstEntry.ID ||
			replayedEntry.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("replayed entry = %#v, created=%v", replayedEntry, created)
		}
		consumed, ok, err := globalDB.PeekNextSessionInput(ctx, sessionID)
		if err != nil {
			t.Fatalf("PeekNextSessionInput() error = %v", err)
		}
		if !ok || consumed.ID != secondEntry.ID {
			t.Fatalf("PeekNextSessionInput() = %#v/%v, want %q", consumed, ok, secondEntry.ID)
		}
	})

	t.Run("Should fence retries after the dispatch boundary", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 15, 0, 0, time.UTC)
		req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "dispatch", now)
		if _, _, err := globalDB.ClaimSessionPromptAdmission(ctx, req); err != nil {
			t.Fatalf("ClaimSessionPromptAdmission() error = %v", err)
		}
		if err := globalDB.CommitSessionPromptDispatch(
			ctx,
			req.WorkspaceID,
			req.SessionID,
			req.IdempotencyKey,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CommitSessionPromptDispatch() error = %v", err)
		}
		if err := globalDB.CommitSessionPromptDispatch(
			ctx,
			req.WorkspaceID,
			req.SessionID,
			req.IdempotencyKey,
			now.Add(2*time.Second),
		); !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("CommitSessionPromptDispatch(retry) error = %v, want dispatch indeterminate", err)
		}
		if err := globalDB.MarkSessionPromptAdmissionIndeterminate(
			ctx,
			req.WorkspaceID,
			req.SessionID,
			req.IdempotencyKey,
			"delivery outcome is unknown",
			now.Add(3*time.Second),
		); err != nil {
			t.Fatalf("MarkSessionPromptAdmissionIndeterminate() error = %v", err)
		}
		if err := globalDB.MarkSessionPromptAdmissionIndeterminate(
			ctx,
			req.WorkspaceID,
			req.SessionID,
			req.IdempotencyKey,
			"delivery outcome is unknown",
			now.Add(4*time.Second),
		); !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("MarkSessionPromptAdmissionIndeterminate(retry) error = %v, want dispatch indeterminate", err)
		}
		_, _, err := globalDB.ClaimSessionPromptAdmission(ctx, req)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("ClaimSessionPromptAdmission(retry) error = %v", err)
		}
	})

	t.Run("Should fence a completed queue admission after activation fails", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 20, 0, 0, time.UTC)
		req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "activation-failure", now)
		req.Mode = store.SessionInputQueueModeSteer
		queueReq := store.SessionInputQueueInsert{
			ID: "steer-admitted-activation-failure", SessionID: sessionID, Text: "steer once",
			TargetTurnID: "turn-active", Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			SessionGeneration: 0, QueueCap: 10, Now: now,
		}
		admission, _, created, err := globalDB.StageAdmittedSessionSteer(ctx, req, queueReq)
		if err != nil || !created || admission.State != store.SessionPromptAdmissionCompleted {
			t.Fatalf("StageAdmittedSessionSteer() = %#v created=%v error=%v", admission, created, err)
		}
		if err := globalDB.MarkSessionPromptAdmissionIndeterminate(
			ctx,
			req.WorkspaceID,
			req.SessionID,
			req.IdempotencyKey,
			"active-turn cancellation failed",
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("MarkSessionPromptAdmissionIndeterminate(completed) error = %v", err)
		}
		_, _, _, err = globalDB.StageAdmittedSessionSteer(ctx, req, queueReq)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("StageAdmittedSessionSteer(retry) error = %v, want dispatch indeterminate", err)
		}
	})
}

func promptAdmissionRequest(
	workspaceID string,
	sessionID string,
	suffix string,
	now time.Time,
) store.SessionPromptAdmissionRequest {
	return store.SessionPromptAdmissionRequest{
		ID: "admission-" + suffix, WorkspaceID: workspaceID, SessionID: sessionID,
		MessageID: "message-" + suffix, IdempotencyKey: "idem-" + suffix,
		Operation: store.SessionPromptOperationPrompt, FingerprintVersion: "prompt/v1",
		RequestFingerprint: "sha256:" + suffix, Mode: store.SessionInputQueueModeQueue,
		AuthoredText: "message " + suffix, TurnID: "turn-" + suffix, EventID: "event-" + suffix,
		Now: now,
	}
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
		ID:            sessionID,
		Name:          "Input Queue",
		AgentName:     "coder",
		Provider:      "claude",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		SessionType:   defaultSessionType,
		State:         "active",
		CreatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	return sessionID
}

func registerInputQueueMigrationPrefixSession(t *testing.T, globalDB *GlobalDB, now time.Time) string {
	t.Helper()

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"input-queue-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)
	sessionID := "sess-input-queue"
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO sessions (
			id, agent_name, provider, workspace_id, state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		"coder",
		"claude",
		workspaceID,
		"active",
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed 00033 session error = %v", err)
	}
	return sessionID
}
