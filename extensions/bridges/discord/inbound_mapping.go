package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func mapDiscordMessageEvent(
	event discordMessageEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	eventID string,
) (discordMappedInbound, bool, error) {
	if strings.TrimSpace(event.ChannelID) == "" || strings.TrimSpace(event.ID) == "" {
		return discordMappedInbound{}, false, errors.New("discord: message event requires channel_id and id")
	}
	if event.Author.Bot {
		return discordMappedInbound{}, true, nil
	}

	receivedAt = parseDiscordReceivedAt(event.Timestamp, receivedAt)
	user := discordUserIdentity{
		ID:       normalizeDiscordUserID(event.Author.ID),
		Username: normalizeUsername(event.Author.Username),
		DisplayName: firstNonEmpty(
			strings.TrimSpace(event.Author.GlobalName),
			strings.TrimSpace(event.Author.Username),
			normalizeDiscordUserID(event.Author.ID),
		),
	}
	peerID, groupID, threadID, direct, err := discordRouteIdentity(
		event.GuildID,
		event.ChannelID,
		event.ParentID,
		event.ChannelType,
	)
	if err != nil {
		return discordMappedInbound{}, false, err
	}

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		PlatformMessageID: strings.TrimSpace(event.ID),
		ReceivedAt:        receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		},
		Content: bridgepkg.MessageContent{
			Text: strings.TrimSpace(event.Content),
		},
		Attachments: normalizeDiscordAttachments(event.Attachments),
		EventFamily: bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(eventID),
			fmt.Sprintf("discord:%s:message:%s", managed.Instance.ID, strings.TrimSpace(event.ID)),
		),
		PeerID:   peerID,
		GroupID:  groupID,
		ThreadID: threadID,
	}
	metadata, err := json.Marshal(map[string]any{
		providerChannelIDKey:   strings.TrimSpace(event.ChannelID),
		providerChannelTypeKey: event.ChannelType,
		"event_id":             strings.TrimSpace(eventID),
		providerGuildIDKey:     strings.TrimSpace(event.GuildID),
		providerMessageIDKey:   strings.TrimSpace(event.ID),
		providerParentIDKey:    strings.TrimSpace(event.ParentID),
	})
	if err != nil {
		return discordMappedInbound{}, false, fmt.Errorf("discord: encode message metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return discordMappedInbound{}, false, err
	}
	return discordMappedInbound{Envelope: envelope, Direct: direct, User: user}, false, nil
}

func mapDiscordInteractionCommand(
	interaction discordInteraction,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (discordMappedInbound, error) {
	if strings.TrimSpace(interaction.ID) == "" || interaction.Data == nil {
		return discordMappedInbound{}, errors.New("discord: command interaction requires id and data")
	}

	user, err := discordUserIdentityFromInteraction(interaction)
	if err != nil {
		return discordMappedInbound{}, err
	}
	peerID, groupID, threadID, direct, err := discordRouteIdentity(
		interaction.GuildID,
		firstNonEmpty(interaction.ChannelID, channelIDFromInteraction(interaction.Channel)),
		parentIDFromInteraction(interaction.Channel),
		channelTypeFromInteraction(interaction.Channel),
	)
	if err != nil {
		return discordMappedInbound{}, err
	}
	command, text := parseDiscordCommand(interaction.Data.Name, interaction.Data.Options)
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		},
		PeerID:      peerID,
		GroupID:     groupID,
		ThreadID:    threadID,
		EventFamily: bridgepkg.InboundEventFamilyCommand,
		Command: &bridgepkg.InboundCommand{
			Command:   command,
			Text:      text,
			TriggerID: strings.TrimSpace(interaction.Token),
		},
		IdempotencyKey: strings.TrimSpace(interaction.ID),
	}
	metadata, err := json.Marshal(map[string]any{
		"application_id":       strings.TrimSpace(interaction.ApplicationID),
		providerChannelIDKey:   firstNonEmpty(interaction.ChannelID, channelIDFromInteraction(interaction.Channel)),
		providerChannelTypeKey: channelTypeFromInteraction(interaction.Channel),
		providerGuildIDKey:     strings.TrimSpace(interaction.GuildID),
		"interaction_id":       strings.TrimSpace(interaction.ID),
		"kind":                 "application_command",
	})
	if err != nil {
		return discordMappedInbound{}, fmt.Errorf("discord: encode command metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return discordMappedInbound{}, err
	}
	return discordMappedInbound{Envelope: envelope, Direct: direct, User: user}, nil
}

