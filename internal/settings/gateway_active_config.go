package settings

import (
	"context"
	"errors"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
)

func (s *service) recordGatewaySectionApply(ctx context.Context, result MutationResult) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	_, desired, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	liveChanged := gatewayLiveFieldsChanged(state.config.Gateway, desired.Gateway)
	if !liveChanged {
		return s.recordMutationApply(ctx, result)
	}
	if mutationLifecycle(result) == lifecycle.Live {
		return s.recordGatewayMutationApply(ctx, result)
	}

	liveResult := result
	liveResult.Lifecycle = lifecycle.Live
	liveResult.DiffClass = lifecycle.DiffClassLive
	if _, err := s.recordGatewayMutationApply(ctx, liveResult); err != nil {
		return ApplyResult{}, err
	}
	return s.recordMutationApply(ctx, result)
}

func (s *service) recordGatewayMutationApply(
	ctx context.Context,
	result MutationResult,
) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, desiredConfig, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	nextActiveConfig, err := projectGatewayActiveConfig(&state.config, &desiredConfig)
	if err != nil {
		return ApplyResult{}, err
	}
	nextActiveHash, err := hashConfigSnapshot(&nextActiveConfig)
	if err != nil {
		return ApplyResult{}, err
	}
	return s.recordProjectedMutationApply(
		ctx,
		result,
		&state,
		desiredHash,
		nextActiveHash,
		&nextActiveConfig,
	)
}

func projectGatewayActiveConfig(
	active *compozyconfig.Config,
	desired *compozyconfig.Config,
) (compozyconfig.Config, error) {
	if active == nil || desired == nil {
		return compozyconfig.Config{}, errors.New("settings: active and desired Gateway configs are required")
	}
	projected := cloneActiveConfig(active)
	projected.Gateway.Enabled = desired.Gateway.Enabled
	projected.Gateway.Pairing = desired.Gateway.Pairing
	projected.Gateway.StreamTicket = desired.Gateway.StreamTicket
	projected.Gateway.Auth = desired.Gateway.Auth
	projected.Gateway.Verify = desired.Gateway.Verify
	return projected, nil
}

func gatewayLiveFieldsChanged(current compozyconfig.GatewayConfig, desired compozyconfig.GatewayConfig) bool {
	return current.Enabled != desired.Enabled ||
		current.Pairing != desired.Pairing ||
		current.StreamTicket != desired.StreamTicket ||
		current.Auth != desired.Auth ||
		current.Verify != desired.Verify
}
