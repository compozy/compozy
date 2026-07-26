package main

import (
	"context"
	"encoding/json"
	"errors"

	"net/http"

	"strings"

	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *linearProvider) handleLinearCommentWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	payload := linearCommentWebhookPayload{}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid linear comment payload"}
	}

	cfg, ok, err := selectLinearConfig(candidates, payload.OrganizationID, linearModeComments)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if !ok {
		return writeWebhookText(w, http.StatusOK, "ignored")
	}
	if strings.TrimSpace(payload.Action) != providerCreateKey {
		return writeWebhookText(w, http.StatusOK, "ok")
	}

	mapped, ignored, err := mapLinearCommentCreated(payload, *cfg.managed, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if ignored || cfg.dedup.Mark(mapped.Envelope.IdempotencyKey) || linearCommentIsSelf(cfg, payload) {
		return writeWebhookText(w, http.StatusOK, "ok")
	}
	if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, mapped.Envelope); err != nil {
		return linearWebhookDispatchHTTPError(err)
	}
	return writeWebhookText(w, http.StatusOK, "ok")
}

func (p *linearProvider) handleLinearAgentSessionWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	payload := linearAgentSessionWebhookPayload{}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid linear agent session payload",
		}
	}

	cfg, ok, err := selectLinearConfig(candidates, payload.OrganizationID, linearModeAgentSessions)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if !ok {
		return writeWebhookText(w, http.StatusOK, "ignored")
	}

	mapped, ignored, err := mapLinearAgentSessionEvent(payload, cfg.managed, request.ReceivedAt, cfg.botUserID)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if ignored || cfg.dedup.Mark(mapped.Envelope.IdempotencyKey) {
		return writeWebhookText(w, http.StatusOK, "ok")
	}
	if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, mapped.Envelope); err != nil {
		return linearWebhookDispatchHTTPError(err)
	}
	return writeWebhookText(w, http.StatusOK, "ok")
}

func linearWebhookDispatchHTTPError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusGatewayTimeout,
			Message:    "linear: webhook ingestion timed out",
		}
	case errors.Is(err, context.Canceled):
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "linear: webhook ingestion canceled",
		}
	default:
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
	}
}
