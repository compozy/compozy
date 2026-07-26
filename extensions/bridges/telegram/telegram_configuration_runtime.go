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

func (p *telegramProvider) stopResources() {
	p.closeAllProgressDispatchers()
	for id, cfg := range p.routes.Snapshot() {
		if cfg.batcher == nil {
			continue
		}
		cfg.batcher.Close()
		p.routes.Update(id, func(current resolvedInstanceConfig) resolvedInstanceConfig {
			current.batcher = nil
			return current
		})
	}
}

func (p *telegramProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareTelegramManagedConfigs,
		Finalize: p.populateTelegramInitialStates,
		Identity: func(config resolvedInstanceConfig) string { return config.instanceID },
		Merge: func(prior resolvedInstanceConfig, next resolvedInstanceConfig) resolvedInstanceConfig {
			if prior.batcher != nil && prior.batcher != next.batcher {
				prior.batcher.Close()
			}
			return next
		},
		OnRemoved: func(config resolvedInstanceConfig) error {
			if config.batcher != nil {
				config.batcher.Close()
			}
			return nil
		},
	}
	configs, err := reconciler.Reconcile(ctx, session, managed)
	if err != nil {
		if !errors.Is(err, bridgesdk.ErrProviderStopped) {
			p.setLastError(err)
		}
		return nil
	}
	return configs
}

func (p *telegramProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg, err := decodeTelegramProviderConfig(managed)
	if err != nil {
		return resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			configError: fmt.Errorf(
				"telegram: decode provider_config for %q: %w",
				managed.Instance.ID,
				err,
			),
		}
	}

	resolved := buildTelegramResolvedInstance(session, managed, cfg)
	validateTelegramResolvedConfig(&resolved)
	if resolved.configError != nil {
		return resolved
	}
	configureTelegramBatcher(p, cfg, &resolved)
	return resolved
}

func decodeTelegramProviderConfig(managed subprocess.InitializeBridgeManagedInstance) (telegramProviderConfig, error) {
	cfg := telegramProviderConfig{}
	if len(managed.Instance.ProviderConfig) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
		return telegramProviderConfig{}, err
	}
	return cfg, nil
}

func buildTelegramResolvedInstance(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
	cfg telegramProviderConfig,
) resolvedInstanceConfig {
	botToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "bot_token")
	webhookSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "webhook_secret")

	resolved := resolvedInstanceConfig{
		managed:    &managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		listenAddr: firstNonEmpty(
			cfg.Webhook.ListenAddr,
			strings.TrimSpace(os.Getenv(telegramListenAddrEnv)),
		),
		webhookPath: normalizeWebhookPath(
			firstNonEmpty(cfg.Webhook.Path, "/telegram/"+strings.TrimSpace(managed.Instance.ID)),
		),
		apiBaseURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(telegramAPIBaseEnv)),
				telegramDefaultAPIBaseURL,
			),
		),
		botToken:        strings.TrimSpace(botToken),
		webhookSecret:   strings.TrimSpace(webhookSecret),
		dmPolicy:        managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:    buildIdentitySet(cfg.DM.AllowUserIDs),
		allowUsernames:  buildIdentitySet(cfg.DM.AllowUsernames),
		pairedUserIDs:   buildIdentitySet(cfg.DM.PairedUserIDs),
		pairedUsernames: buildIdentitySet(cfg.DM.PairedUsernames),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 2000),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(100, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(16),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	return resolved
}

func validateTelegramResolvedConfig(resolved *resolvedInstanceConfig) {
	if resolved == nil {
		return
	}
	switch {
	case resolved.webhookPath == "":
		resolved.configError = errors.New("telegram: webhook path is required")
	case strings.TrimSpace(resolved.listenAddr) != "" && strings.TrimSpace(resolved.webhookSecret) == "":
		resolved.configError = errors.New("telegram: webhook secret is required when webhook listener is enabled")
	}
}

func configureTelegramBatcher(
	provider *telegramProvider,
	cfg telegramProviderConfig,
	resolved *resolvedInstanceConfig,
) {
	if resolved == nil || cfg.Batching.DelayMS <= 0 {
		return
	}

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
			return provider.dispatchInboundBatch(ctx, resolved.instanceID, batch)
		},
		Now: func() time.Time { return provider.now() },
	})
	if err != nil {
		resolved.configError = err
		return
	}
	resolved.batcher = batcher
}

func (p *telegramProvider) determineInitialState(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg == nil {
		err := errors.New("telegram: resolved instance config is required")
		return bridgepkg.BridgeStatusError, nil, err
	}
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.botToken) == "" {
		err := errors.New("telegram: bot_token secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	_, err := p.apiFactory(cfg).GetMe(ctx)
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
