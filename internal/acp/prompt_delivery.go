package acp

import (
	"fmt"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

func (p *AgentProcess) promptDeliveryManifest(
	req PromptRequest,
	sections []DeliveredSpan,
	attachments []acpsdk.ContentBlock,
	startup bool,
	delivery SystemPromptDeliveryMode,
) DeliveryManifest {
	manifest := DeliveryManifest{
		TurnID:   req.TurnID,
		SentAt:   timeNowUTC(),
		Estimate: TextEstimateMethod,
		Spans:    make([]DeliveredSpan, 0),
	}
	if startup {
		p.systemPromptMu.Lock()
		source := CloneStartupManifest(p.startupManifest)
		if len(source.Spans) == 0 {
			source = OpaqueStartupManifest(strings.TrimSpace(p.systemPrompt), false)
		}
		p.systemPromptMu.Unlock()
		for _, span := range source.Spans {
			span.Delivery = string(delivery)
			manifest.Spans = append(manifest.Spans, span)
		}
	}
	manifest.Spans = append(manifest.Spans, sections...)
	for index, attachment := range req.Attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = fmt.Sprintf("attachment-%d", index+1)
		}
		span := DeliveredSpan{Key: "attachment", Kind: "binary", Bytes: int64(len(attachment.Data)), Name: name}
		if classifyPromptAttachmentMIME(attachment.MIMEType) == promptAttachmentText {
			text := string(attachment.Data)
			if block := attachments[index]; block.Text != nil {
				text = block.Text.Text
			}
			span = TextSpan("attachment", text)
			span.Name = name
		}
		manifest.Spans = append(manifest.Spans, span)
	}
	return manifest
}
