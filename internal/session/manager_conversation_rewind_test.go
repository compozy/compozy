package session

import (
	"context"
	"strings"
	"testing"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

type rewindBaselineRecorder struct {
	*fakeEventRecorder
	state store.ConversationRewindState
}

func (r *rewindBaselineRecorder) ListTokenUsage(context.Context) ([]store.TokenUsage, error) {
	return nil, nil
}

func (r *rewindBaselineRecorder) ConversationRewindTarget(
	context.Context,
	string,
) (store.ConversationRewindTarget, error) {
	return store.ConversationRewindTarget{}, store.ErrConversationRewindTargetNotFound
}

func (r *rewindBaselineRecorder) ConversationRewindState(
	context.Context,
) (store.ConversationRewindState, bool, error) {
	return r.state, true, nil
}

func (r *rewindBaselineRecorder) ConversationRewindReceipt(
	context.Context,
	string,
	string,
) (store.ConversationRewindResult, bool, error) {
	return store.ConversationRewindResult{}, false, nil
}

func TestConversationRewindReplayBaselinePreservesEmptyState(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an empty replay baseline", func(t *testing.T) {
		t.Parallel()

		recorder := &rewindBaselineRecorder{
			fakeEventRecorder: &fakeEventRecorder{},
			state:             store.ConversationRewindState{MessagesJSON: "null"},
		}
		messages, found, err := conversationRewindReplayBaseline(t.Context(), recorder)
		if err != nil {
			t.Fatalf("conversationRewindReplayBaseline() error = %v", err)
		}
		if !found || len(messages) != 0 {
			t.Fatalf(
				"conversationRewindReplayBaseline() = (%#v, %t), want present empty baseline",
				messages,
				found,
			)
		}
	})
}

