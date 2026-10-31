package session

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: Move to Later stops and parks a whole hierarchy tree, keeps it out of
// the default catalog, queues agent messages without resuming it, refuses
// non-operator resumes, and brings the root back into the normal flow at its
// return time while children stay parked; an operator prompt resumes it.
// Owner: session lifecycle (Later).
func TestManagerLaterLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should park a session tree and return it at its return time", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		var clockMu sync.Mutex
		now := time.Now().UTC()
		clock := func() time.Time { clockMu.Lock(); defer clockMu.Unlock(); return now }
		h, _ := newMentionHarness(t, WithNow(clock))
		releaser := &recordingLaterReleaser{}
		h.manager.SetLaterProcessReleaser(releaser)
		parent := createSession(t, h)
		child, err := h.manager.Spawn(ctx, SpawnOpts{ParentSessionID: parent.ID, AgentName: "coder", TTL: time.Hour})
		if err != nil {
			t.Fatalf("Spawn() error = %v", err)
		}

		returnAt := now.Add(time.Hour)
		parked, err := h.manager.MoveToLater(ctx, h.workspaceID, parent.ID, LaterRequest{
			Reason: " revisit after the billing spec ", ReturnAt: &returnAt,
		})
		if err != nil {
			t.Fatalf("MoveToLater() error = %v", err)
		}
		if parked.Later == nil || parked.Later.Reason != "revisit after the billing spec" ||
			parked.Later.ReturnAt == nil || !parked.Later.ReturnAt.Equal(returnAt) {
			t.Fatalf("MoveToLater() later = %#v, want trimmed reason and return time", parked.Later)
		}
		for _, id := range []string{parent.ID, child.ID} {
			info, statusErr := h.manager.Status(ctx, id)
			if statusErr != nil {
				t.Fatalf("Status(%s) error = %v", id, statusErr)
			}
			if info.State != StateStopped || info.Later == nil {
				t.Fatalf("Status(%s) = state %q later %#v, want stopped and parked", id, info.State, info.Later)
			}
		}
		if got := releaser.terminatedIDs(); !slices.Equal(got, []string{parent.ID, child.ID}) {
			t.Fatalf("terminated processes for %v, want parent then child", got)
		}
		assertLaterCatalog(t, h, LaterExclude, nil)
		assertLaterCatalog(t, h, LaterOnly, []string{parent.ID, child.ID})

		result, err := h.manager.SendPrompt(ctx, parent.ID, SendPromptOpts{
			Message: "status?", MessageID: "msg-later", IdempotencyKey: "idem-later",
			Origin: &acp.PromptOriginMeta{Kind: "session", SessionID: child.ID, WorkspaceID: h.workspaceID, Hop: 1},
		})
		if err != nil {
			t.Fatalf("SendPrompt(agent) error = %v", err)
		}
		if got := result.Outcome().Disposition; got != DispositionLater {
			t.Fatalf("SendPrompt(agent) disposition = %q, want %q", got, DispositionLater)
		}
		retry, err := h.manager.SendPrompt(ctx, parent.ID, SendPromptOpts{
			Message: "status?", MessageID: "msg-later", IdempotencyKey: "idem-later",
			Origin: &acp.PromptOriginMeta{Kind: "session", SessionID: child.ID, WorkspaceID: h.workspaceID, Hop: 1},
		})
		if err != nil || !retry.Replayed || retry.Outcome().Disposition != DispositionLater {
			t.Fatalf("SendPrompt(agent retry) = %+v, %v, want the replayed Later result", retry, err)
		}
		info, err := h.manager.Status(ctx, parent.ID)
		if err != nil || info.State != StateStopped || info.Later == nil || info.Later.PendingMessages != 1 {
			t.Fatalf("Status(after agent message) = %#v, %v, want parked with one pending message", info, err)
		}
		if _, err := h.manager.Resume(ctx, parent.ID); !errors.Is(err, ErrSessionInLater) {
			t.Fatalf("Resume(parked) error = %v, want ErrSessionInLater", err)
		}
		orphan := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, orphan.ID) })
		if _, err := h.manager.AdoptSession(ctx, h.workspaceID, orphan.ID, parent.ID); !errors.Is(
			err, ErrSessionReparentRefused,
		) {
			t.Fatalf("AdoptSession(parked parent) error = %v, want a reparent refusal", err)
		}

		clockMu.Lock()
		now = returnAt
		clockMu.Unlock()
		returned, err := h.manager.ReturnDueLaterSessions(ctx)
		if err != nil || returned != 1 {
			t.Fatalf("ReturnDueLaterSessions() = %d, %v, want the parent", returned, err)
		}
		info, err = h.manager.Status(ctx, parent.ID)
		if err != nil || info.Later != nil || info.LaterReturnedAt == nil {
			t.Fatalf("Status(returned) = %#v, %v, want back from Later", info, err)
		}
		if got := BadgeForInfo(info); got != BadgeNeedsAttention {
			t.Fatalf("BadgeForInfo(returned) = %q, want %q so it lands in Needs you", got, BadgeNeedsAttention)
		}
		childInfo, err := h.manager.Status(ctx, child.ID)
		if err != nil || childInfo.Later == nil {
			t.Fatalf("Status(child after return) = %#v, %v, want child still parked", childInfo, err)
		}
		if _, err := h.manager.ResumeFromLater(ctx, h.workspaceID, parent.ID, true); err != nil {
			t.Fatalf("ResumeFromLater(with children) error = %v", err)
		}
		if childInfo, err = h.manager.Status(ctx, child.ID); err != nil || childInfo.Later != nil {
			t.Fatalf("Status(child after Back from Later) = %#v, %v, want it out of Later", childInfo, err)
		}
	})

	t.Run("Should resume a parked session for an operator prompt", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		if _, err := h.manager.MoveToLater(ctx, h.workspaceID, sess.ID, LaterRequest{}); err != nil {
			t.Fatalf("MoveToLater() error = %v", err)
		}
		result, err := h.manager.SendPrompt(ctx, sess.ID, SendPromptOpts{
			Message: "back to work", Caller: PromptCaller{Kind: "human", ID: "operator"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(operator) error = %v", err)
		}
		collectEvents(t, result.Events)
		info, err := h.manager.Status(ctx, sess.ID)
		if err != nil || info.State != StateActive || info.Later != nil {
			t.Fatalf("Status(after operator prompt) = %#v, %v, want active and out of Later", info, err)
		}
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
	})

	t.Run("Should keep a multi-line Markdown reason and reject one past the limit", func(t *testing.T) {
		t.Parallel()
		h, _ := newMentionHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		_, err := h.manager.MoveToLater(t.Context(), h.workspaceID, sess.ID, LaterRequest{
			Reason: strings.Repeat("a", MaxLaterReasonLength+1),
		})
		if !errors.Is(err, ErrSessionLaterInvalid) {
			t.Fatalf("MoveToLater(too long) error = %v, want ErrSessionLaterInvalid", err)
		}
		parked, err := h.manager.MoveToLater(t.Context(), h.workspaceID, sess.ID, LaterRequest{
			Reason: " Waiting on **review**\r\n\r\n- rerun the gate\r\n- ping the owner ",
		})
		if err != nil {
			t.Fatalf("MoveToLater(multi-line) error = %v", err)
		}
		want := "Waiting on **review**\n\n- rerun the gate\n- ping the owner"
		if parked.Later == nil || parked.Later.Reason != want {
			t.Fatalf("MoveToLater(multi-line) later = %#v, want reason %q", parked.Later, want)
		}
	})
}

