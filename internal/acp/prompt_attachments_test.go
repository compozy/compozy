package acp

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestAttachmentContentBlocks(t *testing.T) {
	t.Parallel()

	t.Run("Should encode image bytes when image input is supported", func(t *testing.T) {
		t.Parallel()

		data := []byte("image-bytes")
		blocks, err := attachmentContentBlocks(PromptRequest{Attachments: []PromptAttachment{{
			Name: "diagram.png", MIMEType: "image/png", Data: data,
		}}}, Caps{PromptImage: true})
		if err != nil {
			t.Fatalf("attachmentContentBlocks() error = %v", err)
		}
		if len(blocks) != 1 || blocks[0].Image == nil {
			t.Fatalf("blocks = %#v, want one image block", blocks)
		}
		if got, want := blocks[0].Image.Data, base64.StdEncoding.EncodeToString(data); got != want {
			t.Fatalf("image data = %q, want %q", got, want)
		}
		if got, want := blocks[0].Image.MimeType, "image/png"; got != want {
			t.Fatalf("image MIME = %q, want %q", got, want)
		}
	})

	t.Run("Should embed PDF bytes when embedded context is supported", func(t *testing.T) {
		t.Parallel()

		data := []byte("pdf-bytes")
		blocks, err := attachmentContentBlocks(PromptRequest{Attachments: []PromptAttachment{{
			Name: "report.pdf", MIMEType: "application/pdf", Data: data,
		}}}, Caps{PromptEmbeddedContext: true})
		if err != nil {
			t.Fatalf("attachmentContentBlocks() error = %v", err)
		}
		if len(blocks) != 1 || blocks[0].Resource == nil ||
			blocks[0].Resource.Resource.BlobResourceContents == nil {
			t.Fatalf("blocks = %#v, want one blob resource block", blocks)
		}
		resource := blocks[0].Resource.Resource.BlobResourceContents
		if got, want := resource.Blob, base64.StdEncoding.EncodeToString(data); got != want {
			t.Fatalf("resource blob = %q, want %q", got, want)
		}
		if got, want := resource.Uri, "compozy://session-attachments/report.pdf"; got != want {
			t.Fatalf("resource URI = %q, want %q", got, want)
		}
	})

	t.Run("Should embed markdown text when embedded context is supported", func(t *testing.T) {
		t.Parallel()

		blocks, err := attachmentContentBlocks(PromptRequest{Attachments: []PromptAttachment{{
			Name: "notes.md", MIMEType: "text/markdown", Data: []byte("# Notes"),
		}}}, Caps{PromptEmbeddedContext: true})
		if err != nil {
			t.Fatalf("attachmentContentBlocks() error = %v", err)
		}
		if len(blocks) != 1 || blocks[0].Resource == nil ||
			blocks[0].Resource.Resource.TextResourceContents == nil {
			t.Fatalf("blocks = %#v, want one text resource block", blocks)
		}
		if got, want := blocks[0].Resource.Resource.TextResourceContents.Text, "# Notes"; got != want {
			t.Fatalf("resource text = %q, want %q", got, want)
		}
	})

	t.Run("Should fall back to baseline text when embedded context is unavailable", func(t *testing.T) {
		t.Parallel()

		blocks, err := attachmentContentBlocks(PromptRequest{Attachments: []PromptAttachment{{
			Name: "notes.txt", MIMEType: "text/plain", Data: []byte("plain notes"),
		}}}, Caps{})
		if err != nil {
			t.Fatalf("attachmentContentBlocks() error = %v", err)
		}
		if len(blocks) != 1 || blocks[0].Text == nil {
			t.Fatalf("blocks = %#v, want one text block", blocks)
		}
		if got, want := blocks[0].Text.Text, "--- notes.txt ---\nplain notes"; got != want {
			t.Fatalf("fallback text = %q, want %q", got, want)
		}
	})
}

func TestAttachmentContentBlocksCapabilityGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		item    PromptAttachment
		wantErr error
	}{
		{
			name:    "Should reject image input without the image capability",
			item:    PromptAttachment{Name: "photo.png", MIMEType: "image/png", Data: []byte("image")},
			wantErr: ErrPromptImagesUnsupported,
		},
		{
			name:    "Should reject PDF input without embedded context",
			item:    PromptAttachment{Name: "report.pdf", MIMEType: "application/pdf", Data: []byte("pdf")},
			wantErr: ErrPromptFilesUnsupported,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := attachmentContentBlocks(PromptRequest{Attachments: []PromptAttachment{tt.item}}, Caps{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("attachmentContentBlocks() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPromptRequestValidateAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should accept an image-only prompt", func(t *testing.T) {
		t.Parallel()

		err := (PromptRequest{TurnID: "turn-image", Attachments: []PromptAttachment{{
			Name: "photo.png", MIMEType: "image/png", Data: []byte("image"),
		}}}).Validate()
		if err != nil {
			t.Fatalf("PromptRequest.Validate() error = %v", err)
		}
	})

	t.Run("Should reject empty attachment data", func(t *testing.T) {
		t.Parallel()

		err := (PromptRequest{TurnID: "turn-empty-data", Attachments: []PromptAttachment{{
			Name: "photo.png", MIMEType: "image/png",
		}}}).Validate()
		if err == nil || !strings.Contains(err.Error(), "data is required") {
			t.Fatalf("PromptRequest.Validate() error = %v, want missing data error", err)
		}
	})

	t.Run("Should reject unsupported MIME without exposing bytes", func(t *testing.T) {
		t.Parallel()

		secret := "attachment-secret-material"
		err := (PromptRequest{TurnID: "turn-invalid", Attachments: []PromptAttachment{{
			Name: "secret.bin", MIMEType: "application/octet-stream", Data: []byte(secret),
		}}}).Validate()
		if err == nil {
			t.Fatal("PromptRequest.Validate() error = nil, want unsupported MIME error")
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("PromptRequest.Validate() error exposed attachment data: %v", err)
		}
	})
}
