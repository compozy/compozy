package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/transcript"
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
		var meta acp.PromptSyntheticMeta
		if entry.SyntheticPrompt == nil || json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta) != nil ||
			meta.Summary != "" {
			t.Fatalf("empty reply summary = %+v; want an empty typed summary", entry.SyntheticPrompt)
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
		// The wake shares its cap pool with agent messages only, so agent rows fill it here.
		for _, id := range []string{"free-slot", "peer"} {
			if _, _, err := db.EnqueueSessionInput(t.Context(), store.SessionInputQueueInsert{
				ID: id, SessionID: sender.ID, Text: id, QueueCap: 2,
				Origin: []byte(`{"kind":"session","session_id":"sess-peer","workspace_id":"ws","hop":1}`),
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
		// Agent messages dispatch before synthetic wakes.
		if err != nil || len(rows) != 2 || rows[0].ID != "peer" || rows[1].ID != got.DeliveredInputID {
			t.Fatalf("priority = %+v, %v", rows, err)
		}
		wake := rows[1]
		want := fmt.Sprintf(
			"Session %q (%s) replied to your message %s: completed.\n---\n%s\n[Reply truncated at 12000 characters. Read the full turn with compozy__session_history.]",
			target.Info().Name,
			target.ID,
			w.MessageID,
			strings.Repeat("界", 12000),
		)
		if wake.Text != want || wake.MessageID != "prompt-reply:"+w.ID || wake.SyntheticPrompt == nil {
			t.Fatalf("wake contract = %+v", wake)
		}
		var meta acp.PromptSyntheticMeta
		if err := json.Unmarshal(wake.SyntheticPrompt.Metadata, &meta); err != nil {
			t.Fatal(err)
		}
		if meta.Kind != acp.PromptSyntheticKindSessionReply || !meta.ReplyTruncated || meta.Hop != 0 ||
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
	return replyWatchFixtureWith(t, nil, mode...)
}

func replyWatchFixtureWith(
	t *testing.T, opts []Option, mode ...string,
) (*harness, *globaldb.GlobalDB, ReplyWatchService, *Session, *Session, store.ReplyWatch) {
	t.Helper()
	db := openManagerInputQueueStore(t)
	h := newHarness(t, append([]Option{WithSessionInputQueueStore(db), WithSessionCatalog(db)}, opts...)...)
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
	if event.Type == acp.EventTypeUserMessage && event.MessageIDValue() == "reply-message" {
		if err := h.manager.recordPromptInputWithAuthoredText(
			t.Context(),
			target,
			event,
			event.Text,
			"input-event",
		); err != nil {
			t.Fatal(err)
		}
		return
	}
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

// Invariant: real admission faults preserve the dispatch boundary and never invent a reply.
// Owner: session admission/service; canonical reply suite (IT-019).
func TestReplyWatchAdmissionFaults(t *testing.T) {
	for _, fault := range []string{"precommit", "before input", "after input"} {
		t.Run("Should reconcile a real fault "+fault, func(t *testing.T) {
			t.Parallel()
			h, db, service, sender, target, _ := replyWatchFixture(t)
			h.manager.SetReplyWatchService(service)
			if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "precommit":
				_, err := db.DB().
					ExecContext(t.Context(), `CREATE TRIGGER reject_reply_dispatch BEFORE UPDATE OF state ON session_prompt_admissions WHEN NEW.message_id='fault-message' AND NEW.state='dispatch_committed' BEGIN SELECT RAISE(ABORT, 'dispatch commit fault'); END`)
				if err != nil {
					t.Fatal(err)
				}
			case "before input":
				h.manager.hooks = fullHookSet(
					&spyHookDispatcher{
						dispatchInputPreSubmitFn: func(_ context.Context, p hookspkg.InputPreSubmitPayload) (hookspkg.InputPreSubmitPayload, error) {
							if p.SessionID == target.ID {
								return p, errors.New("input hook fault")
							}
							return p, nil
						},
					},
				)
			case "after input":
				h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					return nil, errors.New("provider dispatch fault")
				}
			}
			opts := SendPromptOpts{
				Message:          "question",
				MessageID:        "fault-message",
				IdempotencyKey:   "fault-key",
				NotifyOnComplete: true,
				Origin: &acp.PromptOriginMeta{
					Kind:        "session",
					SessionID:   sender.ID,
					WorkspaceID: h.workspaceID,
					Hop:         3,
				},
			}
			_, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
			if err == nil {
				t.Fatal("fault did not fail the send")
			}
			id := store.ReplyWatchID(target.ID, "fault-message")
			watch, getErr := db.GetReplyWatch(t.Context(), id)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if fault == "precommit" {
				if errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) || watch.State != "abandoned" ||
					watch.AbandonReason != "send_failed" {
					t.Fatalf("precommit = %+v, %v", watch, err)
				}
				if _, err := db.DB().ExecContext(t.Context(), "DROP TRIGGER reject_reply_dispatch"); err != nil {
					t.Fatal(err)
				}
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					return completedSyntheticPromptEvents(req.TurnID), nil
				}
				result, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
				if err != nil || result.ReplyWatch == nil || result.ReplyWatch.ID != id ||
					result.ReplyWatch.State == "abandoned" {
					t.Fatalf("retry = %+v, %v", result, err)
				}
				collectEvents(t, result.Events)
				waitForCondition(
					t,
					"retry settles",
					func() bool { w, err := db.GetReplyWatch(t.Context(), id); return err == nil && w.State == "fired" },
				)
				replay, err := h.manager.SendPrompt(t.Context(), target.ID, opts)
				if err != nil || replay.ReplyWatch.State != "fired" {
					t.Fatalf("replay state = %+v, %v", replay, err)
				}
				if _, err := h.manager.Resume(t.Context(), sender.ID); err != nil {
					t.Fatal(err)
				}
				waitForCondition(
					t,
					"retry delivers once",
					func() bool { w, err := db.GetReplyWatch(t.Context(), id); return err == nil && w.State == "delivered" },
				)
				if err := service.Recover(t.Context()); err != nil {
					t.Fatal(err)
				}
				waitForCondition(t, "one retry wake", func() bool {
					rows, err := h.manager.Events(
						t.Context(),
						sender.ID,
						store.EventQuery{Type: acp.EventTypeSyntheticReentry, Limit: 100},
					)
					return err == nil && len(rows) == 1
				})
				return
			}
			if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) || watch.State != "armed" {
				t.Fatalf("postcommit = %+v, %v", watch, err)
			}
			rows, err := h.manager.Events(
				t.Context(),
				target.ID,
				store.EventQuery{Type: acp.EventTypeUserMessage, Limit: 100},
			)
			if err != nil {
				t.Fatal(err)
			}
			wantInputs := 0
			if fault == "after input" {
				wantInputs = 1
			}
			if len(rows) != wantInputs {
				t.Fatalf("inputs = %d, want %d", len(rows), wantInputs)
			}
			if err := h.manager.Stop(t.Context(), target.ID); err != nil {
				t.Fatal(err)
			}
			waitForCondition(t, "reply reconciled after stop finalization", func() bool {
				current, err := db.GetReplyWatch(t.Context(), id)
				return err == nil && current.State == store.ReplyWatchFired
			})
			watch, err = db.GetReplyWatch(t.Context(), id)
			want := "unknown"
			if fault == "after input" {
				want = "canceled"
			}
			if err != nil || watch.State != "fired" || watch.Outcome != want {
				t.Fatalf("settled = %+v, %v", watch, err)
			}
			if fault == "before input" {
				if _, err := h.manager.Resume(t.Context(), sender.ID); err != nil {
					t.Fatal(err)
				}
				waitForCondition(t, "unknown wake delivered", func() bool {
					rows, err := h.manager.Events(
						t.Context(),
						sender.ID,
						store.EventQuery{Type: acp.EventTypeSyntheticReentry, Limit: 100},
					)
					return err == nil && len(rows) == 1 &&
						strings.Contains(
							rows[0].Content,
							"The message may not have been delivered; check the target session.",
						)
				})
			}
		})
	}
}

