package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func discordUserIdentityFromInteraction(interaction discordInteraction) (discordUserIdentity, error) {
	if interaction.Member != nil && interaction.Member.User != nil {
		return discordUserIdentity{
			ID:       normalizeDiscordUserID(interaction.Member.User.ID),
			Username: normalizeUsername(interaction.Member.User.Username),
			DisplayName: firstNonEmpty(
				strings.TrimSpace(interaction.Member.User.GlobalName),
				strings.TrimSpace(interaction.Member.User.Username),
				normalizeDiscordUserID(interaction.Member.User.ID),
			),
		}, nil
	}
	if interaction.User != nil {
		return discordUserIdentity{
			ID:       normalizeDiscordUserID(interaction.User.ID),
			Username: normalizeUsername(interaction.User.Username),
			DisplayName: firstNonEmpty(
				strings.TrimSpace(interaction.User.GlobalName),
				strings.TrimSpace(interaction.User.Username),
				normalizeDiscordUserID(interaction.User.ID),
			),
		}, nil
	}
	return discordUserIdentity{}, errors.New("discord: interaction is missing a user")
}

func discordRouteIdentity(
	guildID string,
	channelID string,
	parentID string,
	channelType int,
) (string, string, string, bool, error) {
	channelID = strings.TrimSpace(channelID)
	parentID = strings.TrimSpace(parentID)
	if channelID == "" {
		return "", "", "", false, errors.New("discord: channel id is required")
	}

	if isDiscordThreadChannel(channelType) {
		groupID := firstNonEmpty(parentID, channelID)
		return "", groupID, channelID, false, nil
	}
	if strings.TrimSpace(guildID) == "" {
		if channelType == discordChannelTypeGroupDM {
			return "", channelID, "", false, nil
		}
		return channelID, "", "", true, nil
	}
	return "", channelID, "", false, nil
}

func isDiscordThreadChannel(channelType int) bool {
	switch channelType {
	case discordChannelTypeAnnouncementThread, discordChannelTypePublicThread, discordChannelTypePrivateThread:
		return true
	default:
		return false
	}
}

func channelIDFromInteraction(channel *discordInteractionChannel) string {
	if channel == nil {
		return ""
	}
	return strings.TrimSpace(channel.ID)
}

func parentIDFromInteraction(channel *discordInteractionChannel) string {
	if channel == nil {
		return ""
	}
	return strings.TrimSpace(channel.ParentID)
}

func channelTypeFromInteraction(channel *discordInteractionChannel) int {
	if channel == nil {
		return 0
	}
	return channel.Type
}

func messageIDFromInteraction(message *discordInteractionMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.ID)
}

func parseDiscordCommand(name string, options []discordInteractionOption) (string, string) {
	commandParts := []string{strings.TrimSpace(name)}
	if commandParts[0] == "" {
		commandParts[0] = "/"
	} else if !strings.HasPrefix(commandParts[0], "/") {
		commandParts[0] = "/" + commandParts[0]
	}

	valueParts := make([]string, 0)
	var walk func([]discordInteractionOption)
	walk = func(items []discordInteractionOption) {
		for _, item := range items {
			switch item.Type {
			case discordApplicationCommandOptionTypeSubcommand, discordApplicationCommandOptionTypeSubcommandGroup:
				if trimmed := strings.TrimSpace(item.Name); trimmed != "" {
					commandParts = append(commandParts, trimmed)
				}
				walk(item.Options)
			default:
				if item.Value == nil {
					continue
				}
				text := strings.TrimSpace(fmt.Sprint(item.Value))
				if text != "" {
					valueParts = append(valueParts, text)
				}
			}
		}
	}
	walk(options)

	return strings.Join(commandParts, " "), strings.Join(valueParts, " ")
}

func normalizeDiscordAttachments(items []discordAttachment) []bridgepkg.MessageAttachment {
	attachments := make([]bridgepkg.MessageAttachment, 0, len(items))
	for _, item := range items {
		attachment := bridgepkg.MessageAttachment{
			ID:       strings.TrimSpace(item.ID),
			Name:     strings.TrimSpace(item.Filename),
			MIMEType: strings.TrimSpace(item.ContentType),
			URL:      strings.TrimSpace(item.URL),
		}
		if attachment.ID == "" && attachment.Name == "" && attachment.URL == "" {
			continue
		}
		attachments = append(attachments, attachment)
	}
	return attachments
}

func normalizeDiscordEmoji(emoji discordEmoji) string {
	if strings.TrimSpace(emoji.Name) == "" {
		return ""
	}
	if strings.TrimSpace(emoji.ID) == "" {
		return ":" + strings.Trim(strings.TrimSpace(emoji.Name), ":") + ":"
	}
	return "<:" + strings.TrimSpace(emoji.Name) + ":" + strings.TrimSpace(emoji.ID) + ">"
}

func rawDiscordEmoji(emoji discordEmoji) string {
	if strings.TrimSpace(emoji.ID) == "" {
		return strings.TrimSpace(emoji.Name)
	}
	return strings.TrimSpace(emoji.Name) + ":" + strings.TrimSpace(emoji.ID)
}

func discordUsernameFromMember(member *discordInteractionMember) string {
	if member == nil || member.User == nil {
		return ""
	}
	return strings.TrimSpace(member.User.Username)
}

func discordGlobalNameFromMember(member *discordInteractionMember) string {
	if member == nil || member.User == nil {
		return ""
	}
	return strings.TrimSpace(member.User.GlobalName)
}

func normalizeDiscordUserID(value string) string {
	return strings.TrimSpace(value)
}

func buildDiscordIDSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeDiscordUserID(value); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func buildDiscordUsernameSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if normalized := normalizeUsername(value); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func normalizeUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseDiscordReceivedAt(value string, fallback time.Time) time.Time {
	if strings.TrimSpace(value) == "" {
		if fallback.IsZero() {
			return time.Now().UTC()
		}
		return fallback
	}
	if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value)); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value)); err == nil {
		return parsed.UTC()
	}
	if ts, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil {
		return time.Unix(ts, 0).UTC()
	}
	if fallback.IsZero() {
		return time.Now().UTC()
	}
	return fallback
}

func writeDiscordInteractionResponse(w http.ResponseWriter, responseType int) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	return json.NewEncoder(w).Encode(map[string]int{providerTypeKey: responseType})
}

func writeWebhookNoContent(w http.ResponseWriter) error {
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func deliveryStateKey(instanceID string, deliveryID string) string {
	return strings.TrimSpace(instanceID) + ":" + strings.TrimSpace(deliveryID)
}

func encodeRemoteMessageID(channelID string, messageID string) string {
	return strings.TrimSpace(channelID) + ":" + strings.TrimSpace(messageID)
}

func decodeRemoteMessageID(value string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("discord: invalid remote message id %q", value)
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
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	nanoseconds := seconds * float64(time.Second)
	const maxDuration = time.Duration(1<<63 - 1)
	if nanoseconds > float64(maxDuration) {
		return 0
	}
	return time.Duration(math.Ceil(nanoseconds))
}

func normalizeWebhookPath(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "/" + trimmed
	}
	return trimmed
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
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
