package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestHostAPIClarifyAskDerivesScopeFromActiveInvocation(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	sess := env.createSession(t)
	wantChoice := 1
	broker := &hostAPIClarifyBrokerStub{
		askFn: func(
			_ context.Context,
			scope toolspkg.Scope,
			question toolspkg.ClarifyQuestion,
		) (toolspkg.ClarifyAnswer, error) {
			if scope.WorkspaceID != env.workspaceID || scope.SessionID != sess.ID || scope.AgentName != "coder" {
				t.Fatalf("clarification scope = %#v, want daemon session ownership", scope)
			}
			wantQuestion := toolspkg.ClarifyQuestion{
				Question: "Which workspace should I use?",
				Choices:  []string{"staging", "production"},
			}
			if !reflect.DeepEqual(question, wantQuestion) {
				t.Fatalf("clarification question = %#v, want %#v", question, wantQuestion)
			}
			return toolspkg.ClarifyAnswer{Choice: &wantChoice}, nil
		},
	}
	handler := NewHostAPIHandler(
		env.sessions,
		nil,
		nil,
		nil,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIClarify(broker),
		WithHostAPIRateLimit(1000, 1000),
	)
	env.grant("question-tool", []string{hostAPIClarifyAskPath}, nil)
	invocationID, finish, err := handler.BeginExtensionToolCall(t.Context(), "question-tool", sess.ID)
	if err != nil {
		t.Fatalf("BeginExtensionToolCall() error = %v", err)
	}

	result, err := callHostAPIClarify(t, handler, "question-tool", extensioncontract.ClarifyAskParams{
		InvocationID: invocationID,
		Question:     " Which workspace should I use? ",
		Choices:      []string{" staging ", " production "},
	})
	if err != nil {
		t.Fatalf("Handle(clarify/ask) error = %v", err)
	}
	answer, ok := result.(toolspkg.ClarifyAnswer)
	if !ok || answer.Choice == nil || *answer.Choice != wantChoice {
		t.Fatalf("Handle(clarify/ask) = %#v, want choice %d", result, wantChoice)
	}

	finish()
	_, err = callHostAPIClarify(t, handler, "question-tool", extensioncontract.ClarifyAskParams{
		InvocationID: invocationID,
		Question:     "Should fail",
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
}

func TestHostAPIClarifyAskRejectsForeignInvocationAndUndeclaredFields(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	sess := env.createSession(t)
	handler := NewHostAPIHandler(
		env.sessions,
		nil,
		nil,
		nil,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIClarify(&hostAPIClarifyBrokerStub{}),
		WithHostAPIRateLimit(1000, 1000),
	)
	env.grant("owner", []string{hostAPIClarifyAskPath}, nil)
	env.grant("foreign", []string{hostAPIClarifyAskPath}, nil)
	invocationID, finish, err := handler.BeginExtensionToolCall(t.Context(), "owner", sess.ID)
	if err != nil {
		t.Fatalf("BeginExtensionToolCall() error = %v", err)
	}
	t.Cleanup(finish)

	_, err = callHostAPIClarify(t, handler, "foreign", extensioncontract.ClarifyAskParams{
		InvocationID: invocationID,
		Question:     "Should fail",
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)

	raw := json.RawMessage(
		`{"invocation_id":"` + invocationID +
			`","question":"Should fail","session_id":"` + sess.ID + `"}`,
	)
	_, err = handler.Handle(t.Context(), "owner", hostAPIClarifyAskPath, raw)
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
}

func TestHostAPIClarifyAskCancelsWithOriginatingToolCall(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	sess := env.createSession(t)
	askStarted := make(chan struct{})
	askCanceled := make(chan struct{})
	handler := NewHostAPIHandler(
		env.sessions,
		nil,
		nil,
		nil,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIClarify(&hostAPIClarifyBrokerStub{askFn: func(
			ctx context.Context,
			_ toolspkg.Scope,
			_ toolspkg.ClarifyQuestion,
		) (toolspkg.ClarifyAnswer, error) {
			close(askStarted)
			<-ctx.Done()
			close(askCanceled)
			return toolspkg.ClarifyAnswer{}, ctx.Err()
		}}),
		WithHostAPIRateLimit(1000, 1000),
	)
	env.grant("question-tool", []string{hostAPIClarifyAskPath}, nil)
	invocationID, finish, err := handler.BeginExtensionToolCall(t.Context(), "question-tool", sess.ID)
	if err != nil {
		t.Fatalf("BeginExtensionToolCall() error = %v", err)
	}
	raw, err := json.Marshal(extensioncontract.ClarifyAskParams{
		InvocationID: invocationID,
		Question:     "Can this outlive the tool call?",
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	type callResult struct {
		value any
		err   error
	}
	result := make(chan callResult, 1)
	go func() {
		value, callErr := handler.Handle(
			context.Background(),
			"question-tool",
			hostAPIClarifyAskPath,
			raw,
		)
		result <- callResult{value: value, err: callErr}
	}()

	select {
	case <-askStarted:
	case <-time.After(time.Second):
		t.Fatal("clarify/ask did not reach the broker")
	}
	finish()
	select {
	case <-askCanceled:
	case <-time.After(time.Second):
		t.Fatal("clarify/ask outlived the originating tool call")
	}
	var got callResult
	select {
	case got = <-result:
	case <-time.After(time.Second):
		t.Fatal("clarify/ask handler did not return after cancellation")
	}
	if got.value != nil {
		t.Fatalf("Handle(clarify/ask) value = %#v, want nil after tool call finish", got.value)
	}
	assertRPCErrorCode(t, got.err, HostAPIUnavailableCode)
}

func TestHostAPIClarifyAskClassifiesInputAndBrokerFailures(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	sess := env.createSession(t)
	env.grant("question-tool", []string{hostAPIClarifyAskPath}, nil)

	t.Run("Should reject invalid questions", func(t *testing.T) {
		t.Parallel()

		handler := NewHostAPIHandler(
			env.sessions,
			nil,
			nil,
			nil,
			WithHostAPICapabilityChecker(env.checker),
			WithHostAPIClarify(&hostAPIClarifyBrokerStub{}),
			WithHostAPIRateLimit(1000, 1000),
		)
		invocationID, finish, err := handler.BeginExtensionToolCall(t.Context(), "question-tool", sess.ID)
		if err != nil {
			t.Fatalf("BeginExtensionToolCall() error = %v", err)
		}
		defer finish()
		_, err = callHostAPIClarify(t, handler, "question-tool", extensioncontract.ClarifyAskParams{
			InvocationID: invocationID,
			Question:     " ",
		})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	})

	t.Run("Should report unknown broker failures as unavailable", func(t *testing.T) {
		t.Parallel()

		handler := NewHostAPIHandler(
			env.sessions,
			nil,
			nil,
			nil,
			WithHostAPICapabilityChecker(env.checker),
			WithHostAPIClarify(&hostAPIClarifyBrokerStub{askFn: func(
				context.Context,
				toolspkg.Scope,
				toolspkg.ClarifyQuestion,
			) (toolspkg.ClarifyAnswer, error) {
				return toolspkg.ClarifyAnswer{}, errors.New("storage unavailable")
			}}),
			WithHostAPIRateLimit(1000, 1000),
		)
		invocationID, finish, err := handler.BeginExtensionToolCall(t.Context(), "question-tool", sess.ID)
		if err != nil {
			t.Fatalf("BeginExtensionToolCall() error = %v", err)
		}
		defer finish()
		_, err = callHostAPIClarify(t, handler, "question-tool", extensioncontract.ClarifyAskParams{
			InvocationID: invocationID,
			Question:     "Continue?",
		})
		assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	})
}

func callHostAPIClarify(
	t testing.TB,
	handler *HostAPIHandler,
	extensionName string,
	params extensioncontract.ClarifyAskParams,
) (any, error) {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return handler.Handle(context.Background(), extensionName, hostAPIClarifyAskPath, raw)
}

type hostAPIClarifyBrokerStub struct {
	askFn func(context.Context, toolspkg.Scope, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error)
}

func (s *hostAPIClarifyBrokerStub) Ask(
	ctx context.Context,
	scope toolspkg.Scope,
	question toolspkg.ClarifyQuestion,
) (toolspkg.ClarifyAnswer, error) {
	if s.askFn == nil {
		return toolspkg.ClarifyAnswer{}, errors.New("unexpected clarification ask")
	}
	return s.askFn(ctx, scope, question)
}

func (*hostAPIClarifyBrokerStub) Pending(context.Context, toolspkg.Scope) ([]toolspkg.ClarifyPending, error) {
	return nil, errors.New("unexpected clarification pending")
}

func (*hostAPIClarifyBrokerStub) Answer(
	context.Context,
	toolspkg.Scope,
	string,
	toolspkg.ClarifyAnswerRequest,
) (toolspkg.ClarifyAnswer, error) {
	return toolspkg.ClarifyAnswer{}, errors.New("unexpected clarification answer")
}
