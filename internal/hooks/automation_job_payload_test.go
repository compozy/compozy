package hooks

import (
	"encoding/json"
	"testing"
)

func TestAutomationJobPreFirePayloadContract(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve manual job payload across JSON", func(t *testing.T) {
		t.Parallel()

		payload := AutomationJobPreFirePayload{
			JobID:   "job-1",
			Prompt:  "Review",
			Payload: map[string]any{"repo": "acme/api"},
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}

		var decoded AutomationJobPreFirePayload
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if got, want := decoded.Payload["repo"], "acme/api"; got != want {
			t.Fatalf("decoded.Payload[repo] = %#v, want %q", got, want)
		}
	})

	t.Run("Should clone manual job payload for async hooks", func(t *testing.T) {
		t.Parallel()

		payload := AutomationJobPreFirePayload{
			Payload: map[string]any{
				"metadata": map[string]any{"branch": "main"},
			},
		}
		cloned := payload.cloneForAsync()
		metadata, ok := cloned.Payload["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("cloned.Payload[metadata] = %#v, want map", cloned.Payload["metadata"])
		}
		metadata["branch"] = "release"

		originalMetadata, ok := payload.Payload["metadata"].(map[string]any)
		if !ok {
			t.Fatalf("payload.Payload[metadata] = %#v, want map", payload.Payload["metadata"])
		}
		if got, want := originalMetadata["branch"], "main"; got != want {
			t.Fatalf("original payload branch = %#v, want %q", got, want)
		}
	})
}
