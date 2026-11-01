package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/mention"
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

// UT-003 UT-015 UT-018: bind origin once, preserve replay identity (including supersede and in_reply_to,
// #760), and retain the v4 boundary.
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
		replying := *origin
		replying.InReplyTo = "msg-question"
		for _, tc := range []struct {
			name      string
			origin    *acp.PromptOriginMeta
			notify    bool
			supersede bool
		}{
			{"Should reject a different sender", &acp.PromptOriginMeta{Kind: "session", SessionID: "sender-c", WorkspaceID: h.workspaceID, Hop: 1}, false, false},
			{"Should reject operator replay", nil, false, false},
			{"Should reject changed notification", origin, true, false},
			{"Should reject a retry that adds supersede", origin, false, true},
			{"Should reject a retry that adds in_reply_to", &replying, false, false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				request := req
				request.meta.Origin = tc.origin
				request.notifyOnComplete = tc.notify
				request.supersede = tc.supersede
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

// Invariant: a retry finds its admitted receipt and frozen origin by idempotency key and message identity.
// Owner: session admission identity suite (review: replay before new-send refusals).
func TestPromptAdmissionReplayOrigin(t *testing.T) {
	t.Parallel()
	h, _, _, sender, target, w := replyWatchFixture(t)
	for _, tc := range []struct {
		name, session, message, key string
		found                       bool
	}{
		{"Should find the admitted message", target.ID, w.MessageID, "reply-key", true},
		{"Should ignore a different message under the same key", target.ID, "other-message", "reply-key", false},
		{"Should report no receipt for an unknown key", target.ID, "reply-message", "missing-key", false},
		{"Should report no receipt on another session", sender.ID, "reply-message", "reply-key", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			found, origin, err := h.manager.PromptAdmissionReplayOrigin(
				t.Context(), h.workspaceID, tc.session, tc.message, tc.key,
			)
			if err != nil || found != tc.found || origin != nil {
				t.Fatalf("replay origin = %t %+v %v, want found=%t without origin", found, origin, err, tc.found)
			}
		})
	}
}

// Invariant: admitted mentions persist on the admission row and are restored on replay, while the
// fingerprint (and its version) stays independent of them because they derive from the authored text.
// Owner: session admission identity suite (IT-008).
func TestPromptAdmissionMentions(t *testing.T) {
	t.Parallel()

	admitted := []mention.Mention{{
		Kind: mention.KindFile, Token: "@a.go", Target: "a.go", Label: "a.go", Start: 5, End: 10,
	}}

	t.Run("Should keep the fingerprint independent of admitted mentions", func(t *testing.T) {
		t.Parallel()

		plain := promptRequest{messageID: "msg-mentions", authoredMessage: "read @a.go"}
		withMentions := plain
		withMentions.mentions = admitted
		first, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, plain)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(plain) error = %v", err)
		}
		second, err := promptAdmissionFingerprint(store.SessionPromptOperationPrompt, BusyInputModeQueue, withMentions)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(mentions) error = %v", err)
		}
		if first != second || sessionPromptFingerprintVersion != "session-prompt/v5" {
			t.Fatalf("fingerprint = %q vs %q (version %s), want mention-independent v5", first, second,
				sessionPromptFingerprintVersion)
		}
	})

	t.Run("Should persist admitted mentions on the admission request", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		req := promptRequest{target: "sess-mentions", messageID: "msg-1", authoredMessage: "read @a.go"}
		req.mentions = admitted
		admissionReq, err := h.manager.newPromptAdmissionRequest(
			"ws-mentions", req, store.SessionPromptOperationPrompt, BusyInputModeQueue,
		)
		if err != nil {
			t.Fatalf("newPromptAdmissionRequest() error = %v", err)
		}
		want := `[{"kind":"file","token":"@a.go","target":"a.go","label":"a.go","start":5,"end":10}]`
		if admissionReq.MentionsJSON != want {
			t.Fatalf("MentionsJSON = %s, want %s", admissionReq.MentionsJSON, want)
		}
		req.mentions = nil
		plain, err := h.manager.newPromptAdmissionRequest(
			"ws-mentions", req, store.SessionPromptOperationPrompt, BusyInputModeQueue,
		)
		if err != nil || plain.MentionsJSON != store.EmptyMentionsJSON {
			t.Fatalf("mention-free MentionsJSON = %q, %v; want []", plain.MentionsJSON, err)
		}
	})

	t.Run("Should restore admitted mentions when binding a replayed admission", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name, raw string
			want      []mention.Mention
		}{
			{
				name: "Should restore the admitted mentions",
				raw:  `[{"kind":"file","token":"@a.go","target":"a.go","label":"a.go","start":5,"end":10}]`,
				want: admitted,
			},
			{name: "Should restore no mentions for a row recorded before mentions existed", raw: store.EmptyMentionsJSON},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				// Fresh resolution against a changed filesystem must not leak into the replayed request.
				req := promptRequest{mentions: []mention.Mention{{Kind: mention.KindFile, Target: "fresh.go"}}}
				bound, err := bindPromptAdmissionRequest(req, store.SessionPromptAdmission{MentionsJSON: tc.raw})
				if err != nil {
					t.Fatalf("bindPromptAdmissionRequest() error = %v", err)
				}
				if !slices.Equal(bound.mentions, tc.want) {
					t.Fatalf("bound mentions = %#v, want %#v", bound.mentions, tc.want)
				}
			})
		}
	})

	t.Run("Should replay an admitted queued prompt with the mentions it was admitted with", func(t *testing.T) {
		t.Parallel()
		database := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(database))
		registerManagerInputQueueWorkspace(t, database, h)
		writeMentionFixture(t, h.workspace, "a.go")
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, database, h, sess)
		t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
		holdMentionActiveTurn(t, h)

		if _, err := h.manager.Prompt(t.Context(), sess.ID, "active"); err != nil {
			t.Fatal(err)
		}
		opts := SendPromptOpts{
			Message: "read @a.go", Mode: BusyInputModeQueue, MessageID: "msg-1", IdempotencyKey: "key-1",
		}
		first, err := h.manager.SendPrompt(t.Context(), sess.ID, opts)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(h.workspace, "a.go")); err != nil {
			t.Fatal(err)
		}
		replayed, err := h.manager.SendPrompt(t.Context(), sess.ID, opts)
		if err != nil || !replayed.Replayed || replayed.QueueEntryID != first.QueueEntryID {
			t.Fatalf("replay = %#v, %v; want the first admission", replayed, err)
		}
		admission, found, err := database.LookupSessionPromptAdmission(t.Context(), h.workspaceID, sess.ID, "key-1")
		if err != nil || !found ||
			!slices.Equal(mentionTargets(storedMentions(admission.MentionsJSON)), []string{"a.go"}) {
			t.Fatalf("admission mentions = %s, %v, %v; want a.go from admission", admission.MentionsJSON, found, err)
		}
		entry, err := database.GetSessionInputQueueEntry(t.Context(), sess.ID, first.QueueEntryID)
		if err != nil || !slices.Equal(mentionTargets(storedMentions(entry.MentionsJSON)), []string{"a.go"}) {
			t.Fatalf("queued entry mentions = %s, %v; want a.go from admission", entry.MentionsJSON, err)
		}
	})
}

