package main

import (
	"context"

	"github.com/compozy/agh/internal/bridgesdk"
)

func postDiscordDeliveryMessage(
	ctx context.Context,
	api discordAPI,
	request discordPostMessageRequest,
) (*discordPostedMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*discordPostedMessage, error) {
			return api.PostMessage(callCtx, request)
		},
	)
}

func updateDiscordDeliveryMessage(
	ctx context.Context,
	api discordAPI,
	request discordUpdateMessageRequest,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.UpdateMessage(callCtx, request)
	})
	return err
}

func deleteDiscordDeliveryMessage(
	ctx context.Context,
	api discordAPI,
	request discordDeleteMessageRequest,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.DeleteMessage(callCtx, request)
	})
	return err
}
