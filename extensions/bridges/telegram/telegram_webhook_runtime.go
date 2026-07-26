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

func (p *telegramProvider) startServer(listenAddr string) error {
	if p.lifecycle.Stopped() {
		return errors.New("telegram: runtime is stopping")
	}
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}

func (p *telegramProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
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
			return verifyWebhookSecret(ctx, req, body, cfg.webhookSecret)
		},
		RequestKey: func(req *http.Request) string {
			return req.RemoteAddr + "|" + cfg.instanceID
		},
		Now: func() time.Time { return p.now() },
	}, func(w http.ResponseWriter, r *http.Request, request bridgesdk.WebhookRequest) error {
		return p.handleWebhookRequest(w, r, &cfg, request)
	})
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		p.setLastError(err)
		return
	}
	handler.ServeHTTP(w, r)
}

func (p *telegramProvider) handleWebhookRequest(
	w http.ResponseWriter,
	_ *http.Request,
	cfg *resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	update := telegramUpdate{}
	if err := json.Unmarshal(request.Body, &update); err != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid telegram webhook payload",
		}
	}
	message := selectTelegramMessage(update)
	if message == nil {
		return writeTelegramWebhookOK(w)
	}
	if isUnsupportedTextlessTelegramEdit(update) {
		return writeTelegramWebhookOK(w)
	}

	envelope, err := mapTelegramUpdate(update, *cfg.managed, request.ReceivedAt, nil)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if cfg.dedup.Mark(envelope.IdempotencyKey) {
		return writeTelegramWebhookOK(w)
	}
	if !allowDirectMessage(cfg, *message) {
		return writeTelegramWebhookOK(w)
	}
	applyTelegramReplyContext(&envelope, *message, p.parents)
	if err := envelope.Validate(); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}

	if cfg.batcher != nil && shouldBatchTelegramInbound(update) {
		if err := cfg.batcher.Enqueue(envelope); err != nil {
			return &bridgesdk.HTTPError{
				StatusCode: http.StatusInternalServerError,
				Message:    err.Error(),
			}
		}
	} else {
		if err := p.dispatchInboundEnvelope(context.Background(), cfg.instanceID, envelope); err != nil {
			return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
		}
	}
	rememberTelegramReplyParent(p.parents, envelope, *message)
	p.parents.RememberEnvelope(envelope)

	return writeTelegramWebhookOK(w)
}

func (p *telegramProvider) dispatchInboundBatch(
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

func (p *telegramProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("telegram: runtime session is not initialized")
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

func (p *telegramProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf(
			"telegram: delivery targeted unmanaged instance %q",
			instanceID,
		)
	}
	return cfg, nil
}

func (p *telegramProvider) waitForInstanceConfig(
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

func (p *telegramProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	configs := p.routes.ByPath(normalizeWebhookPath(path))
	if len(configs) == 0 {
		return resolvedInstanceConfig{}, false
	}
	return configs[0], true
}

func (p *telegramProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *telegramProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *telegramProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
