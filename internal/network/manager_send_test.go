package network

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestManagerSendCommitsBeforeDispatch(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 14, 4, 0, 0, 0, time.UTC)
	acceptance := &managerAcceptanceStub{}
	notifier := &managerWakeNotifierStub{}
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerClock(func() time.Time { return now }),
		WithManagerAuditWriter(managerAuditWriterStub{}),
		WithManagerAcceptanceStore(acceptance),
		WithManagerWakeNotifier(notifier),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
	joinManagerSendParticipant(t, manager, "sess-recipient-b", "reviewer.sess-bbb")
	joinManagerSendParticipant(t, manager, "sess-recipient-a", "reviewer.sess-aaa")

	acceptanceErr := errors.New("accept transaction failed")
	acceptance.handle = func(req store.AcceptNetworkMessageRequest, call int) (store.AcceptNetworkMessageResult, error) {
		switch call {
		case 1:
			notify := make([]store.CommittedNetworkNotification, 0, len(req.Admissions))
			for index, admission := range req.Admissions {
				notify = append(notify, store.CommittedNetworkNotification{
					WorkspaceID:        req.Message.WorkspaceID,
					RecipientSessionID: admission.RecipientSessionID,
					TaskRunID:          admission.TaskRunID,
					AcceptanceSeq:      int64(index + 1),
				})
			}
			return store.AcceptNetworkMessageResult{
				AcceptanceSeq: 1,
				Dispositions:  slices.Clone(req.Dispositions),
				Notify:        notify,
			}, nil
		case 2:
			return store.AcceptNetworkMessageResult{
				AcceptanceSeq: 1,
				Duplicate:     true,
				Dispositions:  slices.Clone(req.Dispositions),
			}, nil
		default:
			return store.AcceptNetworkMessageResult{}, acceptanceErr
		}
	}

	request := managerThreadSendRequest("msg-manager-commit-1")
	messageID, err := manager.Send(testutil.Context(t), request)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if messageID != "msg-manager-commit-1" {
		t.Fatalf("Send() message id = %q, want committed id", messageID)
	}
	first := acceptance.request(t, 0)
	wantRecipients := []string{"sess-recipient-a", "sess-recipient-b"}
	if got := managerDispositionRecipients(first.Dispositions); !slices.Equal(got, wantRecipients) {
		t.Fatalf("accepted disposition recipients = %#v, want pre-message snapshot %#v", got, wantRecipients)
	}
	if got := managerAdmissionRecipients(first.Admissions); !slices.Equal(got, wantRecipients) {
		t.Fatalf("accepted admission recipients = %#v, want %#v", got, wantRecipients)
	}
	if got := notifier.recipients(); !slices.Equal(got, wantRecipients) {
		t.Fatalf("wake notification order = %#v, want committed order %#v", got, wantRecipients)
	}

	if _, err := manager.Send(testutil.Context(t), request); err != nil {
		t.Fatalf("Send(duplicate) error = %v", err)
	}
	if got := notifier.recipients(); !slices.Equal(got, wantRecipients) {
		t.Fatalf("wake notifications after duplicate = %#v, want no second dispatch", got)
	}

	failed := managerThreadSendRequest("msg-manager-commit-failed")
	if _, err := manager.Send(testutil.Context(t), failed); !errors.Is(err, acceptanceErr) {
		t.Fatalf("Send(accept failure) error = %v, want %v", err, acceptanceErr)
	}
	if got := acceptance.request(t, 2).Message.MessageID; got != "msg-manager-commit-failed" {
		t.Fatalf("failed acceptance message id = %q, want msg-manager-commit-failed", got)
	}
	if got := notifier.recipients(); !slices.Equal(got, wantRecipients) {
		t.Fatalf("wake notifications after failed accept = %#v, want committed notifications only", got)
	}
	stats := manager.stats.snapshot()
	if stats.MessagesSent != 1 || stats.MessagesReceived != 2 || stats.MessagesDelivered != 2 {
		t.Fatalf(
			"post-commit message stats = sent:%d received:%d delivered:%d, want 1/2/2",
			stats.MessagesSent,
			stats.MessagesReceived,
			stats.MessagesDelivered,
		)
	}
}

func TestManagerSendDetachesFromRequestCancellation(t *testing.T) {
	t.Parallel()

	acceptance := &managerAcceptanceStub{}
	manager := newManagerSendTestManager(t, acceptance)
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")

	requestCtx, cancel := context.WithCancel(testutil.Context(t))
	cancel()
	request := managerThreadSendRequest("msg-manager-detached")
	if _, err := manager.Send(requestCtx, request); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got := acceptance.callCount(); got != 1 {
		t.Fatalf("AcceptNetworkMessage calls = %d, want 1 after request cancellation", got)
	}
}

