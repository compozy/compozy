package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"net"
	"net/http"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *githubProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("github: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}

func (p *githubProvider) serveWebhookHTTP(w http.ResponseWriter, r *http.Request) {
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
		InFlightLimiter:     p.inFlightLimiter,
		VerifySignature: func(ctx context.Context, req *http.Request, body []byte) error {
			return verifyGitHubWebhookSignature(ctx, req, body, candidates)
		},
		RequestKey: githubWebhookRequestKey,
		Now:        func() time.Time { return p.now() },
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

func githubWebhookRequestKey(req *http.Request) string {
	if req == nil {
		return "global"
	}

	remote := strings.TrimSpace(req.RemoteAddr)
	host, _, err := net.SplitHostPort(remote)
	if err != nil || strings.TrimSpace(host) == "" {
		host = remote
	}
	if strings.TrimSpace(host) == "" {
		host = "global"
	}
	return strings.TrimSpace(host) + "|" + normalizeWebhookPath(req.URL.Path)
}

func (p *githubProvider) handleWebhookRequest(
	w http.ResponseWriter,
	r *http.Request,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	eventType := strings.ToLower(strings.TrimSpace(r.Header.Get("X-GitHub-Event")))
	switch eventType {
	case "ping":
		return writeWebhookText(w, "pong")
	case "issue_comment":
		return p.handleIssueCommentWebhook(w, r, candidates, request)
	case "pull_request_review_comment":
		return p.handleReviewCommentWebhook(w, r, candidates, request)
	default:
		return writeWebhookText(w, "ok")
	}
}

func (p *githubProvider) handleIssueCommentWebhook(
	w http.ResponseWriter,
	r *http.Request,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	payload := githubIssuePayload{}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid github webhook payload"}
	}
	cfg, ok, err := selectGitHubIssueConfig(candidates, payload)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if !ok {
		return writeWebhookText(w, "ignored")
	}
	if err := verifyGitHubWebhookSignature(r.Context(), r, request.Body, []resolvedInstanceConfig{cfg}); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusUnauthorized, Message: "invalid github webhook signature"}
	}
	if strings.TrimSpace(payload.Action) != providerCreatedKey {
		return writeWebhookText(w, "ok")
	}
	item, err := mapGitHubIssueComment(payload, cfg.managed, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if item.InstallationID > 0 {
		p.storeInstallationID(cfg.repoFullName, item.InstallationID)
	}
	if cfg.dedup.Mark(item.Envelope.IdempotencyKey) || isGitHubSelfMessage(&cfg, payload.Sender) {
		return writeWebhookText(w, "ok")
	}
	if err := p.dispatchInboundEnvelope(context.Background(), cfg.instanceID, item.Envelope); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
	}
	return writeWebhookText(w, "ok")
}

func (p *githubProvider) handleReviewCommentWebhook(
	w http.ResponseWriter,
	r *http.Request,
	candidates []resolvedInstanceConfig,
	request bridgesdk.WebhookRequest,
) error {
	payload := githubReviewPayload{}
	if err := json.Unmarshal(request.Body, &payload); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: "invalid github webhook payload"}
	}
	cfg, ok, err := selectGitHubReviewConfig(candidates, payload)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if !ok {
		return writeWebhookText(w, "ignored")
	}
	if err := verifyGitHubWebhookSignature(r.Context(), r, request.Body, []resolvedInstanceConfig{cfg}); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusUnauthorized, Message: "invalid github webhook signature"}
	}
	if strings.TrimSpace(payload.Action) != providerCreatedKey {
		return writeWebhookText(w, "ok")
	}
	item, err := mapGitHubReviewComment(payload, cfg.managed, request.ReceivedAt)
	if err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusBadRequest, Message: err.Error()}
	}
	if item.InstallationID > 0 {
		p.storeInstallationID(cfg.repoFullName, item.InstallationID)
	}
	if cfg.dedup.Mark(item.Envelope.IdempotencyKey) || isGitHubSelfMessage(&cfg, payload.Sender) {
		return writeWebhookText(w, "ok")
	}
	if err := p.dispatchInboundEnvelope(context.Background(), cfg.instanceID, item.Envelope); err != nil {
		return &bridgesdk.HTTPError{StatusCode: http.StatusInternalServerError, Message: err.Error()}
	}
	return writeWebhookText(w, "ok")
}

