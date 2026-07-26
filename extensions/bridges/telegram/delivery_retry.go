package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/bridgesdk"
)

func sendTelegramOutbound(
	ctx context.Context,
	api telegramAPI,
	request telegramSendMessageRequest,
	outbound telegramOutboundText,
) (*telegramSentMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*telegramSentMessage, error) {
			return sendTelegramOutboundOnce(callCtx, api, request, outbound)
		},
	)
}

func sendTelegramOutboundOnce(
	ctx context.Context,
	api telegramAPI,
	request telegramSendMessageRequest,
	outbound telegramOutboundText,
) (*telegramSentMessage, error) {
	request.Text = outbound.Text
	request.ParseMode = outbound.ParseMode
	sent, err := api.SendMessage(ctx, request)
	if err == nil {
		return validatedTelegramSentMessage(sent)
	}
	var parseErr *telegramMarkdownParseError
	if outbound.ParseMode == "" || bridgesdk.IsCommittedMutation(err) || !errors.As(err, &parseErr) {
		return nil, fmt.Errorf("telegram: send outbound text: %w", err)
	}
	request.Text = outbound.PlainText
	request.ParseMode = ""
	sent, err = api.SendMessage(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("telegram: send plain-text fallback: %w", err)
	}
	return validatedTelegramSentMessage(sent)
}

func editTelegramOutbound(
	ctx context.Context,
	api telegramAPI,
	request telegramEditMessageTextRequest,
	outbound telegramOutboundText,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, editTelegramOutboundOnce(callCtx, api, request, outbound)
	})
	return err
}

func editTelegramOutboundOnce(
	ctx context.Context,
	api telegramAPI,
	request telegramEditMessageTextRequest,
	outbound telegramOutboundText,
) error {
	request.Text = outbound.Text
	request.ParseMode = outbound.ParseMode
	err := api.EditMessageText(ctx, request)
	if err == nil {
		return nil
	}
	var parseErr *telegramMarkdownParseError
	if outbound.ParseMode == "" || bridgesdk.IsCommittedMutation(err) || !errors.As(err, &parseErr) {
		return fmt.Errorf("telegram: edit outbound text: %w", err)
	}
	request.Text = outbound.PlainText
	request.ParseMode = ""
	if err := api.EditMessageText(ctx, request); err != nil {
		return fmt.Errorf("telegram: edit plain-text fallback: %w", err)
	}
	return nil
}

func deleteTelegramDeliveryMessage(
	ctx context.Context,
	api telegramAPI,
	request telegramDeleteMessageRequest,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.DeleteMessage(callCtx, request)
	})
	return err
}
