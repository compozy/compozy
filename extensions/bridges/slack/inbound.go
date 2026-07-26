package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

const (
	slackMessageSubtypeChanged = "message_changed"
	slackMessageSubtypeDeleted = "message_deleted"
)

func mapSlackMessageEvent(
	event slackMessageEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	eventID string,
	teamID string,
	eventTime int64,
	parents *bridgesdk.ParentMessageCache,
) (slackMappedInbound, bool, error) {
	switch strings.TrimSpace(event.Subtype) {
	case slackMessageSubtypeChanged, slackMessageSubtypeDeleted:
		return mapSlackEditEvent(event, managed, receivedAt, eventID, teamID, eventTime, parents)
	}
	if strings.TrimSpace(event.Channel) == "" || strings.TrimSpace(event.TS) == "" {
		return slackMappedInbound{}, false, errors.New("slack: message event requires channel and ts")
	}
	if isIgnoredSlackMessageEvent(event) {
		return slackMappedInbound{}, true, nil
	}
	receivedAt = normalizeSlackReceivedAt(receivedAt, eventTime)

	direct := isSlackDirectConversation(event.ChannelType, event.Channel)
	threadID := inboundSlackThreadID(direct, event.TS, event.ThreadTS)
	user := slackInboundUser(event.User, event.Username)
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  managed.Instance.ID,
		Scope:             managed.Instance.Scope,
		WorkspaceID:       managed.Instance.WorkspaceID,
		PlatformMessageID: strings.TrimSpace(event.TS),
		ReceivedAt:        receivedAt,
		Sender: bridgepkg.MessageSender{
			ID:          user.ID,
			Username:    user.Username,
			DisplayName: user.DisplayName,
		},
		Content: bridgepkg.MessageContent{
			Text: strings.TrimSpace(event.Text),
		},
		Attachments: normalizeSlackAttachments(event.Files),
		EventFamily: bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(eventID),
			fmt.Sprintf(
				"slack:%s:message:%s:%s",
				managed.Instance.ID,
				strings.TrimSpace(event.Channel),
				strings.TrimSpace(event.TS),
			),
		),
	}
	assignSlackInboundRoute(&envelope, event.Channel, threadID, direct)
	parents.EnrichReply(&envelope, slackReplyParentMessageID(event.TS, event.ThreadTS))
	metadata, err := marshalSlackInboundMetadata(
		event.Channel,
		event.ChannelType,
		eventID,
		event.Subtype,
		firstNonEmpty(strings.TrimSpace(event.TeamID), strings.TrimSpace(teamID)),
		event.ThreadTS,
		event.TS,
		event.Type,
	)
	if err != nil {
		return slackMappedInbound{}, false, err
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return slackMappedInbound{}, false, err
	}
	return slackMappedInbound{Envelope: envelope, Direct: direct, User: user}, false, nil
}

func mapSlackEditEvent(
	event slackMessageEvent,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	eventID string,
	teamID string,
	eventTime int64,
	parents *bridgesdk.ParentMessageCache,
) (slackMappedInbound, bool, error) {
	channel := strings.TrimSpace(event.Channel)
	if channel == "" {
		return slackMappedInbound{}, false, errors.New("slack: edit event requires channel")
	}
	snapshot, operation, err := slackEditSnapshot(event)
	if err != nil {
		return slackMappedInbound{}, false, err
	}
	if strings.TrimSpace(snapshot.User) == "" || strings.TrimSpace(snapshot.BotID) != "" {
		return slackMappedInbound{}, true, nil
	}
	messageID := firstNonEmpty(strings.TrimSpace(event.DeletedTS), strings.TrimSpace(snapshot.TS))
	originalTimestamp, err := parseSlackTimestamp(messageID)
	if err != nil {
		return slackMappedInbound{}, false, fmt.Errorf("slack: parse edited message timestamp: %w", err)
	}
	receivedAt = normalizeSlackReceivedAt(receivedAt, eventTime)
	direct := isSlackDirectConversation(event.ChannelType, channel)
	threadID := inboundSlackThreadID(direct, messageID, snapshot.ThreadTS)
	user := slackInboundUser(snapshot.User, snapshot.Username)
	edit, supported := buildSlackInboundEdit(snapshot, operation, messageID, originalTimestamp)
	if !supported {
		return slackMappedInbound{}, true, nil
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
		EventFamily: bridgepkg.InboundEventFamilyEdit,
		Edit:        edit,
		IdempotencyKey: firstNonEmpty(
			strings.TrimSpace(eventID),
			fmt.Sprintf(
				"slack:%s:%s:%s:%s",
				managed.Instance.ID,
				strings.TrimSpace(event.Subtype),
				channel,
				messageID,
			),
		),
	}
	assignSlackInboundRoute(&envelope, channel, threadID, direct)
	parents.EnrichReply(&envelope, slackReplyParentMessageID(messageID, snapshot.ThreadTS))
	metadata, err := marshalSlackInboundMetadata(
		channel,
		event.ChannelType,
		eventID,
		event.Subtype,
		firstNonEmpty(strings.TrimSpace(event.TeamID), strings.TrimSpace(teamID)),
		snapshot.ThreadTS,
		messageID,
		event.Type,
	)
	if err != nil {
		return slackMappedInbound{}, false, err
	}
	envelope.ProviderMetadata = metadata
	if err := envelope.Validate(); err != nil {
		return slackMappedInbound{}, false, err
	}
	return slackMappedInbound{Envelope: envelope, Direct: direct, User: user}, false, nil
}

