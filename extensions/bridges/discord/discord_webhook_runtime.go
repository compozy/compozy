package main

import (
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"net/http"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *discordProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
	cfg, ok := p.configForPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler, err := bridgesdk.NewWebhookHandler(bridgesdk.WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		MaxBodyBytes:        1 << 20,
		RateLimiter:         cfg.rateLimiter,
		InFlightLimiter:     cfg.inFlightLimiter,
		VerifySignature: func(ctx context.Context, req *http.Request, body []byte) error {
			return verifyDiscordSignature(ctx, req, body, cfg.publicKey, p.now())
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

func (p *discordProvider) handleWebhookRequest(
	w http.ResponseWriter,
	r *http.Request,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	var probe discordPayloadProbe
	if err := json.Unmarshal(request.Body, &probe); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid discord webhook payload"}
	}
	if len(bytes.TrimSpace(probe.Token)) > 0 {
		return p.handleInteractionWebhook(w, cfg, request)
	}
	return p.handleEventWebhook(w, r, cfg, request)
}

func (p *discordProvider) handleInteractionWebhook(
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	var interaction discordInteraction
	if err := json.Unmarshal(request.Body, &interaction); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid discord interaction payload"}
	}

	switch interaction.Type {
	case discordInteractionTypePing:
		return writeDiscordInteractionResponse(w, discordInteractionResponseTypePong)
	case discordInteractionTypeApplicationCommand:
		mapped, err := mapDiscordInteractionCommand(interaction, cfg.managed, request.ReceivedAt)
		if err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
		if !cfg.dedup.Mark(mapped.Envelope.IdempotencyKey) &&
			allowDiscordDirectMessage(cfg, mapped.User, mapped.Direct) {
			p.dispatchAsyncInboundEnvelope(cfg.instanceID, mapped.Envelope)
		}
		return writeDiscordInteractionResponse(w, discordInteractionResponseTypeDeferredChannelMessage)
	case discordInteractionTypeMessageComponent:
		mapped, err := mapDiscordInteractionAction(interaction, cfg.managed, request.ReceivedAt)
		if err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
		if !cfg.dedup.Mark(mapped.Envelope.IdempotencyKey) &&
			allowDiscordDirectMessage(cfg, mapped.User, mapped.Direct) {
			p.dispatchAsyncInboundEnvelope(cfg.instanceID, mapped.Envelope)
		}
		return writeDiscordInteractionResponse(w, discordInteractionResponseTypeDeferredUpdateMessage)
	default:
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    fmt.Sprintf("unsupported discord interaction type %d", interaction.Type),
		}
	}
}

func (p *discordProvider) handleEventWebhook(
	w http.ResponseWriter,
	r *http.Request,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	var envelope discordWebhookEventEnvelope
	if err := json.Unmarshal(request.Body, &envelope); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid discord event payload"}
	}

	switch envelope.Type {
	case discordWebhookEnvelopeTypePing:
		return writeWebhookNoContent(w)
	case discordWebhookEnvelopeTypeEvent:
	default:
		return writeWebhookNoContent(w)
	}
	if envelope.Event == nil {
		return writeWebhookNoContent(w)
	}
	ctx := context.Background()
	if r != nil && r.Context() != nil {
		ctx = r.Context()
	}

	switch strings.TrimSpace(envelope.Event.Type) {
	case providerMessageCreateValue:
		return p.handleDiscordMessageWebhookEvent(ctx, w, cfg, request, envelope)
	case providerMessageReactionAddValue, "MESSAGE_REACTION_REMOVE":
		return p.handleDiscordReactionWebhookEvent(ctx, w, cfg, request, envelope)
	default:
		return writeWebhookNoContent(w)
	}
}

func (p *discordProvider) handleDiscordMessageWebhookEvent(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
	envelope discordWebhookEventEnvelope,
) error {
	var event discordMessageEvent
	if err := json.Unmarshal(envelope.Event.Data, &event); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid discord message event"}
	}
	mapped, ignored, err := mapDiscordMessageEvent(event, cfg.managed, request.ReceivedAt, envelope.Event.ID)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if ignored {
		return writeWebhookNoContent(w)
	}
	return p.dispatchDiscordWebhookEnvelope(ctx, w, cfg, mapped, true)
}

func (p *discordProvider) handleDiscordReactionWebhookEvent(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
	envelope discordWebhookEventEnvelope,
) error {
	var event discordReactionEvent
	if err := json.Unmarshal(envelope.Event.Data, &event); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid discord reaction event"}
	}
	mapped, err := mapDiscordReactionEvent(
		event,
		cfg.managed,
		request.ReceivedAt,
		envelope.Event.ID,
		envelope.Event.Type,
	)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	return p.dispatchDiscordWebhookEnvelope(ctx, w, cfg, mapped, false)
}

func (p *discordProvider) dispatchDiscordWebhookEnvelope(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	mapped discordMappedInbound,
	allowBatch bool,
) error {
	if cfg == nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: "discord: config is required"}
	}
	if cfg.dedup.Mark(mapped.Envelope.IdempotencyKey) {
		return writeWebhookNoContent(w)
	}
	if !allowDiscordDirectMessage(cfg, mapped.User, mapped.Direct) {
		return writeWebhookNoContent(w)
	}
	if allowBatch && cfg.batcher != nil {
		if err := cfg.batcher.Enqueue(mapped.Envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
		return writeWebhookNoContent(w)
	}
	if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, mapped.Envelope); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
	}
	return writeWebhookNoContent(w)
}

func (p *discordProvider) dispatchAsyncInboundEnvelope(instanceID string, envelope bridgepkg.InboundMessageEnvelope) {
	p.lifecycle.Go(func() {
		select {
		case <-p.lifecycle.StopChannel():
			return
		default:
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.dispatchInboundEnvelope(ctx, instanceID, envelope); err != nil {
			p.setLastError(err)
		}
	})
}

func (p *discordProvider) dispatchInboundBatch(
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

func (p *discordProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("discord: runtime session is not initialized")
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

func (p *discordProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf("discord: delivery targeted unmanaged instance %q", instanceID)
	}
	return cfg, nil
}

func (p *discordProvider) waitForInstanceConfig(
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

func (p *discordProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	configs := p.routes.ByPath(normalizeWebhookPath(path))
	if len(configs) == 0 {
		return resolvedInstanceConfig{}, false
	}
	return configs[0], true
}

func (p *discordProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func closeDiscordInboundBatchers(batchers map[*bridgesdk.InboundBatcher]struct{}) {
	for batcher := range batchers {
		batcher.Close()
	}
}

func (p *discordProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *discordProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