func (p *githubProvider) dispatchInboundEnvelope(
	ctx context.Context,
	bridgeInstanceID string,
	envelope bridgepkg.InboundMessageEnvelope,
) error {
	session := p.currentSession()
	if session == nil {
		return errors.New("github: runtime session is not initialized")
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

func (p *githubProvider) configForInstance(instanceID string) (resolvedInstanceConfig, error) {
	cfg, ok := p.routes.Get(instanceID)
	if !ok {
		return resolvedInstanceConfig{}, fmt.Errorf("github: unmanaged bridge instance %q", instanceID)
	}
	return cfg, nil
}

func (p *githubProvider) waitForInstanceConfig(ctx context.Context, instanceID string) (resolvedInstanceConfig, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg, err := p.configForInstance(instanceID); err == nil {
		return cfg, nil
	}
	select {
	case <-p.lifecycle.StopChannel():
		return resolvedInstanceConfig{}, bridgesdk.ErrProviderStopped
	case <-ctx.Done():
		return resolvedInstanceConfig{}, ctx.Err()
	case <-p.lifecycle.RoutesReady():
		return p.configForInstance(instanceID)
	}
}

func (p *githubProvider) configsForPath(path string) []resolvedInstanceConfig {
	return p.routes.ByPath(normalizeWebhookPath(path))
}

func (p *githubProvider) currentSession() *bridgesdk.Session {
	return p.lifecycle.Session()
}

func (p *githubProvider) deliveryState(instanceID string, deliveryID string) deliveryState {
	state, _ := p.deliveries.Load(deliveryStateKey(instanceID, deliveryID))
	return state
}

func (p *githubProvider) storeDeliveryState(
	instanceID string,
	deliveryID string,
	event bridgepkg.DeliveryEvent,
	state deliveryState,
) {
	key := deliveryStateKey(instanceID, deliveryID)
	if isTerminalGitHubDeliveryEvent(event) {
		p.deliveries.Delete(key)
		return
	}
	p.deliveries.Store(key, state)
}

func (p *githubProvider) storeInstallationID(repoFullName string, installationID int64) {
	if installationID <= 0 || strings.TrimSpace(repoFullName) == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.installationCache[strings.ToLower(strings.TrimSpace(repoFullName))] = installationID
}

func (p *githubProvider) cachedInstallationID(repoFullName string) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.installationCache[strings.ToLower(strings.TrimSpace(repoFullName))]
}

func (p *githubProvider) resolveDeliveryInstallationID(
	cfg *resolvedInstanceConfig,
	request bridgepkg.DeliveryRequest,
) (int64, error) {
	if cfg == nil {
		return 0, errors.New("github: config is required")
	}
	if cfg.mode != githubModeApp {
		return 0, nil
	}
	if cfg.installationID > 0 {
		return cfg.installationID, nil
	}
	if installationID := installationIDFromMetadata(request.Event.ProviderMetadata); installationID > 0 {
		p.storeInstallationID(cfg.repoFullName, installationID)
		return installationID, nil
	}
	if request.Snapshot != nil {
		if installationID := installationIDFromMetadata(request.Snapshot.ProviderMetadata); installationID > 0 {
			p.storeInstallationID(cfg.repoFullName, installationID)
			return installationID, nil
		}
	}
	if installationID := p.cachedInstallationID(cfg.repoFullName); installationID > 0 {
		return installationID, nil
	}
	return 0, &bridgesdk.PermanentError{
		Err: fmt.Errorf("github: installation id is required for app-mode delivery on %q", cfg.repoFullName),
	}
}

func (p *githubProvider) setLastError(err error) {
	if err == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = err.Error()
}

func (p *githubProvider) clearLastError() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastError = ""
}
