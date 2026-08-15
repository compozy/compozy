package store

import (
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
		req.Attachments = []SessionInputAttachment{validSessionInputAttachment()}
		if err := req.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		normalized := req.Normalize()
		if normalized.Text != "" {
			t.Fatalf("Normalize().Text = %q, want empty", normalized.Text)
		}
		if len(normalized.Attachments) != 1 || normalized.Attachments[0].ID != testStoreAttachmentID {
			t.Fatalf("Normalize().Attachments = %#v", normalized.Attachments)
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

	t.Run("Should reject empty steer text even when attachments are present", func(t *testing.T) {
		t.Parallel()

		req := validSessionInputQueueInsert()
		req.Mode = SessionInputQueueModeSteer
		req.Delivery = SessionInputDeliveryInterruptThenPrompt
		req.TargetTurnID = "turn-active"
		req.Text = ""
		req.Attachments = []SessionInputAttachment{validSessionInputAttachment()}
		if err := req.Validate(); err == nil || !strings.Contains(err.Error(), "text is required") {
			t.Fatalf("Validate() error = %v, want text is required", err)
		}
	})
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
