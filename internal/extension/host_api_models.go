package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apicontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/modelcatalogprojection"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

type hostAPIModelCatalogService interface {
	ListModels(ctx context.Context, opts modelcatalog.ListOptions) ([]modelcatalog.Model, error)
	Refresh(ctx context.Context, opts modelcatalog.RefreshOptions) ([]modelcatalog.SourceStatus, error)
	ListSourceStatus(ctx context.Context, opts modelcatalog.StatusOptions) ([]modelcatalog.SourceStatus, error)
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
		ProviderID:       providerID,
		SourceID:         sourceID,
		Refresh:          params.Refresh,
		IncludeStale:     params.IncludeStale,
		ExecutionContext: hostAPIModelCatalogExecutionContext(ctx),
		Now:              h.hostAPINow(),
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
		ProviderID:       providerID,
		SourceID:         sourceID,
		Force:            params.Force,
		RequestID:        strings.TrimSpace(params.RequestID),
		ExecutionContext: hostAPIModelCatalogExecutionContext(ctx),
		Now:              h.hostAPINow(),
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
	statuses, err := service.ListSourceStatus(ctx, modelcatalog.StatusOptions{
		ProviderID:       providerID,
		ExecutionContext: hostAPIModelCatalogExecutionContext(ctx),
	})
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

func hostAPIModelCatalogExecutionContext(ctx context.Context) modelcatalog.CatalogExecutionContext {
	key, _ := hostAPIInstanceKeyFromContext(ctx)
	profileID := strings.TrimSpace(key.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	workspaceID := strings.TrimSpace(key.WorkspaceID)
	if resourceSession, ok := hostAPIResourceSessionFromContext(ctx); ok {
		switch resourceSession.Actor.MaxScope.Kind {
		case resources.ResourceScopeKindProfile:
			if scopedProfileID := strings.TrimSpace(resourceSession.Actor.MaxScope.ID); scopedProfileID != "" {
				profileID = scopedProfileID
			}
		case resources.ResourceScopeKindWorkspace:
			if scopedWorkspaceID := strings.TrimSpace(resourceSession.Actor.MaxScope.ID); scopedWorkspaceID != "" {
				workspaceID = scopedWorkspaceID
			}
		}
	}
	if workspaceID == "" {
		return modelcatalog.CatalogExecutionContext{
			Scope:     modelcatalog.ExecutionScopeProfile,
			ProfileID: profileID,
		}
	}
	return modelcatalog.CatalogExecutionContext{
		Scope:       modelcatalog.ExecutionScopeWorkspace,
		ProfileID:   profileID,
		WorkspaceID: workspaceID,
	}
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
	return modelcatalogprojection.ProviderModelList(models)
}

func hostAPIProviderModelPayloadFromModel(model modelcatalog.Model) apicontract.ProviderModelPayload {
	return modelcatalogprojection.ProviderModel(model)
}

func hostAPISourceStatusPayloadsFromStatuses(
	statuses []modelcatalog.SourceStatus,
) []apicontract.ModelCatalogSourceStatusPayload {
	return modelcatalogprojection.SourceStatuses(statuses)
}

func hostAPICostPayloadFromModel(model modelcatalog.Model) *apicontract.ModelCatalogCostPayload {
	return modelcatalogprojection.Cost(model)
}
