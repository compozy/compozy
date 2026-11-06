package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	commandpkg "github.com/compozy/compozy/internal/command"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGlobalDBSessionInputQueueGeneration(t *testing.T) {
	t.Run("Should atomically supersede undelivered steering when admitting an interrupt", func(t *testing.T) {
		t.Parallel()
		for _, delivery := range []store.SteerDeliveryMode{
			store.SteerDeliveryPendingBoundary, store.SteerDeliveryInterruptFallback, "",
			store.SteerDeliveryPendingInjection, store.SteerDeliveryInjected,
		} {
			for _, rollback := range []bool{false, true} {
				t.Run(
					fmt.Sprintf("Should preserve atomic supersede for %s rollback %t", delivery, rollback),
					func(t *testing.T) {
						t.Parallel()
						ctx := t.Context()
						db := openTestGlobalDB(t)
						sessionID := registerInputQueueSession(t, db)
						now := time.Date(2026, 11, 4, 22, 0, 0, 0, time.UTC)
						for _, entry := range []store.SessionInputQueueInsert{
							{ID: "queued", SessionID: sessionID, Text: "ordinary queue", QueueCap: 10, Now: now},
							{ID: "old-steer", SessionID: sessionID, Text: "guidance", Mode: store.SessionInputQueueModeSteer, SteerDelivery: delivery, TargetTurnID: "active-turn", Delivery: store.SessionInputDeliveryInterruptThenPrompt, QueueCap: 10, Now: now},
						} {
							if _, _, err := db.EnqueueSessionInput(ctx, entry); err != nil {
								t.Fatal(err)
							}
						}
						if delivery == "" || delivery == store.SteerDeliveryPendingInjection ||
							delivery == store.SteerDeliveryInjected {
							if _, ok, err := db.ReserveSessionSteer(
								ctx,
								sessionID,
								"old-steer",
								now,
							); err != nil ||
								!ok {
								t.Fatalf("reserve=%t/%v", ok, err)
							}
							if delivery != "" {
								if _, err := db.ResolveSessionSteer(
									ctx,
									sessionID,
									"old-steer",
									delivery,
									now,
								); err != nil {
									t.Fatal(err)
								}
							}
						}
						id := "interrupt"
						if rollback {
							id = "queued"
						}
						replacement, canceled, err := db.EnqueueInterruptSessionInput(
							ctx,
							store.SessionInputQueueInsert{
								ID:           id,
								SessionID:    sessionID,
								Text:         "replacement",
								TargetTurnID: "active-turn",
								QueueCap:     10,
								Now:          now.Add(time.Second),
							},
						)
						if rollback {
							if err == nil ||
								!strings.Contains(err.Error(), "UNIQUE constraint failed: session_input_queue.id") {
								t.Fatalf("duplicate interrupt error=%v, want queue ID uniqueness constraint", err)
							}
						} else {
							wantIDs := []string{"old-steer"}
							if delivery == store.SteerDeliveryInjected {
								wantIDs = nil
							}
							if err != nil || canceled != len(wantIDs) ||
								!slices.Equal(replacement.SupersededIDs, wantIDs) {
								t.Fatalf("interrupt=%#v/%d/%v, want superseded %v", replacement, canceled, err, wantIDs)
							}
						}
						old, err := db.GetSessionInputQueueEntry(ctx, sessionID, "old-steer")
						if err != nil {
							t.Fatal(err)
						}
						want := store.SessionInputQueueStatusCanceled
						if rollback {
							want = store.SessionInputQueueStatusQueued
							if delivery == "" {
								want = store.SessionInputQueueStatusDispatching
							}
							if delivery == store.SteerDeliveryPendingInjection {
								want = store.SessionInputQueueStatusSent
							}
						}
						if delivery == store.SteerDeliveryInjected {
							want = store.SessionInputQueueStatusSent
						}
						if old.Status != want {
							t.Fatalf("old steering status=%s, want %s", old.Status, want)
						}
						queued, err := db.GetSessionInputQueueEntry(ctx, sessionID, "queued")
						if err != nil || queued.Status != store.SessionInputQueueStatusQueued {
							t.Fatalf("ordinary queue=%#v/%v", queued, err)
						}
						if !rollback {
							wantReplacement := "interrupt"
							if delivery == store.SteerDeliveryInjected {
								wantReplacement = ""
							}
							if old.SupersededBy != wantReplacement {
								t.Fatalf("superseding identity=%q", old.SupersededBy)
							}
							if delivery == "" {
								_, err := db.ResolveSessionSteer(
									ctx,
									sessionID,
									"old-steer",
									store.SteerDeliveryPendingBoundary,
									now,
								)
								if !errors.Is(err, store.ErrSessionInputQueueEntryNotQueued) {
									t.Fatalf("late resolution=%v", err)
								}
							}
							claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
							if err != nil || !ok || claimed.ID != "interrupt" {
								t.Fatalf("next input=%#v/%t/%v", claimed, ok, err)
							}
						}
					},
				)
			}
		}
	})

	t.Run("Should preserve a bounded Goal input batch across new arrivals and reopen", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "prepared-goal", SessionID: sessionID, Text: "goal", QueueCap: 10, Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx,
			`UPDATE session_input_queue SET owner_kind = 'goal', dispatchable = 0 WHERE id = 'prepared-goal'`,
		); err != nil {
			t.Fatal(err)
		}
		for index, id := range []string{"batch-a", "batch-b", "batch-c"} {
			_, position, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, QueueCap: 10, Now: now.Add(time.Second),
			})
			if err != nil || position != index+1 {
				t.Fatalf("public admission position = %d, %v", position, err)
			}
		}
		current := db
		for index, want := range []string{"batch-a", "batch-b", "batch-c"} {
			peek, found, err := current.PeekNextSessionInput(ctx, sessionID)
			if err != nil || !found || peek.ID != want {
				t.Fatalf("batch selection = %q, %v, %v; want %q", peek.ID, found, err, want)
			}
			pending, err := current.ListPendingSessionInputs(ctx, sessionID)
			if err != nil || len(pending) != 3 || pending[0].ID != want {
				t.Fatalf("public batch order = %#v, %v; want first %q", pending, err, want)
			}
			at := now.Add(time.Duration(2+2*index) * time.Second)
			claimed, found, err := current.ClaimNextSessionInput(ctx, sessionID, at)
			if err != nil || !found || claimed.ID != want {
				t.Fatalf("batch claim = %q, %v, %v; want %q", claimed.ID, found, err, want)
			}
			if err := current.MarkSessionInputSent(ctx, sessionID, claimed.ID, at); err != nil {
				t.Fatal(err)
			}
			id := fmt.Sprintf("late-%d", index)
			if _, _, err := current.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, QueueCap: 10,
				Priority: 10, Now: now.Add(time.Second),
			}); err != nil {
				t.Fatal(err)
			}
			if index == 0 {
				reopened, err := OpenGlobalDB(ctx, db.path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := reopened.Close(context.Background()); err != nil {
						t.Error(err)
					}
				})
				current = reopened
			}
		}
		peek, found, err := current.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !found || peek.ID != "prepared-goal" {
			t.Fatalf("Goal after finite batch = %q, %v, %v", peek.ID, found, err)
		}
		if entry, found, err := current.ClaimNextSessionInput(
			ctx,
			sessionID,
			now.Add(10*time.Second),
		); err != nil ||
			found {
			t.Fatalf("claim must yield to Goal = %#v, %v, %v", entry, found, err)
		}
		pending, err := current.ListPendingSessionInputs(ctx, sessionID)
		if err != nil || len(pending) != 3 || pending[0].ID != "late-0" {
			t.Fatalf("deferred public inputs = %#v, %v", pending, err)
		}
		if _, _, err := current.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "interrupt", SessionID: sessionID, Text: "interrupt", TargetTurnID: "active-turn", QueueCap: 10,
			Mode: store.SessionInputQueueModeInterrupt, Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			Now: now.Add(11 * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
		peek, found, err = current.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !found || peek.ID != "interrupt" {
			t.Fatalf("interrupt must retain precedence = %#v, %v, %v", peek, found, err)
		}
		interrupt, found, err := current.ClaimNextSessionInput(ctx, sessionID, now.Add(12*time.Second))
		if err != nil || !found || interrupt.ID != "interrupt" {
			t.Fatalf("interrupt claim = %#v, %v, %v", interrupt, found, err)
		}
		if err := current.MarkSessionInputSent(ctx, sessionID, interrupt.ID, now.Add(12*time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := current.db.ExecContext(ctx,
			`UPDATE session_input_queue SET status = 'sent' WHERE id = 'prepared-goal'`,
		); err != nil {
			t.Fatal(err)
		}
		peek, found, err = current.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !found || peek.ID != "late-0" {
			t.Fatalf("deferred input after Goal = %q, %v, %v", peek.ID, found, err)
		}
	})
	t.Run("Should hide pending Goals and reject direct operator mutations", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
		for _, id := range []string{"human", "agent", "synthetic", "goal-queued", "goal-dispatching"} {
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, QueueCap: 5, Now: now,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.db.ExecContext(ctx, `UPDATE session_input_queue SET owner_kind = CASE
			WHEN id LIKE 'goal-%' THEN 'goal' WHEN id = 'human' THEN NULL ELSE id END,
			status = CASE WHEN id = 'goal-dispatching' THEN 'dispatching' ELSE 'queued' END`); err != nil {
			t.Fatal(err)
		}
		inputs, err := db.ListPendingSessionInputs(ctx, sessionID)
		if err != nil || len(inputs) != 3 || inputs[0].ID != "agent" || inputs[1].ID != "human" ||
			inputs[2].ID != "synthetic" {
			t.Fatalf("public inputs = %#v, %v", inputs, err)
		}
		summary, err := db.SessionInputQueueSummary(ctx, sessionID)
		if err != nil || summary.PendingActive != 3 || summary.PendingQueued != 3 || summary.PendingLeased != 0 {
			t.Fatalf("public queue summary = %#v, %v", summary, err)
		}
		for _, id := range []string{"goal-queued", "goal-dispatching"} {
			before, err := db.GetSessionInputQueueEntry(ctx, sessionID, id)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.CancelSessionInput(
				ctx,
				sessionID,
				id,
				now,
			); !errors.Is(
				err,
				store.ErrSessionInputQueueEntryNotFound,
			) {
				t.Fatalf("cancel Goal = %v", err)
			}
			replacement := store.SessionInputQueueInsert{
				ID:           "replacement",
				SessionID:    sessionID,
				TargetTurnID: "active-turn",
				Text:         "operator edit",
				QueueCap:     5,
				Now:          now,
			}
			if _, _, err := db.ReplaceSessionInput(
				ctx,
				sessionID,
				id,
				replacement,
			); !errors.Is(
				err,
				store.ErrSessionInputQueueEntryNotFound,
			) {
				t.Fatalf("replace Goal = %v", err)
			}
			if _, _, err := db.PromoteSessionInputToSteer(
				ctx,
				sessionID,
				id,
				replacement,
			); !errors.Is(
				err,
				store.ErrSessionInputQueueEntryNotFound,
			) {
				t.Fatalf("promote Goal = %v", err)
			}
			after, err := db.GetSessionInputQueueEntry(ctx, sessionID, id)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("Goal changed through operator mutation: %#v -> %#v, %v", before, after, err)
			}
		}
	})
	t.Run("Should clear public inputs while preserving Goal dispatch and active outcomes", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
		for _, id := range []string{"active", "human", "agent", "goal"} {
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, QueueCap: 4, Now: now,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.db.ExecContext(ctx,
			`UPDATE session_input_queue SET owner_kind = 'goal', dispatchable = 0 WHERE id = 'goal'`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx,
			`UPDATE session_input_queue SET owner_kind = 'agent' WHERE id = 'agent'`); err != nil {
			t.Fatal(err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now)
		if err != nil || !ok || claimed.ID != "active" {
			t.Fatalf("claim = %#v, %v, %v", claimed, ok, err)
		}
		result, err := db.ClearSessionInputs(ctx, store.SessionInputClearRequest{
			SessionID: sessionID, TurnID: "active-turn", ActorKind: "human", ActorID: "operator", Now: now,
		})
		if err != nil || result.Cleared != 2 || result.Generation != 1 || len(result.Inputs) != 3 {
			t.Fatalf("ClearSessionInputs() = %#v, %v", result, err)
		}
		for _, id := range []string{"human", "agent"} {
			entry, err := db.GetSessionInputQueueEntry(ctx, sessionID, id)
			if err != nil || entry.Status != store.SessionInputQueueStatusCanceled {
				t.Fatalf("cleared input = %#v, %v", entry, err)
			}
		}
		goal, err := db.GetSessionInputQueueEntry(ctx, sessionID, "goal")
		if err != nil || goal.Status != store.SessionInputQueueStatusQueued || goal.TerminalAt != nil ||
			goal.CanceledAt != nil || goal.SessionGeneration != result.Generation || goal.Dispatchable {
			t.Fatalf("preserved Goal = %#v, %v", goal, err)
		}
		traces, err := db.ListPendingSessionInputClearTraces(ctx, sessionID)
		if err != nil || len(traces) != 2 {
			t.Fatalf("clear traces = %#v, %v", traces, err)
		}
		for _, trace := range traces {
			if trace.ActorID != "operator" || trace.ActorKind != "human" || trace.TurnID != "active-turn" {
				t.Fatalf("trace attribution = %#v", trace)
			}
		}
		if err := db.ReleaseSessionInput(ctx, sessionID, claimed.ID, now); err != nil {
			t.Fatal(err)
		}
		next, found, err := db.ClaimNextSessionInput(ctx, sessionID, now)
		if err != nil || !found || next.ID != claimed.ID || next.SessionGeneration != 1 {
			t.Fatalf("released dispatch after clear = %#v, %v, %v", next, found, err)
		}
		if err := db.MarkSessionInputSent(ctx, sessionID, next.ID, now); err != nil {
			t.Fatal(err)
		}
		nextGoal, found, err := db.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !found || nextGoal.ID != "goal" {
			t.Fatalf("Goal dispatch after clear = %#v, %v, %v", nextGoal, found, err)
		}
	})
	t.Run("Should roll back clear and generation when an attributed trace cannot be persisted", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "parked", SessionID: sessionID, Text: "keep me", QueueCap: 1, Now: now,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.db.ExecContext(ctx, `CREATE TRIGGER reject_clear_trace
			BEFORE INSERT ON session_input_clear_traces BEGIN SELECT RAISE(ABORT, 'trace unavailable'); END`); err != nil {
			t.Fatal(err)
		}
		_, err := db.ClearSessionInputs(ctx, store.SessionInputClearRequest{
			SessionID: sessionID, ActorKind: "human", ActorID: "operator", Now: now,
		})
		if err == nil || !strings.Contains(err.Error(), "trace unavailable") {
			t.Fatalf("clear error = %v", err)
		}
		entry, err := db.GetSessionInputQueueEntry(ctx, sessionID, "parked")
		if err != nil || entry.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("entry after failed clear = %#v, %v", entry, err)
		}
		generation, err := db.CurrentSessionInputGeneration(ctx, sessionID)
		if err != nil || generation != 0 {
			t.Fatalf("generation after failed clear = %d, %v", generation, err)
		}
	})
	t.Run("Should preserve the resolved steer delivery through queue leasing", func(t *testing.T) {
		t.Parallel()
		for _, delivery := range []store.SteerDeliveryMode{store.SteerDeliveryInjected, store.SteerDeliveryPendingInjection, store.SteerDeliveryInterruptFallback} {
			t.Run("Should round trip "+string(delivery), func(t *testing.T) {
				t.Parallel()
				ctx := t.Context()
				db := openTestGlobalDB(t)
				sessionID := registerInputQueueSession(t, db)
				now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
				entry, err := db.StageSessionSteer(ctx, store.SessionInputQueueInsert{
					ID: "steer-delivery", SessionID: sessionID, TargetTurnID: "active-turn", Text: "change direction",
					Attachments:   []store.SessionInputAttachment{testSessionInputAttachment()},
					SteerDelivery: delivery, QueueCap: 10, Now: now,
				})
				if err != nil {
					t.Fatal(err)
				}
				if entry.SteerDelivery != delivery {
					t.Fatalf("inserted delivery = %q, want %q", entry.SteerDelivery, delivery)
				}
				claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
				if err != nil || !ok {
					t.Fatalf("ClaimNextSessionInput() = %v, %v", ok, err)
				}
				if claimed.SteerDelivery != delivery || !slices.Equal(claimed.Attachments, entry.Attachments) {
					t.Fatalf("claimed delivery = %q, want %q", claimed.SteerDelivery, delivery)
				}
			})
		}
	})
	t.Run("Should reject internal unsupported delivery before persisting a steer", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		_, err := db.StageSessionSteer(t.Context(), store.SessionInputQueueInsert{
			ID: "unsupported-steer", SessionID: sessionID, Text: "change direction", SteerDelivery: "unsupported",
			QueueCap: 10, Now: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC),
		})
		if err == nil || !strings.Contains(err.Error(), "invalid steer delivery") {
			t.Fatalf("StageSessionSteer() error = %v", err)
		}
		entries, err := db.ListPendingSessionInputs(t.Context(), sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 0 {
			t.Fatalf("persisted rejected steer: %#v", entries)
		}
	})
	t.Run("Should preserve the runtime snapshot through queue leasing", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
		thinkingEnabled := true
		inputRuntime := store.SessionInputRuntime{
			Provider:        "openai",
			Model:           "gpt-5.6",
			ReasoningEffort: "high",
			Speed:           "fast",
			ACPOptions: []store.SessionACPOptionSelection{
				{ID: "thinking", BoolValue: &thinkingEnabled},
				{ID: "context", ValueID: "large"},
			},
		}
		wantRuntime := store.SessionInputRuntime{
			Provider:        "openai",
			Model:           "gpt-5.6",
			ReasoningEffort: "high",
			Speed:           "fast",
			ACPOptions: []store.SessionACPOptionSelection{
				{ID: "context", ValueID: "large"},
				{ID: "thinking", BoolValue: &thinkingEnabled},
			},
		}

		enqueued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID:                "inq-runtime-snapshot",
			SessionID:         sessionID,
			Mode:              store.SessionInputQueueModeQueue,
			Text:              "queued input",
			Runtime:           inputRuntime,
			SessionGeneration: 0,
			QueueCap:          10,
			Now:               now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		if got, want := enqueued.Runtime, wantRuntime; !reflect.DeepEqual(got, want) {
			t.Fatalf("enqueued runtime = %#v, want %#v", got, want)
		}

		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil {
			t.Fatalf("ClaimNextSessionInput() error = %v", err)
		}
		if !ok {
			t.Fatal("ClaimNextSessionInput() ok = false, want true")
		}
		if got, want := claimed.Runtime, wantRuntime; !reflect.DeepEqual(got, want) {
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

	// Invariant: resolved mentions and human ingress persist on the admission and its bound queue row;
	// none persist as [] and unknown authorship as not human.
	t.Run("Should preserve mentions from admission through queue lease", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		const want = `[{"kind":"file","token":"@docs/a.md","target":"docs/a.md","label":"a.md","start":5,"end":15}]`
		request := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "mentions", now)
		request.MentionsJSON = "  " + want + "\n"
		request.HumanIngress = true
		admission, entry, _, created, err := globalDB.EnqueueAdmittedSessionInput(ctx, request,
			store.SessionInputQueueInsert{
				ID: "inq-mentions", SessionID: sessionID, Text: request.AuthoredText, QueueCap: 10, Now: now,
			})
		if err != nil || !created {
			t.Fatalf("EnqueueAdmittedSessionInput() = created %v, error %v", created, err)
		}
		if admission.MentionsJSON != want || entry.MentionsJSON != want || !admission.HumanIngress ||
			!entry.HumanIngress {
			t.Fatalf("admitted mentions = %s / %s (human %v / %v), want %s from a human",
				admission.MentionsJSON, entry.MentionsJSON, admission.HumanIngress, entry.HumanIngress, want)
		}
		loaded, found, err := globalDB.LookupSessionPromptAdmission(
			ctx, request.WorkspaceID, sessionID, request.IdempotencyKey,
		)
		if err != nil || !found || loaded.MentionsJSON != want || !loaded.HumanIngress {
			t.Fatalf("LookupSessionPromptAdmission() = %s, %v, %v", loaded.MentionsJSON, found, err)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil || !ok || claimed.MentionsJSON != want || !claimed.HumanIngress {
			t.Fatalf("claimed mentions = %s, ok %v, error %v", claimed.MentionsJSON, ok, err)
		}

		plain, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-no-mentions", SessionID: sessionID, Text: "plain", QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		var raw string
		if err := globalDB.db.QueryRowContext(ctx,
			`SELECT mentions_json FROM session_input_queue WHERE id = ?`, plain.ID,
		).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		if raw != store.EmptyMentionsJSON || plain.MentionsJSON != store.EmptyMentionsJSON || plain.HumanIngress {
			t.Fatalf("mention-free row = raw %q, entry %q, want []", raw, plain.MentionsJSON)
		}
		if _, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-bad-mentions", SessionID: sessionID, Text: "bad", MentionsJSON: `{"kind":"file"}`,
			QueueCap: 10, Now: now,
		}); err == nil {
			t.Fatal("EnqueueSessionInput() accepted non-array mentions json")
		}
	})

	t.Run("Should preserve attachments through queue insert and lease", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
		want := []store.SessionInputAttachment{testSessionInputAttachment()}
		enqueued, _, err := globalDB.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "inq-attachments", SessionID: sessionID,
			Mode: store.SessionInputQueueModeQueue, Text: "",
			Attachments: want, SessionGeneration: 0, QueueCap: 10, Now: now,
		})
		if err != nil {
			t.Fatalf("EnqueueSessionInput() error = %v", err)
		}
		if !slices.Equal(enqueued.Attachments, want) {
			t.Fatalf("enqueued attachments = %#v, want %#v", enqueued.Attachments, want)
		}
		loaded, err := globalDB.GetSessionInputQueueEntry(ctx, sessionID, enqueued.ID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry() error = %v", err)
		}
		if !slices.Equal(loaded.Attachments, want) {
			t.Fatalf("loaded attachments = %#v, want %#v", loaded.Attachments, want)
		}
		pending, err := globalDB.ListPendingSessionInputs(ctx, sessionID)
		if err != nil {
			t.Fatalf("ListPendingSessionInputs() error = %v", err)
		}
		if len(pending) != 1 || !slices.Equal(pending[0].Attachments, want) {
			t.Fatalf("pending attachments = %#v, want %#v", pending, want)
		}
		claimed, ok, err := globalDB.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Second))
		if err != nil || !ok {
			t.Fatalf("ClaimNextSessionInput() = ok %v, error %v", ok, err)
		}
		if !slices.Equal(claimed.Attachments, want) {
			t.Fatalf("claimed attachments = %#v, want %#v", claimed.Attachments, want)
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
		if _, ok, err := globalDB.ReserveSessionSteer(
			ctx,
			sessionID,
			"steer-new",
			now.Add(2*time.Second),
		); err != nil ||
			!ok {
			t.Fatalf("ReserveSessionSteer(new) = %v, %v", ok, err)
		}
		if _, err := globalDB.ResolveSessionSteer(
			ctx,
			sessionID,
			"steer-new",
			store.SteerDeliveryInterruptFallback,
			now.Add(2*time.Second),
		); err != nil {
			t.Fatalf("ResolveSessionSteer(new) error = %v", err)
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
		leased, ok, err := globalDB.ReserveSessionSteer(ctx, sessionID, "steer-dispatching", now.Add(time.Second))
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
			IdempotencyKey: "idem-replace", Attachments: []store.SessionInputAttachment{testSessionInputAttachment()},
			QueueCap: 10, Now: now.Add(time.Second),
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
		if !slices.Equal(replacement.Attachments, replacementRequest.Attachments) {
			t.Fatalf("replacement attachments = %#v, want %#v", replacement.Attachments, replacementRequest.Attachments)
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
		if !slices.Equal(replayed.Attachments, replacementRequest.Attachments) {
			t.Fatalf("replayed attachments = %#v, want %#v", replayed.Attachments, replacementRequest.Attachments)
		}
		conflictingRequest := replacementRequest
		conflictingRequest.Attachments = append([]store.SessionInputAttachment(nil), replacementRequest.Attachments...)
		conflictingRequest.Attachments[0].Name = "different-shot.png"
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

// Invariant: queue capacity, order, and supersede depend on who authored the input.
func TestGlobalDBSessionInputQueueOriginPolicy(t *testing.T) {
	agentOrigin := func(sender string) json.RawMessage {
		return json.RawMessage(`{"kind":"session","session_id":"` + sender + `","workspace_id":"ws","hop":1}`)
	}
	enqueue := func(
		t *testing.T, db *GlobalDB, req store.SessionInputQueueInsert,
	) (store.SessionInputQueueEntry, int) {
		t.Helper()
		if req.QueueCap == 0 {
			req.QueueCap = 10
		}
		entry, position, err := db.EnqueueSessionInput(testutil.Context(t), req)
		if err != nil {
			t.Fatalf("EnqueueSessionInput(%s) error = %v", req.ID, err)
		}
		return entry, position
	}
	pendingIDs := func(t *testing.T, db *GlobalDB, sessionID string) []string {
		t.Helper()
		pending, err := db.ListPendingSessionInputs(testutil.Context(t), sessionID)
		if err != nil {
			t.Fatalf("ListPendingSessionInputs() error = %v", err)
		}
		ids := make([]string, 0, len(pending))
		for index := range pending {
			ids = append(ids, pending[index].ID)
		}
		return ids
	}

	t.Run("Should dispatch human input before agent input and wakes and report its rank", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		enqueue(t, db, store.SessionInputQueueInsert{
			ID: "wake", SessionID: sessionID, MessageID: "wake-message", TurnID: "wake-turn", Priority: 1,
			Text: "wake", OwnerKind: store.SessionInputOwnerSynthetic,
			SyntheticPrompt: &store.SessionInputSyntheticPrompt{RunID: "run", Metadata: []byte(`{"reason":"reply"}`)},
			Now:             now,
		})
		agent, agentPosition := enqueue(t, db, store.SessionInputQueueInsert{
			ID: "agent", SessionID: sessionID, Text: "status", Origin: agentOrigin("sess-a"), Now: now.Add(time.Second),
		})
		human, humanPosition := enqueue(t, db, store.SessionInputQueueInsert{
			ID: "human", SessionID: sessionID, Text: "operator", Now: now.Add(2 * time.Second),
		})
		if agent.OriginClass != store.SessionInputOriginClassAgent || agent.OriginSessionID != "sess-a" ||
			human.OriginClass != store.SessionInputOriginClassHuman {
			t.Fatalf("origin classes = %q/%q, %q", agent.OriginClass, agent.OriginSessionID, human.OriginClass)
		}
		if agentPosition != 1 || humanPosition != 1 {
			t.Fatalf(
				"positions = agent %d, human %d; want each ranked first when enqueued",
				agentPosition,
				humanPosition,
			)
		}
		if got, want := pendingIDs(t, db, sessionID), []string{"human", "agent", "wake"}; !slices.Equal(got, want) {
			t.Fatalf("pending order = %v, want %v", got, want)
		}
		for _, want := range []string{"human", "agent", "wake"} {
			peek, ok, err := db.PeekNextSessionInput(ctx, sessionID)
			if err != nil || !ok || peek.ID != want {
				t.Fatalf("peek = %q/%v/%v, want %q", peek.ID, ok, err, want)
			}
			claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Minute))
			if err != nil || !ok || claimed.ID != want {
				t.Fatalf("claim = %q/%v/%v, want %q", claimed.ID, ok, err, want)
			}
			if err := db.MarkSessionInputSent(ctx, sessionID, want, now.Add(time.Minute)); err != nil {
				t.Fatalf("MarkSessionInputSent(%s) error = %v", want, err)
			}
		}
	})

	t.Run("Should cap human input separately from agent input and wakes", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 10, 0, 0, time.UTC)
		enqueue(t, db, store.SessionInputQueueInsert{
			ID: "agent-1", SessionID: sessionID, Text: "one", Origin: agentOrigin("sess-a"), QueueCap: 1, Now: now,
		})
		_, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "agent-2", SessionID: sessionID, Text: "two", Origin: agentOrigin("sess-b"), QueueCap: 1,
			Now: now.Add(time.Second),
		})
		if !errors.Is(err, store.ErrSessionInputQueueFull) {
			t.Fatalf("second agent input error = %v, want ErrSessionInputQueueFull", err)
		}
		_, position := enqueue(t, db, store.SessionInputQueueInsert{
			ID: "human", SessionID: sessionID, Text: "operator", QueueCap: 1, Now: now.Add(2 * time.Second),
		})
		if position != 1 {
			t.Fatalf("human position = %d, want 1 ahead of the agent input", position)
		}
	})

	t.Run("Should supersede queued steers only from the same sender", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 20, 0, 0, time.UTC)
		stage := func(id string, origin json.RawMessage, at time.Time) store.SessionInputQueueEntry {
			t.Helper()
			entry, err := db.StageSessionSteer(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, TargetTurnID: "turn-live", Text: id, Origin: origin,
				QueueCap: 10, Now: at,
			})
			if err != nil {
				t.Fatalf("StageSessionSteer(%s) error = %v", id, err)
			}
			return entry
		}
		stage("human-steer", nil, now)
		if first := stage("agent-steer-1", agentOrigin("sess-a"), now.Add(time.Second)); len(first.SupersededIDs) != 0 {
			t.Fatalf("agent steer superseded %v, want the human steer kept", first.SupersededIDs)
		}
		second := stage("agent-steer-2", agentOrigin("sess-a"), now.Add(2*time.Second))
		if !slices.Equal(second.SupersededIDs, []string{"agent-steer-1"}) {
			t.Fatalf("superseded = %v, want only the same sender's older steer", second.SupersededIDs)
		}
		human, err := db.GetSessionInputQueueEntry(ctx, sessionID, "human-steer")
		if err != nil || human.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("human steer = %q, %v; want queued", human.Status, err)
		}
	})

	t.Run("Should degrade an agent steer fallback to after-turn delivery behind human input", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 30, 0, 0, time.UTC)
		enqueue(t, db, store.SessionInputQueueInsert{ID: "human", SessionID: sessionID, Text: "operator", Now: now})
		if _, err := db.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID: "agent-steer", SessionID: sessionID, TargetTurnID: "turn-live", Text: "urgent",
			Origin: agentOrigin("sess-a"), QueueCap: 10, Now: now.Add(time.Second),
		}); err != nil {
			t.Fatalf("StageSessionSteer() error = %v", err)
		}
		if _, ok, err := db.ReserveSessionSteer(
			ctx,
			sessionID,
			"agent-steer",
			now.Add(time.Second),
		); err != nil ||
			!ok {
			t.Fatalf("ReserveSessionSteer() = %v, %v", ok, err)
		}
		resolved, err := db.ResolveSessionSteer(
			ctx, sessionID, "agent-steer", store.SteerDeliveryInterruptFallback, now.Add(time.Second),
		)
		if err != nil {
			t.Fatalf("ResolveSessionSteer() error = %v", err)
		}
		if resolved.Delivery != store.SessionInputDeliveryAfterTurn ||
			resolved.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("resolved = %q/%q, want queued after_turn", resolved.Status, resolved.Delivery)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
		if err != nil || !ok || claimed.ID != "human" {
			t.Fatalf("claim = %q/%v/%v, want the human input first", claimed.ID, ok, err)
		}
	})

	t.Run("Should promote an unchanged agent row as human steering and refuse edits", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 40, 0, 0, time.UTC)
		agent, _ := enqueue(t, db, store.SessionInputQueueInsert{
			ID: "agent", SessionID: sessionID, Text: "child status", Origin: agentOrigin("sess-a"), Now: now,
		})
		if _, err := db.StageSessionSteer(ctx, store.SessionInputQueueInsert{
			ID: "human-steer", SessionID: sessionID, TargetTurnID: "turn-live", Text: "older human steer",
			QueueCap: 10, Now: now,
		}); err != nil {
			t.Fatalf("StageSessionSteer() error = %v", err)
		}
		promotion := func(id, text string) store.SessionInputQueueInsert {
			return store.SessionInputQueueInsert{
				ID: id, TargetTurnID: "turn-live", Text: text, MessageID: id, IdempotencyKey: id,
				QueueCap: 10, Now: now.Add(time.Second),
			}
		}
		if _, _, err := db.ReplaceSessionInput(ctx, sessionID, agent.ID, promotion("replace", "edited")); !errors.Is(
			err, store.ErrSessionInputAgentAuthored,
		) {
			t.Fatalf("replace error = %v, want ErrSessionInputAgentAuthored", err)
		}
		if _, _, err := db.PromoteSessionInputToSteer(
			ctx,
			sessionID,
			agent.ID,
			promotion("edit", "edited"),
		); !errors.Is(
			err,
			store.ErrSessionInputAgentAuthored,
		) {
			t.Fatalf("promote with edited text error = %v, want ErrSessionInputAgentAuthored", err)
		}
		promoted, created, err := db.PromoteSessionInputToSteer(
			ctx, sessionID, agent.ID, promotion("promoted", "child status"),
		)
		if err != nil || !created {
			t.Fatalf("PromoteSessionInputToSteer() = %v, %v", created, err)
		}
		if promoted.OriginClass != store.SessionInputOriginClassHuman || promoted.OriginSessionID != "" ||
			string(promoted.Origin) != string(agent.Origin) {
			t.Fatalf("promoted origin = %q/%q/%s, want human-initiated with sender attribution",
				promoted.OriginClass, promoted.OriginSessionID, promoted.Origin)
		}
		if !slices.Equal(promoted.SupersededIDs, []string{"human-steer"}) {
			t.Fatalf("superseded = %v, want the older human steer", promoted.SupersededIDs)
		}
	})

	admitAgent := func(
		t *testing.T, db *GlobalDB, sessionID, id, sender string, supersede bool, at time.Time,
	) (store.SessionPromptAdmission, store.SessionInputQueueEntry, bool) {
		t.Helper()
		admission := promptAdmissionRequest("ws-input-queue-workspace", sessionID, id, at)
		admission.Origin = agentOrigin(sender)
		admitted, entry, _, created, err := db.EnqueueAdmittedSessionInput(
			testutil.Context(t), admission, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, Supersede: supersede, QueueCap: 10, Now: at,
			},
		)
		if err != nil {
			t.Fatalf("EnqueueAdmittedSessionInput(%s) error = %v", id, err)
		}
		return admitted, entry, created
	}
	statusOf := func(t *testing.T, db *GlobalDB, sessionID, id string) store.SessionInputQueueEntry {
		t.Helper()
		entry, err := db.GetSessionInputQueueEntry(testutil.Context(t), sessionID, id)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry(%s) error = %v", id, err)
		}
		return entry
	}

	t.Run("Should supersede only the same sender's undispatched messages and replay idempotently", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 13, 0, 0, 0, time.UTC)
		admitAgent(t, db, sessionID, "a1", "sess-a", false, now)
		admitAgent(t, db, sessionID, "a2", "sess-a", false, now.Add(time.Second))
		admitAgent(t, db, sessionID, "b1", "sess-b", false, now.Add(2*time.Second))
		enqueue(t, db, store.SessionInputQueueInsert{
			ID: "human", SessionID: sessionID, Text: "operator", Now: now.Add(3 * time.Second),
		})
		admission, latest, created := admitAgent(t, db, sessionID, "a3", "sess-a", true, now.Add(4*time.Second))
		if !created || !slices.Equal(latest.SupersededIDs, []string{"a1", "a2"}) {
			t.Fatalf("supersede = created %v, ids %v; want a1 and a2", created, latest.SupersededIDs)
		}
		if admission.Result == nil || admission.Result.CanceledQueuedEntries != 2 {
			t.Fatalf("admission result = %#v, want 2 canceled entries", admission.Result)
		}
		for id, want := range map[string]string{
			"a1": store.SessionInputQueueStatusCanceled, "a2": store.SessionInputQueueStatusCanceled,
			"b1": store.SessionInputQueueStatusQueued, "human": store.SessionInputQueueStatusQueued,
			"a3": store.SessionInputQueueStatusQueued,
		} {
			if got := statusOf(t, db, sessionID, id); got.Status != want {
				t.Fatalf("%s status = %q, want %q", id, got.Status, want)
			}
		}
		replayed, entry, created := admitAgent(t, db, sessionID, "a3", "sess-a", true, now.Add(5*time.Second))
		if created || entry.ID != "a3" || len(entry.SupersededIDs) != 0 ||
			replayed.Result == nil || replayed.Result.CanceledQueuedEntries != 2 {
			t.Fatalf(
				"replay = created %v, entry %q/%v, result %#v",
				created, entry.ID, entry.SupersededIDs, replayed.Result,
			)
		}
	})

	t.Run("Should claim a consecutive batch behind the head and stamp every entry with one turn", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 13, 10, 0, 0, time.UTC)
		for index, id := range []string{"a1", "a2", "stop", "a4"} {
			enqueue(t, db, store.SessionInputQueueInsert{
				ID: id, SessionID: sessionID, Text: id, Origin: agentOrigin("sess-a"),
				Now: now.Add(time.Duration(index) * time.Second),
			})
		}
		head, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Minute))
		if err != nil || !ok || head.ID != "a1" {
			t.Fatalf("head = %q/%v/%v, want a1", head.ID, ok, err)
		}
		accept := func(entry *store.SessionInputQueueEntry) bool { return entry.ID != "stop" }
		batch, err := db.ClaimSessionInputBatch(ctx, sessionID, 7, accept, now.Add(time.Minute))
		if err != nil || len(batch) != 1 || batch[0].ID != "a2" ||
			batch[0].Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("batch = %#v, %v; want a2 leased and the batch ended at the refused entry", batch, err)
		}
		for _, id := range []string{"a1", "a2"} {
			err := db.MarkSessionInputSentInTurn(ctx, sessionID, id, "turn-batch", now.Add(time.Minute))
			if err != nil {
				t.Fatalf("MarkSessionInputSentInTurn(%s) error = %v", id, err)
			}
			if got := statusOf(t, db, sessionID, id); got.Status != store.SessionInputQueueStatusSent ||
				got.TurnID != "turn-batch" {
				t.Fatalf("%s = %q/%q, want sent in turn-batch", id, got.Status, got.TurnID)
			}
		}
		enqueue(t, db, store.SessionInputQueueInsert{
			ID: "human", SessionID: sessionID, Text: "operator", Now: now.Add(time.Hour),
		})
		acceptAgents := func(entry *store.SessionInputQueueEntry) bool {
			return entry.OriginClass == store.SessionInputOriginClassAgent
		}
		if batch, err := db.ClaimSessionInputBatch(ctx, sessionID, 7, acceptAgents, now.Add(time.Hour)); err != nil ||
			len(batch) != 0 {
			t.Fatalf("batch with operator input first = %#v, %v; want none merged past it", batch, err)
		}
	})

	t.Run("Should promote a queued agent message in place and never past operator input", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 13, 20, 0, 0, time.UTC)
		_, agent, _ := admitAgent(t, db, sessionID, "a1", "sess-a", false, now)
		admitAgent(t, db, sessionID, "a2", "sess-a", false, now.Add(time.Second))
		enqueue(t, db, store.SessionInputQueueInsert{ID: "human", SessionID: sessionID, Text: "operator", Now: now})
		if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
			ctx, sessionID, "a1", "turn-live", now,
		); err != nil || promoted {
			t.Fatalf("promotion with operator input waiting = %v, %v; want refused", promoted, err)
		}
		if _, err := db.CancelSessionInput(ctx, sessionID, "human", now); err != nil {
			t.Fatal(err)
		}
		staged, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(ctx, sessionID, "a1", "turn-live", now)
		if err != nil || !promoted {
			t.Fatalf("PromoteQueuedAgentSessionInputToSteer() = %v, %v", promoted, err)
		}
		if staged.Mode != store.SessionInputQueueModeSteer || staged.TargetTurnID != "turn-live" ||
			staged.Dispatchable || staged.MessageID != agent.MessageID || staged.EventID != agent.EventID ||
			staged.PromptAdmissionID != agent.PromptAdmissionID || staged.OriginClass != store.SessionInputOriginClassAgent {
			t.Fatalf("staged = %#v, want an in-place steer keeping %#v identity", staged, agent)
		}
		if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
			ctx, sessionID, "a2", "turn-live", now,
		); err != nil || promoted {
			t.Fatalf("second promotion for the same sender = %v, %v; want the steer slot respected", promoted, err)
		}
		if reverted, err := db.RevertPromotedAgentSessionInput(ctx, sessionID, "a1", now); err != nil || !reverted {
			t.Fatalf("RevertPromotedAgentSessionInput() = %v, %v", reverted, err)
		}
		if got := statusOf(t, db, sessionID, "a1"); got.Mode != store.SessionInputQueueModeQueue ||
			got.Delivery != store.SessionInputDeliveryAfterTurn || !got.Dispatchable ||
			got.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("reverted = %#v, want queued after-turn delivery", got)
		}
		// A reservation commits the admission dispatch; reverting the reserved row must reopen it, or the
		// ordinary claim that follows fails as an indeterminate dispatch.
		if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
			ctx, sessionID, "a1", "turn-live", now,
		); err != nil || !promoted {
			t.Fatalf("second promotion = %v, %v", promoted, err)
		}
		if reserved, ok, err := db.ReserveSessionSteer(ctx, sessionID, "a1", now); err != nil || !ok ||
			reserved.Status != store.SessionInputQueueStatusDispatching {
			t.Fatalf("ReserveSessionSteer() = %#v, %v, %v; want a dispatching reservation", reserved, ok, err)
		}
		if reverted, err := db.RevertPromotedAgentSessionInput(ctx, sessionID, "a1", now); err != nil || !reverted {
			t.Fatalf("revert of a reserved promotion = %v, %v", reverted, err)
		}
		// The reservation was not a turn claim, so the first ordinary claim reads as attempt 1.
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(time.Minute))
		if err != nil || !ok || claimed.ID != "a1" || claimed.AttemptCount != 1 {
			t.Fatalf("ClaimNextSessionInput() = %q/%d, %v, %v; want the reverted message claimed as attempt 1",
				claimed.ID, claimed.AttemptCount, ok, err)
		}
	})

	t.Run("Should recover steers staged before a restart instead of leaving them undispatchable", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 13, 25, 0, 0, time.UTC)
		admitAgent(t, db, sessionID, "a1", "sess-a", false, now)
		if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
			ctx, sessionID, "a1", "turn-before-restart", now,
		); err != nil || !promoted {
			t.Fatalf("promotion = %v, %v", promoted, err)
		}
		enqueue(t, db, store.SessionInputQueueInsert{
			ID: "human-steer", SessionID: sessionID, Text: "operator guidance", Mode: store.SessionInputQueueModeSteer,
			Delivery: store.SessionInputDeliveryInterruptThenPrompt, TargetTurnID: "turn-before-restart", Now: now,
		})
		if got := statusOf(t, db, sessionID, "human-steer"); got.Dispatchable {
			t.Fatalf("staged human steer = %#v, want undispatchable before recovery", got)
		}
		if _, err := db.FailStaleDispatchingSessionInputs(ctx, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if got := statusOf(t, db, sessionID, "a1"); got.Mode != store.SessionInputQueueModeQueue ||
			got.Delivery != store.SessionInputDeliveryAfterTurn || !got.Dispatchable ||
			got.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("recovered promotion = %#v, want plain after-turn delivery", got)
		}
		if got := statusOf(t, db, sessionID, "human-steer"); !got.Dispatchable ||
			got.Status != store.SessionInputQueueStatusQueued {
			t.Fatalf("recovered human steer = %#v, want dispatchable", got)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Minute))
		if err != nil || !ok || claimed.ID != "human-steer" {
			t.Fatalf("ClaimNextSessionInput() = %q, %v, %v; want the operator steer first", claimed.ID, ok, err)
		}
	})

	t.Run("Should keep the sender's message identity when the operator promotes an agent row", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 13, 30, 0, 0, time.UTC)
		_, agent, _ := admitAgent(t, db, sessionID, "a1", "sess-a", false, now)
		request := store.SessionInputQueueInsert{
			ID: "promoted", TargetTurnID: "turn-live", Text: agent.Text, MessageID: "operator-message",
			IdempotencyKey: "operator-key", QueueCap: 10, Now: now.Add(time.Second),
		}
		promoted, created, err := db.PromoteSessionInputToSteer(ctx, sessionID, agent.ID, request)
		if err != nil || !created || promoted.MessageID != agent.MessageID || promoted.EventID != agent.EventID {
			t.Fatalf("promoted = %q/%q, created %v, %v; want sender message %q event %q",
				promoted.MessageID, promoted.EventID, created, err, agent.MessageID, agent.EventID)
		}
		replayed, created, err := db.PromoteSessionInputToSteer(ctx, sessionID, agent.ID, request)
		if err != nil || created || replayed.ID != promoted.ID {
			t.Fatalf("replayed promotion = %q, created %v, %v; want the same steer", replayed.ID, created, err)
		}
	})

	t.Run("Should settle sent receipts idempotently and fail leases stranded by a restart", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 16, 12, 50, 0, 0, time.UTC)
		enqueue(
			t,
			db,
			store.SessionInputQueueInsert{ID: "sent", SessionID: sessionID, Text: "one", QueueCap: 1, Now: now},
		)
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now); err != nil || !ok {
			t.Fatalf("claim(sent) = %v, %v", ok, err)
		}
		for range 2 {
			if err := db.MarkSessionInputSent(ctx, sessionID, "sent", now); err != nil {
				t.Fatalf("MarkSessionInputSent() error = %v, want idempotent success", err)
			}
		}
		enqueue(
			t,
			db,
			store.SessionInputQueueInsert{ID: "stranded", SessionID: sessionID, Text: "two", QueueCap: 1, Now: now},
		)
		if _, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now); err != nil || !ok {
			t.Fatalf("claim(stranded) = %v, %v", ok, err)
		}
		failed, err := db.FailStaleDispatchingSessionInputs(ctx, now.Add(time.Minute))
		if err != nil || failed != 1 {
			t.Fatalf("FailStaleDispatchingSessionInputs() = %d, %v; want 1", failed, err)
		}
		stranded, err := db.GetSessionInputQueueEntry(ctx, sessionID, "stranded")
		if err != nil || stranded.Status != store.SessionInputQueueStatusFailed ||
			stranded.FailureSummary != store.SessionInputStaleDispatchSummary {
			t.Fatalf(
				"stranded = %q/%q, %v; want failed by restart recovery",
				stranded.Status,
				stranded.FailureSummary,
				err,
			)
		}
		enqueue(
			t,
			db,
			store.SessionInputQueueInsert{ID: "after", SessionID: sessionID, Text: "three", QueueCap: 1, Now: now},
		)
	})
}

