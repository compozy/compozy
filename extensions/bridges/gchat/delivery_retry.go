package main

import (
	"context"

	"github.com/compozy/agh/internal/bridgesdk"
)

func createGChatDeliveryMessage(
	ctx context.Context,
	api gchatAPI,
	request gchatCreateMessageRequest,
) (*gchatSentMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*gchatSentMessage, error) {
			return api.CreateMessage(callCtx, request)
		},
	)
}

func updateGChatDeliveryMessage(
	ctx context.Context,
	api gchatAPI,
	request gchatUpdateMessageRequest,
) (*gchatSentMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*gchatSentMessage, error) {
			return api.UpdateMessage(callCtx, request)
		},
	)
}

func deleteGChatDeliveryMessage(ctx context.Context, api gchatAPI, messageName string) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.DeleteMessage(callCtx, messageName)
	})
	return err
}
