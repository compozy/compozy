package store

import (
	"errors"
	"strings"
	"testing"
)

func TestSessionPromptAdmissionRequestAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should accept empty authored text when attachments are present", func(t *testing.T) {
		t.Parallel()

		req := validSessionPromptAdmissionRequest()
		req.AuthoredText = "  "
		req.Attachments = []SessionInputAttachment{validSessionInputAttachment()}
		if err := req.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		normalized := req.Normalize()
		if normalized.AuthoredText != "" {
			t.Fatalf("Normalize().AuthoredText = %q, want empty", normalized.AuthoredText)
		}
		if len(normalized.Attachments) != 1 || normalized.Attachments[0].Name != "shot.png" {
			t.Fatalf("Normalize().Attachments = %#v", normalized.Attachments)
		}
	})

	t.Run("Should reject empty authored text when attachments are absent", func(t *testing.T) {
		t.Parallel()

		req := validSessionPromptAdmissionRequest()
		req.AuthoredText = ""
		if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "authored text is required") {
			t.Fatalf("Validate() error = %v, want authored text is required", err)
		}
	})

	t.Run("Should reject every steer request that carries attachments", func(t *testing.T) {
		t.Parallel()

		for _, authoredText := range []string{"", "steer with text"} {
			req := validSessionPromptAdmissionRequest()
			req.Operation = SessionPromptOperationSteer
			req.AuthoredText = authoredText
			req.Attachments = []SessionInputAttachment{validSessionInputAttachment()}
			if err := req.Validate(); !errors.Is(err, ErrSessionInputSteerTextOnly) {
				t.Fatalf("Validate(%q) error = %v, want ErrSessionInputSteerTextOnly", authoredText, err)
			}
		}
	})
}

func validSessionPromptAdmissionRequest() SessionPromptAdmissionRequest {
	return SessionPromptAdmissionRequest{
		ID:                 "pad-attachments",
		WorkspaceID:        "ws-attachments",
		SessionID:          "sess-attachments",
		MessageID:          "msg-attachments",
		IdempotencyKey:     "idem-attachments",
		Operation:          SessionPromptOperationPrompt,
		FingerprintVersion: "session-prompt/v3",
		RequestFingerprint: "sha256:attachments",
		AuthoredText:       "look at this",
		TurnID:             "turn-attachments",
		EventID:            "ev-attachments",
	}
}
