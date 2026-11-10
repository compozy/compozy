package acp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestElicitationFormAnswersThroughOperator(t *testing.T) {
	t.Parallel()

	choiceSchema := `{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{
			"type":"object",
			"properties":{
				"q0":{
					"type":"string",
					"title":"Approach",
					"description":"Which approach should I take?",
					"oneOf":[{"const":"fast","title":"Fast path"},{"const":"safe","title":"Safe path"}]
				},
				"q0_custom":{
					"type":"string",
					"title":"Approach (other)",
					"description":"Type your own answer"
				}
			},
			"required":["q0"]
		}
	}`
	freeTextSchema := `{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{
			"type":"object",
			"properties":{
				"q0":{"type":"string","title":"Name","description":"What should I call the module?"}
			},
			"required":["q0"]
		}
	}`
	multiSchema := `{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{
			"type":"object",
			"properties":{
				"q0":{"type":"string","oneOf":[{"const":"a","title":"A"},{"const":"b","title":"B"}]},
				"q1":{"type":"string","oneOf":[{"const":"c","title":"C"},{"const":"d","title":"D"}]}
			}
		}
	}`
	headerSchema := `{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Which database should the worker use?",
		"requestedSchema":{
			"type":"object",
			"properties":{
				"question_0":{
					"type":"string",
					"title":"Database",
					"oneOf":[{"const":"Postgres","title":"Postgres"},{"const":"SQLite","title":"SQLite"}]
				},
				"question_0_custom":{"type":"string","title":"Other"}
			}
		}
	}`

	tests := []struct {
		name      string
		schema    string
		answers   []toolspkg.ClarifyAnswer
		wantAsked []toolspkg.ClarifyQuestion
		want      map[string]any
	}{
		{
			name:   "Should map an operator choice to the provider option value",
			schema: choiceSchema,
			answers: []toolspkg.ClarifyAnswer{
				{Choice: new(1)},
			},
			wantAsked: []toolspkg.ClarifyQuestion{{
				Question: "Which approach should I take?",
				Choices:  []string{"Fast path", "Safe path"},
			}},
			want: map[string]any{"q0": "safe"},
		},
		{
			name:   "Should map a typed answer to the custom companion field",
			schema: choiceSchema,
			answers: []toolspkg.ClarifyAnswer{
				{Text: "measure first"},
			},
			wantAsked: []toolspkg.ClarifyQuestion{{
				Question: "Which approach should I take?",
				Choices:  []string{"Fast path", "Safe path"},
			}},
			want: map[string]any{"q0_custom": "measure first"},
		},
		{
			name:   "Should render a plain string property as a free-text question",
			schema: freeTextSchema,
			answers: []toolspkg.ClarifyAnswer{
				{Text: "ledger"},
			},
			wantAsked: []toolspkg.ClarifyQuestion{{
				Question: "What should I call the module?",
			}},
			want: map[string]any{"q0": "ledger"},
		},
		{
			name:   "Should ask each form property in order",
			schema: multiSchema,
			answers: []toolspkg.ClarifyAnswer{
				{Choice: new(0)},
				{Choice: new(1)},
			},
			wantAsked: []toolspkg.ClarifyQuestion{
				{Question: "Questions", Choices: []string{"A", "B"}},
				{Question: "Questions", Choices: []string{"C", "D"}},
			},
			want: map[string]any{"q0": "a", "q1": "d"},
		},
		{
			name:   "Should use the form message when a single field carries no description",
			schema: headerSchema,
			answers: []toolspkg.ClarifyAnswer{
				{Choice: new(0)},
			},
			wantAsked: []toolspkg.ClarifyQuestion{{
				Question: "Which database should the worker use?",
				Choices:  []string{"Postgres", "SQLite"},
			}},
			want: map[string]any{"question_0": "Postgres"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := newElicitationTestProcess(t)
			asked := make([]toolspkg.ClarifyQuestion, 0, len(tt.answers))
			proc.elicitationAsker = func(
				_ context.Context,
				question toolspkg.ClarifyQuestion,
			) (toolspkg.ClarifyAnswer, error) {
				asked = append(asked, question)
				if len(asked) > len(tt.answers) {
					t.Fatalf("unexpected extra question %+v", question)
				}
				return tt.answers[len(asked)-1], nil
			}

			result, reqErr := proc.handleCreateElicitation(t.Context(), json.RawMessage(tt.schema))
			if reqErr != nil {
				t.Fatalf("handleCreateElicitation() error = %+v", reqErr)
			}
			response := result.(acpsdk.UnstableCreateElicitationResponse)
			if response.Accept == nil {
				t.Fatalf("response = %+v, want accept", response)
			}
			if response.Accept.Action != "accept" {
				t.Fatalf("action = %q, want accept", response.Accept.Action)
			}
			if len(asked) != len(tt.wantAsked) {
				t.Fatalf("asked %d questions, want %d", len(asked), len(tt.wantAsked))
			}
			for index, want := range tt.wantAsked {
				if asked[index].Question != want.Question {
					t.Fatalf("question[%d] = %q, want %q", index, asked[index].Question, want.Question)
				}
				if len(asked[index].Choices) != len(want.Choices) {
					t.Fatalf("question[%d] choices = %#v, want %#v",
						index, asked[index].Choices, want.Choices)
				}
				for choiceIndex, choice := range want.Choices {
					if asked[index].Choices[choiceIndex] != choice {
						t.Fatalf("question[%d] choice[%d] = %q, want %q",
							index, choiceIndex, asked[index].Choices[choiceIndex], choice)
					}
				}
			}
			if len(response.Accept.Content) != len(tt.want) {
				t.Fatalf("content = %#v, want %#v", response.Accept.Content, tt.want)
			}
			for key, want := range tt.want {
				if response.Accept.Content[key] != want {
					t.Fatalf("content[%q] = %#v, want %#v", key, response.Accept.Content[key], want)
				}
			}
		})
	}
}