func TestManagerSendObservesCommittedMessagesOnce(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	acceptance := &managerAcceptanceStub{}
	hooks := &managerHookDispatcherSpy{acceptance: acceptance}
	auditor := &managerAuditWriterSpy{acceptance: acceptance}
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerClock(func() time.Time { return now }),
		WithManagerAuditWriter(auditor),
		WithManagerHookDispatcher(hooks),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
	joinManagerSendParticipant(t, manager, "sess-recipient", "reviewer.sess-target")
	hooks.reset()

	acceptance.handle = func(
		req store.AcceptNetworkMessageRequest,
		call int,
	) (store.AcceptNetworkMessageResult, error) {
		if call == 2 {
			return store.AcceptNetworkMessageResult{
				Duplicate: true, Dispositions: slices.Clone(req.Dispositions),
			}, nil
		}
		return store.AcceptNetworkMessageResult{
			Dispositions: slices.Clone(req.Dispositions),
			Conversation: store.NetworkConversationWriteResult{
				MessageID: req.Message.MessageID, ConversationOpened: true,
				WorkOpened: true, WorkTransitioned: true,
				WorkState: store.NetworkWorkStateCompleted, LastActivityAt: now,
			},
		}, nil
	}
	request := managerThreadSendRequest("msg-manager-observers")
	workID := "work-manager-observers"
	traceID := "trace-manager-observers"
	causationID := "cause-manager-observers"
	request.WorkID = &workID
	request.TraceID = &traceID
	request.CausationID = &causationID

	if _, err := manager.Send(testutil.Context(t), request); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if _, err := manager.Send(testutil.Context(t), request); err != nil {
		t.Fatalf("Send(duplicate) error = %v", err)
	}

	wantHookEvents := []hookspkg.HookEvent{
		hookspkg.HookNetworkThreadOpened,
		hookspkg.HookNetworkMessagePersisted,
		hookspkg.HookNetworkWorkOpened,
		hookspkg.HookNetworkWorkTransitioned,
		hookspkg.HookNetworkWorkClosed,
	}
	if got := hooks.events(); !slices.Equal(got, wantHookEvents) {
		t.Fatalf("hook events = %#v, want %#v", got, wantHookEvents)
	}
	if got := hooks.commitMissCount(); got != 0 {
		t.Fatalf("hooks observed an in-flight acceptance %d times", got)
	}
	for _, call := range hooks.callsSnapshot() {
		encoded, marshalErr := json.Marshal(call.payload)
		if marshalErr != nil {
			t.Fatalf("json.Marshal(hook payload) error = %v", marshalErr)
		}
		if strings.Contains(string(encoded), "body") || strings.Contains(string(encoded), "text") {
			t.Fatalf("hook payload leaked message content: %s", encoded)
		}
		if call.payload.MessageID != "msg-manager-observers" ||
			call.payload.WorkID != workID ||
			call.payload.TraceID != traceID ||
			call.payload.CausationID != causationID {
			t.Fatalf("hook payload lost stable correlation: %#v", call.payload)
		}
	}
	wantAudit := []managerAuditCall{
		{direction: AuditDirectionSent, sessionID: "sess-sender"},
		{direction: AuditDirectionReceived, sessionID: "sess-recipient"},
		{direction: AuditDirectionDelivered, sessionID: "sess-recipient"},
	}
	if got := auditor.callsSnapshot(); !slices.Equal(got, wantAudit) {
		t.Fatalf("audit calls = %#v, want %#v", got, wantAudit)
	}
	if got := auditor.commitMissCount(); got != 0 {
		t.Fatalf("audit observed an in-flight acceptance %d times", got)
	}
}

func TestManagerSendObserverFailuresAreFailOpen(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	acceptance := &managerAcceptanceStub{}
	hookErr := errors.New("hook observer offline")
	auditErr := errors.New("audit observer offline")
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(slog.New(slog.NewTextHandler(&logs, nil))),
		WithManagerAuditWriter(&managerAuditWriterSpy{err: auditErr}),
		WithManagerHookDispatcher(&managerHookDispatcherSpy{err: hookErr}),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
	joinManagerSendParticipant(t, manager, "sess-recipient", "reviewer.sess-target")
	logs.Reset()
	acceptance.handle = func(
		req store.AcceptNetworkMessageRequest,
		_ int,
	) (store.AcceptNetworkMessageResult, error) {
		return store.AcceptNetworkMessageResult{
			Dispositions: slices.Clone(req.Dispositions),
			Conversation: store.NetworkConversationWriteResult{
				MessageID: req.Message.MessageID, ConversationOpened: true,
			},
		}, nil
	}

	if _, err := manager.Send(testutil.Context(t), managerThreadSendRequest("msg-manager-fail-open")); err != nil {
		t.Fatalf("Send() error = %v, want committed success despite observer failures", err)
	}
	for _, want := range []string{
		"network.hook.dispatch_failed",
		"hook observer offline",
		"network.audit.record_sent_failed",
		"network.audit.record_received_failed",
		"network.audit.record_delivered_failed",
		"audit observer offline",
	} {
		if !strings.Contains(logs.String(), want) {
			t.Fatalf("observer logs missing %q: %s", want, logs.String())
		}
	}
	stats := manager.stats.snapshot()
	if stats.MessagesSent != 1 || stats.MessagesReceived != 1 || stats.MessagesDelivered != 1 {
		t.Fatalf("observer failure stats = %#v, want committed sent/received/delivered", stats)
	}
}

func TestManagerSendAuditsAcceptanceRejection(t *testing.T) {
	t.Parallel()

	acceptanceErr := errors.New("atomic acceptance rejected")
	acceptance := &managerAcceptanceStub{
		handle: func(
			store.AcceptNetworkMessageRequest,
			int,
		) (store.AcceptNetworkMessageResult, error) {
			return store.AcceptNetworkMessageResult{}, acceptanceErr
		},
	}
	auditor := &managerAuditWriterSpy{}
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerAuditWriter(auditor),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")

	if _, err := manager.Send(
		testutil.Context(t),
		managerThreadSendRequest("msg-manager-rejected"),
	); !errors.Is(err, acceptanceErr) {
		t.Fatalf("Send() error = %v, want %v", err, acceptanceErr)
	}
	want := []managerAuditCall{{
		direction: AuditDirectionRejected,
		sessionID: "sess-sender",
		reason:    "acceptance_failed",
	}}
	if got := auditor.callsSnapshot(); !slices.Equal(got, want) {
		t.Fatalf("rejection audit calls = %#v, want %#v", got, want)
	}
}

