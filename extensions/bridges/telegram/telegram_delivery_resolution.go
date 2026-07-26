package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"net/http"

	"strconv"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func resolveDeliveryTarget(event bridgepkg.DeliveryEvent) (string, string, error) {
	chatID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.PeerID),
		strings.TrimSpace(event.DeliveryTarget.GroupID),
		strings.TrimSpace(event.RoutingKey.PeerID),
		strings.TrimSpace(event.RoutingKey.GroupID),
	)
	if chatID == "" {
		return "", "", errors.New("telegram: delivery target requires peer_id or group_id")
	}
	threadID := firstNonEmpty(
		strings.TrimSpace(event.DeliveryTarget.ThreadID),
		strings.TrimSpace(event.RoutingKey.ThreadID),
	)
	return chatID, threadID, nil
}

func resolveTelegramThreadID(threadID string, chatID string) (int64, error) {
	trimmedThreadID := strings.TrimSpace(threadID)
	if trimmedThreadID == "" {
		return 0, nil
	}
	if trimmedThreadID == telegramGeneralTopicID &&
		strings.HasPrefix(strings.TrimSpace(chatID), "-") {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmedThreadID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("telegram: invalid thread id %q: %w", trimmedThreadID, err)
	}
	return value, nil
}

func classifyTelegramHTTPError(
	statusCode int,
	response telegramAPIEnvelope[json.RawMessage],
) error {
	message := strings.TrimSpace(response.Description)
	if message == "" {
		message = fmt.Sprintf("telegram bot api error %d", maxInt(statusCode, response.ErrorCode))
	}
	retryAfter := time.Duration(response.Parameters.RetryAfter) * time.Second
	if statusCode == 0 {
		statusCode = response.ErrorCode
	}
	if statusCode == 429 {
		return &bridgesdk.RateLimitError{
			Err: &bridgesdk.HTTPError{
				StatusCode: statusCode,
				Message:    message,
				RetryAfter: retryAfter,
			},
			RetryAfter: retryAfter,
		}
	}
	if statusCode == 401 || statusCode == 403 {
		return &bridgesdk.AuthError{
			Err: &bridgesdk.HTTPError{StatusCode: statusCode, Message: message},
		}
	}
	httpErr := &bridgesdk.HTTPError{
		StatusCode: statusCode,
		Message:    message,
		RetryAfter: retryAfter,
	}
	if statusCode == http.StatusBadRequest && telegramMarkdownParseFailure(message) {
		return &telegramMarkdownParseError{err: httpErr}
	}
	return httpErr
}

func selectTelegramMessage(update telegramUpdate) *telegramMessage {
	switch {
	case update.Message != nil:
		return update.Message
	case update.EditedMessage != nil:
		return update.EditedMessage
	case update.ChannelPost != nil:
		return update.ChannelPost
	case update.EditedChannelPost != nil:
		return update.EditedChannelPost
	default:
		return nil
	}
}

func inboundThreadID(chat telegramChat, messageThreadID int64) string {
	if isDirectChat(chat.Type) {
		return optionalTelegramID(messageThreadID)
	}
	if chat.IsForum && messageThreadID == 0 {
		return telegramGeneralTopicID
	}
	return optionalTelegramID(messageThreadID)
}

func isDirectChat(chatType string) bool {
	return strings.EqualFold(strings.TrimSpace(chatType), "private")
}

func encodeRemoteMessageID(chatID string, messageID int64) string {
	return strings.TrimSpace(chatID) + ":" + strconv.FormatInt(messageID, 10)
}

func decodeRemoteMessageID(remoteMessageID string) (string, int64, error) {
	trimmed := strings.TrimSpace(remoteMessageID)
	index := strings.LastIndex(trimmed, ":")
	if index <= 0 || index == len(trimmed)-1 {
		return "", 0, fmt.Errorf("telegram: invalid remote message id %q", remoteMessageID)
	}
	messageID, err := strconv.ParseInt(trimmed[index+1:], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("telegram: parse remote message id %q: %w", remoteMessageID, err)
	}
	return trimmed[:index], messageID, nil
}
