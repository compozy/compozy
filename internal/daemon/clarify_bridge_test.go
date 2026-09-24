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

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestClarifyBridgeLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should delegate a missing live handle to durable orphan resolution", func(t *testing.T) {
		t.Parallel()

		var (
			gotScope     toolspkg.Scope
			gotRequestID string
			gotRequest   toolspkg.ClarifyAnswerRequest
		)
		publisher := &orphanClarifyPublisher{
			clarifyPublisherStub: newClarifyPublisherStub(),
			resolve: func(
				_ context.Context,
				scope toolspkg.Scope,
				requestID string,
				request toolspkg.ClarifyAnswerRequest,
			) (toolspkg.ClarifyAnswerResult, error) {
				gotScope = scope
				gotRequestID = requestID
				gotRequest = request
				return toolspkg.ClarifyAnswerResult{
					ClarifyAnswer:  toolspkg.ClarifyAnswer{Text: "safe"},
					Outcome:        store.PendingInteractionOutcomeResolvedAfterRestart,
					InteractionID:  "interaction-one",
					RequestID:      requestID,
					ResolvedAnswer: "safe",
				}, nil
			},
		}
		bridge := newTestClarifyBridge(t, time.Second, publisher)
		scope := testClarifyScope()
		scope.Operator = true
		result, err := bridge.Answer(
			testutil.Context(t),
			scope,
			"request-orphan",
			toolspkg.ClarifyAnswerRequest{Text: " safe "},
		)
		if err != nil {
			t.Fatalf("Answer(orphan) error = %v", err)
		}
		if gotScope != scope || gotRequestID != "request-orphan" || gotRequest.Text != " safe " {
			t.Fatalf("orphan resolver call = %#v/%q/%#v", gotScope, gotRequestID, gotRequest)
		}
		if result.Outcome != store.PendingInteractionOutcomeResolvedAfterRestart ||
			result.ResolvedAnswer != "safe" {
			t.Fatalf("Answer(orphan) = %#v", result)
		}
	})

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
		if answer.Choice == nil || *answer.Choice != choice || answer.Text != "" || answer.Fallback ||
			answer.Outcome != store.PendingInteractionOutcomeAnswered ||
			answer.RequestID != pending[0].RequestID || answer.ResolvedAnswer != "Safe" {
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

	t.Run("Should make a publication-woken reader wait for the durable request", func(t *testing.T) {
		t.Parallel()

		publisher := newBlockingClarifyPublisher()
		bridge := newTestClarifyBridge(t, 5*time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.awaitPendingPublish(t)

		pendingResult := make(chan []toolspkg.ClarifyPending, 1)
		pendingErr := make(chan error, 1)
		go func() {
			pending, err := bridge.Pending(testutil.Context(t), scope)
			pendingResult <- pending
			pendingErr <- err
		}()
		select {
		case pending := <-pendingResult:
			t.Fatalf("Pending() returned before publication completed: %#v", pending)
		case <-time.After(25 * time.Millisecond):
		}

		publisher.releasePendingPublish()
		pendingEvent := publisher.delegate.await(t)
		if pendingEvent.Status != toolspkg.ClarifyStatusPending {
			t.Fatalf("pending event status = %q, want %q", pendingEvent.Status, toolspkg.ClarifyStatusPending)
		}
		pending := <-pendingResult
		if err := <-pendingErr; err != nil {
			t.Fatalf("Pending() error = %v", err)
		}
		if len(pending) != 1 || pending[0].RequestID != "clarify-request" {
			t.Fatalf("Pending() = %#v, want the durably published request", pending)
		}
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should make a publication-woken answer wait for the durable request", func(t *testing.T) {
		t.Parallel()

		publisher := newBlockingClarifyPublisher()
		bridge := newTestClarifyBridge(t, 5*time.Second, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.awaitPendingPublish(t)

		answerResult := make(chan clarifyAnswerCallResult, 1)
		go func() {
			answer, err := bridge.Answer(
				testutil.Context(t),
				scope,
				"clarify-request",
				toolspkg.ClarifyAnswerRequest{Text: "yes"},
			)
			answerResult <- clarifyAnswerCallResult{answer: answer, err: err}
		}()
		select {
		case answer := <-answerResult:
			t.Fatalf("Answer() returned before publication completed: %#v", answer)
		case <-time.After(25 * time.Millisecond):
		}

		publisher.releasePendingPublish()
		if event := publisher.delegate.await(t); event.Status != toolspkg.ClarifyStatusPending {
			t.Fatalf("pending event status = %q, want %q", event.Status, toolspkg.ClarifyStatusPending)
		}
		if answer := awaitClarifyAnswerCallResult(t, answerResult); answer.err != nil || answer.answer.Text != "yes" {
			t.Fatalf("Answer() result = %#v, want yes", answer)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "yes" {
			t.Fatalf("Ask() result = %#v, want yes", got)
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

type stubClarifyClock struct {
	mu  sync.Mutex
	now time.Time
}

func newStubClarifyClock(now time.Time) *stubClarifyClock {
	return &stubClarifyClock{now: now.UTC()}
}

func (c *stubClarifyClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stubClarifyClock) Advance(delta time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta)
}

func newTestClarifyBridgeWithClock(
	t *testing.T,
	timeout time.Duration,
	publisher clarifyEventPublisher,
	now func() time.Time,
) *clarifyBridge {
	t.Helper()
	bridge, err := newClarifyBridge(
		timeout,
		publisher,
		&clarifySummaryStub{},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		withClarifyIDGenerator(func() string { return "clarify-request" }),
		withClarifyClock(now),
	)
	if err != nil {
		t.Fatalf("newClarifyBridge() error = %v", err)
	}
	return bridge
}

func assertClarifyBlocked(t *testing.T, result <-chan clarifyResult) {
	t.Helper()
	select {
	case got := <-result:
		t.Fatalf("Ask() returned early: answer=%#v err=%v", got.answer, got.err)
	default:
	}
}

func TestClarifyBridgeUnboundedWaits(t *testing.T) {
	t.Parallel()

	t.Run("Should block past the stub-clock 60s and 5m marks with a zero deadline", func(t *testing.T) {
		t.Parallel()

		clock := newStubClarifyClock(time.Now().UTC())
		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridgeWithClock(t, 0, publisher, clock.Now)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Which env first?"})
		publisher.await(t)
		clock.Advance(61 * time.Second)
		assertClarifyBlocked(t, result)
		clock.Advance(5 * time.Minute)
		assertClarifyBlocked(t, result)
		pending := awaitPendingClarification(t, bridge, scope)
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("Pending().Deadline = %s, want zero for unbounded", pending[0].Deadline)
		}
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{Text: "staging"},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		if got := awaitClarifyResult(t, result); got.err != nil || got.answer.Text != "staging" {
			t.Fatalf("Ask() result = %#v, want staging", got)
		}
	})

	t.Run("Should resolve a late answer with the answer and never the fallback sentinel", func(t *testing.T) {
		t.Parallel()

		clock := newStubClarifyClock(time.Now().UTC())
		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridgeWithClock(t, 0, publisher, clock.Now)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{
			Question: "Which env first?",
			Choices:  []string{"staging", "production"},
		})
		publisher.await(t)
		clock.Advance(10 * time.Minute)
		pending := awaitPendingClarification(t, bridge, scope)
		choice := 0
		if _, err := bridge.Answer(
			testutil.Context(t),
			scope,
			pending[0].RequestID,
			toolspkg.ClarifyAnswerRequest{ChoiceIndex: &choice},
		); err != nil {
			t.Fatalf("Answer() error = %v", err)
		}
		got := awaitClarifyResult(t, result)
		if got.err != nil {
			t.Fatalf("Ask() error = %v", got.err)
		}
		if got.answer.Fallback || got.answer.Choice == nil || *got.answer.Choice != choice {
			t.Fatalf("Ask() answer = %#v, want choice %d without fallback", got.answer, choice)
		}
		if statuses := publisher.statuses(); !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusResolved,
		}) {
			t.Fatalf("published statuses = %v, want pending then resolved", statuses)
		}
	})

	t.Run("Should cancel an unbounded wait with ErrClarifyCanceled and the canceled event", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 0, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		} else if got.answer.Fallback {
			t.Fatalf("Ask() answer = %#v, cancellation must not be fallback", got.answer)
		}
		if statuses := publisher.statuses(); !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusCanceled,
		}) {
			t.Fatalf("published statuses = %v, want pending then canceled", statuses)
		}
	})

	t.Run("Should keep a zero deadline for the live wait after a policy change", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 0, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		bridge.timeout = 5 * time.Minute
		pending := awaitPendingClarification(t, bridge, scope)
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("Pending().Deadline = %s, want pinned zero after policy change", pending[0].Deadline)
		}
		assertClarifyBlocked(t, result)
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should reject a second ask while the unbounded wait keeps its deadline", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 0, publisher)
		scope := testClarifyScope()
		first := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "First?"})
		publisher.await(t)
		if _, err := bridge.Ask(
			testutil.Context(t),
			scope,
			toolspkg.ClarifyQuestion{Question: "Second?"},
		); !errors.Is(err, toolspkg.ErrClarifyPending) {
			t.Fatalf("Ask(second) error = %v, want %v", err, toolspkg.ErrClarifyPending)
		}
		pending := awaitPendingClarification(t, bridge, scope)
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("Pending().Deadline = %s, want zero after conflict", pending[0].Deadline)
		}
		assertClarifyBlocked(t, first)
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, first); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask(first) error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should project an unbounded pending wait with a zero deadline", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 0, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		pending := awaitPendingClarification(t, bridge, scope)
		if len(pending) != 1 {
			t.Fatalf("Pending() = %#v, want one request", pending)
		}
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("Pending().Deadline = %s, want zero", pending[0].Deadline)
		}
		if pending[0].AskedAt.IsZero() || pending[0].RequestID == "" {
			t.Fatalf("Pending() = %#v, want identity and ask timestamps", pending[0])
		}
		bridge.CancelSession(scope.SessionID)
		awaitClarifyResult(t, result)
	})

	t.Run("Should treat a negative policy as unbounded with a zero deadline", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, -time.Minute, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)
		pending := awaitPendingClarification(t, bridge, scope)
		if !pending[0].Deadline.IsZero() {
			t.Fatalf("Pending().Deadline = %s, want zero for a negative policy", pending[0].Deadline)
		}
		assertClarifyBlocked(t, result)
		bridge.CancelSession(scope.SessionID)
		if got := awaitClarifyResult(t, result); !errors.Is(got.err, toolspkg.ErrClarifyCanceled) {
			t.Fatalf("Ask() error = %v, want %v", got.err, toolspkg.ErrClarifyCanceled)
		}
	})

	t.Run("Should expire a finite wait with the exact fallback sentinel and timed_out", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 25*time.Millisecond, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		pendingEvent := publisher.await(t)
		if pendingEvent.Request.Deadline.IsZero() {
			t.Fatalf("pending event deadline is zero, want a real finite deadline")
		}
		got := awaitClarifyResult(t, result)
		if got.err != nil {
			t.Fatalf("Ask() error = %v, want nil with fallback", got.err)
		}
		if got.answer.Choice != nil || got.answer.Text != "" || !got.answer.Fallback {
			t.Fatalf("Ask() answer = %#v, want exact fallback sentinel", got.answer)
		}
		if statuses := publisher.statuses(); !equalClarifyStatuses(statuses, []toolspkg.ClarifyStatus{
			toolspkg.ClarifyStatusPending,
			toolspkg.ClarifyStatusTimedOut,
		}) {
			t.Fatalf("published statuses = %v, want pending then timed_out", statuses)
		}
	})
}

