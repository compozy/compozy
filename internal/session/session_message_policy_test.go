package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type sessionMessageFixture struct {
	h                *harness
	db               *globaldb.GlobalDB
	sender, peer, cc *Session
}

func newSessionMessageFixture(t *testing.T, opts ...Option) sessionMessageFixture {
	t.Helper()
	db := openManagerInputQueueStore(t)
	h := newHarness(t, append([]Option{WithSessionInputQueueStore(db), WithSessionCatalog(db)}, opts...)...)
	registerManagerInputQueueWorkspace(t, db, h)
	cleanupTestManager(t, h.manager)
	f := sessionMessageFixture{
		h: h, db: db, sender: createSession(t, h), peer: createSession(t, h), cc: createSession(t, h),
	}
	t.Cleanup(func() {
		for _, sess := range []*Session{f.sender, f.peer, f.cc} {
			if _, live := h.manager.Get(sess.ID); live {
				if err := h.manager.Stop(context.Background(), sess.ID); err != nil {
					t.Error(err)
				}
			}
		}
	})
	return f
}

// driveTurn starts the sender's active turn from meta, as a dispatched message or wake would.
func (f sessionMessageFixture) driveTurn(t *testing.T, meta acp.PromptMeta) {
	t.Helper()
	_, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	turnID := "message-turn"
	if _, err := f.sender.beginExclusivePromptSetupForRequest(
		promptRequest{turnID: turnID, turnSource: TurnSourceUser, meta: meta},
		cancel,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		f.sender.finishPromptSetup()
		clearPromptState(f.sender, turnID)
	})
}

// runTurn runs fn inside one sender turn started from meta and settles the turn afterwards, so a test can
// chain several turns of the same session.
func (f sessionMessageFixture) runTurn(t *testing.T, turnID string, meta acp.PromptMeta, fn func()) {
	t.Helper()
	_, cancel := context.WithCancel(t.Context())
	defer cancel()
	if _, err := f.sender.beginExclusivePromptSetupForRequest(
		promptRequest{turnID: turnID, turnSource: TurnSourceUser, meta: meta},
		cancel,
	); err != nil {
		t.Fatal(err)
	}
	defer func() {
		f.sender.finishPromptSetup()
		clearPromptState(f.sender, turnID)
	}()
	fn()
}

// admit records one durable admission into target, optionally stamped with a session origin.
func (f sessionMessageFixture) admit(
	t *testing.T,
	target *Session,
	messageID string,
	origin *acp.PromptOriginMeta,
	at time.Time,
) {
	t.Helper()
	var raw json.RawMessage
	if origin != nil {
		encoded, err := json.Marshal(origin)
		if err != nil {
			t.Fatal(err)
		}
		raw = encoded
	}
	if _, _, err := f.db.ClaimSessionPromptAdmission(t.Context(), store.SessionPromptAdmissionRequest{
		ID: "pad-" + messageID, WorkspaceID: f.h.workspaceID, SessionID: target.ID, MessageID: messageID,
		IdempotencyKey: "key-" + messageID, Operation: store.SessionPromptOperationPrompt,
		FingerprintVersion: "test/v1", RequestFingerprint: "fp-" + messageID,
		Mode: store.SessionInputQueueModeQueue, AuthoredText: "message", Origin: raw,
		TurnID: "turn-" + messageID, EventID: "event-" + messageID, Now: at,
	}); err != nil {
		t.Fatal(err)
	}
}

func (f sessionMessageFixture) origin(sender *Session, hop int) *acp.PromptOriginMeta {
	return &acp.PromptOriginMeta{
		Kind: acp.PromptOriginKindSession, SessionID: sender.ID, WorkspaceID: f.h.workspaceID, Hop: hop,
	}
}

func (f sessionMessageFixture) send(
	t *testing.T,
	target *Session,
	inReplyTo string,
) (*acp.PromptOriginMeta, error) {
	t.Helper()
	return f.h.manager.SessionMessageOrigin(t.Context(), MessageOriginRequest{
		SenderSessionID: f.sender.ID, AgentName: "coder", TargetSessionID: target.ID, InReplyTo: inReplyTo,
	})
}

