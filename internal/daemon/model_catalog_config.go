package daemon

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

type modelCatalogMergeOptionsUpdater interface {
	UpdateMergeOptions(options modelcatalog.MergeOptions)
}

type modelCatalogGenerationService interface {
	NewRefreshPlan() (*modelcatalog.RefreshPlan, error)
	CommitRefreshPlan(ctx context.Context, plan *modelcatalog.RefreshPlan) error
}

func (r *modelCatalogRuntime) ReconcileConfig(ctx context.Context, cfg *compozyconfig.Config) error {
	if r == nil || r.service == nil || r.configSource == nil {
		return errors.New("daemon: model catalog config source is unavailable")
	}
	if ctx == nil {
		return errors.New("daemon: model catalog config reconciliation context is required")
	}
	release, err := r.generationGate.lock(ctx, false)
	if err != nil {
		return fmt.Errorf("daemon: wait to reconcile model catalog generation: %w", err)
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("daemon: reconcile model catalog generation canceled: %w", err)
	}
	generationCtx, cancelGeneration := context.WithCancel(ctx)
	stopRootCancel := context.AfterFunc(r.ctx, cancelGeneration)
	defer func() {
		stopRootCancel()
		cancelGeneration()
	}()

	generation, err := r.stageModelCatalogGeneration(generationCtx, cfg)
	if err != nil {
		return err
	}
	if err := generationCtx.Err(); err != nil {
		return fmt.Errorf("daemon: reconcile model catalog generation canceled before publication: %w", err)
	}
	if err := generation.service.CommitRefreshPlan(generationCtx, generation.plan); err != nil {
		return fmt.Errorf("daemon: publish model catalog generation: %w", err)
	}
	r.publishModelCatalogGeneration(generation)
	return nil
}

func recordedLiveRefreshFailure(statuses []modelcatalog.SourceStatus, providerID string) bool {
	sourceID := modelcatalog.SourceKindProviderLiveID(providerID)
	for _, status := range statuses {
		if status.ProviderID == providerID &&
			status.SourceID == sourceID &&
			status.RefreshState == modelcatalog.RefreshStateFailed {
			return true
		}
	}
	return false
}
