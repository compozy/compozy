package session

import (
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"

	"github.com/compozy/compozy/internal/store"
)

func TestPromptAdmissionFingerprintAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should change the v4 fingerprint when the attachment set changes", func(t *testing.T) {
		t.Parallel()

		base := promptRequest{messageID: "msg-fingerprint", authoredMessage: "look at this"}
		one := base
		one.attachments = []AttachmentMeta{{
			ID:     "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SHA256: "digest-a",
		}}
		two := base
		two.attachments = []AttachmentMeta{
			{ID: "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SHA256: "digest-a"},
			{ID: "att_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SHA256: "digest-b"},
		}
		first, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, one)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(one) error = %v", err)
		}
		second, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, two)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(two) error = %v", err)
		}
		if first == second {
			t.Fatal("fingerprint stayed stable after the attachment set changed")
		}
	})

	t.Run("Should stay stable across attachment digest order", func(t *testing.T) {
		t.Parallel()

		base := promptRequest{messageID: "msg-order", authoredMessage: "look at these"}
		forward := base
		forward.attachments = []AttachmentMeta{
			{SHA256: "digest-b"},
			{SHA256: "digest-a"},
		}
		reverse := base
		reverse.attachments = []AttachmentMeta{
			{SHA256: "digest-a"},
			{SHA256: "digest-b"},
		}
		first, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, forward)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(forward) error = %v", err)
		}
		second, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, reverse)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(reverse) error = %v", err)
		}
		if first != second {
			t.Fatalf("fingerprint = %q vs %q, want order-stable v4 attachment identity", first, second)
		}
	})

	t.Run("Should stay stable when canonical metadata enriches an attachment ID", func(t *testing.T) {
		t.Parallel()

		attachmentID := "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		raw := promptRequest{
			messageID: "msg-canonical",
			attachments: []AttachmentMeta{{
				ID: attachmentID,
			}},
		}
		canonical := raw
		canonical.attachments = []AttachmentMeta{{
			ID: attachmentID, SHA256: strings.Repeat("a", 64), Name: "notes.txt", MIMEType: "text/plain",
		}}
		before, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, raw)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(raw) error = %v", err)
		}
		after, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, canonical)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(canonical) error = %v", err)
		}
		if before != after {
			t.Fatalf("fingerprint = %q before canonicalization, %q after", before, after)
		}
	})
}

