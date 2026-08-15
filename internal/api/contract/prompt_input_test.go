package contract_test

import (
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

const testPromptAttachmentID = "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestExtractPromptInputAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should accept an image-only prompt with empty message text", func(t *testing.T) {
		t.Parallel()

		input, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			MessageID:      "msg-image-only",
			IdempotencyKey: "idem-image-only",
			Attachments:    []contract.PromptAttachmentRef{validPromptAttachmentRef()},
		})
		if err != nil {
			t.Fatalf("ExtractPromptInput() error = %v", err)
		}
		if input.Message != "" {
			t.Fatalf("Message = %q, want empty", input.Message)
		}
		if len(input.Attachments) != 1 || input.Attachments[0].ID != testPromptAttachmentID {
			t.Fatalf("Attachments = %#v, want the image-only ref", input.Attachments)
		}
	})

	t.Run("Should populate attachments from the explicit request field", func(t *testing.T) {
		t.Parallel()

		ref := validPromptAttachmentRef()
		input, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			Message:        "Review this screenshot.",
			MessageID:      "msg-with-attachment",
			IdempotencyKey: "idem-with-attachment",
			Attachments:    []contract.PromptAttachmentRef{ref},
		})
		if err != nil {
			t.Fatalf("ExtractPromptInput() error = %v", err)
		}
		if input.Message != "Review this screenshot." {
			t.Fatalf("Message = %q, want Review this screenshot.", input.Message)
		}
		if len(input.Attachments) != 1 || input.Attachments[0] != ref {
			t.Fatalf("Attachments = %#v, want %#v", input.Attachments, []contract.PromptAttachmentRef{ref})
		}
	})

	t.Run("Should keep skipping unknown UI part types while copying attachments", func(t *testing.T) {
		t.Parallel()

		input, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			MessageID:      "msg-ui-parts",
			IdempotencyKey: "idem-ui-parts",
			Messages: []contract.PromptUIMessage{{
				ID:   "msg-ui-parts",
				Role: "user",
				Parts: []contract.PromptUITextPart{
					{Type: "image", Text: "ignored-image-part"},
					{Type: "text", Text: "visible text"},
				},
			}},
			Attachments: []contract.PromptAttachmentRef{validPromptAttachmentRef()},
		})
		if err != nil {
			t.Fatalf("ExtractPromptInput() error = %v", err)
		}
		if input.Message != "visible text" {
			t.Fatalf("Message = %q, want visible text", input.Message)
		}
		if len(input.Attachments) != 1 {
			t.Fatalf("Attachments = %#v, want 1 copied ref", input.Attachments)
		}
	})

	t.Run("Should reject an empty message when no attachments are present", func(t *testing.T) {
		t.Parallel()

		_, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			MessageID:      "msg-empty",
			IdempotencyKey: "idem-empty",
		})
		if err == nil || !strings.Contains(err.Error(), "message is required") {
			t.Fatalf("ExtractPromptInput() error = %v, want message is required", err)
		}
	})

	t.Run("Should reject a bad attachment id", func(t *testing.T) {
		t.Parallel()

		ref := validPromptAttachmentRef()
		ref.ID = "att_not-hex"
		_, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			MessageID:      "msg-bad-id",
			IdempotencyKey: "idem-bad-id",
			Attachments:    []contract.PromptAttachmentRef{ref},
		})
		if err == nil || !strings.Contains(err.Error(), "att_<64-hex>") {
			t.Fatalf("ExtractPromptInput() error = %v, want att_<64-hex>", err)
		}
	})

	t.Run("Should reject attachments that exceed the default count cap", func(t *testing.T) {
		t.Parallel()

		refs := make([]contract.PromptAttachmentRef, contract.DefaultPromptAttachmentLimit+1)
		for index := range refs {
			refs[index] = validPromptAttachmentRef()
		}
		_, err := contract.ExtractPromptInput(contract.SendPromptRequest{
			MessageID:      "msg-too-many",
			IdempotencyKey: "idem-too-many",
			Attachments:    refs,
		})
		if err == nil || !strings.Contains(err.Error(), "max count") {
			t.Fatalf("ExtractPromptInput() error = %v, want max count", err)
		}
	})
}

func TestValidatePromptAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should enforce the caller-provided count cap", func(t *testing.T) {
		t.Parallel()

		err := contract.ValidatePromptAttachments(
			[]contract.PromptAttachmentRef{validPromptAttachmentRef(), validPromptAttachmentRef()},
			1,
		)
		if err == nil || !strings.Contains(err.Error(), "max count 1") {
			t.Fatalf("ValidatePromptAttachments() error = %v, want max count 1", err)
		}
	})

	t.Run("Should reject an empty name and unknown kind", func(t *testing.T) {
		t.Parallel()

		missingName := validPromptAttachmentRef()
		missingName.Name = "  "
		if err := missingName.Validate(); err == nil || !strings.Contains(err.Error(), "name is required") {
			t.Fatalf("Validate() error = %v, want name is required", err)
		}
		badKind := validPromptAttachmentRef()
		badKind.Kind = "video"
		if err := badKind.Validate(); err == nil || !strings.Contains(err.Error(), "kind") {
			t.Fatalf("Validate() error = %v, want kind rejection", err)
		}
	})
}

func validPromptAttachmentRef() contract.PromptAttachmentRef {
	return contract.PromptAttachmentRef{
		ID:       testPromptAttachmentID,
		Name:     "shot.png",
		MIMEType: "image/png",
		Bytes:    1024,
		SHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:     contract.PromptAttachmentKindImage,
		Width:    320,
		Height:   240,
	}
}
