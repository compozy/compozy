package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func isIgnoredSlackMessageEvent(event slackMessageEvent) bool {
	if strings.TrimSpace(event.User) == "" {
		return true
	}
	if strings.TrimSpace(event.BotID) != "" {
		return true
	}
	ignoredSubtypes := map[string]struct{}{
		"bot_message":          {},
		"message_replied":      {},
		"channel_join":         {},
		"channel_leave":        {},
		"channel_topic":        {},
		"channel_purpose":      {},
		providerChannelNameKey: {},
		"group_join":           {},
		"group_leave":          {},
	}
	_, ignored := ignoredSubtypes[strings.TrimSpace(event.Subtype)]
	return ignored
}

func normalizeSlackAttachments(files []slackFile) []bridgepkg.MessageAttachment {
	if len(files) == 0 {
		return nil
	}
	attachments := make([]bridgepkg.MessageAttachment, 0, len(files))
	for _, file := range files {
		attachments = append(attachments, bridgepkg.MessageAttachment{
			ID:       strings.TrimSpace(file.ID),
			Name:     strings.TrimSpace(file.Name),
			MIMEType: strings.TrimSpace(file.MIMEType),
			URL:      strings.TrimSpace(file.URLPrivate),
		})
	}
	return attachments
}

func isSlackDirectConversation(channelType string, channelID string) bool {
	if strings.EqualFold(strings.TrimSpace(channelType), "im") {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(channelID), "D")
}

func isSlackSlashCommandDirect(channelName string, channelID string) bool {
	if strings.EqualFold(strings.TrimSpace(channelName), "directmessage") {
		return true
	}
	return isSlackDirectConversation("", channelID)
}

func inboundSlackThreadID(_ bool, ts string, threadTS string) string {
	return firstNonEmpty(strings.TrimSpace(threadTS), strings.TrimSpace(ts))
}

func slackDirectRootThreadID(channelID string) string {
	return strings.TrimSpace(channelID)
}

func parseSlackTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("slack: timestamp is required")
	}
	parts := strings.SplitN(trimmed, ".", 2)
	seconds, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	nanos := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		fraction := parts[1]
		if len(fraction) > 9 {
			fraction = fraction[:9]
		}
		for len(fraction) < 9 {
			fraction += "0"
		}
		nanos, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return time.Time{}, err
		}
	}
	return time.Unix(seconds, nanos).UTC(), nil
}

func normalizeSlackEmoji(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return ":" + strings.Trim(trimmed, ":") + ":"
}

func normalizeSlackUserID(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func normalizeUsername(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "@")
	return strings.ToLower(trimmed)
}

func buildSlackIDSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	ids := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeSlackUserID(value); normalized != "" {
			ids[normalized] = struct{}{}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func buildSlackUsernameSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	names := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeUsername(value); normalized != "" {
			names[normalized] = struct{}{}
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func encodeRemoteMessageID(channelID string, ts string) string {
	return strings.TrimSpace(channelID) + ":" + strings.TrimSpace(ts)
}

func decodeRemoteMessageID(value string) (string, string, error) {
	trimmed := strings.TrimSpace(value)
	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("slack: invalid remote message id %q", value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}

func referenceRemoteMessageID(reference *bridgepkg.DeliveryMessageReference) string {
	if reference == nil {
		return ""
	}
	return strings.TrimSpace(reference.RemoteMessageID)
}

func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func writeWebhookOK(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("OK"))
	return err
}

func normalizeWebhookPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.TrimRight(trimmed, "/")
}

func normalizeDeliveryEventType(value bridgepkg.DeliveryEventType) bridgepkg.DeliveryEventType {
	return bridgepkg.DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func maxInt(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