func buildSlackInboundEdit(
	snapshot slackMessageSnapshot,
	operation bridgepkg.InboundEditOperation,
	messageID string,
	originalTimestamp time.Time,
) (*bridgepkg.InboundEdit, bool) {
	edit := &bridgepkg.InboundEdit{
		MessageID:         messageID,
		OriginalTimestamp: originalTimestamp,
		Operation:         operation,
	}
	if operation != bridgepkg.InboundEditOperationUpdated {
		return edit, true
	}
	edit.NewText = strings.TrimSpace(snapshot.Text)
	return edit, edit.NewText != ""
}

func slackEditSnapshot(
	event slackMessageEvent,
) (slackMessageSnapshot, bridgepkg.InboundEditOperation, error) {
	switch strings.TrimSpace(event.Subtype) {
	case slackMessageSubtypeChanged:
		if event.Message == nil {
			return slackMessageSnapshot{}, "", errors.New("slack: message_changed requires message")
		}
		return *event.Message, bridgepkg.InboundEditOperationUpdated, nil
	case slackMessageSubtypeDeleted:
		if event.PreviousMessage == nil {
			return slackMessageSnapshot{}, "", errors.New("slack: message_deleted requires previous_message")
		}
		return *event.PreviousMessage, bridgepkg.InboundEditOperationDeleted, nil
	default:
		return slackMessageSnapshot{}, "", fmt.Errorf("slack: unsupported edit subtype %q", event.Subtype)
	}
}

func shouldBatchSlackInbound(event slackMessageEvent) bool {
	switch strings.TrimSpace(event.Subtype) {
	case slackMessageSubtypeChanged, slackMessageSubtypeDeleted:
		return false
	default:
		return slackReplyParentMessageID(event.TS, event.ThreadTS) == ""
	}
}

func slackEventReplyParentMessageID(event slackMessageEvent) string {
	switch strings.TrimSpace(event.Subtype) {
	case slackMessageSubtypeChanged:
		if event.Message == nil {
			return ""
		}
		return slackReplyParentMessageID(event.Message.TS, event.Message.ThreadTS)
	case slackMessageSubtypeDeleted:
		if event.PreviousMessage == nil {
			return ""
		}
		messageID := firstNonEmpty(strings.TrimSpace(event.DeletedTS), strings.TrimSpace(event.PreviousMessage.TS))
		return slackReplyParentMessageID(messageID, event.PreviousMessage.ThreadTS)
	default:
		return slackReplyParentMessageID(event.TS, event.ThreadTS)
	}
}

func slackReplyParentMessageID(messageID string, threadID string) string {
	messageID = strings.TrimSpace(messageID)
	threadID = strings.TrimSpace(threadID)
	if threadID == "" || threadID == messageID {
		return ""
	}
	return threadID
}

func slackInboundUser(userID string, username string) slackUserIdentity {
	normalizedID := normalizeSlackUserID(userID)
	return slackUserIdentity{
		ID:          normalizedID,
		Username:    normalizeUsername(username),
		DisplayName: firstNonEmpty(strings.TrimSpace(username), normalizedID),
	}
}

func assignSlackInboundRoute(
	envelope *bridgepkg.InboundMessageEnvelope,
	channel string,
	threadID string,
	direct bool,
) {
	if direct {
		envelope.PeerID = strings.TrimSpace(channel)
	} else {
		envelope.GroupID = strings.TrimSpace(channel)
	}
	envelope.ThreadID = strings.TrimSpace(threadID)
}

func normalizeSlackReceivedAt(receivedAt time.Time, eventTime int64) time.Time {
	if eventTime > 0 {
		return time.Unix(eventTime, 0).UTC()
	}
	if receivedAt.IsZero() {
		return time.Now().UTC()
	}
	return receivedAt.UTC()
}

func marshalSlackInboundMetadata(
	channel string,
	channelType string,
	eventID string,
	subtype string,
	teamID string,
	threadTS string,
	timestamp string,
	eventType string,
) (json.RawMessage, error) {
	metadata, err := json.Marshal(map[string]any{
		providerChannelIDKey: strings.TrimSpace(channel),
		"channel_type":       strings.TrimSpace(channelType),
		"event_id":           strings.TrimSpace(eventID),
		"subtype":            strings.TrimSpace(subtype),
		providerTeamIDKey:    strings.TrimSpace(teamID),
		"thread_ts":          strings.TrimSpace(threadTS),
		"ts":                 strings.TrimSpace(timestamp),
		providerTypeKey:      strings.TrimSpace(eventType),
	})
	if err != nil {
		return nil, fmt.Errorf("slack: marshal inbound metadata: %w", err)
	}
	return metadata, nil
}
