package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/modelcatalogprojection"
	"github.com/compozy/compozy/internal/modelcatalog"
)

const (
	openAIModelObjectValue     = "model"
	openAIModelListObjectValue = "list"
)

func ProviderModelListPayloadFromModels(models []modelcatalog.Model) contract.ProviderModelListResponse {
	return modelcatalogprojection.ProviderModelList(models)
}

func ProviderModelPayloadFromModel(model modelcatalog.Model) contract.ProviderModelPayload {
	return modelcatalogprojection.ProviderModel(model)
}

func SourceRefPayloadsFromRefs(refs []modelcatalog.SourceRef) []contract.ModelCatalogSourceRefPayload {
	return modelcatalogprojection.SourceRefs(refs)
}

func SourceStatusPayloadsFromStatuses(
	statuses []modelcatalog.SourceStatus,
) []contract.ModelCatalogSourceStatusPayload {
	return modelcatalogprojection.SourceStatuses(statuses)
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
			RefreshedAt:            modelcatalogprojection.TimeString(model.RefreshedAt),
			ContextWindow:          model.ContextWindow,
			MaxInputTokens:         model.MaxInputTokens,
			MaxOutputTokens:        model.MaxOutputTokens,
			SupportsTools:          model.SupportsTools,
			SupportsReasoning:      model.SupportsReasoning,
			ReasoningEfforts:       append([]contract.ReasoningEffort(nil), model.ReasoningEfforts...),
			DefaultReasoningEffort: model.DefaultReasoningEffort,
			Cost:                   modelcatalogprojection.Cost(model),
			LastError:              modelcatalog.RedactString(model.LastError),
		},
	}
}

func sourceIDsFromRefs(refs []modelcatalog.SourceRef) []string {
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.SourceID)
	}
	return ids
}