// The store decorator gates the result write, leaving real admission and event transactions intact.
type replyAdmissionCompletionBarrier struct {
	store.SessionPromptAdmissionStore
	beforeComplete func(context.Context) error
}

func (b replyAdmissionCompletionBarrier) CompleteSessionPromptAdmission(
	ctx context.Context,
	workspace, target, key string,
	result store.SessionPromptAdmissionResult,
	now time.Time,
) (store.SessionPromptAdmission, error) {
	if err := b.beforeComplete(ctx); err != nil {
		return store.SessionPromptAdmission{}, err
	}
	return b.SessionPromptAdmissionStore.CompleteSessionPromptAdmission(ctx, workspace, target, key, result, now)
}

// Invariant: settlement before the result receipt binds still fires exactly once (IT-017).
func TestReplyWatchFastSettlement(t *testing.T) {
	t.Run("Should reconcile a completed turn before returning its admission receipt", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, _ := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		id := store.ReplyWatchID(target.ID, "fast-message")
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Text: "fast answer"}
			events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
			close(events)
			return events, nil
		}
		h.manager.promptAdmissionStore = replyAdmissionCompletionBarrier{
			SessionPromptAdmissionStore: db,
			beforeComplete: func(ctx context.Context) error {
				if err := h.manager.WaitForPromptDrains(ctx); err != nil {
					return err
				}
				w, err := db.GetReplyWatch(ctx, id)
				if err != nil {
					return err
				}
				if w.State != "fired" || w.ReplyText != "fast answer" {
					return fmt.Errorf("pre-receipt watch = %+v", w)
				}
				return nil
			},
		}
		result, err := h.manager.SendPrompt(
			t.Context(),
			target.ID,
			SendPromptOpts{
				Message:          "fast",
				MessageID:        "fast-message",
				IdempotencyKey:   "fast-key",
				NotifyOnComplete: true,
				Origin: &acp.PromptOriginMeta{
					Kind:        "session",
					SessionID:   sender.ID,
					WorkspaceID: h.workspaceID,
					Hop:         1,
				},
			},
		)
		if err != nil || result.ReplyWatch == nil || result.ReplyWatch.ID != id {
			t.Fatal(result, err)
		}
		for range result.Events {
			continue
		}
		replay, err := h.manager.SendPrompt(
			t.Context(),
			target.ID,
			SendPromptOpts{
				Message:          "fast",
				MessageID:        "fast-message",
				IdempotencyKey:   "fast-key",
				NotifyOnComplete: true,
				Origin: &acp.PromptOriginMeta{
					Kind:        "session",
					SessionID:   sender.ID,
					WorkspaceID: h.workspaceID,
					Hop:         1,
				},
			},
		)
		if err != nil || !replay.Replayed || replay.ReplyWatch == nil || *replay.ReplyWatch != *result.ReplyWatch {
			t.Fatal(replay, err)
		}
		if len(managerPromptCalls(h)) != 1 {
			t.Fatal("replay started another prompt")
		}
	})
}

// Invariant: an unconsumed pending steer follows fallback, or drops when stop supersedes it, never the old turn (IT-018).
func TestReplyWatchSteerSupersession(t *testing.T) {
	for _, requeue := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should resolve pending guidance with requeue=%t", requeue), func(t *testing.T) {
			t.Parallel()
			h, db, service, sender, target, _ := replyWatchFixture(t)
			h.manager.SetReplyWatchService(service)
			if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
				t.Fatal(err)
			}
			target.processHandle().caps.SteerCapability = compozyconfig.SteerCapabilityExtension
			completion := make(chan error, 2)
			release := make(chan struct{})
			var once sync.Once
			t.Cleanup(func() { close(completion); once.Do(func() { close(release) }) })
			h.manager.driver = &steeringTestDriver{
				fakeDriver: h.driver,
				completion: completion,
				steer: func(context.Context, *AgentProcess, string, string) (acp.SteerAttempt, error) {
					return acp.SteerAttemptPendingInjection, nil
				},
			}
			h.driver.cancelHook = func(*fakeProcess) error { once.Do(func() { close(release) }); return nil }
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent, 2)
				go func() {
					defer close(events)
					if req.Message == "hold" {
						<-release
					}
					events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Text: "new turn answer"}
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
				}()
				return events, nil
			}
			held, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{Message: "hold"})
			if err != nil {
				t.Fatal(err)
			}
			original := target.CurrentTurnID()
			result, err := h.manager.SendPrompt(
				t.Context(),
				target.ID,
				SendPromptOpts{
					Message:          "guidance",
					MessageID:        "pending-message",
					IdempotencyKey:   "pending-key",
					Mode:             BusyInputModeSteer,
					NotifyOnComplete: true,
					Origin: &acp.PromptOriginMeta{
						Kind:        "session",
						SessionID:   sender.ID,
						WorkspaceID: h.workspaceID,
						Hop:         2,
					},
				},
			)
			if err != nil || result.SteerDelivery != store.SteerDeliveryPendingInjection || result.ReplyWatch == nil {
				t.Fatal(result, err)
			}
			watch, err := db.GetReplyWatch(t.Context(), result.ReplyWatch.ID)
			if err != nil || watch.State != "armed" {
				t.Fatal(watch, err)
			}
			if requeue {
				completion <- errors.New("pending injection superseded by provider")
				// An agent steer degrades to a queued turn, so it dispatches once the held turn ends.
				once.Do(func() { close(release) })
			} else {
				if err := h.manager.Stop(t.Context(), target.ID); err != nil {
					t.Fatal(err)
				}
				completion <- errors.New("pending injection superseded by stop")
			}
			waitForCondition(t, "pending watch resolved", func() bool {
				w, e := db.GetReplyWatch(t.Context(), result.ReplyWatch.ID)
				return e == nil && w.State == "fired"
			})
			watch, err = db.GetReplyWatch(t.Context(), result.ReplyWatch.ID)
			if err != nil {
				t.Fatal(err)
			}
			if requeue {
				if watch.Outcome != "completed" || watch.TurnID == original || watch.ReplyText != "new turn answer" {
					t.Fatal(watch)
				}
			} else if watch.Outcome != "dropped" || watch.TurnID != "" {
				t.Fatal(watch)
			}
			once.Do(func() { close(release) })
			for range held.Events {
				continue
			}
		})
	}
}

