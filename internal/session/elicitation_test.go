package session

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestElicitationSessionIsInteractive(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sessionType Type
		want        bool
	}{
		{sessionType: SessionTypeUser, want: true},
		{sessionType: SessionTypeCoordinator, want: true},
		{sessionType: SessionTypeSystem},
		{sessionType: SessionTypeSpawned},
		{sessionType: SessionTypeTriage},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("Should report interactive=%t for %s", tt.want, tt.sessionType), func(t *testing.T) {
			t.Parallel()
			if got := elicitationSessionIsInteractive(tt.sessionType); got != tt.want {
				t.Fatalf("elicitationSessionIsInteractive(%q) = %t, want %t",
					tt.sessionType, got, tt.want)
			}
		})
	}
}

func TestElicitationAskerRoutesThroughClarifyBroker(t *testing.T) {
	t.Parallel()

	t.Run("Should ask with the session scope and return the operator answer", func(t *testing.T) {
		t.Parallel()

		broker := &recordingClarifyBroker{answer: toolspkg.ClarifyAnswer{Text: "ledger"}}
		manager := &Manager{}
		manager.SetClarifyBroker(broker)
		ask := manager.elicitationAsker("sess-1", "ws-1", "profile-1", "agent-1", SessionTypeUser)

		answer, err := ask(t.Context(), toolspkg.ClarifyQuestion{
			Question: "What should I call the module?",
			Choices:  []string{"ledger", "journal"},
		})
		if err != nil {
			t.Fatalf("ask() error = %v", err)
		}
		if answer.Text != "ledger" {
			t.Fatalf("answer = %+v, want ledger", answer)
		}
		if len(broker.calls) != 1 {
			t.Fatalf("broker calls = %d, want 1", len(broker.calls))
		}
		wantScope := toolspkg.Scope{
			ProfileID:   "profile-1",
			WorkspaceID: "ws-1",
			SessionID:   "sess-1",
			AgentName:   "agent-1",
		}
		if broker.calls[0].scope != wantScope {
			t.Fatalf("scope = %+v, want %+v", broker.calls[0].scope, wantScope)
		}
		if broker.calls[0].question.Question != "What should I call the module?" {
			t.Fatalf("question = %+v", broker.calls[0].question)
		}
	})
}

func TestElicitationAskerRefusesHeadlessSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse system, spawned, and triage sessions without asking", func(t *testing.T) {
		t.Parallel()

		broker := &recordingClarifyBroker{}
		manager := &Manager{}
		manager.SetClarifyBroker(broker)

		for _, sessionType := range []Type{SessionTypeSystem, SessionTypeSpawned, SessionTypeTriage} {
			ask := manager.elicitationAsker("sess-1", "ws-1", "profile-1", "agent-1", sessionType)
			if _, err := ask(t.Context(), toolspkg.ClarifyQuestion{Question: "Continue?"}); !errors.Is(
				err, acp.ErrElicitationUnavailable,
			) {
				t.Fatalf("%s: error = %v, want %v", sessionType, err, acp.ErrElicitationUnavailable)
			}
		}
		if len(broker.calls) != 0 {
			t.Fatalf("broker calls = %d, want a headless session to skip the broker", len(broker.calls))
		}
	})
}

func TestElicitationAskerWithoutBrokerIsUnavailable(t *testing.T) {
	t.Parallel()

	t.Run("Should report unavailable when the broker is not wired", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{}
		ask := manager.elicitationAsker("sess-1", "ws-1", "profile-1", "agent-1", SessionTypeUser)
		if _, err := ask(t.Context(), toolspkg.ClarifyQuestion{Question: "Continue?"}); !errors.Is(
			err, acp.ErrElicitationUnavailable,
		) {
			t.Fatalf("error = %v, want %v", err, acp.ErrElicitationUnavailable)
		}
	})
}

func TestElicitationAskerHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	t.Run("Should return the canceled turn error without asking", func(t *testing.T) {
		t.Parallel()

		broker := &recordingClarifyBroker{}
		manager := &Manager{}
		manager.SetClarifyBroker(broker)
		ask := manager.elicitationAsker("sess-1", "ws-1", "profile-1", "agent-1", SessionTypeUser)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := ask(ctx, toolspkg.ClarifyQuestion{Question: "Continue?"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
		if len(broker.calls) != 0 {
			t.Fatalf("broker calls = %d, want a canceled turn to skip the broker", len(broker.calls))
		}
	})
}

type recordingClarifyBroker struct {
	calls  []recordingClarifyCall
	answer toolspkg.ClarifyAnswer
	err    error
}

type recordingClarifyCall struct {
	scope    toolspkg.Scope
	question toolspkg.ClarifyQuestion
}

func (b *recordingClarifyBroker) Ask(
	_ context.Context,
	scope toolspkg.Scope,
	question toolspkg.ClarifyQuestion,
) (toolspkg.ClarifyAnswer, error) {
	b.calls = append(b.calls, recordingClarifyCall{scope: scope, question: question})
	return b.answer, b.err
}

func (b *recordingClarifyBroker) Pending(context.Context, toolspkg.Scope) ([]toolspkg.ClarifyPending, error) {
	return nil, nil
}

func (b *recordingClarifyBroker) Answer(
	context.Context,
	toolspkg.Scope,
	string,
	toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswerResult, error) {
	return toolspkg.ClarifyAnswerResult{}, nil
}
