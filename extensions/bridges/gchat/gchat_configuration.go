package main

import (
	"context"

	"encoding/json"
	"errors"
	"fmt"

	"os"

	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *gchatProvider) stopResources() {
	p.closeAllGChatProgressDispatchers()
	batchers := make(map[*bridgesdk.InboundBatcher]struct{})
	routes := p.routes.Snapshot()
	for id := range routes {
		cfg := routes[id]
		if cfg.batcher == nil {
			continue
		}
		batchers[cfg.batcher] = struct{}{}
		p.routes.Update(id, func(current resolvedInstanceConfig) resolvedInstanceConfig {
			current.batcher = nil
			return current
		})
	}
	for batcher := range batchers {
		batcher.Close()
	}
}

func (p *gchatProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	batchersToClose := make(map[*bridgesdk.InboundBatcher]struct{})
	removedInstanceIDs := make([]string, 0)
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareGChatConfigs,
		Finalize: p.finalizeGChatConfigs,
		Identity: func(config resolvedInstanceConfig) string { return config.instanceID },
		Merge: func(prior resolvedInstanceConfig, next resolvedInstanceConfig) resolvedInstanceConfig {
			if prior.batcher != nil && prior.batcher != next.batcher {
				batchersToClose[prior.batcher] = struct{}{}
			}
			return next
		},
		OnRemoved: func(config resolvedInstanceConfig) error {
			if config.batcher != nil {
				batchersToClose[config.batcher] = struct{}{}
			}
			removedInstanceIDs = append(removedInstanceIDs, config.instanceID)
			return nil
		},
		OnPublish: func() {
			closeGChatInboundBatchers(batchersToClose)
			p.reconcileGChatAPIClients(p.routes.Snapshot(), removedInstanceIDs)
		},
	}
	configs, err := reconciler.Reconcile(ctx, session, managed)
	if err != nil {
		p.setLastError(err)
		return nil
	}
	return configs
}

func (p *gchatProvider) finalizeGChatConfigs(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		status, degradation, probeErr := p.determineInitialState(ctx, &configs[idx])
		if probeErr != nil {
			p.setLastError(probeErr)
		}
		configs[idx].initialStatus = status
		configs[idx].initialDegradation = degradation
	}
	return configs, nil
}

func (p *gchatProvider) prepareGChatConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	configs, requestedListen := prepareGChatConfigConstraints(configs)
	p.applyGChatListenErrors(configs, requestedListen)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func prepareGChatConfigConstraints(
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, string) {
	requestedListen := strings.TrimSpace(os.Getenv(gchatListenAddrEnv))
	usedPaths := make(map[string]int, len(configs))

	for idx := range configs {
		requestedListen = applyGChatListenConstraint(&configs[idx], requestedListen)
		applyGChatWebhookPathConflict(&configs[idx], configs[:idx], usedPaths)
	}
	return configs, requestedListen
}

func applyGChatListenConstraint(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"gchat: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func applyGChatWebhookPathConflict(
	cfg *resolvedInstanceConfig,
	configs []resolvedInstanceConfig,
	usedPaths map[string]int,
) {
	if cfg == nil || cfg.webhookPath == "" {
		return
	}
	if ownerIdx, ok := usedPaths[cfg.webhookPath]; ok {
		owner := configs[ownerIdx].instanceID
		conflictErr := fmt.Errorf(
			"gchat: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			owner,
			cfg.instanceID,
		)
		if configs[ownerIdx].configError == nil {
			configs[ownerIdx].configError = conflictErr
		}
		if cfg.configError == nil {
			cfg.configError = conflictErr
		}
		return
	}
	usedPaths[cfg.webhookPath] = len(configs)
}

func (p *gchatProvider) applyGChatListenErrors(configs []resolvedInstanceConfig, requestedListen string) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("gchat: webhook listen address is required")
			}
		}
		return
	}
	if err := p.startServer(requestedListen); err != nil {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = err
			}
		}
	}
}

func (p *gchatProvider) reconcileGChatAPIClients(
	configs map[string]resolvedInstanceConfig,
	removedInstanceIDs []string,
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for instanceID := range configs {
		cfg := configs[instanceID]
		if cached, ok := p.apiClients[instanceID]; ok && cached.key != gchatAPIClientCacheKey(&cfg) {
			delete(p.apiClients, instanceID)
		}
	}
	for _, instanceID := range removedInstanceIDs {
		delete(p.apiClients, instanceID)
	}
}

func closeGChatInboundBatchers(batchers map[*bridgesdk.InboundBatcher]struct{}) {
	for batcher := range batchers {
		batcher.Close()
	}
}

