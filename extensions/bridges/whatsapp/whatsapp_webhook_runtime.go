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

func (p *whatsappProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
	cfg, ok := p.configForPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if r.Method == http.MethodGet {
		p.handleVerifyChallenge(w, r, cfg)
		return
	}

	handler, err := bridgesdk.NewWebhookHandler(bridgesdk.WebhookGuardConfig{
		AllowedMethods:      []string{http.MethodPost},
		AllowedContentTypes: []string{"application/json"},
		MaxBodyBytes:        1 << 20,
		RateLimiter:         cfg.rateLimiter,
		InFlightLimiter:     cfg.inFlightLimiter,
		VerifySignature: func(ctx context.Context, req *http.Request, body []byte) error {
			return verifyWhatsAppSignature(ctx, req, body, cfg.appSecret)
		},
		RequestKey: func(req *http.Request) string {
			return req.RemoteAddr + "|" + cfg.instanceID
		},
		Now: func() time.Time { return p.now() },
	}, func(w http.ResponseWriter, r *http.Request, request bridgesdk.WebhookRequest) error {
		return p.handleWebhookRequest(w, r, cfg, request)
	})
	if err != nil {
		http.Error(
			w,
			http.StatusText(http.StatusInternalServerError),
			http.StatusInternalServerError,
		)
		p.setInstanceError(cfg.instanceID, err)
		return
	}
	handler.ServeHTTP(w, r)
}

func (p *whatsappProvider) handleVerifyChallenge(
	w http.ResponseWriter,
	r *http.Request,
	cfg resolvedInstanceConfig,
) {
	mode := strings.TrimSpace(r.URL.Query().Get("hub.mode"))
	token := strings.TrimSpace(r.URL.Query().Get("hub.verify_token"))
	challenge, err := parseWhatsAppVerifyChallenge(r.URL.Query().Get("hub.challenge"))
	if mode == "subscribe" && token == strings.TrimSpace(cfg.verifyToken) {
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			p.setInstanceError(cfg.instanceID, err)
			return
		}
		if err := writeWhatsAppVerifyChallenge(w, challenge); err != nil {
			p.setInstanceError(cfg.instanceID, err)
		}
		return
	}
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
}

func (p *whatsappProvider) handleWebhookRequest(
	w http.ResponseWriter,
	_ *http.Request,
	cfg resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	payload := whatsappWebhookPayload{}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusBadRequest,
			Message:    "invalid whatsapp webhook payload",
		}
	}

	var dispatchErr error
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			if strings.TrimSpace(change.Field) != providerMessagesKey {
				continue
			}
			if !matchesPhoneNumberID(cfg, change.Value.Metadata.PhoneNumberID) {
				continue
			}
			contacts := contactsByWaID(change.Value.Contacts)
			for _, message := range change.Value.Messages {
				envelope, err := mapWhatsAppInboundMessage(
					message,
					contacts[strings.TrimSpace(message.From)],
					cfg.managed,
					request.ReceivedAt,
					cfg.phoneNumberID,
				)
				if err != nil {
					return &bridgesdk.HTTPError{
						StatusCode: http.StatusBadRequest,
						Message:    err.Error(),
					}
				}
				if cfg.dedup.Mark(envelope.IdempotencyKey) {
					continue
				}
				if !allowWhatsAppDirectMessage(cfg, envelope.Sender) {
					continue
				}
				if cfg.batcher != nil {
					if err := cfg.batcher.Enqueue(envelope); err != nil {
						return &bridgesdk.HTTPError{
							StatusCode: http.StatusInternalServerError,
							Message:    err.Error(),
						}
					}
					continue
				}
				if err := p.dispatchInboundEnvelope(context.Background(), cfg.instanceID, envelope); err != nil {
					dispatchErr = err
					break
				}
			}
			if dispatchErr != nil {
				break
			}
		}
		if dispatchErr != nil {
			break
		}
	}
	if dispatchErr != nil {
		return &bridgesdk.HTTPError{
			StatusCode: http.StatusInternalServerError,
			Message:    dispatchErr.Error(),
		}
	}

	return writeWhatsAppWebhookOK(w)
}

