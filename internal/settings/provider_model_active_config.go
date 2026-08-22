package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
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
	nextActiveConfig *compozyconfig.Config,
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
		result.WriteTarget,
		result.writePath,
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
		ProfileName:     result.ProfileName,
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
	active *compozyconfig.Config,
	desired *compozyconfig.Config,
	providerID string,
) (compozyconfig.Config, error) {
	if active == nil || desired == nil {
		return compozyconfig.Config{}, errors.New("settings: active and desired provider configs are required")
	}
	normalizedProviderID := strings.TrimSpace(providerID)
	if normalizedProviderID == "" {
		return compozyconfig.Config{}, errors.New("settings: provider id is required for live model apply")
	}
	desiredProvider, ok := desired.Providers[normalizedProviderID]
	if !ok {
		return compozyconfig.Config{}, fmt.Errorf(
			"settings: desired provider %q is required for live model apply",
			normalizedProviderID,
		)
	}
	projected := cloneActiveConfig(active)
	activeProvider, ok := projected.Providers[normalizedProviderID]
	if !ok {
		resolvedActiveProvider, err := active.ResolveProvider(normalizedProviderID)
		if err != nil {
			return compozyconfig.Config{}, fmt.Errorf(
				"settings: resolve active provider %q for live model apply: %w",
				normalizedProviderID,
				err,
			)
		}
		activeProvider = resolvedActiveProvider
	}
	activeProvider.Models = cloneProviderModelsConfig(desiredProvider.Models)
	if projected.Providers == nil {
		projected.Providers = make(map[string]compozyconfig.ProviderConfig)
	}
	projected.Providers[normalizedProviderID] = activeProvider
	return projected, nil
}
