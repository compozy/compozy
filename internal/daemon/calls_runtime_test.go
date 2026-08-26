package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: activation recovery reuses the deterministic child identity and
// resumes a stopped child before replaying its idempotent initial prompt.
func TestDaemonCallSessionInvokerRecovery(t *testing.T) {
	t.Parallel()
	t.Run("Should resume the deterministic child before replaying the initial prompt", func(t *testing.T) {
		t.Parallel()

		const (
			callID   = "call_recovery"
			parentID = "ses_parent"
			childID  = "ses_call_recovery"
		)
		manager := &callSessionManagerStub{
			info: &session.Info{
				ID: childID, AgentName: "reviewer", State: session.StateStopped,
				Lineage: &store.SessionLineage{ParentSessionID: parentID},
			},
		}
		invoker := &daemonCallSessionInvoker{sessions: manager}

		ref, err := invoker.SpawnChild(context.Background(), callspkg.ChildSpec{
			CallID: callID, ParentSessionID: parentID, AgentName: "reviewer", Prompt: "Review the patch.",
		})
		if err != nil {
			t.Fatalf("SpawnChild() error = %v", err)
		}
		if ref.ID != childID {
			t.Fatalf("SpawnChild() id = %q, want %q", ref.ID, childID)
		}
		if manager.resumeCalls != 1 {
			t.Fatalf("Resume() calls = %d, want 1", manager.resumeCalls)
		}
		wantPrompt := callspkg.CallPromptWithRemainingDepth("Review the patch.", 0)
		if manager.sentSessionID != childID || manager.sent.Message != wantPrompt ||
			manager.sent.MessageID != "msg_"+callID || manager.sent.IdempotencyKey != "call:"+callID {
			t.Fatalf("SendPrompt() = %q %#v, want deterministic recovery delivery", manager.sentSessionID, manager.sent)
		}
		if manager.sent.Synthetic == nil || manager.sent.Synthetic.CallID != callID ||
			manager.sent.Synthetic.CallState != string(callspkg.StateRunning) ||
			manager.sent.Synthetic.ChildSessionID != childID ||
			manager.sent.Synthetic.ChildAgentName != "reviewer" ||
			manager.sent.Synthetic.Reason != "call_request" {
			t.Fatalf("SendPrompt() synthetic metadata = %#v, want call request identity", manager.sent.Synthetic)
		}
	})
}

func TestDaemonCallSessionInvokerReviveMetadata(t *testing.T) {
	t.Run("Should identify a revived call follow-up as daemon-authored input", func(t *testing.T) {
		t.Parallel()

		manager := &callSessionManagerStub{info: &session.Info{ID: "ses-child", State: session.StateStopped}}
		invoker := &daemonCallSessionInvoker{sessions: manager}
		if err := invoker.Revive(context.Background(), "ses-child", "Follow up.", "call-2"); err != nil {
			t.Fatalf("Revive() error = %v", err)
		}
		if manager.sent.Synthetic == nil || manager.sent.Synthetic.CallID != "call-2" ||
			manager.sent.Synthetic.CallState != string(callspkg.StateRunning) ||
			manager.sent.Synthetic.ChildSessionID != "ses-child" ||
			manager.sent.Synthetic.Reason != "call_follow_up" {
			t.Fatalf("SendPrompt() synthetic metadata = %#v, want call follow-up identity", manager.sent.Synthetic)
		}
	})

	t.Run("Should preserve the session identity and resume failure cause", func(t *testing.T) {
		t.Parallel()

		resumeErr := errors.New("resume transport failed")
		manager := &callSessionManagerStub{
			info:      &session.Info{ID: "ses-child", State: session.StateStopped},
			resumeErr: resumeErr,
		}
		invoker := &daemonCallSessionInvoker{sessions: manager}
		err := invoker.Revive(context.Background(), "ses-child", "Follow up.", "call-2")
		if !errors.Is(err, resumeErr) {
			t.Fatalf("Revive() error = %v, want resume cause %v", err, resumeErr)
		}
		if !strings.Contains(err.Error(), "ses-child") {
			t.Fatalf("Revive() error = %q, want session identity", err)
		}
	})
}

