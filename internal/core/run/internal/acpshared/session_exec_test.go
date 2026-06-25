package acpshared

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestSessionTurnControllerPauseAndMessageResumeSameACPSession(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initialSession := newControlledPromptSession("sess-1")
	client := &pauseResumeClient{}
	pausedCh := make(chan struct{})
	var pausedOnce sync.Once
	submitter := &stubRuntimeEventSubmitter{
		submitFn: func(ev eventspkg.Event) error {
			if ev.Kind == eventspkg.EventKindJobPaused {
				pausedOnce.Do(func() { close(pausedCh) })
			}
			return nil
		},
	}
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    ctx,
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  initialSession.ID(),
		RunID:      "run-1",
		RunJournal: submitter,
	})
	execution := &SessionExecution{
		Session: initialSession,
		Client:  client,
		Handler: handler,
		Logger:  slog.New(slog.NewTextHandler(testingWriter{t: t}, nil)),
	}
	job := &job{SafeName: "task_01"}
	controller := newSessionTurnController(
		ctx,
		&config{RunArtifacts: model.RunArtifacts{RunID: "run-1"}},
		0,
		execution,
		job,
		0,
		false,
	)

	runDone := make(chan JobAttemptResult, 1)
	go func() {
		runDone <- controller.run()
	}()

	pauseResp, err := controller.Pause(ctx, model.JobControlRequest{})
	if err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if pauseResp.Status != model.JobControlStatusPausing || pauseResp.SessionID != "sess-1" {
		t.Fatalf("Pause() = %#v, want pausing in sess-1", pauseResp)
	}
	if got := client.cancelledSessions(); !slices.Equal(got, []string{"sess-1"}) {
		t.Fatalf("CancelSession calls = %#v, want [sess-1]", got)
	}

	initialSession.finish(&agent.PromptCancelledError{SessionID: "sess-1"})
	select {
	case <-pausedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job.paused")
	}

	message := "please adjust the implementation"
	sendDone := make(chan model.JobControlResponse, 1)
	sendErr := make(chan error, 1)
	go func() {
		resp, sendErrValue := controller.SendMessage(ctx, model.JobControlRequest{Message: message})
		if sendErrValue != nil {
			sendErr <- sendErrValue
			return
		}
		sendDone <- resp
	}()

	var sendResp model.JobControlResponse
	select {
	case err := <-sendErr:
		t.Fatalf("SendMessage() error = %v", err)
	case sendResp = <-sendDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SendMessage()")
	}
	if sendResp.Status != model.JobControlStatusResumed || sendResp.SessionID != "sess-1" ||
		stringsTrim(sendResp.MessageID) == "" {
		t.Fatalf("SendMessage() = %#v, want resumed in same session with message id", sendResp)
	}
	req := client.lastPromptRequest()
	if req.SessionID != "sess-1" || string(req.Prompt) != message || req.MessageID != sendResp.MessageID {
		t.Fatalf("PromptSession request = %#v, want same session/message", req)
	}
	resumedSession := client.resumedSession()
	if resumedSession == nil {
		t.Fatal("expected PromptSession to install a resumed session")
	}
	resumedSession.finish(nil)

	select {
	case result := <-runDone:
		if result.Status != attemptStatusSuccess {
			t.Fatalf("controller run result = %#v, want success", result)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller completion")
	}

	assertPauseResumeRuntimeEvents(t, submitter.snapshot(), sendResp.MessageID, message)
}

func TestSessionTurnControllerRejectsNilJobControlContext(t *testing.T) {
	t.Parallel()

	t.Run("Should reject nil context for pause and message controls", func(t *testing.T) {
		t.Parallel()

		controller := &sessionTurnController{}
		var nilCtx context.Context
		if _, err := controller.Pause(
			nilCtx,
			model.JobControlRequest{},
		); !errors.Is(
			err,
			errJobControlContextRequired,
		) {
			t.Fatalf("Pause(nil) error = %v, want errJobControlContextRequired", err)
		}
		if _, err := controller.SendMessage(nilCtx, model.JobControlRequest{Message: "continue"}); !errors.Is(
			err,
			errJobControlContextRequired,
		) {
			t.Fatalf("SendMessage(nil) error = %v, want errJobControlContextRequired", err)
		}
	})
}

