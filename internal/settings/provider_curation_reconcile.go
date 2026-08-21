package settings

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
)

func (s *service) reconcileProviderCuratedWrite(
	ctx context.Context,
	providerID string,
	settings ProviderSettings,
	modelCuration *ProviderModelCurationRequest,
) (ProviderSettings, error) {
	if !settings.ModelsSet && modelCuration != nil {
		return ProviderSettings{}, validationError(
			errors.New("settings: provider model curation requires models in the provider payload"),
		)
	}
	if settings.ModelsSet && providerModelsWriteClearsConfig(settings.Models) {
		if modelCuration != nil {
			return ProviderSettings{}, validationError(
				errors.New("settings: provider model curation cannot accompany a models clear"),
			)
		}
		return settings, nil
	}
	cfg, _, err := s.loadConfig(ctx, ScopeUser, "")
	if err != nil {
		return ProviderSettings{}, err
	}
	rawProvider := cfg.Providers[providerID]
	if !settings.ModelsSet {
		settings.Models = cloneProviderModelsConfig(rawProvider.Models)
		return settings, nil
	}
	resolvedProvider, err := cfg.ResolveProvider(providerID)
	if err != nil {
		if modelCuration != nil {
			return ProviderSettings{}, providerModelNotFoundError(providerID, modelCuration.ModelID)
		}
		return settings, nil
	}
	raw := rawProvider.Models.Curated
	reconciled := cloneProviderModelConfigs(raw)
	if modelCuration != nil {
		if err := s.requireProviderModelCatalog(); err != nil {
			return ProviderSettings{}, err
		}
	}
	if settings.Models.Curated != nil {
		currentModels, currentErr := s.currentProviderModels(ctx, providerID)
		if currentErr != nil {
			return ProviderSettings{}, currentErr
		}
		reconciled = reconcileProviderCuratedRows(
			raw,
			resolvedProvider.Models.Curated,
			currentModels,
			settings.Models.Curated,
		)
	}
	if modelCuration != nil {
		reconciled, err = s.applyProviderModelCurationIntent(ctx, providerID, reconciled, *modelCuration)
		if err != nil {
			return ProviderSettings{}, err
		}
	}
	settings.Models.Curated = reconciled
	return settings, nil
}

func (s *service) applyProviderModelCurationIntent(
	ctx context.Context,
	providerID string,
	rows []compozyconfig.ProviderModelConfig,
	request ProviderModelCurationRequest,
) ([]compozyconfig.ProviderModelConfig, error) {
	request.ProviderID = strings.TrimSpace(providerID)
	request.ModelID = strings.TrimSpace(request.ModelID)
	if request.ModelID == "" {
		return nil, providerModelNotFoundError(request.ProviderID, request.ModelID)
	}
	model, err := s.findProviderModel(ctx, request.ProviderID, request.ModelID)
	if err != nil {
		return nil, err
	}
	rowIndex := make(map[string]int, len(rows))
	for index := range rows {
		if id := strings.TrimSpace(rows[index].ID); id != "" {
			rowIndex[id] = index
		}
	}
	index := ensureProviderCuratedRow(&rows, rowIndex, request.ModelID)
	curated, err := providerModelCurationConfig(rows[index], model, request)
	if err != nil {
		return nil, err
	}
	rows[index] = curated
	return rows, nil
}

func (s *service) currentProviderModels(
	ctx context.Context,
	providerID string,
) ([]modelcatalog.Model, error) {
	if s.modelCatalog == nil {
		return nil, validationError(
			errors.New("settings: model catalog is required to reconcile curated provider models"),
		)
	}
	models, err := s.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: providerID,
		View:       modelcatalog.CatalogViewAll,
	})
	if err != nil {
		return nil, fmt.Errorf("settings: list provider models before write: %w", err)
	}
	return models, nil
}

func reconcileProviderCuratedRows(
	raw []compozyconfig.ProviderModelConfig,
	configured []compozyconfig.ProviderModelConfig,
	currentModels []modelcatalog.Model,
	desired []compozyconfig.ProviderModelConfig,
) []compozyconfig.ProviderModelConfig {
	rows := cloneProviderModelConfigs(raw)
	rowIndex := make(map[string]int, len(rows))
	for index := range rows {
		if id := strings.TrimSpace(rows[index].ID); id != "" {
			rowIndex[id] = index
		}
	}
	projectedModels := providerModelConfigsFromCatalog(currentModels, configured)
	currentByID := make(map[string]compozyconfig.ProviderModelConfig, len(projectedModels))
	currentIDs := make([]string, 0, len(currentModels))
	for _, model := range projectedModels {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		currentByID[id] = model
	}
	for _, model := range currentModels {
		id := strings.TrimSpace(model.ModelID)
		if id == "" {
			continue
		}
		if model.Curated {
			currentIDs = append(currentIDs, id)
		}
	}
	currentIDs = uniqueProviderModelIDs(currentIDs)
	current := providerModelIDSet(currentIDs)
	desiredValues := make([]string, 0, len(desired))
	for _, model := range desired {
		desiredValues = append(desiredValues, model.ID)
	}
	desiredIDs := uniqueProviderModelIDs(desiredValues)
	desiredSet := providerModelIDSet(desiredIDs)
	for _, desiredModel := range desired {
		id := strings.TrimSpace(desiredModel.ID)
		if id == "" {
			continue
		}
		index, configured := rowIndex[id]
		rawModel := compozyconfig.ProviderModelConfig{ID: id}
		if configured {
			rawModel = rows[index]
		}
		currentModel := currentByID[id]
		currentModel.ID = id
		merged := mergeProviderModelConfigDeltas(rawModel, currentModel, desiredModel)
		if configured {
			rows[index] = merged
			continue
		}
		if reflect.DeepEqual(merged, rawModel) {
			continue
		}
		rows = append(rows, merged)
		rowIndex[id] = len(rows) - 1
	}
	if providerModelIDSetsEqual(current, desiredSet) {
		return rows
	}
	for _, id := range currentIDs {
		if _, keep := desiredSet[id]; keep {
			continue
		}
		index := ensureProviderCuratedRow(&rows, rowIndex, id)
		rows[index].Hidden = new(true)
	}
	for _, id := range desiredIDs {
		index := ensureProviderCuratedRow(&rows, rowIndex, id)
		if _, alreadyCurated := current[id]; !alreadyCurated {
			rows[index].Hidden = new(false)
			rows[index].Deprecated = new(false)
		}
	}
	return rows
}

