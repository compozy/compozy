package store

import (
	"errors"
	"strings"
	"testing"
)

const testStoreAttachmentID = "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestSessionInputQueueInsertAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should accept empty text when attachments are present", func(t *testing.T) {
		t.Parallel()

		req := validSessionInputQueueInsert()
		req.Text = "  "
		attachment := validSessionInputAttachment()
		attachment.ID = " " + attachment.ID + " "
		attachment.Name = " " + attachment.Name + " "
		attachment.MIMEType = " " + attachment.MIMEType + " "
		attachment.SHA256 = " " + attachment.SHA256 + " "
		attachment.Kind = " " + attachment.Kind + " "
		req.Attachments = []SessionInputAttachment{attachment}
		if err := req.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		normalized := req.Normalize()
		if normalized.Text != "" {
			t.Fatalf("Normalize().Text = %q, want empty", normalized.Text)
		}
		wantAttachment := validSessionInputAttachment()
		if len(normalized.Attachments) != 1 || normalized.Attachments[0] != wantAttachment {
			t.Fatalf(
				"Normalize().Attachments = %#v, want %#v",
				normalized.Attachments,
				[]SessionInputAttachment{wantAttachment},
			)
		}
	})

	t.Run("Should reject empty text when attachments are absent", func(t *testing.T) {
		t.Parallel()

		req := validSessionInputQueueInsert()
		req.Text = ""
		if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "text is required") {
			t.Fatalf("Validate() error = %v, want text is required", err)
		}
	})

	for _, tc := range []struct {
		name string
		text string
	}{
		{name: "Should reject steer attachments with empty text", text: ""},
		{name: "Should reject steer attachments with non-empty text", text: "redirect with context"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := validSessionInputQueueInsert()
			req.Mode = SessionInputQueueModeSteer
			req.Delivery = SessionInputDeliveryInterruptThenPrompt
			req.TargetTurnID = "turn-active"
			req.Text = tc.text
			req.Attachments = []SessionInputAttachment{validSessionInputAttachment()}
			if err := req.Validate(); !errors.Is(err, ErrSessionInputSteerTextOnly) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrSessionInputSteerTextOnly)
			}
		})
	}
}

func validSessionInputQueueInsert() SessionInputQueueInsert {
	return SessionInputQueueInsert{
		ID:        "inq-attachments",
		SessionID: "sess-attachments",
		Mode:      SessionInputQueueModeQueue,
		Delivery:  SessionInputDeliveryAfterTurn,
		Text:      "queued text",
		QueueCap:  10,
	}
}

func validSessionInputAttachment() SessionInputAttachment {
	return SessionInputAttachment{
		ID:       testStoreAttachmentID,
		Name:     "shot.png",
		MIMEType: "image/png",
		Bytes:    1024,
		SHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:     SessionInputAttachmentKindImage,
	}
}