func (p *whatsappProvider) dispatchInboundBatch(
	ctx context.Context,
	bridgeInstanceID string,
	batch bridgesdk.InboundBatch,
) error {
	if len(batch.Items) == 0 {
		return nil
	}
	if !canMergeWhatsAppInboundBatch(batch.Items) {
		for _, item := range batch.Items {
			if err := p.dispatchInboundEnvelope(ctx, bridgeInstanceID, item); err != nil {
				return err
			}
		}
		return nil
	}
	merged := batch.Items[0]
	if len(batch.Items) > 1 {
		parts := make([]string, 0, len(batch.Items))
		for _, item := range batch.Items {
			if strings.TrimSpace(item.Content.Text) != "" {
				parts = append(parts, item.Content.Text)
			}
		}
		merged.Content.Text = strings.Join(parts, "\n")
		merged.IdempotencyKey = fmt.Sprintf("%s:batch:%d", merged.IdempotencyKey, len(batch.Items))
	}
	return p.dispatchInboundEnvelope(ctx, bridgeInstanceID, merged)
}

func (p *whatsappProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("whatsapp: runtime session is not initialized")
	}
	cfg, err := p.configForInstance(bridgeInstanceID)
	if err != nil {
		p.setInstanceError(bridgeInstanceID, err)
		return err
	}

	_, err = p.lifecycle.Host().IngestBridgeMessage(ctx, session, envelope)
	if err != nil {
		p.setInstanceError(cfg.instanceID, err)
		return err
	}
	if err := p.lifecycle.Host().ReportReadyIfNeeded(ctx, session, cfg.instanceID); err != nil {
		p.setInstanceError(cfg.instanceID, err)
	} else {
		p.clearInstanceError(cfg.instanceID)
	}
	return nil
}

func canMergeWhatsAppInboundBatch(items []bridgepkg.InboundMessageEnvelope) bool {
	for _, item := range items {
		if len(item.Attachments) > 0 ||
			len(bytes.TrimSpace(item.ProviderMetadata)) > 0 ||
			item.Command != nil ||
			item.Action != nil ||
			item.Reaction != nil ||
			item.Conversation != nil {
			return false
		}
		family := item.EventFamily.Normalize()
		if family != "" && family != bridgepkg.InboundEventFamilyMessage {
			return false
		}
	}
	return true
}

func (p *whatsappProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	trimmedInstanceID := strings.TrimSpace(instanceID)
	cfg, ok := p.routes.Get(trimmedInstanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf(
			"%w: %q",
			errWhatsAppInstanceConfigUnavailable,
			trimmedInstanceID,
		)
	}
	if cfg.configError != nil {
		return cfg, fmt.Errorf(
			"%w: %q: %w",
			errWhatsAppInstanceConfigInvalid,
			trimmedInstanceID,
			cfg.configError,
		)
	}
	return cfg, nil
}

func (p *whatsappProvider) waitForInstanceConfig(
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
	if !ok || cfg.configError != nil {
		return p.configForInstance(instanceID)
	}
	return cfg, nil
}

func (p *whatsappProvider) configForPath(path string) (resolvedInstanceConfig, bool) {
	normalizedPath := normalizeWebhookPath(path)
	configs := p.routes.ByPath(normalizedPath)
	if len(configs) != 1 {
		return resolvedInstanceConfig{}, false
	}
	cfg := configs[0]
	if cfg.configError != nil {
		return resolvedInstanceConfig{}, false
	}
	return cfg, true
}

func (p *whatsappProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *whatsappProvider) setLastError(err error) {
	p.setInstanceError("", err)
}

func (p *whatsappProvider) setInstanceError(instanceID string, err error) {
	if err == nil {
		return
	}
	instanceID = strings.TrimSpace(instanceID)
	p.mu.Lock()
	defer p.mu.Unlock()
	if instanceID != "" {
		if p.instanceErrors == nil {
			p.instanceErrors = make(map[string]string)
		}
		p.instanceErrors[instanceID] = err.Error()
		return
	}
	p.lastError = err.Error()
	p.lastErrorSeq++
}

func (p *whatsappProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
	p.lastErrorSeq++
	p.instanceErrors = make(map[string]string)
}

func (p *whatsappProvider) clearGlobalError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
	p.lastErrorSeq++
}

func (p *whatsappProvider) clearGlobalErrorIfUnchanged(seq uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.lastErrorSeq != seq {
		return
	}
	p.lastError = ""
	p.lastErrorSeq++
}

func (p *whatsappProvider) clearInstanceError(instanceID string) {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		p.clearGlobalError()
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.instanceErrors, instanceID)
}