// Invariant: consecutive queued messages from other sessions dispatch as one attributed turn; each entry is
// marked sent with that turn, keeps its own input event and origin, and its reply watch binds to that turn.
// Owner: session reply-watch suite (#760).
func TestReplyWatchBatchedDispatch(t *testing.T) {
	t.Parallel()
	t.Run("Should answer three queued agent messages in one turn and fire every watch from it", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, _ := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		other := createSession(t, h)
		for _, id := range []string{sender.ID, other.ID} {
			if err := h.manager.Stop(t.Context(), id); err != nil {
				t.Fatal(err)
			}
		}
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		dispatched := make(chan acp.PromptRequest, 4)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 2)
			go func() {
				defer close(events)
				if req.Message == "hold" {
					<-release
				} else {
					dispatched <- req
				}
				events <- acp.AgentEvent{Type: acp.EventTypeAgentMessage, TurnID: req.TurnID, Text: "batch answer"}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
			}()
			return events, nil
		}
		held, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{Message: "hold"})
		if err != nil {
			t.Fatal(err)
		}
		senders := map[string]string{"batch-1": sender.ID, "batch-2": sender.ID, "batch-3": other.ID}
		watches := map[string]string{}
		entries := map[string]string{}
		for _, id := range []string{"batch-1", "batch-2", "batch-3"} {
			result, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{
				Message: "question " + id, MessageID: id, IdempotencyKey: id + "-key",
				Mode: BusyInputModeQueue, NotifyOnComplete: true,
				Origin: &acp.PromptOriginMeta{
					Kind: "session", SessionID: senders[id], WorkspaceID: h.workspaceID, Hop: 1,
				},
			})
			if err != nil || result.ReplyWatch == nil || result.QueueEntryID == "" {
				t.Fatalf("SendPrompt(%s) = %#v, %v", id, result, err)
			}
			watches[id], entries[id] = result.ReplyWatch.ID, result.QueueEntryID
		}
		once.Do(func() { close(release) })
		for range held.Events {
			continue
		}
		var req acp.PromptRequest
		select {
		case req = <-dispatched:
		case <-time.After(5 * time.Second):
			t.Fatal("batched turn did not dispatch")
		}
		for id, from := range senders {
			if !strings.Contains(req.Message, "question "+id) || !strings.Contains(req.Message, from) {
				t.Fatalf("batched prompt lacks attributed %s from %s:\n%s", id, from, req.Message)
			}
		}
		waitForCondition(t, "every batched watch fired", func() bool {
			for _, watchID := range watches {
				w, err := db.GetReplyWatch(t.Context(), watchID)
				if err != nil || w.Outcome == "" {
					return false
				}
			}
			return true
		})
		for id, watchID := range watches {
			w, err := db.GetReplyWatch(t.Context(), watchID)
			if err != nil || w.Outcome != store.ReplyOutcomeCompleted || w.TurnID != req.TurnID ||
				w.ReplyText != "batch answer" {
				t.Fatalf("%s watch = %+v, %v; want completed from turn %s", id, w, err, req.TurnID)
			}
			entry, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entries[id])
			if err != nil || entry.Status != store.SessionInputQueueStatusSent || entry.TurnID != req.TurnID {
				t.Fatalf("%s entry = %q/%q, %v; want sent in %s", id, entry.Status, entry.TurnID, err, req.TurnID)
			}
		}
		select {
		case extra := <-dispatched:
			t.Fatalf("batched messages produced another turn: %#v", extra)
		default:
		}
		inputs := map[string]string{}
		stored, err := h.manager.Events(t.Context(), target.ID, store.EventQuery{
			TurnID: req.TurnID, Type: acp.EventTypeUserMessage, Limit: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range stored {
			decoded, err := transcript.UnmarshalAgentEvent(event.Content)
			if err != nil || decoded.PromptOrigin() == nil {
				t.Fatalf("batched input event = %#v, %v; want an attributed message", decoded, err)
			}
			inputs[decoded.MessageIDValue()] = decoded.PromptOrigin().SessionID
		}
		if len(inputs) != 3 || inputs["batch-1"] != sender.ID || inputs["batch-2"] != sender.ID ||
			inputs["batch-3"] != other.ID {
			t.Fatalf("turn inputs = %v, want one attributed input per batched message", inputs)
		}
	})
	t.Run("Should answer a queued agent message and a reply wake together in one turn", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		dispatched := make(chan acp.PromptRequest, 4)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			go func() {
				defer close(events)
				if req.Message == "hold" {
					<-release
				} else {
					dispatched <- req
				}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
			}()
			return events, nil
		}
		held, err := h.manager.SendPrompt(t.Context(), sender.ID, SendPromptOpts{Message: "hold"})
		if err != nil {
			t.Fatal(err)
		}
		message, err := h.manager.SendPrompt(t.Context(), sender.ID, SendPromptOpts{
			Message: "status from peer", MessageID: "peer-status", IdempotencyKey: "peer-status-key",
			Mode: BusyInputModeQueue,
			Origin: &acp.PromptOriginMeta{
				Kind: "session", SessionID: target.ID, WorkspaceID: h.workspaceID, Hop: 1,
			},
		})
		if err != nil || message.QueueEntryID == "" {
			t.Fatalf("queued agent message = %#v, %v", message, err)
		}
		if _, err := db.FireReplyWatch(
			t.Context(), w.ID, store.ReplyWatchFire{Outcome: store.ReplyOutcomeCompleted, Text: "per job"},
		); err != nil {
			t.Fatal(err)
		}
		if err := service.Deliver(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		pending, err := db.ListPendingSessionInputs(t.Context(), sender.ID)
		if err != nil || len(pending) != 2 {
			t.Fatalf("pending = %#v, %v; want the agent message and the reply wake", pending, err)
		}
		once.Do(func() { close(release) })
		for range held.Events {
			continue
		}
		var req acp.PromptRequest
		select {
		case req = <-dispatched:
		case <-time.After(5 * time.Second):
			t.Fatal("batched turn did not dispatch")
		}
		if !strings.Contains(req.Message, "status from peer") || !strings.Contains(req.Message, "per job") {
			t.Fatalf("batched prompt lacks the agent message or the reply wake:\n%s", req.Message)
		}
		for index := range pending {
			waitForCondition(t, "batched entry sent", func() bool {
				entry, err := db.GetSessionInputQueueEntry(t.Context(), sender.ID, pending[index].ID)
				return err == nil && entry.Status == store.SessionInputQueueStatusSent && entry.TurnID == req.TurnID
			})
		}
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchDelivered {
			t.Fatalf("watch = %+v, %v; want delivered by the batched turn", got, err)
		}
		select {
		case extra := <-dispatched:
			t.Fatalf("the reply wake produced another turn: %#v", extra)
		default:
		}
	})
	t.Run("Should bind a watch through the sent receipt when the input event is missing", func(t *testing.T) {
		t.Parallel()
		h, db, service, _, target, w := replyWatchFixture(t)
		entry := replyQueuedInput(t, h, db, target, w)
		if _, ok, err := db.ClaimNextSessionInput(t.Context(), target.ID, h.manager.now()); err != nil || !ok {
			t.Fatalf("claim = %v, %v", ok, err)
		}
		// The follower was delivered in the head's turn, but its own input event was never written.
		if err := db.MarkSessionInputSentInTurn(t.Context(), target.ID, entry.ID, "batch-turn", h.manager.now()); err != nil {
			t.Fatal(err)
		}
		if err := service.Reconcile(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.TurnID != "batch-turn" {
			t.Fatalf("watch = %+v, %v; want it bound to the consuming turn batch-turn", got, err)
		}
	})
	t.Run("Should dispatch a released follower on its own after the batched head fails to start", func(t *testing.T) {
		t.Parallel()
		h, db, _, sender, target, _ := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		dispatched := make(chan acp.PromptRequest, 4)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if strings.Contains(req.Message, "question head") {
				return nil, errors.New("provider start fault")
			}
			events := make(chan acp.AgentEvent, 1)
			go func() {
				defer close(events)
				if req.Message == "hold" {
					<-release
				} else {
					dispatched <- req
				}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
			}()
			return events, nil
		}
		held, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{Message: "hold"})
		if err != nil {
			t.Fatal(err)
		}
		entries := map[string]string{}
		for _, id := range []string{"head", "follower"} {
			result, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{
				Message: "question " + id, MessageID: id, IdempotencyKey: id + "-key", Mode: BusyInputModeQueue,
				Origin: &acp.PromptOriginMeta{
					Kind: "session", SessionID: sender.ID, WorkspaceID: h.workspaceID, Hop: 1,
				},
			})
			if err != nil || result.QueueEntryID == "" {
				t.Fatalf("SendPrompt(%s) = %#v, %v", id, result, err)
			}
			entries[id] = result.QueueEntryID
		}
		once.Do(func() { close(release) })
		for range held.Events {
			continue
		}
		var req acp.PromptRequest
		select {
		case req = <-dispatched:
		case <-time.After(5 * time.Second):
			t.Fatal("released follower did not dispatch on its own")
		}
		if !strings.Contains(req.Message, "question follower") || strings.Contains(req.Message, "question head") {
			t.Fatalf("follower prompt = %q, want the follower alone", req.Message)
		}
		waitForCondition(t, "follower sent in its own turn", func() bool {
			entry, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entries["follower"])
			return err == nil && entry.Status == store.SessionInputQueueStatusSent && entry.TurnID == req.TurnID
		})
		stored, err := h.manager.Events(t.Context(), target.ID, store.EventQuery{
			Type: acp.EventTypeUserMessage, Limit: 20,
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range stored {
			decoded, err := transcript.UnmarshalAgentEvent(event.Content)
			if err != nil {
				t.Fatal(err)
			}
			if decoded.MessageIDValue() == "follower" && event.TurnID != req.TurnID {
				t.Fatalf("follower input event in turn %q, want only its own turn %q", event.TurnID, req.TurnID)
			}
		}
	})
	t.Run("Should dispatch the head without a follower whose input hook fails", func(t *testing.T) {
		t.Parallel()
		h, db, _, sender, target, _ := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		h.manager.hooks = fullHookSet(&spyHookDispatcher{
			dispatchInputPreSubmitFn: func(
				_ context.Context, p hookspkg.InputPreSubmitPayload,
			) (hookspkg.InputPreSubmitPayload, error) {
				if strings.Contains(p.Message, "question bad") {
					return p, errors.New("follower hook fault")
				}
				return p, nil
			},
		})
		release := make(chan struct{})
		var once sync.Once
		t.Cleanup(func() { once.Do(func() { close(release) }) })
		dispatched := make(chan acp.PromptRequest, 4)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			go func() {
				defer close(events)
				if req.Message == "hold" {
					<-release
				} else {
					dispatched <- req
				}
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
			}()
			return events, nil
		}
		held, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{Message: "hold"})
		if err != nil {
			t.Fatal(err)
		}
		entries := map[string]string{}
		for _, id := range []string{"head", "bad", "tail"} {
			result, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{
				Message: "question " + id, MessageID: id, IdempotencyKey: id + "-key", Mode: BusyInputModeQueue,
				Origin: &acp.PromptOriginMeta{
					Kind: "session", SessionID: sender.ID, WorkspaceID: h.workspaceID, Hop: 1,
				},
			})
			if err != nil || result.QueueEntryID == "" {
				t.Fatalf("SendPrompt(%s) = %#v, %v", id, result, err)
			}
			entries[id] = result.QueueEntryID
		}
		once.Do(func() { close(release) })
		for range held.Events {
			continue
		}
		var req acp.PromptRequest
		select {
		case req = <-dispatched:
		case <-time.After(5 * time.Second):
			t.Fatal("head turn did not dispatch")
		}
		if !strings.Contains(req.Message, "question head") || !strings.Contains(req.Message, "question tail") ||
			strings.Contains(req.Message, "question bad") {
			t.Fatalf("batched prompt = %q, want head and tail without the failing follower", req.Message)
		}
		waitForCondition(t, "the failing follower settled as its own head", func() bool {
			entry, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entries["bad"])
			return err == nil && entry.Status == store.SessionInputQueueStatusFailed
		})
		for _, id := range []string{"head", "tail"} {
			entry, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entries[id])
			if err != nil || entry.Status != store.SessionInputQueueStatusSent || entry.TurnID != req.TurnID {
				t.Fatalf("%s entry = %q/%q, %v; want sent in %s", id, entry.Status, entry.TurnID, err, req.TurnID)
			}
		}
	})
}