func TestInstanceReportsReachProvider(t *testing.T) {
	for _, scenario := range []string{
		"success", "provider_command", "default_busy_mode", "dispatch_failure",
		"async_failure", "async_recovery", "startup_failure_new_key", "startup_failure_same_key",
	} {
		fail := scenario == "dispatch_failure" || strings.HasPrefix(scenario, "async_")
		name := "Should inject admitted reports once through SendPrompt"
		if fail {
			name = "Should mark uncertain driver delivery without resending the report"
		}
		t.Run(name+" "+scenario, func(t *testing.T) {
			t.Parallel()
			database := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(database), WithEventLedger(database))
			registerManagerInputQueueWorkspace(t, database, h)
			var sess *Session
			if strings.HasPrefix(scenario, "startup_failure") {
				created, err := h.manager.CreateAccepted(t.Context(), CreateAcceptedOpts{Session: CreateOpts{AgentName: "coder", Name: "session", Workspace: h.workspaceID}})
				if err != nil {
					t.Fatal(err)
				}
				var ok bool
				sess, ok = h.manager.Get(created.ID)
				if !ok {
					t.Fatal("logical session missing")
				}
			} else {
				sess = createSession(t, h)
			}
			registerManagerInputQueueSession(t, database, h, sess)
			t.Cleanup(func() { reportSessionStop(t, h, sess.ID) })
			instance, _, err := database.Instances().AdmitCreate(t.Context(), agentinstance.Instance{
				ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID, AgentName: sess.AgentName,
				PermissionCeiling: "{}", CreatedByKind: "human",
			}, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.Instances().BindMain(t.Context(), instance.ID, sess.ID); err != nil {
				t.Fatal(err)
			}
			reportCount := 50
			if fail {
				reportCount = 1
			}
			var report agentinstance.Report
			for i := range reportCount {
				report, err = database.InstanceReports().
					AppendReport(t.Context(), instance.ID, agentinstance.ReportInput{
						SourceKey: fmt.Sprintf("occ:driver-report-%d", i), Kind: "failure",
						Payload: []byte(`{"reason":"failed","detail":"</compozy-reports>"}`),
					})
				if err != nil {
					t.Fatal(err)
				}
				if err := database.InstanceReports().
					MarkReportPublished(t.Context(), report.ID, sess.ID, int64(i+1)); err != nil {
					t.Fatal(err)
				}
			}

			opts := SendPromptOpts{
				Caller:         PromptCaller{Kind: "human", ID: "operator", Source: "web"},
				Message:        "Read the update",
				MessageID:      "msg-report",
				IdempotencyKey: "key-report",
				Mode:           BusyInputModeQueue,
			}
			if scenario == "default_busy_mode" {
				opts.Mode = ""
			}
			if strings.HasPrefix(scenario, "startup_failure") {
				h.driver.startHook = func(acp.StartOpts, int) (*fakeProcess, error) { return nil, errors.New("runtime unavailable") }
				if _, err := h.manager.SendPrompt(t.Context(), sess.ID, opts); err == nil || !strings.Contains(err.Error(), "runtime unavailable") {
					t.Fatalf("expected runtime failure, got %v", err)
				}
				pending, err := database.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, State: "pending", Limit: 100})
				if err != nil || len(pending) != reportCount {
					t.Fatalf("pending after failed start=%d err=%v", len(pending), err)
				}
				h.driver.startHook = nil
				if scenario == "startup_failure_new_key" {
					opts.MessageID += "-retry"
					opts.IdempotencyKey += "-retry"
				}
			}
			if fail {
				h.driver.promptHook = func(_ *fakeProcess, _ acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					return nil, errors.New("uncertain driver delivery")
				}
			}
			if strings.HasPrefix(scenario, "async_") {
				h.manager.promptRecoveryDelays = []time.Duration{0}
				var attempts atomic.Int32
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					events := make(chan acp.AgentEvent, 1)
					event := acp.AgentEvent{Type: acp.EventTypeError, TurnID: req.TurnID, Error: "delivery lost"}
					if scenario == "async_recovery" {
						if attempts.Add(1) == 1 {
							event.Failure = &store.SessionFailure{Kind: store.FailureTransport, Summary: "delivery lost"}
						} else {
							if strings.Contains(req.Message, "<compozy-reports>") {
								t.Error("recovery resent uncertain reports")
							}
							event = acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
						}
					}
					events <- event
					close(events)
					return events, nil
				}
			}
			if scenario == "provider_command" {
				sess.replaceAdvertisedCommands([]store.SessionAdvertisedCommand{{Name: "compact"}}, h.manager.now())
				commandOpts := opts
				commandOpts.Message = "/compact keep decisions"
				commandOpts.MessageID = "msg-command"
				commandOpts.IdempotencyKey = "key-command"
				commandOpts.AllowCommands = true
				commandResult, err := h.manager.SendPrompt(t.Context(), sess.ID, commandOpts)
				if err != nil {
					t.Fatal(err)
				}
				drainPromptEvents(t, commandResult.Events)
				h.driver.mu.Lock()
				commandCalls := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
				h.driver.mu.Unlock()
				if len(commandCalls) != 1 || !commandCalls[0].AgentCommand ||
					commandCalls[0].Message != commandOpts.Message {
					t.Fatalf("provider command=%+v", commandCalls)
				}
				pending, err := database.InstanceReports().ListReports(t.Context(), agentinstance.ReportQuery{
					InstanceID: instance.ID, State: "pending", Limit: 100,
				})
				if err != nil || len(pending) != reportCount {
					t.Fatalf("pending after command=%d error=%v", len(pending), err)
				}
			}

			result, err := h.manager.SendPrompt(t.Context(), sess.ID, opts)
			if strings.HasPrefix(scenario, "async_") {
				if err != nil {
					t.Fatal(err)
				}
				events := collectEvents(t, result.Events)
				if scenario == "async_recovery" && countAgentEvents(events, acp.EventTypeRuntimeRecoverySucceeded) != 1 {
					t.Fatalf("recovery events=%+v", events)
				}
			}
			if fail {
				if err == nil && !strings.HasPrefix(scenario, "async_") {
					t.Fatal("expected driver error")
				}
				rows, readErr := database.InstanceReports().
					ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, State: "indeterminate", Limit: 20})
				if readErr != nil || len(rows) != 1 {
					t.Fatalf("indeterminate: %#v %v", rows, readErr)
				}
				// Simulate a crash after the durable note but before catalog acknowledgement.
				if _, err := database.DB().
					ExecContext(t.Context(), "UPDATE agent_instance_reports SET recovery_noted=0 WHERE id=?", report.ID); err != nil {
					t.Fatal(err)
				}
				original := h.manager.promptAdmissionStore
				h.manager.promptAdmissionStore = recoveryReportReadFailure{
					SessionPromptAdmissionStore: original,
					reader:                      database,
				}
				if err := h.manager.publishIndeterminateReportNotes(t.Context()); err == nil {
					t.Fatal("missing session recovery error was lost")
				}
				found := false
				for _, event := range readStoredEvents(t, sess) {
					found = found || event.ID == "report-indeterminate:"+report.ID+"-recovery"
				}
				if !found {
					t.Fatal("missing session prevented the later recovery note")
				}

				h.manager.promptAdmissionStore = recoveryReportAckFailure{
					SessionPromptAdmissionStore: original,
					indeterminateReportReader:   database,
				}
				if err := h.manager.publishIndeterminateReportNotes(t.Context()); err == nil {
					t.Fatal("missing acknowledgement error")
				}
				unnoted, err := database.IndeterminatePromptReports(t.Context(), "")
				if err != nil || len(unnoted) != 1 {
					t.Fatalf("failed acknowledgement lost retry: %#v, %v", unnoted, err)
				}
				h.manager.promptAdmissionStore = original
				if err := h.manager.publishIndeterminateReportNotes(t.Context()); err != nil {
					t.Fatal(err)
				}
				count := 0
				for _, event := range readStoredEvents(t, sess) {
					if event.ID == "report-indeterminate:"+report.ID {
						count++
					}
				}
				if count != 1 {
					t.Fatalf("indeterminate note count: %d", count)
				}

				remaining, err := database.IndeterminatePromptReports(t.Context(), "")
				if err != nil || len(remaining) != 0 {
					t.Fatalf("already noted reports returned for recovery: %#v, %v", remaining, err)
				}
				if err := h.manager.publishIndeterminateReportNotes(t.Context()); err != nil {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			drainPromptEvents(t, result.Events)
			h.driver.mu.Lock()
			calls := append([]acp.PromptRequest(nil), h.driver.promptCalls...)
			h.driver.mu.Unlock()
			if scenario == "provider_command" {
				calls = calls[1:]
			}
			if len(calls) != 1 || strings.Count(calls[0].Message, "<compozy-reports>") != 1 ||
				strings.Count(
					calls[0].Message,
					"</compozy-reports>",
				) != 1 || !strings.Contains(calls[0].Message, "Count: 20") || !strings.Contains(calls[0].Message, "GET /api/agent-instances/{instance_id}/reports") {
				t.Fatalf("provider requests: %#v", calls)
			}
			reports, err := database.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, State: "delivered", Limit: 20})
			if err != nil || len(reports) != 20 {
				t.Fatalf("delivered reports: %#v %v", reports, err)
			}
			pending, err := database.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, State: "pending", Limit: 100})
			if err != nil || len(pending) != 30 {
				t.Fatalf("pending reports: %d, %v", len(pending), err)
			}

			replay, err := h.manager.SendPrompt(t.Context(), sess.ID, opts)
			if err != nil || !replay.Replayed {
				t.Fatalf("replay: %#v %v", replay, err)
			}
			h.driver.mu.Lock()
			count := len(h.driver.promptCalls)
			h.driver.mu.Unlock()
			if scenario == "provider_command" {
				count--
			}
			if count != 1 {
				t.Fatalf("replayed report invoked driver %d times", count)
			}
		})
	}
}