// Invariant: hop measures forwarding depth, so answering the session that drives the current turn (by its
// message, its reply wake, or an explicit in_reply_to) keeps the hop, a reply wake restores the sender's own
// hop so sequential orchestration never accumulates, while forwarding to a third session adds one and stays
// bounded by MaxSessionMessageHops. Owner: session message policy (issue #760 item 4).
func TestSessionMessageOriginHop(t *testing.T) {
	t.Parallel()
	t.Run("Should keep the hop when answering the session that started the turn", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		f.driveTurn(t, acp.PromptMeta{Origin: f.origin(f.peer, 3)})
		answer, err := f.send(t, f.peer, "")
		if err != nil || answer.Hop != 3 || answer.SessionID != f.sender.ID || answer.TitleAtSend == "" {
			t.Fatalf("answer origin = %+v, %v; want hop 3 stamped from the sender", answer, err)
		}
		forward, err := f.send(t, f.cc, "")
		if err != nil || forward.Hop != 4 {
			t.Fatalf("forward origin = %+v, %v; want hop 4", forward, err)
		}
	})
	t.Run("Should keep the hop when a reply wake drives the turn", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		f.driveTurn(t, acp.PromptMeta{Synthetic: &acp.PromptSyntheticMeta{
			Kind: acp.PromptSyntheticKindSessionReply, Reason: store.ReplyOutcomeCompleted,
			ChildSessionID: f.peer.ID, Hop: 2,
		}})
		next, err := f.send(t, f.peer, "")
		if err != nil || next.Hop != 2 {
			t.Fatalf("follow-up origin = %+v, %v; want the wake's hop 2", next, err)
		}
	})
	t.Run("Should never accumulate hops across sequential wake and dispatch stages", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		var wake *acp.PromptSyntheticMeta
		for stage := range 20 {
			child := []*Session{f.peer, f.cc}[stage%2]
			meta := acp.PromptMeta{Synthetic: wake}
			var dispatched *acp.PromptOriginMeta
			f.runTurn(t, fmt.Sprintf("stage-%d", stage), meta, func() {
				origin, err := f.send(t, child, "")
				if err != nil {
					t.Fatalf("stage %d dispatch error = %v, want no hop limit", stage, err)
				}
				dispatched = origin
			})
			if dispatched.Hop != 1 {
				t.Fatalf("stage %d dispatch hop = %d, want 1", stage, dispatched.Hop)
			}
			wake = &acp.PromptSyntheticMeta{
				Kind: acp.PromptSyntheticKindSessionReply, Reason: store.ReplyOutcomeCompleted,
				ChildSessionID: child.ID, Hop: replyWakeHop(store.ReplyWatch{Hop: dispatched.Hop}),
			}
		}
	})
	t.Run("Should still bound a genuine forwarding chain", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		var inbound *acp.PromptOriginMeta
		for hop := 1; ; hop++ {
			var (
				forwarded *acp.PromptOriginMeta
				err       error
			)
			f.runTurn(t, fmt.Sprintf("forward-%d", hop), acp.PromptMeta{Origin: inbound}, func() {
				forwarded, err = f.send(t, f.cc, "")
			})
			if hop > acp.MaxSessionMessageHops {
				if !errors.Is(err, ErrSessionMessageHopLimit) {
					t.Fatalf("forward %d error = %v, want ErrSessionMessageHopLimit", hop, err)
				}
				return
			}
			if err != nil || forwarded.Hop != hop {
				t.Fatalf("forward %d origin = %+v, %v; want hop %d", hop, forwarded, err, hop)
			}
			// The next turn is driven by another session forwarding the same chain onward.
			inbound = f.origin(f.peer, hop)
		}
	})
	t.Run("Should keep the hop for an explicit in_reply_to answer", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		f.admit(t, f.sender, "plan-from-peer", f.origin(f.peer, 1), f.h.manager.now())
		f.admit(t, f.sender, "note-from-cc", f.origin(f.cc, 1), f.h.manager.now())
		f.driveTurn(t, acp.PromptMeta{Origin: f.origin(f.cc, 5)})
		reply, err := f.send(t, f.peer, "plan-from-peer")
		if err != nil || reply.Hop != 5 || reply.InReplyTo != "plan-from-peer" {
			t.Fatalf("reply origin = %+v, %v; want hop 5 with in_reply_to", reply, err)
		}
		mismatched, err := f.send(t, f.peer, "note-from-cc")
		if err != nil || mismatched.Hop != 6 || mismatched.InReplyTo != "note-from-cc" {
			t.Fatalf("mismatched reply origin = %+v, %v; want a forwarded hop 6", mismatched, err)
		}
	})
	t.Run("Should bound forwarding depth but still allow the answer at the limit", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		f.driveTurn(t, acp.PromptMeta{Origin: f.origin(f.cc, acp.MaxSessionMessageHops)})
		if answer, err := f.send(t, f.cc, ""); err != nil || answer.Hop != acp.MaxSessionMessageHops {
			t.Fatalf("answer at limit = %+v, %v", answer, err)
		}
		if _, err := f.send(t, f.peer, ""); !errors.Is(err, ErrSessionMessageHopLimit) {
			t.Fatalf("forward past limit error = %v, want ErrSessionMessageHopLimit", err)
		}
	})
	t.Run("Should refuse self targeting and malformed in_reply_to", func(t *testing.T) {
		t.Parallel()
		f := newSessionMessageFixture(t)
		if _, err := f.send(t, f.sender, ""); !errors.Is(err, ErrSessionMessageSelfTarget) {
			t.Fatalf("self target error = %v", err)
		}
		if _, err := f.send(t, f.peer, "has space"); !errors.Is(err, acp.ErrInvalidPromptMetadata) {
			t.Fatalf("malformed in_reply_to error = %v", err)
		}
	})
}

