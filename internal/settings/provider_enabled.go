package settings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
)

// SetProviderEnabled edits only providers.<name>.enabled in the global overlay, under the settings
// mutation serialization, and applies it live (BR-10, BR-11). enabled=false writes the key;
// enabled=true deletes it and deletes the provider table when it becomes empty. Other keys are
// never read or rewritten. Disabling the desired or active default provider is rejected before
// anything is written.
func (s *service) SetProviderEnabled(
	ctx context.Context,
	name string,
	enabled bool,
) (ProviderItem, ApplyResult, error) {
	if ctx == nil {
		return ProviderItem{}, ApplyResult{}, errors.New("settings: provider enabled context is required")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	providerID := compozyconfig.CanonicalProviderName(strings.TrimSpace(name))
	currentConfig, _, err := s.loadConfig(ctx, ScopeUser, "", "")
	if err != nil {
		return ProviderItem{}, ApplyResult{}, err
	}
	if !providerDeclared(&currentConfig, providerID) {
		return ProviderItem{}, ApplyResult{}, notFoundError(fmt.Errorf("settings: provider %q not found", name))
	}
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ProviderItem{}, ApplyResult{}, err
	}
	authored := enabledOverlayValue(enabled)
	nextConfig := withProviderEnabled(&currentConfig, providerID, authored)
	if err := nextConfig.Validate(); err != nil {
		return ProviderItem{}, ApplyResult{}, unprocessableError(err)
	}
	projected := withProviderEnabled(&state.config, providerID, authored)
	if err := validateEnabledProjection(&projected); err != nil {
		return ProviderItem{}, ApplyResult{}, unprocessableError(err)
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return ProviderItem{}, ApplyResult{}, err
	}
	mutation := mutationResultAtPath(mutationResultForProvider(target.Kind(), true), target.Path())
	fileCurrent := reflect.DeepEqual(currentConfig.Providers[providerID].Enabled, authored)
	activeCurrent := reflect.DeepEqual(state.config.Providers[providerID].Enabled, authored)
	switch {
	case fileCurrent && activeCurrent:
		mutation.Warnings = []string{sectionsNoChangesValue}
	case fileCurrent:
		// The file already holds the value but an earlier apply did not reach the active config:
		// retry the projection instead of reporting a no-op (BR-10).
	default:
		if err := writeProviderEnabled(s.homePaths, target, providerID, authored); err != nil {
			return ProviderItem{}, ApplyResult{}, err
		}
		if err := s.emitSettingsChanged(ctx, mutation, "replace"); err != nil {
			return ProviderItem{}, ApplyResult{}, err
		}
	}
	apply, err := s.recordProviderEnabledApply(ctx, mutation, []string{providerID})
	if err != nil {
		return ProviderItem{}, ApplyResult{}, err
	}
	item, err := s.providerItemByName(ctx, providerID)
	if err != nil {
		return ProviderItem{}, ApplyResult{}, err
	}
	return item, apply, nil
}

func providerDeclared(cfg *compozyconfig.Config, providerID string) bool {
	if providerID == "" {
		return false
	}
	if _, builtin := compozyconfig.BuiltinProviders()[providerID]; builtin {
		return true
	}
	_, configured := cfg.Providers[providerID]
	return configured
}

// enabledOverlayValue maps a requested state to its authored form: enabling removes the key.
func enabledOverlayValue(enabled bool) *bool {
	if enabled {
		return nil
	}
	return new(false)
}

func writeProviderEnabled(
	homePaths compozyconfig.HomePaths,
	target compozyconfig.WriteTarget,
	providerID string,
	authored *bool,
) error {
	path := []string{string(CollectionProviders), providerID, sectionsEnabledKey}
	if _, err := compozyconfig.EditConfigOverlay(
		homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if authored == nil {
				return editor.DeleteKeyPruningEmptyTable(path)
			}
			return editor.SetValue(path, *authored)
		},
	); err != nil {
		return fmt.Errorf("settings: write provider %q enabled: %w", providerID, err)
	}
	return nil
}

// withProviderEnabled clones cfg with providers.<id>.enabled set to authored. An override entry
// left with no fields is removed so the projection hashes like a config without that block.
func withProviderEnabled(cfg *compozyconfig.Config, providerID string, authored *bool) compozyconfig.Config {
	projected := cloneActiveConfig(cfg)
	if projected.Providers == nil {
		projected.Providers = make(map[string]compozyconfig.ProviderConfig)
	}
	provider := projected.Providers[providerID]
	provider.Enabled = cloneEnabled(authored)
	if reflect.DeepEqual(provider, compozyconfig.ProviderConfig{}) {
		delete(projected.Providers, providerID)
	} else {
		projected.Providers[providerID] = provider
	}
	return projected
}

// projectProvidersEnabled copies only the desired enabled values of providerIDs onto the active
// config; every other pending field stays pending.
func projectProvidersEnabled(
	active *compozyconfig.Config,
	desired *compozyconfig.Config,
	providerIDs []string,
) compozyconfig.Config {
	projected := cloneActiveConfig(active)
	for _, providerID := range providerIDs {
		projected = withProviderEnabled(&projected, providerID, desired.Providers[providerID].Enabled)
	}
	return projected
}

