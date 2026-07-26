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

func (p *teamsProvider) prepareTeamsManagedConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(teamsListenAddrEnv))
	usedPaths := make(map[string]int, len(configs))

	for idx := range configs {
		requestedListen = updateTeamsRequestedListen(&configs[idx], requestedListen)
		markDuplicateTeamsWebhookPath(configs, idx, usedPaths)
	}
	applyTeamsListenErrors(configs, requestedListen, p.startServer)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.mu.Unlock()
	return configs, nil
}

func updateTeamsRequestedListen(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"teams: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func markDuplicateTeamsWebhookPath(
	configs []resolvedInstanceConfig,
	idx int,
	usedPaths map[string]int,
) {
	if idx < 0 || idx >= len(configs) {
		return
	}
	cfg := &configs[idx]
	if cfg.webhookPath == "" {
		return
	}
	if ownerIdx, ok := usedPaths[cfg.webhookPath]; ok {
		ownerID := strings.TrimSpace(configs[ownerIdx].instanceID)
		collisionErr := fmt.Errorf(
			"teams: webhook path %q is shared by %q and %q",
			cfg.webhookPath,
			ownerID,
			cfg.instanceID,
		)
		configs[ownerIdx].configError = joinTeamsConfigError(configs[ownerIdx].configError, collisionErr)
		cfg.configError = joinTeamsConfigError(cfg.configError, collisionErr)
		return
	}
	usedPaths[cfg.webhookPath] = idx
}

func joinTeamsConfigError(existing error, next error) error {
	if next == nil {
		return existing
	}
	if existing == nil {
		return next
	}
	return errors.Join(existing, next)
}

func applyTeamsListenErrors(
	configs []resolvedInstanceConfig,
	requestedListen string,
	startServer func(string) error,
) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("teams: webhook listen address is required")
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

func (p *teamsProvider) populateTeamsInitialStates(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		status, degradation, err := p.determineInitialState(ctx, configs[idx])
		if err != nil {
			p.setLastError(err)
		}
		configs[idx].initialStatus = status
		configs[idx].initialDegradation = degradation
	}
	return configs, nil
}

func decodeTeamsProviderConfig(
	managed subprocess.InitializeBridgeManagedInstance,
) (teamsProviderConfig, error) {
	cfg := teamsProviderConfig{}
	if len(managed.Instance.ProviderConfig) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
		return teamsProviderConfig{}, err
	}
	return cfg, nil
}

func buildTeamsResolvedInstance(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
	cfg teamsProviderConfig,
) resolvedInstanceConfig {
	appID, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "app_id")
	appPassword, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "app_password")
	appTenantID, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "app_tenant_id")

	resolved := resolvedInstanceConfig{
		managed:    &managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		listenAddr: firstNonEmpty(
			cfg.Webhook.ListenAddr,
			strings.TrimSpace(os.Getenv(teamsListenAddrEnv)),
		),
		webhookPath: normalizeWebhookPath(
			firstNonEmpty(cfg.Webhook.Path, "/teams/"+strings.TrimSpace(managed.Instance.ID)),
		),
		serviceURL: normalizeURL(firstNonEmpty(
			strings.TrimSpace(os.Getenv(teamsServiceURLEnv)),
			teamsDefaultServiceURL,
		)),
		openIDMetadataURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(teamsOpenIDMetadataURLEnv)),
				teamsDefaultOpenIDMetadata,
			),
		),
		tokenURL: normalizeURL(
			firstNonEmpty(
				strings.TrimSpace(os.Getenv(teamsOAuthTokenURLEnvName())),
				defaultTeamsTokenURL(strings.TrimSpace(appTenantID)),
			),
		),
		appID:           strings.TrimSpace(appID),
		appPassword:     strings.TrimSpace(appPassword),
		appTenantID:     strings.TrimSpace(appTenantID),
		dmPolicy:        managed.Instance.DMPolicy.Normalize(),
		allowUserIDs:    buildTeamsIDSet(cfg.DM.AllowUserIDs),
		allowUsernames:  buildTeamsUsernameSet(cfg.DM.AllowUsernames),
		pairedUserIDs:   buildTeamsIDSet(cfg.DM.PairedUserIDs),
		pairedUsernames: buildTeamsUsernameSet(cfg.DM.PairedUsernames),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		rateLimiter:     bridgesdk.NewFixedWindowRateLimiter(200, time.Minute),
		inFlightLimiter: bridgesdk.NewInFlightLimiter(24),
	}
	if resolved.dmPolicy == "" {
		resolved.dmPolicy = bridgepkg.BridgeDMPolicyOpen
	}
	return resolved
}

func validateTeamsResolvedConfig(resolved *resolvedInstanceConfig) {
	if resolved == nil {
		return
	}
	switch {
	case resolved.webhookPath == "":
		resolved.configError = errors.New("teams: webhook path is required")
	case resolved.serviceURL == "":
		resolved.configError = errors.New("teams: service url is required")
	case !validTeamsServiceURL(resolved.serviceURL):
		resolved.configError = fmt.Errorf(
			"teams: service url %q is invalid",
			resolved.serviceURL,
		)
	case resolved.openIDMetadataURL == "":
		resolved.configError = errors.New("teams: openid metadata url is required")
	case !validTeamsCredentialedURL(resolved.openIDMetadataURL):
		resolved.configError = fmt.Errorf("teams: openid metadata url %q is invalid", resolved.openIDMetadataURL)
	case resolved.tokenURL == "":
		resolved.configError = errors.New("teams: token url is required")
	case !validTeamsCredentialedURL(resolved.tokenURL):
		resolved.configError = fmt.Errorf("teams: token url %q is invalid", resolved.tokenURL)
	case resolved.appTenantID != "" && !looksLikeTenantID(resolved.appTenantID):
		resolved.configError = fmt.Errorf(
			"teams: app_tenant_id %q is malformed",
			resolved.appTenantID,
		)
	}
}

func configureTeamsBatcher(
	provider *teamsProvider,
	cfg teamsProviderConfig,
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