func TestDeliverySubscriptionModePrefersExactThreadThenChannel(t *testing.T) {
	t.Parallel()

	threadID := "thread-release"
	delivery := Delivery{
		SessionID: "session-reviewer",
		PeerID:    "reviewer.peer",
		Envelope: Envelope{
			WorkspaceID: testWorkspaceID,
			Channel:     "builders",
			ThreadID:    &threadID,
		},
	}
	tests := []struct {
		name        string
		threadMode  string
		channelMode string
		wantMode    string
		wantQueries int
	}{
		{
			name:       "Should let an exact thread preference override the channel fallback",
			threadMode: store.NetworkSubscriptionModeFull, channelMode: store.NetworkSubscriptionModeMute,
			wantMode: store.NetworkSubscriptionModeFull, wantQueries: 1,
		},
		{
			name:        "Should use the channel fallback when no exact thread preference exists",
			channelMode: store.NetworkSubscriptionModeMute,
			wantMode:    store.NetworkSubscriptionModeMute, wantQueries: 2,
		},
		{
			name:     "Should default to full delivery when neither preference exists",
			wantMode: store.NetworkSubscriptionModeFull, wantQueries: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			preferences := &managerSubscriptionStoreStub{
				threadMode: test.threadMode, channelMode: test.channelMode,
			}
			mode, err := deliverySubscriptionMode(testutil.Context(t), preferences, delivery)
			if err != nil {
				t.Fatalf("deliverySubscriptionMode() error = %v", err)
			}
			if mode != test.wantMode {
				t.Fatalf("deliverySubscriptionMode() = %q, want %q", mode, test.wantMode)
			}
			if len(preferences.queries) != test.wantQueries {
				t.Fatalf("subscription queries = %#v, want %d exact reads", preferences.queries, test.wantQueries)
			}
			for index, query := range preferences.queries {
				if !query.ExactThread || query.Limit != 1 || query.SessionID != delivery.SessionID {
					t.Fatalf("subscription query %d = %#v, want exact single-row session read", index, query)
				}
			}
			if preferences.queries[0].ThreadID != threadID {
				t.Fatalf("first subscription query = %#v, want exact thread %q", preferences.queries[0], threadID)
			}
			if test.wantQueries == 2 && preferences.queries[1].ThreadID != "" {
				t.Fatalf("fallback subscription query = %#v, want exact channel row", preferences.queries[1])
			}
		})
	}
}

type managerSubscriptionStoreStub struct {
	threadMode  string
	channelMode string
	queries     []store.NetworkSubscriptionQuery
}

func (s *managerSubscriptionStoreStub) ListNetworkSubscriptions(
	_ context.Context,
	query store.NetworkSubscriptionQuery,
) ([]store.NetworkSubscriptionEntry, error) {
	s.queries = append(s.queries, query)
	mode := s.channelMode
	if query.ThreadID != "" {
		mode = s.threadMode
	}
	if mode == "" {
		return nil, nil
	}
	return []store.NetworkSubscriptionEntry{{
		WorkspaceID: query.WorkspaceID,
		Channel:     query.Channel,
		ThreadID:    query.ThreadID,
		SessionID:   query.SessionID,
		Mode:        mode,
	}}, nil
}

func TestManagerSendWritesConfiguredAuditSinks(t *testing.T) {
	t.Parallel()

	acceptance := &managerAcceptanceStub{}
	acceptance.handle = func(
		req store.AcceptNetworkMessageRequest,
		_ int,
	) (store.AcceptNetworkMessageResult, error) {
		return store.AcceptNetworkMessageResult{
			Dispositions: slices.Clone(req.Dispositions),
			Conversation: store.NetworkConversationWriteResult{MessageID: req.Message.MessageID},
		}, nil
	}
	auditStore := &recordingAuditStore{}
	auditPath := filepath.Join(t.TempDir(), "logs", "network.audit")
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		auditPath,
		auditStore,
		WithManagerLogger(discardManagerLogger()),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
	joinManagerSendParticipant(t, manager, "sess-recipient", "reviewer.sess-target")

	if _, err := manager.Send(
		testutil.Context(t),
		managerThreadSendRequest("msg-manager-real-audit"),
	); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got, want := len(auditStore.entries), 2; got != want {
		t.Fatalf("durable audit entries = %d, want received and delivered only", got)
	}
	if got := []string{auditStore.entries[0].Direction, auditStore.entries[1].Direction}; !slices.Equal(
		got,
		[]string{AuditDirectionReceived, AuditDirectionDelivered},
	) {
		t.Fatalf("durable audit directions = %#v, want received and delivered", got)
	}
	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", auditPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("audit file records = %d, want sent received delivered: %s", len(lines), data)
	}
	directions := make([]string, 0, len(lines))
	for _, line := range lines {
		var entry store.NetworkAuditEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal(audit line) error = %v", err)
		}
		directions = append(directions, entry.Direction)
	}
	if want := []string{AuditDirectionSent, AuditDirectionReceived, AuditDirectionDelivered}; !slices.Equal(
		directions,
		want,
	) {
		t.Fatalf("file audit directions = %#v, want %#v", directions, want)
	}
}

