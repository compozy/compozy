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

func (p *slackProvider) stopResources() {
	p.closeAllProgressDispatchers()
	batchersToClose := make(map[*bridgesdk.InboundBatcher]struct{})
	for id, cfg := range p.routes.Snapshot() {
		if cfg.batcher == nil {
			continue
		}
		batchersToClose[cfg.batcher] = struct{}{}
		p.routes.Update(id, func(current resolvedInstanceConfig) resolvedInstanceConfig {
			current.batcher = nil
			return current
		})
	}
	closeInboundBatchers(batchersToClose)
}

func (p *slackProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	batchersToClose := make(map[*bridgesdk.InboundBatcher]struct{})
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareSlackConfigs,
		Finalize: p.finalizeSlackConfigs,
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
			return nil
		},
		OnPublish: func() { closeInboundBatchers(batchersToClose) },
	}
	configs, err := reconciler.Reconcile(ctx, session, managed)
	if err != nil {
		p.setLastError(err)
		return nil
	}
	return configs
}

func (p *slackProvider) finalizeSlackConfigs(
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

func (p *slackProvider) prepareSlackConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(slackListenAddrEnv))
	usedPaths := make(map[string]int, len(configs))

	for idx := range configs {
		requestedListen = applySlackListenConstraint(&configs[idx], requestedListen)
		applySlackWebhookPathConflict(&configs[idx], usedPaths, configs[:idx])
	}
	p.applySlackListenErrors(configs, requestedListen)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func applySlackListenConstraint(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"slack: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func applySlackWebhookPathConflict(
	cfg *resolvedInstanceConfig,
	usedPaths map[string]int,
	configs []resolvedInstanceConfig,
) {
	if cfg == nil || cfg.webhookPath == "" {
		return
	}
	if ownerIdx, ok := usedPaths[cfg.webhookPath]; ok {
		ownerID := ""
		if ownerIdx >= 0 && ownerIdx < len(configs) {
			ownerID = configs[ownerIdx].instanceID
		}
		conflictErr := fmt.Errorf(
			"slack: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			ownerID,
			cfg.instanceID,
		)
		if ownerIdx >= 0 && ownerIdx < len(configs) && configs[ownerIdx].configError == nil {
			configs[ownerIdx].configError = conflictErr
		}
		if cfg.configError == nil {
			cfg.configError = conflictErr
		}
		return
	}
	usedPaths[cfg.webhookPath] = len(configs)
}

func (p *slackProvider) applySlackListenErrors(configs []resolvedInstanceConfig, requestedListen string) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("slack: webhook listen address is required")
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

func (p *slackProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg := slackProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
			return resolvedInstanceConfig{
				managed:     &managed,
				instanceID:  managed.Instance.ID,
				configError: fmt.Errorf("slack: decode provider_config for %q: %w", managed.Instance.ID, err),
			}
		}
	}

	botToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "bot_token")
	signingSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "signing_secret")
	listenAddr := firstNonEmpty(cfg.Webhook.ListenAddr, strings.TrimSpace(os.Getenv(slackListenAddrEnv)))
	webhookPath := normalizeWebhookPath(
		firstNonEmpty(cfg.Webhook.Path, "/slack/"+strings.TrimSpace(managed.Instance.ID)),
	)
	apiBaseURL := normalizeURL(
		firstNonEmpty(strings.TrimSpace(os.Getenv(slackAPIBaseEnv)), slackDefaultAPIBaseURL),
	)

	resolved := resolvedInstanceConfig{
		managed:         &managed,
		instanceID:      strings.TrimSpace(managed.Instance.ID),
		listenAddr:      listenAddr,
		webhookPath:     webhookPath,
		apiBaseURL:      apiBaseURL,
		botToken:        strings.TrimSpace(botToken),
		signingSecret:   strings.TrimSpace(signingSecret),
		dmPolicy:        managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:    buildSlackIDSet(cfg.DM.AllowUserIDs),
		allowUsernames:  buildSlackUsernameSet(cfg.DM.AllowUsernames),
		pairedUserIDs:   buildSlackIDSet(cfg.DM.PairedUserIDs),
		pairedUsernames: buildSlackUsernameSet(cfg.DM.PairedUsernames),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(200, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(24),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	if resolved.webhookPath == "" {
		resolved.configError = errors.New("slack: webhook path is required")
		return resolved
	}

	if cfg.Batching.DelayMS > 0 {
		batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
			Context: context.Background(),
			Delay:   time.Duration(cfg.Batching.DelayMS) * time.Millisecond,
			SplitDelay: func() time.Duration {
				if cfg.Batching.SplitDelayMS <= 0 {
					return time.Duration(cfg.Batching.DelayMS) * time.Millisecond
				}
				return time.Duration(cfg.Batching.SplitDelayMS) * time.Millisecond
			}(),
			SplitThreshold: cfg.Batching.SplitThreshold,
			Dispatch: func(ctx context.Context, batch bridgesdk.InboundBatch) error {
				return p.dispatchInboundBatch(ctx, resolved.instanceID, batch)
			},
			Now: func() time.Time { return p.now() },
		})
		if err != nil {
			resolved.configError = err
			return resolved
		}
		resolved.batcher = batcher
	}

	return resolved
}

func (p *slackProvider) determineInitialState(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg == nil {
		err := errors.New("slack: resolved instance config is required")
		return bridgepkg.BridgeStatusError, nil, err
	}
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.botToken) == "" {
		err := errors.New("slack: bot_token secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if strings.TrimSpace(cfg.signingSecret) == "" {
		err := errors.New("slack: signing_secret secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	_, err := p.apiFactory(cfg).AuthTest(ctx)
	if err != nil {
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

func (p *slackProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("slack: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}
