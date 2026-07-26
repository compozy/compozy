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

func (p *teamsProvider) stopResources() {
	p.closeAllTeamsProgressDispatchers()
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

func (p *teamsProvider) reconcileInstanceConfigs(
	ctx context.Context,
	session *bridgesdk.Session,
	managed []subprocess.InitializeBridgeManagedInstance,
) []resolvedInstanceConfig {
	reconciler := bridgesdk.ManagedConfigReconciler[resolvedInstanceConfig]{
		Routes:   p.routes,
		Resolve:  p.resolveInstanceConfig,
		Prepare:  p.prepareTeamsManagedConfigs,
		Finalize: p.populateTeamsInitialStates,
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
			p.lifecycle.Host().ForgetStatus(config.instanceID)
			p.mu.Lock()
			delete(p.reportedHealth, config.instanceID)
			p.mu.Unlock()
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

func (p *teamsProvider) resolveInstanceConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) resolvedInstanceConfig {
	cfg, err := decodeTeamsProviderConfig(managed)
	if err != nil {
		return resolvedInstanceConfig{
			managed:    &managed,
			instanceID: managed.Instance.ID,
			configError: fmt.Errorf(
				"teams: decode provider_config for %q: %w",
				managed.Instance.ID,
				err,
			),
		}
	}

	resolved := buildTeamsResolvedInstance(session, managed, cfg)
	validateTeamsResolvedConfig(&resolved)
	if resolved.configError != nil {
		return resolved
	}
	configureTeamsBatcher(p, cfg, &resolved)
	return resolved
}

func (p *teamsProvider) determineInitialState(
	ctx context.Context,
	cfg resolvedInstanceConfig,
) (bridgepkg.BridgeStatus, *bridgepkg.BridgeDegradation, error) {
	if cfg.configError != nil {
		return bridgepkg.BridgeStatusDegraded, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonTenantConfigInvalid,
			Message: cfg.configError.Error(),
		}, cfg.configError
	}
	if strings.TrimSpace(cfg.appID) == "" {
		err := errors.New("teams: app_id secret binding is required")
		return bridgepkg.BridgeStatusAuthRequired, &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: err.Error(),
		}, err
	}
	if strings.TrimSpace(cfg.appPassword) == "" {
		err := errors.New("teams: app_password secret binding is required")
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

func (p *teamsProvider) startServer(listenAddr string) error {
	if err := p.http.Start(listenAddr); err != nil {
		return fmt.Errorf("teams: %w", err)
	}
	p.markers.RecordListen(p.http.Address())
	return nil
}