// validateEnabledProjection validates a partially applied active config (B-013). A default
// provider rejection names the pending defaults.provider change.
func validateEnabledProjection(projected *compozyconfig.Config) error {
	err := projected.Validate()
	if defaultErr, ok := errors.AsType[*compozyconfig.ProviderDefaultDisableError](err); ok {
		return &compozyconfig.ProviderDefaultDisableError{Provider: defaultErr.Provider, PendingDefault: true}
	}
	return err
}

// changedProviderEnabledIDs lists providers whose authored enabled value differs.
func changedProviderEnabledIDs(active *compozyconfig.Config, desired *compozyconfig.Config) []string {
	ids := slices.Sorted(maps.Keys(active.Providers))
	for id := range desired.Providers {
		if _, ok := active.Providers[id]; !ok {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	changed := make([]string, 0, len(ids))
	for _, id := range ids {
		if !reflect.DeepEqual(active.Providers[id].Enabled, desired.Providers[id].Enabled) {
			changed = append(changed, id)
		}
	}
	return changed
}

func (s *service) recordProviderEnabledApply(
	ctx context.Context,
	result MutationResult,
	providerIDs []string,
) (ApplyResult, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return ApplyResult{}, err
	}
	desiredHash, desiredConfig, err := s.currentDesiredConfigHash()
	if err != nil {
		return ApplyResult{}, err
	}
	nextActiveConfig := projectProvidersEnabled(&state.config, &desiredConfig, providerIDs)
	if err := validateEnabledProjection(&nextActiveConfig); err != nil {
		return ApplyResult{}, unprocessableError(err)
	}
	nextActiveHash, err := hashConfigSnapshot(&nextActiveConfig)
	if err != nil {
		return ApplyResult{}, err
	}
	apply, err := s.recordProjectedMutationApply(ctx, result, &state, desiredHash, nextActiveHash, &nextActiveConfig)
	if err != nil {
		return ApplyResult{}, err
	}
	reportPendingRestart(&apply, &nextActiveConfig, &desiredConfig, desiredHash != nextActiveHash)
	return apply, nil
}

// reportPendingRestart keeps the enabled apply live but reports restart-required edits that the
// projection left pending, so callers still see the restart next action.
func reportPendingRestart(
	apply *ApplyResult,
	active *compozyconfig.Config,
	desired *compozyconfig.Config,
	diverged bool,
) {
	if !diverged || apply.NextAction == lifecycle.NextActionRetry ||
		classifyReloadLifecycle(active, desired) != lifecycle.RestartRequired {
		return
	}
	apply.RestartRequired = true
	apply.RestartScope = restartScopeForLifecycle(lifecycle.RestartRequired)
	apply.NextAction = lifecycle.NextActionRestartDaemon
}

// reloadWithEnabledProjection splits a restart-required reload (B-008): the changed
// providers.*.enabled values apply live while every other path stays pending. An invalid
// projection is skipped with a warning and the whole reload stays pending.
func (s *service) reloadWithEnabledProjection(
	ctx context.Context,
	state *activeSnapshot,
	desiredHash string,
	desiredConfig *compozyconfig.Config,
	providerIDs []string,
) (ApplyResult, []string, bool, error) {
	projected := projectProvidersEnabled(&state.config, desiredConfig, providerIDs)
	if err := validateEnabledProjection(&projected); err != nil {
		warning := fmt.Sprintf("providers %s enabled change was not applied live: %v",
			strings.Join(providerIDs, ", "), err)
		return ApplyResult{}, []string{warning}, false, nil
	}
	projectedHash, err := hashConfigSnapshot(&projected)
	if err != nil {
		return ApplyResult{}, nil, false, err
	}
	record, plan, err := s.persistRuntimeApply(
		ctx, state, desiredHash, projectedHash, &projected, lifecycle.Live, false, "", "",
	)
	if err != nil {
		return ApplyResult{}, nil, false, err
	}
	nextAction := lifecycle.NextActionRestartDaemon
	if plan.status == lifecycle.StatusFailed {
		nextAction = lifecycle.NextActionRetry
	}
	return ApplyResult{
		Record:          record,
		Applied:         plan.applied,
		NextAction:      nextAction,
		RestartRequired: true,
		RestartScope:    restartScopeForLifecycle(lifecycle.RestartRequired),
		PartialFailures: plan.partialFailures,
	}, nil, true, nil
}

func (s *service) providerItemByName(ctx context.Context, providerID string) (ProviderItem, error) {
	cfg, _, err := s.loadConfig(ctx, ScopeUser, "", "")
	if err != nil {
		return ProviderItem{}, err
	}
	items, err := s.buildProviderItems(ctx, &cfg)
	if err != nil {
		return ProviderItem{}, err
	}
	for index := range items {
		if items[index].Name == providerID {
			return items[index], nil
		}
	}
	return ProviderItem{}, notFoundError(fmt.Errorf("settings: provider %q not found", providerID))
}

func cloneEnabled(value *bool) *bool {
	if value == nil {
		return nil
	}
	return new(*value)
}

// preserveAuthoredProviderEnabled copies the authored providers.<name>.enabled into a rebuilt
// provider table so a full edit never flips it (BR-11).
func (s *service) preserveAuthoredProviderEnabled(ctx context.Context, name string, values map[string]any) error {
	cfg, _, err := s.loadConfig(ctx, ScopeUser, "", "")
	if err != nil {
		return fmt.Errorf("settings: load provider %q enabled: %w", name, err)
	}
	if enabled := cfg.Providers[name].Enabled; enabled != nil {
		values[sectionsEnabledKey] = *enabled
	}
	return nil
}