// Invariant: reconciliation reads only the admitted input and its consuming turn (M-2).
func TestReplyWatchScopedEvidence(t *testing.T) {
	t.Run("Should ignore undecodable events outside the consuming turn", func(t *testing.T) {
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
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"})
		recorder := target.recorderHandle()
		if err := recorder.Record(
			t.Context(),
			store.SessionEvent{
				ID:        "broken-other",
				SessionID: target.ID,
				AgentName: target.Info().AgentName,
				TurnID:    "other",
				Type:      acp.EventTypeUserMessage,
				Content:   `{"type":"user_message","synthetic":"invalid"}`,
				Timestamp: h.manager.now(),
			},
		); err != nil {
			t.Fatal(err)
		}
		target.mu.Lock()
		target.recorder = replyScopedRecorder{EventRecorder: recorder}
		target.mu.Unlock()
		defer func() { target.mu.Lock(); target.recorder = recorder; target.mu.Unlock() }()
		if err := service.Reconcile(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != store.ReplyOutcomeCompleted {
			t.Fatalf("watch = %+v, %v", got, err)
		}
	})
}

type replyScopedRecorder struct{ EventRecorder }

func (r replyScopedRecorder) Query(ctx context.Context, q store.EventQuery) ([]store.SessionEvent, error) {
	if q.ID != "input-event" && q.TurnID != "watched" {
		return nil, fmt.Errorf("out-of-turn query: %+v", q)
	}
	return r.EventRecorder.Query(ctx, q)
}

func (r replyScopedRecorder) AppendEventIfAbsent(
	ctx context.Context,
	e store.SessionEvent,
) (store.SessionEvent, error) {
	return r.EventRecorder.(idempotentSessionEventAppender).AppendEventIfAbsent(ctx, e)
}

// Invariant: one broken watch cannot prevent recovery of a healthy row (M-3).
func TestReplyWatchRecoveryIsolation(t *testing.T) {
	t.Run("Should continue recovery after one watch fails", func(t *testing.T) {
		t.Parallel()
		h, db, _, sender, target, w := replyWatchFixture(t)
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
		service, err := NewReplyWatchService(replyRecoveryStore{ReplyWatchStore: db}, h.manager)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Recover(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "fired" {
			t.Fatalf("healthy watch = %+v, %v", got, err)
		}
	})
}

type replyRecoveryStore struct{ store.ReplyWatchStore }

func (s replyRecoveryStore) ListReplyWatches(
	ctx context.Context,
	f store.ReplyWatchFilter,
) ([]store.ReplyWatch, error) {
	rows, err := s.ReplyWatchStore.ListReplyWatches(ctx, f)
	return append(
		[]store.ReplyWatch{
			{ID: "broken", State: store.ReplyWatchArmed, CreatedAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		rows...), err
}

// Invariant: disappearance of the target settles unconsumed watches immediately (M-4).
func TestReplyWatchTargetGone(t *testing.T) {
	for _, archive := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should settle target disappearance with archive=%t", archive), func(t *testing.T) {
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
			if archive {
				if _, err := h.manager.Archive(t.Context(), h.workspaceID, target.ID); err != nil {
					t.Fatal(err)
				}
			} else if err := h.manager.Delete(t.Context(), target.ID); err != nil {
				t.Fatal(err)
			}
			got, err := db.GetReplyWatch(t.Context(), w.ID)
			if err != nil || got.State != "fired" || got.Outcome != "unknown" {
				t.Fatalf("target gone = %+v, %v", got, err)
			}
		})
	}
}

// Invariant: an earlier settle cannot resolve a later uncertain dispatch (m-2).
func TestReplyWatchDispatchAnchor(t *testing.T) {
	t.Run("Should wait for a settle after the dispatch commit", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "earlier"})
		committed := h.manager.now().Add(time.Hour)
		if err := db.CommitSessionPromptDispatch(
			t.Context(),
			h.workspaceID,
			target.ID,
			"reply-key",
			committed,
		); err != nil {
			t.Fatal(err)
		}
		if err := service.Reconcile(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "armed" {
			t.Fatalf("premature settle = %+v, %v", got, err)
		}
	})
}

