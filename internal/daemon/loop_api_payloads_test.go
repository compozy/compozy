package daemon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	speedpkg "github.com/compozy/compozy/internal/speed"
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

	t.Run("Should project applied runtime and lifecycle evidence", func(t *testing.T) {
		t.Parallel()

		resolved := looppkg.ResolvedRuntime{
			Runtime: looppkg.RuntimeSpec{
				Provider: "mock", Model: "node-model", Reasoning: "high", Speed: speedpkg.SpeedFast,
			},
			Source: looppkg.RuntimeProvenance{
				Provider: looppkg.RuntimeSourceAgent,
				Model:    looppkg.RuntimeSourceNode, Reasoning: looppkg.RuntimeSourceDefault,
				Speed: looppkg.RuntimeSourceInput,
			},
			SpeedResolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedFast, Status: speedpkg.ResolutionUnsupported,
				Reason: speedpkg.ReasonCapabilityAbsent,
			},
		}
		nextAttemptAt := time.Date(2026, 8, 2, 18, 0, 0, 0, time.UTC)
		failureClass := looppkg.FailureTransport
		payload := loopGenerationOutputsPayload(
			[]looppkg.GenerationOutput{
				{Generation: 1, NodeID: "work", ItemIndex: 2, Status: "succeeded", ResolvedRuntime: &resolved},
				{
					Generation: 2, NodeID: "retry", Status: "failed", Attempt: 2,
					NextAttemptAt: &nextAttemptAt,
				},
			},
			[]looppkg.NodeAttempt{{
				Generation: 2, NodeID: "retry", Attempt: 2, FailureClass: &failureClass,
				Disposition: looppkg.AttemptRetried,
			}},
		)
		if len(payload) != 2 || payload[0].ResolvedRuntime == nil {
			t.Fatalf("loopGenerationOutputsPayload() = %#v, want runtime and lifecycle outputs", payload)
		}
		runtime := payload[0].ResolvedRuntime
		if runtime.Provider != "mock" || runtime.Model != "node-model" || runtime.Reasoning != "high" ||
			runtime.Speed != speedpkg.SpeedFast || runtime.SpeedResolution == nil ||
			runtime.SpeedResolution.Status != speedpkg.ResolutionUnsupported ||
			runtime.Source.Provider != "agent" || runtime.Source.Model != "node" ||
			runtime.Source.Reasoning != "default" || runtime.Source.Speed != "input" {
			t.Fatalf("resolved runtime payload = %#v, want applied runtime and provenance", runtime)
		}
		encoded, err := json.Marshal(runtime)
		if err != nil {
			t.Fatalf("json.Marshal(runtime) error = %v", err)
		}
		want := `{"provider":"mock","model":"node-model","reasoning":"high","speed":"fast",` +
			`"speed_resolution":{"requested":"fast","status":"unsupported","reason":"capability_absent"},` +
			`"source":{"provider":"agent","model":"node","reasoning":"default","speed":"input"}}`
		if string(encoded) != want {
			t.Fatalf("resolved runtime JSON = %s, want %s", encoded, want)
		}
		legacyEncoded, err := json.Marshal(payload[0])
		if err != nil {
			t.Fatalf("json.Marshal(legacy output) error = %v", err)
		}
		if strings.Contains(string(legacyEncoded), `"attempt"`) ||
			strings.Contains(string(legacyEncoded), `"failure_class"`) ||
			strings.Contains(string(legacyEncoded), `"disposition"`) {
			t.Fatalf("legacy output JSON = %s, want lifecycle evidence omitted", legacyEncoded)
		}
		lifecycle := payload[1]
		if lifecycle.Attempt != 2 || lifecycle.NextAttemptAt == nil ||
			lifecycle.FailureClass != string(looppkg.FailureTransport) ||
			lifecycle.Disposition != string(looppkg.AttemptRetried) {
			t.Fatalf("lifecycle output = %#v, want durable attempt evidence", lifecycle)
		}
	})
}
