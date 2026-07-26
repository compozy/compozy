// Suite: Network manager durable inbox
// Invariant: inbox reads are session/workspace scoped and waits wake only for committed delivery.
// Boundary IN: the in-process Manager plus the durable inbox store contract.
// Boundary OUT: API/CLI rendering, owned by their transport suites.
package network

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestManagerInboxScopesAndReconstructsStoredMessages(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC)
	entry := managerInboxMessageEntry(timestamp)
	inbox := &managerInboxStoreStub{responses: [][]store.NetworkMessageEntry{{entry}}}
	manager := newManagerInboxTestManager(t, inbox)
	joinManagerSendParticipant(t, manager, "sess-inbox", "reviewer.sess-inbox")

	messages, err := manager.Inbox(testutil.Context(t), " sess-inbox ")
	if err != nil {
		t.Fatalf("Inbox() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("Inbox() messages = %#v, want one", messages)
	}
	message := messages[0]
	if message.ID != entry.MessageID || message.WorkspaceID != testWorkspaceID ||
		message.Channel != "builders" || message.Kind != KindSay || message.TS != timestamp.Unix() {
		t.Fatalf("Inbox() envelope identity = %#v, want stored identity", message)
	}
	if message.Surface == nil || *message.Surface != SurfaceThread ||
		message.ThreadID == nil || *message.ThreadID != "thread-inbox" ||
		message.To == nil || *message.To != "sender.sess-target" {
		t.Fatalf("Inbox() conversation references = %#v, want thread and target", message)
	}
	if !slices.Equal(message.Mentions, []string{"reviewer.sess-inbox"}) ||
		string(message.Ext["agh.test"]) != `"value"` || string(message.Body) != `{"text":"stored"}` {
		t.Fatalf("Inbox() message payload = %#v, want stored body/ext/mentions", message)
	}
	queries := inbox.queriesSnapshot()
	if len(queries) != 1 || queries[0].workspaceID != testWorkspaceID ||
		queries[0].sessionID != "sess-inbox" || queries[0].channel != "" ||
		queries[0].limit != store.NetworkMessageDefaultLimit {
		t.Fatalf("ListNetworkInbox query = %#v, want authoritative session scope", queries)
	}
	entry.Mentions[0] = "mutated"
	entry.Body[0] = '['
	if message.Mentions[0] != "reviewer.sess-inbox" || string(message.Body) != `{"text":"stored"}` {
		t.Fatalf("Inbox() envelope aliased store memory: %#v", message)
	}
}

func TestManagerWaitInboxWakesOnlyForDeliveredRecipient(t *testing.T) {
	t.Parallel()

	inbox := &managerInboxStoreStub{
		responses: [][]store.NetworkMessageEntry{
			nil,
			{managerInboxMessageEntry(time.Date(2026, 7, 17, 13, 5, 0, 0, time.UTC))},
		},
		listed: make(chan struct{}, 2),
	}
	manager := newManagerInboxTestManager(t, inbox)
	joinManagerSendParticipant(t, manager, "sess-inbox", "reviewer.sess-inbox")

	type waitResult struct {
		messages []Envelope
		err      error
	}
	result := make(chan waitResult, 1)
	go func() {
		messages, err := manager.WaitInbox(t.Context(), "sess-inbox", "builders")
		result <- waitResult{messages: messages, err: err}
	}()
	select {
	case <-inbox.listed:
	case <-time.After(time.Second):
		t.Fatal("WaitInbox() did not perform the initial durable read")
	}

	manager.notifyInboxRecipients([]store.NetworkMessageDisposition{{
		RecipientSessionID: "sess-inbox", Decision: store.NetworkDispositionIgnore,
	}})
	select {
	case got := <-result:
		t.Fatalf("WaitInbox() returned for ignored recipient: %#v", got)
	case <-time.After(25 * time.Millisecond):
	}
	manager.notifyInboxRecipients([]store.NetworkMessageDisposition{{
		RecipientSessionID: "sess-inbox", Decision: store.NetworkDispositionDeliver,
	}})
	select {
	case got := <-result:
		if got.err != nil || len(got.messages) != 1 || got.messages[0].ID != "msg-inbox" {
			t.Fatalf("WaitInbox() result = %#v, want committed delivery", got)
		}
	case <-time.After(time.Second):
		t.Fatal("WaitInbox() did not wake for committed delivery")
	}
	queries := inbox.queriesSnapshot()
	if len(queries) != 2 || queries[0].channel != "builders" || queries[1].channel != "builders" {
		t.Fatalf("WaitInbox() queries = %#v, want two channel-scoped reads", queries)
	}
}