// Invariant: call target lookup failures retain their typed storage cause.
func TestDaemonCallDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the target lookup failure cause", func(t *testing.T) {
		t.Parallel()

		lookupErr := errors.New("target lookup failed")
		directory := &daemonCallDirectory{
			store: callDirectoryStoreStub{resolveErr: lookupErr},
			state: &bootState{},
		}
		_, _, err := directory.ResolveCallTarget(context.Background(), callspkg.CreateInput{})
		if !errors.Is(err, lookupErr) {
			t.Fatalf("ResolveCallTarget() error = %v, want lookup cause %v", err, lookupErr)
		}
		if !strings.Contains(err.Error(), "resolve call target context") {
			t.Fatalf("ResolveCallTarget() error = %q, want operation context", err)
		}
	})
}

// Invariant: an accepted queue entry is not projected as delivered until its durable state is sent.
func TestDaemonCallDeliveryTracksDurableQueueState(t *testing.T) {
	t.Parallel()
	t.Run("Should surface operator completion as attention without attaching a model runtime", func(t *testing.T) {
		t.Parallel()

		manager := &callSessionManagerStub{
			info: &session.Info{ID: "ses_operator", State: session.StateStopped},
		}
		invoker := &daemonCallSessionInvoker{
			sessions: manager,
			isOperatorCallerSession: func(_ context.Context, sessionID string) (bool, error) {
				return sessionID == "ses_operator", nil
			},
		}
		outcome, err := invoker.DeliverAtBoundary(context.Background(), callspkg.Delivery{
			CallID: "call_operator", RecipientSessionID: "ses_operator", Kind: callspkg.DeliveryKindCompletion,
		})
		if err != nil {
			t.Fatalf("DeliverAtBoundary(operator) error = %v", err)
		}
		if outcome.State != callspkg.DeliveryStateInjected || outcome.Reason != "operator_attention" {
			t.Fatalf("DeliverAtBoundary(operator) = %#v, want injected operator attention", outcome)
		}
		if manager.statusCalls != 0 || manager.resumeCalls != 0 || manager.sendCalls != 0 {
			t.Fatalf(
				"operator delivery runtime calls = status %d resume %d send %d, want none",
				manager.statusCalls,
				manager.resumeCalls,
				manager.sendCalls,
			)
		}
	})

	t.Run("Should project delivery from the durable queue state", func(t *testing.T) {
		t.Parallel()

		manager := &callSessionManagerStub{
			info: &session.Info{ID: "ses_parent", State: session.StateActive},
			sendResult: session.SendPromptResult{
				Status: "queued", Delivery: store.SessionInputDeliveryAfterTurn, QueueEntryID: "queue_wake",
			},
			queuedStatus: session.InputDeliveryStatus{Status: store.SessionInputQueueStatusQueued},
		}
		invoker := &daemonCallSessionInvoker{sessions: manager}
		delivery := callspkg.Delivery{
			CallID: "call_1", RecipientSessionID: "ses_parent", Body: "wake", Kind: "completion",
			WakeEventID: "wake_1", Metadata: acp.PromptSyntheticMeta{
				CallID: "call_1", CallState: "completed", ResultRef: "sha256:result",
				DeliveryKind: "completion", Reason: "call_completion", WakeEventID: "wake_1",
			},
		}

		queued, err := invoker.DeliverAtBoundary(context.Background(), delivery)
		if err != nil {
			t.Fatalf("DeliverAtBoundary(queued) error = %v", err)
		}
		if queued.State != "pending" {
			t.Fatalf("DeliverAtBoundary(queued) state = %q, want pending", queued.State)
		}
		if manager.sent.Synthetic == nil {
			t.Fatal("SendPrompt() synthetic metadata = nil")
		}
		if manager.sent.Synthetic.CallID != "call_1" {
			t.Fatalf("SendPrompt() synthetic call id = %q, want call_1", manager.sent.Synthetic.CallID)
		}
		if manager.sent.Synthetic.WakeEventID != "wake_1" {
			t.Fatalf("SendPrompt() synthetic metadata = %#v, want durable call identity", manager.sent.Synthetic)
		}
		manager.queuedStatus = session.InputDeliveryStatus{Status: store.SessionInputQueueStatusSent}
		delivered, err := invoker.DeliverAtBoundary(context.Background(), delivery)
		if err != nil {
			t.Fatalf("DeliverAtBoundary(sent) error = %v", err)
		}
		if delivered.State != "injected" {
			t.Fatalf("DeliverAtBoundary(sent) state = %q, want injected", delivered.State)
		}

		sentBeforeBusy := manager.sendCalls
		manager.prompting = true
		busy, err := invoker.DeliverAtBoundary(context.Background(), delivery)
		if err != nil {
			t.Fatalf("DeliverAtBoundary(busy) error = %v", err)
		}
		if busy.State != "pending" {
			t.Fatalf("DeliverAtBoundary(busy) state = %q, want pending", busy.State)
		}
		if busy.Reason != "recipient_busy" {
			t.Fatalf("DeliverAtBoundary(busy) reason = %q, want recipient_busy", busy.Reason)
		}
		if manager.sendCalls != sentBeforeBusy {
			t.Fatalf("SendPrompt() calls while busy = %d, want %d", manager.sendCalls, sentBeforeBusy)
		}

		wakeManager := &callSessionManagerStub{
			info: &session.Info{ID: "ses_parked", State: session.StateStopped},
		}
		wakeInvoker := &daemonCallSessionInvoker{sessions: wakeManager}
		woken, err := wakeInvoker.DeliverAtBoundary(context.Background(), callspkg.Delivery{
			CallID: "call_2", RecipientSessionID: "ses_parked", Body: "wake", Kind: "message",
			WakeEventID: "wake_2",
		})
		if err != nil {
			t.Fatalf("DeliverAtBoundary(parked) error = %v", err)
		}
		if woken.State != "woken" {
			t.Fatalf("DeliverAtBoundary(parked) state = %q, want woken", woken.State)
		}
		if wakeManager.resumeCalls != 1 {
			t.Fatalf("Resume() calls = %d, want 1", wakeManager.resumeCalls)
		}
	})
}

