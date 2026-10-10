package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

// Invariant: only the turn that consumed the message settles its watch; owner: session reply service.
func TestReplyWatchReconcile(t *testing.T) {
	for _, tc := range []struct {
		name, outcome, text string
		fail, canceled      bool
	}{
		{name: "Should return the last assistant message UT-040", outcome: "completed", text: "final answer"},
		{name: "Should use the failure summary UT-043", outcome: "failed", text: "provider failed", fail: true},
		{name: "Should preserve cancellation UT-043", outcome: "canceled", text: "partial answer", canceled: true},
		{name: "Should bound Unicode text UT-041", outcome: "completed", text: strings.Repeat("界", 12001)},
		{name: "Should supply an empty reply body UT-041", outcome: "completed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, db, service, sender, target, w := replyWatchFixture(t)
			if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
				t.Fatal(err)
			}
			recordReplyEvent(
				t,
				h,
				target,
				acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "watched"}.WithMessageID(w.MessageID),
			)
			if tc.text != "" && !tc.fail {
				recordReplyEvent(
					t,
					h,
					target,
					acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: "watched", Text: tc.text},
				)
			}
			recordReplyEvent(
				t,
				h,
				target,
				acp.AgentEvent{
					Type:   acp.EventTypeAgentMessage,
					TurnID: "watched",
					Text:   "nested output",
				}.WithProviderToolMetadata(
					"nested-tool",
					"",
					"",
				),
			)
			if tc.fail {
				recordReplyEvent(
					t,
					h,
					target,
					acp.AgentEvent{Type: acp.EventTypeError, TurnID: "watched", Error: tc.text},
				)
			}
			done := acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"}
			if tc.canceled {
				done.PromptStopReason = acp.PromptStopReasonCancelled
			}
			recordReplyEvent(t, h, target, done)
			// A different turn must never contribute text, even if it finished later.
			recordReplyEvent(
				t,
				h,
				target,
				acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: "other", Text: "unrelated answer"},
			)
			service.OnTurnSettled(t.Context(), target.ID, "other")
			got, err := db.GetReplyWatch(t.Context(), w.ID)
			want := tc.text
			if want == "" {
				want = "(no reply text)"
			}
			cut := len([]rune(want)) > 12000
			if cut {
				want = string([]rune(want)[:12000])
			}
			if err != nil || got.State != "fired" || got.Outcome != tc.outcome || got.ReplyText != want ||
				got.ReplyTruncated != cut ||
				got.TurnID != "watched" {
				t.Fatalf("watch = %+v, %v", got, err)
			}
		})
	}
	t.Run("Should prefer a recorded turn over terminal queue evidence UT-043 UT-048", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		recordReplyEvent(
			t,
			h,
			target,
			acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "watched"}.WithMessageID(w.MessageID),
		)
		recordReplyEvent(
			t,
			h,
			target,
			acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: "watched", Text: "accepted"},
		)
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"})
		entry := replyQueuedInput(t, h, db, target, w)
		if _, err := db.CancelSessionInput(t.Context(), target.ID, entry.ID, h.manager.now()); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Go(func() { service.OnTurnSettled(t.Context(), target.ID, "watched") })
		wg.Go(func() { service.OnQueueEntryTerminal(t.Context(), &entry) })
		wg.Wait()
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != "completed" || got.ReplyText != "accepted" {
			t.Fatalf("precedence = %+v, %v", got, err)
		}
	})
	t.Run("Should retain queued input across a target stop then drop it on cancel UT-044 UT-043", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		entry := replyQueuedInput(t, h, db, target, w)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), target.ID); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "armed" {
			t.Fatalf("stopped queue = %+v, %v", got, err)
		}
		canceled, err := db.CancelSessionInput(t.Context(), target.ID, entry.ID, h.manager.now())
		if err != nil {
			t.Fatal(err)
		}
		service.OnQueueEntryTerminal(t.Context(), &canceled)
		got, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != "dropped" ||
			got.ReplyText != "The message was removed from the queue before it ran." {
			t.Fatalf("drop = %+v, %v", got, err)
		}
	})
	t.Run("Should wait for settle before resolving uncertain dispatch UT-043", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		if err := db.CommitSessionPromptDispatch(
			t.Context(),
			h.workspaceID,
			target.ID,
			"reply-key",
			h.manager.now(),
		); err != nil {
			t.Fatal(err)
		}
		if err := db.MarkSessionPromptAdmissionIndeterminate(
			t.Context(),
			h.workspaceID,
			target.ID,
			"reply-key",
			"uncertain",
			h.manager.now(),
		); err != nil {
			t.Fatal(err)
		}
		service.OnSendResult(t.Context(), w.ID, store.ErrSessionPromptDispatchIndeterminate)
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "armed" {
			t.Fatalf("premature = %+v, %v", got, err)
		}
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "next"})
		service.OnTurnSettled(t.Context(), target.ID, "next")
		got, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != "unknown" ||
			got.ReplyText != "The message may not have been delivered; check the target session." {
			t.Fatalf("unknown = %+v, %v", got, err)
		}
	})
	t.Run("Should abandon a precommit failure without a wake UT-043", func(t *testing.T) {
		t.Parallel()
		_, db, service, _, _, w := replyWatchFixture(t)
		service.OnSendResult(t.Context(), w.ID, errors.New("admission failed"))
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "abandoned" || got.AbandonReason != "send_failed" || got.DeliveredInputID != "" {
			t.Fatalf("failed send = %+v, %v", got, err)
		}
	})
}