func TestPromptOriginAdmissionIdentity(t *testing.T) {
	t.Parallel()
	t.Run("Should freeze sender origin and scope replay to actor and notify", func(t *testing.T) {
		t.Parallel()
		database := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(database))
		registerManagerInputQueueWorkspace(t, database, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, database, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		origin := &acp.PromptOriginMeta{
			Kind:        "session",
			SessionID:   "sender-a",
			WorkspaceID: h.workspaceID,
			TitleAtSend: "Original",
			Hop:         2,
		}
		req := promptRequest{
			target:          sess.ID,
			messageID:       "msg-origin",
			idempotencyKey:  "idem-origin",
			authoredMessage: "Q?",
			turnID:          "turn-origin",
			meta:            acp.PromptMeta{Origin: origin},
		}
		request, err := h.manager.newPromptAdmissionRequest(
			h.workspaceID,
			req,
			store.SessionPromptOperationPrompt,
			BusyInputModeQueue,
		)
		if err != nil {
			t.Fatal(err)
		}
		first, created, err := database.ClaimSessionPromptAdmission(t.Context(), request)
		if err != nil || !created || first.FingerprintVersion != "session-prompt/v5" {
			t.Fatal(first, created, err)
		}
		changed := *origin
		changed.TitleAtSend = "Renamed"
		changed.Hop = 7
		req.meta.Origin = &changed
		retry, err := h.manager.newPromptAdmissionRequest(
			h.workspaceID,
			req,
			store.SessionPromptOperationPrompt,
			BusyInputModeQueue,
		)
		if err != nil {
			t.Fatal(err)
		}
		replayed, _, err := database.ClaimSessionPromptAdmission(t.Context(), retry)
		if err != nil {
			t.Fatal(err)
		}
		bound, err := bindPromptAdmissionRequest(req, replayed)
		if err != nil || bound.meta.Origin == nil || *bound.meta.Origin != *origin {
			t.Fatal(bound.meta.Origin, err)
		}
		queued, err := h.manager.newQueuedInputPromptRequest(
			sess.ID,
			queuedInput{origin: first.Origin, text: "Q?", turnID: "turn-q"},
		)
		if err != nil || *queued.meta.Origin != *origin {
			t.Fatal(queued.meta.Origin, err)
		}
		for _, tc := range []struct {
			name   string
			origin *acp.PromptOriginMeta
			notify bool
		}{
			{"Should reject a different sender", &acp.PromptOriginMeta{Kind: "session", SessionID: "sender-c", WorkspaceID: h.workspaceID, Hop: 1}, false},
			{"Should reject operator replay", nil, false},
			{"Should reject changed notification", origin, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				request := req
				request.meta.Origin = tc.origin
				request.notifyOnComplete = tc.notify
				collision, err := h.manager.newPromptAdmissionRequest(
					h.workspaceID,
					request,
					store.SessionPromptOperationPrompt,
					BusyInputModeQueue,
				)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err := database.ClaimSessionPromptAdmission(
					t.Context(),
					collision,
				); !errors.Is(
					err,
					store.ErrSessionPromptIdempotencyConflict,
				) {
					t.Fatal(err)
				}
			})
		}
	})
	t.Run("Should replay a v4 admission without inventing attribution", func(t *testing.T) {
		t.Parallel()
		database := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(database))
		registerManagerInputQueueWorkspace(t, database, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, database, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		req := promptRequest{
			target:          sess.ID,
			messageID:       "legacy-msg",
			idempotencyKey:  "legacy-key",
			authoredMessage: "legacy text",
			turnID:          "legacy-turn",
		}
		request, err := h.manager.newPromptAdmissionRequest(
			h.workspaceID,
			req,
			store.SessionPromptOperationPrompt,
			BusyInputModeQueue,
		)
		if err != nil {
			t.Fatal(err)
		}
		legacy := request
		legacy.FingerprintVersion = "session-prompt/v4"
		legacy.RequestFingerprint = request.LegacyRequestFingerprint
		if _, _, err := database.ClaimSessionPromptAdmission(t.Context(), legacy); err != nil {
			t.Fatal(err)
		}
		if err := database.CommitSessionPromptDispatch(
			t.Context(),
			h.workspaceID,
			sess.ID,
			legacy.IdempotencyKey,
			h.manager.now(),
		); err != nil {
			t.Fatal(err)
		}
		if _, err := database.CompleteSessionPromptAdmission(
			t.Context(),
			h.workspaceID,
			sess.ID,
			legacy.IdempotencyKey,
			store.SessionPromptAdmissionResult{Status: "accepted", NewTurnID: "legacy-turn"},
			h.manager.now(),
		); err != nil {
			t.Fatal(err)
		}
		req.meta.Origin = &acp.PromptOriginMeta{
			Kind:             "session",
			SessionID:        "sender",
			WorkspaceID:      h.workspaceID,
			Hop:              1,
			NotifyOnComplete: true,
		}
		req.notifyOnComplete = true
		request, err = h.manager.newPromptAdmissionRequest(
			h.workspaceID,
			req,
			store.SessionPromptOperationPrompt,
			BusyInputModeQueue,
		)
		if err != nil {
			t.Fatal(err)
		}
		replayed, found, err := database.ReplaySessionPromptAdmission(t.Context(), request)
		if err != nil || !found || len(replayed.Origin) != 0 || replayed.Result.NewTurnID != "legacy-turn" {
			t.Fatal(replayed, err)
		}
		result, err := sendPromptResultFromAdmission(replayed)
		if err != nil || result.ReplyWatch != nil {
			t.Fatal(result, err)
		}
		watches, err := database.ListReplyWatches(t.Context(), store.ReplyWatchFilter{})
		if err != nil || len(watches) != 0 {
			t.Fatalf("legacy watches = %+v, %v", watches, err)
		}
		request.LegacyRequestFingerprint = "wrong"
		if _, _, err := database.ReplaySessionPromptAdmission(
			t.Context(),
			request,
		); !errors.Is(
			err,
			store.ErrSessionPromptIdempotencyConflict,
		) {
			t.Fatal(err)
		}
	})
}