type recoveryReportReadFailure struct {
	store.SessionPromptAdmissionStore
	reader indeterminateReportReader
}

func (r recoveryReportReadFailure) MarkIndeterminateReportNoted(ctx context.Context, id string) error {
	return r.reader.MarkIndeterminateReportNoted(ctx, id)
}

func (r recoveryReportReadFailure) IndeterminatePromptReports(
	ctx context.Context,
	after string,
) ([]store.IndeterminatePromptReport, error) {
	reports, err := r.reader.IndeterminatePromptReports(ctx, after)
	if err != nil {
		return nil, err
	}
	for i := range reports {
		reports[i].ID += "-recovery"
	}
	return append(
		[]store.IndeterminatePromptReport{{ID: "missing-report", SessionID: "missing-session"}},
		reports...), nil
}

func TestStoppedInstanceSupersededPrompt(t *testing.T) {
	for _, mode := range []BusyInputMode{"", BusyInputModeQueue} {
		t.Run(fmt.Sprintf("Should reject stopped predecessor before resume with mode %q", mode), func(t *testing.T) {
			t.Parallel()
			database := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(database), WithEventLedger(database))
			registerManagerInputQueueWorkspace(t, database, h)
			main := createSession(t, h)
			next := createSession(t, h)
			registerManagerInputQueueSession(t, database, h, main)
			registerManagerInputQueueSession(t, database, h, next)
			instance, err := database.Instances().
				Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID, AgentName: main.AgentName, PermissionCeiling: "{}", CreatedByKind: "human"}, main.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := database.DB().
				ExecContext(t.Context(), "UPDATE agent_instances SET main_session_id=? WHERE id=?", next.ID, instance.ID); err != nil {
				t.Fatal(err)
			}
			if err := h.manager.Stop(t.Context(), main.ID); err != nil {
				t.Fatal(err)
			}
			_, err = h.manager.SendPrompt(
				t.Context(),
				main.ID,
				SendPromptOpts{
					Message:        "Continue",
					MessageID:      "superseded",
					IdempotencyKey: "superseded",
					Mode:           mode,
					Caller:         PromptCaller{Kind: "human", ID: "operator", Source: "web"},
				},
			)
			if !errors.Is(err, store.ErrSessionSuperseded) || !strings.Contains(err.Error(), next.ID) {
				t.Fatalf("superseded = %v", err)
			}
			if _, live := h.manager.Get(main.ID); live {
				t.Fatal("superseded predecessor resumed")
			}
		})
	}
}

