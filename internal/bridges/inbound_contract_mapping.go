package bridges

import (
	"slices"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func networkConversationRefToContract(reference NetworkConversationRef) bridgecontract.NetworkConversationRef {
	return bridgecontract.NetworkConversationRef{
		Channel:     reference.Channel,
		Surface:     bridgecontract.NetworkConversationSurface(reference.Surface),
		ThreadID:    reference.ThreadID,
		DirectID:    reference.DirectID,
		WorkID:      reference.WorkID,
		ReplyTo:     reference.ReplyTo,
		TraceID:     reference.TraceID,
		CausationID: reference.CausationID,
	}
}

func networkConversationRefFromContract(reference bridgecontract.NetworkConversationRef) NetworkConversationRef {
	return NetworkConversationRef{
		Channel:     reference.Channel,
		Surface:     NetworkConversationSurface(reference.Surface),
		ThreadID:    reference.ThreadID,
		DirectID:    reference.DirectID,
		WorkID:      reference.WorkID,
		ReplyTo:     reference.ReplyTo,
		TraceID:     reference.TraceID,
		CausationID: reference.CausationID,
	}
}

func inboundEnvelopeToContract(envelope InboundMessageEnvelope) bridgecontract.InboundMessageEnvelope {
	converted := bridgecontract.InboundMessageEnvelope{
		BridgeInstanceID:  envelope.BridgeInstanceID,
		Scope:             bridgecontract.Scope(envelope.Scope),
		WorkspaceID:       envelope.WorkspaceID,
		PeerID:            envelope.PeerID,
		ThreadID:          envelope.ThreadID,
		GroupID:           envelope.GroupID,
		PlatformMessageID: envelope.PlatformMessageID,
		ReceivedAt:        envelope.ReceivedAt,
		Sender: bridgecontract.MessageSender{
			ID: envelope.Sender.ID, Username: envelope.Sender.Username, DisplayName: envelope.Sender.DisplayName,
		},
		Content:           bridgecontract.MessageContent{Text: envelope.Content.Text},
		EventFamily:       bridgecontract.InboundEventFamily(envelope.EventFamily),
		ReplyToText:       envelope.ReplyToText,
		ReplyToAuthorID:   envelope.ReplyToAuthorID,
		ReplyToAuthorName: envelope.ReplyToAuthorName,
		IdempotencyKey:    envelope.IdempotencyKey,
	}
	if envelope.Attachments != nil {
		converted.Attachments = make([]bridgecontract.MessageAttachment, len(envelope.Attachments))
		for index, attachment := range envelope.Attachments {
			converted.Attachments[index] = bridgecontract.MessageAttachment{
				ID: attachment.ID, Name: attachment.Name, MIMEType: attachment.MIMEType, URL: attachment.URL,
			}
		}
	}
	converted.ProviderMetadata = slices.Clone(envelope.ProviderMetadata)
	if envelope.Command != nil {
		command := inboundCommandToContract(*envelope.Command)
		converted.Command = &command
	}
	if envelope.Action != nil {
		action := inboundActionToContract(*envelope.Action)
		converted.Action = &action
	}
	if envelope.Reaction != nil {
		reaction := inboundReactionToContract(*envelope.Reaction)
		converted.Reaction = &reaction
	}
	if envelope.Edit != nil {
		edit := inboundEditToContract(*envelope.Edit)
		converted.Edit = &edit
	}
	if envelope.Conversation != nil {
		conversation := networkConversationRefToContract(*envelope.Conversation)
		converted.Conversation = &conversation
	}
	return converted
}
