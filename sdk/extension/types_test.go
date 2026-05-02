package extension_test

import (
	"encoding/json"
	"strings"
	"testing"

	extension "github.com/compozy/compozy/sdk/extension"
)

func TestSessionRequestJSONUsesReadablePromptText(t *testing.T) {
	t.Parallel()

	request := extension.SessionRequest{
		Prompt:     []byte("plain prompt"),
		WorkingDir: "/tmp/work",
		Model:      "gpt-5.5",
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal session request: %v", err)
	}
	if strings.Contains(string(raw), "cGxhaW4gcHJvbXB0") {
		t.Fatalf("expected prompt text instead of base64 JSON, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"prompt":"plain prompt"`) {
		t.Fatalf("expected readable prompt JSON, got %s", string(raw))
	}

	var roundTrip extension.SessionRequest
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal session request: %v", err)
	}
	if got := string(roundTrip.Prompt); got != "plain prompt" {
		t.Fatalf("unexpected round-trip prompt: %q", got)
	}
}

func TestResumeSessionRequestJSONUsesReadablePromptText(t *testing.T) {
	t.Parallel()

	request := extension.ResumeSessionRequest{
		SessionID:  "sess-123",
		Prompt:     []byte("resume prompt"),
		WorkingDir: "/tmp/work",
		Model:      "gpt-5.5",
	}

	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal resume session request: %v", err)
	}
	if strings.Contains(string(raw), "cmVzdW1lIHByb21wdA==") {
		t.Fatalf("expected prompt text instead of base64 JSON, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"prompt":"resume prompt"`) {
		t.Fatalf("expected readable resume prompt JSON, got %s", string(raw))
	}

	var roundTrip extension.ResumeSessionRequest
	if err := json.Unmarshal(raw, &roundTrip); err != nil {
		t.Fatalf("unmarshal resume session request: %v", err)
	}
	if got := string(roundTrip.Prompt); got != "resume prompt" {
		t.Fatalf("unexpected round-trip resume prompt: %q", got)
	}
}
