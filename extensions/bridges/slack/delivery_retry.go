package main

import (
	"context"

	"github.com/compozy/agh/internal/bridgesdk"
)

func postSlackDeliveryMessage(
	ctx context.Context,
	api slackAPI,
	request slackPostMessageRequest,
) (*slackPostedMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*slackPostedMessage, error) {
			message, err := api.PostMessage(callCtx, request)
			if err != nil {
				return nil, err
			}
			return validateSlackPostedMutation(message)
		},
	)
}

func updateSlackDeliveryMessage(
	ctx context.Context,
	api slackAPI,
	request slackUpdateMessageRequest,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.UpdateMessage(callCtx, request)
	})
	return err
}

func deleteSlackDeliveryMessage(
	ctx context.Context,
	api slackAPI,
	request slackDeleteMessageRequest,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.DeleteMessage(callCtx, request)
	})
	return err
}

func findSlackDeliveryMessage(
	ctx context.Context,
	reconciler slackDeliveryReconciler,
	request slackFindDeliveryMessageRequest,
) (*slackPostedMessage, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*slackPostedMessage, error) {
			return reconciler.FindDeliveryMessage(callCtx, request)
		},
	)
}