func TestManagerWaitInboxHonorsCancellationAndShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Should return caller cancellation", func(t *testing.T) {
		t.Parallel()

		inbox := &managerInboxStoreStub{listed: make(chan struct{}, 1)}
		manager := newManagerInboxTestManager(t, inbox)
		joinManagerSendParticipant(t, manager, "sess-inbox", "reviewer.sess-inbox")
		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			_, err := manager.WaitInbox(ctx, "sess-inbox", "builders")
			result <- err
		}()
		<-inbox.listed
		cancel()
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitInbox() error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should return lifecycle cancellation on shutdown", func(t *testing.T) {
		t.Parallel()

		inbox := &managerInboxStoreStub{listed: make(chan struct{}, 1)}
		manager := newManagerInboxTestManager(t, inbox)
		joinManagerSendParticipant(t, manager, "sess-inbox", "reviewer.sess-inbox")
		result := make(chan error, 1)
		go func() {
			_, err := manager.WaitInbox(t.Context(), "sess-inbox", "builders")
			result <- err
		}()
		<-inbox.listed
		if err := manager.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitInbox() error = %v, want lifecycle context.Canceled", err)
		}
	})
}

func TestManagerInboxRejectsInvalidBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil context and unmanaged sessions", func(t *testing.T) {
		t.Parallel()

		manager := newManagerInboxTestManager(t, &managerInboxStoreStub{})
		if _, err := manager.Inbox(nilManagerInboxContext(), "sess-inbox"); err == nil {
			t.Fatal("Inbox(nil context) error = nil, want non-nil")
		}
		if _, err := manager.Inbox(t.Context(), "sess-unmanaged"); err == nil {
			t.Fatal("Inbox(unmanaged session) error = nil, want non-nil")
		}
		if _, err := manager.WaitInbox(nilManagerInboxContext(), "sess-inbox", "builders"); err == nil {
			t.Fatal("WaitInbox(nil context) error = nil, want non-nil")
		}
	})

	t.Run("Should propagate store and stored-extension errors", func(t *testing.T) {
		t.Parallel()

		storeErr := errors.New("inbox unavailable")
		inbox := &managerInboxStoreStub{err: storeErr}
		manager := newManagerInboxTestManager(t, inbox)
		joinManagerSendParticipant(t, manager, "sess-inbox", "reviewer.sess-inbox")
		if _, err := manager.Inbox(t.Context(), "sess-inbox"); !errors.Is(err, storeErr) {
			t.Fatalf("Inbox(store failure) error = %v, want %v", err, storeErr)
		}
		inbox.setError(nil)
		inbox.setResponses([][]store.NetworkMessageEntry{{{
			MessageID: "msg-invalid-ext", WorkspaceID: testWorkspaceID, Channel: "builders",
			Kind: string(KindSay), PeerFrom: "sender.sess-a", Body: json.RawMessage(`{"text":"safe"}`),
			ExtJSON: json.RawMessage(`{`), Timestamp: time.Now().UTC(),
		}}})
		if _, err := manager.Inbox(t.Context(), "sess-inbox"); err == nil {
			t.Fatal("Inbox(invalid stored ext) error = nil, want non-nil")
		}
	})
}

func newManagerInboxTestManager(t *testing.T, inbox *managerInboxStoreStub) *Manager {
	t.Helper()
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		inbox,
		WithManagerLogger(discardManagerLogger()),
		WithManagerAuditWriter(managerAuditWriterStub{}),
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

func nilManagerInboxContext() context.Context {
	return nil
}

func managerInboxMessageEntry(timestamp time.Time) store.NetworkMessageEntry {
	return store.NetworkMessageEntry{
		MessageID: "msg-inbox", WorkspaceID: testWorkspaceID, Channel: "builders",
		Surface: store.NetworkSurfaceThread, ThreadID: "thread-inbox",
		Direction: AuditDirectionSent, PeerFrom: "sender.sess-a", PeerTo: "sender.sess-target",
		Kind: string(KindSay), Mentions: []string{"reviewer.sess-inbox"},
		ExtJSON: json.RawMessage(`{"agh.test":"value"}`), Body: json.RawMessage(`{"text":"stored"}`),
		Timestamp: timestamp,
	}
}

type managerInboxQuery struct {
	workspaceID string
	sessionID   string
	channel     string
	limit       int
}

type managerInboxStoreStub struct {
	mu        sync.Mutex
	responses [][]store.NetworkMessageEntry
	queries   []managerInboxQuery
	err       error
	listed    chan struct{}
}

func (s *managerInboxStoreStub) WriteNetworkAudit(context.Context, store.NetworkAuditEntry) error {
	return nil
}

func (s *managerInboxStoreStub) ListNetworkInbox(
	_ context.Context,
	workspaceID string,
	sessionID string,
	channel string,
	limit int,
) ([]store.NetworkMessageEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, managerInboxQuery{
		workspaceID: workspaceID, sessionID: sessionID, channel: channel, limit: limit,
	})
	if s.listed != nil {
		select {
		case s.listed <- struct{}{}:
		default:
		}
	}
	if s.err != nil {
		return nil, s.err
	}
	index := len(s.queries) - 1
	if index >= len(s.responses) {
		return nil, nil
	}
	return slices.Clone(s.responses[index]), nil
}

func (s *managerInboxStoreStub) queriesSnapshot() []managerInboxQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.queries)
}

func (s *managerInboxStoreStub) setError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

func (s *managerInboxStoreStub) setResponses(responses [][]store.NetworkMessageEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responses = responses
	s.queries = nil
}
