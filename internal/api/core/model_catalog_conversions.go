package core

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
)

const (
	openAIModelObjectValue     = "model"
	openAIModelListObjectValue = "list"
)

func ProviderModelListPayloadFromModels(models []modelcatalog.Model) contract.ProviderModelListResponse {
	payload := contract.ProviderModelListResponse{
		Models: make([]contract.ProviderModelPayload, 0, len(models)),
	}
	for _, model := range models {
		payload.Models = append(payload.Models, ProviderModelPayloadFromModel(model))
	}
	return payload
}

func ProviderModelPayloadFromModel(model modelcatalog.Model) contract.ProviderModelPayload {
	payload := contract.ProviderModelPayload{
		ProviderID:             model.ProviderID,
		ModelID:                model.ModelID,
		DisplayName:            model.DisplayName,
		Sources:                SourceRefPayloadsFromRefs(model.Sources),
		Available:              model.Available,
		AvailabilityState:      string(model.AvailabilityState),
		Stale:                  model.Stale,
		RefreshedAt:            modelCatalogTimeString(model.RefreshedAt),
		ContextWindow:          model.ContextWindow,
		MaxInputTokens:         model.MaxInputTokens,
		MaxOutputTokens:        model.MaxOutputTokens,
		SupportsTools:          model.SupportsTools,
		SupportsReasoning:      model.SupportsReasoning,
		ReasoningEfforts:       append([]contract.ReasoningEffort(nil), model.ReasoningEfforts...),
		DefaultReasoningEffort: model.DefaultReasoningEffort,
		ConfigOptions:          providerModelConfigOptions(model.ConfigOptions),
		Configurations:         providerModelConfigurations(model.TransportBindings),
		Curated:                model.Curated,
		Deprecated:             model.Deprecated,
		Hidden:                 model.Hidden,
		Featured:               model.Featured,
		ReleaseDate:            optionalModelCatalogString(model.ReleaseDate),
		ReasoningSource:        model.ReasoningSource,
		LastError:              modelcatalog.RedactString(model.LastError),
	}
	if modelCatalogHasCost(model) {
		payload.Cost = &contract.ModelCatalogCostPayload{
			InputPerMillion:      model.CostInputPerMillion,
			OutputPerMillion:     model.CostOutputPerMillion,
			CacheReadPerMillion:  model.CostCacheReadPerMillion,
			CacheWritePerMillion: model.CostCacheWritePerMillion,
			ReasoningPerMillion:  model.CostReasoningPerMillion,
		}
	}
	return payload
}

func providerModelConfigOptions(
	options []modelcatalog.ModelOptionDescriptor,
) []contract.SessionConfigOptionPayload {
	if len(options) == 0 {
		return nil
	}
	payloads := make([]contract.SessionConfigOptionPayload, 0, len(options))
	for _, option := range options {
		payload := contract.SessionConfigOptionPayload{
			ID:             option.ID,
			Label:          option.Label,
			Description:    option.Description,
			Category:       option.Category,
			Kind:           string(option.Kind),
			CurrentValueID: option.CurrentValueID,
			Values:         providerModelConfigOptionValues(option.Values),
		}
		if option.CurrentBool != nil {
			payload.CurrentBool = new(*option.CurrentBool)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func providerModelConfigOptionValues(
	values []modelcatalog.ModelOptionValue,
) []contract.SessionConfigOptionValuePayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]contract.SessionConfigOptionValuePayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SessionConfigOptionValuePayload{
			Value:       value.ValueID,
			Label:       value.Label,
			Description: value.Description,
			GroupID:     value.GroupID,
			GroupLabel:  value.GroupLabel,
		})
	}
	return payloads
}

func providerModelConfigurations(
	bindings []modelcatalog.ModelTransportBinding,
) []contract.ProviderModelConfigurationPayload {
	if len(bindings) == 0 {
		return nil
	}
	configurations := make([]contract.ProviderModelConfigurationPayload, 0, len(bindings))
	for _, binding := range bindings {
		configuration := contract.ProviderModelConfigurationPayload{}
		if binding.ReasoningEffort != nil {
			value := contract.ReasoningEffort(*binding.ReasoningEffort)
			configuration.ReasoningEffort = new(value)
		}
		if binding.Fast != nil {
			configuration.Fast = new(*binding.Fast)
		}
		if binding.Thinking != nil {
			configuration.Thinking = new(*binding.Thinking)
		}
		configurations = append(configurations, configuration)
	}
	return configurations
}

