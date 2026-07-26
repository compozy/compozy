package aghsdk

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestToolRequestAskClarificationCarriesOnlyBoundInvocationAndQuestion(t *testing.T) {
	t.Parallel()

	transport := &clarifyRecordingTransport{
		answer: ClarifyAnswer{Text: "Use staging"},
	}
	request := ToolRequest[struct{}]{
		Host:         NewHostAPI(transport, func() bool { return true }),
		invocationID: "inv-1",
	}
	answer, err := request.AskClarification(t.Context(), ClarifyQuestion{
		Question: "Which workspace should I use?",
		Choices:  []string{"staging", "production"},
	})
	if err != nil {
		t.Fatalf("AskClarification() error = %v", err)
	}
	if answer.Text != "Use staging" || answer.Choice != nil || answer.Fallback {
		t.Fatalf("AskClarification() = %#v, want exact text answer", answer)
	}
	if transport.method != string(HostAPIMethodClarifyAsk) {
		t.Fatalf("host method = %q, want %q", transport.method, HostAPIMethodClarifyAsk)
	}
	want := map[string]any{
		"invocation_id": "inv-1",
		"question":      "Which workspace should I use?",
		"choices":       []any{"staging", "production"},
	}
	if !reflect.DeepEqual(transport.params, want) {
		t.Fatalf("host params = %#v, want %#v", transport.params, want)
	}
}

type clarifyRecordingTransport struct {
	method string
	params map[string]any
	answer ClarifyAnswer
}

func (*clarifyRecordingTransport) Handle(string, TransportHandler) {}

func (t *clarifyRecordingTransport) Call(_ context.Context, method string, params any, result any) error {
	t.method = method
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &t.params); err != nil {
		return err
	}
	encoded, err := json.Marshal(t.answer)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, result)
}

func (*clarifyRecordingTransport) Run(context.Context) error { return nil }
func (*clarifyRecordingTransport) Close() error              { return nil }
