package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func mapSlackSlashCommand(
	values url.Values,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (slackMappedInbound, error) {
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	command := strings.TrimSpace(values.Get("command"))
	channelID := strings.TrimSpace(values.Get(providerChannelIDKey))
	userID := normalizeSlackUserID(values.Get("user_id"))
	if command == "" || channelID == "" || userID == "" {
		return slackMappedInbound{}, errors.New("slack: slash command requires command, channel_id, and user_id")
	}
	direct := isSlackSlashCommandDirect(values.Get(providerChannelNameKey), channelID)
	user := slackUserIdentity{
		ID:          userID,
		Username:    normalizeUsername(values.Get("user_name")),
		DisplayName: firstNonEmpty(normalizeUsername(values.Get("user_name")), userID),
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
		EventFamily: bridgepkg.InboundEventFamilyCommand,
		Command: &bridgepkg.InboundCommand{
			Command:   command,
			Text:      strings.TrimSpace(values.Get("text")),
			TriggerID: strings.TrimSpace(values.Get(providerTriggerIDKey)),
		},
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(values.Get(providerTriggerIDKey)),
			fmt.Sprintf("slack:%s:command:%s:%s:%s", managed.Instance.ID, channelID, userID, command),
		),
	}
	if direct {
		envelope.PeerID = channelID
		envelope.ThreadID = slackDirectRootThreadID(channelID)
	} else {
		envelope.GroupID = channelID
		envelope.ThreadID = slackDirectRootThreadID(channelID)
	}
	metadata, err := json.Marshal(map[string]any{
		providerChannelIDKey:   channelID,
		providerChannelNameKey: strings.TrimSpace(values.Get(providerChannelNameKey)),
		providerResponseURLKey: strings.TrimSpace(values.Get(providerResponseURLKey)),
		providerTeamIDKey:      strings.TrimSpace(values.Get(providerTeamIDKey)),
		providerTriggerIDKey:   strings.TrimSpace(values.Get(providerTriggerIDKey)),
		providerTypeKey:        "slash_command",
	})
	if err != nil {
		return slackMappedInbound{}, fmt.Errorf("slack: encode slash command metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return slackMappedInbound{}, err
	}
	return slackMappedInbound{Envelope: envelope, Direct: direct, User: user}, nil
}

func mapSlackBlockActions(
	payload slackBlockActionsPayload,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) ([]slackMappedInbound, error) {
	if receivedAt.IsZero() {
		receivedAt = time.Now().UTC()
	}
	ctx, err := newSlackBlockActionContext(payload, managed, receivedAt)
	if err != nil {
		return nil, err
	}
	items := make([]slackMappedInbound, 0, len(payload.Actions))
	for idx := range payload.Actions {
		item, err := buildSlackBlockActionItem(ctx, payload, payload.Actions[idx])
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

type slackBlockActionContext struct {
	managed    subprocess.InitializeBridgeManagedInstance
	receivedAt time.Time
	channelID  string
	messageTS  string
	threadTS   string
	messageID  string
	threadID   string
	direct     bool
	user       slackUserIdentity
}

func newSlackBlockActionContext(
	payload slackBlockActionsPayload,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
) (slackBlockActionContext, error) {
	if len(payload.Actions) == 0 {
		return slackBlockActionContext{}, errors.New("slack: block actions payload requires at least one action")
	}
	channelID := firstNonEmpty(strings.TrimSpace(payload.Channel.ID), strings.TrimSpace(payload.Container.ChannelID))
	messageTS := firstNonEmpty(strings.TrimSpace(payload.Message.TS), strings.TrimSpace(payload.Container.MessageTS))
	threadTS := firstNonEmpty(
		strings.TrimSpace(payload.Message.ThreadTS),
		strings.TrimSpace(payload.Container.ThreadTS),
	)
	userID := normalizeSlackUserID(payload.User.ID)
	if channelID == "" || messageTS == "" || userID == "" {
		return slackBlockActionContext{}, errors.New(
			"slack: block actions payload requires channel, message ts, and user id",
		)
	}
	direct := isSlackDirectConversation("", channelID)
	user := slackUserIdentity{
		ID:       userID,
		Username: normalizeUsername(firstNonEmpty(payload.User.Username, payload.User.Name)),
		DisplayName: firstNonEmpty(
			strings.TrimSpace(payload.User.Name),
			strings.TrimSpace(payload.User.Username),
			userID,
		),
	}
	return slackBlockActionContext{
		managed:    managed,
		receivedAt: receivedAt,
		channelID:  channelID,
		messageTS:  messageTS,
		threadTS:   threadTS,
		messageID:  strings.TrimSpace(messageTS),
		threadID:   inboundSlackThreadID(direct, messageTS, threadTS),
		direct:     direct,
		user:       user,
	}, nil
}

func buildSlackBlockActionItem(
	ctx slackBlockActionContext,
	payload slackBlockActionsPayload,
	action slackBlockAction,
) (slackMappedInbound, error) {
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: ctx.managed.Instance.ID,
		Scope:            ctx.managed.Instance.Scope,
		WorkspaceID:      ctx.managed.Instance.WorkspaceID,
		ReceivedAt:       ctx.receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          ctx.user.ID,
			Username:    ctx.user.Username,
			DisplayName: ctx.user.DisplayName,
		},
		EventFamily: bridgepkg.InboundEventFamilyAction,
		Action: &bridgepkg.InboundAction{
			ActionID:  strings.TrimSpace(action.ActionID),
			MessageID: ctx.messageID,
			Value:     slackBlockActionValue(action),
			TriggerID: strings.TrimSpace(payload.TriggerID),
		},
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(action.ActionTS),
			fmt.Sprintf(
				"slack:%s:action:%s:%s:%s",
				ctx.managed.Instance.ID,
				ctx.messageTS,
				ctx.user.ID,
				strings.TrimSpace(action.ActionID),
			),
		),
	}
	if ctx.direct {
		envelope.PeerID = ctx.channelID
		envelope.ThreadID = ctx.threadID
	} else {
		envelope.GroupID = ctx.channelID
		envelope.ThreadID = ctx.threadID
	}
	metadata, err := slackBlockActionMetadata(payload, action, ctx)
	if err != nil {
		return slackMappedInbound{}, fmt.Errorf("slack: encode block action metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return slackMappedInbound{}, err
	}
	return slackMappedInbound{Envelope: envelope, Direct: ctx.direct, User: ctx.user}, nil
}

func slackBlockActionValue(action slackBlockAction) string {
	value := strings.TrimSpace(action.Value)
	if action.SelectedOption != nil && strings.TrimSpace(action.SelectedOption.Value) != "" {
		return strings.TrimSpace(action.SelectedOption.Value)
	}
	return value
}

func slackBlockActionMetadata(
	payload slackBlockActionsPayload,
	action slackBlockAction,
	ctx slackBlockActionContext,
) ([]byte, error) {
	return json.Marshal(map[string]any{
		"action_ts":            strings.TrimSpace(action.ActionTS),
		"block_id":             strings.TrimSpace(action.BlockID),
		providerChannelIDKey:   ctx.channelID,
		"container":            strings.TrimSpace(payload.Container.Type),
		"is_ephemeral":         payload.Container.IsEphemeral,
		"message_ts":           ctx.messageTS,
		providerResponseURLKey: strings.TrimSpace(payload.ResponseURL),
		"thread_ts":            strings.TrimSpace(ctx.threadTS),
		providerTriggerIDKey:   strings.TrimSpace(payload.TriggerID),
		providerTypeKey:        strings.TrimSpace(action.Type),
	})
}

func mapSlackReactionEvent(
	event slackReactionEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	eventID string,
	teamID string,
) (slackMappedInbound, error) {
	if strings.TrimSpace(event.Item.Type) != providerMessageKey {
		return slackMappedInbound{}, errors.New("slack: reaction event item.type must be message")
	}
	if strings.TrimSpace(event.Item.Channel) == "" || strings.TrimSpace(event.Item.TS) == "" ||
		strings.TrimSpace(event.Reaction) == "" ||
		strings.TrimSpace(event.User) == "" {
		return slackMappedInbound{}, errors.New(
			"slack: reaction event requires item channel, item ts, reaction, and user",
		)
	}
	receivedAt = slackReactionReceivedAt(event, receivedAt)
	direct := isSlackDirectConversation("", event.Item.Channel)
	user := slackUserIdentity{
		ID:          normalizeSlackUserID(event.User),
		DisplayName: normalizeSlackUserID(event.User),
	}
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			DisplayName: user.DisplayName,
		},
		EventFamily: bridgepkg.InboundEventFamilyReaction,
		Reaction: &bridgepkg.InboundReaction{
			MessageID: strings.TrimSpace(event.Item.TS),
			Emoji:     normalizeSlackEmoji(event.Reaction),
			RawEmoji:  strings.TrimSpace(event.Reaction),
			Added:     strings.TrimSpace(event.Type) == providerReactionAddedKey,
		},
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(event.EventTS),
			strings.TrimSpace(eventID),
			fmt.Sprintf(
				"slack:%s:reaction:%s:%s:%s:%s",
				managed.Instance.ID,
				strings.TrimSpace(event.Item.Channel),
				strings.TrimSpace(event.Item.TS),
				user.ID,
				strings.TrimSpace(event.Reaction),
			),
		),
	}
	if direct {
		envelope.PeerID = strings.TrimSpace(event.Item.Channel)
		envelope.ThreadID = strings.TrimSpace(event.Item.TS)
	} else {
		envelope.GroupID = strings.TrimSpace(event.Item.Channel)
		envelope.ThreadID = strings.TrimSpace(event.Item.TS)
	}
	metadata, err := json.Marshal(map[string]any{
		providerChannelIDKey: strings.TrimSpace(event.Item.Channel),
		"event_id":           strings.TrimSpace(eventID),
		"event_ts":           strings.TrimSpace(event.EventTS),
		"item_user":          strings.TrimSpace(event.ItemUser),
		"message_ts":         strings.TrimSpace(event.Item.TS),
		providerTeamIDKey:    strings.TrimSpace(teamID),
		providerTypeKey:      strings.TrimSpace(event.Type),
	})
	if err != nil {
		return slackMappedInbound{}, fmt.Errorf("slack: encode reaction metadata: %w", err)
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return slackMappedInbound{}, err
	}
	return slackMappedInbound{Envelope: envelope, Direct: direct, User: user}, nil
}

func slackReactionReceivedAt(event slackReactionEvent, fallback time.Time) time.Time {
	if fallback.IsZero() {
		fallback = time.Now().UTC()
	}
	if strings.TrimSpace(event.EventTS) == "" {
		return fallback
	}
	parsed, err := parseSlackTimestamp(strings.TrimSpace(event.EventTS))
	if err != nil {
		return fallback
	}
	return parsed
}

func allowSlackDirectMessage(cfg *resolvedInstanceConfig, user slackUserIdentity, direct bool) bool {
	if !direct {
		return true
	}
	if cfg == nil {
		return false
	}

	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return slackIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	case bridgepkg.BridgeDMPolicyPairing:
		if slackIdentityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, user) {
			return true
		}
		return slackIdentityAllowed(cfg.allowUserIDs, cfg.allowUsernames, user)
	default:
		return false
	}
}

func slackIdentityAllowed(ids map[string]struct{}, usernames map[string]struct{}, user slackUserIdentity) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[normalizeSlackUserID(user.ID)]; ok {
		return true
	}
	if _, ok := usernames[normalizeUsername(user.Username)]; ok {
		return true
	}
	return false
}