func TestConversationRewindRestartsSameSessionFromRetainedPrefix(t *testing.T) {
	t.Parallel()

	t.Run("Should cut before the selected user message and replay only the retained prefix", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created := createSession(t, h)
		for _, prompt := range []string{"keep this path", "take the wrong path"} {
			events, err := h.manager.Prompt(testutil.Context(t), created.ID, prompt)
			if err != nil {
				t.Fatalf("Prompt(%q) error = %v", prompt, err)
			}
			collectEvents(t, events)
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), created.ID, transcript.PageQuery{Limit: 20})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		var targetID string
		for _, entry := range page.Entries {
			if transcript.UIMessageText(entry.Message) == "take the wrong path" {
				targetID = entry.Message.ID
				break
			}
		}
		if targetID == "" {
			t.Fatal("selected durable user message was not materialized")
		}
		originalACP := created.Info().ACPSessionID
		opts := ConversationRewindOptions{
			MessageID: targetID, IdempotencyKey: "idem-manager-rewind",
			ExpectedEpoch:      created.Info().TranscriptEpoch,
			ExpectedGeneration: page.Generation, ExpectedMaxSequence: page.MaxSequence,
		}
		result, err := h.manager.RewindConversation(testutil.Context(t), created.ID, opts)
		if err != nil {
			t.Fatalf("RewindConversation() error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), result.Session.ID); err != nil {
				t.Errorf("cleanup Stop() error = %v", err)
			}
		})
		if result.Session.ID != created.ID || result.Session.Info().ACPSessionID == originalACP {
			t.Fatalf("rewound session = %#v, want same logical id and fresh ACP context", result.Session.Info())
		}
		if result.DraftText != "take the wrong path" || result.TranscriptEpoch <= opts.ExpectedEpoch {
			t.Fatalf("rewind result = %#v, want selected draft and advanced epoch", result)
		}
		if got := h.driver.startCalls[len(h.driver.startCalls)-1].ResumeSessionID; got != "" {
			t.Fatalf("rewind restart ResumeSessionID = %q, want fresh provider context", got)
		}
		marker := requireTranscriptMarker(t, h.manager, created.ID, transcript.MarkerSessionRecovered)
		if got := marker.Evidence["fallback_reason"]; got != "conversation_rewind" {
			t.Fatalf("rewind recovery fallback_reason = %v, want conversation_rewind", got)
		}
		rewoundPage, err := h.manager.TranscriptPage(
			testutil.Context(t), created.ID, transcript.PageQuery{Limit: 20},
		)
		if err != nil {
			t.Fatalf("TranscriptPage(after rewind) error = %v", err)
		}
		if result.Generation != rewoundPage.Generation || result.MaxSequence != rewoundPage.MaxSequence {
			t.Fatalf("rewind fences = (%d,%d), want active transcript fences (%d,%d)",
				result.Generation, result.MaxSequence, rewoundPage.Generation, rewoundPage.MaxSequence)
		}
		activeAudit, err := h.manager.Events(testutil.Context(t), created.ID, store.EventQuery{
			Type: eventspkg.SessionConversationRewound, Archive: store.EventArchiveUnarchived,
		})
		if err != nil {
			t.Fatalf("Events(active rewind audit) error = %v", err)
		}
		if len(activeAudit) != 0 {
			t.Fatalf(
				"active rewind audit events = %#v, want audit excluded from active transcript history",
				activeAudit,
			)
		}

		continued, err := h.manager.Prompt(testutil.Context(t), created.ID, "take a better path")
		if err != nil {
			t.Fatalf("Prompt(after rewind) error = %v", err)
		}
		collectEvents(t, continued)
		replay := h.driver.promptCalls[len(h.driver.promptCalls)-1].Message
		if !strings.Contains(replay, "keep this path") || strings.Contains(replay, "take the wrong path") {
			t.Fatalf("rewind replay = %q, want retained prefix without discarded prompt", replay)
		}

		replayed, err := h.manager.RewindConversation(testutil.Context(t), created.ID, opts)
		if err != nil {
			t.Fatalf("RewindConversation(retry) error = %v", err)
		}
		if !replayed.Replayed || replayed.Session != result.Session {
			t.Fatalf("RewindConversation(retry) = %#v, want idempotent active-session replay", replayed)
		}
		archivedAudit, err := h.manager.Events(testutil.Context(t), created.ID, store.EventQuery{
			Type: eventspkg.SessionConversationRewound, Archive: store.EventArchiveArchived,
		})
		if err != nil {
			t.Fatalf("Events(archived rewind audit) error = %v", err)
		}
		if got, want := len(archivedAudit), 1; got != want {
			t.Fatalf("len(archived rewind audit) = %d, want idempotent count %d", got, want)
		}
	})

	t.Run("Should rewind and restart an already stopped user session", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		created := createSession(t, h)
		events, err := h.manager.Prompt(testutil.Context(t), created.ID, "replace this request")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		if err := h.manager.Stop(testutil.Context(t), created.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		page, err := h.manager.TranscriptPage(testutil.Context(t), created.ID, transcript.PageQuery{Limit: 20})
		if err != nil {
			t.Fatalf("TranscriptPage() error = %v", err)
		}
		var targetID string
		for _, entry := range page.Entries {
			if transcript.UIMessageText(entry.Message) == "replace this request" {
				targetID = entry.Message.ID
				break
			}
		}
		if targetID == "" {
			t.Fatal("stopped session user message was not materialized")
		}

		result, err := h.manager.RewindConversation(testutil.Context(t), created.ID, ConversationRewindOptions{
			MessageID: targetID, IdempotencyKey: "idem-stopped-rewind",
			ExpectedEpoch: created.Info().TranscriptEpoch, ExpectedGeneration: page.Generation,
			ExpectedMaxSequence: page.MaxSequence,
		})
		if err != nil {
			t.Fatalf("RewindConversation(stopped) error = %v", err)
		}
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), result.Session.ID); err != nil {
				t.Errorf("cleanup Stop() error = %v", err)
			}
		})
		if result.Session.Info().State != StateActive || result.DraftText != "replace this request" {
			t.Fatalf("RewindConversation(stopped) = %#v", result)
		}
	})
}
