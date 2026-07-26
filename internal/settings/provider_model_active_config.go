package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"
)

func (s *service) recordProviderModelsMutationApply(
	ctx context.Context,
	result MutationResult,
	providerID string,
) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, desiredConfig, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	nextActiveConfig, err := projectProviderModelsActiveConfig(&state.config, &desiredConfig, providerID)
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

func (s *service) recordProjectedMutationApply(
	ctx context.Context,
	result MutationResult,
	state *activeSnapshot,
	desiredHash string,
	nextActiveHash string,
	nextActiveConfig *aghconfig.Config,
) (ApplyResult, error) {
	configLifecycle := mutationLifecycle(result)
	noChanges := mutationResultHasNoChanges(result)
	record, plan, err := s.persistRuntimeApply(
		ctx,
		state,
		desiredHash,
		nextActiveHash,
		nextActiveConfig,
		configLifecycle,
		noChanges,
	)
	if err != nil {
		return ApplyResult{}, err
	}
	return ApplyResult{
		Record:          record,
		Section:         result.Section,
		Scope:           result.Scope,
		WriteTarget:     result.WriteTarget,
		WorkspaceID:     result.WorkspaceID,
		AgentName:       result.AgentName,
		Applied:         plan.applied,
		NextAction:      lifecycle.NextActionForLifecycle(configLifecycle, plan.status),
		RestartRequired: configLifecycle == lifecycle.RestartRequired,
		RestartScope:    restartScopeForLifecycle(configLifecycle),
		Warnings:        append([]string(nil), result.Warnings...),
		PartialFailures: plan.partialFailures,
		Skipped:         noChanges,
		SkippedReason:   skippedReason(noChanges),
		MCPServer:       cloneMCPServerItemPointer(result.MCPServer),
	}, nil
}

func projectProviderModelsActiveConfig(
	active *aghconfig.Config,
	desired *aghconfig.Config,
	providerID string,
) (aghconfig.Config, error) {
	if active == nil || desired == nil {
		return aghconfig.Config{}, errors.New("settings: active and desired provider configs are required")
	}
	normalizedProviderID := strings.TrimSpace(providerID)
	if normalizedProviderID == "" {
		return aghconfig.Config{}, errors.New("settings: provider id is required for live model apply")
	}
	desiredProvider, ok := desired.Providers[normalizedProviderID]
	if !ok {
		return aghconfig.Config{}, fmt.Errorf(
			"settings: desired provider %q is required for live model apply",
			normalizedProviderID,
		)
	}
	projected := cloneActiveConfig(active)
	activeProvider, ok := projected.Providers[normalizedProviderID]
	if !ok {
		resolvedActiveProvider, err := active.ResolveProvider(normalizedProviderID)
		if err != nil {
			return aghconfig.Config{}, fmt.Errorf(
				"settings: resolve active provider %q for live model apply: %w",
				normalizedProviderID,
				err,
			)
		}
		activeProvider = resolvedActiveProvider
	}
	activeProvider.Models = cloneProviderModelsConfig(desiredProvider.Models)
	if projected.Providers == nil {
		projected.Providers = make(map[string]aghconfig.ProviderConfig)
	}
	projected.Providers[normalizedProviderID] = activeProvider
	return projected, nil
}
