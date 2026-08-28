package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/modelcatalog"
)

// ProviderModelCurationRequest identifies one global provider-model curation mutation.
type ProviderModelCurationRequest struct {
	ProviderID             string
	ModelID                string
	Hidden                 *bool
	Featured               *bool
	Deprecated             *bool
	DefaultReasoningEffort *modelcatalog.ReasoningEffort
}

// ProviderModelCurationResult reports the applied config generation and effective model row.
type ProviderModelCurationResult struct {
	Model modelcatalog.Model
	Apply ApplyResult
}

// ApplyProviderModelCuration serializes one model-only config mutation through the live apply lifecycle.
func (s *service) ApplyProviderModelCuration(
	ctx context.Context,
	req ProviderModelCurationRequest,
) (ProviderModelCurationResult, error) {
	if ctx == nil {
		return ProviderModelCurationResult{}, errors.New("settings: provider model curation context is required")
	}
	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	providerID := strings.TrimSpace(req.ProviderID)
	modelID := strings.TrimSpace(req.ModelID)
	if providerID == "" || modelID == "" {
		return ProviderModelCurationResult{}, providerModelNotFoundError(providerID, modelID)
	}
	if err := s.requireProviderModelCatalog(); err != nil {
		return ProviderModelCurationResult{}, err
	}
	if _, err := s.ensureActiveConfigState(ctx); err != nil {
		return ProviderModelCurationResult{}, err
	}
	currentConfig, _, err := s.loadConfig(ctx, ScopeUser, "", "")
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	if _, err := currentConfig.ResolveProvider(providerID); err != nil {
		return ProviderModelCurationResult{}, providerModelNotFoundError(providerID, modelID)
	}

	model, err := s.findProviderModel(ctx, providerID, modelID)
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	base := configuredProviderModelCurationBase(&currentConfig, providerID, modelID)
	curated, err := providerModelCurationConfig(base, model, req)
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	values := providerModelConfigMaps([]compozyconfig.ProviderModelConfig{curated})
	if len(values) != 1 {
		return ProviderModelCurationResult{}, errors.New("settings: provider model curation produced no config row")
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", compozyconfig.WriteScopeUser, "")
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	if _, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return editor.UpsertArrayTableItem(
				[]string{"providers", providerID, "models", "curated"},
				"id",
				modelID,
				values[0],
			)
		},
	); err != nil {
		return ProviderModelCurationResult{}, fmt.Errorf(
			"settings: curate provider model %q/%q: %w",
			providerID,
			modelID,
			err,
		)
	}

	mutation := mutationResultForProvider(target.Kind(), true)
	if err := s.emitSettingsChanged(ctx, mutation, "replace"); err != nil {
		return ProviderModelCurationResult{}, err
	}
	apply, err := s.recordProviderModelsMutationApply(ctx, mutation, providerID)
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	effective, err := s.findProviderModel(ctx, providerID, modelID)
	if err != nil {
		return ProviderModelCurationResult{}, err
	}
	return ProviderModelCurationResult{Model: effective, Apply: apply}, nil
}

func (s *service) requireProviderModelCatalog() error {
	if s.modelCatalog == nil {
		return fmt.Errorf(
			"settings: model catalog is required for provider curation: %w",
			modelcatalog.ErrAllSourcesFailed,
		)
	}
	return nil
}

func (s *service) findProviderModel(
	ctx context.Context,
	providerID string,
	modelID string,
) (modelcatalog.Model, error) {
	models, err := s.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID: providerID,
		View:       modelcatalog.CatalogViewAll,
	})
	if err != nil {
		return modelcatalog.Model{}, fmt.Errorf("settings: list provider models for curation: %w", err)
	}
	for _, model := range models {
		if model.ProviderID == providerID && model.ModelID == modelID {
			return model, nil
		}
	}
	return modelcatalog.Model{}, providerModelNotFoundError(providerID, modelID)
}

