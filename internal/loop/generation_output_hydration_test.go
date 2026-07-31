package loop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type stubOutputPayloadReader struct {
	payloads map[string]json.RawMessage
	calls    map[string]int
}

func (r *stubOutputPayloadReader) ListGenerationOutputs(
	_ context.Context,
	_ WorkspaceID,
	_ RunID,
	_ int,
) ([]GenerationOutput, error) {
	return nil, nil
}

func (r *stubOutputPayloadReader) GetGenerationOutputPayload(
	_ context.Context,
	outputRef string,
) (json.RawMessage, error) {
	if r.calls == nil {
		r.calls = map[string]int{}
	}
	r.calls[outputRef]++
	payload, ok := r.payloads[outputRef]
	if !ok {
		return nil, ErrOutputRefNotFound
	}
	return payload, nil
}

func TestHydrateGenerationOutputsShouldResolveExternalizedPayloads(t *testing.T) {
	t.Parallel()

	// A node output above LoopOutputInlineLimitBytes is stored as a content-addressed ref.
	// Left unresolved, every template reading that node sees the hash instead of the value.
	largeRef := OutputRefForPayload(json.RawMessage(`{"tasks":[{"id":"task_02"}]}`))

	t.Run("Should replace content-addressed refs with their payload", func(t *testing.T) {
		t.Parallel()

		reader := &stubOutputPayloadReader{payloads: map[string]json.RawMessage{
			largeRef: json.RawMessage(`{"tasks":[{"id":"task_02"}]}`),
		}}
		outputs := []GenerationOutput{{NodeID: "load_tasks", Status: "succeeded", OutputRef: largeRef}}

		if err := hydrateGenerationOutputs(context.Background(), reader, outputs); err != nil {
			t.Fatalf("hydrateGenerationOutputs() error = %v", err)
		}
		if got, want := outputs[0].OutputRef, `{"tasks":[{"id":"task_02"}]}`; got != want {
			t.Fatalf("output ref = %q, want %q", got, want)
		}
	})

	t.Run("Should leave inline outputs untouched", func(t *testing.T) {
		t.Parallel()

		reader := &stubOutputPayloadReader{}
		outputs := []GenerationOutput{
			{NodeID: "slug_input", Status: "succeeded", OutputRef: `"marketplace-ops-board"`},
			{NodeID: "pending", Status: "pending"},
		}

		if err := hydrateGenerationOutputs(context.Background(), reader, outputs); err != nil {
			t.Fatalf("hydrateGenerationOutputs() error = %v", err)
		}
		if got, want := outputs[0].OutputRef, `"marketplace-ops-board"`; got != want {
			t.Fatalf("inline ref = %q, want %q", got, want)
		}
		if got := len(reader.calls); got != 0 {
			t.Fatalf("payload lookups = %d, want 0", got)
		}
	})

	t.Run("Should resolve each ref once when outputs share a payload", func(t *testing.T) {
		t.Parallel()

		reader := &stubOutputPayloadReader{payloads: map[string]json.RawMessage{
			largeRef: json.RawMessage(`{"tasks":[]}`),
		}}
		outputs := []GenerationOutput{
			{NodeID: "load_tasks", ItemIndex: 0, OutputRef: largeRef},
			{NodeID: "load_tasks", ItemIndex: 1, OutputRef: largeRef},
		}

		if err := hydrateGenerationOutputs(context.Background(), reader, outputs); err != nil {
			t.Fatalf("hydrateGenerationOutputs() error = %v", err)
		}
		if got, want := reader.calls[largeRef], 1; got != want {
			t.Fatalf("payload lookups = %d, want %d", got, want)
		}
	})

	t.Run("Should surface a missing payload instead of degrading to the ref", func(t *testing.T) {
		t.Parallel()

		reader := &stubOutputPayloadReader{}
		outputs := []GenerationOutput{{NodeID: "load_tasks", OutputRef: largeRef}}

		err := hydrateGenerationOutputs(context.Background(), reader, outputs)
		if !errors.Is(err, ErrOutputRefNotFound) {
			t.Fatalf("hydrateGenerationOutputs() error = %v, want %v", err, ErrOutputRefNotFound)
		}
	})
}

func TestHydratedOutputsShouldResolveFanOutCollections(t *testing.T) {
	t.Parallel()

	// Regression for the reported failure: before hydration the fan-out collection reference
	// resolved against the literal "sha256:…" string and the coordinator aborted the run.
	payload := json.RawMessage(`{"tasks":[{"id":"task_02"},{"id":"task_04"}]}`)
	ref := OutputRefForPayload(payload)
	reader := &stubOutputPayloadReader{payloads: map[string]json.RawMessage{ref: payload}}
	outputs := []GenerationOutput{{NodeID: "load_tasks", Status: "succeeded", OutputRef: ref}}

	t.Run("Should expose node output fields after hydration", func(t *testing.T) {
		t.Parallel()

		if _, ok := valueAtPath(namespaceForOutputs(outputs), []string{
			"nodes", "load_tasks", "output", "tasks",
		}); ok {
			t.Fatal("collection resolved before hydration, want unavailable")
		}
		if err := hydrateGenerationOutputs(context.Background(), reader, outputs); err != nil {
			t.Fatalf("hydrateGenerationOutputs() error = %v", err)
		}
		value, ok := valueAtPath(namespaceForOutputs(outputs), []string{
			"nodes", "load_tasks", "output", "tasks",
		})
		if !ok {
			t.Fatal("collection reference unavailable after hydration")
		}
		items, err := collectionItems(value)
		if err != nil {
			t.Fatalf("collectionItems() error = %v", err)
		}
		if got, want := len(items), 2; got != want {
			t.Fatalf("items len = %d, want %d", got, want)
		}
	})
}

func namespaceForOutputs(outputs []GenerationOutput) map[string]any {
	nodes := map[string]any{}
	for _, output := range outputs {
		nodes[output.NodeID] = map[string]any{
			"status": output.Status,
			"output": outputValue(output.OutputRef),
		}
	}
	return map[string]any{"nodes": nodes}
}
