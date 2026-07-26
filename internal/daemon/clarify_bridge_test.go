package daemon

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestClarifyBridgeLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a bounded choice and release the blocked caller", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{
			Question: " Which path? ",
			Choices:  []string{" Fast ", "Safe"},
		})
		pendingEvent := publisher.await(t)
		if pendingEvent.Status != toolspkg.ClarifyStatusPending {
			t.Fatalf("pending event status = %q, want %q", pendingEvent.Status, toolspkg.ClarifyStatusPending)
		}
		pending := awaitPendingClarification(t, bridge, scope)
		choice := 1
		answer, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &choice},
		)
		if err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		if answer.Choice == nil || *answer.Choice != choice || answer.Text != "" || answer.Fallback {
			t.Fatalf("Answer() = %#v, want resolved choice %d", answer, choice)
		}
		got := awaitClarifyResult(t, result)
		if got.err != nil {
			t.Fatalf("Ask() error = %v", got.err)
		}
		if got.answer.Choice == nil || *got.answer.Choice != choice {
			t.Fatalf("Ask() answer = %#v, want choice %d", got.answer, choice)
		}
		resolved := publisher.await(t)
		if resolved.Status != toolspkg.ClarifyStatusResolved {
			t.Fatalf("resolved event status = %q, want %q", resolved.Status, toolspkg.ClarifyStatusResolved)
		}
	})

	t.Run("Should reject a second unresolved question without replacing the first", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		first := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "First?"})
		publisher.await(t)
		_, err := bridge.Ask(testutil.Context(t), scope, toolspkg.ClarifyQuestion{Question: "Second?"})
		if !errors.Is(err, toolspkg.ErrClarifyPending) {
			t.Fatalf("Ask(second) error = %v, want %v", err, toolspkg.ErrClarifyPending)
		}
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, first); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask(first) error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should cancel a pending request while the session recorder is still finalizing", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		bridge.OnSessionFinalizing(testutil.Context(t), &session.Session{ID: scope.SessionID})
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
		canceled := publisher.await(t)
		if got, want := canceled.Status, toolspkg.ClarifyStatusCanceled; got != want {
			t.Fatalf("canceled event status = %q, want %q", got, want)
		}
	})

	t.Run("Should return only the exact fallback sentinel on timeout", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 10*time.Millisecond, publisher)
		answer, err := bridge.Ask(
			testutil.Context(t),
			testClarifyScope(),
			toolspkg.ClarifyQuestion{Question: "Continue?"},
		)
		if err != nil {
			t.Fatalf("Ask() error = %v", err)
		}
		if answer.Choice != nil || answer.Text != "" || !answer.Fallback {
			t.Fatalf("Ask() answer = %#v, want exact fallback sentinel", answer)
		}
		statuses := publisher.statuses()
		if !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusTimedOut,
		}) {
			t.Fatalf("published statuses = %v, want pending then timed_out", statuses)
		}
	})

	t.Run("Should return a typed cancellation error instead of fallback", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		ctx, cancel := context.WithCancel(testutil.Context(t))
		result := make(chan clarifyResult, 1)
		go func() {
			answer, err := bridge.Ask(ctx, testClarifyScope(), toolspkg.ClarifyQuestion{Question: "Continue?"})
			result <- clarifyResult{answer: answer, err: err}
		}()
		publisher.await(t)
		cancel()
		got := awaitClarifyResult(t, result)
		if !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
		if got.answer.Fallback {
			t.Fatalf("Ask() answer = %#v, cancellation must not be fallback", got.answer)
		}
	})
}