// Invariant: each sender→target pair has a durable admission budget per window; an exhausted pair refuses
// with the typed rate error while other pairs and expired admissions do not count.
// Owner: session message policy (issue #760 item 4).
func TestSessionMessageOriginRateBudget(t *testing.T) {
	t.Parallel()
	t.Run("Should refuse an exhausted sender to target budget and keep other pairs open", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		cfg := compozyconfig.SessionMessagingConfig{RateLimitMessages: 2, RateLimitWindow: 10 * time.Minute}
		f := newSessionMessageFixture(t, WithNow(func() time.Time { return now }), WithSessionMessagingConfig(cfg))
		f.admit(t, f.peer, "old", f.origin(f.sender, 1), now.Add(-time.Hour))
		f.admit(t, f.peer, "operator", nil, now.Add(-time.Minute))
		f.admit(t, f.peer, "other-sender", f.origin(f.cc, 1), now.Add(-time.Minute))
		f.admit(t, f.peer, "first", f.origin(f.sender, 1), now.Add(-4*time.Minute))
		if _, err := f.send(t, f.peer, ""); err != nil {
			t.Fatalf("send within budget error = %v", err)
		}
		f.admit(t, f.peer, "second", f.origin(f.sender, 1), now.Add(-time.Minute))
		_, err := f.send(t, f.peer, "")
		limit, ok := errors.AsType[*MessageRateLimitError](err)
		if !ok || !errors.Is(err, ErrSessionMessageRateLimited) {
			t.Fatalf("exhausted budget error = %v, want MessageRateLimitError", err)
		}
		if limit.Limit != 2 || limit.Window != 10*time.Minute || limit.RetryAfter != 6*time.Minute ||
			limit.TargetSessionID != f.peer.ID {
			t.Fatalf("rate limit = %+v, want 2 per 10m retrying when the oldest admission leaves the window", limit)
		}
		if _, err := f.send(t, f.cc, ""); err != nil {
			t.Fatalf("other target error = %v, want an independent budget", err)
		}
	})
}

// Invariant: reply authority belongs only to the recipient and original sender of a durable admission.
// Owning layer: session message policy; canonical suite: session_message_policy_test.go.
func TestSessionMessageReplyAuthority(t *testing.T) {
	t.Parallel()
	f := newSessionMessageFixture(t)
	f.admit(t, f.sender, "peer-question", f.origin(f.peer, 1), f.h.manager.now())
	for _, tc := range []struct {
		name, sender, target, message string
		allowed                       bool
	}{
		{"Should authorize the admitted peer", f.sender.ID, f.peer.ID, "peer-question", true},
		{"Should reject another target", f.sender.ID, f.cc.ID, "peer-question", false},
		{"Should reject another recipient", f.cc.ID, f.peer.ID, "peer-question", false},
		{"Should reject an unknown message", f.sender.ID, f.peer.ID, "unknown", false},
		{"Should reject an unsolicited message", f.sender.ID, f.peer.ID, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			allowed, err := f.h.manager.CanReplyToSession(t.Context(), tc.sender, tc.target, tc.message)
			if err != nil || allowed != tc.allowed {
				t.Fatalf("reply authority = %v, %v; want %v", allowed, err, tc.allowed)
			}
		})
	}
}