func TestInstanceContextExhaustionReport(t *testing.T) {
	for _, tc := range []struct {
		name, message string
		want          int
	}{
		{"Should report provider context exhaustion once per turn", "context_length_exceeded", 1},
		{"Should report a provider prompt length refusal", "prompt is too long", 1},
		{"Should ignore deadline failures", "context deadline exceeded", 0},
		{"Should ignore generic provider failures", "provider unavailable", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			database := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(database), WithEventLedger(database))
			registerManagerInputQueueWorkspace(t, database, h)
			main := createSession(t, h)
			registerManagerInputQueueSession(t, database, h, main)
			instance, err := database.Instances().
				Adopt(t.Context(), agentinstance.Instance{ProfileID: store.DefaultProfileID, WorkspaceID: h.workspaceID, AgentName: main.AgentName, PermissionCeiling: "{}", CreatedByKind: "human"}, main.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := h.manager.recordEvent(
					t.Context(),
					main,
					acp.AgentEvent{Type: acp.EventTypeError, TurnID: "exhausted-turn", Error: tc.message},
				); err != nil {
					t.Fatal(err)
				}
			}
			reports, err := database.InstanceReports().
				ListReports(t.Context(), agentinstance.ReportQuery{InstanceID: instance.ID, Limit: 10})
			if err != nil || len(reports) != tc.want {
				t.Fatalf("reports=%#v err=%v", reports, err)
			}
			if tc.want == 1 &&
				(reports[0].SourceKey != "turn:exhausted-turn" || reports[0].Kind != "failure" || !strings.Contains(string(reports[0].Payload), "context_exhausted")) {
				t.Fatalf("report=%#v", reports[0])
			}
		})
	}
}

type recoveryReportAckFailure struct {
	store.SessionPromptAdmissionStore
	indeterminateReportReader
}

func (r recoveryReportAckFailure) MarkIndeterminateReportNoted(context.Context, string) error {
	return errors.New("acknowledgement unavailable")
}