func TestManagerSendGeneratesWhoisResponsesAfterRequestAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("Should persist a lean response from the matching peer", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipant(t, manager, "sess-responder", "reviewer.sess-target")
		joinManagerSendParticipant(t, manager, "sess-other", "reviewer.sess-other")
		messageID := "msg-whois-lean"

		if _, err := manager.Send(testutil.Context(t), SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: KindWhois, Body: mustRawJSON(t, WhoisBody{
				Type: WhoisTypeRequest, Query: "reviewer.sess-target",
			}),
			ID: &messageID,
		}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}

		if got := acceptance.callCount(); got != 2 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want request then one response", got)
		}
		request := acceptance.request(t, 0)
		response := acceptance.request(t, 1)
		if request.Message.MessageID != messageID {
			t.Fatalf("first accepted message id = %q, want request %q", request.Message.MessageID, messageID)
		}
		assertWhoisResponseMessage(t, response.Message, "sess-responder", "sess-sender", messageID)
		var ext ExtensionMap
		if err := json.Unmarshal(response.Message.ExtJSON, &ext); err != nil {
			t.Fatalf("json.Unmarshal(lean response ext) error = %v", err)
		}
		if len(ext) != 0 {
			t.Fatalf("lean response ext = %#v, want empty", ext)
		}
	})

	t.Run("Should persist a filtered rich capability response", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipantWithCapabilities(
			t,
			manager,
			"sess-responder",
			"reviewer.sess-rich",
			[]session.NetworkPeerCapability{
				{ID: "review-pr", Summary: "Review pull requests", Outcome: "Actionable feedback"},
				{ID: "draft-spec", Summary: "Draft technical specifications", Outcome: "A reviewed plan"},
			},
		)
		target := "reviewer.sess-rich"
		messageID := "msg-whois-rich"

		if _, err := manager.Send(testutil.Context(t), SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: KindWhois, To: &target, Body: mustRawJSON(t, WhoisBody{Type: WhoisTypeRequest}),
			ID: &messageID,
			Ext: ExtensionMap{
				whoisIncludeExtKey:       mustRawJSON(t, []string{whoisCapabilityCatalogIncludeItem}),
				whoisCapabilityIDsExtKey: mustRawJSON(t, []string{"draft-spec"}),
			},
		}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}

		response := acceptance.request(t, 1).Message
		body := decodeWhoisResponseMessage(t, response)
		if got, want := body.PeerCard.Capabilities, []string{"draft-spec"}; !slices.Equal(got, want) {
			t.Fatalf("response capabilities = %#v, want %#v", got, want)
		}
		var ext ExtensionMap
		if err := json.Unmarshal(response.ExtJSON, &ext); err != nil {
			t.Fatalf("json.Unmarshal(response ext) error = %v", err)
		}
		catalog := decodeWhoisResponseCatalog(t, ext)
		if len(catalog.Capabilities) != 1 || catalog.Capabilities[0].ID != "draft-spec" {
			t.Fatalf("filtered response catalog = %#v", catalog.Capabilities)
		}
	})

	t.Run("Should persist an explicit empty rich catalog", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipant(t, manager, "sess-responder", "reviewer.sess-empty")
		target := "reviewer.sess-empty"
		messageID := "msg-whois-empty"

		if _, err := manager.Send(testutil.Context(t), SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: KindWhois, To: &target, Body: mustRawJSON(t, WhoisBody{Type: WhoisTypeRequest}),
			ID: &messageID,
			Ext: ExtensionMap{
				whoisIncludeExtKey: mustRawJSON(t, []string{whoisCapabilityCatalogIncludeItem}),
			},
		}); err != nil {
			t.Fatalf("Send() error = %v", err)
		}

		response := acceptance.request(t, 1).Message
		var ext ExtensionMap
		if err := json.Unmarshal(response.ExtJSON, &ext); err != nil {
			t.Fatalf("json.Unmarshal(response ext) error = %v", err)
		}
		catalog := decodeWhoisResponseCatalog(t, ext)
		if len(catalog.Capabilities) != 0 {
			t.Fatalf("empty response catalog = %#v", catalog.Capabilities)
		}
	})

	t.Run("Should reject an oversized response after persisting the request", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipantWithCapabilities(
			t,
			manager,
			"sess-responder",
			"reviewer.sess-large",
			[]session.NetworkPeerCapability{{
				ID: "review-pr", Summary: "Review pull requests",
				Outcome: strings.Repeat("x", maxProtocolEnvelopeBytes),
			}},
		)
		target := "reviewer.sess-large"
		messageID := "msg-whois-large"

		_, err := manager.Send(testutil.Context(t), SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: KindWhois, To: &target, Body: mustRawJSON(t, WhoisBody{Type: WhoisTypeRequest}),
			ID: &messageID,
			Ext: ExtensionMap{
				whoisIncludeExtKey: mustRawJSON(t, []string{whoisCapabilityCatalogIncludeItem}),
			},
		})
		if !errors.Is(err, ErrEnvelopeTooLarge) {
			t.Fatalf("Send() error = %v, want ErrEnvelopeTooLarge", err)
		}
		if got := acceptance.callCount(); got != 1 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want only the durable request", got)
		}
		if got := acceptance.request(t, 0).Message.MessageID; got != messageID {
			t.Fatalf("accepted message id = %q, want request %q", got, messageID)
		}
	})

	t.Run("Should retry a failed response with the same message identity", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		var committedDispositions []store.NetworkMessageDisposition
		acceptance.handle = func(
			req store.AcceptNetworkMessageRequest,
			call int,
		) (store.AcceptNetworkMessageResult, error) {
			switch call {
			case 1:
				committedDispositions = slices.Clone(req.Dispositions)
				return store.AcceptNetworkMessageResult{
					Dispositions: slices.Clone(committedDispositions),
				}, nil
			case 2:
				return store.AcceptNetworkMessageResult{}, errors.New("response persistence failed")
			case 3:
				return store.AcceptNetworkMessageResult{
					Duplicate: true, Dispositions: slices.Clone(committedDispositions),
				}, nil
			default:
				return store.AcceptNetworkMessageResult{}, nil
			}
		}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipant(t, manager, "sess-responder", "reviewer.sess-retry")
		messageID := "msg-whois-retry"
		request := SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: KindWhois, Body: mustRawJSON(t, WhoisBody{Type: WhoisTypeRequest}),
			ID: &messageID,
		}

		if _, err := manager.Send(testutil.Context(t), request); err == nil ||
			!strings.Contains(err.Error(), "response persistence failed") {
			t.Fatalf("Send(first attempt) error = %v, want response persistence failure", err)
		}
		firstResponseID := acceptance.request(t, 1).Message.MessageID
		joinManagerSendParticipant(t, manager, "sess-late-responder", "reviewer.sess-late")
		if _, err := manager.Send(testutil.Context(t), request); err != nil {
			t.Fatalf("Send(retry) error = %v", err)
		}
		if got := acceptance.callCount(); got != 4 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want request/response twice", got)
		}
		if retryResponseID := acceptance.request(t, 3).Message.MessageID; retryResponseID != firstResponseID {
			t.Fatalf("retry response id = %q, want stable id %q", retryResponseID, firstResponseID)
		}
		if got := acceptance.request(t, 3).Message.PeerFrom; got != "sess-responder" {
			t.Fatalf("retry responder session = %q, want original sess-responder", got)
		}
	})
}

