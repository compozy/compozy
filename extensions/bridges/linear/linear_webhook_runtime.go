package main

import (
	"context"

	"errors"
	"fmt"

	"net/http"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *linearProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("linear: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}

func (p *linearProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
	candidates := p.configsForPath(r.URL.Path)
	if len(candidates) == 0 {
		http.NotFound(w, r)
		return
	}

	handler, err := bridgesdk.NewWebhookHandler(bridgesdk.WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		MaxBodyBytes:        1 << 20,
		RateLimiter:         p.rateLimiter,
		InFlightLimiter:     p.inFlight,
		VerifySignature: func(_ context.Context, req *http.Request, body []byte) error {
			scopedCandidates, err := selectLinearWebhookSignatureCandidates(body, candidates)
			if err != nil {
				return err
			}
			return verifyLinearWebhookSignature(req, body, scopedCandidates)
		},
		RequestKey: func(req *http.Request) string {
			return req.RemoteAddr + "|" + normalizeWebhookPath(req.URL.Path)
		},
		Now: func() time.Time { return p.now() },
	}, func(w http.ResponseWriter, r *http.Request, request bridgesdk.WebhookRequest) error {
		return p.handleWebhookRequest(w, r, candidates, request)
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		p.setLastError(err)
		return
	}
	handler.ServeHTTP(w, r)
}

func (p *linearProvider) handleWebhookRequest(
	w http.ResponseWriter,
	r *http.Request,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	eventType, err := decodeLinearWebhookEnvelopeType(request.Body, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid linear webhook payload"}
	}
	ctx, cancel := p.webhookIngressContext(r)
	defer cancel()

	switch strings.TrimSpace(eventType) {
	case providerCommentValue:
		return p.handleLinearCommentWebhook(ctx, w, candidates, request)
	case providerAgentSessionEventValue:
		return p.handleLinearAgentSessionWebhook(ctx, w, candidates, request)
	default:
		return writeWebhookText(w, http.StatusOK, "ok")
	}
}

func (p *linearProvider) webhookIngressContext(r *http.Request) (context.Context, context.CancelFunc) {
	base := context.Background()
	if r != nil {
		base = context.WithoutCancel(r.Context())
	}
	timeout := linearWebhookIngressTimeout
	if p != nil && p.webhookIngressTimeout > 0 {
		timeout = p.webhookIngressTimeout
	}
	return context.WithTimeout(base, timeout)
}

func (p *linearProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("linear: runtime session is not initialized")
	}
	cfg, err := p.configForInstance(bridgeInstanceID)
	if err != nil {
		return err
	}
	if _, err := p.lifecycle.Host().IngestBridgeMessage(ctx, session, envelope); err != nil {
		return err
	}
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setLastError(err)
	} else {
		p.clearLastError()
	}
	return nil
}

func (p *linearProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf("linear: unmanaged bridge instance %q", instanceID)
	}
	return cfg, nil
}

func (p *linearProvider) waitForInstanceConfig(
	ctx context.Context,
	instanceID string,
	timeout time.Duration,
) (resolvedInstanceConfig, error) {
	if timeout <= 0 {
		return p.configForInstance(instanceID)
	}
	cfg, ok, err := p.routes.Wait(ctx, instanceID, timeout, p.lifecycle.StopChannel())
	if err != nil {
		return resolvedInstanceConfig{}, err
	}
	if !ok {
		return p.configForInstance(instanceID)
	}
	return cfg, nil
}

func (p *linearProvider) configsForPath(path string) []resolvedInstanceConfig {
	return p.routes.ByPath(normalizeWebhookPath(path))
}

func (p *linearProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *linearProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *linearProvider) storeDeliveryState(instanceID string, deliveryID string, state deliveryState) {
	p.deliveries.Store(deliveryStateKey(instanceID, deliveryID), state)
}

func (p *linearProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *linearProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
