package main

import (
	"context"

	"errors"
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *whatsappProvider) handleInitialize(_ context.Context, session *bridgesdk.Session) error {
	p.mu.Lock()
	p.initializeErrorSeq = p.lastErrorSeq
	p.mu.Unlock()
	return p.lifecycle.Initialize(context.Background(), session)
}

func (p *whatsappProvider) stopResources() {
	p.closeAllWhatsAppProgressDispatchers()
	batchers := make(map[*bridgesdk.InboundBatcher]struct{})
	for id, cfg := range p.routes.Snapshot() {
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

func (p *whatsappProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareWhatsAppManagedConfigs,
		Finalize: p.populateWhatsAppInitialStates,
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
		p.setLastError(err)
		return nil
	}
	return configs
}

func (p *whatsappProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg, err := decodeWhatsAppProviderConfig(managed)
	if err != nil {
		return resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			configError: fmt.Errorf(
				"whatsapp: decode provider_config for %q: %w",
				managed.Instance.ID,
				err,
			),
		}
	}

	resolved := buildWhatsAppResolvedInstance(session, managed, cfg)
	validateWhatsAppResolvedConfig(&resolved)
	if resolved.configError != nil {
		return resolved
	}
	configureWhatsAppBatcher(p, cfg, &resolved)
	return resolved
}

func (p *whatsappProvider) determineInitialState(
	ctx context.Context,
	cfg resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.accessToken) == "" {
		err := errors.New("whatsapp: access_token secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if strings.TrimSpace(cfg.appSecret) == "" {
		err := errors.New("whatsapp: app_secret secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if strings.TrimSpace(cfg.verifyToken) == "" {
		err := errors.New("whatsapp: verify_token secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}

	_, err := p.apiFactory(cfg).GetPhoneNumber(ctx, cfg.phoneNumberID)
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

func (p *whatsappProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("whatsapp: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}
