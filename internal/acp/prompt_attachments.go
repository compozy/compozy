package acp

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

var (
	// ErrPromptImagesUnsupported reports that image blocks exceed the negotiated prompt capabilities.
	ErrPromptImagesUnsupported = errors.New("acp: prompt images unsupported")
	// ErrPromptFilesUnsupported reports that embedded file blocks exceed the negotiated prompt capabilities.
	ErrPromptFilesUnsupported = errors.New("acp: prompt files unsupported")
)

const promptAttachmentURIPrefix = "compozy://session-attachments/"

func attachmentContentBlocks(req PromptRequest, caps Caps) ([]acpsdk.ContentBlock, error) {
	blocks := make([]acpsdk.ContentBlock, 0, len(req.Attachments))
	for index, attachment := range req.Attachments {
		mimeType := strings.ToLower(strings.TrimSpace(attachment.MIMEType))
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = fmt.Sprintf("attachment-%d", index+1)
		}

		switch {
		case strings.HasPrefix(mimeType, "image/"):
			if !caps.PromptImage {
				return nil, fmt.Errorf("acp: prompt attachment %q: %w", name, ErrPromptImagesUnsupported)
			}
			blocks = append(blocks, acpsdk.ImageBlock(base64.StdEncoding.EncodeToString(attachment.Data), mimeType))
		case mimeType == "application/pdf":
			if !caps.PromptEmbeddedContext {
				return nil, fmt.Errorf("acp: prompt attachment %q: %w", name, ErrPromptFilesUnsupported)
			}
			blocks = append(blocks, blobResourceBlock(name, mimeType, attachment.Data))
		case mimeType == "text/markdown", mimeType == "text/plain":
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
	return promptAttachmentURIPrefix + url.PathEscape(name)
}

func isAllowedPromptAttachmentMIME(mimeType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mimeType))
	return strings.HasPrefix(normalized, "image/") ||
		normalized == "application/pdf" ||
		normalized == "text/markdown" ||
		normalized == "text/plain"
}