func TestReplyWatchRecovery(t *testing.T) {
	t.Run("Should deliver a deferred reply on resume UT-045", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		recordReplyEvent(
			t,
			h,
			target,
			acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "watched"}.WithMessageID(w.MessageID),
		)
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"})
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "fired" {
			t.Fatalf("deferred = %+v, %v", got, err)
		}
		if _, err := h.manager.Resume(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "delivered" || got.DeliveredInputID == "" {
			t.Fatalf("delivery = %+v, %v", got, err)
		}
		entry, err := db.GetSessionInputQueueEntry(t.Context(), sender.ID, got.DeliveredInputID)
		if err != nil || entry.MessageID != "prompt-reply:"+w.ID || entry.Priority != 1 ||
			!strings.Contains(entry.Text, "(no reply text)") {
			t.Fatalf("wake = %+v, %v", entry, err)
		}
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should recover a missing target without blocking boot", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		replyQueuedInput(t, h, db, target, w)
		if err := h.manager.Stop(t.Context(), target.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.Delete(t.Context(), target.ID); err != nil {
			t.Fatal(err)
		}
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "fired" || got.Outcome != "unknown" {
			t.Fatalf("deleted target = %+v, %v", got, err)
		}
		if _, err := h.manager.Resume(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "delivered" {
			t.Fatalf("deleted target delivery = %+v, %v", got, err)
		}
		entry, err := db.GetSessionInputQueueEntry(t.Context(), sender.ID, got.DeliveredInputID)
		if err != nil || !strings.Contains(entry.Text, `Session "a deleted session"`) {
			t.Fatalf("deleted target wake = %+v, %v", entry, err)
		}
	})
	for _, action := range []string{"archive", "delete"} {
		t.Run("Should abandon watches when the sender is removed by "+action+" UT-046", func(t *testing.T) {
			t.Parallel()
			h, db, service, sender, _, w := replyWatchFixture(t)
			h.manager.SetReplyWatchService(service)
			if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
				t.Fatal(err)
			}
			if action == "archive" {
				if _, err := h.manager.Archive(t.Context(), h.workspaceID, sender.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := h.manager.Delete(t.Context(), sender.ID); err != nil {
					t.Fatal(err)
				}
			}
			if err := service.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			got, err := db.GetReplyWatch(t.Context(), w.ID)
			if err != nil || got.State != "abandoned" || got.AbandonReason != "sender_gone" {
				t.Fatalf("removed sender = %+v, %v", got, err)
			}
		})
	}
	t.Run("Should scope the pending watch list UT-047", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		for _, scope := range []struct {
			workspace, sender string
			count             int
		}{{h.workspaceID, sender.ID, 1}, {"foreign", sender.ID, 0}, {h.workspaceID, target.ID, 0}} {
			rows, err := service.ListForSender(t.Context(), scope.workspace, scope.sender)
			if err != nil || len(rows) != scope.count {
				t.Fatalf("list = %+v, %v", rows, err)
			}
		}
		if err := db.AbandonReplyWatch(t.Context(), w.ID, "send_failed"); err != nil {
			t.Fatal(err)
		}
		rows, err := service.ListForSender(t.Context(), h.workspaceID, sender.ID)
		if err != nil || len(rows) != 0 {
			t.Fatalf("terminal list = %+v, %v", rows, err)
		}
	})
}