func TestSessionTurnControllerUnexpectedPromptCancelFailsJob(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initialSession := newControlledPromptSession("sess-1")
	controller := newSessionTurnController(
		ctx,
		&config{RunArtifacts: model.RunArtifacts{RunID: "run-1"}},
		0,
		&SessionExecution{
			Session: initialSession,
			Client:  &pauseResumeClient{},
			Handler: newSessionUpdateHandler(SessionUpdateHandlerConfig{
				Context:   ctx,
				Index:     0,
				AgentID:   model.IDECodex,
				SessionID: initialSession.ID(),
				RunID:     "run-1",
			}),
			Logger: slog.New(slog.NewTextHandler(testingWriter{t: t}, nil)),
		},
		&job{SafeName: "task_01"},
		0,
		false,
	)

	runDone := make(chan JobAttemptResult, 1)
	go func() {
		runDone <- controller.run()
	}()
	initialSession.finish(&agent.PromptCancelledError{SessionID: "sess-1"})

	select {
	case result := <-runDone:
		if result.Status != attemptStatusFailure {
			t.Fatalf("controller run result = %#v, want failed for unexpected prompt cancellation", result)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller completion")
	}
}

func TestResolveACPInitTimeoutScalesAndCapsActivityTimeout(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{name: "disabled", in: 0, want: 0},
		{name: "scales", in: 5 * time.Minute, want: 15 * time.Minute},
		{name: "caps", in: 20 * time.Minute, want: 30 * time.Minute},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveACPInitTimeout(tc.in); got != tc.want {
				t.Fatalf("ResolveACPInitTimeout(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveTimeoutErrorPrefersTypedInitThenActivityTimeouts(t *testing.T) {
	t.Parallel()

	initErr := NewInitTimeoutError(time.Second)
	activityErr := NewActivityTimeoutError(time.Minute)
	wrapped := errors.Join(errors.New("outer"), initErr, activityErr)

	if got := ResolveTimeoutError(time.Hour, wrapped); !errors.Is(got, initErr) || !IsInitTimeout(got) {
		t.Fatalf("expected init timeout to win, got %T %[1]v", got)
	}
	if got := ResolveTimeoutError(time.Hour, activityErr); !errors.Is(got, activityErr) || !IsActivityTimeout(got) {
		t.Fatalf("expected activity timeout, got %T %[1]v", got)
	}
	fallbackErr := errors.New("deadline exceeded elsewhere")
	if got := ResolveTimeoutError(time.Hour, fallbackErr); !errors.Is(got, fallbackErr) {
		t.Fatalf("expected fallback error, got %T %[1]v", got)
	}
	if got := ResolveTimeoutError(time.Hour); !IsActivityTimeout(got) {
		t.Fatalf("expected synthesized activity timeout, got %T %[1]v", got)
	}
}

func TestSessionCancellationHelpersKeepTimeoutsDistinct(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("shutdown requested"))
	if !isSessionCancellation(ctx, ctx.Err()) {
		t.Fatal("expected canceled context to be session cancellation")
	}
	if isSessionCancellation(context.Background(), NewInitTimeoutError(time.Second)) {
		t.Fatal("expected init timeout not to be session cancellation")
	}
	if got := ResolveCancellationError(nil, context.Canceled); !errors.Is(got, context.Canceled) {
		t.Fatalf("expected context cancellation, got %T %[1]v", got)
	}
	if got := ResolveCancellationError(); !errors.Is(got, context.Canceled) {
		t.Fatalf("expected default context cancellation, got %T %[1]v", got)
	}
}

func TestSessionTurnControllerRejectsConcurrentResumeMessages(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	initialSession := newControlledPromptSession("sess-1")
	client := &pauseResumeClient{
		promptStarted: make(chan struct{}),
		promptRelease: make(chan struct{}),
	}
	pausedCh := make(chan struct{})
	var pausedOnce sync.Once
	submitter := &stubRuntimeEventSubmitter{
		submitFn: func(ev eventspkg.Event) error {
			if ev.Kind == eventspkg.EventKindJobPaused {
				pausedOnce.Do(func() { close(pausedCh) })
			}
			return nil
		},
	}
	handler := newSessionUpdateHandler(SessionUpdateHandlerConfig{
		Context:    ctx,
		Index:      0,
		AgentID:    model.IDECodex,
		SessionID:  initialSession.ID(),
		RunID:      "run-1",
		RunJournal: submitter,
	})
	execution := &SessionExecution{
		Session: initialSession,
		Client:  client,
		Handler: handler,
		Logger:  slog.New(slog.NewTextHandler(testingWriter{t: t}, nil)),
	}
	controller := newSessionTurnController(
		ctx,
		&config{RunArtifacts: model.RunArtifacts{RunID: "run-1"}},
		0,
		execution,
		&job{SafeName: "task_01"},
		0,
		false,
	)

	runDone := make(chan JobAttemptResult, 1)
	go func() {
		runDone <- controller.run()
	}()
	if _, err := controller.Pause(ctx, model.JobControlRequest{}); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	initialSession.finish(&agent.PromptCancelledError{SessionID: "sess-1"})
	select {
	case <-pausedCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job.paused")
	}

	firstDone := make(chan error, 1)
	go func() {
		_, err := controller.SendMessage(ctx, model.JobControlRequest{Message: "first"})
		firstDone <- err
	}()
	select {
	case <-client.promptStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first PromptSession call")
	}

	_, err := controller.SendMessage(ctx, model.JobControlRequest{Message: "second"})
	if !errors.Is(err, model.ErrJobControlConflict) {
		t.Fatalf("second SendMessage() error = %v, want ErrJobControlConflict", err)
	}

	close(client.promptRelease)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first SendMessage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first SendMessage()")
	}
	client.resumedSession().finish(nil)
	select {
	case result := <-runDone:
		if result.Status != attemptStatusSuccess {
			t.Fatalf("controller run result = %#v, want success", result)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for controller completion")
	}
}

func assertPauseResumeRuntimeEvents(
	t *testing.T,
	events []eventspkg.Event,
	messageID string,
	message string,
) {
	t.Helper()

	wantKinds := []eventspkg.EventKind{
		eventspkg.EventKindJobPausing,
		eventspkg.EventKindJobPaused,
		eventspkg.EventKindSessionUpdate,
		eventspkg.EventKindJobResumed,
	}
	seen := make([]eventspkg.EventKind, 0, len(events))
	for _, ev := range events {
		if slices.Contains(wantKinds, ev.Kind) {
			seen = append(seen, ev.Kind)
		}
	}
	if !slices.Equal(seen[:min(len(seen), len(wantKinds))], wantKinds) {
		t.Fatalf("pause/resume event prefix = %#v, want %#v in order; all events=%#v", seen, wantKinds, events)
	}

	var userUpdate *kinds.SessionUpdatePayload
	var resumed *kinds.JobResumedPayload
	for _, ev := range events {
		switch ev.Kind {
		case eventspkg.EventKindSessionUpdate:
			var payload kinds.SessionUpdatePayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				t.Fatalf("decode session.update: %v", err)
			}
			if payload.Update.Kind == kinds.UpdateKindUserMessageChunk {
				userUpdate = &payload
			}
		case eventspkg.EventKindJobResumed:
			var payload kinds.JobResumedPayload
			if err := json.Unmarshal(ev.Payload, &payload); err != nil {
				t.Fatalf("decode job.resumed: %v", err)
			}
			resumed = &payload
		}
	}
	if userUpdate == nil {
		t.Fatalf("missing user message session.update in events: %#v", events)
	}
	if userUpdate.Index != 0 || userUpdate.Update.MessageID != messageID ||
		userUpdate.Update.Kind != kinds.UpdateKindUserMessageChunk {
		t.Fatalf("user message update = %#v, want matching index/message id", userUpdate)
	}
	if len(userUpdate.Update.Blocks) != 1 {
		t.Fatalf("user message blocks = %#v, want one text block", userUpdate.Update.Blocks)
	}
	block, err := userUpdate.Update.Blocks[0].AsText()
	if err != nil {
		t.Fatalf("decode user message text block: %v", err)
	}
	if block.Text != message {
		t.Fatalf("user message text = %q, want %q", block.Text, message)
	}
	if resumed == nil || resumed.SessionID != "sess-1" || resumed.MessageID != messageID {
		t.Fatalf("job.resumed payload = %#v, want same session/message id", resumed)
	}
}

type controlledPromptSession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}
	errMu   sync.Mutex
	err     error
	once    sync.Once
}