func (p *gchatProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg := gchatProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
			return resolvedInstanceConfig{
				managed:     managed,
				instanceID:  managed.Instance.ID,
				configError: fmt.Errorf("gchat: decode provider_config for %q: %w", managed.Instance.ID, err),
			}
		}
	}

	credentials, err := resolveGChatCredentials(session, managed)
	if err != nil {
		return resolvedInstanceConfig{
			managed:     managed,
			instanceID:  managed.Instance.ID,
			configError: err,
		}
	}
	resolved, err := p.newResolvedGChatConfig(managed, cfg, credentials, session)
	if err != nil {
		resolved.configError = err
		return resolved
	}
	if err := validateResolvedGChatConfig(&resolved, cfg.Mode); err != nil {
		resolved.configError = err
		return resolved
	}
	if err := p.attachGChatBatcher(&resolved, cfg.Batching); err != nil {
		resolved.configError = err
		return resolved
	}
	return resolved
}

func resolveGChatCredentials(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (serviceAccountCredentials, error) {
	credentialsJSON, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "credentials_json")
	credentials := serviceAccountCredentials{}
	if strings.TrimSpace(credentialsJSON) == "" {
		return credentials, nil
	}
	if err := json.Unmarshal([]byte(credentialsJSON), &credentials); err != nil {
		return serviceAccountCredentials{}, fmt.Errorf(
			"gchat: decode credentials_json for %q: %w",
			managed.Instance.ID,
			err,
		)
	}
	return credentials, nil
}

func (p *gchatProvider) newResolvedGChatConfig(
	managed subprocess.InitializeBridgeManagedInstance,
	cfg gchatProviderConfig,
	credentials serviceAccountCredentials,
	session *bridgesdk.Session,
) (resolvedInstanceConfig, error) {
	projectNumber, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "project_number")
	directCertsURL, pubsubCertsURL, err := resolveGChatVerificationURLs(cfg)
	if err != nil {
		return resolvedInstanceConfig{
			managed:    managed,
			instanceID: strings.TrimSpace(managed.Instance.ID),
		}, err
	}

	mode := normalizeGChatMode(cfg.Mode)
	if mode == "" {
		mode = gchatModeDirect
	}

	resolved := resolvedInstanceConfig{
		managed:    managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		listenAddr: firstNonEmpty(
			cfg.Webhook.ListenAddr,
			strings.TrimSpace(os.Getenv(gchatListenAddrEnv)),
		),
		webhookPath: normalizeWebhookPath(
			firstNonEmpty(cfg.Webhook.Path, "/gchat/"+strings.TrimSpace(managed.Instance.ID)),
		),
		apiBaseURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(gchatAPIBaseEnv)),
				gchatDefaultAPIBaseURL,
			),
		),
		tokenURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(gchatTokenURLEnv)),
				gchatDefaultAuthEndpointURL,
			),
		),
		mode:                      mode,
		credentials:               credentials,
		projectNumber:             strings.TrimSpace(projectNumber),
		directIssuer:              firstNonEmpty(cfg.Verification.DirectIssuer, gchatDefaultDirectIssuer),
		directCertsURL:            directCertsURL,
		pubsubAudience:            strings.TrimSpace(cfg.Verification.PubSubAudience),
		pubsubIssuer:              firstNonEmpty(cfg.Verification.PubSubIssuer, gchatDefaultPubSubIssuerURL),
		pubsubCertsURL:            pubsubCertsURL,
		pubsubServiceAccountEmail: strings.TrimSpace(cfg.Verification.PubSubServiceAccount),
		dmPolicy:                  managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:              buildIdentitySet(cfg.DM.AllowUserIDs),
		allowUsernames:            buildIdentitySet(cfg.DM.AllowUsernames),
		pairedUserIDs:             buildIdentitySet(cfg.DM.PairedUserIDs),
		pairedUsernames:           buildIdentitySet(cfg.DM.PairedUsernames),
		dedup:                     bridgesdk.NewDedupCache(5*time.Minute, 4000),
		rateLimiter:               bridgesdk.NewFixedWindowRateLimiter(200, time.Minute),
		inFlightLimiter:           bridgesdk.NewInFlightLimiter(24),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	return resolved, nil
}

