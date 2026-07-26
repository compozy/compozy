package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/modelcatalog"
)

type hostAPIModelCatalogService interface {
	ListModels(ctx context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	Refresh(ctx context.Context, opts modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	ListSourceStatus(ctx context.Context, providerID string) ([]modelcatalog.SourceStatus, error)
}

// WithHostAPIModelCatalogService injects daemon-owned model catalog projections.
func WithHostAPIModelCatalogService(service modelcatalog.Service) HostAPIOption {
	return func(handler *HostAPIHandler) {
		handler.modelCatalog = service
	}
}

func (h *HostAPIHandler) handleModelsList(
	ctx context.Context,
	raw json.RawMessage,
) (any, error) {
	var params extensioncontract.ModelsListParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	sourceID, err := validateHostAPIModelSourceID(params.SourceID)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	providerID, err := validateHostAPIModelProviderID(params.ProviderID)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	service, err := h.modelCatalogService()
	if err != nil {
		return nil, unavailableRPCError(err)
	}
	models, err := service.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:   providerID,
		SourceID:     sourceID,
		Refresh:      params.Refresh,
		IncludeStale: params.IncludeStale,
		Now:          h.hostAPINow(),
	})
	if err != nil {
		return nil, hostAPIModelCatalogRPCError(err)
	}
	return hostAPIProviderModelListPayloadFromModels(models), nil
}

func (h *HostAPIHandler) handleModelsRefresh(
	ctx context.Context,
	raw json.RawMessage,
) (any, error) {
	var params extensioncontract.ModelsRefreshParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	sourceID, err := validateHostAPIModelSourceID(params.SourceID)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	providerID, err := validateHostAPIModelProviderID(params.ProviderID)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	service, err := h.modelCatalogService()
	if err != nil {
		return nil, unavailableRPCError(err)
	}
	statuses, err := service.Refresh(ctx, modelcatalog.RefreshOptions{
		ProviderID: providerID,
		SourceID:   sourceID,
		Force:      params.Force,
		RequestID:  strings.TrimSpace(params.RequestID),
		Now:        h.hostAPINow(),
	})
	payload := apicontract.ProviderModelRefreshResponse{
		Sources: hostAPISourceStatusPayloadsFromStatuses(statuses),
	}
	if err != nil {
		if len(payload.Sources) > 0 {
			payload.Error = modelcatalog.RedactString(err.Error())
			return payload, nil
		}
		return nil, hostAPIModelCatalogRPCError(err)
	}
	return payload, nil
}

func (h *HostAPIHandler) handleModelsStatus(
	ctx context.Context,
	raw json.RawMessage,
) (any, error) {
	var params extensioncontract.ModelsStatusParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	providerID, err := validateHostAPIModelProviderID(params.ProviderID)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	service, err := h.modelCatalogService()
	if err != nil {
		return nil, unavailableRPCError(err)
	}
	statuses, err := service.ListSourceStatus(ctx, providerID)
	if err != nil {
		return nil, unavailableRPCError(err)
	}
	return apicontract.ProviderModelStatusResponse{
		Sources: hostAPISourceStatusPayloadsFromStatuses(statuses),
	}, nil
}

func (h *HostAPIHandler) modelCatalogService() (hostAPIModelCatalogService, error) {
	if h == nil || h.modelCatalog == nil {
		return nil, errors.New("extension: model catalog service is unavailable")
	}
	return h.modelCatalog, nil
}

func (h *HostAPIHandler) hostAPINow() time.Time {
	if h == nil || h.now == nil {
		return time.Now().UTC()
	}
	return h.now().UTC()
}

func validateHostAPIModelSourceID(sourceID string) (string, error) {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return "", nil
	}
	if err := modelcatalog.ValidateSourceID(trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func validateHostAPIModelProviderID(providerID string) (string, error) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return "", nil
	}
	for idx, ch := range trimmed {
		valid := ch >= 'a' && ch <= 'z' ||
			ch >= '0' && ch <= '9' ||
			(idx > 0 && (ch == '-' || ch == '_'))
		if !valid {
			return "", fmt.Errorf("provider_id %q must match ^[a-z0-9][a-z0-9_-]*$", providerID)
		}
	}
	return trimmed, nil
}

func hostAPIModelCatalogRPCError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, modelcatalog.ErrSourceNotRegistered) {
		return invalidParamsRPCError(err)
	}
	return unavailableRPCError(errors.New(modelcatalog.RedactString(err.Error())))
}