func TestRouterRoutePreparedUsesOnePreMessagePresenceSnapshot(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 16, 18, 0, 0, 0, time.UTC)
	peers, err := NewPeerRegistry(WithPeerRegistryClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewPeerRegistry() error = %v", err)
	}
	register := func(sessionID string, peerID string) {
		t.Helper()
		card, cardErr := DefaultPeerCard(peerID)
		if cardErr != nil {
			t.Fatalf("DefaultPeerCard(%q) error = %v", peerID, cardErr)
		}
		if _, registerErr := peers.RegisterLocal(
			sessionID,
			testWorkspaceID,
			"builders",
			card,
			now,
		); registerErr != nil {
			t.Fatalf("RegisterLocal(%q) error = %v", sessionID, registerErr)
		}
	}
	register("sess-sender", "sender.sess-abc")
	register("sess-recipient-old", "reviewer.sess-old")
	resolver := newBlockingThreadParticipantResolver([]store.NetworkThreadParticipant{{
		WorkspaceID: testWorkspaceID,
		Channel:     "builders",
		ThreadID:    "thread_presence_snapshot",
		SessionID:   "sess-recipient-old",
	}})
	t.Cleanup(resolver.release)
	router, err := NewRouter(
		peers,
		DefaultMaxReplayAge,
		WithRouterClock(func() time.Time { return now }),
		WithRouterThreadParticipantResolver(resolver),
	)
	if err != nil {
		t.Fatalf("NewRouter() error = %v", err)
	}
	surface := SurfaceThread
	threadID := "thread_presence_snapshot"
	messageID := "msg_presence_snapshot"
	prepared, err := router.PrepareSend(t.Context(), SendRequest{
		SessionID:   "sess-sender",
		WorkspaceID: testWorkspaceID,
		Channel:     "builders",
		Surface:     &surface,
		ThreadID:    &threadID,
		Kind:        KindSay,
		Body:        json.RawMessage(`{"text":"snapshot this membership"}`),
		ID:          &messageID,
	})
	if err != nil {
		t.Fatalf("PrepareSend() error = %v", err)
	}

	type routeOutcome struct {
		result RouteResult
		err    error
	}
	done := make(chan routeOutcome, 1)
	go func() {
		result, routeErr := router.RoutePrepared(t.Context(), prepared)
		done <- routeOutcome{result: result, err: routeErr}
	}()
	select {
	case <-resolver.started:
	case <-time.After(time.Second):
		t.Fatal("RoutePrepared() did not reach the participant snapshot boundary")
	}
	if _, ok := peers.LeaveLocal("sess-recipient-old"); !ok {
		t.Fatal("LeaveLocal(old recipient) = false, want true")
	}
	register("sess-recipient-new", "reviewer.sess-new")
	resolver.release()

	var outcome routeOutcome
	select {
	case outcome = <-done:
	case <-time.After(time.Second):
		t.Fatal("RoutePrepared() did not finish after participant snapshot release")
	}
	if outcome.err != nil {
		t.Fatalf("RoutePrepared() error = %v", outcome.err)
	}
	if got, want := routeDeliverySessionIDs(outcome.result.Deliveries),
		[]string{"sess-recipient-old"}; !slices.Equal(got, want) {
		t.Fatalf("delivery sessions = %#v, want pre-message snapshot %#v", got, want)
	}
	if got, want := routeDispositionSessionIDs(outcome.result.Dispositions),
		[]string{"sess-recipient-old"}; !slices.Equal(got, want) {
		t.Fatalf("disposition sessions = %#v, want pre-message snapshot %#v", got, want)
	}
}

func TestManagerWakeAdmissionEligibility(t *testing.T) {
	t.Parallel()

	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerAuditWriter(managerAuditWriterStub{}),
		WithManagerAcceptanceStore(&managerAcceptanceStub{}),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-recipient", "reviewer.sess-target")

	tests := []struct {
		name      string
		kind      Kind
		direct    bool
		mention   bool
		eligible  bool
		addressed bool
		trigger   string
	}{
		{
			name: "direct say", kind: KindSay, direct: true,
			eligible: true, addressed: true, trigger: store.NetworkWakeTriggerDirect,
		},
		{
			name: "mentioned say", kind: KindSay, mention: true,
			eligible: true, addressed: true, trigger: store.NetworkWakeTriggerMention,
		},
		{name: "unaddressed say", kind: KindSay, eligible: true},
		{name: "greet control", kind: KindGreet, direct: true},
		{name: "whois control", kind: KindWhois, direct: true},
		{name: "capability status", kind: KindCapability, direct: true},
		{name: "receipt control", kind: KindReceipt, direct: true},
		{name: "trace control", kind: KindTrace, direct: true},
	}
	for _, test := range tests {
		t.Run("Should classify "+test.name, func(t *testing.T) {
			envelope := Envelope{Kind: test.kind}
			if test.direct {
				target := "reviewer.sess-target"
				envelope.To = &target
			}
			if test.mention {
				envelope.Mentions = []string{"reviewer.sess-target"}
			}
			admissions := manager.wakeAdmissions([]Delivery{{
				SessionID: "sess-recipient",
				PeerID:    "reviewer.sess-target",
				Envelope:  envelope,
			}}, "root-eligibility", 0)
			if len(admissions) != 1 {
				t.Fatalf("wakeAdmissions() = %#v, want one recipient decision", admissions)
			}
			admission := admissions[0]
			if admission.Eligible != test.eligible || admission.Addressed != test.addressed ||
				admission.Trigger != test.trigger {
				t.Fatalf("wake admission = %#v, want eligible=%t addressed=%t trigger=%q",
					admission,
					test.eligible,
					test.addressed,
					test.trigger,
				)
			}
			if test.addressed {
				if admission.WakeID == "" || admission.TaskRunID == "" {
					t.Fatalf("addressed wake IDs = (%q, %q), want allocated IDs", admission.WakeID, admission.TaskRunID)
				}
				return
			}
			if admission.WakeID != "" || admission.TaskRunID != "" {
				t.Fatalf("non-addressed wake IDs = (%q, %q), want none", admission.WakeID, admission.TaskRunID)
			}
		})
	}
}