type callSessionManagerStub struct {
	info          *session.Info
	statusCalls   int
	resumeErr     error
	resumeCalls   int
	sentSessionID string
	sent          session.SendPromptOpts
	sendResult    session.SendPromptResult
	queuedStatus  session.InputDeliveryStatus
	prompting     bool
	sendCalls     int
}

func (s *callSessionManagerStub) Status(context.Context, string) (*session.Info, error) {
	s.statusCalls++
	return s.info, nil
}

func (s *callSessionManagerStub) IsPrompting(string) bool { return s.prompting }

func (s *callSessionManagerStub) Resume(context.Context, string) (*session.Session, error) {
	s.resumeCalls++
	if s.resumeErr != nil {
		return nil, s.resumeErr
	}
	return &session.Session{ID: s.info.ID}, nil
}

func (s *callSessionManagerStub) SendPrompt(
	_ context.Context,
	sessionID string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	s.sendCalls++
	s.sentSessionID = sessionID
	s.sent = opts
	if s.sendResult.Status != "" {
		return s.sendResult, nil
	}
	return session.SendPromptResult{Status: "accepted"}, nil
}

func (s *callSessionManagerStub) QueuedInputDeliveryStatus(
	context.Context,
	string,
	string,
) (session.InputDeliveryStatus, error) {
	return s.queuedStatus, nil
}

func (s *callSessionManagerStub) StopWithCause(context.Context, string, session.StopCause, string) error {
	return nil
}

type callDirectoryStoreStub struct {
	callspkg.Store
	resolveErr error
}

func (s callDirectoryStoreStub) ResolveCallTargetContext(
	context.Context,
	callspkg.CreateInput,
) (callspkg.TargetContext, error) {
	return callspkg.TargetContext{}, s.resolveErr
}
