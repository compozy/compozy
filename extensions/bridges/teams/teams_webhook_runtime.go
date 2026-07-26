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

func (p *teamsProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
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
			return verifyTeamsAuthorization(ctx, req, body, cfg)
		},
		RequestKey: func(req *http.Request) string {
			return req.RemoteAddr + "|" + cfg.instanceID
		},
		Now: func() time.Time { return p.now() },
	}, func(w http.ResponseWriter, _ *http.Request, request bridgesdk.WebhookRequest) error {
		return p.handleWebhookRequest(w, cfg, request)
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

func (p *teamsProvider) handleWebhookRequest(
	w http.ResponseWriter,
	cfg resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	var activity teamsActivity
	if err := json.Unmarshal(request.Body, &activity); err != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid teams activity payload",
		}
	}

	p.storeUserContext(cfg.instanceID, activity)

	items, err := mapTeamsActivity(activity, cfg, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if len(items) == 0 {
		return writeWebhookOK(w)
	}

	for _, item := range items {
		if cfg.dedup.Mark(item.Envelope.IdempotencyKey) {
			continue
		}
		if !allowTeamsDirectMessage(cfg, item.User, item.Direct) {
			continue
		}
		if cfg.batcher != nil &&
			item.Envelope.EventFamily.Normalize() == bridgepkg.InboundEventFamilyMessage {
			if err := cfg.batcher.Enqueue(item.Envelope); err != nil {
				return &bridgesdk.HTTPError{
					StatusCode: http.StatusInternalServerError,
					Message:    err.Error(),
				}
			}
			continue
		}
		if err := p.dispatchInboundEnvelope(context.Background(), cfg.instanceID, item.Envelope); err != nil {
			return &bridgesdk.HTTPError{
				StatusCode: http.StatusInternalServerError,
				Message:    err.Error(),
			}
		}
	}
	return writeWebhookOK(w)
}

func (p *teamsProvider) dispatchInboundBatch(
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

func (p *teamsProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("teams: runtime session is not initialized")
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

func (p *teamsProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf(
			"teams: bridge instance %q is not initialized",
			instanceID,
		)
	}
	return cfg, nil
}

func (p *teamsProvider) waitForInstanceConfig(
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

func (p *teamsProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	configs := p.routes.ByPath(normalizeWebhookPath(path))
	if len(configs) != 1 || configs[0].configError != nil {
		return resolvedInstanceConfig{}, false
	}
	return configs[0], true
}

func (p *teamsProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *teamsProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *teamsProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