func TestManagerSendRejectsRawCredentialsBeforeAcceptance(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	acceptance := &managerAcceptanceStub{}
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
		WithManagerAuditWriter(managerAuditWriterStub{}),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
	logs.Reset()

	rawToken := "agh_claim_MANAGER_SECURITY_123"
	tests := []struct {
		name string
		body json.RawMessage
		ext  ExtensionMap
	}{
		{
			name: "body",
			body: json.RawMessage(`{"text":"unsafe","nested":{"claim_token":"` + rawToken + `"}}`),
		},
		{
			name: "extension metadata",
			body: json.RawMessage(`{"text":"unsafe ext"}`),
			ext: ExtensionMap{
				"agh.metadata": json.RawMessage(`{"claim_token":"` + rawToken + `"}`),
			},
		},
	}
	for _, test := range tests {
		t.Run("Should reject raw claim token in "+test.name, func(t *testing.T) {
			request := managerThreadSendRequest("msg-security-" + strings.ReplaceAll(test.name, " ", "-"))
			request.Body = test.body
			request.Ext = test.ext
			if _, sendErr := manager.Send(testutil.Context(t), request); !errors.Is(sendErr, ErrInvalidBody) {
				t.Fatalf("Send() error = %v, want ErrInvalidBody", sendErr)
			}
		})
	}

	if got := acceptance.callCount(); got != 0 {
		t.Fatalf("AcceptNetworkMessage calls = %d, want 0", got)
	}
	logOutput := logs.String()
	if strings.Contains(logOutput, rawToken) {
		t.Fatalf("pre-persistence logs leaked raw claim token: %s", logOutput)
	}
	if got := strings.Count(logOutput, "network.message.rejected_pre_persistence"); got != len(tests) {
		t.Fatalf("pre-persistence rejection log count = %d, want %d; logs=%s", got, len(tests), logOutput)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(logOutput), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("json.Unmarshal(log) error = %v; line=%s", err, line)
		}
		digest, ok := record["payload_sha256"].(string)
		if !ok {
			t.Fatalf("rejection log missing payload_sha256: %#v", record)
		}
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 {
			t.Fatalf("payload_sha256 = %q, want 32-byte SHA-256; error=%v", digest, err)
		}
	}
}

func TestManagerSendRejectsInvalidEnvelopeBeforeAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an oversized envelope", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		request := managerThreadSendRequest("msg-manager-oversized")
		request.Body = json.RawMessage(`{"text":"` + strings.Repeat("x", 1<<20) + `"}`)

		if _, err := manager.Send(testutil.Context(t), request); !errors.Is(err, ErrEnvelopeTooLarge) {
			t.Fatalf("Send() error = %v, want ErrEnvelopeTooLarge", err)
		}
		if got := acceptance.callCount(); got != 0 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want 0", got)
		}
	})

	t.Run("Should compact insignificant whitespace before acceptance", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		request := managerThreadSendRequest("msg-manager-whitespace")
		request.Body = json.RawMessage(`{` + strings.Repeat(" ", maxProtocolEnvelopeBytes) + `"text":"ok"}`)

		if _, err := manager.Send(testutil.Context(t), request); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if got, want := string(acceptance.request(t, 0).Message.Body), `{"text":"ok"}`; got != want {
			t.Fatalf("accepted body = %q, want canonical %q", got, want)
		}
	})

	t.Run("Should preserve the missing-target sentinel for session sends", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		surface := SurfaceDirect
		target := "reviewer.sess-missing"
		directID := "direct_0123456789abcdef0123456789abcdef"
		request := SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Surface: &surface, DirectID: &directID, Kind: KindSay, To: &target,
			Body: json.RawMessage(`{"text":"find target"}`),
		}

		if _, err := manager.Send(testutil.Context(t), request); !errors.Is(err, ErrTargetPeerNotFound) {
			t.Fatalf("Send() error = %v, want ErrTargetPeerNotFound", err)
		}
		if got := acceptance.callCount(); got != 0 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want 0", got)
		}
	})

	t.Run("Should preserve the missing-target sentinel for runtime sends", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		surface := SurfaceDirect
		target := "reviewer.sess-missing"
		directID := "direct_0123456789abcdef0123456789abcdef"
		request := RuntimeSendRequest{
			WorkspaceID: testWorkspaceID, Channel: "builders", Surface: &surface,
			DirectID: &directID, Kind: KindSay, To: &target,
			Body: json.RawMessage(`{"text":"find target"}`),
		}

		if _, err := manager.SendFromRuntimePeer(testutil.Context(t), request); !errors.Is(err, ErrTargetPeerNotFound) {
			t.Fatalf("SendFromRuntimePeer() error = %v, want ErrTargetPeerNotFound", err)
		}
		if got := acceptance.callCount(); got != 0 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want 0", got)
		}
	})

	t.Run("Should reject a direct room bound to another target pair", func(t *testing.T) {
		t.Parallel()

		acceptance := &managerAcceptanceStub{}
		manager := newManagerSendTestManager(t, acceptance)
		joinManagerSendParticipant(t, manager, "sess-sender", "sender.sess-abc")
		joinManagerSendParticipant(t, manager, "sess-recipient", "reviewer.sess-target")
		canonicalID, _, _, err := store.NetworkDirectRoomIdentity(
			testWorkspaceID,
			"builders",
			"sess-sender",
			"sess-recipient",
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		mismatchedID := "direct_0123456789abcdef0123456789abcdef"
		if mismatchedID == canonicalID {
			mismatchedID = "direct_fedcba9876543210fedcba9876543210"
		}
		surface := SurfaceDirect
		target := "reviewer.sess-target"
		request := SendRequest{
			SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
			Surface: &surface, DirectID: &mismatchedID, Kind: KindSay, To: &target,
			Body: json.RawMessage(`{"text":"wrong room"}`),
		}

		if _, sendErr := manager.Send(testutil.Context(t), request); !errors.Is(sendErr, ErrDirectRoomCollision) {
			t.Fatalf("Send() error = %v, want ErrDirectRoomCollision", sendErr)
		}
		if got := acceptance.callCount(); got != 0 {
			t.Fatalf("AcceptNetworkMessage calls = %d, want 0", got)
		}
	})
}

func newManagerSendTestManager(t *testing.T, acceptance *managerAcceptanceStub) *Manager {
	t.Helper()

	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerAuditWriter(managerAuditWriterStub{}),
		WithManagerAcceptanceStore(acceptance),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})
	return manager
}