func TestElicitationDeclinesWithoutAnAnswer(t *testing.T) {
	t.Parallel()

	params := json.RawMessage(`{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{"type":"object","properties":{
			"q0":{"type":"string","oneOf":[{"const":"a","title":"A"}]}
		}}
	}`)
	tests := []struct {
		name    string
		timeout time.Duration
		asker   ElicitationAsker
	}{
		{
			name:    "Should decline when no operator channel is wired",
			timeout: time.Second,
			asker:   nil,
		},
		{
			name:    "Should decline when the session type cannot answer",
			timeout: time.Second,
			asker: func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
				return toolspkg.ClarifyAnswer{}, ErrElicitationUnavailable
			},
		},
		{
			name:    "Should decline a timed-out question instead of hanging the turn",
			timeout: 20 * time.Millisecond,
			asker: func(ctx context.Context, _ toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
				<-ctx.Done()
				return toolspkg.ClarifyAnswer{}, ctx.Err()
			},
		},
		{
			name:    "Should decline a broker fallback answer",
			timeout: time.Second,
			asker: func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
				return toolspkg.ClarifyAnswer{Fallback: true}, nil
			},
		},
		{
			name:    "Should decline an asker failure",
			timeout: time.Second,
			asker: func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
				return toolspkg.ClarifyAnswer{}, errors.New("broker exploded")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := newElicitationTestProcess(t)
			proc.permissionTimeout = tt.timeout
			proc.elicitationAsker = tt.asker

			result, reqErr := proc.handleCreateElicitation(t.Context(), params)
			if reqErr != nil {
				t.Fatalf("handleCreateElicitation() error = %+v", reqErr)
			}
			response := result.(acpsdk.UnstableCreateElicitationResponse)
			if response.Decline == nil {
				t.Fatalf("response = %+v, want decline", response)
			}
			if response.Decline.Action != "decline" {
				t.Fatalf("action = %q, want decline", response.Decline.Action)
			}
		})
	}
}

func TestElicitationCancelsWithTheTurn(t *testing.T) {
	t.Parallel()

	params := json.RawMessage(`{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{"type":"object","properties":{
			"q0":{"type":"string","description":"Continue?"}
		}}
	}`)
	proc := newElicitationTestProcess(t)
	started := make(chan struct{})
	proc.elicitationAsker = func(ctx context.Context, _ toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
		close(started)
		<-ctx.Done()
		return toolspkg.ClarifyAnswer{}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		<-started
		cancel()
	}()

	result, reqErr := proc.handleCreateElicitation(ctx, params)
	if reqErr != nil {
		t.Fatalf("handleCreateElicitation() error = %+v", reqErr)
	}
	response := result.(acpsdk.UnstableCreateElicitationResponse)
	if response.Cancel == nil {
		t.Fatalf("response = %+v, want cancel", response)
	}
	if response.Cancel.Action != "cancel" {
		t.Fatalf("action = %q, want cancel", response.Cancel.Action)
	}
}

func TestElicitationRejectsUnrenderableForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params string
	}{
		{
			name: "Should decline URL elicitation",
			params: `{
				"sessionId":"sess-direct",
				"mode":"url",
				"message":"Sign in",
				"url":"https://example.com",
				"elicitationId":"elicit-1"
			}`,
		},
		{
			name: "Should decline a multiselect field",
			params: `{
				"sessionId":"sess-direct",
				"mode":"form",
				"message":"Questions",
				"requestedSchema":{"type":"object","properties":{
					"q0":{"type":"array","items":{"type":"string"}}
				}}
			}`,
		},
		{
			name: "Should decline a choice list above the operator cap",
			params: `{
				"sessionId":"sess-direct",
				"mode":"form",
				"message":"Questions",
				"requestedSchema":{"type":"object","properties":{
					"q0":{"type":"string","oneOf":[
						{"const":"a","title":"A"},{"const":"b","title":"B"},{"const":"c","title":"C"},
						{"const":"d","title":"D"},{"const":"e","title":"E"}
					]}
				}}
			}`,
		},
		{
			name: "Should decline numeric fields",
			params: `{
				"sessionId":"sess-direct",
				"mode":"form",
				"message":"Questions",
				"requestedSchema":{"type":"object","properties":{
					"q0":{"type":"integer"}
				}}
			}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			proc := newElicitationTestProcess(t)
			proc.elicitationAsker = func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
				t.Fatal("an unrenderable form must not reach the operator")
				return toolspkg.ClarifyAnswer{}, nil
			}

			result, reqErr := proc.handleCreateElicitation(t.Context(), json.RawMessage(tt.params))
			if reqErr != nil {
				t.Fatalf("handleCreateElicitation() error = %+v", reqErr)
			}
			response := result.(acpsdk.UnstableCreateElicitationResponse)
			if response.Decline == nil {
				t.Fatalf("response = %+v, want decline", response)
			}
		})
	}
}

func TestElicitationDeclinesForeignSession(t *testing.T) {
	t.Parallel()

	proc := newElicitationTestProcess(t)
	proc.bindSessionRoute("sess-direct")
	proc.elicitationAsker = func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
		t.Fatal("a foreign session must not reach the operator")
		return toolspkg.ClarifyAnswer{}, nil
	}

	result, reqErr := proc.handleCreateElicitation(t.Context(), json.RawMessage(`{
		"sessionId":"sess-foreign",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{"type":"object","properties":{
			"q0":{"type":"string","description":"Continue?"}
		}}
	}`))
	if reqErr != nil {
		t.Fatalf("handleCreateElicitation() error = %+v", reqErr)
	}
	if result.(acpsdk.UnstableCreateElicitationResponse).Decline == nil {
		t.Fatalf("response = %+v, want decline", result)
	}
	if dropped := proc.foreignSessionTrafficDropped(); dropped != 1 {
		t.Fatalf("foreign dropped = %d, want 1", dropped)
	}
}

func TestElicitationDispatchWireEnvelope(t *testing.T) {
	t.Parallel()

	proc := newElicitationTestProcess(t)
	proc.elicitationAsker = func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
		return toolspkg.ClarifyAnswer{Choice: new(0)}, nil
	}

	result, reqErr := proc.handleInbound(t.Context(), acpsdk.ClientMethodElicitationCreate, json.RawMessage(`{
		"sessionId":"sess-direct",
		"mode":"form",
		"message":"Questions",
		"requestedSchema":{"type":"object","properties":{
			"q0":{"type":"string","oneOf":[{"const":"keep","title":"Keep"},{"const":"drop","title":"Drop"}]}
		}}
	}`))
	if reqErr != nil {
		t.Fatalf("handleInbound() error = %+v", reqErr)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var envelope struct {
		Action  string         `json:"action"`
		Content map[string]any `json:"content"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode response %s: %v", payload, err)
	}
	if envelope.Action != "accept" {
		t.Fatalf("action = %q, want accept", envelope.Action)
	}
	if envelope.Content["q0"] != "keep" {
		t.Fatalf("content = %#v, want q0=keep", envelope.Content)
	}
}

func TestElicitationRejectsMalformedParams(t *testing.T) {
	t.Parallel()

	proc := newElicitationTestProcess(t)
	_, reqErr := proc.handleInbound(
		t.Context(),
		acpsdk.ClientMethodElicitationCreate,
		json.RawMessage(`{"mode":`),
	)
	if reqErr == nil || reqErr.Code != -32602 {
		t.Fatalf("request error = %+v, want invalid params", reqErr)
	}
}

func TestStartElicitationAskerReadsSessionPolicy(t *testing.T) {
	t.Parallel()

	if asker := startElicitationAsker(StartOpts{}); asker != nil {
		t.Fatal("startElicitationAsker() without a policy = non-nil, want nil")
	}
	asker := ElicitationAsker(func(context.Context, toolspkg.ClarifyQuestion) (toolspkg.ClarifyAnswer, error) {
		return toolspkg.ClarifyAnswer{}, nil
	})
	got := startElicitationAsker(StartOpts{
		PermissionPolicy: &StartPermissionPolicy{ElicitationAsker: asker},
	})
	if got == nil {
		t.Fatal("startElicitationAsker() dropped the session asker")
	}
}

func newElicitationTestProcess(t *testing.T) *AgentProcess {
	t.Helper()
	proc := newDirectProcess(t, compozyconfig.PermissionModeApproveReads)
	proc.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return proc
}