func providerModelCurationConfig(
	base compozyconfig.ProviderModelConfig,
	model modelcatalog.Model,
	req ProviderModelCurationRequest,
) (compozyconfig.ProviderModelConfig, error) {
	curated := cloneProviderModelConfigs([]compozyconfig.ProviderModelConfig{base})[0]
	curated.ID = model.ModelID
	if req.Hidden != nil {
		curated.Hidden = cloneBoolPtr(req.Hidden)
	}
	if req.Featured != nil {
		curated.Featured = cloneBoolPtr(req.Featured)
	}
	if req.Deprecated != nil {
		curated.Deprecated = cloneBoolPtr(req.Deprecated)
	}
	if req.DefaultReasoningEffort == nil {
		return curated, nil
	}
	effort := strings.TrimSpace(string(*req.DefaultReasoningEffort))
	canonical := modelcatalog.ReasoningEffort(effort)
	if !modelcatalog.IsValidEffort(effort) ||
		!slices.Contains(providerModelReasoningEfforts(model), canonical) {
		return compozyconfig.ProviderModelConfig{}, providerModelEffortUnsupportedError(model, effort)
	}
	curated.DefaultReasoningEffort = effort
	return curated, nil
}

func providerModelReasoningEfforts(model modelcatalog.Model) []modelcatalog.ReasoningEffort {
	efforts := append([]modelcatalog.ReasoningEffort(nil), model.ReasoningEfforts...)
	for _, binding := range model.TransportBindings {
		if binding.ReasoningEffort == nil || slices.Contains(efforts, *binding.ReasoningEffort) {
			continue
		}
		efforts = append(efforts, *binding.ReasoningEffort)
	}
	return efforts
}

func configuredProviderModelCurationBase(
	cfg *compozyconfig.Config,
	providerID string,
	modelID string,
) compozyconfig.ProviderModelConfig {
	base := compozyconfig.ProviderModelConfig{ID: modelID}
	if cfg == nil {
		return base
	}
	provider, ok := cfg.Providers[providerID]
	if !ok {
		return base
	}
	for _, configured := range provider.Models.Curated {
		if strings.TrimSpace(configured.ID) != modelID {
			continue
		}
		return cloneProviderModelConfigs([]compozyconfig.ProviderModelConfig{configured})[0]
	}
	return base
}

func providerModelNotFoundError(providerID string, modelID string) error {
	cause := fmt.Errorf(
		"provider model %q/%q was not found in the current catalog",
		strings.TrimSpace(providerID),
		strings.TrimSpace(modelID),
	)
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "provider.models.model_not_found",
		Code:          diagnosticcontract.CodeModelNotFound,
		Category:      diagnosticcontract.CategoryProvider,
		Title:         "Provider model not found",
		Message:       cause.Error(),
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"provider_id": strings.TrimSpace(providerID),
			"model_id":    strings.TrimSpace(modelID),
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}

func providerModelEffortUnsupportedError(model modelcatalog.Model, effort string) error {
	efforts := providerModelReasoningEfforts(model)
	choices := make([]string, 0, len(efforts))
	for _, choice := range efforts {
		choices = append(choices, string(choice))
	}
	slices.Sort(choices)
	cause := fmt.Errorf(
		"reasoning effort %q is unavailable for provider model %q/%q",
		strings.TrimSpace(effort),
		model.ProviderID,
		model.ModelID,
	)
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "provider.models.reasoning_effort_unsupported",
		Code:          diagnosticcontract.CodeReasoningEffortUnsupported,
		Category:      diagnosticcontract.CategoryProvider,
		Title:         "Reasoning effort is unsupported",
		Message:       cause.Error(),
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	},
		diagnostics.WithEvidence(map[string]any{
			"provider_id":   model.ProviderID,
			"model_id":      model.ModelID,
			"requested":     strings.TrimSpace(effort),
			"valid_choices": choices,
		}),
	)
	return diagnostics.NewStructuredError(item, cause)
}