func mapDiscordInteractionAction(
	interaction discordInteraction,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (discordMappedInbound, error) {
	if strings.TrimSpace(interaction.ID) == "" || interaction.Data == nil {
		return discordMappedInbound{}, errors.New("discord: action interaction requires id and data")
	}
	if strings.TrimSpace(interaction.Data.CustomID) == "" {
		return discordMappedInbound{}, errors.New("discord: action interaction requires custom_id")
	}

	user, err := discordUserIdentityFromInteraction(interaction)
	if err != nil {
		return discordMappedInbound{}, err
	}
	peerID, groupID, threadID, direct, err := discordRouteIdentity(
		interaction.GuildID,
		firstNonEmpty(interaction.ChannelID, channelIDFromInteraction(interaction.Channel)),
		parentIDFromInteraction(interaction.Channel),
		channelTypeFromInteraction(interaction.Channel),
	)
	if err != nil {
		return discordMappedInbound{}, err
	}
	value := strings.TrimSpace(interaction.Data.CustomID)
	if len(interaction.Data.Values) > 0 && strings.TrimSpace(interaction.Data.Values[0]) != "" {
		value = strings.TrimSpace(interaction.Data.Values[0])
	}
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		},
		PeerID:      peerID,
		GroupID:     groupID,
		ThreadID:    threadID,
		EventFamily: bridgepkg.InboundEventFamilyAction,
		Action: &bridgepkg.InboundAction{
			ActionID:  strings.TrimSpace(interaction.Data.CustomID),
			MessageID: messageIDFromInteraction(interaction.Message),
			Value:     value,
			TriggerID: strings.TrimSpace(interaction.Token),
		},
		IdempotencyKey: strings.TrimSpace(interaction.ID),
	}
	metadata, err := json.Marshal(map[string]any{
		"application_id":       strings.TrimSpace(interaction.ApplicationID),
		providerChannelIDKey:   firstNonEmpty(interaction.ChannelID, channelIDFromInteraction(interaction.Channel)),
		providerChannelTypeKey: channelTypeFromInteraction(interaction.Channel),
		"component_type":       interaction.Data.ComponentType,
		providerGuildIDKey:     strings.TrimSpace(interaction.GuildID),
		"interaction_id":       strings.TrimSpace(interaction.ID),
		"kind":                 "message_component",
	})
	if err != nil {
		return discordMappedInbound{}, fmt.Errorf("discord: encode action metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return discordMappedInbound{}, err
	}
	return discordMappedInbound{Envelope: envelope, Direct: direct, User: user}, nil
}

func mapDiscordReactionEvent(
	event discordReactionEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	eventID string,
	eventType string,
) (discordMappedInbound, error) {
	if strings.TrimSpace(event.ChannelID) == "" || strings.TrimSpace(event.MessageID) == "" ||
		strings.TrimSpace(event.UserID) == "" {
		return discordMappedInbound{}, errors.New(
			"discord: reaction event requires channel_id, message_id, and user_id",
		)
	}

	receivedAt = parseDiscordReceivedAt(event.Timestamp, receivedAt)
	user := discordUserIdentity{
		ID:       normalizeDiscordUserID(event.UserID),
		Username: normalizeUsername(discordUsernameFromMember(event.Member)),
		DisplayName: firstNonEmpty(
			discordGlobalNameFromMember(event.Member),
			discordUsernameFromMember(event.Member),
			normalizeDiscordUserID(event.UserID),
		),
	}
	peerID, groupID, threadID, direct, err := discordRouteIdentity(
		event.GuildID,
		event.ChannelID,
		event.ParentID,
		event.ChannelType,
	)
	if err != nil {
		return discordMappedInbound{}, err
	}
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		},
		PeerID:      peerID,
		GroupID:     groupID,
		ThreadID:    threadID,
		EventFamily: bridgepkg.InboundEventFamilyReaction,
		Reaction: &bridgepkg.InboundReaction{
			MessageID: strings.TrimSpace(event.MessageID),
			Emoji:     normalizeDiscordEmoji(event.Emoji),
			RawEmoji:  rawDiscordEmoji(event.Emoji),
			Added:     strings.TrimSpace(eventType) == providerMessageReactionAddValue,
		},
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(eventID),
			fmt.Sprintf(
				"discord:%s:reaction:%s:%s:%s:%s",
				managed.Instance.ID,
				strings.TrimSpace(event.ChannelID),
				strings.TrimSpace(event.MessageID),
				strings.TrimSpace(event.UserID),
				rawDiscordEmoji(event.Emoji),
			),
		),
	}
	metadata, err := json.Marshal(map[string]any{
		providerChannelIDKey:   strings.TrimSpace(event.ChannelID),
		providerChannelTypeKey: event.ChannelType,
		"event_id":             strings.TrimSpace(eventID),
		"event_type":           strings.TrimSpace(eventType),
		providerGuildIDKey:     strings.TrimSpace(event.GuildID),
		providerParentIDKey:    strings.TrimSpace(event.ParentID),
	})
	if err != nil {
		return discordMappedInbound{}, fmt.Errorf("discord: encode reaction metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return discordMappedInbound{}, err
	}
	return discordMappedInbound{Envelope: envelope, Direct: direct, User: user}, nil
}

func allowDiscordDirectMessage(cfg *resolvedInstanceConfig, user discordUserIdentity, direct bool) bool {
	if cfg == nil {
		return false
	}
	if !direct {
		return true
	}

	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return discordIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	case bridgepkg.BridgeDMPolicyPairing:
		if discordIdentityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, user) {
			return true
		}
		return discordIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	default:
		return false
	}
}

func discordIdentityAllowed(ids map[string]struct{}, usernames map[string]struct{}, user discordUserIdentity) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[normalizeDiscordUserID(user.ID)]; ok {
		return true
	}
	if _, ok := usernames[normalizeUsername(user.Username)]; ok {
		return true
	}
	return false
}
