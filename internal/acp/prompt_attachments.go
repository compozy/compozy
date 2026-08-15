package acp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/attachments"
)

var (
	// ErrPromptImagesUnsupported reports that image blocks exceed the negotiated prompt capabilities.
	ErrPromptImagesUnsupported = errors.New("acp: prompt images unsupported")
	// ErrPromptFilesUnsupported reports that embedded file blocks exceed the negotiated prompt capabilities.
	ErrPromptFilesUnsupported = errors.New("acp: prompt files unsupported")
	// ErrPromptAttachmentDataRequired reports an attachment without content bytes.
	ErrPromptAttachmentDataRequired = errors.New("acp: prompt attachment data is required")
	// ErrPromptAttachmentMIMEUnsupported reports an attachment MIME type outside the prompt contract.
	ErrPromptAttachmentMIMEUnsupported = errors.New("acp: prompt attachment mime type unsupported")
)

const (
	mimeTypePDF       = "application/pdf"
	mimeTypeMarkdown  = "text/markdown"
	mimeTypePlainText = "text/plain"
)

type promptAttachmentClass uint8

const (
	promptAttachmentUnsupported promptAttachmentClass = iota
	promptAttachmentImage
	promptAttachmentPDF
	promptAttachmentText
)

func attachmentContentBlocks(req PromptRequest, caps Caps) ([]acpsdk.ContentBlock, error) {
	blocks := make([]acpsdk.ContentBlock, 0, len(req.Attachments))
	for index, attachment := range req.Attachments {
		mimeType := strings.ToLower(strings.TrimSpace(attachment.MIMEType))
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = fmt.Sprintf("attachment-%d", index+1)
		}

		switch classifyPromptAttachmentMIME(mimeType) {
		case promptAttachmentImage:
			if !caps.PromptImage {
				return nil, fmt.Errorf("acp: prompt attachment %q: %w", name, ErrPromptImagesUnsupported)
			}
			blocks = append(blocks, acpsdk.ImageBlock(base64.StdEncoding.EncodeToString(attachment.Data), mimeType))
		case promptAttachmentPDF:
			if !caps.PromptEmbeddedContext {
				return nil, fmt.Errorf("acp: prompt attachment %q: %w", name, ErrPromptFilesUnsupported)
			}
			blocks = append(blocks, blobResourceBlock(name, mimeType, attachment.Data))
		case promptAttachmentText:
			if caps.PromptEmbeddedContext {
				blocks = append(blocks, textResourceBlock(name, mimeType, attachment.Data))
				continue
			}
			blocks = append(blocks, acpsdk.TextBlock(fmt.Sprintf("--- %s ---\n%s", name, attachment.Data)))
		default:
			return nil, fmt.Errorf("acp: prompt attachment %q has unsupported mime type %q", name, mimeType)
		}
	}
	return blocks, nil
}

func blobResourceBlock(name string, mimeType string, data []byte) acpsdk.ContentBlock {
	return acpsdk.ResourceBlock(acpsdk.EmbeddedResourceResource{
		BlobResourceContents: &acpsdk.BlobResourceContents{
			Blob:     base64.StdEncoding.EncodeToString(data),
			MimeType: &mimeType,
			Uri:      promptAttachmentURI(name),
		},
	})
}

func textResourceBlock(name string, mimeType string, data []byte) acpsdk.ContentBlock {
	return acpsdk.ResourceBlock(acpsdk.EmbeddedResourceResource{
		TextResourceContents: &acpsdk.TextResourceContents{
			MimeType: &mimeType,
			Text:     string(data),
			Uri:      promptAttachmentURI(name),
		},
	})
}

func promptAttachmentURI(name string) string {
	return attachments.AttachmentURIPrefix + url.PathEscape(name)
}

func classifyPromptAttachmentMIME(mimeType string) promptAttachmentClass {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(normalized, "image/"):
		return promptAttachmentImage
	case normalized == mimeTypePDF:
		return promptAttachmentPDF
	case normalized == mimeTypeMarkdown, normalized == mimeTypePlainText:
		return promptAttachmentText
	default:
		return promptAttachmentUnsupported
	}
}
