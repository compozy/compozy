package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/agh/internal/bridgesdk"
)

func (c *slackBotClient) AuthTest(ctx context.Context) (*slackAuthIdentity, error) {
	var result slackAuthIdentity
	var headers http.Header
	if err := c.callSlack(
		ctx,
		"auth.test",
		map[string]any{},
		&result,
		&headers,
		bridgesdk.HTTPResponseNoCommit,
	); err != nil {
		return nil, err
	}
	result.GrantedScopes = splitSlackScopes(headers.Get("X-OAuth-Scopes"))
	return &result, nil
}

func (c *slackBotClient) PostMessage(ctx context.Context, req slackPostMessageRequest) (*slackPostedMessage, error) {
	var result slackPostedMessage
	if err := c.callMutation(ctx, "chat.postMessage", req, &result); err != nil {
		return nil, err
	}
	return validateSlackPostedMutation(&result)
}

func (c *slackBotClient) FindDeliveryMessage(
	ctx context.Context,
	req slackFindDeliveryMessageRequest,
) (*slackPostedMessage, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	method := "conversations.history"
	payload := slackConversationMessagesRequest{
		Channel: strings.TrimSpace(req.Channel),
		Limit:   100,
	}
	if strings.TrimSpace(req.ThreadTS) != "" {
		method = "conversations.replies"
		payload.TS = strings.TrimSpace(req.ThreadTS)
		payload.Inclusive = true
	}

	for {
		var result slackConversationMessagesResponse
		if err := c.call(ctx, method, payload, &result); err != nil {
			return nil, err
		}
		for idx := range result.Messages {
			message := result.Messages[idx]
			if slackMetadataMatchesDelivery(message.Metadata, req) {
				return &slackPostedMessage{TS: strings.TrimSpace(message.TS)}, nil
			}
		}
		nextCursor := ""
		if result.ResponseMetadata != nil {
			nextCursor = strings.TrimSpace(result.ResponseMetadata.NextCursor)
		}
		if !result.HasMore && nextCursor == "" {
			break
		}
		payload.Cursor = nextCursor
	}
	return nil, nil
}

func (c *slackBotClient) UpdateMessage(ctx context.Context, req slackUpdateMessageRequest) error {
	var result slackPostedMessage
	return c.callMutation(ctx, "chat.update", req, &result)
}

func (c *slackBotClient) DeleteMessage(ctx context.Context, req slackDeleteMessageRequest) error {
	var result json.RawMessage
	return c.callMutation(ctx, "chat.delete", req, &result)
}

func validateSlackPostedMutation(message *slackPostedMessage) (*slackPostedMessage, error) {
	if message == nil {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("slack: post message returned no response"))
	}
	timestamp := strings.TrimSpace(message.TS)
	if timestamp == "" {
		return nil, bridgesdk.MarkCommittedMutation(errors.New("slack: post message returned an empty timestamp"))
	}
	message.TS = timestamp
	return message, nil
}

func slackMetadataMatchesDelivery(
	metadata *slackMessageMetadata,
	req slackFindDeliveryMessageRequest,
) bool {
	if metadata == nil {
		return false
	}
	if strings.TrimSpace(metadata.EventType) != providerAghBridgeDeliveryKey {
		return false
	}
	return strings.TrimSpace(metadata.EventPayload.DeliveryID) == strings.TrimSpace(req.DeliveryID) &&
		strings.TrimSpace(metadata.EventPayload.BridgeInstanceID) == strings.TrimSpace(req.BridgeInstanceID)
}

func (c *slackBotClient) call(ctx context.Context, method string, payload any, result any) error {
	return c.callSlack(ctx, method, payload, result, nil, bridgesdk.HTTPResponseNoCommit)
}

func (c *slackBotClient) callMutation(ctx context.Context, method string, payload any, result any) error {
	return c.callSlack(ctx, method, payload, result, nil, bridgesdk.HTTPResponseCommitOnSuccessStatus)
}

func classifySlackAPIError(status int, errorText string, retryAfter time.Duration) error {
	trimmed := strings.TrimSpace(errorText)
	lowered := strings.ToLower(trimmed)
	switch {
	case status == http.StatusTooManyRequests || lowered == "ratelimited":
		return &bridgesdk.RateLimitError{
			Err:        fmt.Errorf("slack api rate limited: %s", firstNonEmpty(trimmed, "ratelimited")),
			RetryAfter: retryAfter,
		}
	case status == http.StatusUnauthorized, status == http.StatusForbidden,
		lowered == "invalid_auth", lowered == "not_authed", lowered == "account_inactive",
		lowered == "token_revoked", lowered == "missing_scope":
		return &bridgesdk.AuthError{Err: fmt.Errorf("slack api auth failed: %s", firstNonEmpty(trimmed, "auth failed"))}
	case status == http.StatusGatewayTimeout, status == http.StatusRequestTimeout, lowered == "request_timeout":
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusGatewayTimeout,
			Message:    fmt.Sprintf("slack api timeout: %s", firstNonEmpty(trimmed, "request_timeout")),
		}
	case status >= http.StatusInternalServerError:
		return &bridgesdk.HTTPError{
			StatusCode: status,
			Message: fmt.Sprintf(
				"slack api server failure: %s",
				firstNonEmpty(trimmed, "service unavailable"),
			),
			RetryAfter: retryAfter,
		}
	case lowered == "internal_error",
		lowered == "fatal_error",
		lowered == "service_unavailable":
		return &bridgesdk.TransientError{
			Err: fmt.Errorf("slack api transient failure: %s", firstNonEmpty(trimmed, "service unavailable")),
		}
	case trimmed != "":
		return &bridgesdk.PermanentError{Err: fmt.Errorf("slack api error: %s", trimmed)}
	default:
		return &bridgesdk.HTTPError{
			StatusCode: maxInt(status, http.StatusInternalServerError),
			Message: fmt.Sprintf(
				"slack api request failed with status %d",
				maxInt(status, http.StatusInternalServerError),
			),
		}
	}
}
