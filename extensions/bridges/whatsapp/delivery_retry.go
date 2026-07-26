package main

import (
	"context"

	"github.com/compozy/agh/internal/bridgesdk"
)

func sendWhatsAppDeliveryMessage(
	ctx context.Context,
	api whatsappAPI,
	phoneNumberID string,
	request whatsappSendMessageRequest,
) (*whatsappSendMessageResponse, error) {
	retryConfig := bridgesdk.DefaultRetryConfig()
	retryConfig.OnRetry = func(retryCtx context.Context, attempt bridgesdk.RetryAttempt) error {
		api.ReportRetry(retryCtx, attempt)
		return nil
	}
	return bridgesdk.RetryDo(
		ctx,
		retryConfig,
		func(callCtx context.Context) (*whatsappSendMessageResponse, error) {
			return api.SendTextMessage(callCtx, phoneNumberID, request)
		},
	)
}