// Invariant: surfaces that cannot render Markdown (the back-from-Later
// notification) read a Later reason as one plain line, and plain reasons stay
// unchanged. Owner: session lifecycle (Later).
func TestLaterReasonPlainText(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		reason string
		want   string
	}{
		{
			name:   "Should keep a plain reason",
			reason: "revisit after the billing spec",
			want:   "revisit after the billing spec",
		},
		{
			name:   "Should drop inline Markdown delimiters and keep link text",
			reason: "Waiting on **review** of *the* `gate` in ~~old~~ [the spec](https://compozy.com) 3 \\* 4",
			want:   "Waiting on review of the gate in old the spec 3 * 4",
		},
		{
			name:   "Should join list items and paragraphs into one line",
			reason: "Blocked:\n\n- rerun the gate\n2. ping the owner",
			want:   "Blocked: · rerun the gate · ping the owner",
		},
		{name: "Should keep snake_case words", reason: "fix later_return_at", want: "fix later_return_at"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := LaterReasonPlainText(tc.reason); got != tc.want {
				t.Fatalf("LaterReasonPlainText(%q) = %q, want %q", tc.reason, got, tc.want)
			}
		})
	}
}

func assertLaterCatalog(t *testing.T, h *harness, filter store.SessionLaterFilter, want []string) {
	t.Helper()
	page, err := h.manager.ListPage(t.Context(), ListQuery{
		ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
		WorkspaceID: h.workspaceID,
		Later:       filter,
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListPage(later=%s) error = %v", filter, err)
	}
	got := make([]string, 0, len(page.Sessions))
	for _, info := range page.Sessions {
		got = append(got, info.ID)
	}
	slices.Sort(got)
	expected := slices.Clone(want)
	slices.Sort(expected)
	if !slices.Equal(got, expected) || page.Total != len(expected) {
		t.Fatalf("ListPage(later=%s) = %v total=%d, want %v", filter, got, page.Total, expected)
	}
}

// recordingLaterReleaser stands in for daemon process attribution at the
// session layer's I/O boundary.
type recordingLaterReleaser struct {
	mu         sync.Mutex
	terminated []string
}

func (r *recordingLaterReleaser) CaptureSessionProcesses(context.Context) error { return nil }

func (r *recordingLaterReleaser) TerminateSessionProcesses(_ context.Context, info *Info) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.terminated = append(r.terminated, info.ID)
	return nil
}

func (r *recordingLaterReleaser) terminatedIDs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.terminated)
}
