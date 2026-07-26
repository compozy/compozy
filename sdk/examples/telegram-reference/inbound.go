package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

type telegramUpdate struct {
	BridgeInstanceID  string           `json:"bridge_instance_id,omitempty"`
	UpdateID          int64            `json:"update_id"`
	Message           *telegramMessage `json:"message,omitempty"`
	ChannelPost       *telegramMessage `json:"channel_post,omitempty"`
	EditedMessage     *telegramMessage `json:"edited_message,omitempty"`
	EditedChannelPost *telegramMessage `json:"edited_channel_post,omitempty"`
}

type telegramMessage struct {
	MessageID       int64            `json:"message_id"`
	MessageThreadID int64            `json:"message_thread_id,omitempty"`
	Date            int64            `json:"date"`
	EditDate        int64            `json:"edit_date,omitempty"`
	Chat            telegramChat     `json:"chat"`
	From            telegramUser     `json:"from"`
	SenderChat      *telegramChat    `json:"sender_chat,omitempty"`
	Text            string           `json:"text,omitempty"`
	Caption         string           `json:"caption,omitempty"`
	ReplyToMessage  *telegramMessage `json:"reply_to_message,omitempty"`
}

type telegramChat struct {
	ID    int64  `json:"id"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

type telegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type telegramUpdatePayload struct {
	message *telegramMessage
	edited  bool
}

func mapTelegramUpdate(
	update telegramUpdate,
	bridgeRuntime subprocess.InitializeBridgeManagedInstance,
	now func() time.Time,
) (bridgepkg.InboundMessageEnvelope, error) {
	payload, err := update.payload()
	if err != nil {
		return bridgepkg.InboundMessageEnvelope{}, err
	}
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	message := payload.message
	receivedAt := now().UTC()
	if payload.edited && message.EditDate > 0 {
		receivedAt = time.Unix(message.EditDate, 0).UTC()
	} else if message.Date > 0 {
		receivedAt = time.Unix(message.Date, 0).UTC()
	}

	envelope := bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID: bridgeRuntime.Instance.ID,
		Scope:            bridgeRuntime.Instance.Scope,
		WorkspaceID:      bridgeRuntime.Instance.WorkspaceID,
		PeerID:           strconv.FormatInt(message.Chat.ID, 10),
		ThreadID:         optionalTelegramID(message.MessageThreadID),
		ReceivedAt:       receivedAt,
		Sender:           telegramInboundSender(*message),
		IdempotencyKey:   fmt.Sprintf("telegram:%s:%d", bridgeRuntime.Instance.ID, update.UpdateID),
	}
	envelope.ReplyToText, envelope.ReplyToAuthorID, envelope.ReplyToAuthorName =
		telegramReplyContext(message.ReplyToMessage)

	if payload.edited {
		envelope.EventFamily = bridgepkg.InboundEventFamilyEdit
		envelope.Edit = &bridgepkg.InboundEdit{
			MessageID:         strconv.FormatInt(message.MessageID, 10),
			NewText:           telegramMessageText(*message),
			OriginalTimestamp: telegramTimestamp(message.Date),
			Operation:         bridgepkg.InboundEditOperationUpdated,
		}
		return envelope, nil
	}

	envelope.EventFamily = bridgepkg.InboundEventFamilyMessage
	envelope.PlatformMessageID = strconv.FormatInt(message.MessageID, 10)
	envelope.Content = bridgepkg.MessageContent{Text: telegramMessageText(*message)}
	return envelope, nil
}

func (u telegramUpdate) payload() (telegramUpdatePayload, error) {
	candidates := []telegramUpdatePayload{
		{message: u.Message},
		{message: u.ChannelPost},
		{message: u.EditedMessage, edited: true},
		{message: u.EditedChannelPost, edited: true},
	}
	var selected telegramUpdatePayload
	count := 0
	for _, candidate := range candidates {
		if candidate.message == nil {
			continue
		}
		selected = candidate
		count++
	}
	if count != 1 {
		return telegramUpdatePayload{}, errors.New(
			"telegram-reference: update must contain exactly one supported message payload",
		)
	}
	return selected, nil
}

func telegramMessageText(message telegramMessage) string {
	if text := strings.TrimSpace(message.Text); text != "" {
		return text
	}
	return strings.TrimSpace(message.Caption)
}

func telegramInboundSender(message telegramMessage) bridgepkg.MessageSender {
	if message.From.ID != 0 || strings.TrimSpace(message.From.Username) != "" ||
		strings.TrimSpace(message.From.FirstName) != "" || strings.TrimSpace(message.From.LastName) != "" {
		return bridgepkg.MessageSender{
			ID:          optionalTelegramID(message.From.ID),
			Username:    strings.TrimSpace(message.From.Username),
			DisplayName: telegramUserDisplayName(message.From),
		}
	}
	if message.SenderChat != nil {
		return telegramChatSender(*message.SenderChat)
	}
	if strings.EqualFold(strings.TrimSpace(message.Chat.Type), "channel") {
		return telegramChatSender(message.Chat)
	}
	return bridgepkg.MessageSender{}
}

func telegramReplyContext(message *telegramMessage) (string, string, string) {
	if message == nil {
		return "", "", ""
	}
	sender := telegramInboundSender(*message)
	return telegramMessageText(*message), sender.ID, sender.DisplayName
}

func telegramChatSender(chat telegramChat) bridgepkg.MessageSender {
	return bridgepkg.MessageSender{
		ID:          optionalTelegramID(chat.ID),
		DisplayName: strings.TrimSpace(chat.Title),
	}
}

func telegramUserDisplayName(user telegramUser) string {
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}

func telegramTimestamp(unixSeconds int64) time.Time {
	if unixSeconds <= 0 {
		return time.Time{}
	}
	return time.Unix(unixSeconds, 0).UTC()
}

func optionalTelegramID(value int64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}