func joinManagerSendParticipant(t *testing.T, manager *Manager, sessionID string, peerID string) {
	t.Helper()
	joinManagerSendParticipantWithCapabilities(t, manager, sessionID, peerID, nil)
}

func joinManagerSendParticipantWithCapabilities(
	t *testing.T,
	manager *Manager,
	sessionID string,
	peerID string,
	capabilities []session.NetworkPeerCapability,
) {
	t.Helper()
	spec := managerSendLiveSpec()
	if err := manager.JoinChannel(testutil.Context(t), session.NetworkPeerJoin{
		SessionID:            sessionID,
		PeerID:               peerID,
		WorkspaceID:          testWorkspaceID,
		Channel:              "builders",
		OwnerKey:             "session:" + sessionID,
		NetworkParticipation: spec,
		Capabilities:         capabilities,
	}); err != nil {
		t.Fatalf("JoinChannel(%q) error = %v", sessionID, err)
	}
}

func assertWhoisResponseMessage(
	t *testing.T,
	message store.NetworkConversationMessage,
	wantFrom string,
	wantTo string,
	wantReplyTo string,
) {
	t.Helper()
	if message.Kind != string(KindWhois) {
		t.Fatalf("response kind = %q, want %q", message.Kind, KindWhois)
	}
	if message.PeerFrom != wantFrom || message.PeerTo != wantTo {
		t.Fatalf("response peers = %q -> %q, want %q -> %q", message.PeerFrom, message.PeerTo, wantFrom, wantTo)
	}
	if message.ReplyTo != wantReplyTo {
		t.Fatalf("response reply_to = %q, want %q", message.ReplyTo, wantReplyTo)
	}
	body := decodeWhoisResponseMessage(t, message)
	if body.PeerCard.PeerID != "reviewer.sess-target" {
		t.Fatalf("response peer card id = %q, want reviewer.sess-target", body.PeerCard.PeerID)
	}
}

func decodeWhoisResponseMessage(t *testing.T, message store.NetworkConversationMessage) WhoisBody {
	t.Helper()
	body, err := DecodeBody(KindWhois, message.Body)
	if err != nil {
		t.Fatalf("DecodeBody(response) error = %v", err)
	}
	whois, ok := body.(WhoisBody)
	if !ok || whois.Type != WhoisTypeResponse || whois.PeerCard == nil {
		t.Fatalf("response body = %#v, want whois response with peer card", body)
	}
	return whois
}

func decodeWhoisResponseCatalog(t *testing.T, ext ExtensionMap) whoisCapabilityCatalogPayload {
	t.Helper()
	raw, ok := ext[whoisCapabilityCatalogExtKey]
	if !ok {
		t.Fatal("whois response capability catalog extension is missing")
	}
	var payload whoisCapabilityCatalogPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("json.Unmarshal(capability catalog) error = %v", err)
	}
	return payload
}

func managerSendLiveSpec() participation.Spec {
	defaults := aghconfig.DefaultNetworkConfig().Live.Defaults
	return participation.Spec{
		Version: participation.SpecVersion, Mode: participation.ModeLive,
		WorkspaceID: testWorkspaceID, ChannelStrategy: participation.StrategyNamed,
		ChannelID: "builders", Source: participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes: defaults.MaxWakes, MaxWakeWallTime: defaults.MaxWakeWallTime,
			MaxTotalWallTime: defaults.MaxTotalWallTime, MaxInputTokens: defaults.MaxInputTokens,
			MaxOutputTokens: defaults.MaxOutputTokens, MaxWakeDepth: defaults.MaxWakeDepth,
			CoalesceWindow: defaults.CoalesceWindow,
		},
	}
}

func managerThreadSendRequest(messageID string) SendRequest {
	surface := SurfaceThread
	threadID := "thread_manager_commit"
	return SendRequest{
		SessionID: "sess-sender", WorkspaceID: testWorkspaceID, Channel: "builders",
		Surface: &surface, ThreadID: &threadID, Kind: KindSay,
		Body: json.RawMessage(`{"text":"review this"}`), ID: &messageID,
	}
}

func managerDispositionRecipients(values []store.NetworkMessageDisposition) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value.Decision == store.NetworkDispositionDeliver {
			result = append(result, value.RecipientSessionID)
		}
	}
	return result
}

func managerAdmissionRecipients(values []store.NetworkWakeAdmissionInput) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.RecipientSessionID)
	}
	return result
}

func routeDeliverySessionIDs(values []Delivery) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.SessionID)
	}
	return result
}

func routeDispositionSessionIDs(values []RoutingDisposition) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.SessionID)
	}
	return result
}

type blockingThreadParticipantResolver struct {
	started      chan struct{}
	proceed      chan struct{}
	startOnce    sync.Once
	releaseOnce  sync.Once
	participants []store.NetworkThreadParticipant
}

func newBlockingThreadParticipantResolver(
	participants []store.NetworkThreadParticipant,
) *blockingThreadParticipantResolver {
	return &blockingThreadParticipantResolver{
		started:      make(chan struct{}),
		proceed:      make(chan struct{}),
		participants: slices.Clone(participants),
	}
}

func (r *blockingThreadParticipantResolver) ListThreadParticipants(
	ctx context.Context,
	_ store.NetworkChannelRef,
	_ string,
) ([]store.NetworkThreadParticipant, error) {
	r.startOnce.Do(func() { close(r.started) })
	select {
	case <-r.proceed:
		return slices.Clone(r.participants), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *blockingThreadParticipantResolver) release() {
	r.releaseOnce.Do(func() { close(r.proceed) })
}

type managerAcceptanceStub struct {
	mu       sync.Mutex
	requests []store.AcceptNetworkMessageRequest
	handle   func(store.AcceptNetworkMessageRequest, int) (store.AcceptNetworkMessageResult, error)
	inFlight bool
}

func (s *managerAcceptanceStub) AcceptNetworkMessage(
	_ context.Context,
	req store.AcceptNetworkMessageRequest,
) (store.AcceptNetworkMessageResult, error) {
	s.mu.Lock()
	s.requests = append(s.requests, req)
	call := len(s.requests)
	handle := s.handle
	s.inFlight = true
	s.mu.Unlock()
	var result store.AcceptNetworkMessageResult
	var err error
	if handle == nil {
		result = store.AcceptNetworkMessageResult{}
	} else {
		result, err = handle(req, call)
	}
	s.mu.Lock()
	s.inFlight = false
	s.mu.Unlock()
	return result, err
}

func (*managerAcceptanceStub) SettleNetworkWake(
	context.Context,
	store.WakeReservation,
	store.NetworkWakeOutcome,
) error {
	return nil
}

func (s *managerAcceptanceStub) request(t *testing.T, index int) store.AcceptNetworkMessageRequest {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.requests) {
		t.Fatalf("acceptance request index = %d, calls = %d", index, len(s.requests))
	}
	return s.requests[index]
}