func hostAPIProviderModelListPayloadFromModels(models []modelcatalog.Model) apicontract.ProviderModelListResponse {
	payload := apicontract.ProviderModelListResponse{
		Models: make([]apicontract.ProviderModelPayload, 0, len(models)),
	}
	for _, model := range models {
		payload.Models = append(payload.Models, hostAPIProviderModelPayloadFromModel(model))
	}
	return payload
}

func hostAPIProviderModelPayloadFromModel(model modelcatalog.Model) apicontract.ProviderModelPayload {
	return apicontract.ProviderModelPayload{
		ProviderID:             model.ProviderID,
		ModelID:                model.ModelID,
		DisplayName:            model.DisplayName,
		Sources:                hostAPISourceRefPayloadsFromRefs(model.Sources),
		Available:              model.Available,
		AvailabilityState:      string(model.AvailabilityState),
		Stale:                  model.Stale,
		RefreshedAt:            hostAPIModelCatalogTimeString(model.RefreshedAt),
		ContextWindow:          model.ContextWindow,
		MaxInputTokens:         model.MaxInputTokens,
		MaxOutputTokens:        model.MaxOutputTokens,
		SupportsTools:          model.SupportsTools,
		SupportsReasoning:      model.SupportsReasoning,
		ReasoningEfforts:       append([]apicontract.ReasoningEffort(nil), model.ReasoningEfforts...),
		DefaultReasoningEffort: model.DefaultReasoningEffort,
		Cost:                   hostAPICostPayloadFromModel(model),
		Curated:                model.Curated,
		Deprecated:             model.Deprecated,
		Hidden:                 model.Hidden,
		Featured:               model.Featured,
		ReleaseDate:            hostAPIOptionalModelCatalogString(model.ReleaseDate),
		ReasoningSource:        model.ReasoningSource,
		LastError:              modelcatalog.RedactString(model.LastError),
	}
}

func hostAPISourceRefPayloadsFromRefs(refs []modelcatalog.SourceRef) []apicontract.ModelCatalogSourceRefPayload {
	payloads := make([]apicontract.ModelCatalogSourceRefPayload, 0, len(refs))
	for _, ref := range refs {
		payloads = append(payloads, apicontract.ModelCatalogSourceRefPayload{
			SourceID:    ref.SourceID,
			SourceKind:  string(ref.SourceKind),
			Priority:    ref.Priority,
			RefreshedAt: hostAPIModelCatalogTimeString(ref.RefreshedAt),
			Stale:       ref.Stale,
			LastError:   modelcatalog.RedactString(ref.LastError),
		})
	}
	return payloads
}

func hostAPISourceStatusPayloadsFromStatuses(
	statuses []modelcatalog.SourceStatus,
) []apicontract.ModelCatalogSourceStatusPayload {
	payloads := make([]apicontract.ModelCatalogSourceStatusPayload, 0, len(statuses))
	for _, status := range statuses {
		payloads = append(payloads, apicontract.ModelCatalogSourceStatusPayload{
			SourceID:     status.SourceID,
			SourceKind:   string(status.SourceKind),
			ProviderID:   status.ProviderID,
			Priority:     status.Priority,
			LastRefresh:  hostAPIModelCatalogTimeString(status.LastRefresh),
			NextRefresh:  hostAPIModelCatalogTimeString(status.NextRefresh),
			LastSuccess:  hostAPIModelCatalogTimeString(status.LastSuccess),
			LastError:    modelcatalog.RedactString(status.LastError),
			RefreshState: string(status.RefreshState),
			RowCount:     status.RowCount,
			Stale:        status.Stale,
		})
	}
	return payloads
}

func hostAPICostPayloadFromModel(model modelcatalog.Model) *apicontract.ModelCatalogCostPayload {
	if model.CostInputPerMillion == nil &&
		model.CostOutputPerMillion == nil &&
		model.CostCacheReadPerMillion == nil &&
		model.CostCacheWritePerMillion == nil &&
		model.CostReasoningPerMillion == nil {
		return nil
	}
	return &apicontract.ModelCatalogCostPayload{
		InputPerMillion:      model.CostInputPerMillion,
		OutputPerMillion:     model.CostOutputPerMillion,
		CacheReadPerMillion:  model.CostCacheReadPerMillion,
		CacheWritePerMillion: model.CostCacheWritePerMillion,
		ReasoningPerMillion:  model.CostReasoningPerMillion,
	}
}

func hostAPIOptionalModelCatalogString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func hostAPIModelCatalogTimeString(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