const humanTurnAdmissionIndex = "idx_session_prompt_admissions_human_turn"

func TestGlobalDBSessionPromptAdmissionMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should create the prompt admission schema on a fresh database", func(t *testing.T) {
		globalDB := openFreshTestGlobalDB(t)
		assertTablesPresent(t, globalDB.db, "session_prompt_admissions", "session_input_queue")
		assertTableHasColumns(t, globalDB.db, "session_input_queue", []string{
			"prompt_admission_id",
			"message_id",
			"idempotency_key",
			"turn_id",
			"event_id",
			"attachments_json",
			"steer_delivery",
		})
		assertTableHasColumns(t, globalDB.db, "session_prompt_admissions", []string{
			"attachments_json",
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
			humanTurnAdmissionIndex,
		)
	})

	fixture := openSessionPromptAdmissionMigrationFixture(t)

	t.Run(
		"Should search committed human admissions by session turn on fresh and upgraded databases",
		func(t *testing.T) {
			t.Parallel()
			for name, globalDB := range map[string]*GlobalDB{
				"fresh":    openFreshTestGlobalDB(t),
				"upgraded": openSessionPromptAdmissionMigrationCopy(t, fixture.path),
			} {
				// HumanSessionTurn from queries/session_input.sql.
				plan := explainQueryPlan(t, globalDB.db, `SELECT EXISTS (SELECT 1 FROM session_input_queue AS queued
				WHERE queued.session_id = ?1 AND queued.turn_id = ?2 AND queued.origin_class = 'human'
				AND queued.status IN ('dispatching','sent')
				UNION ALL SELECT 1 FROM session_prompt_admissions AS admitted
				WHERE admitted.session_id = ?1 AND admitted.turn_id = ?2
				AND admitted.human_ingress = 1 AND admitted.dispatch_committed_at IS NOT NULL)`, "sess", "turn")
				if !strings.Contains(
					plan,
					"SEARCH admitted USING INDEX "+humanTurnAdmissionIndex+" (session_id=? AND turn_id=?)",
				) {
					t.Fatalf("%s plan = %q, want a search of %s", name, plan, humanTurnAdmissionIndex)
				}
			}
		},
	)

	t.Run("Should preserve populated runtime rows through the 00094 table rebuild", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		reopened := openSessionPromptAdmissionMigrationCopy(t, fixture.path)
		workspaceID := fixture.runtimeWorkspaceID
		const (
			sessionID   = "sess-before-00094"
			admissionID = "admission-before-00094"
			queueID     = "queue-before-00094"
		)

		var provider, model, effort, speed, selectedProvider, selectedModel string
		var selectedEffort, selectedSpeed, acpOptionsJSON, selectedACPOptionsJSON string
		if err := reopened.db.QueryRowContext(ctx, `SELECT
			provider, model, reasoning_effort, speed,
			selected_provider, selected_model, selected_reasoning_effort, selected_speed,
			acp_options_json, selected_acp_options_json
			FROM sessions WHERE id = ?`, sessionID).Scan(
			&provider,
			&model,
			&effort,
			&speed,
			&selectedProvider,
			&selectedModel,
			&selectedEffort,
			&selectedSpeed,
			&acpOptionsJSON,
			&selectedACPOptionsJSON,
		); err != nil {
			t.Fatalf("query migrated session error = %v", err)
		}
		if provider != "cursor" || model != "grok-4.6" || effort != "high" || speed != "fast" ||
			selectedProvider != "cursor" || selectedModel != "grok-4.6" || selectedEffort != "high" ||
			selectedSpeed != "fast" || acpOptionsJSON != "[]" || selectedACPOptionsJSON != "[]" {
			t.Fatalf(
				"migrated session runtime = %q/%q/%q/%q selected=%q/%q/%q/%q options=%s/%s",
				provider,
				model,
				effort,
				speed,
				selectedProvider,
				selectedModel,
				selectedEffort,
				selectedSpeed,
				acpOptionsJSON,
				selectedACPOptionsJSON,
			)
		}

		var admissionProvider, admissionModel, admissionEffort, admissionSpeed, admissionOptions string
		if err := reopened.db.QueryRowContext(ctx, `SELECT
			runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed, runtime_acp_options_json
			FROM session_prompt_admissions WHERE id = ?`, admissionID).Scan(
			&admissionProvider,
			&admissionModel,
			&admissionEffort,
			&admissionSpeed,
			&admissionOptions,
		); err != nil {
			t.Fatalf("query migrated prompt admission error = %v", err)
		}
		if admissionProvider != "cursor" || admissionModel != "grok-4.6" || admissionEffort != "high" ||
			admissionSpeed != "fast" || admissionOptions != "[]" {
			t.Fatalf(
				"migrated admission runtime = %q/%q/%q/%q options=%s",
				admissionProvider,
				admissionModel,
				admissionEffort,
				admissionSpeed,
				admissionOptions,
			)
		}

		entry, err := reopened.GetSessionInputQueueEntry(ctx, sessionID, queueID)
		if err != nil {
			t.Fatalf("GetSessionInputQueueEntry(migrated) error = %v", err)
		}
		if got, want := entry.Runtime, (store.SessionInputRuntime{
			Provider: "cursor", Model: "grok-4.6", ReasoningEffort: "high", Speed: "fast",
		}); !reflect.DeepEqual(got, want) {
			t.Fatalf("migrated queue runtime = %#v, want %#v", got, want)
		}
		if entry.SteerDelivery != "" {
			t.Fatalf("pre-steering row delivery = %q, want unset", entry.SteerDelivery)
		}
		if entry.PromptAdmissionID != admissionID || entry.MessageID != "message-before-00094" ||
			entry.IdempotencyKey != "idem-before-00094" || entry.TurnID != "turn-before-00094" ||
			entry.EventID != "event-before-00094" {
			t.Fatalf("migrated queue identity = %#v, want preserved admission identity", entry)
		}

		if _, err := reopened.db.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, workspaceID); err != nil {
			t.Fatalf("delete workspace after 00094 upgrade error = %v", err)
		}
		var remainingSessions int
		if err := reopened.db.QueryRowContext(
			ctx,
			`SELECT COUNT(*) FROM sessions WHERE id = ?`,
			sessionID,
		).Scan(&remainingSessions); err != nil {
			t.Fatalf("count sessions after workspace delete error = %v", err)
		}
		if remainingSessions != 0 {
			t.Fatalf("sessions after workspace delete = %d, want 0", remainingSessions)
		}
	})

	t.Run("Should require queue admissions to exist and clear deleted receipts", func(t *testing.T) {
		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
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
		t.Parallel()
		database := openSessionPromptAdmissionMigrationCopy(t, fixture.path)

		t.Run("Should preserve legacy queue values and clear orphaned receipt links", func(t *testing.T) {
			t.Parallel()
			legacy, err := database.GetSessionInputQueueEntry(
				testutil.Context(t),
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
			}); !reflect.DeepEqual(got, want) {
				t.Fatalf("legacy queue runtime = %#v, want %#v", got, want)
			}
			orphaned, err := database.GetSessionInputQueueEntry(
				testutil.Context(t),
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
			t.Parallel()
			status, err := store.Status(testutil.Context(t), database.db, MigrationStream())
			if err != nil {
				t.Fatalf("Status(global) error = %v", err)
			}
			assertCompleteMigrationStream(t, status, MigrationStream())
		})

		t.Run("Should enforce receipt constraints for migrated queue entries", func(t *testing.T) {
			t.Parallel()
			database := openSessionPromptAdmissionMigrationCopy(t, fixture.path)
			admissionReq := promptAdmissionRequest(
				"ws-input-queue-workspace",
				fixture.sessionID,
				"migration",
				fixture.now,
			)
			admission, created, err := database.ClaimSessionPromptAdmission(testutil.Context(t), admissionReq)
			if err != nil {
				t.Fatalf("ClaimSessionPromptAdmission() error = %v", err)
			}
			if !created {
				t.Fatal("ClaimSessionPromptAdmission() created = false, want true")
			}
			_, err = database.db.ExecContext(
				testutil.Context(t),
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
			_, entry, _, created, err := database.EnqueueAdmittedSessionInput(
				testutil.Context(t),
				admissionReq,
				queueReq,
			)
			if err != nil {
				t.Fatalf("EnqueueAdmittedSessionInput() error = %v", err)
			}
			if !created || entry.PromptAdmissionID != admission.ID {
				t.Fatalf("admitted queue entry = %#v, created=%v, want admission %q", entry, created, admission.ID)
			}
			_, err = database.db.ExecContext(
				testutil.Context(t),
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
			if _, err := database.db.ExecContext(
				testutil.Context(t),
				`DELETE FROM session_prompt_admissions WHERE id = ?`,
				admission.ID,
			); err != nil {
				t.Fatalf("delete migrated session prompt admission error = %v", err)
			}
			migratedEntry, err := database.GetSessionInputQueueEntry(
				testutil.Context(t),
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
	path               string
	sessionID          string
	runtimeWorkspaceID string
	now                time.Time
}

func seedSessionPromptAdmissionRuntimePrefix(t *testing.T, prefixDB *sql.DB) string {
	t.Helper()
	const (
		sessionID   = "sess-before-00094"
		admissionID = "admission-before-00094"
		queueID     = "queue-before-00094"
	)
	ctx := globalMigrationTestContext(t)
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	workspaceID := registerPrefixWorkspace(
		t,
		prefixDB,
		"acp-options-migration",
		filepath.Join(t.TempDir(), "workspace"),
	)
	if _, err := prefixDB.ExecContext(ctx, `INSERT INTO sessions (
		id, profile_id, name, agent_name,
		provider, model, reasoning_effort, speed,
		selected_provider, selected_model, selected_reasoning_effort, selected_speed,
		workspace_id, state, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID,
		store.DefaultProfileID,
		"Pre-00094 session",
		"coder",
		"cursor",
		"grok-4.6",
		"high",
		"fast",
		"cursor",
		"grok-4.6",
		"high",
		"fast",
		workspaceID,
		"active",
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed 00093 session error = %v", err)
	}
	if _, err := prefixDB.ExecContext(ctx, `INSERT INTO session_prompt_admissions (
		id, workspace_id, session_id, message_id, idempotency_key, operation,
		fingerprint_version, request_fingerprint, state, mode, authored_text,
		runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
		turn_id, event_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		admissionID,
		workspaceID,
		sessionID,
		"message-before-00094",
		"idem-before-00094",
		store.SessionPromptOperationPrompt,
		"prompt/v1",
		"sha256:before-00094",
		store.SessionPromptAdmissionReserved,
		store.SessionInputQueueModeQueue,
		"preserve admission",
		"cursor",
		"grok-4.6",
		"high",
		"fast",
		"turn-before-00094",
		"event-before-00094",
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed 00093 session_prompt_admissions error = %v", err)
	}
	if _, err := prefixDB.ExecContext(ctx, `INSERT INTO session_input_queue (
		id, session_id, prompt_admission_id, message_id, idempotency_key, turn_id, event_id,
		status, mode, text, runtime_provider, runtime_model, runtime_reasoning_effort, runtime_speed,
		enqueued_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		queueID,
		sessionID,
		admissionID,
		"message-before-00094",
		"idem-before-00094",
		"turn-before-00094",
		"event-before-00094",
		store.SessionInputQueueStatusQueued,
		store.SessionInputQueueModeQueue,
		"preserve queue",
		"cursor",
		"grok-4.6",
		"high",
		"fast",
		store.FormatTimestamp(now),
		store.FormatTimestamp(now),
	); err != nil {
		t.Fatalf("seed 00093 session_input_queue error = %v", err)
	}
	return workspaceID
}

func openSessionPromptAdmissionMigrationCopy(t *testing.T, sourcePath string) *GlobalDB {
	t.Helper()
	path := filepath.Join(t.TempDir(), GlobalDatabaseName)
	for _, suffix := range []string{"", "-wal", "-shm", ".telemetry.db", ".telemetry.db-wal", ".telemetry.db-shm"} {
		if err := copyGlobalMigrationSeedFile(sourcePath+suffix, path+suffix); err != nil {
			if suffix != "" && suffix != ".telemetry.db" && errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatalf("copy populated upgraded database family: %v", err)
		}
	}
	return openGlobalDBForTest(t, path)
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

	ctx := globalMigrationTestContext(t)
	now := time.Date(2026, 7, 31, 14, 0, 0, 0, time.UTC)
	sessionID := registerInputQueueMigrationPrefixSession(t, prefixDB, now)
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
	if err := applyGlobalMigrationPrefix(t, prefixDB, globalMigrationPrefixBefore(t, "00094_schema.sql")); err != nil {
		t.Fatalf("Apply(global through 00093) error = %v", err)
	}
	runtimeWorkspaceID := seedSessionPromptAdmissionRuntimePrefix(t, prefixDB)
	if err := prefixDB.Close(); err != nil {
		t.Fatalf("prefixDB.Close() error = %v", err)
	}
	prefixClosed = true

	upgraded, err := openGlobalMigrationUpgrade(t, path)
	if err != nil {
		t.Fatalf("OpenGlobalDB(populated prefix upgrade) error = %v", err)
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

	return sessionPromptAdmissionMigrationFixture{
		path:               path,
		sessionID:          sessionID,
		runtimeWorkspaceID: runtimeWorkspaceID,
		now:                now,
	}
}

func TestGlobalDBSessionPromptAdmission(t *testing.T) {
	t.Run("Should complete superseded steering for exact replay", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			path    string
			reserve bool
		}{
			{"typed", true}, {"empty", true}, {"rollback", true},
			{"typed", false}, {"empty", false}, {"rollback", false},
		} {
			t.Run(
				fmt.Sprintf("Should preserve replay for %s with reservation %t", tc.path, tc.reserve),
				func(t *testing.T) {
					t.Parallel()
					ctx := t.Context()
					db := openTestGlobalDB(t)
					sessionID := registerInputQueueSession(t, db)
					now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
					req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "reserved-superseded", now)
					req.Mode = store.SessionInputQueueModeSteer
					admission, entry, _, err := db.StageAdmittedSessionSteer(
						ctx,
						req,
						store.SessionInputQueueInsert{
							ID:           "reserved-steer",
							SessionID:    sessionID,
							Text:         "guidance",
							TargetTurnID: "original-turn",
							Delivery:     store.SessionInputDeliveryInterruptThenPrompt,
							QueueCap:     10,
							Now:          now,
						},
					)
					if err != nil {
						t.Fatal(err)
					}
					if tc.reserve {
						if _, ok, err := db.ReserveSessionSteer(ctx, sessionID, entry.ID, now); err != nil || !ok {
							t.Fatalf("reserve=%t/%v", ok, err)
						}
					}
					if tc.path == "empty" {
						_, err = db.SupersedeSessionBoundarySteers(
							ctx,
							sessionID,
							"original-turn",
							"empty-interrupt",
							now.Add(time.Second),
						)
					} else {
						id := "replacement"
						if tc.path == "rollback" {
							id = entry.ID
						}
						_, _, err = db.EnqueueInterruptSessionInput(
							ctx,
							store.SessionInputQueueInsert{
								ID:           id,
								SessionID:    sessionID,
								Text:         "replacement",
								TargetTurnID: "original-turn",
								QueueCap:     10,
								Now:          now.Add(time.Second),
							},
						)
					}
					if tc.path == "rollback" {
						if err == nil ||
							!strings.Contains(err.Error(), "UNIQUE constraint failed: session_input_queue.id") {
							t.Fatalf("rollback error=%v", err)
						}
					} else if err != nil {
						t.Fatal(err)
					}
					receipt, found, err := db.ReplaySessionPromptAdmission(ctx, req)
					if tc.path == "rollback" {
						if tc.reserve && !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
							t.Fatalf("reserved receipt escaped rollback: %#v/%v", receipt, err)
						}
						if !tc.reserve &&
							(err != nil || !found || !reflect.DeepEqual(receipt, admission)) {
							t.Fatalf("staged receipt escaped rollback: %#v/%t/%v", receipt, found, err)
						}
						return
					}
					if !tc.reserve && !reflect.DeepEqual(receipt, admission) {
						t.Fatalf("completed staged receipt changed: before=%#v after=%#v", admission, receipt)
					}
					if err != nil || !found || receipt.State != store.SessionPromptAdmissionCompleted ||
						receipt.Result == nil ||
						receipt.Result.PreviousTurnID != "original-turn" ||
						receipt.Result.Status != store.SessionPromptResultStatusSteering {
						t.Fatalf("superseded receipt=%#v/%t/%v", receipt, found, err)
					}
				},
			)
		}
	})

	t.Run("Should resolve named messages in admission and return held without claiming a turn", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "named-message", "sess-named-main")
		instance, err := db.Instances().
			Adopt(ctx, agentinstance.Instance{ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace, PermissionCeiling: "{}", CreatedByKind: "human"}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now()
		req := promptAdmissionRequest(workspace, "stale-main", "named", now)
		req.InstanceTarget = &store.InstancePromptTarget{
			ProfileID:   store.DefaultProfileID,
			WorkspaceID: workspace,
			AgentName:   "coder",
		}
		req.Origin = []byte(`{"kind":"session","session_id":"sender","hop":1}`)
		insert := store.SessionInputQueueInsert{
			ID:        "named-input",
			SessionID: req.SessionID,
			Text:      req.AuthoredText,
			QueueCap:  10,
			Now:       now,
		}
		if _, _, _, _, err := db.EnqueueAdmittedSessionInput(
			ctx,
			req,
			insert,
		); !errors.Is(
			err,
			store.ErrInstancePromptTargetChanged,
		) {
			t.Fatalf("stale name resolution: %v", err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		req.SessionID, insert.SessionID = main, main
		receipt, entry, _, created, err := db.EnqueueAdmittedSessionInput(ctx, req, insert)
		if err != nil || !created || receipt.Result.Delivery != "held" ||
			receipt.Result.HeldReason != "manual" || receipt.Result.HeldInstanceID != instance.ID {
			t.Fatalf("held receipt: %+v %v", receipt, err)
		}
		if _, found, err := db.ClaimNextSessionInput(ctx, main, now); err != nil || found {
			t.Fatalf("held selection: %v %v", found, err)
		}
		rows, err := db.ListPendingSessionInputs(ctx, main)
		if err != nil || len(rows) != 1 || !rows[0].Held {
			t.Fatalf("held rows: %+v %v", rows, err)
		}
		replay, _, _, created, err := db.EnqueueAdmittedSessionInput(ctx, req, insert)
		if err != nil || created || replay.Result.QueueEntryID != entry.ID {
			t.Fatalf("held replay: %+v %v", replay, err)
		}
		req.InstanceTarget.AgentName = "missing"
		if _, _, _, _, err := db.EnqueueAdmittedSessionInput(
			ctx,
			req,
			insert,
		); !errors.Is(
			err,
			agentinstance.ErrNotFound,
		) {
			t.Fatalf("missing address: %v", err)
		}
	})

	t.Run(
		"Should admit and replay interrupt at capacity without canceling parked or dispatching input",
		func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			sessionID := registerInputQueueSession(t, db)
			now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
			for _, id := range []string{"dispatching", "parked", "goal-parked"} {
				_, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
					ID: id, SessionID: sessionID, Text: id, QueueCap: 3, Now: now,
				})
				if err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.db.ExecContext(
				ctx,
				`UPDATE session_input_queue SET owner_kind = 'goal', dispatchable = 0 WHERE id = 'goal-parked'`,
			); err != nil {
				t.Fatal(err)
			}
			claimed, found, err := db.ClaimNextSessionInput(ctx, sessionID, now)
			if err != nil || !found || claimed.ID != "dispatching" {
				t.Fatalf("ClaimNextSessionInput() = %#v, %v, %v", claimed, found, err)
			}
			request := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "interrupt", now)
			request.Mode = store.SessionInputQueueModeInterrupt
			input := store.SessionInputQueueInsert{
				ID: "replacement", SessionID: sessionID, Text: "urgent replacement",
				Mode: store.SessionInputQueueModeInterrupt, QueueCap: 3, Now: now.Add(time.Second),
				Delivery:     store.SessionInputDeliveryInterruptThenPrompt,
				TargetTurnID: "active-turn",
			}
			for attempt := range 2 {
				admission, entry, _, created, enqueueErr := db.EnqueueAdmittedSessionInput(ctx, request, input)
				if enqueueErr != nil || created != (attempt == 0) {
					t.Fatalf("EnqueueAdmittedSessionInput() = created %v, error %v", created, enqueueErr)
				}
				if entry.SessionGeneration != 0 || admission.Result.CanceledQueuedEntries != 0 {
					t.Fatalf("interrupt mutated queue generation or canceled input: %#v, %#v", entry, admission.Result)
				}
			}
			for id, want := range map[string]string{
				"dispatching": store.SessionInputQueueStatusDispatching,
				"parked":      store.SessionInputQueueStatusQueued,
				"goal-parked": store.SessionInputQueueStatusQueued,
				"replacement": store.SessionInputQueueStatusQueued,
			} {
				entry, getErr := db.GetSessionInputQueueEntry(ctx, sessionID, id)
				if getErr != nil || entry.Status != want {
					t.Fatalf("GetSessionInputQueueEntry(%s) = %#v, %v, want %s", id, entry, getErr, want)
				}
			}
			next, found, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
			if err != nil || !found || next.ID != "replacement" {
				t.Fatalf("next dispatch = %#v, %v, %v, want replacement before parked input", next, found, err)
			}
		},
	)
	t.Run("Should settle pending steer once without changing its acceptance receipt", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name     string
			delivery store.SteerDeliveryMode
			stale    bool
		}{
			{"Should record successful injection", store.SteerDeliveryInjected, false},
			{"Should release fallback once", store.SteerDeliveryInterruptFallback, false},
			{"Should park a delayed refusal at the boundary", store.SteerDeliveryPendingBoundary, false},
			{"Should queue a delayed refusal after the turn", store.SteerDeliveryAfterTurn, false},
			{"Should fence late failure from an older generation", store.SteerDeliveryInterruptFallback, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				delivery := tc.delivery
				t.Parallel()
				ctx := t.Context()
				db := openTestGlobalDB(t)
				sessionID := registerInputQueueSession(t, db)
				now := time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)
				req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "late-steer", now)
				req.Mode = store.SessionInputQueueModeSteer
				_, entry, _, err := db.StageAdmittedSessionSteer(ctx, req, store.SessionInputQueueInsert{
					ID: "late-steer", SessionID: sessionID, Text: "change direction", TargetTurnID: "original-turn",
					Delivery: store.SessionInputDeliveryInterruptThenPrompt, QueueCap: 10, Now: now,
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, ok, err := db.ReserveSessionSteer(ctx, sessionID, entry.ID, now); err != nil || !ok {
					t.Fatalf("reserve=%v/%v", ok, err)
				}
				if _, err := db.ResolveSessionSteer(
					ctx,
					sessionID,
					entry.ID,
					store.SteerDeliveryPendingInjection,
					now,
				); err != nil {
					t.Fatal(err)
				}
				if tc.stale {
					if _, err := db.AdvanceSessionInputGeneration(ctx, sessionID, now); err != nil {
						t.Fatal(err)
					}
				}
				settled, changed, err := db.SettlePendingSessionSteer(
					ctx,
					sessionID,
					entry.ID,
					delivery,
					now.Add(time.Second),
				)
				if err != nil || changed == tc.stale || (!tc.stale && settled.SteerDelivery != delivery) {
					t.Fatalf("settle=%#v/%v/%v", settled, changed, err)
				}
				if _, changed, err := db.SettlePendingSessionSteer(
					ctx,
					sessionID,
					entry.ID,
					delivery,
					now.Add(2*time.Second),
				); err != nil ||
					changed {
					t.Fatalf("duplicate settlement=%v/%v", changed, err)
				}
				claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(3*time.Second))
				wantDispatch := !tc.stale && delivery != store.SteerDeliveryInjected
				if err != nil || ok != wantDispatch ||
					(ok && (claimed.ID != entry.ID || claimed.TurnID == "original-turn")) {
					t.Fatalf("dispatch=%#v/%v/%v", claimed, ok, err)
				}
				receipt, found, err := db.ReplaySessionPromptAdmission(ctx, req)
				if err != nil || !found || receipt.Result == nil ||
					receipt.Result.SteerDelivery != store.SteerDeliveryPendingInjection {
					t.Fatalf("acceptance changed=%#v/%v/%v", receipt, found, err)
				}
			})
		}
	})
	t.Run("Should reserve steering once and replay its resolved delivery atomically", func(t *testing.T) {
		t.Parallel()
		for _, delivery := range []store.SteerDeliveryMode{store.SteerDeliveryInjected, store.SteerDeliveryPendingInjection, store.SteerDeliveryInterruptFallback, store.SteerDeliveryPendingBoundary, store.SteerDeliveryAfterTurn} {
			t.Run("Should resolve "+string(delivery), func(t *testing.T) {
				t.Parallel()
				ctx := t.Context()
				db := openTestGlobalDB(t)
				sessionID := registerInputQueueSession(t, db)
				now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
				req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "steer-reservation", now)
				req.Mode = store.SessionInputQueueModeSteer
				_, entry, _, err := db.StageAdmittedSessionSteer(ctx, req, store.SessionInputQueueInsert{
					ID: "reserved-steer", SessionID: sessionID, Text: "change direction", TargetTurnID: "original-turn",
					Delivery: store.SessionInputDeliveryInterruptThenPrompt, QueueCap: 10, Now: now,
				})
				if err != nil {
					t.Fatal(err)
				}
				_, premature, err := db.ClaimNextSessionInput(ctx, sessionID, now)
				if err != nil || premature {
					t.Fatalf("queue claimed steering before delivery resolution: claimed=%v error=%v", premature, err)
				}
				reserved, ok, err := db.ReserveSessionSteer(ctx, sessionID, entry.ID, now.Add(time.Second))
				if err != nil || !ok || reserved.ID != entry.ID ||
					reserved.Status != store.SessionInputQueueStatusDispatching {
					t.Fatalf("ReserveSessionSteer() = %#v, %v, %v", reserved, ok, err)
				}
				_, second, err := db.ReserveSessionSteer(ctx, sessionID, entry.ID, now.Add(2*time.Second))
				if err != nil || second {
					t.Fatalf("duplicate ReserveSessionSteer() = %v, %v", second, err)
				}
				_, dispatched, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
				if err != nil || dispatched {
					t.Fatalf("queue claimed a reserved steer: %v, %v", dispatched, err)
				}
				_, _, err = db.ClaimSessionPromptAdmission(ctx, req)
				if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
					t.Fatalf("in-flight replay = %v", err)
				}
				resolved, err := db.ResolveSessionSteer(ctx, sessionID, entry.ID, delivery, now.Add(3*time.Second))
				if err != nil {
					t.Fatal(err)
				}
				if resolved.SteerDelivery != delivery {
					t.Fatalf("resolved delivery = %q", resolved.SteerDelivery)
				}
				if delivery == store.SteerDeliveryInterruptFallback || delivery == store.SteerDeliveryPendingBoundary ||
					delivery == store.SteerDeliveryAfterTurn {
					if resolved.Status != store.SessionInputQueueStatusQueued {
						t.Fatalf("fallback status = %q", resolved.Status)
					}
					claimed, ok, claimErr := db.ClaimNextSessionInput(ctx, sessionID, now.Add(4*time.Second))
					if claimErr != nil || !ok || claimed.ID != entry.ID {
						t.Fatalf(
							"resolved fallback must dispatch: entry=%#v claimed=%v error=%v",
							claimed,
							ok,
							claimErr,
						)
					}
				} else if resolved.Status != store.SessionInputQueueStatusSent ||
					resolved.TurnID != "original-turn" {
					t.Fatalf("injected input lost its original turn: %#v", resolved)
				}
				replay, found, err := db.ReplaySessionPromptAdmission(ctx, req)
				if err != nil || !found || replay.Result == nil {
					t.Fatalf("ReplaySessionPromptAdmission() = %#v, %v, %v", replay, found, err)
				}
				if replay.Result.SteerDelivery != delivery || replay.Result.PreviousTurnID != "original-turn" {
					t.Fatalf("replayed steer = %#v", replay.Result)
				}
			})
		}
	})
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

	t.Run("Should persist and replay admission attachments", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC)
		req := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "attachments", now)
		req.AuthoredText = ""
		req.Attachments = []store.SessionInputAttachment{testSessionInputAttachment()}
		first, created, err := globalDB.ClaimSessionPromptAdmission(ctx, req)
		if err != nil {
			t.Fatalf("ClaimSessionPromptAdmission() error = %v", err)
		}
		if !created {
			t.Fatal("ClaimSessionPromptAdmission() created = false, want true")
		}
		if !slices.Equal(first.Attachments, req.Attachments) {
			t.Fatalf("first attachments = %#v, want %#v", first.Attachments, req.Attachments)
		}
		replayed, created, err := globalDB.ClaimSessionPromptAdmission(ctx, req)
		if err != nil {
			t.Fatalf("ClaimSessionPromptAdmission(replay) error = %v", err)
		}
		if created {
			t.Fatal("ClaimSessionPromptAdmission(replay) created = true, want false")
		}
		if !slices.Equal(replayed.Attachments, req.Attachments) {
			t.Fatalf("replayed attachments = %#v, want %#v", replayed.Attachments, req.Attachments)
		}
	})

	t.Run("Should replay an admitted queue entry without consuming capacity twice", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, globalDB)
		now := time.Date(2026, 7, 31, 12, 5, 0, 0, time.UTC)
		admissionReq := promptAdmissionRequest("ws-input-queue-workspace", sessionID, "queue", now)
		admissionReq.Attachments = []store.SessionInputAttachment{testSessionInputAttachment()}
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
		if !slices.Equal(entry.Attachments, admissionReq.Attachments) {
			t.Fatalf("entry attachments = %#v, want %#v", entry.Attachments, admissionReq.Attachments)
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
		if !slices.Equal(replayedEntry.Attachments, admissionReq.Attachments) {
			t.Fatalf("replayed entry attachments = %#v, want %#v", replayedEntry.Attachments, admissionReq.Attachments)
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
		firstAdmission.Attachments = []store.SessionInputAttachment{testSessionInputAttachment()}
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

		if len(secondEntry.SupersededIDs) != 1 || secondEntry.SupersededIDs[0] != firstEntry.ID {
			t.Fatalf("superseded steering identities = %v, want [%s]", secondEntry.SupersededIDs, firstEntry.ID)
		}

		_, replayedEntry, created, err := globalDB.StageAdmittedSessionSteer(
			ctx,
			firstAdmission,
			firstQueue,
		)
		if err != nil {
			t.Fatalf("StageAdmittedSessionSteer(replay) error = %v", err)
		}
		if !slices.Equal(replayedEntry.Attachments, firstAdmission.Attachments) || created ||
			replayedEntry.ID != firstEntry.ID ||
			replayedEntry.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("replayed entry = %#v, created=%v", replayedEntry, created)
		}
		pending, err := globalDB.ListPendingSessionInputs(ctx, sessionID)
		if err != nil || len(pending) != 1 || pending[0].ID != secondEntry.ID || pending[0].Dispatchable {
			t.Fatalf("pending steering = %#v, error=%v; want only unresolved %q", pending, err, secondEntry.ID)
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

func TestGlobalDBSessionAttachmentsMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should add attachments_json with an empty-array default on reopen", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00064_schema.sql"),
		)
		if err != nil {
			t.Fatalf("open prefix before 00064 error = %v", err)
		}
		prefixClosed := false
		t.Cleanup(func() {
			if prefixClosed {
				return
			}
			if closeErr := prefixDB.Close(); closeErr != nil {
				t.Errorf("prefixDB.Close() error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, prefixDB, "session_input_queue", []string{"text"})
		queueColumns, err := tableColumns(testutil.Context(t), prefixDB, "session_input_queue")
		if err != nil {
			t.Fatalf("tableColumns(session_input_queue) error = %v", err)
		}
		if _, ok := queueColumns["attachments_json"]; ok {
			t.Fatal("attachments_json present before 00064, want absent")
		}
		admissionColumns, err := tableColumns(testutil.Context(t), prefixDB, "session_prompt_admissions")
		if err != nil {
			t.Fatalf("tableColumns(session_prompt_admissions) error = %v", err)
		}
		if _, ok := admissionColumns["attachments_json"]; ok {
			t.Fatal("session_prompt_admissions.attachments_json present before 00064, want absent")
		}
		if _, err := prefixDB.ExecContext(testutil.Context(t), "PRAGMA foreign_keys = OFF"); err != nil {
			t.Fatalf("disable foreign keys for pre-00063 fixture: %v", err)
		}
		if _, err := prefixDB.ExecContext(
			testutil.Context(t),
			`INSERT INTO session_prompt_admissions (
				id, workspace_id, session_id, message_id, idempotency_key, operation,
				fingerprint_version, request_fingerprint, state, turn_id, event_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"admission-before-63", "workspace-before-63", "session-before-63", "message-before-63",
			"idempotency-before-63", "prompt", "prompt/v1", "sha256:before-63", "reserved",
			"turn-before-63", "event-before-63", "2026-08-15T00:00:00Z", "2026-08-15T00:00:00Z",
		); err != nil {
			t.Fatalf("insert pre-00063 session_prompt_admissions row: %v", err)
		}
		if _, err := prefixDB.ExecContext(
			testutil.Context(t),
			`INSERT INTO session_input_queue (
				id, session_id, status, mode, text, enqueued_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			"queue-before-63", "session-before-63", "queued", "queue", "preserved text",
			"2026-08-15T00:00:00Z", "2026-08-15T00:00:00Z",
		); err != nil {
			t.Fatalf("insert pre-00063 session_input_queue row: %v", err)
		}
		if err := applyGlobalMigrationPrefix(
			t,
			prefixDB,
			globalMigrationPrefixThrough(t, "00064_schema.sql"),
		); err != nil {
			t.Fatalf("Apply(through 00064) error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("prefixDB.Close() error = %v", err)
		}
		prefixClosed = true

		reopened, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixThrough(t, "00064_schema.sql"),
		)
		if err != nil {
			t.Fatalf("reopen through 00064 error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(); closeErr != nil {
				t.Errorf("reopened.Close() error = %v", closeErr)
			}
		})
		assertTableHasColumns(t, reopened, "session_input_queue", []string{"attachments_json"})
		assertTableHasColumns(t, reopened, "session_prompt_admissions", []string{"attachments_json"})
		var queueDefault string
		if err := reopened.QueryRowContext(
			testutil.Context(t),
			`SELECT attachments_json FROM session_input_queue WHERE id = ?`,
			"queue-before-63",
		).Scan(&queueDefault); err != nil {
			t.Fatalf("read session_input_queue.attachments_json error = %v", err)
		}
		if queueDefault != "[]" {
			t.Fatalf("session_input_queue.attachments_json = %q, want []", queueDefault)
		}
		var admissionDefault string
		if err := reopened.QueryRowContext(
			testutil.Context(t),
			`SELECT attachments_json FROM session_prompt_admissions WHERE id = ?`,
			"admission-before-63",
		).Scan(&admissionDefault); err != nil {
			t.Fatalf("read session_prompt_admissions.attachments_json error = %v", err)
		}
		if admissionDefault != "[]" {
			t.Fatalf("session_prompt_admissions.attachments_json = %q, want []", admissionDefault)
		}
		for _, mutation := range []struct {
			name  string
			query string
			id    string
		}{
			{
				name:  "session input queue",
				query: `UPDATE session_input_queue SET attachments_json = ? WHERE id = ?`,
				id:    "queue-before-63",
			},
			{
				name:  "session prompt admissions",
				query: `UPDATE session_prompt_admissions SET attachments_json = ? WHERE id = ?`,
				id:    "admission-before-63",
			},
		} {
			_, updateErr := reopened.ExecContext(testutil.Context(t), mutation.query, "not-json", mutation.id)
			if updateErr == nil || !strings.Contains(updateErr.Error(), "CHECK constraint failed") {
				t.Fatalf(
					"invalid attachments_json update for %s error = %v, want CHECK constraint failure",
					mutation.name,
					updateErr,
				)
			}
		}
	})
}

func globalMigrationPrefixThrough(t *testing.T, lastMigration string) store.MigrationStream {
	t.Helper()

	fullStream := MigrationStream()
	entries, err := fs.ReadDir(fullStream.FS, fullStream.Dir)
	if err != nil {
		t.Fatalf("read global migration directory: %v", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") || entry.Name() > lastMigration {
			continue
		}
		names = append(names, entry.Name())
	}
	return globalMigrationPrefix(t, names...)
}

func testSessionInputAttachment() store.SessionInputAttachment {
	return store.SessionInputAttachment{
		ID:       "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Name:     "shot.png",
		MIMEType: "image/png",
		Bytes:    1024,
		SHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:     store.SessionInputAttachmentKindImage,
		Width:    320,
		Height:   240,
	}
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
	if err := globalDB.RegisterSession(testutil.Context(t), new(SessionInfo{
		ID:            sessionID,
		ProfileID:     store.DefaultProfileID,
		Name:          "Input Queue",
		AgentName:     "coder",
		Provider:      "claude",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		SessionType:   defaultSessionType,
		State:         "active",
		CreatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	})); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
	return sessionID
}

func registerInputQueueMigrationPrefixSession(t *testing.T, prefixDB *sql.DB, now time.Time) string {
	t.Helper()

	workspaceID := registerPrefixWorkspace(
		t,
		prefixDB,
		"input-queue-workspace",
		filepath.Join(t.TempDir(), "workspace"),
	)
	sessionID := "sess-input-queue"
	if _, err := prefixDB.ExecContext(
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

func requireSQLiteConstraintError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("error = nil, want sqlite constraint error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "constraint") {
		t.Fatalf("error = %v, want sqlite constraint failure", err)
	}
}

// UT-058, IT-007: synthetic message replay and priority belong to the durable queue.
func TestGlobalDBSubagentQueuePriority(t *testing.T) {
	t.Run("Should replay synthetic messages and dispatch human input ahead of prioritized wakes", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
		if _, _, err := db.EnqueueSessionInput(
			ctx,
			store.SessionInputQueueInsert{ID: "user", SessionID: sessionID, Text: "user", QueueCap: 10, Now: now},
		); err != nil {
			t.Fatal(err)
		}
		req := store.SessionInputQueueInsert{
			ID:        "wake",
			SessionID: sessionID,
			MessageID: "wake-message",
			TurnID:    "wake-turn",
			Priority:  1,
			Text:      "wake",
			OwnerKind: store.SessionInputOwnerSynthetic,
			SyntheticPrompt: &store.SessionInputSyntheticPrompt{
				RunID:    "run",
				Metadata: []byte(`{"reason":"subagent"}`),
			},
			QueueCap: 10,
			Now:      now.Add(time.Second),
		}
		first, _, err := db.EnqueueSessionInput(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		req.ID = "retry"
		req.QueueCap = 1
		replay, _, err := db.EnqueueSessionInput(ctx, req)
		if err != nil || replay.ID != first.ID {
			t.Fatalf("replay while full=%#v %v", replay, err)
		}
		pending, err := db.ListPendingSessionInputs(ctx, sessionID)
		if err != nil || len(pending) != 2 || pending[0].ID != "user" || pending[1].ID != "wake" ||
			pending[1].Priority != 1 {
			t.Fatalf("order=%v %v", pending, err)
		}
		peek, ok, err := db.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !ok || peek.ID != "user" {
			t.Fatalf("peek=%#v %v %v", peek, ok, err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
		if err != nil || !ok || claimed.ID != "user" {
			t.Fatalf("claim=%#v %v %v", claimed, ok, err)
		}
		if err = db.MarkSessionInputSent(ctx, sessionID, "user", now.Add(2*time.Second)); err != nil {
			t.Fatal(err)
		}
		claimed, ok, err = db.ClaimNextSessionInput(ctx, sessionID, now.Add(2*time.Second))
		if err != nil || !ok || claimed.ID != "wake" {
			t.Fatalf("claim=%#v %v %v", claimed, ok, err)
		}
		replay, _, err = db.EnqueueSessionInput(ctx, req)
		if err != nil || replay.ID != "wake" {
			t.Fatalf("dispatch replay=%#v %v", replay, err)
		}
		if err = db.MarkSessionInputSent(ctx, sessionID, "wake", now.Add(3*time.Second)); err != nil {
			t.Fatal(err)
		}
		req.QueueCap = 10
		if again, _, err := db.EnqueueSessionInput(ctx, req); err != nil || again.ID != "retry" {
			t.Fatalf("reoffer terminal=%#v %v", again, err)
		}
		req.MessageID = ""
		req.ID = "empty-a"
		if _, _, err = db.EnqueueSessionInput(ctx, req); err != nil {
			t.Fatal(err)
		}
		req.ID = "empty-b"
		if _, _, err = db.EnqueueSessionInput(ctx, req); err != nil {
			t.Fatal(err)
		}
		if _, _, err = db.EnqueueSessionInput(
			ctx,
			store.SessionInputQueueInsert{
				ID:           "interrupt",
				SessionID:    sessionID,
				Text:         "interrupt",
				Mode:         store.SessionInputQueueModeInterrupt,
				Delivery:     store.SessionInputDeliveryInterruptThenPrompt,
				TargetTurnID: "current",
				QueueCap:     10,
				Now:          now.Add(4 * time.Second),
			},
		); err != nil {
			t.Fatal(err)
		}
		peek, ok, err = db.PeekNextSessionInput(ctx, sessionID)
		if err != nil || !ok || peek.ID != "interrupt" {
			t.Fatalf("delivery precedence=%#v %v %v", peek, ok, err)
		}
	})
}

// IT-021: upgrading 00130 preserves existing queue entries, including shared user message IDs.
func TestGlobalDBSubagentMigration(t *testing.T) {
	t.Run("Should upgrade populated 00130 without changing user queue identity or order", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixThrough(t, "00130_retire_memory.sql"),
		)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
		old := &GlobalDB{db: prefix, readDB: prefix, path: path, now: func() time.Time { return now }}
		old.initializeRepositories(openConfig{})
		workspaceID := registerWorkspaceForGlobalTests(
			t,
			old,
			"input-queue-workspace",
			filepath.Join(t.TempDir(), "workspace"),
		)
		id := "sess-input-queue"
		created := store.FormatTimestamp(time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC))
		insertPrefixRow(t, prefix, "sessions", map[string]any{
			"id": id, "profile_id": store.DefaultProfileID, "name": "Input Queue", "agent_name": "coder",
			"provider": "claude", "runtime_status": string(store.SessionRuntimeUnbound), "workspace_id": workspaceID,
			"session_type": defaultSessionType, "state": "active", "created_at": created, "updated_at": created,
		})
		for _, entry := range []string{"a", "b"} {
			if _, err = prefix.ExecContext(
				ctx,
				`INSERT INTO session_input_queue(id,session_id,message_id,status,mode,text,enqueued_at,updated_at) VALUES(?,?,'shared','queued','queue',?,?,?)`,
				entry,
				id,
				entry,
				store.FormatTimestamp(now),
				store.FormatTimestamp(now),
			); err != nil {
				t.Fatal(err)
			}
		}
		if _, err = prefix.ExecContext(
			ctx,
			`INSERT INTO session_input_clear_traces(entry_id,session_id,turn_id,actor_kind,actor_id,queue_generation,created_at) VALUES('a',?,'clear','human','operator',0,?)`,
			id,
			store.FormatTimestamp(now),
		); err != nil {
			t.Fatal(err)
		}
		if err = prefix.Close(); err != nil {
			t.Fatal(err)
		}
		db := openGlobalDBForTest(t, path)
		items, err := db.ListPendingSessionInputs(ctx, id)
		if err != nil || len(items) != 2 || items[0].ID != "a" || items[1].ID != "b" {
			t.Fatalf("preserved=%v %v", items, err)
		}
		for _, item := range items {
			if item.Priority != 0 || item.MessageID != "shared" {
				t.Fatalf("changed user identity=%#v", item)
			}
		}
		if _, _, err = db.ReplaceSessionInput(
			ctx,
			id,
			"a",
			store.SessionInputQueueInsert{ID: "replacement", SessionID: id, Text: "edited", QueueCap: 10, Now: now},
		); err != nil {
			t.Fatal(err)
		}
		var traceCount int
		if err = db.db.QueryRowContext(ctx, `SELECT count(*) FROM session_input_clear_traces WHERE entry_id='a'`).
			Scan(&traceCount); err != nil ||
			traceCount != 1 {
			t.Fatalf("preserved clear trace = %d, %v", traceCount, err)
		}
		status, err := store.Status(ctx, db.db, MigrationStream())
		if err != nil {
			t.Fatal(err)
		}
		assertCompleteMigrationStream(t, status, MigrationStream())
		var workspace string
		if err = db.db.QueryRowContext(ctx, `SELECT workspace_id FROM sessions WHERE id=?`, id).
			Scan(&workspace); err != nil {
			t.Fatal(err)
		}
		row := reserveSubagent(t, db.SessionRepo, workspace, id, "migrated", now)
		if row.PendingTask == nil {
			t.Fatal("pending task missing")
		}
		if _, _, err = db.ReserveSubagent(
			ctx,
			store.SessionSubagent{
				ID:              "invalid",
				WorkspaceID:     workspace,
				ParentSessionID: id,
				ParentTurnID:    "turn",
				IdempotencyKey:  "invalid",
				Origin:          "invalid",
				Depth:           1,
				WakePolicy:      "always",
			},
		); err == nil {
			t.Fatal("missing origin check")
		}
		if _, _, err = db.ReserveSubagent(
			ctx,
			store.SessionSubagent{
				ID:              "foreign",
				WorkspaceID:     "foreign",
				ParentSessionID: id,
				ParentTurnID:    "turn",
				IdempotencyKey:  "foreign",
				Origin:          "delegated",
				Depth:           1,
				WakePolicy:      "always",
			},
		); err == nil {
			t.Fatal("missing scoped parent foreign key")
		}
	})
}

func TestGlobalDBReplyWatchHandoff(t *testing.T) {
	// Invariant: a reply commits one durable queue handoff; owner: global store queue suite.
	t.Run("Should register atomically and replay the same watch UT-042", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sender := registerInputQueueSession(t, db)
		now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
		req := promptAdmissionRequest("ws-input-queue-workspace", sender, "reply", now)
		registration := store.ReplyWatchRegistration{
			WorkspaceID:       req.WorkspaceID,
			SenderSessionID:   sender,
			TargetWorkspaceID: req.WorkspaceID,
			TargetSessionID:   sender,
			MessageID:         req.MessageID,
			AdmissionID:       req.ID,
			Hop:               1,
			CreatedAt:         now,
		}
		aborted := errors.New("abort admission")
		var first store.ReplyWatch
		err := db.SessionRepo.withImmediateTransaction(
			t.Context(),
			"test registration rollback",
			func(exec globalSQLExecutor) error {
				if _, _, err := claimSessionPromptAdmission(t.Context(), exec, req); err != nil {
					return err
				}
				var err error
				first, err = db.InsertReplyWatchTx(t.Context(), exec, registration)
				if err != nil {
					return err
				}
				return aborted
			},
		)
		if !errors.Is(err, aborted) {
			t.Fatal(err)
		}
		if _, err := db.GetReplyWatch(t.Context(), first.ID); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("rolled back watch: %v", err)
		}
		err = db.SessionRepo.withImmediateTransaction(
			t.Context(),
			"test registration commit",
			func(exec globalSQLExecutor) error {
				if _, _, err := claimSessionPromptAdmission(t.Context(), exec, req); err != nil {
					return err
				}
				a, err := db.InsertReplyWatchTx(t.Context(), exec, registration)
				if err != nil {
					return err
				}
				b, err := db.InsertReplyWatchTx(t.Context(), exec, registration)
				if err != nil {
					return err
				}
				if a.ID != b.ID || a.State != "armed" {
					t.Errorf("replay = %+v, %+v", a, b)
				}
				return nil
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := db.ListReplyWatches(t.Context(), store.ReplyWatchFilter{SenderSessionID: sender})
		if err != nil || len(rows) != 1 {
			t.Fatalf("watches = %+v, %v", rows, err)
		}
		registration.Hop = 2
		if _, err := db.InsertReplyWatchTx(
			t.Context(),
			db.DB(),
			registration,
		); !errors.Is(
			err,
			store.ErrSessionPromptIdempotencyConflict,
		) {
			t.Fatalf("mismatch = %v", err)
		}
	})
	t.Run("Should commit one handoff even after sent and reopen UT-049 IT-021", func(t *testing.T) {
		t.Parallel()
		db, w, entry := replyHandoffFixture(t)
		const workers = 8
		results := make(chan bool, workers)
		failures := make(chan error, workers)
		for range workers {
			go func() {
				changed, err := db.DeliverReplyWatch(t.Context(), w.ID, entry)
				results <- changed
				failures <- err
			}()
		}
		wins := 0
		for range workers {
			if <-results {
				wins++
			}
			if err := <-failures; err != nil {
				t.Fatal(err)
			}
		}
		if wins != 1 {
			t.Fatalf("deliveries = %d", wins)
		}
		claimed, ok, err := db.ClaimNextSessionInput(t.Context(), entry.SessionID, entry.Now)
		if err != nil || !ok || claimed.ID != entry.ID {
			t.Fatalf("claim = %+v, %v, %v", claimed, ok, err)
		}
		if err := db.MarkSessionInputSent(t.Context(), entry.SessionID, entry.ID, entry.Now); err != nil {
			t.Fatal(err)
		}
		// Reopen the same SQLite file; a sent row is outside the pending unique index.
		reopened, err := OpenGlobalDB(t.Context(), db.path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		entry.ID = "second-input"
		if changed, err := reopened.DeliverReplyWatch(t.Context(), w.ID, entry); err != nil || changed {
			t.Fatalf("redelivery = %v, %v", changed, err)
		}
		var count int
		if err := reopened.DB().
			QueryRowContext(t.Context(), "SELECT count(*) FROM session_input_queue WHERE message_id = ?", entry.MessageID).
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("input count = %d, %v", count, err)
		}
	})
	t.Run("Should queue the wake in the current generation after a concurrent clear", func(t *testing.T) {
		t.Parallel()
		db, w, entry := replyHandoffFixture(t)
		stale := entry.SessionGeneration
		current, err := db.AdvanceSessionInputGeneration(t.Context(), entry.SessionID, entry.Now)
		if err != nil || current == stale {
			t.Fatalf("advance = %d, %v; want a new generation", current, err)
		}
		if changed, err := db.DeliverReplyWatch(t.Context(), w.ID, entry); err != nil || !changed {
			t.Fatalf("delivery = %v, %v", changed, err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(t.Context(), entry.SessionID, entry.Now)
		if err != nil || !ok || claimed.ID != entry.ID || claimed.SessionGeneration != current {
			t.Fatalf("claim = %+v, %v, %v; want the wake dispatchable in generation %d", claimed, ok, err, current)
		}
	})
	t.Run("Should roll back a full queue and abandon an archived sender UT-049 UT-046", func(t *testing.T) {
		t.Parallel()
		db, w, entry := replyHandoffFixture(t)
		if _, _, err := db.EnqueueSessionInput(
			t.Context(),
			store.SessionInputQueueInsert{
				ID:        "full",
				SessionID: entry.SessionID,
				Text:      "another session",
				Origin:    json.RawMessage(`{"kind":"session","session_id":"sess-peer","workspace_id":"ws","hop":1}`),
				QueueCap:  1,
				Now:       entry.Now,
			},
		); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DeliverReplyWatch(t.Context(), w.ID, entry); !errors.Is(err, store.ErrSessionInputQueueFull) {
			t.Fatalf("full = %v", err)
		}
		current, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || current.State != "fired" || current.DeliveredInputID != "" {
			t.Fatalf("watch = %+v, %v", current, err)
		}
		if _, err := db.CancelSessionInput(t.Context(), entry.SessionID, "full", entry.Now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.DB().
			ExecContext(t.Context(), "UPDATE sessions SET state='stopped', archived_at=? WHERE id=?", store.FormatTimestamp(entry.Now), entry.SessionID); err != nil {
			t.Fatal(err)
		}
		if changed, err := db.DeliverReplyWatch(t.Context(), w.ID, entry); err != nil || changed {
			t.Fatalf("archive delivery = %v, %v", changed, err)
		}
		current, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || current.State != "abandoned" || current.AbandonReason != "sender_gone" {
			t.Fatalf("abandoned = %+v, %v", current, err)
		}
	})
}

func replyHandoffFixture(t *testing.T) (*GlobalDB, store.ReplyWatch, store.SessionInputQueueInsert) {
	t.Helper()
	db := openTestGlobalDB(t)
	sender := registerInputQueueSession(t, db)
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	req := promptAdmissionRequest("ws-input-queue-workspace", sender, "handoff", now)
	if _, _, err := db.ClaimSessionPromptAdmission(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	w, err := db.InsertReplyWatchTx(
		t.Context(),
		db.DB(),
		store.ReplyWatchRegistration{
			WorkspaceID:       req.WorkspaceID,
			SenderSessionID:   sender,
			TargetWorkspaceID: req.WorkspaceID,
			TargetSessionID:   sender,
			MessageID:         req.MessageID,
			AdmissionID:       req.ID,
			Hop:               1,
			CreatedAt:         now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := db.FireReplyWatch(
		t.Context(),
		w.ID,
		store.ReplyWatchFire{Outcome: "completed", Text: "answer"},
	); err != nil ||
		!changed {
		t.Fatalf("fire = %v, %v", changed, err)
	}
	return db, w, store.SessionInputQueueInsert{
		ID:        "reply-input",
		SessionID: sender,
		MessageID: "prompt-reply:" + w.ID,
		Priority:  1,
		OwnerKind: store.SessionInputOwnerSynthetic,
		Text:      "reply",
		TurnID:    "reply-turn",
		Mode:      store.SessionInputQueueModeQueue,
		Delivery:  store.SessionInputDeliveryAfterTurn,
		QueueCap:  1,
		Now:       now,
		SyntheticPrompt: &store.SessionInputSyntheticPrompt{
			RunID:    "prompt-reply:" + w.ID,
			Metadata: []byte(`{"kind":"session_reply","reason":"completed"}`),
		},
	}
}

// UT-017: queued session messages reject edit/promote and still permit cancellation.
func TestGlobalDBSessionOriginMutation(t *testing.T) {
	t.Parallel()
	t.Run("Should refuse editing and promotion but allow cancellation of agent input", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		origin := json.RawMessage(`{"kind":"session","session_id":"sender","workspace_id":"ws","hop":1}`)
		now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
		entry, _, err := db.EnqueueSessionInput(
			t.Context(),
			store.SessionInputQueueInsert{
				ID:        "origin-entry",
				SessionID: sessionID,
				Text:      "Q?",
				Origin:    origin,
				QueueCap:  10,
				Now:       now,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		replacement := store.SessionInputQueueInsert{
			ID:           "replacement",
			Text:         "changed",
			TargetTurnID: "turn-active",
			QueueCap:     10,
			Now:          now,
		}
		if _, _, err := db.ReplaceSessionInput(
			t.Context(),
			sessionID,
			entry.ID,
			replacement,
		); !errors.Is(
			err,
			store.ErrSessionInputAgentAuthored,
		) {
			t.Fatal(err)
		}
		if _, _, err := db.PromoteSessionInputToSteer(
			t.Context(),
			sessionID,
			entry.ID,
			replacement,
		); !errors.Is(
			err,
			store.ErrSessionInputAgentAuthored,
		) {
			t.Fatal(err)
		}
		replay := store.SessionInputQueueInsert{
			ID:        "existing-replacement",
			SessionID: sessionID,
			Text:      entry.Text,
			QueueCap:  10,
			Now:       now,
		}
		if _, _, err := db.EnqueueSessionInput(t.Context(), replay); err != nil {
			t.Fatal(err)
		}
		if _, _, err := db.ReplaceSessionInput(
			t.Context(),
			sessionID,
			entry.ID,
			replay,
		); !errors.Is(
			err,
			store.ErrSessionInputAgentAuthored,
		) {
			t.Fatalf("attributed mutation replay: %v", err)
		}
		loaded, err := db.GetSessionInputQueueEntry(t.Context(), sessionID, entry.ID)
		if err != nil || loaded.Text != "Q?" || string(loaded.Origin) != string(origin) {
			t.Fatal(loaded, err)
		}
		canceled, err := db.CancelSessionInput(t.Context(), sessionID, entry.ID, now)
		if err != nil || canceled.Status != store.SessionInputQueueStatusCanceled {
			t.Fatal(canceled, err)
		}
	})
}

// IT-024: origin migration preserves existing queue/admission rows.
func TestGlobalDBSessionOriginMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve pre-origin admissions and queue rows across upgrade and reopen", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		db, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00135_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := db.ExecContext(t.Context(), "PRAGMA foreign_keys = OFF"); err != nil {
			t.Fatal(err)
		}
		now := "2026-10-09T00:00:00Z"
		if _, err := db.ExecContext(t.Context(), `INSERT INTO session_prompt_admissions
 (id,workspace_id,session_id,message_id,idempotency_key,operation,fingerprint_version,request_fingerprint,state,authored_text,turn_id,event_id,result_json,created_at,updated_at)
 VALUES ('pad-origin','ws-origin','sess-origin','msg-origin','key-origin','prompt','session-prompt/v4','sha256:legacy','completed','preserved prompt','turn-origin','event-origin','{"status":"accepted","new_turn_id":"turn-origin"}',?,?)`, now, now); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(t.Context(), `INSERT INTO session_input_queue
 (id,session_id,prompt_admission_id,status,mode,text,enqueued_at,updated_at) VALUES ('inq-origin','sess-origin','pad-origin','queued','queue','preserved queue',?,?)`, now, now); err != nil {
			t.Fatal(err)
		}
		readRows := func() map[string][]any {
			t.Helper()
			result := make(map[string][]any)
			for _, table := range []string{"session_prompt_admissions", "session_input_queue"} {
				rows, err := db.QueryContext(t.Context(), "SELECT * FROM "+table)
				if err != nil {
					t.Fatal(err)
				}
				columns, err := rows.Columns()
				if err != nil {
					t.Fatal(err)
				}
				for rows.Next() {
					values := make([]any, len(columns))
					dest := make([]any, len(columns))
					for i := range dest {
						dest[i] = &values[i]
					}
					if err := rows.Scan(dest...); err != nil {
						t.Fatal(err)
					}
					for i, column := range columns {
						if column != "origin_json" {
							result[table] = append(result[table], values[i])
						}
					}
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				if err := rows.Close(); err != nil {
					t.Fatal(err)
				}
			}
			return result
		}
		before := readRows()
		if err := applyGlobalMigrationPrefix(t, db, globalMigrationPrefixThrough(t, "00135_schema.sql")); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db, err = openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixThrough(t, "00135_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if after := readRows(); !reflect.DeepEqual(before, after) {
			t.Fatalf("upgrade changed existing values: before=%v after=%v", before, after)
		}
		// The repository's queries target the head schema, so finish the upgrade before replaying.
		if err := applyGlobalMigrationPrefix(t, db, MigrationStream()); err != nil {
			t.Fatal(err)
		}
		// IT-030: the real upgrade preserves a v4 receipt for the store replay boundary.
		repo := &SessionRepo{repoBase: newRepoBase(db, db, time.Now, new(atomic.Int32))}
		replay, found, err := repo.ReplaySessionPromptAdmission(t.Context(), store.SessionPromptAdmissionRequest{
			ID:                       "new-admission",
			WorkspaceID:              "ws-origin",
			SessionID:                "sess-origin",
			MessageID:                "msg-origin",
			IdempotencyKey:           "key-origin",
			Operation:                store.SessionPromptOperationPrompt,
			FingerprintVersion:       "session-prompt/v5",
			RequestFingerprint:       "sha256:actor-v5",
			LegacyRequestFingerprint: "sha256:legacy",
			AuthoredText:             "preserved prompt",
			TurnID:                   "new-turn",
			EventID:                  "new-event",
			Origin: json.RawMessage(
				`{"kind":"session","session_id":"sender","workspace_id":"ws-origin","hop":1}`,
			),
		})
		if err != nil || !found || replay.ID != "pad-origin" || len(replay.Origin) != 0 ||
			replay.FingerprintVersion != "session-prompt/v4" || replay.Result.Status != "accepted" || replay.Result.NewTurnID != "turn-origin" {
			t.Fatal(replay, found, err)
		}

		var text, fingerprint, link string
		var origin sql.NullString
		if err := db.QueryRowContext(t.Context(), `SELECT authored_text,fingerprint_version,origin_json FROM session_prompt_admissions WHERE id='pad-origin'`).
			Scan(&text, &fingerprint, &origin); err != nil {
			t.Fatal(err)
		}
		if text != "preserved prompt" || fingerprint != "session-prompt/v4" || origin.Valid {
			t.Fatal(text, fingerprint, origin)
		}
		if err := db.QueryRowContext(t.Context(), `SELECT text,prompt_admission_id,origin_json FROM session_input_queue WHERE id='inq-origin'`).
			Scan(&text, &link, &origin); err != nil {
			t.Fatal(err)
		}
		if text != "preserved queue" || link != "pad-origin" || origin.Valid {
			t.Fatal(text, link, origin)
		}
	})
}

// Invariant: watch registration and the admission/queue row commit or roll back together.
// Owner: global store transaction boundary; canonical admission/queue suite.
func TestGlobalDBReplyWatchAdmission(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should roll back a failed watch registration with queued=%t", queued), func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			target := registerInputQueueSession(t, db)
			req := promptAdmissionRequest("ws-input-queue-workspace", target, "atomic-watch", time.Now().UTC())
			req.FingerprintVersion = "session-prompt/v5"
			req.Origin = json.RawMessage(
				`{"kind":"session","session_id":"sender","workspace_id":"ws-sender","hop":1,"notify_on_complete":true}`,
			)
			if _, err := db.DB().
				ExecContext(t.Context(), `CREATE TRIGGER reject_reply_watch BEFORE INSERT ON session_prompt_reply_watches BEGIN SELECT RAISE(ABORT, 'watch insertion fault'); END`); err != nil {
				t.Fatal(err)
			}
			var err error
			if queued {
				_, _, _, _, err = db.EnqueueAdmittedSessionInput(
					t.Context(),
					req,
					store.SessionInputQueueInsert{
						ID:        "atomic-input",
						SessionID: target,
						Text:      "question",
						QueueCap:  10,
						Now:       req.Now,
					},
				)
			} else {
				_, _, err = db.ClaimSessionPromptAdmission(t.Context(), req)
			}
			if err == nil || !strings.Contains(err.Error(), "watch insertion fault") {
				t.Fatalf("registration error = %v", err)
			}
			for _, table := range []string{"session_prompt_admissions", "session_input_queue", "session_prompt_reply_watches"} {
				var count int
				if err := db.DB().
					QueryRowContext(t.Context(), "SELECT count(*) FROM "+table).
					Scan(&count); err != nil ||
					count != 0 {
					t.Fatalf("%s rows = %d, %v", table, count, err)
				}
			}
			if _, err := db.DB().ExecContext(t.Context(), "DROP TRIGGER reject_reply_watch"); err != nil {
				t.Fatal(err)
			}
			if _, _, err := db.ClaimSessionPromptAdmission(t.Context(), req); err != nil {
				t.Fatal(err)
			}
			w, err := db.GetReplyWatch(t.Context(), store.ReplyWatchID(target, req.MessageID))
			if err != nil || w.AdmissionID != req.ID || w.State != "armed" {
				t.Fatal(w, err)
			}
		})
	}
}

// IT-025: persisted reply lifecycle values reject invalid states, outcomes, and abandonment reasons.
func TestGlobalDBReplyWatchConstraints(t *testing.T) {
	t.Run("Should enforce the reply lifecycle CHECK constraints", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		target := registerInputQueueSession(t, db)
		req := promptAdmissionRequest(
			"ws-input-queue-workspace",
			target,
			"checks",
			time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC),
		)
		req.FingerprintVersion = "session-prompt/v5"
		req.Origin = json.RawMessage(
			`{"kind":"session","session_id":"sender","workspace_id":"ws-sender","hop":1,"notify_on_complete":true}`,
		)
		if _, _, err := db.ClaimSessionPromptAdmission(t.Context(), req); err != nil {
			t.Fatal(err)
		}
		for _, column := range []string{"state", "outcome", "abandon_reason"} {
			_, err := db.DB().
				ExecContext(t.Context(), "UPDATE session_prompt_reply_watches SET "+column+" = ? WHERE id = ?", "invalid", store.ReplyWatchID(target, req.MessageID))
			if err == nil || !strings.Contains(err.Error(), "CHECK constraint failed") {
				t.Fatalf("%s error = %v", column, err)
			}
		}
		got, err := db.GetReplyWatch(t.Context(), store.ReplyWatchID(target, req.MessageID))
		if err != nil || got.State != "armed" || got.Outcome != "" || got.AbandonReason != "" {
			t.Fatalf("preserved row = %+v, %v", got, err)
		}
	})
}

// Invariant: a synthetic wake counts as admitted from enqueue through sent, so a replayed wake whose
// queue entry was already delivered is not admitted twice; unknown message ids are not admitted.
// Owning layer/canonical suite: globaldb session input queue persistence.
// Invariant: pending monitor wakes share one row that reports every outcome, spends no input capacity,
// and keeps each wake in the admission ledger.
func TestGlobalDBSessionInputCoalescedWakes(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	wake := func(sessionID, id, text string, bound store.SessionInputCoalesce) store.SessionInputQueueInsert {
		return store.SessionInputQueueInsert{
			ID: "inq-" + id, SessionID: sessionID, MessageID: "monitor:" + id, Priority: 1,
			OwnerKind: store.SessionInputOwnerSynthetic, Text: text, TurnID: "turn-" + id,
			Mode: store.SessionInputQueueModeQueue, Delivery: store.SessionInputDeliveryAfterTurn,
			QueueCap: 1, Now: now,
			SyntheticPrompt: &store.SessionInputSyntheticPrompt{
				RunID: "run-" + id, Metadata: []byte(`{"kind":"monitor"}`), CoalesceKey: "monitor",
				Coalesce: &bound,
			},
		}
	}
	bound := store.SessionInputCoalesce{MaxFullBytes: 1 << 10, MaxMembers: 16}
	mustWake := func(t *testing.T, db *GlobalDB, insert store.SessionInputQueueInsert) store.SessionInputQueueEntry {
		t.Helper()
		entry, _, err := db.EnqueueSessionInput(t.Context(), insert)
		if err != nil {
			t.Fatalf("EnqueueSessionInput(%s) error = %v", insert.MessageID, err)
		}
		return entry
	}
	// UT-026: the paused queue retains a coalesced wake and re-applies the brake
	// between released turns. Store selection owns this invariant.
	t.Run("Should stop released wakes at the unattended limit", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "held-coalesced", "sess-held-coalesced")
		instance, err := db.Instances().Adopt(t.Context(), agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.SetUnattendedTurnLimit(1); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(t.Context(), instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		first := mustWake(t, db, wake(main, "a", "first monitor", bound))
		merged := mustWake(t, db, wake(main, "b", "second monitor", bound))
		if first.ID != merged.ID || merged.SyntheticPrompt.CoalescedWakes() != 2 {
			t.Fatalf("coalesced=%+v", merged)
		}
		separate := wake(main, "child", "child result", bound)
		separate.Now = now.Add(time.Second)
		separate.SyntheticPrompt.Coalesce = nil
		separate.SyntheticPrompt.CoalesceKey = ""
		separate.SyntheticPrompt.Metadata = []byte(`{"kind":"session_reply","reason":"subagent"}`)
		child := mustWake(t, db, separate)
		if _, found, err := db.PeekNextSessionInput(t.Context(), main); err != nil || found {
			t.Fatalf("paused peek=%v %v", found, err)
		}
		if _, err := db.Instances().Resume(t.Context(), instance.ID, 10, true); err != nil {
			t.Fatal(err)
		}
		claimed, found, err := db.ClaimNextSessionInput(t.Context(), main, now.Add(2*time.Second))
		if err != nil || !found || claimed.ID != first.ID || claimed.PausedInstance == nil {
			t.Fatalf("release=%+v %v %v", claimed, found, err)
		}
		if err := db.MarkSessionInputSent(t.Context(), main, claimed.ID, now.Add(3*time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := db.ClaimNextSessionInput(t.Context(), main, now.Add(4*time.Second)); err != nil || found {
			t.Fatalf("next wake escaped brake=%v %v", found, err)
		}
		if _, err := db.Instances().Resume(t.Context(), instance.ID, 10, true); err != nil {
			t.Fatal(err)
		}
		claimed, found, err = db.ClaimNextSessionInput(t.Context(), main, now.Add(5*time.Second))
		if err != nil || !found || claimed.ID != child.ID {
			t.Fatalf("child own turn=%+v %v %v", claimed, found, err)
		}
	})

	t.Run("Should merge more wakes than the cap and leave operator input accepted", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		var entry store.SessionInputQueueEntry
		for _, id := range []string{"a", "b", "c", "b"} {
			entry = mustWake(t, db, wake(sessionID, id, "outcome "+id, bound))
		}
		if entry.SyntheticPrompt.CoalescedWakes() != 3 {
			t.Fatalf("wakes = %d, want 3 with the replayed wake deduplicated", entry.SyntheticPrompt.CoalescedWakes())
		}
		for _, want := range []string{"outcome a", "outcome b", "outcome c"} {
			if strings.Count(entry.Text, want) != 1 {
				t.Fatalf("text = %q, want %q exactly once", entry.Text, want)
			}
		}
		for _, id := range []string{"monitor:a", "monitor:b", "monitor:c"} {
			if admitted, err := db.SyntheticSessionInputAdmitted(t.Context(), sessionID, id); err != nil || !admitted {
				t.Fatalf("%s admitted = %v, %v; want true", id, admitted, err)
			}
		}
		if _, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
			ID: "inq-operator", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue,
			Text: "operator", QueueCap: 1, Now: now,
		}); err != nil {
			t.Fatalf("operator input error = %v, want it accepted", err)
		}
		if _, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
			ID: "inq-agent", SessionID: sessionID, Mode: store.SessionInputQueueModeQueue, Text: "agent", QueueCap: 1,
			Now: now, Origin: json.RawMessage(`{"kind":"session","session_id":"sess-a","workspace_id":"ws","hop":1}`),
		}); err != nil {
			t.Fatalf("agent input error = %v, want the wake row outside its pool", err)
		}
	})

	t.Run("Should open a new row once the waiting row was claimed", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		first := mustWake(t, db, wake(sessionID, "a", "outcome a", bound))
		claimed, ok, err := db.ClaimNextSessionInput(t.Context(), sessionID, now)
		if err != nil || !ok || claimed.ID != first.ID {
			t.Fatalf("claim = %+v, %v, %v", claimed, ok, err)
		}
		second := mustWake(t, db, wake(sessionID, "b", "outcome b", bound))
		if second.ID == first.ID || strings.Contains(claimed.Text, "outcome b") {
			t.Fatalf("second = %+v, want its own row behind the claimed one", second)
		}
	})

	t.Run("Should drop the output past the byte budget and refuse past the member bound", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		small := store.SessionInputCoalesce{CompactText: "compact b", MaxFullBytes: 16, MaxMembers: 2}
		mustWake(t, db, wake(sessionID, "a", "outcome a", small))
		merged := mustWake(t, db, wake(sessionID, "b", strings.Repeat("x", 64), small))
		if !strings.Contains(merged.Text, "compact b") || strings.Contains(merged.Text, "xxx") {
			t.Fatalf("text = %q, want the compact outcome", merged.Text)
		}
		_, _, err := db.EnqueueSessionInput(t.Context(), wake(sessionID, "c", "outcome c", small))
		if !errors.Is(err, store.ErrSessionInputQueueFull) {
			t.Fatalf("error = %v, want ErrSessionInputQueueFull at the member bound", err)
		}
	})

	t.Run("Should keep a row queued before coalescing shipped as its own entry", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		legacy := wake(sessionID, "old", "outcome old", bound)
		legacy.SyntheticPrompt.Coalesce, legacy.SyntheticPrompt.CoalesceKey = nil, ""
		old := mustWake(t, db, legacy)
		merged := mustWake(t, db, wake(sessionID, "new", "outcome new", bound))
		if merged.ID == old.ID {
			t.Fatal("a new wake merged into a legacy row")
		}
		pending, err := db.ListPendingSessionInputs(t.Context(), sessionID)
		if err != nil || len(pending) != 2 {
			t.Fatalf("pending = %d, %v; want both rows delivered separately", len(pending), err)
		}
	})

	t.Run("Should requeue an unconfirmed promoted wake on boot recovery", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		entry := mustWake(t, db, wake(sessionID, "a", "outcome a", bound))
		_, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(t.Context(), sessionID, entry.ID, "turn-live", now)
		if err != nil || !promoted {
			t.Fatalf("promote = %v, %v; want promoted", promoted, err)
		}
		if _, reserved, err := db.ReserveSessionSteer(t.Context(), sessionID, entry.ID, now); err != nil || !reserved {
			t.Fatalf("reserve = %v, %v; want reserved", reserved, err)
		}
		if _, err := db.FailStaleDispatchingSessionInputs(t.Context(), now); err != nil {
			t.Fatal(err)
		}
		recovered, err := db.GetSessionInputQueueEntry(t.Context(), sessionID, entry.ID)
		if err != nil || recovered.Status != store.SessionInputQueueStatusQueued ||
			recovered.Mode != store.SessionInputQueueModeQueue || !recovered.Dispatchable {
			t.Fatalf("recovered = %+v, %v; want plain queued delivery", recovered, err)
		}
	})

	t.Run("Should requeue a promoted wake left pending injection on boot recovery", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		entry := mustWake(t, db, wake(sessionID, "a", "outcome a", bound))
		_, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(t.Context(), sessionID, entry.ID, "turn-live", now)
		if err != nil || !promoted {
			t.Fatalf("promote = %v, %v; want promoted", promoted, err)
		}
		if _, reserved, err := db.ReserveSessionSteer(t.Context(), sessionID, entry.ID, now); err != nil || !reserved {
			t.Fatalf("reserve = %v, %v; want reserved", reserved, err)
		}
		sent, err := db.ResolveSessionSteer(t.Context(), sessionID, entry.ID, store.SteerDeliveryPendingInjection, now)
		if err != nil || sent.Status != store.SessionInputQueueStatusSent {
			t.Fatalf("resolve = %+v, %v; want sent pending injection", sent, err)
		}
		if _, err := db.FailStaleDispatchingSessionInputs(t.Context(), now); err != nil {
			t.Fatal(err)
		}
		recovered, err := db.GetSessionInputQueueEntry(t.Context(), sessionID, entry.ID)
		if err != nil || recovered.Status != store.SessionInputQueueStatusQueued ||
			recovered.Mode != store.SessionInputQueueModeQueue || !recovered.Dispatchable ||
			recovered.SteerDelivery != "" || recovered.TurnID != "" || recovered.SentAt != nil {
			t.Fatalf("recovered = %+v, %v; want plain queued delivery", recovered, err)
		}
	})

	t.Run("Should fail an ordinary steer stranded by a restart", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		entry, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
			ID: "ordinary-steer", SessionID: sessionID, Text: "steer", TargetTurnID: "turn-live",
			Mode: store.SessionInputQueueModeSteer, Delivery: store.SessionInputDeliveryInterruptThenPrompt,
			QueueCap: 3, Now: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, reserved, err := db.ReserveSessionSteer(t.Context(), sessionID, entry.ID, now); err != nil || !reserved {
			t.Fatalf("reserve = %v, %v; want reserved", reserved, err)
		}
		failed, err := db.FailStaleDispatchingSessionInputs(t.Context(), now)
		if err != nil || failed != 1 {
			t.Fatalf("FailStaleDispatchingSessionInputs() = %d, %v; want 1", failed, err)
		}
		stranded, err := db.GetSessionInputQueueEntry(t.Context(), sessionID, entry.ID)
		if err != nil || stranded.Status != store.SessionInputQueueStatusFailed {
			t.Fatalf("stranded = %+v, %v; want failed by restart recovery", stranded, err)
		}
	})
}

func TestGlobalDBSyntheticSessionInputAdmitted(t *testing.T) {
	t.Parallel()
	t.Run("Should report queued and sent synthetic inputs as admitted", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
		entry, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
			ID: "monitor-wake", SessionID: sessionID, MessageID: "monitor:mon-aaaaaaaaaaaa", Priority: 1,
			OwnerKind: store.SessionInputOwnerSynthetic, Text: "wake", TurnID: "monitor-turn",
			Mode: store.SessionInputQueueModeQueue, Delivery: store.SessionInputDeliveryAfterTurn,
			QueueCap: 3, Now: now,
			SyntheticPrompt: &store.SessionInputSyntheticPrompt{
				RunID: "monitor:mon-aaaaaaaaaaaa", Metadata: []byte(`{"kind":"monitor","reason":"monitor_exit"}`),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		admitted, err := db.SyntheticSessionInputAdmitted(t.Context(), sessionID, "monitor:mon-aaaaaaaaaaaa")
		if err != nil || !admitted {
			t.Fatalf("queued admitted = %v, %v; want true", admitted, err)
		}
		claimed, ok, err := db.ClaimNextSessionInput(t.Context(), sessionID, now.Add(time.Millisecond))
		if err != nil || !ok || claimed.ID != entry.ID {
			t.Fatalf("claim = %+v, %v, %v; want the wake entry", claimed, ok, err)
		}
		if err := db.MarkSessionInputSent(t.Context(), sessionID, entry.ID, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		admitted, err = db.SyntheticSessionInputAdmitted(t.Context(), sessionID, "monitor:mon-aaaaaaaaaaaa")
		if err != nil || !admitted {
			t.Fatalf("sent admitted = %v, %v; want true", admitted, err)
		}
		other, err := db.SyntheticSessionInputAdmitted(t.Context(), sessionID, "monitor:mon-bbbbbbbbbbbb")
		if err != nil || other {
			t.Fatalf("unknown message admitted = %v, %v; want false", other, err)
		}
	})

	t.Run("Should not report a canceled synthetic input as admitted", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		sessionID := registerInputQueueSession(t, db)
		now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
		entry, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
			ID: "monitor-wake-canceled", SessionID: sessionID, MessageID: "monitor:mon-cccccccccccc", Priority: 1,
			OwnerKind: store.SessionInputOwnerSynthetic, Text: "wake", TurnID: "monitor-turn",
			Mode: store.SessionInputQueueModeQueue, Delivery: store.SessionInputDeliveryAfterTurn,
			QueueCap: 3, Now: now,
			SyntheticPrompt: &store.SessionInputSyntheticPrompt{
				RunID: "monitor:mon-cccccccccccc", Metadata: []byte(`{"kind":"monitor","reason":"monitor_exit"}`),
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.CancelSessionInput(t.Context(), sessionID, entry.ID, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		admitted, err := db.SyntheticSessionInputAdmitted(t.Context(), sessionID, "monitor:mon-cccccccccccc")
		if err != nil || admitted {
			t.Fatalf("canceled admitted = %v, %v; want false so the wake can be admitted again", admitted, err)
		}
	})
}

// UT-001, UT-025: paused queue selection retains autonomous input and permits human input.
func TestGlobalDBPausedInstanceInputSelection(t *testing.T) {
	// Invariant: definition pauses retain mains but release dispatched descendants; safety pauses hold both.
	for _, reason := range []string{"definition_missing", "definition_not_always_on", "manual", "fatigue", "unattended", "unknown", "definition_future"} {
		t.Run("Should preserve the lineage brake for "+reason, func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "definition-pause", "definition-main")
			instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
			parent := main
			ids := []string{main}
			for depth := 1; depth <= 2; depth++ {
				id := fmt.Sprintf("definition-child-%d", depth)
				if err := db.RegisterSession(ctx, &store.SessionInfo{
					ID: id, ProfileID: store.DefaultProfileID, WorkspaceID: workspace, AgentName: "coder",
					State: "active", RuntimeStatus: store.SessionRuntimeUnbound, CreatedAt: now, UpdatedAt: now,
					Lineage: &store.SessionLineage{ParentSessionID: parent, RootSessionID: main,
						SpawnDepth: depth, Kind: store.LineageKindSpawn},
				}); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
				parent = id
			}
			// An older main still belongs to the instance after rotation.
			if _, err := db.DB().
				ExecContext(ctx, "UPDATE agent_instances SET main_session_id=NULL WHERE id=?", instance.ID); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Instances().Pause(ctx, instance.ID, reason); err != nil {
				t.Fatal(err)
			}
			definition := reason == "definition_missing" || reason == "definition_not_always_on"
			for _, id := range ids {
				req := promptAdmissionRequest(workspace, id, "pause-"+id, now)
				req.Origin = []byte(`{"kind":"session","session_id":"sender","hop":1}`)
				receipt, _, _, _, err := db.EnqueueAdmittedSessionInput(ctx, req, store.SessionInputQueueInsert{
					ID: "peer-" + id, SessionID: id, Text: req.AuthoredText, QueueCap: 10, Now: now,
				})
				if err != nil {
					t.Fatal(err)
				}
				held := !definition || id == main
				if (receipt.Result.Delivery == "held") != held {
					t.Fatalf("receipt %s: %+v", id, receipt.Result)
				}
				if held && (receipt.Result.HeldReason != reason || receipt.Result.HeldInstanceID != instance.ID) {
					t.Fatalf("hold explanation: %+v", receipt.Result)
				}
				if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
					ID:          "monitor-" + id,
					SessionID:   id,
					Text:        "monitor wake",
					OriginClass: store.SessionInputOriginClassSynthetic,
					QueueCap:    10,
					Now:         now.Add(time.Second),
				}); err != nil {
					t.Fatal(err)
				}
				rows, err := db.ListPendingSessionInputs(ctx, id)
				if err != nil || len(rows) != 2 {
					t.Fatalf("inputs: %+v %v", rows, err)
				}
				hold, err := db.SessionInputHold(ctx, id)
				if err != nil || (hold != nil) != held {
					t.Fatalf("hold summary: %+v %v", hold, err)
				}
				if held && (hold.InstanceID != instance.ID || hold.Reason != reason || !hold.Since.Equal(now)) {
					t.Fatalf("hold summary: %+v", hold)
				}
				for _, row := range rows {
					if row.Held != held || (held && (row.HeldReason != reason || row.HeldInstanceID != instance.ID)) {
						t.Fatalf("input hold: %+v", row)
					}
				}
				if _, found, err := db.PeekNextSessionInput(ctx, id); err != nil || found == held {
					t.Fatalf("peek %s: found=%v held=%v err=%v", id, found, held, err)
				}
			}
			wantHeld := int64(6)
			if definition {
				wantHeld = 2
			}
			if held, err := db.Instances().HeldWakes(ctx, instance.ID); err != nil || held != wantHeld {
				t.Fatalf("held count=%d want=%d err=%v", held, wantHeld, err)
			}
			queued, err := db.Instances().QueuedSessions(ctx, instance.ID, "")
			if err != nil || len(queued) != map[bool]int{true: 2, false: 0}[definition] {
				t.Fatalf("release targets=%v err=%v", queued, err)
			}
			if definition {
				if err := db.SetUnattendedTurnLimit(1); err != nil {
					t.Fatal(err)
				}
				claimed, found, err := db.ClaimNextSessionInput(ctx, ids[1], now.Add(2*time.Second))
				if err != nil || !found || claimed.PausedInstance == nil ||
					claimed.PausedInstance.Reason != "unattended" {
					t.Fatalf("descendant brake: %+v %v %v", claimed, found, err)
				}
			}
			for _, id := range ids {
				if _, found, err := db.ClaimNextSessionInput(ctx, id, now.Add(3*time.Second)); err != nil || found {
					t.Fatalf("safety brake %s: %v %v", id, found, err)
				}
			}
		})
	}

	// Invariant (UT-002, UT-027): holds follow durable lineage through both depths and old mains;
	// archive cancels every held row. Owning layer: globaldb queue selection.
	// Invariant (UT-084): a held backlog above the configured queue limit never blocks a human prompt,
	// which is admitted and selected ahead of the held rows.
	t.Run("Should admit a human prompt behind an overfull held backlog", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "overfull-held", "sess-overfull-main")
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		for i := range 3 {
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: fmt.Sprintf("held-%d", i), SessionID: main, Text: "peer work",
				Origin:   json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`),
				QueueCap: 3, Now: now.Add(time.Duration(i) * time.Second),
			}); err != nil {
				t.Fatal(err)
			}
		}
		// The configured limit is now below the held backlog; a further agent row is refused.
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "held-over", SessionID: main, Text: "peer work",
			Origin:   json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`),
			QueueCap: 2, Now: now.Add(time.Minute),
		}); !errors.Is(err, store.ErrSessionInputQueueFull) {
			t.Fatalf("over-limit agent row error = %v, want queue full", err)
		}
		if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
			ID: "human", SessionID: main, Text: "operator", QueueCap: 2, Now: now.Add(time.Hour),
		}); err != nil {
			t.Fatalf("human prompt behind the held backlog: %v", err)
		}
		claimed, found, err := db.ClaimNextSessionInput(ctx, main, now.Add(2*time.Hour))
		if err != nil || !found || claimed.ID != "human" {
			t.Fatalf("claim = %q/%v/%v, want the human prompt", claimed.ID, found, err)
		}
		for i := range 3 {
			entry, err := db.GetSessionInputQueueEntry(ctx, main, fmt.Sprintf("held-%d", i))
			if err != nil || entry.Status != store.SessionInputQueueStatusQueued {
				t.Fatalf("held row %d = %+v, %v", i, entry, err)
			}
		}
		if _, found, err := db.ClaimNextSessionInput(ctx, main, now.Add(3*time.Hour)); err != nil || found {
			t.Fatalf("held claim after the human prompt = %v, %v", found, err)
		}
	})
	for _, creator := range []string{"human", "agent"} {
		t.Run("Should hold and archive descendants of a "+creator+" created main after rotation", func(t *testing.T) {
			t.Parallel()
			db := openTestGlobalDB(t)
			ctx := t.Context()
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "paused-lineage", "sess-lineage-main")
			instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
				PermissionCeiling: "{}", CreatedByKind: creator, CreatedByID: "creator",
			}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			parent := main
			ids := []string{main}
			for depth := 1; depth <= 2; depth++ {
				id := fmt.Sprintf("lineage-child-%d", depth)
				if err := db.RegisterSession(ctx, &store.SessionInfo{
					ID:            id,
					ProfileID:     store.DefaultProfileID,
					WorkspaceID:   workspace,
					AgentName:     "coder",
					State:         "active",
					RuntimeStatus: store.SessionRuntimeUnbound,
					CreatedAt:     now,
					UpdatedAt:     now,
					Lineage: &store.SessionLineage{
						ParentSessionID: parent, RootSessionID: main, SpawnDepth: depth, Kind: store.LineageKindSpawn,
					},
				}); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
				parent = id
			}
			successor := "sess-lineage-successor"
			if err := db.RegisterSession(ctx, &store.SessionInfo{
				ID: successor, ProfileID: store.DefaultProfileID, WorkspaceID: workspace,
				AgentName: "coder", State: "active", RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				t.Fatal(err)
			}
			// Selection concerns old-main lineage; rotation publication itself has its canonical suite.
			if _, err := db.DB().
				ExecContext(ctx, "UPDATE agent_instances SET main_session_id=? WHERE id=?", successor, instance.ID); err != nil {
				t.Fatal(err)
			}
			for _, id := range ids {
				if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
					ID:        "held-" + id,
					SessionID: id,
					Text:      "work",
					Origin:    json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`),
					QueueCap:  2,
					Now:       now,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
				t.Fatal(err)
			}
			for _, id := range ids {
				if _, found, err := db.PeekNextSessionInput(ctx, id); err != nil || found {
					t.Fatalf("peek %s=%v %v", id, found, err)
				}
				if _, found, err := db.ClaimNextSessionInput(ctx, id, now); err != nil || found {
					t.Fatalf("claim %s=%v %v", id, found, err)
				}
			}
			if held, err := db.Instances().HeldWakes(ctx, instance.ID); err != nil || held != 3 {
				t.Fatalf("held=%d %v", held, err)
			}
			metrics, err := db.Instances().Metrics(ctx, store.DefaultProfileID, workspace)
			if err != nil || metrics.HeldInputCount != 3 || metrics.PauseCounts["manual"] != 1 {
				t.Fatalf("scoped metrics=%+v %v", metrics, err)
			}
			other, err := db.Instances().Metrics(ctx, "another-profile", workspace)
			if err != nil || other.HeldInputCount != 0 || len(other.PauseCounts) != 0 {
				t.Fatalf("metrics leaked profile=%+v %v", other, err)
			}

			if _, err := db.Instances().Archive(ctx, instance.ID); err != nil {
				t.Fatal(err)
			}
			for _, id := range ids {
				row, err := db.GetSessionInputQueueEntry(ctx, id, "held-"+id)
				if err != nil || row.Status != store.SessionInputQueueStatusCanceled {
					t.Fatalf("archive row=%+v %v", row, err)
				}
			}
		})
	}
	// Invariant: a paused prepared Goal cannot strand a released descendant's own continuations.
	t.Run("Should select child continuation behind a prepared Goal during a definition pause", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "child-goal", "child-goal-main")
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
		child := "goal-child"
		if err := db.RegisterSession(ctx, &store.SessionInfo{
			ID:            child,
			ProfileID:     store.DefaultProfileID,
			WorkspaceID:   workspace,
			AgentName:     "coder",
			State:         "active",
			RuntimeStatus: store.SessionRuntimeUnbound,
			CreatedAt:     now,
			UpdatedAt:     now,
			Lineage: &store.SessionLineage{
				ParentSessionID: main,
				RootSessionID:   main,
				SpawnDepth:      1,
				Kind:            store.LineageKindSpawn,
			},
		}); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"prepared-child-goal", "child-monitor", "child-peer"} {
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: id, SessionID: child, Text: id, OriginClass: store.SessionInputOriginClassSynthetic,
				QueueCap: 3, Now: now,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.DB().
			ExecContext(ctx, "UPDATE session_input_queue SET owner_kind='goal',dispatchable=0,goal_input_batch_max_rowid=0 WHERE id='prepared-child-goal'"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "definition_missing"); err != nil {
			t.Fatal(err)
		}
		peek, found, err := db.PeekNextSessionInput(ctx, child)
		if err != nil || !found || peek.ID != "child-monitor" {
			t.Fatalf("peek: %+v %v %v", peek, found, err)
		}
		claimed, found, err := db.ClaimNextSessionInput(ctx, child, now.Add(time.Second))
		if err != nil || !found || claimed.ID != "child-monitor" {
			t.Fatalf("claim: %+v %v %v", claimed, found, err)
		}
		if err := db.MarkSessionInputSent(ctx, child, claimed.ID, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		claimed, found, err = db.ClaimNextSessionInput(ctx, child, now.Add(2*time.Second))
		if err != nil || !found || claimed.ID != "child-peer" {
			t.Fatalf("peer claim: %+v %v %v", claimed, found, err)
		}
	})
	t.Run("Should select a human message behind a prepared Goal while paused", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "paused-goal", "sess-paused-goal")
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID:         store.DefaultProfileID,
			AgentName:         "coder",
			WorkspaceID:       workspace,
			PermissionCeiling: "{}",
			CreatedByKind:     "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"prepared-goal", "human-after-goal"} {
			if _, _, err := db.EnqueueSessionInput(
				ctx,
				store.SessionInputQueueInsert{ID: id, SessionID: main, Text: id, QueueCap: 3, Now: time.Now()},
			); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.DB().
			ExecContext(ctx, "UPDATE session_input_queue SET owner_kind='goal',origin_class='synthetic',dispatchable=0,goal_input_batch_max_rowid=0 WHERE id='prepared-goal'"); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		peek, found, err := db.PeekNextSessionInput(ctx, main)
		if err != nil || !found || peek.ID != "human-after-goal" {
			t.Fatalf("peek=%+v %v %v", peek, found, err)
		}
		claimed, found, err := db.ClaimNextSessionInput(ctx, main, time.Now())
		if err != nil || !found || claimed.ID != peek.ID {
			t.Fatalf("claim=%+v %v %v", claimed, found, err)
		}
	})
	t.Run("Should hold agent input across peek claim batch and promotion until resume", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "paused-input", "sess-paused-input")
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
		for _, input := range []store.SessionInputQueueInsert{
			{ID: "held", SessionID: main, Text: "agent", Origin: json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`), QueueCap: 1, Now: now},
			{ID: "human", SessionID: main, Text: "human", QueueCap: 1, Now: now.Add(time.Second)},
		} {
			if _, _, err := db.EnqueueSessionInput(ctx, input); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Instances().Pause(ctx, instance.ID, "manual"); err != nil {
			t.Fatal(err)
		}
		if held, err := db.Instances().HeldWakes(ctx, instance.ID); err != nil || held != 1 {
			t.Fatalf("held wakes = %d, %v", held, err)
		}
		peek, found, err := db.PeekNextSessionInput(ctx, main)
		if err != nil || !found || peek.ID != "human" {
			t.Fatalf("peek = %+v, %v, %v", peek, found, err)
		}
		claimed, found, err := db.ClaimNextSessionInput(ctx, main, now)
		if err != nil || !found || claimed.ID != "human" {
			t.Fatalf("claim = %+v, %v, %v", claimed, found, err)
		}
		batch, err := db.ClaimSessionInputBatch(
			ctx,
			main,
			10,
			func(*store.SessionInputQueueEntry) bool { return true },
			now,
		)
		if err != nil || len(batch) != 0 {
			t.Fatalf("held batch = %+v, %v", batch, err)
		}
		if _, found, err := db.PeekNextSessionInput(ctx, main); err != nil || found {
			t.Fatalf("held peek = %v, %v", found, err)
		}
		if _, found, err := db.ClaimNextSessionInput(ctx, main, now); err != nil || found {
			t.Fatalf("held claim = %v, %v", found, err)
		}
		if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
			ctx,
			main,
			"held",
			"turn-live",
			now,
		); err != nil ||
			promoted {
			t.Fatalf("held promotion = %v, %v", promoted, err)
		}
		if _, err := db.Instances().Resume(ctx, instance.ID, 10, true); err != nil {
			t.Fatal(err)
		}
		if held, err := db.Instances().HeldWakes(ctx, instance.ID); err != nil || held != 0 {
			t.Fatalf("resumed held wakes = %d, %v", held, err)
		}
		claimed, found, err = db.ClaimNextSessionInput(ctx, main, now)
		if err != nil || !found || claimed.ID != "held" {
			t.Fatalf("released claim = %+v, %v, %v", claimed, found, err)
		}
	})
}

// UT-004, UT-006: queue claims count once and atomically stop subsequent autonomous dispatch.
func TestGlobalDBInstanceUnattendedBrake(t *testing.T) {
	// Invariant (UT-004): one turn's compatible batch advances the owning instance exactly once.
	// The canonical queue suite owns dispatch accounting, independently of transport.
	t.Run("Should count a three row batch as one unattended turn", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		db := openTestGlobalDB(t)
		workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "batch-brake", "sess-batch-brake")
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Now().UTC()
		for i := range 3 {
			if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
				ID: fmt.Sprintf("batch-%d", i), SessionID: main, Text: "work",
				Origin: json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`), QueueCap: 3, Now: now,
			}); err != nil {
				t.Fatal(err)
			}
		}
		if _, found, err := db.ClaimNextSessionInput(ctx, main, now); err != nil || !found {
			t.Fatalf("head=%v %v", found, err)
		}
		batch, err := db.ClaimSessionInputBatch(
			ctx,
			main,
			10,
			func(*store.SessionInputQueueEntry) bool { return true },
			now,
		)
		if err != nil || len(batch) != 2 {
			t.Fatalf("batch=%+v %v", batch, err)
		}
		wake, err := db.Instances().WakeState(ctx, instance.ID)
		if err != nil || wake.UnattendedTurns != 1 {
			t.Fatalf("turns=%+v %v", wake, err)
		}
	})
	// Invariant (UT-004/UT-026): a steering reservation is not a turn claim. When injection is refused, the
	// row's first standalone dispatch is counted once; a retry of that claimed turn is not counted again.
	for _, pending := range []bool{false, true} {
		name := "Should count the first standalone turn after a refused steering injection"
		if pending {
			name = "Should count the first standalone turn after a pending injection falls back"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			main := registerInputQueueSession(t, db)
			instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: "ws-input-queue-workspace",
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
			admission := promptAdmissionRequest("ws-input-queue-workspace", main, "peer-steer", now)
			admission.Origin = json.RawMessage(`{"kind":"session","session_id":"peer","workspace_id":"ws","hop":1}`)
			if _, _, _, _, err := db.EnqueueAdmittedSessionInput(ctx, admission, store.SessionInputQueueInsert{
				ID: "peer-steer", SessionID: main, Text: "work", QueueCap: 10, Now: now,
			}); err != nil {
				t.Fatal(err)
			}
			if _, promoted, err := db.PromoteQueuedAgentSessionInputToSteer(
				ctx, main, "peer-steer", "turn-live", now,
			); err != nil || !promoted {
				t.Fatalf("promote = %v, %v", promoted, err)
			}
			if _, reserved, err := db.ReserveSessionSteer(ctx, main, "peer-steer", now); err != nil || !reserved {
				t.Fatalf("reserve = %v, %v", reserved, err)
			}
			if pending {
				if _, err := db.ResolveSessionSteer(
					ctx, main, "peer-steer", store.SteerDeliveryPendingInjection, now,
				); err != nil {
					t.Fatal(err)
				}
				if _, settled, err := db.SettlePendingSessionSteer(
					ctx, main, "peer-steer", store.SteerDeliveryInterruptFallback, now,
				); err != nil || !settled {
					t.Fatalf("fallback = %v, %v", settled, err)
				}
			}
			reverted, err := db.RevertPromotedAgentSessionInput(ctx, main, "peer-steer", now)
			if err != nil || !reverted {
				t.Fatalf("revert = %v, %v", reverted, err)
			}
			if wake, err := db.Instances().WakeState(ctx, instance.ID); err != nil || wake.UnattendedTurns != 0 {
				t.Fatalf("steering reservation counted = %+v, %v", wake, err)
			}
			claimed, found, err := db.ClaimNextSessionInput(ctx, main, now.Add(time.Second))
			if err != nil || !found || claimed.ID != "peer-steer" || claimed.AttemptCount != 1 {
				t.Fatalf("standalone claim = %+v, %v, %v", claimed, found, err)
			}
			if wake, err := db.Instances().WakeState(ctx, instance.ID); err != nil || wake.UnattendedTurns != 1 {
				t.Fatalf("standalone turn not counted = %+v, %v", wake, err)
			}
			if err := db.ReleaseSessionInput(ctx, main, claimed.ID, now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, found, err := db.ClaimNextSessionInput(ctx, main, now.Add(2*time.Second)); err != nil || !found {
				t.Fatalf("retry claim = %v, %v", found, err)
			}
			if wake, err := db.Instances().WakeState(ctx, instance.ID); err != nil || wake.UnattendedTurns != 1 {
				t.Fatalf("retried turn counted again = %+v, %v", wake, err)
			}
		})
	}
	// Invariant (UT-026, DBE2-2): v0.4.8 counted steering reservations in attempt_count and left a reverted
	// fallback indistinguishable from a released claim. No release ran the brake, so an upgraded row is
	// counted once on its first claim, keeps its retained attempt_count, and its retry is not counted again.
	t.Run("Should count a released fallback row once after upgrade and keep its attempt count", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00164_schema.sql"))
		if err != nil {
			t.Fatalf("open v0.4.8 prefix: %v", err)
		}
		ctx := globalMigrationTestContext(t)
		const main = "sess-released-fallback"
		seeded := store.FormatTimestamp(time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC))
		for _, statement := range []string{
			`INSERT INTO workspaces (id, root_dir, add_dirs, name, created_at, updated_at)
			VALUES ('ws-released-fallback', '` + t.TempDir() + `', '[]', 'released-fallback', ?1, ?1)`,
			`INSERT INTO sessions (id, profile_id, name, agent_name, workspace_id, session_type, state, created_at, updated_at)
			VALUES ('` + main + `', '` + store.DefaultProfileID + `', 'Released', 'coder', 'ws-released-fallback',
			'user', 'active', ?1, ?1)`,
			// v0.4.8 reserved, refused and reverted this steer: mode queue, steer_delivery NULL, attempt 1.
			`INSERT INTO session_input_queue (id, session_id, status, mode, delivery, text, origin_class,
			origin_session_id, attempt_count, dispatchable, enqueued_at, updated_at)
			VALUES ('released-steer', '` + main + `', 'queued', 'queue', 'after_turn', 'work', 'agent', 'peer', 1, 1, ?1, ?1)`,
		} {
			if _, err := prefixDB.ExecContext(ctx, statement, seeded); err != nil {
				t.Fatalf("seed released row: %v", err)
			}
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatal(err)
		}
		db, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		t.Cleanup(func() {
			if err := db.Close(testutil.Context(t)); err != nil {
				t.Error(err)
			}
		})
		var attempts, counted int
		if err := db.db.QueryRowContext(ctx, `SELECT attempt_count, unattended_counted FROM session_input_queue
			WHERE id = 'released-steer'`).Scan(&attempts, &counted); err != nil || attempts != 1 || counted != 0 {
			t.Fatalf("upgraded row attempts=%d counted=%d err=%v", attempts, counted, err)
		}
		instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
			ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: "ws-released-fallback",
			PermissionCeiling: "{}", CreatedByKind: "human",
		}, main, 10)
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
		claimed, found, err := db.ClaimNextSessionInput(ctx, main, now)
		if err != nil || !found || claimed.ID != "released-steer" || claimed.AttemptCount != 2 {
			t.Fatalf("first claim after upgrade = %+v, %v, %v", claimed, found, err)
		}
		if wake, err := db.Instances().WakeState(ctx, instance.ID); err != nil || wake.UnattendedTurns != 1 {
			t.Fatalf("released row not counted = %+v, %v", wake, err)
		}
		if err := db.ReleaseSessionInput(ctx, main, claimed.ID, now.Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, found, err := db.ClaimNextSessionInput(ctx, main, now.Add(2*time.Second)); err != nil || !found {
			t.Fatalf("retry claim = %v, %v", found, err)
		}
		if wake, err := db.Instances().WakeState(ctx, instance.ID); err != nil || wake.UnattendedTurns != 1 {
			t.Fatalf("retried released row counted again = %+v, %v", wake, err)
		}
	})
	for _, limit := range []int64{0, 2, 300} {
		t.Run(fmt.Sprintf("Should count and enforce unattended limit %d", limit), func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			db := openTestGlobalDB(t)
			workspace, main := registerHeartbeatWorkspaceAndSession(t, db, "brake", "sess-brake")
			instance, err := db.Instances().Adopt(ctx, agentinstance.Instance{
				ProfileID: store.DefaultProfileID, AgentName: "coder", WorkspaceID: workspace,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, main, 10)
			if err != nil {
				t.Fatal(err)
			}
			if limit != 300 {
				if err := db.SetUnattendedTurnLimit(limit); err != nil {
					t.Fatal(err)
				}
			}
			now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
			if limit == 300 || limit == 0 {
				initial := 298
				if limit == 0 {
					initial = 1000
				}
				if _, err := db.DB().
					ExecContext(ctx, "UPDATE agent_instance_wake_state SET unattended_turns=? WHERE instance_id=?", initial, instance.ID); err != nil {
					t.Fatal(err)
				}
			}
			for i := range 3 {
				if _, _, err := db.EnqueueSessionInput(ctx, store.SessionInputQueueInsert{
					ID: fmt.Sprintf("agent-%d", i), SessionID: main, Text: "work",
					Origin: json.RawMessage(`{"kind":"session","session_id":"peer","hop":1}`), QueueCap: 10,
					Now: now.Add(time.Duration(i) * time.Second),
				}); err != nil {
					t.Fatal(err)
				}
			}
			for i := range 2 {
				entry, found, err := db.ClaimNextSessionInput(ctx, main, now)
				if err != nil || !found {
					t.Fatalf("claim %d = %+v %v %v", i, entry, found, err)
				}
				if i == 0 {
					if err := db.ReleaseSessionInput(ctx, main, entry.ID, now); err != nil {
						t.Fatal(err)
					}
					retried, found, retryErr := db.ClaimNextSessionInput(ctx, main, now)
					if retryErr != nil || !found || retried.ID != entry.ID || retried.PausedInstance != nil {
						t.Fatalf("retry = %+v %v %v", retried, found, retryErr)
					}
				}
				if (entry.PausedInstance != nil) != (limit != 0 && i == 1) {
					t.Fatalf("trip %d = %+v", i, entry.PausedInstance)
				}
				if err := db.MarkSessionInputSent(ctx, main, entry.ID, now); err != nil {
					t.Fatal(err)
				}
			}
			current, err := db.Instances().Get(ctx, instance.ID)
			if err != nil {
				t.Fatal(err)
			}
			if limit == 0 {
				if current.State != "active" {
					t.Fatalf("disabled brake paused: %+v", current)
				}
				return
			}
			if current.State != "paused" || current.PauseReason != "unattended" {
				t.Fatalf("trip state = %+v", current)
			}
			if _, found, err := db.ClaimNextSessionInput(ctx, main, now); err != nil || found {
				t.Fatalf("next claim = %v %v", found, err)
			}
			reports, err := db.InstanceReports().
				ListReports(ctx, agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil || len(reports) != 1 {
				t.Fatalf("reports = %+v %v", reports, err)
			}
			if _, err := db.Instances().Resume(ctx, instance.ID, 10, true); err != nil {
				t.Fatal(err)
			}
			state, err := db.Instances().WakeState(ctx, instance.ID)
			if err != nil || state.InstanceID != instance.ID {
				t.Fatalf("wake state = %+v %v", state, err)
			}
			var turns int64
			if err := db.DB().
				QueryRowContext(ctx, "SELECT unattended_turns FROM agent_instance_wake_state WHERE instance_id=?", instance.ID).
				Scan(&turns); err != nil ||
				turns != 0 {
				t.Fatalf("resume turns=%d err=%v", turns, err)
			}
		})
	}
}
