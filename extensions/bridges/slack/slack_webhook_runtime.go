package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"net/http"
	"net/url"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *slackProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
	cfg, ok := p.configForPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler, err := bridgesdk.NewWebhookHandler(bridgesdk.WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json", "application/x-www-form-urlencoded"},
		MaxBodyBytes:        1 << 20,
		RateLimiter:         cfg.rateLimiter,
		InFlightLimiter:     cfg.inFlightLimiter,
		VerifySignature: func(ctx context.Context, req *http.Request, body []byte) error {
			return verifySlackSignature(ctx, req, body, cfg.signingSecret, p.now())
		},
		RequestKey: func(req *http.Request) string {
			return req.RemoteAddr + "|" + cfg.instanceID
		},
		Now: func() time.Time { return p.now() },
	}, func(w http.ResponseWriter, r *http.Request, request bridgesdk.WebhookRequest) error {
		return p.handleWebhookRequest(w, r, &cfg, request)
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		p.setLastError(err)
		return
	}
	handler.ServeHTTP(w, r)
}

func (p *slackProvider) handleWebhookRequest(
	w http.ResponseWriter,
	r *http.Request,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		return p.handleFormWebhook(r.Context(), w, cfg, request)
	}
	return p.handleJSONWebhook(r.Context(), w, cfg, request)
}

func (p *slackProvider) handleFormWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	values, err := url.ParseQuery(string(request.Body))
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack form payload"}
	}
	if values.Has("command") && !values.Has("payload") {
		mapped, err := mapSlackSlashCommand(values, *cfg.managed, request.ReceivedAt)
		if err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
		if cfg.dedup.Seen(mapped.Envelope.IdempotencyKey) {
			return writeWebhookOK(w)
		}
		if !allowSlackDirectMessage(cfg, mapped.User, mapped.Direct) {
			return writeWebhookOK(w)
		}
		if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, mapped.Envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
		cfg.dedup.Mark(mapped.Envelope.IdempotencyKey)
		return writeWebhookOK(w)
	}

	payloadStr := strings.TrimSpace(values.Get("payload"))
	if payloadStr == "" {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "missing slack interactive payload"}
	}
	var payload slackBlockActionsPayload
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack interactive payload"}
	}
	if strings.TrimSpace(payload.Type) != "block_actions" {
		return writeWebhookOK(w)
	}

	mapped, err := mapSlackBlockActions(payload, *cfg.managed, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	for _, item := range mapped {
		if cfg.dedup.Seen(item.Envelope.IdempotencyKey) {
			continue
		}
		if !allowSlackDirectMessage(cfg, item.User, item.Direct) {
			continue
		}
		if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, item.Envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
		cfg.dedup.Mark(item.Envelope.IdempotencyKey)
	}
	return writeWebhookOK(w)
}

func (p *slackProvider) handleJSONWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	var payload slackWebhookEnvelope
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack webhook payload"}
	}
	if handled, err := handleSlackJSONHandshake(w, payload); handled || err != nil {
		return err
	}
	if len(payload.Event) == 0 {
		return writeWebhookOK(w)
	}

	var eventType slackEventTypePayload
	if err := json.Unmarshal(payload.Event, &eventType); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack event payload"}
	}

	switch strings.TrimSpace(eventType.Type) {
	case providerMessageKey, "app_mention":
		return p.handleSlackMessageJSONEvent(ctx, w, cfg, request, payload)
	case providerReactionAddedKey, "reaction_removed":
		return p.handleSlackReactionJSONEvent(ctx, w, cfg, request, payload)
	default:
		return writeWebhookOK(w)
	}
}

func handleSlackJSONHandshake(w http.ResponseWriter, payload slackWebhookEnvelope) (bool, error) {
	switch strings.TrimSpace(payload.Type) {
	case "url_verification":
		if strings.TrimSpace(payload.Challenge) == "" {
			return true, &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "missing slack challenge"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return true, json.NewEncoder(w).Encode(map[string]string{"challenge": payload.Challenge})
	case "event_callback":
		return false, nil
	default:
		return true, writeWebhookOK(w)
	}
}

func (p *slackProvider) handleSlackMessageJSONEvent(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
	payload slackWebhookEnvelope,
) error {
	var event slackMessageEvent
	if err := json.Unmarshal(payload.Event, &event); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack message event"}
	}
	mapped, ignored, err := mapSlackMessageEvent(
		event,
		*cfg.managed,
		request.ReceivedAt,
		payload.EventID,
		payload.TeamID,
		payload.EventTime,
		nil,
	)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if ignored {
		return writeWebhookOK(w)
	}
	return p.dispatchSlackWebhookEnvelope(
		ctx,
		w,
		cfg,
		mapped,
		slackEventReplyParentMessageID(event),
		shouldBatchSlackInbound(event),
	)
}

