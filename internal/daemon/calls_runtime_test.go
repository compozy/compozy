package daemon

import (
	"context"
	"testing"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: activation recovery reuses the deterministic child identity and
// resumes a stopped child before replaying its idempotent initial prompt.
func TestDaemonCallSessionInvokerRecovery(t *testing.T) {
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
	if manager.sentSessionID != childID || manager.sent.Message != "Review the patch." ||
		manager.sent.MessageID != "msg_"+callID || manager.sent.IdempotencyKey != "call:"+callID {
		t.Fatalf("SendPrompt() = %q %#v, want deterministic recovery delivery", manager.sentSessionID, manager.sent)
	}
}

type callSessionManagerStub struct {
	info          *session.Info
	resumeCalls   int
	sentSessionID string
	sent          session.SendPromptOpts
}

func (s *callSessionManagerStub) Status(context.Context, string) (*session.Info, error) {
	return s.info, nil
}

func (s *callSessionManagerStub) Resume(context.Context, string) (*session.Session, error) {
	s.resumeCalls++
	return &session.Session{ID: s.info.ID}, nil
}

func (s *callSessionManagerStub) SendPrompt(
	_ context.Context,
	sessionID string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	s.sentSessionID = sessionID
	s.sent = opts
	return session.SendPromptResult{Status: "accepted"}, nil
}

func (s *callSessionManagerStub) StopWithCause(context.Context, string, session.StopCause, string) error {
	return nil
}