func resolveGChatVerificationURLs(cfg gchatProviderConfig) (string, string, error) {
	directCertsURL, directErr := resolveAllowedGoogleURLOverride(
		strings.TrimSpace(os.Getenv(gchatDirectCertsEnv)),
		cfg.Verification.DirectCertsURL,
		gchatDefaultDirectCertsURL,
		"provider_config.verification.direct_certs_url",
		"www.googleapis.com",
	)
	if directErr != nil {
		return "", "", directErr
	}
	pubsubCertsURL, pubsubErr := resolveAllowedGoogleURLOverride(
		strings.TrimSpace(os.Getenv(gchatPubSubCertsEnv)),
		cfg.Verification.PubSubCertsURL,
		gchatDefaultPubSubCertsURL,
		"provider_config.verification.pubsub_certs_url",
		"www.googleapis.com",
	)
	if pubsubErr != nil {
		return "", "", pubsubErr
	}
	return directCertsURL, pubsubCertsURL, nil
}

func validateResolvedGChatConfig(resolved *resolvedInstanceConfig, configuredMode string) error {
	if resolved == nil {
		return errors.New("gchat: resolved config is required")
	}
	apiBaseErr := validateGChatEndpointURL(resolved.apiBaseURL)
	tokenURLErr := validateGChatEndpointURL(resolved.tokenURL)
	switch {
	case resolved.webhookPath == "":
		return errors.New("gchat: webhook path is required")
	case resolved.apiBaseURL == "":
		return errors.New("gchat: api base url is required")
	case resolved.tokenURL == "":
		return errors.New("gchat: oauth token url is required")
	case apiBaseErr != nil:
		return apiBaseErr
	case tokenURLErr != nil:
		return tokenURLErr
	case !validGChatMode(resolved.mode):
		return fmt.Errorf("gchat: unsupported provider_config.mode %q", configuredMode)
	case modeUsesDirectIngress(resolved.mode) && strings.TrimSpace(resolved.projectNumber) == "":
		return fmt.Errorf("gchat: project_number secret binding is required for mode %q", resolved.mode)
	case modeUsesDirectIngress(resolved.mode) && resolved.directCertsURL == "":
		return errors.New("gchat: direct certs url is required")
	case modeUsesPubSubIngress(resolved.mode) && resolved.pubsubAudience == "":
		return fmt.Errorf(
			"gchat: provider_config.verification.pubsub_audience is required for mode %q",
			resolved.mode,
		)
	case modeUsesPubSubIngress(resolved.mode) && resolved.pubsubServiceAccountEmail == "":
		return fmt.Errorf(
			"gchat: provider_config.verification.pubsub_service_account_email is required for mode %q",
			resolved.mode,
		)
	case modeUsesPubSubIngress(resolved.mode) && resolved.pubsubCertsURL == "":
		return errors.New("gchat: pubsub certs url is required")
	default:
		return nil
	}
}

func (p *gchatProvider) attachGChatBatcher(
	resolved *resolvedInstanceConfig,
	cfg struct {
		DelayMS        int `json:"delay_ms,omitempty"`
		SplitDelayMS   int `json:"split_delay_ms,omitempty"`
		SplitThreshold int `json:"split_threshold,omitempty"`
	},
) error {
	if resolved == nil || cfg.DelayMS <= 0 {
		return nil
	}
	batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
		Context: context.Background(),
		Delay:   time.Duration(cfg.DelayMS) * time.Millisecond,
		SplitDelay: func() time.Duration {
			if cfg.SplitDelayMS <= 0 {
				return time.Duration(cfg.DelayMS) * time.Millisecond
			}
			return time.Duration(cfg.SplitDelayMS) * time.Millisecond
		}(),
		SplitThreshold: cfg.SplitThreshold,
		Dispatch: func(ctx context.Context, batch bridgesdk.InboundBatch) error {
			return p.dispatchInboundBatch(ctx, resolved.instanceID, batch)
		},
		Now: func() time.Time { return p.now() },
	})
	if err != nil {
		return err
	}
	resolved.batcher = batcher
	return nil
}

func (p *gchatProvider) determineInitialState(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg == nil {
		return bridgepkg.BridgeStatusError, nil, errors.New("gchat: config is required")
	}
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.credentials.ClientEmail) == "" || strings.TrimSpace(cfg.credentials.PrivateKey) == "" {
		err := errors.New("gchat: credentials_json secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if err := p.apiFactory(cfg).ValidateAuth(ctx); err != nil {
		classified := bridgesdk.ClassifyError(err)
		recovery := classified.Recovery()
		status := recovery.Status
		if status == "" {
			status = bridgepkg.BridgeStatusError
		}
		if recovery.Degradation != nil {
			return status, recovery.Degradation, err
		}
		return status, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonProviderTimeout,
			Message: classified.Message,
		}, err
	}
	return bridgepkg.BridgeStatusReady, nil, nil
}