// Invariant: a full sender queue defers the durable wake until a retry can hand it off once.
// Owner: session delivery; this suite owns the wake text, priority and metadata contract.
func TestReplyWatchQueueRetry(t *testing.T) {
	t.Run("Should retry a full queue with one truncated prioritized wake UT-041 UT-049", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.busyInput.QueueCap = 2
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		h.driver.cancelHook = func(*fakeProcess) error { once.Do(func() { close(release) }); return nil }
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.Message != "active" {
				return completedSyntheticPromptEvents(req.TurnID), nil
			}
			events := make(chan acp.AgentEvent, 1)
			go func() {
				<-release
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
				close(events)
			}()
			return events, nil
		}
		active, err := h.manager.Prompt(t.Context(), sender.ID, "active")
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"free-slot", "operator"} {
			if _, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
				ID: id, SessionID: sender.ID, Text: id, QueueCap: 2,
			}); err != nil {
				t.Fatal(err)
			}
		}
		text := strings.Repeat("界", 12001)
		recordReplyEvent(
			t,
			h,
			target,
			acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "watched"}.WithMessageID(w.MessageID),
		)
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: "watched", Text: text})
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"})
		service.OnTurnSettled(t.Context(), target.ID, "watched")
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "fired" || got.DeliveredInputID != "" {
			t.Fatalf("full queue = %+v, %v", got, err)
		}
		if _, err := db.CancelSessionInput(t.Context(), sender.ID, "free-slot", h.manager.now()); err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Go(func() { service.OnSenderTurnSettled(t.Context(), sender.ID) })
		wg.Go(func() {
			if err := service.Recover(t.Context()); err != nil {
				t.Error(err)
			}
		})
		wg.Wait()
		got, err = db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "delivered" {
			t.Fatalf("retry = %+v, %v", got, err)
		}
		rows, err := db.ListPendingSessionInputs(t.Context(), sender.ID)
		if err != nil || len(rows) != 2 || rows[0].ID != got.DeliveredInputID || rows[1].ID != "operator" {
			t.Fatalf("priority = %+v, %v", rows, err)
		}
		want := fmt.Sprintf(
			"Session %q (%s) replied to your message %s: completed.\n---\n%s\n[Reply truncated at 12000 characters. Read the full turn with compozy__session_history.]",
			target.Info().Name,
			target.ID,
			w.MessageID,
			strings.Repeat("界", 12000),
		)
		if rows[0].Text != want || rows[0].MessageID != "prompt-reply:"+w.ID || rows[0].SyntheticPrompt == nil {
			t.Fatalf("wake contract = %+v", rows[0])
		}
		var meta acp.PromptSyntheticMeta
		if err := json.Unmarshal(rows[0].SyntheticPrompt.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Kind != acp.PromptSyntheticKindSessionReply || !meta.ReplyTruncated || meta.Hop != 1 ||
			meta.WakeEventID != w.ID {
			t.Fatalf("metadata = %+v", meta)
		}
		once.Do(func() { close(release) })
		for range active {
			continue
		}
		if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReplyWatchSteerEvidence(t *testing.T) {
	t.Run(
		"Should keep pending injection and fallback open until the consuming turn settles UT-043",
		func(t *testing.T) {
			t.Parallel()
			h, db, service, sender, target, w := replyWatchFixture(t, store.SessionInputQueueModeSteer)
			if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
				t.Fatal(err)
			}
			_, entry, _, err := db.StageAdmittedSessionSteer(t.Context(), store.SessionPromptAdmissionRequest{
				ID:                 w.AdmissionID,
				WorkspaceID:        h.workspaceID,
				SessionID:          target.ID,
				MessageID:          w.MessageID,
				IdempotencyKey:     "reply-key",
				Operation:          store.SessionPromptOperationPrompt,
				FingerprintVersion: "test/v1",
				RequestFingerprint: "reply-fingerprint",
				Mode:               store.SessionInputQueueModeSteer,
				AuthoredText:       "question",
				TurnID:             "admitted-turn",
				EventID:            "input-event",
				Now:                h.manager.now(),
			}, store.SessionInputQueueInsert{
				ID: "steer", SessionID: target.ID, Text: "question", TargetTurnID: "original",
				Delivery: store.SessionInputDeliveryInterruptThenPrompt, QueueCap: 8, Now: h.manager.now(),
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, reserved, err := db.ReserveSessionSteer(
				t.Context(),
				target.ID,
				entry.ID,
				h.manager.now(),
			); err != nil ||
				!reserved {
				t.Fatalf("reserve = %v, %v", reserved, err)
			}
			if _, err := db.ResolveSessionSteer(
				t.Context(),
				target.ID,
				entry.ID,
				store.SteerDeliveryPendingInjection,
				h.manager.now(),
			); err != nil {
				t.Fatal(err)
			}
			recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "original"})
			service.OnTurnSettled(t.Context(), target.ID, "original")
			got, err := db.GetReplyWatch(t.Context(), w.ID)
			if err != nil || got.State != "armed" {
				t.Fatalf("pending = %+v, %v", got, err)
			}
			if _, changed, err := db.SettlePendingSessionSteer(
				t.Context(),
				target.ID,
				entry.ID,
				store.SteerDeliveryInterruptFallback,
				h.manager.now(),
			); err != nil ||
				!changed {
				t.Fatalf("fallback = %v, %v", changed, err)
			}
			service.OnSteerResolved(t.Context(), target.ID, w.MessageID)
			got, err = db.GetReplyWatch(t.Context(), w.ID)
			if err != nil || got.State != "armed" {
				t.Fatalf("fallback watch = %+v, %v", got, err)
			}
			recordReplyEvent(
				t,
				h,
				target,
				acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "fallback"}.WithMessageID(w.MessageID),
			)
			recordReplyEvent(
				t,
				h,
				target,
				acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: "fallback", Text: "fallback answer"},
			)
			recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "fallback"})
			service.OnTurnSettled(t.Context(), target.ID, "fallback")
			got, err = db.GetReplyWatch(t.Context(), w.ID)
			if err != nil || got.State != "fired" || got.TurnID != "fallback" || got.ReplyText != "fallback answer" {
				t.Fatalf("fallback result = %+v, %v", got, err)
			}
		},
	)
}

