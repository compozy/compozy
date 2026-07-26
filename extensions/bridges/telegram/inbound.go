package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func mapTelegramUpdate(
	update telegramUpdate,
	managed subprocess.InitializeBridgeManagedInstance,
	receivedAt time.Time,
	parents *bridgesdk.ParentMessageCache,
) (bridgepkg.InboundMessageEnvelope, error) {
	message := selectTelegramMessage(update)
	if message == nil {
		return bridgepkg.InboundMessageEnvelope{}, errors.New("telegram: message update is required")
	}
	edited := isTelegramEditUpdate(update)
	receivedAt = normalizeTelegramReceivedAt(receivedAt, *message, edited)
	text := telegramMessageText(*message)
	threadID := inboundThreadID(message.Chat, message.MessageThreadID)
	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: managed.Instance.ID,
		Scope:            managed.Instance.Scope,
		WorkspaceID:      managed.Instance.WorkspaceID,
		ReceivedAt:       receivedAt,
		Sender:           telegramMessageSender(*message),
		IdempotencyKey:   fmt.Sprintf("telegram:%s:%d", managed.Instance.ID, update.UpdateID),
	}
	if edited {
		envelope.EventFamily = bridgepkg.InboundEventFamilyEdit
		envelope.Edit = &bridgepkg.InboundEdit{
			MessageID:         strconv.FormatInt(message.MessageID, 10),
			NewText:           text,
			OriginalTimestamp: telegramOriginalTimestamp(message.Date),
			Operation:         bridgepkg.InboundEditOperationUpdated,
		}
	} else {
		envelope.PlatformMessageID = strconv.FormatInt(message.MessageID, 10)
		envelope.Content = bridgepkg.MessageContent{Text: text}
		envelope.EventFamily = bridgepkg.InboundEventFamilyMessage
	}
	assignTelegramInboundRoute(&envelope, message.Chat, threadID)
	applyTelegramReplyContext(&envelope, *message, parents)
	metadata, err := marshalTelegramInboundMetadata(update, *message, edited)
	if err != nil {
		return bridgepkg.InboundMessageEnvelope{}, err
	}
	envelope.ProviderMetadata = metadata
	return envelope, envelope.Validate()
}

func applyTelegramReplyContext(
	envelope *bridgepkg.InboundMessageEnvelope,
	message telegramMessage,
	parents *bridgesdk.ParentMessageCache,
) {
	parentMessageID := telegramReplyParentMessageID(message)
	if parentMessageID == "" {
		return
	}
	parents.EnrichReply(envelope, parentMessageID)
	if message.ReplyToMessage == nil {
		return
	}
	bridgesdk.ApplyParentMessage(envelope, telegramParentMessage(*message.ReplyToMessage))
}

func rememberTelegramReplyParent(
	parents *bridgesdk.ParentMessageCache,
	envelope bridgepkg.InboundMessageEnvelope,
	message telegramMessage,
) {
	if message.ReplyToMessage == nil {
		return
	}
	parents.RememberParent(
		envelope,
		telegramReplyParentMessageID(message),
		telegramParentMessage(*message.ReplyToMessage),
	)
}

func telegramParentMessage(message telegramMessage) bridgesdk.ParentMessage {
	sender := telegramMessageSender(message)
	return bridgesdk.ParentMessage{
		Text:       telegramMessageText(message),
		AuthorID:   sender.ID,
		AuthorName: firstNonEmpty(sender.DisplayName, sender.Username),
	}
}

func shouldBatchTelegramInbound(update telegramUpdate) bool {
	message := selectTelegramMessage(update)
	if message == nil || isTelegramEditUpdate(update) {
		return false
	}
	return telegramReplyParentMessageID(*message) == ""
}

func isTelegramEditUpdate(update telegramUpdate) bool {
	return update.EditedMessage != nil || update.EditedChannelPost != nil
}

func isUnsupportedTextlessTelegramEdit(update telegramUpdate) bool {
	message := selectTelegramMessage(update)
	return message != nil && isTelegramEditUpdate(update) && telegramMessageText(*message) == ""
}

func telegramMessageText(message telegramMessage) string {
	if text := strings.TrimSpace(message.Text); text != "" {
		return text
	}
	return strings.TrimSpace(message.Caption)
}

func telegramMessageSender(message telegramMessage) bridgepkg.MessageSender {
	if message.From.ID != 0 {
		return telegramUserSender(message.From)
	}
	if message.SenderChat != nil && message.SenderChat.ID != 0 {
		return telegramChatSender(*message.SenderChat)
	}
	if strings.EqualFold(strings.TrimSpace(message.Chat.Type), "channel") {
		return telegramChatSender(message.Chat)
	}
	return telegramUserSender(message.From)
}

func telegramUserSender(user telegramUser) bridgepkg.MessageSender {
	displayName := strings.TrimSpace(
		strings.TrimSpace(user.FirstName) + " " + strings.TrimSpace(user.LastName),
	)
	return bridgepkg.MessageSender{
		ID:          optionalTelegramID(user.ID),
		Username:    normalizeUsername(user.Username),
		DisplayName: displayName,
	}
}

func telegramChatSender(chat telegramChat) bridgepkg.MessageSender {
	return bridgepkg.MessageSender{
		ID:          optionalTelegramID(chat.ID),
		Username:    normalizeUsername(chat.Username),
		DisplayName: strings.TrimSpace(chat.Title),
	}
}

func assignTelegramInboundRoute(
	envelope *bridgepkg.InboundMessageEnvelope,
	chat telegramChat,
	threadID string,
) {
	if isDirectChat(chat.Type) {
		envelope.PeerID = strconv.FormatInt(chat.ID, 10)
	} else {
		envelope.GroupID = strconv.FormatInt(chat.ID, 10)
	}
	envelope.ThreadID = strings.TrimSpace(threadID)
}

func telegramReplyParentMessageID(message telegramMessage) string {
	if message.ReplyToMessage == nil || message.ReplyToMessage.MessageID == 0 {
		return ""
	}
	return strconv.FormatInt(message.ReplyToMessage.MessageID, 10)
}

func normalizeTelegramReceivedAt(
	receivedAt time.Time,
	message telegramMessage,
	edited bool,
) time.Time {
	if edited && message.EditDate > 0 {
		return time.Unix(message.EditDate, 0).UTC()
	}
	if !edited && message.Date > 0 {
		return time.Unix(message.Date, 0).UTC()
	}
	if receivedAt.IsZero() {
		return time.Now().UTC()
	}
	return receivedAt.UTC()
}

func telegramOriginalTimestamp(unixSeconds int64) time.Time {
	if unixSeconds <= 0 {
		return time.Time{}
	}
	return time.Unix(unixSeconds, 0).UTC()
}

func marshalTelegramInboundMetadata(
	update telegramUpdate,
	message telegramMessage,
	edited bool,
) (json.RawMessage, error) {
	replyToMessageID := int64(0)
	if message.ReplyToMessage != nil {
		replyToMessageID = message.ReplyToMessage.MessageID
	}
	metadata, err := json.Marshal(map[string]any{
		"chat_id":             message.Chat.ID,
		"chat_type":           strings.TrimSpace(message.Chat.Type),
		"edit_date":           message.EditDate,
		"edited":              edited,
		"is_forum":            message.Chat.IsForum,
		"message_id":          message.MessageID,
		"message_thread_id":   message.MessageThreadID,
		"reply_to_message_id": replyToMessageID,
		"update_id":           update.UpdateID,
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: marshal inbound metadata: %w", err)
	}
	return metadata, nil
}
