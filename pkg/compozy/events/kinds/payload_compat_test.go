package kinds

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestShutdownPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	now := time.Unix(10, 0).UTC()
	payload := ShutdownTerminatedPayload{
		ShutdownBase: ShutdownBase{
			Source:      "signal",
			RequestedAt: now,
			DeadlineAt:  now.Add(3 * time.Second),
		},
		Forced: true,
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"source":       "signal",
		"requested_at": now.Format(time.RFC3339),
		"deadline_at":  now.Add(3 * time.Second).Format(time.RFC3339),
		"forced":       true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("shutdown payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func TestJobAttemptPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	payload := JobAttemptFinishedPayload{
		JobAttemptInfo: JobAttemptInfo{
			Index:       1,
			Attempt:     2,
			MaxAttempts: 3,
		},
		Status:    "failure",
		ExitCode:  17,
		Retryable: true,
		Error:     "transient",
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"index":        float64(1),
		"attempt":      float64(2),
		"max_attempts": float64(3),
		"status":       "failure",
		"exit_code":    float64(17),
		"retryable":    true,
		"error":        "transient",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("job attempt payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func TestExtensionReadyPayloadJSONCompatibility(t *testing.T) {
	t.Parallel()

	payload := ExtensionReadyPayload{
		Extension:            "mock-ext",
		Source:               "workspace",
		Version:              "1.0.0",
		ProtocolVersion:      "1",
		AcceptedCapabilities: []string{"events.read", "tasks.read"},
		SupportedHookEvents:  []string{"prompt.post_build"},
	}

	got := mustMarshalMap(t, payload)
	want := map[string]any{
		"extension":             "mock-ext",
		"source":                "workspace",
		"version":               "1.0.0",
		"protocol_version":      "1",
		"accepted_capabilities": []any{"events.read", "tasks.read"},
		"supported_hook_events": []any{"prompt.post_build"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("extension ready payload JSON mismatch: got %#v want %#v", got, want)
	}
}

func mustMarshalMap(t *testing.T, payload any) map[string]any {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return decoded
}