func newControlledPromptSession(id string) *controlledPromptSession {
	return &controlledPromptSession{
		id:      id,
		updates: make(chan model.SessionUpdate),
		done:    make(chan struct{}),
	}
}

func (s *controlledPromptSession) ID() string { return s.id }

func (s *controlledPromptSession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
}

func (s *controlledPromptSession) Updates() <-chan model.SessionUpdate { return s.updates }
func (s *controlledPromptSession) Done() <-chan struct{}               { return s.done }

func (s *controlledPromptSession) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

func (s *controlledPromptSession) SlowPublishes() uint64  { return 0 }
func (s *controlledPromptSession) DroppedUpdates() uint64 { return 0 }

func (s *controlledPromptSession) finish(err error) {
	s.once.Do(func() {
		s.errMu.Lock()
		s.err = err
		s.errMu.Unlock()
		close(s.updates)
		close(s.done)
	})
}

type pauseResumeClient struct {
	mu            sync.Mutex
	cancelSession []string
	promptReqs    []agent.PromptSessionRequest
	resume        *controlledPromptSession
	promptStarted chan struct{}
	promptRelease chan struct{}
	promptOnce    sync.Once
}

func (c *pauseResumeClient) CreateSession(context.Context, agent.SessionRequest) (agent.Session, error) {
	return nil, nil
}

