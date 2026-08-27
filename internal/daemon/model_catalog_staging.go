package daemon

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sort"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

type stagedLiveProviderConfigs struct {
	effective map[string]compozyconfig.ProviderConfig
	sources   map[string]*modelcatalog.LiveProviderSource
	previous  map[string]*modelcatalog.LiveProviderSource
	changed   []string
}

type stagedModelCatalogGeneration struct {
	service        modelCatalogGenerationService
	plan           *modelcatalog.RefreshPlan
	providers      map[string]compozyconfig.ProviderConfig
	reasoningApply map[string]bool
	live           stagedLiveProviderConfigs
}

func (r *modelCatalogRuntime) stageModelCatalogGeneration(
	ctx context.Context,
	cfg *compozyconfig.Config,
) (stagedModelCatalogGeneration, error) {
	reasoningApply, err := effectiveCatalogReasoningApply(cfg)
	if err != nil {
		return stagedModelCatalogGeneration{}, err
	}
	providers := map[string]compozyconfig.ProviderConfig(nil)
	if cfg != nil {
		providers = compozyconfig.CloneProviderConfigs(cfg.Providers)
	}
	live, err := r.stageLiveProviderConfigs(providers)
	if err != nil {
		return stagedModelCatalogGeneration{}, err
	}
	service, ok := r.service.(modelCatalogGenerationService)
	if !ok {
		return stagedModelCatalogGeneration{}, errors.New(
			"daemon: model catalog service cannot publish atomic generations",
		)
	}
	plan, err := service.NewRefreshPlan()
	if err != nil {
		return stagedModelCatalogGeneration{}, fmt.Errorf(
			"daemon: create model catalog refresh plan: %w",
			err,
		)
	}
	now := time.Now().UTC()
	if r.now != nil {
		now = r.now().UTC()
	}
	configSource := modelcatalog.NewConfigSource(providers)
	if err := r.refreshStagedConfig(ctx, plan, configSource, providers, now); err != nil {
		return stagedModelCatalogGeneration{}, err
	}
	if err := refreshStagedLive(ctx, plan, live, r.catalogExecutionContexts(), now); err != nil {
		return stagedModelCatalogGeneration{}, err
	}
	return stagedModelCatalogGeneration{
		service:        service,
		plan:           plan,
		providers:      providers,
		reasoningApply: reasoningApply,
		live:           live,
	}, nil
}

func (r *modelCatalogRuntime) refreshStagedConfig(
	ctx context.Context,
	plan *modelcatalog.RefreshPlan,
	source *modelcatalog.ProviderConfigSource,
	providers map[string]compozyconfig.ProviderConfig,
	now time.Time,
) error {
	providerIDs := reconcileProviderIDs(r.configSource.ProviderIDs(), providers)
	for _, providerID := range providerIDs {
		if _, err := plan.Refresh(ctx, source, modelcatalog.RefreshOptions{
			ProviderID:       providerID,
			ExecutionContext: modelcatalog.GlobalCatalogExecutionContext(),
			Force:            true,
			Now:              now,
		}); err != nil {
			return fmt.Errorf("daemon: reconcile model catalog config provider %q: %w", providerID, err)
		}
	}
	return nil
}

func refreshStagedLive(
	ctx context.Context,
	plan *modelcatalog.RefreshPlan,
	live stagedLiveProviderConfigs,
	executionContexts []modelcatalog.CatalogExecutionContext,
	now time.Time,
) error {
	for _, executionContext := range executionContexts {
		for _, providerID := range live.changed {
			previousExecutionContext, err := previousLiveExecutionContext(
				live.previous[providerID],
				executionContext,
			)
			if err != nil {
				return fmt.Errorf(
					"daemon: resolve prior live model catalog context for %q: %w",
					providerID,
					err,
				)
			}
			statuses, refreshErr := plan.Refresh(ctx, live.sources[providerID], modelcatalog.RefreshOptions{
				ProviderID:               providerID,
				ExecutionContext:         executionContext,
				PreviousExecutionContext: previousExecutionContext,
				Force:                    true,
				Now:                      now,
			})
			if refreshErr == nil {
				continue
			}
			if ctx.Err() != nil {
				return fmt.Errorf("daemon: reconcile live model catalog provider %q: %w", providerID, ctx.Err())
			}
			if !recordedLiveRefreshFailure(statuses, providerID) {
				return fmt.Errorf("daemon: reconcile live model catalog provider %q: %w", providerID, refreshErr)
			}
		}
	}
	return nil
}

func (r *modelCatalogRuntime) publishModelCatalogGeneration(generation stagedModelCatalogGeneration) {
	r.configSource.ReplaceProviders(generation.providers)
	if updater, ok := r.service.(modelCatalogMergeOptionsUpdater); ok {
		updater.UpdateMergeOptions(modelcatalog.MergeOptions{ReasoningApply: generation.reasoningApply})
	}
	for providerID, source := range r.liveSources {
		source.ReplaceProvider(generation.live.effective[providerID])
	}
}

func reconcileProviderIDs(
	previous []string,
	next map[string]compozyconfig.ProviderConfig,
) []string {
	providerSet := make(map[string]struct{}, len(previous)+len(next))
	for _, providerID := range previous {
		providerSet[providerID] = struct{}{}
	}
	for providerID := range next {
		providerSet[providerID] = struct{}{}
	}
	providerIDs := make([]string, 0, len(providerSet))
	for providerID := range providerSet {
		providerIDs = append(providerIDs, providerID)
	}
	sort.Strings(providerIDs)
	return providerIDs
}

func (r *modelCatalogRuntime) stageLiveProviderConfigs(
	providers map[string]compozyconfig.ProviderConfig,
) (stagedLiveProviderConfigs, error) {
	effective := compozyconfig.BuiltinProviders()
	maps.Copy(effective, providers)
	staged := stagedLiveProviderConfigs{
		effective: effective,
		sources:   make(map[string]*modelcatalog.LiveProviderSource, len(r.liveSources)),
		previous:  make(map[string]*modelcatalog.LiveProviderSource, len(r.liveSources)),
		changed:   make([]string, 0, len(r.liveSources)),
	}
	for providerID, source := range r.liveSources {
		clone, changed, err := source.CloneWithProvider(effective[providerID])
		if err != nil {
			return stagedLiveProviderConfigs{}, fmt.Errorf(
				"daemon: stage live model catalog provider %q: %w",
				providerID,
				err,
			)
		}
		staged.sources[providerID] = clone
		staged.previous[providerID] = source
		if changed {
			staged.changed = append(staged.changed, providerID)
		}
	}
	sort.Strings(staged.changed)
	return staged, nil
}

func previousLiveExecutionContext(
	source *modelcatalog.LiveProviderSource,
	executionContext modelcatalog.CatalogExecutionContext,
) (modelcatalog.CatalogExecutionContext, error) {
	if source == nil {
		return modelcatalog.CatalogExecutionContext{}, errors.New(
			"daemon: previous live model catalog source is required",
		)
	}
	fingerprint, err := source.CatalogExecutionFingerprint()
	if err != nil {
		return modelcatalog.CatalogExecutionContext{}, err
	}
	return executionContext.WithCommandFingerprint(fingerprint)
}
