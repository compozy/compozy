package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

func ensureTeamsConversation(
	ctx context.Context,
	api teamsAPI,
	cfg resolvedInstanceConfig,
	target teamsResolvedTarget,
) (teamsResolvedTarget, error) {
	if strings.TrimSpace(target.ConversationID) != "" {
		return target, nil
	}
	request := teamsCreateConversationRequest{
		Bot:      teamsChannelAccount{ID: cfg.appID},
		Members:  []teamsChannelAccount{{ID: target.UserID}},
		IsGroup:  false,
		TenantID: target.TenantID,
		ChannelData: map[string]any{
			"tenant": map[string]any{"id": target.TenantID},
		},
	}
	created, err := bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*teamsConversationResourceResponse, error) {
			return api.CreateConversation(callCtx, target.ServiceURL, request)
		},
	)
	if err != nil {
		return teamsResolvedTarget{}, fmt.Errorf("teams: create conversation: %w", err)
	}
	if created == nil || strings.TrimSpace(created.ID) == "" {
		return teamsResolvedTarget{}, &bridgesdk.CommittedMutationError{
			Err: errors.New("teams: create conversation response omitted id"),
		}
	}
	target.ConversationID = strings.TrimSpace(created.ID)
	return target, nil
}

func sendTeamsDeliveryActivity(
	ctx context.Context,
	api teamsAPI,
	serviceURL string,
	conversationID string,
	replyToID string,
	activity teamsOutboundActivity,
) (*teamsResourceResponse, error) {
	return bridgesdk.RetryDo(
		ctx,
		bridgesdk.DefaultRetryConfig(),
		func(callCtx context.Context) (*teamsResourceResponse, error) {
			return api.SendActivity(callCtx, serviceURL, conversationID, replyToID, activity)
		},
	)
}

func updateTeamsDeliveryActivity(
	ctx context.Context,
	api teamsAPI,
	serviceURL string,
	conversationID string,
	activityID string,
	activity teamsOutboundActivity,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.UpdateActivity(callCtx, serviceURL, conversationID, activityID, activity)
	})
	return err
}

func deleteTeamsDeliveryActivity(
	ctx context.Context,
	api teamsAPI,
	serviceURL string,
	conversationID string,
	activityID string,
) error {
	_, err := bridgesdk.RetryDo(ctx, bridgesdk.DefaultRetryConfig(), func(callCtx context.Context) (struct{}, error) {
		return struct{}{}, api.DeleteActivity(callCtx, serviceURL, conversationID, activityID)
	})
	return err
}