// UT-043: a failed queue dispatch without an input receipt produces one dropped reply.
func TestReplyWatchDispatchFailed(t *testing.T) {
	t.Run("Should settle dispatch failed queue evidence as dropped", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		entry := replyQueuedInput(t, h, db, target, w)
		if _, claimed, err := db.ClaimNextSessionInput(
			t.Context(),
			target.ID,
			h.manager.now(),
		); err != nil ||
			!claimed {
			t.Fatal(claimed, err)
		}
		if err := db.MarkSessionInputFailed(
			t.Context(),
			target.ID,
			entry.ID,
			"dispatch_failed",
			h.manager.now(),
		); err != nil {
			t.Fatal(err)
		}
		failed, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		service.OnQueueEntryTerminal(t.Context(), &failed)
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != "dropped" || got.State != "fired" {
			t.Fatalf("dispatch failed = %+v, %v", got, err)
		}
	})
}

// m-1: an activation refusal abandons its watch before cancellation emits a terminal edge.
func TestReplyWatchActivationFailure(t *testing.T) {
	t.Run("Should return the send error without also delivering a dropped wake", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		entry := replyQueuedInput(t, h, db, target, w)
		origin, err := encodePromptOrigin(
			&acp.PromptOriginMeta{
				Kind:             "session",
				SessionID:        sender.ID,
				WorkspaceID:      h.workspaceID,
				Hop:              1,
				NotifyOnComplete: true,
				ReplyWatchID:     w.ID,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		entry.Origin = origin
		refused := errors.New("activation refused")
		err = h.manager.cleanupInterruptingInputActivationFailure(t.Context(), &entry, refused)
		if !errors.Is(err, refused) {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.State != "abandoned" || got.AbandonReason != "send_failed" || got.DeliveredInputID != "" {
			t.Fatalf("activation failure = %+v, %v", got, err)
		}
		canceled, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entry.ID)
		if err != nil || canceled.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("queue = %+v, %v", canceled, err)
		}
	})
	t.Run("Should still cancel the entry when its origin cannot be decoded", func(t *testing.T) {
		t.Parallel()
		h, db, service, _, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		entry := replyQueuedInput(t, h, db, target, w)
		entry.Origin = []byte(`{"kind":`)
		refused := errors.New("activation refused")
		err := h.manager.cleanupInterruptingInputActivationFailure(t.Context(), &entry, refused)
		if !errors.Is(err, refused) {
			t.Fatalf("cleanup error = %v, want the activation cause kept", err)
		}
		canceled, err := db.GetSessionInputQueueEntry(t.Context(), target.ID, entry.ID)
		if err != nil || canceled.Status != store.SessionInputQueueStatusCanceled {
			t.Fatalf("queue = %+v, %v; want the undeliverable entry canceled", canceled, err)
		}
	})
}

type replyRefreshFailureStore struct{ *globaldb.GlobalDB }

func (s replyRefreshFailureStore) GetReplyWatch(context.Context, string) (store.ReplyWatch, error) {
	return store.ReplyWatch{}, errors.New("reply watch read fault")
}