func (c *pauseResumeClient) ResumeSession(context.Context, agent.ResumeSessionRequest) (agent.Session, error) {
	return nil, nil
}

func (c *pauseResumeClient) CancelSession(_ context.Context, sessionID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelSession = append(c.cancelSession, sessionID)
	return nil
}

func (c *pauseResumeClient) SetSessionModel(context.Context, string, string) error {
	return nil
}

func (c *pauseResumeClient) PromptSession(
	_ context.Context,
	req agent.PromptSessionRequest,
) (agent.Session, error) {
	if c.promptStarted != nil {
		c.promptOnce.Do(func() { close(c.promptStarted) })
	}
	if c.promptRelease != nil {
		<-c.promptRelease
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.promptReqs = append(c.promptReqs, req)
	c.resume = newControlledPromptSession(req.SessionID)
	return c.resume, nil
}

func (c *pauseResumeClient) SupportsLoadSession() bool { return true }
func (c *pauseResumeClient) Close() error              { return nil }
func (c *pauseResumeClient) Kill() error               { return nil }

func (c *pauseResumeClient) cancelledSessions() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.cancelSession...)
}

func (c *pauseResumeClient) lastPromptRequest() agent.PromptSessionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.promptReqs) == 0 {
		return agent.PromptSessionRequest{}
	}
	return c.promptReqs[len(c.promptReqs)-1]
}

func (c *pauseResumeClient) resumedSession() *controlledPromptSession {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resume
}

type testingWriter struct {
	t *testing.T
}

func (w testingWriter) Write(p []byte) (int, error) {
	w.t.Helper()
	w.t.Log(stringsTrim(string(p)))
	return len(p), nil
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