func mergeProviderModelConfigDeltas(
	raw compozyconfig.ProviderModelConfig,
	current compozyconfig.ProviderModelConfig,
	desired compozyconfig.ProviderModelConfig,
) compozyconfig.ProviderModelConfig {
	next := cloneProviderModelConfigs([]compozyconfig.ProviderModelConfig{raw})[0]
	next.ID = strings.TrimSpace(desired.ID)
	mergeProviderModelValueDelta(&next.DisplayName, current.DisplayName, desired.DisplayName)
	mergeProviderModelPointerDelta(&next.ContextWindow, current.ContextWindow, desired.ContextWindow)
	mergeProviderModelPointerDelta(&next.MaxInputTokens, current.MaxInputTokens, desired.MaxInputTokens)
	mergeProviderModelPointerDelta(&next.MaxOutputTokens, current.MaxOutputTokens, desired.MaxOutputTokens)
	mergeProviderModelPointerDelta(&next.SupportsTools, current.SupportsTools, desired.SupportsTools)
	mergeProviderModelPointerDelta(&next.SupportsReasoning, current.SupportsReasoning, desired.SupportsReasoning)
	if !reflect.DeepEqual(desired.ReasoningEfforts, current.ReasoningEfforts) {
		next.ReasoningEfforts = cloneStringSlicePreserveNil(desired.ReasoningEfforts)
	}
	mergeProviderModelValueDelta(
		&next.DefaultReasoningEffort,
		current.DefaultReasoningEffort,
		desired.DefaultReasoningEffort,
	)
	mergeProviderModelPointerDelta(
		&next.CostInputPerMillion,
		current.CostInputPerMillion,
		desired.CostInputPerMillion,
	)
	mergeProviderModelPointerDelta(
		&next.CostOutputPerMillion,
		current.CostOutputPerMillion,
		desired.CostOutputPerMillion,
	)
	mergeProviderModelPointerDelta(
		&next.CostCacheReadPerMillion,
		current.CostCacheReadPerMillion,
		desired.CostCacheReadPerMillion,
	)
	mergeProviderModelPointerDelta(
		&next.CostCacheWritePerMillion,
		current.CostCacheWritePerMillion,
		desired.CostCacheWritePerMillion,
	)
	mergeProviderModelPointerDelta(
		&next.CostReasoningPerMillion,
		current.CostReasoningPerMillion,
		desired.CostReasoningPerMillion,
	)
	mergeProviderModelPointerDelta(&next.Deprecated, current.Deprecated, desired.Deprecated)
	mergeProviderModelPointerDelta(&next.Hidden, current.Hidden, desired.Hidden)
	mergeProviderModelPointerDelta(&next.Featured, current.Featured, desired.Featured)
	mergeProviderModelValueDelta(&next.ReleaseDate, current.ReleaseDate, desired.ReleaseDate)
	return next
}

func mergeProviderModelValueDelta[T comparable](target *T, current T, desired T) {
	if desired != current {
		*target = desired
	}
}

func mergeProviderModelPointerDelta[T comparable](target **T, current *T, desired *T) {
	if current == nil && desired == nil {
		return
	}
	if current != nil && desired != nil && *current == *desired {
		return
	}
	if desired == nil {
		*target = nil
		return
	}
	value := *desired
	*target = &value
}

func providerModelsWriteClearsConfig(models compozyconfig.ProviderModelsConfig) bool {
	return strings.TrimSpace(models.Default) == "" &&
		models.Curated == nil &&
		models.Discovery.Enabled == nil &&
		strings.TrimSpace(models.Discovery.Command) == "" &&
		strings.TrimSpace(models.Discovery.Endpoint) == "" &&
		strings.TrimSpace(models.Discovery.Timeout) == "" &&
		models.Reasoning.Apply == ""
}

func uniqueProviderModelIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	ids := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func providerModelIDSetsEqual(left, right map[string]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for id := range left {
		if _, exists := right[id]; !exists {
			return false
		}
	}
	return true
}

func providerModelIDSet(ids []string) map[string]struct{} {
	values := make(map[string]struct{}, len(ids))
	for _, value := range ids {
		if id := strings.TrimSpace(value); id != "" {
			values[id] = struct{}{}
		}
	}
	return values
}

func ensureProviderCuratedRow(
	rows *[]compozyconfig.ProviderModelConfig,
	index map[string]int,
	modelID string,
) int {
	if existing, ok := index[modelID]; ok {
		return existing
	}
	*rows = append(*rows, compozyconfig.ProviderModelConfig{ID: modelID})
	created := len(*rows) - 1
	index[modelID] = created
	return created
}