// Invariant: a delivered notified send returns its receipt even when the post-dispatch watch read fails.
// Owner: session reply-watch suite (review N-1).
func TestReplyWatchReceiptRefresh(t *testing.T) {
	t.Run("Should keep the admission receipt when the watch state read fails", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, _ := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		h.manager.inputQueueStore = replyRefreshFailureStore{db}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			return completedSyntheticPromptEvents(req.TurnID), nil
		}
		result, err := h.manager.SendPrompt(t.Context(), target.ID, SendPromptOpts{
			Message:          "question",
			MessageID:        "refresh-message",
			IdempotencyKey:   "refresh-key",
			NotifyOnComplete: true,
			Origin: &acp.PromptOriginMeta{
				Kind:        "session",
				SessionID:   sender.ID,
				WorkspaceID: h.workspaceID,
				Hop:         1,
			},
		})
		if err != nil {
			t.Fatalf("send = %v", err)
		}
		if result.ReplyWatch == nil || result.ReplyWatch.ID != store.ReplyWatchID(target.ID, "refresh-message") ||
			result.ReplyWatch.State != store.ReplyWatchArmed {
			t.Fatalf("receipt = %+v", result.ReplyWatch)
		}
		collectEvents(t, result.Events)
	})
}

// Invariant: an open reply watch is work for its sender only while the answer can still arrive.
// Owner: session reply-watch suite (orchestrator inactivity stop, 2026-10-10 report).
func TestReplyWatchAwaitingReplySignals(t *testing.T) {
	t.Run("Should count armed watches on live targets and fired watches until delivered", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, live := replyWatchFixture(t)
		now := h.manager.now()
		stopped := createSession(t, h)
		if err := h.manager.Stop(t.Context(), stopped.ID); err != nil {
			t.Fatal(err)
		}
		admission := store.SessionPromptAdmissionRequest{
			ID: "stopped-admission", WorkspaceID: h.workspaceID, SessionID: stopped.ID,
			MessageID: "stopped-message", IdempotencyKey: "stopped-key", Operation: store.SessionPromptOperationPrompt,
			FingerprintVersion: "test/v1", RequestFingerprint: "stopped-fingerprint",
			Mode: store.SessionInputQueueModeQueue, AuthoredText: "question", TurnID: "stopped-turn",
			EventID: "stopped-event", Now: now,
		}
		if _, _, err := db.ClaimSessionPromptAdmission(t.Context(), admission); err != nil {
			t.Fatal(err)
		}
		gone, err := db.InsertReplyWatchTx(t.Context(), db.DB(), store.ReplyWatchRegistration{
			WorkspaceID: h.workspaceID, SenderSessionID: sender.ID, TargetWorkspaceID: h.workspaceID,
			TargetSessionID: stopped.ID, MessageID: admission.MessageID, AdmissionID: admission.ID,
			Hop: 1, CreatedAt: now,
		})
		if err != nil {
			t.Fatal(err)
		}
		until := now.Add(time.Minute)
		refs := func() []string {
			signals, err := service.AwaitingReplySignals(t.Context(), sender.ID, until)
			if err != nil {
				t.Fatal(err)
			}
			out := make([]string, 0, len(signals))
			for _, signal := range signals {
				if signal.Kind != WorkSignalAwaitingReply || !signal.ValidUntil.Equal(until) || signal.Since.IsZero() {
					t.Fatalf("signal = %#v", signal)
				}
				out = append(out, signal.Ref)
			}
			return out
		}
		if got := refs(); !slices.Equal(got, []string{live.ID}) {
			t.Fatalf("armed refs = %v, want only the live target %s", got, live.ID)
		}
		if _, err := db.FireReplyWatch(
			t.Context(), gone.ID, store.ReplyWatchFire{Outcome: store.ReplyOutcomeUnknown, Text: "unknown"},
		); err != nil {
			t.Fatal(err)
		}
		if got := refs(); len(got) != 2 || !slices.Contains(got, gone.ID) {
			t.Fatalf("fired refs = %v, want the undelivered fired watch", got)
		}
		if other := mustAwaitingReplySignals(t, service, target.ID, until); len(other) != 0 {
			t.Fatalf("target signals = %#v, want none for a session that sent nothing", other)
		}
	})
	t.Run("Should keep an orchestrator out of the inactivity stop while its target works", func(t *testing.T) {
		t.Parallel()
		var nowMu sync.Mutex
		now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
		clock := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return now }
		cfg := testSupervisionConfig()
		cfg.QuietAfter, cfg.StopGrace = time.Second, time.Second
		h, _, service, sender, target, _ := replyWatchFixtureWith(
			t, []Option{WithNow(clock), WithSessionSupervision(cfg)},
		)
		h.manager.SetReplyWatchService(service)
		installAbsentWorkSources(h.manager)
		h.manager.SetWorkSignalSources(
			// The target is busy with the question; only the sender's wait is under test.
			WorkSignalSource{Kind: WorkSignalToolRunning, Source: SignalSourceFunc(
				func(_ context.Context, id string) ([]WorkSignal, error) {
					if id != target.ID {
						return nil, nil
					}
					at := clock()
					return []WorkSignal{
						{
							Kind:       WorkSignalToolRunning,
							Since:      at.Add(-time.Minute),
							ValidUntil: at.Add(time.Hour),
							Ref:        "tool",
						},
					}, nil
				},
			)},
			WorkSignalSource{Kind: WorkSignalAwaitingReply, Source: SignalSourceFunc(
				func(ctx context.Context, id string) ([]WorkSignal, error) {
					return service.AwaitingReplySignals(ctx, id, clock().Add(time.Minute))
				},
			)},
		)
		for range 5 {
			if err := h.manager.Supervise(t.Context(), clock()); err != nil {
				t.Fatal(err)
			}
			nowMu.Lock()
			now = now.Add(time.Minute)
			nowMu.Unlock()
		}
		if info := sender.Info(); info.State != StateActive || info.Supervision.QuietWarning != nil {
			t.Fatalf("waiting orchestrator = %#v, want active without a quiet warning", info)
		}
		if target.Info().State != StateActive {
			t.Fatal("busy target was stopped")
		}
	})
}

func mustAwaitingReplySignals(t *testing.T, service ReplyWatchService, id string, until time.Time) []WorkSignal {
	t.Helper()
	signals, err := service.AwaitingReplySignals(t.Context(), id, until)
	if err != nil {
		t.Fatal(err)
	}
	return signals
}

