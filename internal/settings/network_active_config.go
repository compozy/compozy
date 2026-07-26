package settings

import (
	"context"
	"errors"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

func (s *service) recordNetworkSectionApply(ctx context.Context, result MutationResult) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	_, desired, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	availabilityChanged := state.config.Network.Enabled != desired.Network.Enabled
	if !availabilityChanged {
		return s.recordMutationApply(ctx, result)
	}
	if mutationLifecycle(result) == lifecycle.Live {
		return s.recordNetworkMutationApply(ctx, result)
	}

	liveResult := result
	liveResult.Lifecycle = lifecycle.Live
	liveResult.DiffClass = lifecycle.DiffClassLive
	if _, err := s.recordNetworkMutationApply(ctx, liveResult); err != nil {
		return ApplyResult{}, err
	}
	return s.recordMutationApply(ctx, result)
}

func (s *service) recordNetworkMutationApply(
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
	nextActiveConfig, err := projectNetworkActiveConfig(&state.config, &desiredConfig)
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

func projectNetworkActiveConfig(
	active *aghconfig.Config,
	desired *aghconfig.Config,
) (aghconfig.Config, error) {
	if active == nil || desired == nil {
		return aghconfig.Config{}, errors.New("settings: active and desired Network configs are required")
	}
	projected := cloneActiveConfig(active)
	projected.Network.Enabled = desired.Network.Enabled
	return projected, nil
}