func replyWatchFixture(
	t *testing.T, mode ...string,
) (*harness, *globaldb.GlobalDB, ReplyWatchService, *Session, *Session, store.ReplyWatch) {
	t.Helper()
	db := openManagerInputQueueStore(t)
	h := newHarness(t, WithSessionInputQueueStore(db), WithSessionCatalog(db))
	registerManagerInputQueueWorkspace(t, db, h)
	cleanupTestManager(t, h.manager)
	sender, target := createSession(t, h), createSession(t, h)
	t.Cleanup(func() {
		for _, id := range []string{sender.ID, target.ID} {
			if _, live := h.manager.Get(id); live {
				if err := h.manager.Stop(context.Background(), id); err != nil {
					t.Error(err)
				}
			}
		}
	})
	now := h.manager.now()
	admissionMode := store.SessionInputQueueModeQueue
	if len(mode) > 0 {
		admissionMode = mode[0]
	}
	admission := store.SessionPromptAdmissionRequest{
		ID:                 "reply-admission",
		WorkspaceID:        h.workspaceID,
		SessionID:          target.ID,
		MessageID:          "reply-message",
		IdempotencyKey:     "reply-key",
		Operation:          store.SessionPromptOperationPrompt,
		FingerprintVersion: "test/v1",
		RequestFingerprint: "reply-fingerprint",
		Mode:               admissionMode,
		AuthoredText:       "question",
		TurnID:             "admitted-turn",
		EventID:            "input-event",
		Now:                now,
	}
	if _, _, err := db.ClaimSessionPromptAdmission(t.Context(), admission); err != nil {
		t.Fatal(err)
	}
	w, err := db.InsertReplyWatchTx(
		t.Context(),
		db.DB(),
		store.ReplyWatchRegistration{
			WorkspaceID:       h.workspaceID,
			SenderSessionID:   sender.ID,
			TargetWorkspaceID: h.workspaceID,
			TargetSessionID:   target.ID,
			MessageID:         admission.MessageID,
			AdmissionID:       admission.ID,
			Hop:               1,
			CreatedAt:         now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewReplyWatchService(db, h.manager)
	if err != nil {
		t.Fatal(err)
	}
	return h, db, service, sender, target, w
}

func recordReplyEvent(t *testing.T, h *harness, target *Session, event acp.AgentEvent) {
	t.Helper()
	event.SessionID, event.Timestamp = target.ID, h.manager.now()
	if err := h.manager.recordEvent(t.Context(), target, event); err != nil {
		t.Fatal(err)
	}
}

func replyQueuedInput(
	t *testing.T,
	h *harness,
	db *globaldb.GlobalDB,
	target *Session,
	w store.ReplyWatch,
) store.SessionInputQueueEntry {
	t.Helper()
	// Join the already reserved admission exactly as the queue admission owner does.
	req := store.SessionPromptAdmissionRequest{
		ID:                 w.AdmissionID,
		WorkspaceID:        h.workspaceID,
		SessionID:          target.ID,
		MessageID:          w.MessageID,
		IdempotencyKey:     "reply-key",
		Operation:          store.SessionPromptOperationPrompt,
		FingerprintVersion: "test/v1",
		RequestFingerprint: "reply-fingerprint",
		Mode:               store.SessionInputQueueModeQueue,
		AuthoredText:       "question",
		TurnID:             "admitted-turn",
		EventID:            "input-event",
		Now:                h.manager.now(),
	}
	admitted, entry, _, _, err := db.EnqueueAdmittedSessionInput(
		t.Context(),
		req,
		store.SessionInputQueueInsert{
			ID:        "target-queue",
			SessionID: target.ID,
			Text:      "question",
			QueueCap:  8,
			Now:       h.manager.now(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if admitted.ID != w.AdmissionID {
		t.Fatalf("admission = %+v", admitted)
	}
	return entry
}
