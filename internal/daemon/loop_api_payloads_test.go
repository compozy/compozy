package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	looppkg "github.com/compozy/compozy/internal/loop"
)

func TestLoopRunPayloadShouldPropagateMapCloneErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should reject non-serializable loop run inputs", func(t *testing.T) {
		t.Parallel()

		_, err := loopRunPayload(looppkg.Run{
			ID:     "run-1",
			Inputs: map[string]any{"bad": func() {}},
		})
		if err == nil {
			t.Fatal("loopRunPayload() error = nil, want clone error")
		}
		if !strings.Contains(err.Error(), "clone loop api map") {
			t.Fatalf("loopRunPayload() error = %v, want clone loop api map", err)
		}
	})
}

func TestLoopGenerationOutputsPayloadShouldProjectAppliedRuntime(t *testing.T) {
	t.Parallel()

	resolved := looppkg.ResolvedRuntime{
		Runtime: looppkg.RuntimeSpec{Provider: "mock", Model: "node-model", Reasoning: "high"},
		Source: looppkg.RuntimeProvenance{
			Provider: looppkg.RuntimeSourceAgent,
			Model:    looppkg.RuntimeSourceNode, Reasoning: looppkg.RuntimeSourceDefault,
		},
	}
	payload := loopGenerationOutputsPayload([]looppkg.GenerationOutput{{
		Generation: 1, NodeID: "work", ItemIndex: 2, Status: "succeeded", ResolvedRuntime: &resolved,
	}})
	if len(payload) != 1 || payload[0].ResolvedRuntime == nil {
		t.Fatalf("loopGenerationOutputsPayload() = %#v, want one resolved runtime", payload)
	}
	runtime := payload[0].ResolvedRuntime
	if runtime.Provider != "mock" || runtime.Model != "node-model" || runtime.Reasoning != "high" ||
		runtime.Source.Provider != "agent" || runtime.Source.Model != "node" ||
		runtime.Source.Reasoning != "default" {
		t.Fatalf("resolved runtime payload = %#v, want applied runtime and provenance", runtime)
	}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		t.Fatalf("json.Marshal(runtime) error = %v", err)
	}
	want := `{"provider":"mock","model":"node-model","reasoning":"high",` +
		`"source":{"provider":"agent","model":"node","reasoning":"default"}}`
	if string(encoded) != want {
		t.Fatalf("resolved runtime JSON = %s, want %s", encoded, want)
	}
}
