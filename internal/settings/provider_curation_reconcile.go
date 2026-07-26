package settings

import (
	"context"
	"errors"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
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
	cfg, _, err := s.loadConfig(ctx, ScopeGlobal, "")
	if err != nil {
		return ProviderSettings{}, err
	}
	rawProvider := cfg.Providers[providerID]
	if !settings.ModelsSet {
		settings.Models = cloneProviderModelsConfig(rawProvider.Models)
		return settings, nil
	}
	_, err = cfg.ResolveProvider(providerID)
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
		currentIDs, currentErr := s.currentCuratedProviderModelIDs(ctx, providerID)
		if currentErr != nil {
			return ProviderSettings{}, currentErr
		}
		reconciled = reconcileProviderCuratedRows(raw, currentIDs, settings.Models.Curated)
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
	rows []aghconfig.ProviderModelConfig,
	request ProviderModelCurationRequest,
) ([]aghconfig.ProviderModelConfig, error) {
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

func (s *service) currentCuratedProviderModelIDs(
	ctx context.Context,
	providerID string,
) ([]string, error) {
	if err := s.requireProviderModelCatalog(); err != nil {
		return nil, err
	}
	models, err := s.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: providerID,
		View:       modelcatalog.CatalogViewCurated,
	})
	if err != nil {
		return nil, fmt.Errorf("settings: list provider models before write: %w", err)
	}
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ModelID)
	}
	return ids, nil
}

func reconcileProviderCuratedRows(
	raw []aghconfig.ProviderModelConfig,
	currentIDs []string,
	desired []aghconfig.ProviderModelConfig,
) []aghconfig.ProviderModelConfig {
	rows := cloneProviderModelConfigs(raw)
	rowIndex := make(map[string]int, len(rows))
	for index := range rows {
		if id := strings.TrimSpace(rows[index].ID); id != "" {
			rowIndex[id] = index
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

func providerModelsWriteClearsConfig(models aghconfig.ProviderModelsConfig) bool {
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
	rows *[]aghconfig.ProviderModelConfig,
	index map[string]int,
	modelID string,
) int {
	if existing, ok := index[modelID]; ok {
		return existing
	}
	*rows = append(*rows, aghconfig.ProviderModelConfig{ID: modelID})
	created := len(*rows) - 1
	index[modelID] = created
	return created
}
