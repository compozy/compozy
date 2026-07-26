package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func resolveGChatDeliveryTarget(event bridgepkg.DeliveryEvent) (gchatResolvedTarget, error) {
	if thread := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
	); thread != "" {
		if decoded, err := decodeGChatThreadID(thread); err == nil && strings.TrimSpace(decoded.SpaceName) != "" {
			return gchatResolvedTarget{
				SpaceName:  strings.TrimSpace(decoded.SpaceName),
				ThreadName: strings.TrimSpace(decoded.ThreadName),
			}, nil
		}
	}

	spaceName := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.DeliveryTarget.GroupID),
		strings.TrimSpace(event.RoutingKey.PeerID),
		strings.TrimSpace(event.RoutingKey.GroupID),
	)
	if spaceName == "" {
		return gchatResolvedTarget{}, errors.New("gchat: delivery target requires peer_id or group_id")
	}
	return gchatResolvedTarget{SpaceName: spaceName}, nil
}

func decodePubSubMessage(push gchatPubSubPushMessage) (gchatWorkspaceEventNotification, error) {
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(push.Message.Data))
	if err != nil {
		return gchatWorkspaceEventNotification{}, fmt.Errorf("gchat: decode pubsub payload: %w", err)
	}
	payload := struct {
		Message  *gchatMessage  `json:"message,omitempty"`
		Reaction *gchatReaction `json:"reaction,omitempty"`
	}{}
	if err := json.Unmarshal(data, &payload); err != nil {
		return gchatWorkspaceEventNotification{}, fmt.Errorf("gchat: decode pubsub notification payload: %w", err)
	}
	attributes := push.Message.Attributes
	return gchatWorkspaceEventNotification{
		Subscription:   strings.TrimSpace(push.Subscription),
		TargetResource: strings.TrimSpace(attributes["ce-subject"]),
		EventType:      strings.TrimSpace(attributes["ce-type"]),
		EventTime:      firstNonEmpty(attributes["ce-time"], push.Message.PublishTime),
		Message:        payload.Message,
		Reaction:       payload.Reaction,
	}, nil
}

func detectGChatWebhookShape(body []byte) string {
	probe := gchatWebhookProbe{}
	if err := json.Unmarshal(body, &probe); err != nil {
		return ""
	}
	if strings.TrimSpace(probe.Subscription) != "" && strings.TrimSpace(probe.Message.Data) != "" {
		return gchatModePubSub
	}
	if probe.Chat != nil {
		return gchatModeDirect
	}
	return ""
}

func normalizeReceivedAt(fallback time.Time, value string) time.Time {
	if strings.TrimSpace(value) == "" {
		if fallback.IsZero() {
			return time.Now().UTC()
		}
		return fallback.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed.UTC()
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback.UTC()
}

func normalizeGChatText(message gchatMessage) string {
	text := firstNonEmpty(message.ArgumentText, message.Text, message.FormattedText)
	return strings.TrimSpace(text)
}

func normalizeGChatAttachments(items []gchatAttachment) []bridgepkg.MessageAttachment {
	attachments := make([]bridgepkg.MessageAttachment, 0, len(items))
	for _, item := range items {
		attachment := bridgepkg.MessageAttachment{
			ID:       strings.TrimSpace(item.Name),
			Name:     strings.TrimSpace(firstNonEmpty(item.ContentName, item.Name)),
			MIMEType: strings.TrimSpace(item.ContentType),
			URL:      strings.TrimSpace(item.DownloadURI),
		}
		if attachment.ID == "" && attachment.Name == "" && attachment.MIMEType == "" && attachment.URL == "" {
			continue
		}
		attachments = append(attachments, attachment)
	}
	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func gchatSender(user gchatUser) bridgepkg.MessageSender {
	displayName := strings.TrimSpace(user.DisplayName)
	username := normalizeUsername(firstNonEmpty(strings.TrimSpace(user.Email), displayName))
	if username == "" {
		username = normalizeUsername(strings.TrimPrefix(strings.TrimSpace(user.Name), "users/"))
	}
	return bridgepkg.MessageSender{
		ID:          strings.TrimSpace(user.Name),
		Username:    username,
		DisplayName: displayName,
	}
}

func isDirectSpace(space gchatSpace) bool {
	return strings.EqualFold(strings.TrimSpace(space.Type), "DM") ||
		strings.EqualFold(strings.TrimSpace(space.SpaceType), "DIRECT_MESSAGE")
}

func isBotUser(user gchatUser) bool {
	return strings.EqualFold(strings.TrimSpace(user.Type), "BOT")
}

func threadNameForMessage(message gchatMessage, direct bool) string {
	if direct {
		return ""
	}
	if message.Thread != nil && strings.TrimSpace(message.Thread.Name) != "" {
		return strings.TrimSpace(message.Thread.Name)
	}
	return strings.TrimSpace(message.Name)
}

func paramValue(params map[string]string, key string) string {
	if len(params) == 0 {
		return ""
	}
	return strings.TrimSpace(params[key])
}

func derefUser(user *gchatUser) gchatUser {
	if user == nil {
		return gchatUser{}
	}
	return *user
}

func reactionEmoji(reaction *gchatReaction) string {
	if reaction == nil || reaction.Emoji == nil {
		return ""
	}
	return strings.TrimSpace(reaction.Emoji.Unicode)
}

func normalizeGChatEmoji(value string) string {
	return strings.TrimSpace(value)
}

func extractReactionMessageName(reactionName string) string {
	matches := reactionMessagePattern.FindStringSubmatch(strings.TrimSpace(reactionName))
	if len(matches) != 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func encodeGChatThreadID(ref gchatThreadRef) string {
	space := strings.TrimSpace(ref.SpaceName)
	thread := strings.TrimSpace(ref.ThreadName)
	if space == "" {
		return ""
	}
	encodedThread := ""
	if thread != "" {
		encodedThread = ":" + base64.RawURLEncoding.EncodeToString([]byte(thread))
	}
	dmSuffix := ""
	if ref.IsDM {
		dmSuffix = ":dm"
	}
	return "gchat:" + space + encodedThread + dmSuffix
}

func decodeGChatThreadID(value string) (gchatThreadRef, error) {
	trimmed := strings.TrimSpace(value)
	isDM := strings.HasSuffix(trimmed, ":dm")
	if isDM {
		trimmed = strings.TrimSuffix(trimmed, ":dm")
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) < 2 || parts[0] != providerGchatKey {
		return gchatThreadRef{}, errors.New("gchat: invalid thread id")
	}
	ref := gchatThreadRef{
		SpaceName: strings.TrimSpace(parts[1]),
		IsDM:      isDM,
	}
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(parts[2]))
		if err != nil {
			return gchatThreadRef{}, err
		}
		ref.ThreadName = string(decoded)
	}
	return ref, nil
}
