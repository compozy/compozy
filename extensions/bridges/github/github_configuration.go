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

func (p *githubProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:    p.routes,
		Resolve:   p.resolveInstanceConfig,
		Prepare:   p.prepareGitHubConfigs,
		Finalize:  p.finalizeGitHubConfigs,
		Identity:  func(config resolvedInstanceConfig) string { return config.instanceID },
		OnPublish: p.lifecycle.MarkRoutesReady,
	}
	configs, err := reconciler.Reconcile(ctx, session, managed)
	if err != nil {
		p.setLastError(err)
		return nil
	}
	return configs
}

func (p *githubProvider) prepareGitHubConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		p.mu.Lock()
		p.installationCache = make(map[string]int64)
		p.apiClients = make(map[string]githubAPI)
		p.mu.Unlock()
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(githubListenAddrEnv))
	seenRepos := make(map[string]string, len(configs))

	for idx := range configs {
		requestedListen = applyGitHubListenConstraint(&configs[idx], requestedListen)
		applyGitHubRepoConflict(&configs[idx], seenRepos)
	}
	p.applyGitHubListenErrors(configs, requestedListen)
	p.mu.Lock()
	p.listenAddr = requestedListen
	p.apiClients = make(map[string]githubAPI, len(configs))
	p.mu.Unlock()
	return configs, nil
}

func applyGitHubListenConstraint(cfg *resolvedInstanceConfig, requestedListen string) string {
	if cfg == nil || cfg.listenAddr == "" {
		return requestedListen
	}
	if requestedListen == "" {
		return cfg.listenAddr
	}
	if requestedListen != cfg.listenAddr && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"github: instance %q configured incompatible listen_addr %q (runtime uses %q)",
			cfg.instanceID,
			cfg.listenAddr,
			requestedListen,
		)
	}
	return requestedListen
}

func applyGitHubRepoConflict(cfg *resolvedInstanceConfig, seenRepos map[string]string) {
	if cfg == nil || cfg.repoFullName == "" {
		return
	}
	if owner, ok := seenRepos[cfg.repoFullName]; ok && cfg.configError == nil {
		cfg.configError = fmt.Errorf(
			"github: repository %q is already owned by %q and cannot also belong to %q",
			cfg.repoFullName,
			owner,
			cfg.instanceID,
		)
	}
	seenRepos[cfg.repoFullName] = cfg.instanceID
}

func (p *githubProvider) applyGitHubListenErrors(configs []resolvedInstanceConfig, requestedListen string) {
	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("github: webhook listen address is required")
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

func (p *githubProvider) finalizeGitHubConfigs(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		updated, status, degradation, err := p.determineInitialState(ctx, &configs[idx])
		if err != nil {
			p.setLastError(err)
		}
		updated.initialStatus = status
		updated.initialDegradation = degradation
		configs[idx] = *updated
	}
	return configs, nil
}

func (p *githubProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg := githubProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
			return resolvedInstanceConfig{
				managed:     managed,
				instanceID:  managed.Instance.ID,
				configError: fmt.Errorf("github: decode provider_config for %q: %w", managed.Instance.ID, err),
			}
		}
	}

	webhookSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "webhook_secret")
	token, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "token")
	appID, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "app_id")
	privateKey, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "private_key")

	listenAddr := firstNonEmpty(cfg.Webhook.ListenAddr, strings.TrimSpace(os.Getenv(githubListenAddrEnv)))
	webhookPath := normalizeWebhookPath(firstNonEmpty(cfg.Webhook.Path, "/github"))
	apiBaseURL := normalizeURL(
		firstNonEmpty(strings.TrimSpace(os.Getenv(githubAPIBaseEnv)), githubDefaultAPIBaseURL),
	)
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode == "" {
		switch {
		case strings.TrimSpace(token) != "" && strings.TrimSpace(appID) == "" && strings.TrimSpace(privateKey) == "":
			mode = githubModePAT
		case strings.TrimSpace(token) == "" && strings.TrimSpace(appID) != "" && strings.TrimSpace(privateKey) != "":
			mode = githubModeApp
		}
	}

	repoOwner, repoName, repoFullName, repoErr := normalizeGitHubRepository(
		cfg.Repository.Owner,
		cfg.Repository.Name,
		cfg.Repository.FullName,
	)
	resolved := resolvedInstanceConfig{
		managed:        managed,
		instanceID:     strings.TrimSpace(managed.Instance.ID),
		listenAddr:     listenAddr,
		webhookPath:    webhookPath,
		apiBaseURL:     apiBaseURL,
		mode:           mode,
		repoOwner:      repoOwner,
		repoName:       repoName,
		repoFullName:   repoFullName,
		installationID: cfg.InstallationID,
		webhookSecret:  strings.TrimSpace(webhookSecret),
		token:          strings.TrimSpace(token),
		appID:          strings.TrimSpace(appID),
		privateKey:     strings.TrimSpace(privateKey),
		botLogin:       strings.TrimSpace(cfg.BotLogin),
		dedup:          bridgesdk.NewDedupCache(5*time.Minute, 4000),
	}
	switch {
	case repoErr != nil:
		resolved.configError = repoErr
	case resolved.webhookPath == "":
		resolved.configError = errors.New("github: webhook path is required")
	case resolved.mode != githubModePAT && resolved.mode != githubModeApp:
		resolved.configError = fmt.Errorf("github: provider mode must be %q or %q", githubModePAT, githubModeApp)
	case resolved.installationID < 0:
		resolved.configError = errors.New("github: installation_id must be non-negative")
	}

	return resolved
}

func (p *githubProvider) determineInitialState(
	ctx context.Context,
	cfg *resolvedInstanceConfig,
) (*resolvedInstanceConfig, bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg == nil {
		err := errors.New("github: config is required")
		return nil, bridgepkg.BridgeStatusError, nil, err
	}
	if cfg.configError != nil {
		return cfg, bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.webhookSecret) == "" {
		err := errors.New("github: webhook_secret secret binding is required")
		return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}

	switch cfg.mode {
	case githubModePAT:
		if strings.TrimSpace(cfg.token) == "" {
			err := errors.New("github: token secret binding is required for PAT mode")
			return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: err.Error(),
			}, err
		}
	case githubModeApp:
		if strings.TrimSpace(cfg.appID) == "" || strings.TrimSpace(cfg.privateKey) == "" {
			err := errors.New("github: app_id and private_key secret bindings are required for app mode")
			return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: err.Error(),
			}, err
		}
		if err := validateGitHubAppCredentials(cfg); err != nil {
			return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: err.Error(),
			}, err
		}
	}

	if cfg.mode == githubModeApp && cfg.installationID == 0 {
		return cfg, bridgepkg.BridgeStatusReady, nil, nil
	}

	viewer, err := p.apiFactory(*cfg).ValidateAuth(ctx, cfg.installationID)
	if err != nil {
		classified := bridgesdk.ClassifyError(err)
		recovery := classified.Recovery()
		status := recovery.Status
		if status == "" {
			status = bridgepkg.BridgeStatusError
		}
		if recovery.Degradation != nil {
			return cfg, status, recovery.Degradation, err
		}
		return cfg, status, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonProviderTimeout,
			Message: classified.Message,
		}, err
	}
	if strings.TrimSpace(cfg.botLogin) == "" && viewer != nil {
		cfg.botLogin = strings.TrimSpace(viewer.Login)
	}
	return cfg, bridgepkg.BridgeStatusReady, nil, nil
}
