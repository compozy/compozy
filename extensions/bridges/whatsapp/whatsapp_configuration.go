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

func (p *whatsappProvider) prepareWhatsAppManagedConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(whatsappListenAddrEnv))
	usedPaths := make(map[string]string, len(configs))

	for idx := range configs {
		requestedListen = updateWhatsAppRequestedListen(&configs[idx], requestedListen)
		markDuplicateWhatsAppWebhookPath(&configs[idx], usedPaths)
	}
	applyWhatsAppListenErrors(configs, requestedListen, p.startServer)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func updateWhatsAppRequestedListen(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"whatsapp: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func markDuplicateWhatsAppWebhookPath(cfg *resolvedInstanceConfig, usedPaths map[string]string) {
	if cfg == nil || cfg.webhookPath == "" {
		return
	}
	if owner, ok := usedPaths[cfg.webhookPath]; ok && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"whatsapp: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			owner,
			cfg.instanceID,
		)
	}
	usedPaths[cfg.webhookPath] = cfg.instanceID
}

func applyWhatsAppListenErrors(
	configs []resolvedInstanceConfig,
	requestedListen string,
	startServer func(string) error,
) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New(
					"whatsapp: webhook listen address is required",
				)
			}
		}
		return
	}

	if err := startServer(requestedListen); err != nil {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = err
			}
		}
	}
}

func (p *whatsappProvider) populateWhatsAppInitialStates(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		status, degradation, err := p.determineInitialState(ctx, configs[idx])
		if err != nil {
			p.setInstanceError(configs[idx].instanceID, err)
		} else {
			p.clearInstanceError(configs[idx].instanceID)
		}
		configs[idx].initialStatus = status
		configs[idx].initialDegradation = degradation
	}
	return configs, nil
}

func decodeWhatsAppProviderConfig(
	managed subprocess.InitializeBridgeManagedInstance,
) (whatsappProviderConfig, error) {
	cfg := whatsappProviderConfig{}
	if len(managed.Instance.ProviderConfig) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
		return whatsappProviderConfig{}, err
	}
	return cfg, nil
}

func buildWhatsAppResolvedInstance(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
	cfg whatsappProviderConfig,
) resolvedInstanceConfig {
	accessToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "access_token")
	appSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, whatsappAppSecretSlot)
	verifyToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, whatsappVerifyTokenSlot)

	resolved := resolvedInstanceConfig{
		managed:    &managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		listenAddr: firstNonEmpty(
			cfg.Webhook.ListenAddr,
			strings.TrimSpace(os.Getenv(whatsappListenAddrEnv)),
		),
		webhookPath: normalizeWebhookPath(
			firstNonEmpty(cfg.Webhook.Path, "/whatsapp/"+strings.TrimSpace(managed.Instance.ID)),
		),
		apiBaseURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(whatsappAPIBaseEnv)),
				whatsappDefaultAPIBaseURL,
			),
		),
		apiVersion:      firstNonEmpty(cfg.APIVersion, whatsappDefaultAPIVersion),
		phoneNumberID:   strings.TrimSpace(cfg.PhoneNumberID),
		accessToken:     strings.TrimSpace(accessToken),
		appSecret:       strings.TrimSpace(appSecret),
		verifyToken:     strings.TrimSpace(verifyToken),
		dmPolicy:        managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:    buildIdentitySet(cfg.DM.AllowUserIDs),
		allowUsernames:  buildIdentitySet(cfg.DM.AllowUsernames),
		pairedUserIDs:   buildIdentitySet(cfg.DM.PairedUserIDs),
		pairedUsernames: buildIdentitySet(cfg.DM.PairedUsernames),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(120, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(24),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	return resolved
}

func validateWhatsAppResolvedConfig(resolved *resolvedInstanceConfig) {
	if resolved == nil {
		return
	}
	switch {
	case resolved.webhookPath == "":
		resolved.configError = errors.New("whatsapp: webhook path is required")
	case resolved.phoneNumberID == "":
		resolved.configError = errors.New("whatsapp: provider_config.phone_number_id is required")
	}
}

func configureWhatsAppBatcher(
	provider *whatsappProvider,
	cfg whatsappProviderConfig,
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
