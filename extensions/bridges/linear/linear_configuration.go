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

func (p *linearProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:    p.routes,
		Resolve:   p.resolveInstanceConfig,
		Prepare:   p.prepareLinearConfigs,
		Finalize:  p.finalizeLinearConfigs,
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

func (p *linearProvider) finalizeLinearConfigs(
	ctx context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	for idx := range configs {
		updated, status, degradation, probeErr := p.determineInitialState(ctx, configs[idx])
		if probeErr != nil {
			p.setLastError(probeErr)
		}
		updated.initialStatus = status
		updated.initialDegradation = degradation
		configs[idx] = updated
	}
	return configs, nil
}

func (p *linearProvider) prepareLinearConfigs(
	_ context.Context,
	_ *bridgesdk.Session,
	configs []resolvedInstanceConfig,
) ([]resolvedInstanceConfig, error) {
	if len(configs) == 0 {
		return configs, nil
	}
	requestedListen := strings.TrimSpace(os.Getenv(linearListenAddrEnv))
	seenOwnership := make(map[string]string, len(configs))

	for idx := range configs {
		cfg := &configs[idx]
		if cfg.listenAddr != "" {
			if requestedListen == "" {
				requestedListen = cfg.listenAddr
			} else if requestedListen != cfg.listenAddr && cfg.configError == nil {
				cfg.configError = fmt.Errorf(
					"linear: instance %q configured incompatible listen_addr %q (runtime uses %q)",
					cfg.instanceID,
					cfg.listenAddr,
					requestedListen,
				)
			}
		}
		ownershipKey := cfg.ownershipKey()
		if owner, ok := seenOwnership[ownershipKey]; ok && ownershipKey != "" && cfg.configError == nil {
			cfg.configError = fmt.Errorf(
				"linear: organization %q mode %q already belongs to %q and cannot also belong to %q",
				cfg.organizationID,
				cfg.mode,
				owner,
				cfg.instanceID,
			)
		}
		if ownershipKey != "" {
			seenOwnership[ownershipKey] = cfg.instanceID
		}
	}

	if requestedListen == "" {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = errors.New("linear: webhook listen address is required")
			}
		}
	} else if err := p.startServer(requestedListen); err != nil {
		for idx := range configs {
			if configs[idx].configError == nil {
				configs[idx].configError = err
			}
		}
	}

	return configs, nil
}

func (p *linearProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	webhookSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "webhook_secret")
	apiKey, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "api_key")
	clientID, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "client_id")
	clientSecret, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "client_secret")

	return resolveLinearInstanceConfig(
		managed,
		instanceSecretValues{
			webhookSecret: webhookSecret,
			apiKey:        apiKey,
			clientID:      clientID,
			clientSecret:  clientSecret,
		},
		resolveLinearEnv{
			listenAddr: strings.TrimSpace(os.Getenv(linearListenAddrEnv)),
			apiBaseURL: strings.TrimSpace(os.Getenv(linearAPIBaseEnv)),
			tokenURL:   strings.TrimSpace(os.Getenv(linearOAuthTokenURLEnvName())),
		},
	)
}

type instanceSecretValues struct {
	webhookSecret string
	apiKey        string
	clientID      string
	clientSecret  string
}

type resolveLinearEnv struct {
	listenAddr string
	apiBaseURL string
	tokenURL   string
}

