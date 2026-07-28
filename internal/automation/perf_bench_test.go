package automation

import (
	"context"
	"fmt"
	"testing"
)

type benchmarkNoopTriggerDispatcher struct{}

func (benchmarkNoopTriggerDispatcher) Dispatch(context.Context, DispatchRequest) (*Run, error) {
	return nil, nil
}

func BenchmarkTriggerEngineFireMatchingRegistrations(b *testing.B) {
	b.ReportAllocs()

	engine, err := NewTriggerEngine(benchmarkNoopTriggerDispatcher{})
	if err != nil {
		b.Fatalf("NewTriggerEngine() error: %v", err)
	}

	for i := range 32 {
		registration := TriggerRegistration{
			Trigger: Trigger{
				ID:          fmt.Sprintf("trigger-%02d", i),
				Scope:       AutomationScopeWorkspace,
				Name:        fmt.Sprintf("Trigger %02d", i),
				AgentName:   "codex",
				WorkspaceID: "ws-alpha",
				Prompt:      "Summarize the latest session activity.",
				Event:       "session.stopped",
				Filter: map[string]string{
					"data.kind":          "assistant",
					"data.metadata.step": "complete",
					"source":             string(ActivationSourceObserver),
				},
				Enabled:   true,
				Retry:     DefaultRetryConfig(),
				FireLimit: DefaultFireLimitConfig(),
				Source:    JobSourceDynamic,
			},
		}
		if err := engine.Register(registration); err != nil {
			b.Fatalf("Register() error: %v", err)
		}
	}

	ctx := context.Background()
	envelope := ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"kind": "assistant",
			"metadata": map[string]any{
				"step": "complete",
			},
		},
	}

	for b.Loop() {
		if _, err := engine.Fire(ctx, envelope); err != nil {
			b.Fatalf("Fire() error: %v", err)
		}
	}
}

func BenchmarkExactFilterMatchNestedData(b *testing.B) {
	b.ReportAllocs()

	filter := map[string]string{
		"data.kind":          "assistant",
		"data.metadata.step": "complete",
		"source":             string(ActivationSourceObserver),
		"workspace_id":       "ws-alpha",
	}
	envelope := ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"kind": "assistant",
			"metadata": map[string]any{
				"step": "complete",
			},
		},
	}

	for b.Loop() {
		if !exactFilterMatch(filter, envelope) {
			b.Fatal("exactFilterMatch() = false, want true")
		}
	}
}

func BenchmarkRenderTriggerPromptStatic(b *testing.B) {
	b.ReportAllocs()

	envelope := &ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      ActivationSourceObserver,
		Data:        map[string]any{"kind": "assistant"},
	}

	for b.Loop() {
		rendered, err := renderTriggerPrompt("Summarize the latest session activity.", envelope)
		if err != nil {
			b.Fatalf("renderTriggerPrompt() error: %v", err)
		}
		if rendered == "" {
			b.Fatal("renderTriggerPrompt() returned empty string")
		}
	}
}

func BenchmarkRenderTriggerPromptTemplate(b *testing.B) {
	b.ReportAllocs()

	envelope := &ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws-alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"kind": "assistant",
			"metadata": map[string]any{
				"step": "complete",
			},
		},
	}
	raw := "Kind={{.Data.kind}} Step={{index .Data.metadata \"step\"}} Workspace={{.WorkspaceID}} Source={{.Source}}"

	for b.Loop() {
		rendered, err := renderTriggerPrompt(raw, envelope)
		if err != nil {
			b.Fatalf("renderTriggerPrompt() error: %v", err)
		}
		if rendered == "" {
			b.Fatal("renderTriggerPrompt() returned empty string")
		}
	}
}