func (p *slackProvider) handleSlackReactionJSONEvent(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
	payload slackWebhookEnvelope,
) error {
	var event slackReactionEvent
	if err := json.Unmarshal(payload.Event, &event); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid slack reaction event"}
	}
	mapped, err := mapSlackReactionEvent(event, *cfg.managed, request.ReceivedAt, payload.EventID, payload.TeamID)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	return p.dispatchSlackWebhookEnvelope(ctx, w, cfg, mapped, "", false)
}

func (p *slackProvider) dispatchSlackWebhookEnvelope(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	mapped slackMappedInbound,
	parentMessageID string,
	allowBatch bool,
) error {
	if cfg.dedup.Seen(mapped.Envelope.IdempotencyKey) {
		return writeWebhookOK(w)
	}
	if !allowSlackDirectMessage(cfg, mapped.User, mapped.Direct) {
		return writeWebhookOK(w)
	}
	if p.parents.EnrichReply(&mapped.Envelope, parentMessageID) {
		if err := mapped.Envelope.Validate(); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
	}
	if allowBatch && cfg.batcher != nil {
		if err := cfg.batcher.Enqueue(mapped.Envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
		p.parents.RememberEnvelope(mapped.Envelope)
		cfg.dedup.Mark(mapped.Envelope.IdempotencyKey)
		return writeWebhookOK(w)
	}
	if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, mapped.Envelope); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
	}
	p.parents.RememberEnvelope(mapped.Envelope)
	cfg.dedup.Mark(mapped.Envelope.IdempotencyKey)
	return writeWebhookOK(w)
}

func (p *slackProvider) dispatchInboundBatch(
	ctx context.Context,
	bridgeInstanceID string,
	batch bridgesdk.InboundBatch,
) error {
	if len(batch.Items) == 0 {
		return nil
	}
	merged := batch.Items[0]
	if len(batch.Items) > 1 {
		parts := make([]string, 0, len(batch.Items))
		for _, item := range batch.Items {
			if text := strings.TrimSpace(item.Content.Text); text != "" {
				parts = append(parts, text)
			}
		}
		merged.Content.Text = strings.Join(parts, "\n")
		merged.IdempotencyKey = fmt.Sprintf("%s:batch:%d", merged.IdempotencyKey, len(batch.Items))
	}
	return p.dispatchInboundEnvelope(ctx, bridgeInstanceID, merged)
}

func (p *slackProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("slack: runtime session is not initialized")
	}
	cfg, err := p.configForInstance(bridgeInstanceID)
	if err != nil {
		return err
	}

	_, err = p.lifecycle.Host().IngestBridgeMessage(ctx, session, envelope)
	if err != nil {
		return err
	}
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	}
	return nil
}

func (p *slackProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf("slack: delivery targeted unmanaged instance %q", instanceID)
	}
	return cfg, nil
}

func (p *slackProvider) waitForInstanceConfig(
	instanceID string,
	timeout time.Duration,
) (resolvedInstanceConfig, error) {
	if timeout <= 0 {
		return p.configForInstance(instanceID)
	}

	cfg, ok, err := p.routes.Wait(context.Background(), instanceID, timeout, p.lifecycle.StopChannel())
	if err != nil {
		return resolvedInstanceConfig{}, err
	}
	if !ok {
		return p.configForInstance(instanceID)
	}
	return cfg, nil
}

func (p *slackProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	configs := p.routes.ByPath(normalizeWebhookPath(path))
	if len(configs) == 0 {
		return resolvedInstanceConfig{}, false
	}
	return configs[0], true
}

func (p *slackProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func closeInboundBatchers(batchers map[*bridgesdk.InboundBatcher]struct{}) {
	for batcher := range batchers {
		batcher.Close()
	}
}

func (p *slackProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *slackProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