func resolveLinearInstanceConfig(
	managed subprocess.InitializeBridgeManagedInstance,
	secrets instanceSecretValues,
	env resolveLinearEnv,
) resolvedInstanceConfig {
	cfg := linearProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &cfg); err != nil {
			return resolvedInstanceConfig{
				managed:         &managed,
				instanceID:      strings.TrimSpace(managed.Instance.ID),
				configError:     fmt.Errorf("linear: decode provider_config for %q: %w", managed.Instance.ID, err),
				dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
				oauthTokenCache: &linearOAuthTokenCache{},
			}
		}
	}

	resolved := resolvedInstanceConfig{
		managed:         &managed,
		instanceID:      strings.TrimSpace(managed.Instance.ID),
		organizationID:  strings.TrimSpace(cfg.OrganizationID),
		mode:            normalizeLinearMode(cfg.Mode),
		authMode:        normalizeLinearAuthMode(cfg.AuthMode),
		listenAddr:      firstNonEmpty(cfg.Webhook.ListenAddr, env.listenAddr),
		webhookPath:     normalizeWebhookPath(firstNonEmpty(cfg.Webhook.Path, linearDefaultWebhookPath)),
		apiBaseURL:      normalizeURL(firstNonEmpty(env.apiBaseURL, linearDefaultAPIBaseURL)),
		oauthTokenURL:   normalizeURL(firstNonEmpty(env.tokenURL, linearDefaultOAuthTokenURL())),
		webhookSecret:   strings.TrimSpace(secrets.webhookSecret),
		apiKey:          strings.TrimSpace(secrets.apiKey),
		clientID:        strings.TrimSpace(secrets.clientID),
		clientSecret:    strings.TrimSpace(secrets.clientSecret),
		dedup:           bridgesdk.NewDedupCache(5*time.Minute, 4000),
		oauthTokenCache: &linearOAuthTokenCache{},
	}

	switch {
	case resolved.organizationID == "":
		resolved.configError = errors.New("linear: provider_config.organization_id is required")
	case resolved.mode == "":
		resolved.configError = errors.New("linear: provider_config.mode is required")
	case resolved.mode != linearModeComments && resolved.mode != linearModeAgentSessions:
		resolved.configError = fmt.Errorf("linear: unsupported provider_config.mode %q", cfg.Mode)
	case resolved.authMode == "":
		resolved.configError = errors.New("linear: provider_config.auth_mode is required")
	case resolved.authMode != linearAuthModeAPIKey && resolved.authMode != linearAuthModeOAuth:
		resolved.configError = fmt.Errorf("linear: unsupported provider_config.auth_mode %q", cfg.AuthMode)
	case resolved.webhookPath == "":
		resolved.configError = errors.New("linear: webhook path is required")
	case resolved.apiBaseURL == "":
		resolved.configError = errors.New("linear: api base url is required")
	case !validLinearCredentialedURL(resolved.apiBaseURL):
		resolved.configError = fmt.Errorf("linear: api base url %q is invalid", resolved.apiBaseURL)
	case resolved.authMode == linearAuthModeOAuth && resolved.oauthTokenURL == "":
		resolved.configError = errors.New("linear: oauth token url is required for oauth auth_mode")
	case resolved.authMode == linearAuthModeOAuth && !validLinearCredentialedURL(resolved.oauthTokenURL):
		resolved.configError = fmt.Errorf("linear: oauth token url %q is invalid", resolved.oauthTokenURL)
	}

	return resolved
}

func (p *linearProvider) determineInitialState(
	ctx context.Context,
	cfg resolvedInstanceConfig,
) (resolvedInstanceConfig, bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg.configError != nil {
		return cfg, bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.webhookSecret) == "" {
		err := errors.New("linear: webhook_secret secret binding is required")
		return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}

	switch cfg.authMode {
	case linearAuthModeAPIKey:
		if strings.TrimSpace(cfg.apiKey) == "" {
			err := errors.New("linear: api_key secret binding is required for api_key auth_mode")
			return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: err.Error(),
			}, err
		}
	case linearAuthModeOAuth:
		if strings.TrimSpace(cfg.clientID) == "" || strings.TrimSpace(cfg.clientSecret) == "" {
			err := errors.New("linear: client_id and client_secret secret bindings are required for oauth auth_mode")
			return cfg, bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: err.Error(),
			}, err
		}
	}

	viewer, err := p.apiFactory(cfg).ValidateAuth(ctx)
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

	if viewer != nil {
		if strings.TrimSpace(cfg.organizationID) != "" && strings.TrimSpace(viewer.OrganizationID) != "" &&
			!strings.EqualFold(strings.TrimSpace(cfg.organizationID), strings.TrimSpace(viewer.OrganizationID)) {
			err := fmt.Errorf(
				"linear: provider_config.organization_id %q does not match authenticated organization %q",
				cfg.organizationID,
				viewer.OrganizationID,
			)
			return cfg, bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
				Message: err.Error(),
			}, err
		}
		cfg.botUserID = strings.TrimSpace(viewer.ID)
		cfg.botDisplayName = strings.TrimSpace(viewer.DisplayName)
	}
	return cfg, bridgepkg.BridgeStatusReady, nil, nil
}
