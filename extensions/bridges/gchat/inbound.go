package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func mapGChatMessage(
	message gchatMessage,
	space gchatSpace,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	idempotencyBase string,
	source string,
	parents *bridgesdk.ParentMessageCache,
) (gchatMappedInbound, error) {
	direct := isDirectSpace(space)
	threadName := threadNameForMessage(message, direct)
	threadID := ""
	if direct || threadName != "" {
		threadID = encodeGChatThreadID(gchatThreadRef{
			SpaceName:  strings.TrimSpace(space.Name),
			ThreadName: threadName,
			IsDM:       direct,
		})
	}
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		PlatformMessageID: strings.TrimSpace(message.Name),
		ReceivedAt:        normalizeReceivedAt(receivedAt, message.CreateTime),
		Sender:            gchatSender(message.Sender),
		Content: bridgepkg.MessageContent{
			Text: normalizeGChatText(message),
		},
		Attachments:    normalizeGChatAttachments(message.Attachment),
		EventFamily:    bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: fmt.Sprintf("gchat:%s:%s", managed.Instance.ID, strings.TrimSpace(idempotencyBase)),
	}
	if direct {
		envelope.PeerID = strings.TrimSpace(space.Name)
	} else {
		envelope.GroupID = strings.TrimSpace(space.Name)
	}
	envelope.ThreadID = threadID
	applyGChatReplyContext(&envelope, message, parents)
	metadata, err := marshalGChatInboundMetadata(message, space, source, threadName)
	if err != nil {
		return gchatMappedInbound{}, err
	}
	envelope.ProviderMetadata = metadata

	item := gchatMappedInbound{
		Envelope: envelope,
		Direct:   direct,
		User: gchatUserIdentity{
			ID:          envelope.Sender.ID,
			Username:    envelope.Sender.Username,
			DisplayName: envelope.Sender.DisplayName,
		},
	}
	return item, item.Envelope.Validate()
}

func applyGChatReplyContext(
	envelope *bridgepkg.InboundMessageEnvelope,
	message gchatMessage,
	parents *bridgesdk.ParentMessageCache,
) {
	parentMessageID := gchatReplyParentMessageID(message)
	if parentMessageID == "" {
		return
	}
	parents.EnrichReply(envelope, parentMessageID)
	if message.QuotedMessageMetadata == nil || message.QuotedMessageMetadata.QuotedMessageSnapshot == nil {
		return
	}
	bridgesdk.ApplyParentMessage(
		envelope,
		gchatParentMessage(*message.QuotedMessageMetadata.QuotedMessageSnapshot),
	)
}

func rememberGChatQuotedParent(
	parents *bridgesdk.ParentMessageCache,
	envelope bridgepkg.InboundMessageEnvelope,
	message gchatMessage,
) {
	if message.QuotedMessageMetadata == nil || message.QuotedMessageMetadata.QuotedMessageSnapshot == nil {
		return
	}
	parents.RememberParent(
		envelope,
		gchatReplyParentMessageID(message),
		gchatParentMessage(*message.QuotedMessageMetadata.QuotedMessageSnapshot),
	)
}

func gchatParentMessage(message gchatQuotedMessageSnapshot) bridgesdk.ParentMessage {
	return bridgesdk.ParentMessage{
		Text:     firstNonEmpty(strings.TrimSpace(message.Text), strings.TrimSpace(message.FormattedText)),
		AuthorID: strings.TrimSpace(message.Sender),
	}
}

func shouldBatchGChatMessage(message gchatMessage) bool {
	return gchatReplyParentMessageID(message) == ""
}

func gchatReplyParentMessageID(message gchatMessage) string {
	metadata := message.QuotedMessageMetadata
	if metadata == nil {
		return ""
	}
	quoteType := strings.ToUpper(strings.TrimSpace(metadata.QuoteType))
	if quoteType != "" && quoteType != "REPLY" {
		return ""
	}
	return strings.TrimSpace(metadata.Name)
}

func marshalGChatInboundMetadata(
	message gchatMessage,
	space gchatSpace,
	source string,
	threadName string,
) (json.RawMessage, error) {
	quotedMessageName := ""
	quoteType := ""
	if message.QuotedMessageMetadata != nil {
		quotedMessageName = strings.TrimSpace(message.QuotedMessageMetadata.Name)
		quoteType = strings.TrimSpace(message.QuotedMessageMetadata.QuoteType)
	}
	metadata, err := json.Marshal(map[string]any{
		providerSourceKey:     strings.TrimSpace(source),
		providerSpaceNameKey:  strings.TrimSpace(space.Name),
		"space_type":          firstNonEmpty(strings.TrimSpace(space.SpaceType), strings.TrimSpace(space.Type)),
		providerThreadNameKey: threadName,
		"message_name":        strings.TrimSpace(message.Name),
		"quoted_message_name": quotedMessageName,
		"quote_type":          quoteType,
	})
	if err != nil {
		return nil, fmt.Errorf("gchat: marshal inbound metadata: %w", err)
	}
	return metadata, nil
}