func (s *managerAcceptanceStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *managerAcceptanceStub) accepting() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inFlight
}

type managerHookCall struct {
	event   hookspkg.HookEvent
	payload hookspkg.NetworkPayload
}

type managerHookDispatcherSpy struct {
	mu         sync.Mutex
	calls      []managerHookCall
	err        error
	acceptance *managerAcceptanceStub
	misses     int
}

func (s *managerHookDispatcherSpy) DispatchNetworkPeerJoined(
	_ context.Context,
	payload hookspkg.NetworkPeerJoinedPayload,
) (hookspkg.NetworkPeerJoinedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkPeerJoined, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkPeerLeft(
	_ context.Context,
	payload hookspkg.NetworkPeerLeftPayload,
) (hookspkg.NetworkPeerLeftPayload, error) {
	return payload, s.record(hookspkg.HookNetworkPeerLeft, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkThreadOpened(
	_ context.Context,
	payload hookspkg.NetworkThreadOpenedPayload,
) (hookspkg.NetworkThreadOpenedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkThreadOpened, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkDirectRoomOpened(
	_ context.Context,
	payload hookspkg.NetworkDirectRoomOpenedPayload,
) (hookspkg.NetworkDirectRoomOpenedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkDirectRoomOpened, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkMessagePersisted(
	_ context.Context,
	payload hookspkg.NetworkMessagePersistedPayload,
) (hookspkg.NetworkMessagePersistedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkMessagePersisted, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkWorkOpened(
	_ context.Context,
	payload hookspkg.NetworkWorkOpenedPayload,
) (hookspkg.NetworkWorkOpenedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkWorkOpened, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkWorkTransitioned(
	_ context.Context,
	payload hookspkg.NetworkWorkTransitionedPayload,
) (hookspkg.NetworkWorkTransitionedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkWorkTransitioned, payload)
}

func (s *managerHookDispatcherSpy) DispatchNetworkWorkClosed(
	_ context.Context,
	payload hookspkg.NetworkWorkClosedPayload,
) (hookspkg.NetworkWorkClosedPayload, error) {
	return payload, s.record(hookspkg.HookNetworkWorkClosed, payload)
}

func (s *managerHookDispatcherSpy) record(event hookspkg.HookEvent, payload hookspkg.NetworkPayload) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acceptance != nil && s.acceptance.accepting() {
		s.misses++
	}
	s.calls = append(s.calls, managerHookCall{event: event, payload: payload})
	return s.err
}

func (s *managerHookDispatcherSpy) events() []hookspkg.HookEvent {
	calls := s.callsSnapshot()
	events := make([]hookspkg.HookEvent, 0, len(calls))
	for _, call := range calls {
		events = append(events, call.event)
	}
	return events
}

func (s *managerHookDispatcherSpy) callsSnapshot() []managerHookCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *managerHookDispatcherSpy) commitMissCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.misses
}

func (s *managerHookDispatcherSpy) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = nil
	s.misses = 0
}

type managerAuditCall struct {
	direction string
	sessionID string
	reason    string
}

type managerAuditWriterSpy struct {
	mu         sync.Mutex
	calls      []managerAuditCall
	err        error
	acceptance *managerAcceptanceStub
	misses     int
}

func (s *managerAuditWriterSpy) RecordSent(_ context.Context, sessionID string, _ Envelope) error {
	return s.record(AuditDirectionSent, sessionID, "")
}

func (s *managerAuditWriterSpy) RecordReceived(_ context.Context, sessionID string, _ Envelope) error {
	return s.record(AuditDirectionReceived, sessionID, "")
}

func (s *managerAuditWriterSpy) RecordRejected(
	_ context.Context,
	sessionID string,
	_ Envelope,
	reason string,
) error {
	return s.record(AuditDirectionRejected, sessionID, reason)
}

func (s *managerAuditWriterSpy) RecordDelivered(_ context.Context, sessionID string, _ Envelope) error {
	return s.record(AuditDirectionDelivered, sessionID, "")
}

func (s *managerAuditWriterSpy) record(direction string, sessionID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.acceptance != nil && s.acceptance.accepting() {
		s.misses++
	}
	s.calls = append(s.calls, managerAuditCall{direction: direction, sessionID: sessionID, reason: reason})
	return s.err
}

func (s *managerAuditWriterSpy) callsSnapshot() []managerAuditCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.calls)
}

func (s *managerAuditWriterSpy) commitMissCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.misses
}

type managerWakeNotifierStub struct {
	mu            sync.Mutex
	notifications []store.CommittedNetworkNotification
}

func (s *managerWakeNotifierStub) NotifyNetworkWake(notification store.CommittedNetworkNotification) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifications = append(s.notifications, notification)
}

func (s *managerWakeNotifierStub) recipients() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.notifications))
	for _, notification := range s.notifications {
		result = append(result, notification.RecipientSessionID)
	}
	return result
}

type managerAuditWriterStub struct{}

func (managerAuditWriterStub) RecordSent(context.Context, string, Envelope) error { return nil }
func (managerAuditWriterStub) RecordReceived(context.Context, string, Envelope) error {
	return nil
}
func (managerAuditWriterStub) RecordRejected(context.Context, string, Envelope, string) error {
	return nil
}
func (managerAuditWriterStub) RecordDelivered(context.Context, string, Envelope) error { return nil }