func TestClarifyBridgeFailurePaths(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back registration when pending persistence fails", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		publisher.failNext(toolspkg.ClarifyStatusPending, errors.New("write pending"))
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		_, err := bridge.Ask(testutil.Context(t), scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		if err == nil || !strings.Contains(err.Error(), "write pending") {
			t.Fatalf("Ask() error = %v, want pending persistence failure", err)
		}
		pending, pendingErr := bridge.Pending(testutil.Context(t), scope)
		if pendingErr != nil {
			t.Fatalf("Pending() error = %v", pendingErr)
		}
		if len(pending) != 0 {
			t.Fatalf("Pending() = %#v, want rollback", pending)
		}
	})

	t.Run("Should keep an unpersisted registration exclusive but non-interactive", func(t *testing.T) {
		t.Parallel()

		publisher := newBlockingClarifyPublisher()
		bridge := newTestClarifyBridge(t, 5*time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.awaitPendingPublish(t)

		_, err := bridge.Ask(testutil.Context(t), scope, toolspkg.ClarifyQuestion{Question: "Second?"})
		if !errors.Is(err, toolspkg.ErrClarifyPending) {
			t.Fatalf("Ask(second) error = %v, want %v", err, toolspkg.ErrClarifyPending)
		}
		pending, err := bridge.Pending(testutil.Context(t), scope)
		if err != nil {
			t.Fatalf("Pending() error = %v", err)
		}
		if len(pending) != 0 {
			t.Fatalf("Pending() = %#v, want unpersisted request hidden", pending)
		}
		_, err = bridge.Answer(
			testutil.Context(t),
			scope,
			"clarify-request",
			toolspkg.ClarifyAnswerRequest{Text: "yes"},
		)
		if !errors.Is(err, toolspkg.ErrClarifyNotFound) {
			t.Fatalf("Answer() error = %v, want %v", err, toolspkg.ErrClarifyNotFound)
		}

		publisher.releasePendingPublish()
		pendingEvent := publisher.delegate.await(t)
		if pendingEvent.Status != toolspkg.ClarifyStatusPending {
			t.Fatalf("pending event status = %q, want %q", pendingEvent.Status, toolspkg.ClarifyStatusPending)
		}
		awaitPendingClarification(t, bridge, scope)
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should keep explicit resolution pending and retryable after persistence failure", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Name?"})
		publisher.await(t)
		pending := awaitPendingClarification(t, bridge, scope)
		publisher.failNext(toolspkg.ClarifyStatusResolved, errors.New("write resolved"))
		_, err := bridge.Answer(
			testutil.Context(t), scope, pending[0].RequestID, toolspkg.ClarifyAnswerRequest{Text: "Ada"},
		)
		if err == nil || !strings.Contains(err.Error(), "write resolved") {
			t.Fatalf("Answer(first) error = %v, want persistence failure", err)
		}
		stillPending, pendingErr := bridge.Pending(testutil.Context(t), scope)
		if pendingErr != nil || len(stillPending) != 1 {
			t.Fatalf("Pending(after failure) = %#v, %v, want retryable request", stillPending, pendingErr)
		}
		answer, err := bridge.Answer(
			testutil.Context(t), scope, pending[0].RequestID, toolspkg.ClarifyAnswerRequest{Text: " Ada "},
		)
		if err != nil {
			t.Fatalf("Answer(retry) error = %v", err)
		}
		if answer.Text != "Ada" {
			t.Fatalf("Answer(retry).Text = %q, want Ada", answer.Text)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "Ada" {
			t.Fatalf("Ask() result = %#v, want Ada", got)
		}
	})

	t.Run("Should hide and reject a foreign workspace request", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		foreign := scope
		foreign.WorkspaceID = "workspace-foreign"
		pending, err := bridge.Pending(testutil.Context(t), foreign)
		if err != nil {
			t.Fatalf("Pending(foreign) error = %v", err)
		}
		if len(pending) != 0 {
			t.Fatalf("Pending(foreign) = %#v, want empty", pending)
		}
		_, err = bridge.Answer(
			testutil.Context(t),
			foreign,
			"clarify-request",
			toolspkg.ClarifyAnswerRequest{Text: "yes"},
		)
		if !errors.Is(err, toolspkg.ErrClarifyNotFound) {
			t.Fatalf("Answer(foreign) error = %v, want %v", err, toolspkg.ErrClarifyNotFound)
		}
		bridge.CancelSession(scope.SessionID)
		awaitClarifyResult(t, result)
	})

	t.Run("Should cancel and join callers before closing", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		if err := bridge.Close(testutil.Context(t)); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
		_, err := bridge.Ask(testutil.Context(t), scope, toolspkg.ClarifyQuestion{Question: "Again?"})
		if !errors.Is(err, toolspkg.ErrClarifyClosed) {
			t.Fatalf("Ask(after Close) error = %v, want %v", err, toolspkg.ErrClarifyClosed)
		}
	})

	t.Run("Should honor the shutdown deadline while publishing cancellation", func(t *testing.T) {
		t.Parallel()

		publisher := &cancelBlockingClarifyPublisher{delegate: newClarifyPublisherStub()}
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.delegate.await(t)
		ctx, cancel := context.WithTimeout(testutil.Context(t), 25*time.Millisecond)
		defer cancel()
		startedAt := time.Now()
		err := bridge.Close(ctx)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Close() error = %v, want %v", err, context.DeadlineExceeded)
		}
		if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
			t.Fatalf("Close() elapsed = %s, want caller deadline respected", elapsed)
		}
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})
}

