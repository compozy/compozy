package modelcatalogprojection

import (
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
)

// ProviderModelList converts catalog models into the shared public API contract.
func ProviderModelList(models []modelcatalog.Model) contract.ProviderModelListResponse {
	payload := contract.ProviderModelListResponse{
		Models: make([]contract.ProviderModelPayload, 0, len(models)),
	}
	for _, model := range models {
		payload.Models = append(payload.Models, ProviderModel(model))
	}
	return payload
}

// ProviderModel converts one catalog model without exposing transport aliases.
func ProviderModel(model modelcatalog.Model) contract.ProviderModelPayload {
	payload := contract.ProviderModelPayload{
		ProviderID:             model.ProviderID,
		ModelID:                model.ModelID,
		DisplayName:            model.DisplayName,
		Sources:                SourceRefs(model.Sources),
		Available:              model.Available,
		AvailabilityState:      string(model.AvailabilityState),
		Stale:                  model.Stale,
		RefreshedAt:            TimeString(model.RefreshedAt),
		ContextWindow:          model.ContextWindow,
		MaxInputTokens:         model.MaxInputTokens,
		MaxOutputTokens:        model.MaxOutputTokens,
		SupportsTools:          model.SupportsTools,
		SupportsReasoning:      model.SupportsReasoning,
		ReasoningEfforts:       append([]contract.ReasoningEffort(nil), model.ReasoningEfforts...),
		DefaultReasoningEffort: model.DefaultReasoningEffort,
		ConfigOptions:          configOptions(model.ConfigOptions),
		Configurations:         configurations(model.TransportBindings),
		Cost:                   Cost(model),
		Curated:                model.Curated,
		Deprecated:             model.Deprecated,
		Hidden:                 model.Hidden,
		Featured:               model.Featured,
		ReleaseDate:            OptionalString(model.ReleaseDate),
		ReasoningSource:        model.ReasoningSource,
		LastError:              modelcatalog.RedactString(model.LastError),
	}
	return payload
}

func configOptions(options []modelcatalog.ModelOptionDescriptor) []contract.SessionConfigOptionPayload {
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
			Values:         configOptionValues(option.Values),
		}
		if option.CurrentBool != nil {
			payload.CurrentBool = new(*option.CurrentBool)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func configOptionValues(values []modelcatalog.ModelOptionValue) []contract.SessionConfigOptionValuePayload {
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

func configurations(
	bindings []modelcatalog.ModelTransportBinding,
) []contract.ProviderModelConfigurationPayload {
	if len(bindings) == 0 {
		return nil
	}
	payloads := make([]contract.ProviderModelConfigurationPayload, 0, len(bindings))
	for _, binding := range bindings {
		payload := contract.ProviderModelConfigurationPayload{}
		if binding.ReasoningEffort != nil {
			effort := *binding.ReasoningEffort
			payload.ReasoningEffort = new(effort)
		}
		if binding.Fast != nil {
			payload.Fast = new(*binding.Fast)
		}
		if binding.Thinking != nil {
			payload.Thinking = new(*binding.Thinking)
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

// SourceRefs converts model source provenance to the public contract.
func SourceRefs(refs []modelcatalog.SourceRef) []contract.ModelCatalogSourceRefPayload {
	payloads := make([]contract.ModelCatalogSourceRefPayload, 0, len(refs))
	for _, ref := range refs {
		payloads = append(payloads, contract.ModelCatalogSourceRefPayload{
			SourceID:    ref.SourceID,
			SourceKind:  string(ref.SourceKind),
			Priority:    ref.Priority,
			RefreshedAt: TimeString(ref.RefreshedAt),
			Stale:       ref.Stale,
			LastError:   modelcatalog.RedactString(ref.LastError),
		})
	}
	return payloads
}

// SourceStatuses converts catalog lifecycle state to the public contract.
func SourceStatuses(statuses []modelcatalog.SourceStatus) []contract.ModelCatalogSourceStatusPayload {
	payloads := make([]contract.ModelCatalogSourceStatusPayload, 0, len(statuses))
	for _, status := range statuses {
		payloads = append(payloads, contract.ModelCatalogSourceStatusPayload{
			SourceID:     status.SourceID,
			SourceKind:   string(status.SourceKind),
			ProviderID:   status.ProviderID,
			Priority:     status.Priority,
			LastRefresh:  TimeString(status.LastRefresh),
			NextRefresh:  TimeString(status.NextRefresh),
			LastSuccess:  TimeString(status.LastSuccess),
			LastError:    modelcatalog.RedactString(status.LastError),
			RefreshState: string(status.RefreshState),
			RowCount:     status.RowCount,
			Stale:        status.Stale,
		})
	}
	return payloads
}

// Cost projects the complete five-rate catalog cost when any rate is known.
func Cost(model modelcatalog.Model) *contract.ModelCatalogCostPayload {
	if model.CostInputPerMillion == nil && model.CostOutputPerMillion == nil &&
		model.CostCacheReadPerMillion == nil && model.CostCacheWritePerMillion == nil &&
		model.CostReasoningPerMillion == nil {
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

// OptionalString projects a nullable catalog string to its wire representation.
func OptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// TimeString projects catalog timestamps in the canonical wire format.
func TimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