func SourceRefPayloadsFromRefs(refs []modelcatalog.SourceRef) []contract.ModelCatalogSourceRefPayload {
	payloads := make([]contract.ModelCatalogSourceRefPayload, 0, len(refs))
	for _, ref := range refs {
		payloads = append(payloads, contract.ModelCatalogSourceRefPayload{
			SourceID:    ref.SourceID,
			SourceKind:  string(ref.SourceKind),
			Priority:    ref.Priority,
			RefreshedAt: modelCatalogTimeString(ref.RefreshedAt),
			Stale:       ref.Stale,
			LastError:   modelcatalog.RedactString(ref.LastError),
		})
	}
	return payloads
}

func SourceStatusPayloadsFromStatuses(
	statuses []modelcatalog.SourceStatus,
) []contract.ModelCatalogSourceStatusPayload {
	payloads := make([]contract.ModelCatalogSourceStatusPayload, 0, len(statuses))
	for _, status := range statuses {
		payloads = append(payloads, contract.ModelCatalogSourceStatusPayload{
			SourceID:     status.SourceID,
			SourceKind:   string(status.SourceKind),
			ProviderID:   status.ProviderID,
			Priority:     status.Priority,
			LastRefresh:  modelCatalogTimeString(status.LastRefresh),
			NextRefresh:  modelCatalogTimeString(status.NextRefresh),
			LastSuccess:  modelCatalogTimeString(status.LastSuccess),
			LastError:    modelcatalog.RedactString(status.LastError),
			RefreshState: string(status.RefreshState),
			RowCount:     status.RowCount,
			Stale:        status.Stale,
		})
	}
	return payloads
}

func OpenAIModelListPayloadFromModels(models []modelcatalog.Model) contract.OpenAIModelListResponse {
	payload := contract.OpenAIModelListResponse{
		Object: openAIModelListObjectValue,
		Data:   make([]contract.OpenAIModelPayload, 0, len(models)),
	}
	for _, model := range models {
		payload.Data = append(payload.Data, OpenAIModelPayloadFromModel(model))
	}
	return payload
}

func OpenAIModelPayloadFromModel(model modelcatalog.Model) contract.OpenAIModelPayload {
	return contract.OpenAIModelPayload{
		ID:      model.ModelID,
		Object:  openAIModelObjectValue,
		Created: 0,
		OwnedBy: model.ProviderID,
		Compozy: contract.OpenAIModelCompozyPayload{
			ProviderID:             model.ProviderID,
			ModelID:                model.ModelID,
			DisplayName:            model.DisplayName,
			Sources:                sourceIDsFromRefs(model.Sources),
			Available:              model.Available,
			AvailabilityState:      string(model.AvailabilityState),
			Stale:                  model.Stale,
			RefreshedAt:            modelCatalogTimeString(model.RefreshedAt),
			ContextWindow:          model.ContextWindow,
			MaxInputTokens:         model.MaxInputTokens,
			MaxOutputTokens:        model.MaxOutputTokens,
			SupportsTools:          model.SupportsTools,
			SupportsReasoning:      model.SupportsReasoning,
			ReasoningEfforts:       append([]contract.ReasoningEffort(nil), model.ReasoningEfforts...),
			DefaultReasoningEffort: model.DefaultReasoningEffort,
			Cost:                   costPayloadFromModel(model),
			LastError:              modelcatalog.RedactString(model.LastError),
		},
	}
}

func costPayloadFromModel(model modelcatalog.Model) *contract.ModelCatalogCostPayload {
	if !modelCatalogHasCost(model) {
		return nil
	}
	return &contract.ModelCatalogCostPayload{
		InputPerMillion:      model.CostInputPerMillion,
		OutputPerMillion:     model.CostOutputPerMillion,
		CacheReadPerMillion:  model.CostCacheReadPerMillion,
		CacheWritePerMillion: model.CostCacheWritePerMillion,
		ReasoningPerMillion:  model.CostReasoningPerMillion,
	}
}

func modelCatalogHasCost(model modelcatalog.Model) bool {
	return model.CostInputPerMillion != nil ||
		model.CostOutputPerMillion != nil ||
		model.CostCacheReadPerMillion != nil ||
		model.CostCacheWritePerMillion != nil ||
		model.CostReasoningPerMillion != nil
}

func sourceIDsFromRefs(refs []modelcatalog.SourceRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.SourceID)
	}
	return ids
}

func optionalModelCatalogString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func modelCatalogTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