func TestClarifyBridgeAnswerExpiryRace(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve concurrent answers racing finite expiry exactly once", func(t *testing.T) {
		t.Parallel()

		publisher := newClarifyPublisherStub()
		bridge := newTestClarifyBridge(t, 20*time.Millisecond, publisher)
		scope := testClarifyScope()
		result := askClarification(t, bridge, scope, toolspkg.ClarifyQuestion{Question: "Continue?"})
		publisher.await(t)

		const callers = 4
		errs := make([]error, callers)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				_, errs[i] = bridge.Answer(
					testutil.Context(t),
					scope,
					"clarify-request",
					toolspkg.ClarifyAnswerRequest{Text: "yes"},
				)
			}(i)
		}
		wg.Wait()

		succeeded := 0
		for _, err := range errs {
			if err == nil {
				succeeded++
			} else if !errors.Is(err, toolspkg.ErrClarifyNotFound) {
				t.Fatalf("Answer() error = %v, want nil or %v", err, toolspkg.ErrClarifyNotFound)
			}
		}
		got := awaitClarifyResult(t, result)
		statuses := publisher.statuses()
		if len(statuses) != 2 || statuses[0] != toolspkg.ClarifyStatusPending {
			t.Fatalf("published statuses = %v, want pending plus one terminal event", statuses)
		}
		switch statuses[1] {
		case toolspkg.ClarifyStatusResolved:
			if succeeded != 1 {
				t.Fatalf("Answer() successes = %d, want exactly one", succeeded)
			}
			if got.err != nil || got.answer.Text != "yes" || got.answer.Fallback {
				t.Fatalf("Ask() result = %#v, want the winning answer", got)
			}
		case toolspkg.ClarifyStatusTimedOut:
			if succeeded != 0 {
				t.Fatalf("Answer() successes = %d, want none after expiry won", succeeded)
			}
			if got.err != nil || got.answer.Choice != nil || got.answer.Text != "" || !got.answer.Fallback {
				t.Fatalf("Ask() answer = %#v, want exact fallback sentinel", got.answer)
			}
		default:
			t.Fatalf("terminal status = %q, want resolved or timed_out", statuses[1])
		}
	})
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

type clarifyAnswerCallResult struct {
	answer toolspkg.ClarifyAnswerResult
	err    error
}

func awaitClarifyAnswerCallResult(
	t *testing.T,
	result <-chan clarifyAnswerCallResult,
) clarifyAnswerCallResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clarification answer result")
		return clarifyAnswerCallResult{}
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

type orphanClarifyPublisher struct {
	*clarifyPublisherStub
	resolve func(
		context.Context,
		toolspkg.Scope,
		string,
		toolspkg.ClarifyAnswerRequest,
	) (toolspkg.ClarifyAnswerResult, error)
}

func (s *orphanClarifyPublisher) ResolveOrphanedClarification(
	ctx context.Context,
	scope toolspkg.Scope,
	requestID string,
	request toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	return s.resolve(ctx, scope, requestID, request)
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
