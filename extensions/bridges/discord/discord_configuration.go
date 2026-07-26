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

func (p *discordProvider) stopResources() {
	progressToClose := p.takeDiscordProgressDispatchers()
	batchersToClose := make(map[*bridgesdk.InboundBatcher]struct{})
	routes := p.routes.Snapshot()
	for instanceID := range routes {
		cfg := routes[instanceID]
		if cfg.batcher == nil {
			continue
		}
		batchersToClose[cfg.batcher] = struct{}{}
		p.routes.Update(instanceID, func(current resolvedInstanceConfig) resolvedInstanceConfig {
			current.batcher = nil
			return current
		})
	}
	closeDiscordInboundBatchers(batchersToClose)
	closeDiscordProgressDispatchers(progressToClose)
}

func (p *discordProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	batchersToClose := make(map[*bridgesdk.InboundBatcher]struct{})
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareDiscordConfigs,
		Finalize: p.finalizeDiscordConfigs,
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
		OnPublish: func() { closeDiscordInboundBatchers(batchersToClose) },
	}
	configs, err := reconciler.Reconcile(ctx, session, managed)
	if err != nil {
		p.setLastError(err)
		return nil
	}
	return configs
}

func (p *discordProvider) finalizeDiscordConfigs(
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

func (p *discordProvider) prepareDiscordConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(discordListenAddrEnv))
	usedPaths := make(map[string]string, len(configs))

	for idx := range configs {
		requestedListen = applyDiscordListenConstraint(&configs[idx], requestedListen)
		applyDiscordWebhookPathConflict(&configs[idx], usedPaths)
	}
	p.applyDiscordListenErrors(configs, requestedListen)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func applyDiscordListenConstraint(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"discord: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func applyDiscordWebhookPathConflict(cfg *resolvedInstanceConfig, usedPaths map[string]string) {
	if cfg == nil || cfg.webhookPath == "" {
		return
	}
	if owner, ok := usedPaths[cfg.webhookPath]; ok && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"discord: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			owner,
			cfg.instanceID,
		)
	}
	usedPaths[cfg.webhookPath] = cfg.instanceID
}

func (p *discordProvider) applyDiscordListenErrors(configs []resolvedInstanceConfig, requestedListen string) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("discord: webhook listen address is required")
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

func (p *discordProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg := discordProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
			return resolvedInstanceConfig{
				managed:     managed,
				instanceID:  managed.Instance.ID,
				configError: fmt.Errorf("discord: decode provider_config for %q: %w", managed.Instance.ID, err),
			}
		}
	}

	botToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "bot_token")
	publicKey, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "public_key")
	listenAddr := firstNonEmpty(cfg.Webhook.ListenAddr, strings.TrimSpace(os.Getenv(discordListenAddrEnv)))
	webhookPath := normalizeWebhookPath(
		firstNonEmpty(cfg.Webhook.Path, "/discord/"+strings.TrimSpace(managed.Instance.ID)),
	)
	apiBaseURL := normalizeURL(firstNonEmpty(
		strings.TrimSpace(os.Getenv(discordAPIBaseEnv)),
		discordDefaultAPIBaseURL,
	))

	resolved := resolvedInstanceConfig{
		managed:         managed,
		instanceID:      strings.TrimSpace(managed.Instance.ID),
		listenAddr:      listenAddr,
		webhookPath:     webhookPath,
		apiBaseURL:      apiBaseURL,
		applicationID:   strings.TrimSpace(cfg.ApplicationID),
		botToken:        strings.TrimSpace(botToken),
		publicKey:       strings.TrimSpace(publicKey),
		dmPolicy:        managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:    buildDiscordIDSet(cfg.DM.AllowUserIDs),
		allowUsernames:  buildDiscordUsernameSet(cfg.DM.AllowUsernames),
		pairedUserIDs:   buildDiscordIDSet(cfg.DM.PairedUserIDs),
		pairedUsernames: buildDiscordUsernameSet(cfg.DM.PairedUsernames),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(200, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(24),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	if resolved.webhookPath == "" {
		resolved.configError = errors.New("discord: webhook path is required")
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

func (p *discordProvider) determineInitialState(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg == nil {
		return bridgepkg.BridgeStatusError, nil, errors.New("discord: config is required")
	}
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.botToken) == "" {
		err := errors.New("discord: bot_token secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if _, err := decodeDiscordPublicKey(cfg.publicKey); err != nil {
		wrapped := fmt.Errorf("discord: public_key secret binding invalid: %w", err)
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: wrapped.Error(),
		}, wrapped
	}
	bot, err := p.apiFactory(*cfg).GetBotUser(ctx)
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
	if cfg.applicationID != "" && strings.TrimSpace(bot.ID) != "" && strings.TrimSpace(bot.ID) != cfg.applicationID {
		err := fmt.Errorf("discord: application_id %q does not match bot identity %q", cfg.applicationID, bot.ID)
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: err.Error(),
		}, err
	}
	return bridgepkg.BridgeStatusReady, nil, nil
}

func (p *discordProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}