func newTestClarifyBridge(
	t *testing.T,
	timeout time.Duration,
	publisher clarifyEventPublisher,
) *clarifyBridge {
	t.Helper()
	bridge, err := newClarifyBridge(
		timeout,
		publisher,
		&clarifySummaryStub{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		withClarifyIDGenerator(func() string { return "clarify-request" }),
	)
	if err != nil {
		t.Fatalf("newClarifyBridge() error = %v", err)
	}
	return bridge
}

type blockingClarifyPublisher struct {
	delegate *clarifyPublisherStub
	started  chan struct{}
	release  chan struct{}
	once     sync.Once
}

type cancelBlockingClarifyPublisher struct {
	delegate *clarifyPublisherStub
}

func (p *cancelBlockingClarifyPublisher) PublishClarifyEvent(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
) error {
	if event.Status == toolspkg.ClarifyStatusCanceled {
		<-ctx.Done()
		return ctx.Err()
	}
	return p.delegate.PublishClarifyEvent(ctx, event)
}

func newBlockingClarifyPublisher() *blockingClarifyPublisher {
	return &blockingClarifyPublisher{
		delegate: newClarifyPublisherStub(),
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
}

func (p *blockingClarifyPublisher) PublishClarifyEvent(
	ctx context.Context,
	event toolspkg.ClarifyEvent,
) error {
	if event.Status == toolspkg.ClarifyStatusPending {
		p.once.Do(func() { close(p.started) })
		select {
		case <-p.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return p.delegate.PublishClarifyEvent(ctx, event)
}

func (p *blockingClarifyPublisher) awaitPendingPublish(t *testing.T) {
	t.Helper()
	select {
	case <-p.started:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending clarification publication")
	}
}

func (p *blockingClarifyPublisher) releasePendingPublish() {
	close(p.release)
}

func testClarifyScope() toolspkg.Scope {
	return toolspkg.Scope{
		WorkspaceID: "workspace-one",
		SessionID:   "session-one",
		AgentName:   "coder",
	}
}

func askClarification(
	t *testing.T,
	bridge *clarifyBridge,
	scope toolspkg.Scope,
	question toolspkg.ClarifyQuestion,
) <-chan clarifyResult {
	t.Helper()
	result := make(chan clarifyResult, 1)
	go func() {
		answer, err := bridge.Ask(testutil.Context(t), scope, question)
		result <- clarifyResult{answer: answer, err: err}
	}()
	return result
}

func awaitClarifyResult(t *testing.T, result <-chan clarifyResult) clarifyResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clarification result")
		return clarifyResult{}
	}
}

func awaitPendingClarification(
	t *testing.T,
	bridge *clarifyBridge,
	scope toolspkg.Scope,
) []toolspkg.ClarifyPending {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		pending, err := bridge.Pending(testutil.Context(t), scope)
		if err != nil {
			t.Fatalf("Pending() error = %v", err)
		}
		if len(pending) == 1 {
			return pending
		}
		select {
		case <-deadline:
			t.Fatalf("Pending() = %#v, want one persisted request", pending)
		case <-time.After(time.Millisecond):
		}
	}
}

type clarifyPublisherStub struct {
	mu       sync.Mutex
	events   []toolspkg.ClarifyEvent
	failures map[toolspkg.ClarifyStatus]error
	wake     chan toolspkg.ClarifyEvent
}

func newClarifyPublisherStub() *clarifyPublisherStub {
	return &clarifyPublisherStub{
		failures: make(map[toolspkg.ClarifyStatus]error),
		wake:     make(chan toolspkg.ClarifyEvent, 8),
	}
}

func (s *clarifyPublisherStub) PublishClarifyEvent(
	_ context.Context,
	event toolspkg.ClarifyEvent,
) error {
	s.mu.Lock()
	if err := s.failures[event.Status]; err != nil {
		delete(s.failures, event.Status)
		s.mu.Unlock()
		return err
	}
	s.events = append(s.events, event)
	s.mu.Unlock()
	s.wake <- event
	return nil
}

func (s *clarifyPublisherStub) failNext(status toolspkg.ClarifyStatus, err error) {
	s.mu.Lock()
	s.failures[status] = err
	s.mu.Unlock()
}

func (s *clarifyPublisherStub) await(t *testing.T) toolspkg.ClarifyEvent {
	t.Helper()
	select {
	case event := <-s.wake:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clarification event")
		return toolspkg.ClarifyEvent{}
	}
}

func (s *clarifyPublisherStub) statuses() []toolspkg.ClarifyStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	statuses := make([]toolspkg.ClarifyStatus, 0, len(s.events))
	for _, event := range s.events {
		statuses = append(statuses, event.Status)
	}
	return statuses
}

type clarifySummaryStub struct{}

func (*clarifySummaryStub) WriteEventSummary(context.Context, store.EventSummary) error { return nil }

func equalClarifyStatuses(left, right []toolspkg.ClarifyStatus) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