// Invariant: a reply wakes a sender the daemon stopped and delivers once; an operator stop is respected.
// Owner: session reply-watch suite (orchestrator inactivity stop, 2026-10-10 report).
func TestReplyWatchWakesStoppedSender(t *testing.T) {
	t.Run("Should classify which stops a reply may resume", func(t *testing.T) {
		t.Parallel()
		archived := time.Now()
		for _, tc := range []struct {
			name string
			info *Info
			want bool
		}{
			{"inactivity", &Info{State: StateStopped, StopReason: store.StopTimeout}, true},
			{"shutdown", &Info{State: StateStopped, StopReason: store.StopShutdown}, true},
			{"crash", &Info{State: StateStopped, StopReason: store.StopAgentCrashed}, true},
			{"operator", &Info{State: StateStopped, StopReason: store.StopUserCanceled}, false},
			{"completed", &Info{State: StateStopped, StopReason: store.StopCompleted}, false},
			{"archived", &Info{State: StateStopped, StopReason: store.StopTimeout, ArchivedAt: &archived}, false},
			{"active", &Info{State: StateActive}, false},
		} {
			t.Run("Should classify "+tc.name, func(t *testing.T) {
				t.Parallel()
				if got := replySenderWakeable(tc.info); got != tc.want {
					t.Fatalf("%s = %t, want %t", tc.name, got, tc.want)
				}
			})
		}
	})
	t.Run("Should leave a sender whose provider runtime died stopped instead of retrying resume", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, _, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.Stop(t.Context(), sender.ID); err != nil {
			t.Fatal(err)
		}
		meta := readMeta(t, sender.MetaPath())
		crashed := store.StopAgentCrashed
		meta.StopReason = &crashed
		meta.Failure = &store.SessionFailure{Kind: store.FailureProcess, Summary: "provider exited"}
		if err := store.WriteSessionMeta(sender.MetaPath(), &meta); err != nil {
			t.Fatal(err)
		}
		if h.manager.replySenderResumable(t.Context(), sender.ID) {
			t.Fatal("a dead-runtime sender was reported resumable")
		}
		if _, err := db.FireReplyWatch(
			t.Context(), w.ID, store.ReplyWatchFire{Outcome: store.ReplyOutcomeCompleted, Text: "per job"},
		); err != nil {
			t.Fatal(err)
		}
		if err := service.Deliver(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		if err := service.Close(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, live := h.manager.Get(sender.ID); live {
			t.Fatal("dead-runtime sender was resumed")
		}
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchFired {
			t.Fatalf("watch = %+v, %v; want fired until the operator recovers the sender", got, err)
		}
	})
	t.Run("Should not resume a sender once the service is closed for shutdown", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, _, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		if err := h.manager.RequestStopWithCause(t.Context(), sender.ID, CauseShutdown, ""); err != nil {
			t.Fatal(err)
		}
		if outcome, err := h.manager.AwaitStopped(t.Context(), sender.ID); err != nil || !outcome.Verified {
			t.Fatalf("sender stop = %#v, %v", outcome, err)
		}
		if err := h.manager.QuiesceReplyWatches(t.Context()); err != nil {
			t.Fatal(err)
		}
		if _, err := db.FireReplyWatch(
			t.Context(), w.ID, store.ReplyWatchFire{Outcome: store.ReplyOutcomeCompleted, Text: "per job"},
		); err != nil {
			t.Fatal(err)
		}
		if err := service.Deliver(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		if _, live := h.manager.Get(sender.ID); live {
			t.Fatal("a reply resumed a sender during shutdown")
		}
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchFired {
			t.Fatalf("watch = %+v, %v; want fired for the next boot", got, err)
		}
	})
	for _, cause := range []StopCause{CauseInactivity, CauseUserRequested} {
		t.Run(fmt.Sprintf("Should handle a sender stopped by %s", cause), func(t *testing.T) {
			t.Parallel()
			var nowMu sync.Mutex
			now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
			clock := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return now }
			cfg := testSupervisionConfig()
			cfg.QuietAfter, cfg.StopGrace = time.Second, time.Second
			h, db, service, sender, target, w := replyWatchFixtureWith(
				t, []Option{WithNow(clock), WithSessionSupervision(cfg)},
			)
			h.manager.SetReplyWatchService(service)
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				return completedSyntheticPromptEvents(req.TurnID), nil
			}
			if cause == CauseInactivity {
				// The target gives no evidence (absent sources), as when it stopped with the message queued:
				// the real supervisor stops the sender whose in-flight turn went quiet.
				installAbsentWorkSources(h.manager)
				markTurnInFlight(sender)
				for range 3 {
					if err := h.manager.Supervise(t.Context(), clock()); err != nil {
						t.Fatal(err)
					}
					nowMu.Lock()
					now = now.Add(time.Minute)
					nowMu.Unlock()
				}
				if err := h.manager.Stop(t.Context(), target.ID); err != nil {
					t.Fatal(err)
				}
			} else if err := h.manager.RequestStopWithCause(t.Context(), sender.ID, cause, ""); err != nil {
				t.Fatal(err)
			}
			if outcome, err := h.manager.AwaitStopped(t.Context(), sender.ID); err != nil || !outcome.Verified {
				t.Fatalf("sender stop = %#v, %v", outcome, err)
			}
			if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchArmed {
				t.Fatalf("watch before the reply = %+v, %v; want armed", got, err)
			}
			if _, err := db.FireReplyWatch(
				t.Context(), w.ID, store.ReplyWatchFire{Outcome: store.ReplyOutcomeCompleted, Text: "per job"},
			); err != nil {
				t.Fatal(err)
			}
			if err := service.Deliver(t.Context(), w.ID); err != nil {
				t.Fatal(err)
			}
			if !h.manager.replySenderResumable(t.Context(), sender.ID) {
				t.Fatal("a cleanly stopped sender was reported not resumable")
			}
			if cause == CauseUserRequested {
				if _, live := h.manager.Get(sender.ID); live {
					t.Fatal("an operator-stopped sender was resumed by a reply")
				}
				if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchFired {
					t.Fatalf("operator-stopped watch = %+v, %v; want fired until the operator resumes", got, err)
				}
				return
			}
			waitForCondition(t, "reply resumes and delivers once", func() bool {
				got, err := db.GetReplyWatch(t.Context(), w.ID)
				_, live := h.manager.Get(sender.ID)
				return err == nil && got.State == store.ReplyWatchDelivered && live
			})
			if err := service.Deliver(t.Context(), w.ID); err != nil {
				t.Fatal(err)
			}
			waitForCondition(t, "one reply wake", func() bool {
				rows, err := h.manager.Events(
					t.Context(), sender.ID, store.EventQuery{Type: acp.EventTypeSyntheticReentry, Limit: 100},
				)
				return err == nil && len(rows) == 1
			})
		})
	}
}

