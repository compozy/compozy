package extensionpkg

import (
	"encoding/json"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func bridgeInstancesContract(instances []bridgepkg.BridgeInstance) []bridgecontract.BridgeInstance {
	if instances == nil {
		return nil
	}
	converted := make([]bridgecontract.BridgeInstance, len(instances))
	for index := range instances {
		converted[index] = bridgepkg.BridgeInstanceToContract(instances[index])
	}
	return converted
}

func bridgeInboundEnvelopeDomain(envelope bridgecontract.InboundMessageEnvelope) bridgepkg.InboundMessageEnvelope {
	converted := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  envelope.BridgeInstanceID,
		Scope:             bridgepkg.Scope(envelope.Scope),
		WorkspaceID:       envelope.WorkspaceID,
		PeerID:            envelope.PeerID,
		ThreadID:          envelope.ThreadID,
		GroupID:           envelope.GroupID,
		PlatformMessageID: envelope.PlatformMessageID,
		ReceivedAt:        envelope.ReceivedAt,
		Sender: bridgepkg.MessageSender{
			ID: envelope.Sender.ID, Username: envelope.Sender.Username,
			DisplayName: envelope.Sender.DisplayName,
		},
		Content:           bridgepkg.MessageContent{Text: envelope.Content.Text},
		EventFamily:       bridgepkg.InboundEventFamily(envelope.EventFamily),
		ReplyToText:       envelope.ReplyToText,
		ReplyToAuthorID:   envelope.ReplyToAuthorID,
		ReplyToAuthorName: envelope.ReplyToAuthorName,
		IdempotencyKey:    envelope.IdempotencyKey,
	}
	converted.ProviderMetadata = append(json.RawMessage(nil), envelope.ProviderMetadata...)
	converted.Attachments = bridgeAttachmentsDomain(envelope.Attachments)
	if envelope.Command != nil {
		converted.Command = &bridgepkg.InboundCommand{
			Command: envelope.Command.Command, Text: envelope.Command.Text,
			TriggerID: envelope.Command.TriggerID,
		}
	}
	if envelope.Action != nil {
		converted.Action = &bridgepkg.InboundAction{
			ActionID: envelope.Action.ActionID, MessageID: envelope.Action.MessageID,
			Value: envelope.Action.Value, TriggerID: envelope.Action.TriggerID,
		}
	}
	if envelope.Reaction != nil {
		converted.Reaction = &bridgepkg.InboundReaction{
			MessageID: envelope.Reaction.MessageID, Emoji: envelope.Reaction.Emoji,
			RawEmoji: envelope.Reaction.RawEmoji, Added: envelope.Reaction.Added,
		}
	}
	if envelope.Edit != nil {
		converted.Edit = &bridgepkg.InboundEdit{
			MessageID: envelope.Edit.MessageID, NewText: envelope.Edit.NewText,
			OriginalTimestamp: envelope.Edit.OriginalTimestamp,
			Operation:         bridgepkg.InboundEditOperation(envelope.Edit.Operation),
		}
	}
	if envelope.Conversation != nil {
		converted.Conversation = &bridgepkg.NetworkConversationRef{
			Channel:  envelope.Conversation.Channel,
			Surface:  bridgepkg.NetworkConversationSurface(envelope.Conversation.Surface),
			ThreadID: envelope.Conversation.ThreadID, DirectID: envelope.Conversation.DirectID,
			WorkID: envelope.Conversation.WorkID, ReplyTo: envelope.Conversation.ReplyTo,
			TraceID: envelope.Conversation.TraceID, CausationID: envelope.Conversation.CausationID,
		}
	}
	return converted
}

func bridgeAttachmentsDomain(attachments []bridgecontract.MessageAttachment) []bridgepkg.MessageAttachment {
	if attachments == nil {
		return nil
	}
	converted := make([]bridgepkg.MessageAttachment, len(attachments))
	for index := range attachments {
		converted[index] = bridgepkg.MessageAttachment{
			ID: attachments[index].ID, Name: attachments[index].Name,
			MIMEType: attachments[index].MIMEType, URL: attachments[index].URL,
		}
	}
	return converted
}

func bridgeStatusDomain(status bridgecontract.BridgeStatus) bridgepkg.BridgeStatus {
	return bridgepkg.BridgeStatus(status)
}

func bridgeDegradationDomain(degradation *bridgecontract.BridgeDegradation) *bridgepkg.BridgeDegradation {
	if degradation == nil {
		return nil
	}
	return &bridgepkg.BridgeDegradation{
		Reason:  bridgepkg.BridgeDegradationReason(degradation.Reason),
		Message: degradation.Message,
	}
}

func bridgeRoutingKeyContract(key bridgepkg.RoutingKey) bridgecontract.RoutingKey {
	return bridgecontract.RoutingKey{
		Scope: bridgecontract.Scope(key.Scope), WorkspaceID: key.WorkspaceID,
		BridgeInstanceID: key.BridgeInstanceID, PeerID: key.PeerID,
		ThreadID: key.ThreadID, GroupID: key.GroupID,
	}
}

func bridgeRoutingKeyDomain(key bridgecontract.RoutingKey) bridgepkg.RoutingKey {
	return bridgepkg.RoutingKey{
		Scope: bridgepkg.Scope(key.Scope), WorkspaceID: key.WorkspaceID,
		BridgeInstanceID: key.BridgeInstanceID, PeerID: key.PeerID,
		ThreadID: key.ThreadID, GroupID: key.GroupID,
	}
}
