package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func resolveTeamsInboundContext(
	activity teamsActivity,
	cfg resolvedInstanceConfig,
	receivedAt time.Time,
	kind string,
) (teamsInboundContext, error) {
	serviceURL := firstNonEmpty(normalizeURL(activity.ServiceURL), cfg.serviceURL)
	if serviceURL == "" {
		return teamsInboundContext{}, fmt.Errorf("teams: %s requires serviceUrl", kind)
	}
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	if parsed := parseTeamsTimestamp(activity.Timestamp); !parsed.IsZero() {
		receivedAt = parsed
	}
	conversationID := strings.TrimSpace(activity.Conversation.ID)
	if conversationID == "" {
		return teamsInboundContext{}, fmt.Errorf("teams: %s requires conversation.id", kind)
	}

	return teamsInboundContext{
		receivedAt:         receivedAt,
		serviceURL:         serviceURL,
		conversationID:     conversationID,
		direct:             isTeamsDirectConversation(activity.Conversation),
		baseConversationID: baseTeamsConversationID(conversationID),
		threadID: encodeTeamsThreadID(teamsThreadRef{
			ConversationID: conversationID,
			ServiceURL:     serviceURL,
		}),
		user: teamsUserIdentity{
			ID:       normalizeTeamsID(activity.From.ID),
			Username: normalizeTeamsUsername(activity.From.Name),
			DisplayName: firstNonEmpty(
				strings.TrimSpace(activity.From.Name),
				normalizeTeamsID(activity.From.ID),
			),
		},
	}, nil
}

func buildTeamsActionEnvelope(
	cfg resolvedInstanceConfig,
	inboundContext teamsInboundContext,
	activity teamsActivity,
	payload teamsActionPayload,
	source string,
	messageID string,
) bridgepkg.InboundMessageEnvelope {
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: cfg.instanceID,
		Scope:            cfg.managed.Instance.Scope,
		WorkspaceID:      cfg.managed.Instance.WorkspaceID,
		ReceivedAt:       inboundContext.receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          inboundContext.user.ID,
			Username:    inboundContext.user.Username,
			DisplayName: inboundContext.user.DisplayName,
		},
		EventFamily: bridgepkg.InboundEventFamilyAction,
		Action: &bridgepkg.InboundAction{
			ActionID:  strings.TrimSpace(payload.ActionID),
			MessageID: messageID,
			Value:     strings.TrimSpace(payload.Value),
		},
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(activity.ID),
			fmt.Sprintf(
				"teams:%s:action:%s:%s",
				cfg.instanceID,
				messageID,
				strings.TrimSpace(payload.ActionID),
			),
		),
		ThreadID: inboundContext.threadID,
	}
	if inboundContext.direct {
		envelope.PeerID = inboundContext.baseConversationID
	} else {
		envelope.GroupID = inboundContext.baseConversationID
	}
	metadata, err := json.Marshal(map[string]any{
		providerActivityIDKey:         strings.TrimSpace(activity.ID),
		"action_id":                   strings.TrimSpace(payload.ActionID),
		providerBaseConversationIDKey: inboundContext.baseConversationID,
		providerConversationIDKey:     inboundContext.conversationID,
		"message_id":                  messageID,
		providerServiceURLKey:         inboundContext.serviceURL,
		"source":                      source,
		providerTenantIDKey:           extractTeamsTenantID(activity),
	})
	if err == nil {
		envelope.ProviderMetadata = metadata
	}
	return envelope
}

func buildTeamsMessageEnvelope(
	cfg resolvedInstanceConfig,
	inboundContext teamsInboundContext,
	activity teamsActivity,
) bridgepkg.InboundMessageEnvelope {
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  cfg.instanceID,
		Scope:             cfg.managed.Instance.Scope,
		WorkspaceID:       cfg.managed.Instance.WorkspaceID,
		PlatformMessageID: strings.TrimSpace(activity.ID),
		ReceivedAt:        inboundContext.receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          inboundContext.user.ID,
			Username:    inboundContext.user.Username,
			DisplayName: inboundContext.user.DisplayName,
		},
		Content: bridgepkg.MessageContent{
			Text: normalizeTeamsText(activity.Text),
		},
		Attachments: normalizeTeamsAttachments(activity.Attachments),
		EventFamily: bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: fmt.Sprintf(
			"teams:%s:message:%s",
			cfg.instanceID,
			strings.TrimSpace(activity.ID),
		),
		ThreadID: inboundContext.threadID,
	}
	if inboundContext.direct {
		envelope.PeerID = inboundContext.baseConversationID
	} else {
		envelope.GroupID = inboundContext.baseConversationID
	}
	metadata, err := json.Marshal(map[string]any{
		providerActivityIDKey:         strings.TrimSpace(activity.ID),
		providerBaseConversationIDKey: inboundContext.baseConversationID,
		"channel_id":                  strings.TrimSpace(activity.ChannelID),
		providerConversationIDKey:     inboundContext.conversationID,
		"conversation_type":           strings.TrimSpace(activity.Conversation.ConversationType),
		"reply_to_id":                 strings.TrimSpace(activity.ReplyToID),
		providerServiceURLKey:         inboundContext.serviceURL,
		providerTenantIDKey:           extractTeamsTenantID(activity),
		"type":                        strings.TrimSpace(activity.Type),
	})
	if err == nil {
		envelope.ProviderMetadata = metadata
	}
	return envelope
}

func mapTeamsReactionItem(
	cfg resolvedInstanceConfig,
	inboundContext teamsInboundContext,
	activity teamsActivity,
	messageID string,
	reaction teamsMessageReaction,
	added bool,
) (mappedTeamsInbound, bool, error) {
	raw := strings.TrimSpace(reaction.Type)
	if raw == "" {
		return mappedTeamsInbound{}, false, nil
	}

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: cfg.instanceID,
		Scope:            cfg.managed.Instance.Scope,
		WorkspaceID:      cfg.managed.Instance.WorkspaceID,
		ReceivedAt:       inboundContext.receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          inboundContext.user.ID,
			Username:    inboundContext.user.Username,
			DisplayName: inboundContext.user.DisplayName,
		},
		EventFamily: bridgepkg.InboundEventFamilyReaction,
		Reaction: &bridgepkg.InboundReaction{
			MessageID: messageID,
			Emoji:     normalizeTeamsEmoji(raw),
			RawEmoji:  raw,
			Added:     added,
		},
		IdempotencyKey: fmt.Sprintf(
			"teams:%s:reaction:%s:%s:%t",
			cfg.instanceID,
			messageID,
			raw,
			added,
		),
		ThreadID: inboundContext.threadID,
	}
	if inboundContext.direct {
		envelope.PeerID = inboundContext.baseConversationID
	} else {
		envelope.GroupID = inboundContext.baseConversationID
	}
	metadata, err := json.Marshal(map[string]any{
		providerActivityIDKey:         strings.TrimSpace(activity.ID),
		providerBaseConversationIDKey: inboundContext.baseConversationID,
		providerConversationIDKey:     inboundContext.conversationID,
		"message_id":                  messageID,
		providerServiceURLKey:         inboundContext.serviceURL,
		providerTenantIDKey:           extractTeamsTenantID(activity),
		"type":                        strings.TrimSpace(activity.Type),
	})
	if err == nil {
		envelope.ProviderMetadata = metadata
	}
	if err := envelope.Validate(); err != nil {
		return mappedTeamsInbound{}, false, err
	}
	return mappedTeamsInbound{
		Envelope: envelope,
		Direct:   inboundContext.direct,
		User:     inboundContext.user,
	}, true, nil
}
