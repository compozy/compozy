package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/agh/internal/bridgesdk"
)

type telegramBotClient struct {
	baseURL               string
	botToken              string
	httpClient            *http.Client
	reportResponseCleanup func(error)
}

func (c *telegramBotClient) GetMe(ctx context.Context) (*telegramBotIdentity, error) {
	var result telegramBotIdentity
	if err := c.call(ctx, "getMe", map[string]any{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *telegramBotClient) SendMessage(
	ctx context.Context,
	req telegramSendMessageRequest,
) (*telegramSentMessage, error) {
	var result telegramSentMessage
	if err := c.callMutation(ctx, "sendMessage", req, &result); err != nil {
		return nil, err
	}
	return validatedTelegramSentMessage(&result)
}

func (c *telegramBotClient) EditMessageText(
	ctx context.Context,
	req telegramEditMessageTextRequest,
) error {
	var result json.RawMessage
	return c.callMutation(ctx, "editMessageText", req, &result)
}

func (c *telegramBotClient) DeleteMessage(
	ctx context.Context,
	req telegramDeleteMessageRequest,
) error {
	var result bool
	return c.callMutation(ctx, "deleteMessage", req, &result)
}

func (c *telegramBotClient) call(
	ctx context.Context,
	method string,
	payload any,
	result any,
) error {
	return c.callTelegram(ctx, method, payload, result, bridgesdk.HTTPResponseNoCommit)
}

func (c *telegramBotClient) callMutation(
	ctx context.Context,
	method string,
	payload any,
	result any,
) error {
	return c.callTelegram(ctx, method, payload, result, bridgesdk.HTTPResponseCommitOnSuccessStatus)
}

func validatedTelegramSentMessage(message *telegramSentMessage) (*telegramSentMessage, error) {
	if message == nil {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("telegram: send message returned no response"))
	}
	if message.MessageID <= 0 {
		return nil, bridgesdk.MarkCommittedMutation(
			fmt.Errorf("telegram: send message returned invalid message id %d", message.MessageID),
		)
	}
	return message, nil
}