// Invariant: an armed watch past session.messaging.reply_watch_ttl (default 24h) fires once with the expired
// outcome and stops counting as awaited work; a younger watch, or any watch under an explicit 0, is untouched. Owner: session reply-watch suite (#760).
func TestReplyWatchTTLExpiry(t *testing.T) {
	t.Parallel()
	t.Run("Should expire a stale armed watch and wake the sender once", func(t *testing.T) {
		t.Parallel()
		var nowMu sync.Mutex
		now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
		clock := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return now }
		cfg := compozyconfig.SessionMessagingConfig{ReplyWatchTTL: time.Hour}
		h, db, service, sender, _, w := replyWatchFixtureWith(
			t, []Option{WithNow(clock), WithSessionMessagingConfig(cfg)},
		)
		h.manager.SetReplyWatchService(service)
		replies := service.(*replyWatchService)
		replies.expireStale(t.Context())
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchArmed {
			t.Fatalf("young watch = %+v, %v; want armed", got, err)
		}
		nowMu.Lock()
		now = now.Add(2 * time.Hour)
		nowMu.Unlock()
		if signals := mustAwaitingReplySignals(t, service, sender.ID, clock().Add(time.Minute)); len(signals) != 0 {
			t.Fatalf("expired watch signals = %#v, want none", signals)
		}
		replies.expireStale(t.Context())
		if err := h.manager.WaitForPromptDrains(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != store.ReplyOutcomeExpired || got.State != store.ReplyWatchDelivered ||
			!strings.Contains(got.ReplyText, "1h0m0s") {
			t.Fatalf("expired watch = %+v, %v; want a delivered expired outcome", got, err)
		}
		entry, err := db.GetSessionInputQueueEntry(t.Context(), sender.ID, got.DeliveredInputID)
		var meta acp.PromptSyntheticMeta
		if err != nil || entry.SyntheticPrompt == nil ||
			json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta) != nil || meta.Reason != store.ReplyOutcomeExpired {
			t.Fatalf("expiry wake = %+v, %v; want a session_reply wake with reason expired", entry, err)
		}
		replies.expireStale(t.Context())
		if again, err := db.GetReplyWatch(
			t.Context(),
			w.ID,
		); err != nil ||
			again.DeliveredInputID != got.DeliveredInputID {
			t.Fatalf("second sweep = %+v, %v; want the single delivered wake", again, err)
		}
	})
	t.Run("Should apply the documented messaging defaults without an explicit config", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		if cfg := h.manager.messagingConfig(); cfg != compozyconfig.DefaultSessionMessagingConfig() {
			t.Fatalf("messagingConfig() = %+v, want the documented defaults", cfg)
		}
	})
	t.Run("Should expire an armed watch after the default TTL", func(t *testing.T) {
		t.Parallel()
		var nowMu sync.Mutex
		now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
		clock := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return now }
		h, db, service, sender, _, w := replyWatchFixtureWith(
			t,
			[]Option{WithNow(clock), WithSessionMessagingConfig(compozyconfig.DefaultSessionMessagingConfig())},
		)
		h.manager.SetReplyWatchService(service)
		advance := func(d time.Duration) { nowMu.Lock(); now = now.Add(d); nowMu.Unlock() }
		advance(23 * time.Hour)
		service.(*replyWatchService).expireStale(t.Context())
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchArmed {
			t.Fatalf("watch after 23h = %+v, %v; want armed under the 24h default", got, err)
		}
		advance(2 * time.Hour)
		service.(*replyWatchService).expireStale(t.Context())
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.Outcome != store.ReplyOutcomeExpired {
			t.Fatalf("watch after 25h = %+v, %v; want expired under the 24h default", got, err)
		}
		if signals := mustAwaitingReplySignals(t, service, sender.ID, clock()); len(signals) != 0 {
			t.Fatalf("expired default-config watch still keeps its sender awake: %+v", signals)
		}
	})
	t.Run("Should never expire an armed watch when the TTL is explicitly zero", func(t *testing.T) {
		t.Parallel()
		var nowMu sync.Mutex
		now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
		clock := func() time.Time { nowMu.Lock(); defer nowMu.Unlock(); return now }
		cfg := compozyconfig.DefaultSessionMessagingConfig()
		cfg.ReplyWatchTTL = 0
		h, db, service, sender, _, w := replyWatchFixtureWith(
			t, []Option{WithNow(clock), WithSessionMessagingConfig(cfg)},
		)
		h.manager.SetReplyWatchService(service)
		nowMu.Lock()
		now = now.Add(60 * 24 * time.Hour)
		nowMu.Unlock()
		service.(*replyWatchService).expireStale(t.Context())
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchArmed {
			t.Fatalf("watch after 60 days = %+v, %v; want armed", got, err)
		}
		if signals := mustAwaitingReplySignals(t, service, sender.ID, clock()); len(signals) == 0 {
			t.Fatal("never-expire watch stopped keeping its sender awake")
		}
	})
}

// Invariant: an explicit in_reply_to answer from the watched target never releases the watch, so an early
// "ack, starting" cannot cancel the sender's completion wake; the watch still fires from the turn that
// consumed the message. Owner: session reply-watch suite (#760).
func TestReplyWatchExplicitReplyKeepsCompletionWake(t *testing.T) {
	t.Parallel()
	t.Run("Should keep the watch armed after an early in_reply_to and fire on completion", func(t *testing.T) {
		t.Parallel()
		h, db, service, sender, target, w := replyWatchFixture(t)
		h.manager.SetReplyWatchService(service)
		ack, err := h.manager.SendPrompt(t.Context(), sender.ID, SendPromptOpts{
			Message: "ack, starting", MessageID: "early-ack", IdempotencyKey: "early-ack-key",
			Origin: &acp.PromptOriginMeta{
				Kind: acp.PromptOriginKindSession, SessionID: target.ID, WorkspaceID: h.workspaceID, Hop: 1,
				InReplyTo: w.MessageID,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if ack.Events != nil {
			for range ack.Events {
				continue
			}
		}
		if got, err := db.GetReplyWatch(t.Context(), w.ID); err != nil || got.State != store.ReplyWatchArmed {
			t.Fatalf("watch after early ack = %+v, %v; want armed", got, err)
		}
		recordReplyEvent(
			t, h, target,
			acp.AgentEvent{Type: acp.EventTypeUserMessage, TurnID: "watched"}.WithMessageID(w.MessageID),
		)
		recordReplyEvent(t, h, target, acp.AgentEvent{
			Type: acp.EventTypeAgentMessage, TurnID: "watched", Text: "final result",
		})
		recordReplyEvent(t, h, target, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "watched"})
		if err := service.Reconcile(t.Context(), w.ID); err != nil {
			t.Fatal(err)
		}
		got, err := db.GetReplyWatch(t.Context(), w.ID)
		if err != nil || got.Outcome != store.ReplyOutcomeCompleted || got.ReplyText != "final result" ||
			got.State == store.ReplyWatchAbandoned {
			t.Fatalf("watch after completion = %+v, %v; want the completion wake", got, err)
		}
	})
}
