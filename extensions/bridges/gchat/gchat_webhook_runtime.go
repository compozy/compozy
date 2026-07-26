package main

import (
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

func (p *gchatProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("gchat: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}

func (p *gchatProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
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
			return verifyGChatWebhookBearer(ctx, req, body, &cfg)
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

func (p *gchatProvider) handleWebhookRequest(
	w http.ResponseWriter,
	r *http.Request,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	ctx := context.Background()
	if r != nil && r.Context() != nil {
		ctx = r.Context()
	}
	shape := detectGChatWebhookShape(request.Body)
	switch shape {
	case gchatModePubSub:
		if !modeUsesPubSubIngress(cfg.mode) {
			return writeWebhookJSON(w, map[string]any{"ignored": true})
		}
		return p.handlePubSubWebhook(ctx, w, cfg, request)
	case gchatModeDirect:
		if !modeUsesDirectIngress(cfg.mode) {
			return writeWebhookJSON(w, map[string]any{"ignored": true})
		}
		return p.handleDirectWebhook(ctx, w, cfg, request)
	default:
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid google chat webhook payload"}
	}
}

func (p *gchatProvider) handleDirectWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	event := gchatEvent{}
	if err := json.Unmarshal(request.Body, &event); err != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid google chat direct webhook payload",
		}
	}

	if event.Chat == nil {
		return writeWebhookJSON(w, map[string]any{})
	}
	if item, ok, err := mapDirectActionEvent(event, cfg.managed, request.ReceivedAt); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		if cfg.dedup.Mark(item.Envelope.IdempotencyKey) {
			return writeWebhookJSON(w, map[string]any{})
		}
		if allowGChatDirectMessage(cfg, item.User, item.Direct) {
			if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, item.Envelope); err != nil {
				return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
			}
		}
		return writeWebhookJSON(w, map[string]any{})
	}
	if item, ok, err := mapDirectMessageEvent(event, cfg.managed, request.ReceivedAt, nil); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	} else if ok {
		if cfg.dedup.Mark(item.Envelope.IdempotencyKey) {
			return writeWebhookJSON(w, map[string]any{})
		}
		if !allowGChatDirectMessage(cfg, item.User, item.Direct) {
			return writeWebhookJSON(w, map[string]any{})
		}
		message := event.Chat.MessagePayload.Message
		applyGChatReplyContext(&item.Envelope, message, p.parents)
		if err := item.Envelope.Validate(); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
		if cfg.batcher != nil && shouldBatchGChatMessage(event.Chat.MessagePayload.Message) {
			if err := cfg.batcher.Enqueue(item.Envelope); err != nil {
				return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
			}
		} else {
			if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, item.Envelope); err != nil {
				return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
			}
		}
		rememberGChatQuotedParent(p.parents, item.Envelope, message)
		p.parents.RememberEnvelope(item.Envelope)
	}
	return writeWebhookJSON(w, map[string]any{})
}

func (p *gchatProvider) handlePubSubWebhook(
	ctx context.Context,
	w http.ResponseWriter,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	push := gchatPubSubPushMessage{}
	if err := json.Unmarshal(request.Body, &push); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid google chat pubsub payload"}
	}
	notification, err := decodePubSubMessage(push)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}

	switch {
	case notification.Message != nil:
		item, mapErr := mapPubSubMessageEvent(notification, cfg.managed, request.ReceivedAt, nil)
		if mapErr != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: mapErr.Error()}
		}
		if cfg.dedup.Mark(item.Envelope.IdempotencyKey) {
			return writeWebhookJSON(w, map[string]any{providerSuccessKey: true})
		}
		if !allowGChatDirectMessage(cfg, item.User, item.Direct) {
			return writeWebhookJSON(w, map[string]any{providerSuccessKey: true})
		}
		applyGChatReplyContext(&item.Envelope, *notification.Message, p.parents)
		if err := item.Envelope.Validate(); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
		}
		if cfg.batcher != nil && shouldBatchGChatMessage(*notification.Message) {
			if err := cfg.batcher.Enqueue(item.Envelope); err != nil {
				return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
			}
		} else {
			if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, item.Envelope); err != nil {
				return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
			}
		}
		rememberGChatQuotedParent(p.parents, item.Envelope, *notification.Message)
		p.parents.RememberEnvelope(item.Envelope)
	case notification.Reaction != nil:
		item, mapErr := mapPubSubReactionEvent(ctx, p.apiFactory(cfg), notification, cfg.managed, request.ReceivedAt)
		if mapErr != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: mapErr.Error()}
		}
		if cfg.dedup.Mark(item.Envelope.IdempotencyKey) {
			return writeWebhookJSON(w, map[string]any{providerSuccessKey: true})
		}
		if !allowGChatDirectMessage(cfg, item.User, item.Direct) {
			return writeWebhookJSON(w, map[string]any{providerSuccessKey: true})
		}
		if err := p.dispatchInboundEnvelope(ctx, cfg.instanceID, item.Envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
	}
	return writeWebhookJSON(w, map[string]any{providerSuccessKey: true})
}

func (p *gchatProvider) dispatchInboundBatch(
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
		attachments := make([]bridgepkg.MessageAttachment, 0)
		for _, item := range batch.Items {
			if text := strings.TrimSpace(item.Content.Text); text != "" {
				parts = append(parts, text)
			}
			attachments = append(attachments, item.Attachments...)
		}
		merged.Content.Text = strings.Join(parts, "\n")
		merged.Attachments = attachments
		merged.IdempotencyKey = fmt.Sprintf("%s:batch:%d", merged.IdempotencyKey, len(batch.Items))
	}
	return p.dispatchInboundEnvelope(ctx, bridgeInstanceID, merged)
}

func (p *gchatProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("gchat: runtime session is not initialized")
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
	} else {
		p.clearLastError()
	}
	return nil
}

func (p *gchatProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf("gchat: delivery targeted unmanaged instance %q", instanceID)
	}
	return cfg, nil
}

func (p *gchatProvider) waitForInstanceConfig(
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

func (p *gchatProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	normalizedPath := normalizeWebhookPath(path)
	configs := p.routes.ByPath(normalizedPath)
	if len(configs) != 1 || configs[0].configError != nil {
		return resolvedInstanceConfig{}, false
	}
	return configs[0], true
}

func (p *gchatProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *gchatProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *gchatProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
