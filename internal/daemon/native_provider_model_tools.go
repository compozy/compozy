package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/modelcatalog"
	settingspkg "github.com/compozy/agh/internal/settings"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type providerModelsListInput struct {
	ProviderID   string `json:"provider_id,omitempty"`
	SourceID     string `json:"source_id,omitempty"`
	View         string `json:"view,omitempty"`
	IncludeStale bool   `json:"include_stale,omitempty"`
}

type providerModelsRefreshInput struct {
	ProviderID string `json:"provider_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	Force      bool   `json:"force,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

type providerModelsStatusInput struct {
	ProviderID string `json:"provider_id,omitempty"`
}

type providerModelsCurateInput struct {
	ProviderID    string                        `json:"provider_id"`
	ModelID       string                        `json:"model_id"`
	Hidden        *bool                         `json:"hidden,omitempty"`
	Featured      *bool                         `json:"featured,omitempty"`
	Deprecated    *bool                         `json:"deprecated,omitempty"`
	DefaultEffort *modelcatalog.ReasoningEffort `json:"default_effort,omitempty"`
}

func (n *daemonNativeTools) providerModelToolBindings(
	readAvailability toolspkg.NativeAvailabilityFunc,
	mutationAvailability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDProviderModelsList: {
			call:         n.providerModelsList,
			availability: readAvailability,
		},
		toolspkg.ToolIDProviderModelsRefresh: {
			call:         n.providerModelsRefresh,
			availability: readAvailability,
		},
		toolspkg.ToolIDProviderModelsStatus: {
			call:         n.providerModelsStatus,
			availability: readAvailability,
		},
		toolspkg.ToolIDProviderModelsCurate: {
			call:         n.providerModelsCurate,
			availability: mutationAvailability,
		},
	}
}

func (n *daemonNativeTools) providerModelReadAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool { return n.deps.ModelCatalog != nil })
}

func (n *daemonNativeTools) providerModelMutationAvailability() toolspkg.NativeAvailabilityFunc {
	return n.dependencyAvailability(func() bool {
		return n.deps.ModelCatalog != nil && n.settingsService() != nil
	})
}

func (n *daemonNativeTools) providerModelsList(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input providerModelsListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	providerID, err := nativeProviderModelProviderID(req.ToolID, input.ProviderID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sourceID, err := nativeProviderModelSourceID(req.ToolID, input.SourceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	view, err := nativeProviderModelView(req.ToolID, input.View)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	models, err := n.deps.ModelCatalog.ListModels(ctx, modelcatalog.ListOptions{
		ProviderID:   providerID,
		SourceID:     sourceID,
		View:         view,
		IncludeStale: input.IncludeStale,
		Now:          time.Now().UTC(),
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeProviderModelToolError(req.ToolID, err)
	}
	payload := core.ProviderModelListPayloadFromModels(models)
	return structuredResult(payload, fmt.Sprintf("%d provider models", len(payload.Models)))
}

func (n *daemonNativeTools) providerModelsCurate(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input providerModelsCurateInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	providerID, err := nativeProviderModelProviderID(req.ToolID, input.ProviderID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	service := n.settingsService()
	if service == nil {
		return toolspkg.ToolResult{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			req.ToolID,
			"provider model curation settings service is unavailable",
			toolspkg.ErrToolUnavailable,
			toolspkg.ReasonBackendUnhealthy,
		)
	}
	result, err := service.ApplyProviderModelCuration(
		settingspkg.WithMutationSource(ctx, req.ToolID.String()),
		settingspkg.ProviderModelCurationRequest{
			ProviderID:             providerID,
			ModelID:                input.ModelID,
			Hidden:                 input.Hidden,
			Featured:               input.Featured,
			Deprecated:             input.Deprecated,
			DefaultReasoningEffort: input.DefaultEffort,
		},
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeProviderModelToolError(req.ToolID, err)
	}
	payload := contract.ProviderModelCurationResponse{
		Model: core.ProviderModelPayloadFromModel(result.Model),
		Apply: core.SettingsApplyResponseFromResult(result.Apply),
	}
	return structuredResult(payload, fmt.Sprintf("curated provider model %s/%s", providerID, result.Model.ModelID))
}

func (n *daemonNativeTools) settingsService() core.SettingsService {
	if n == nil || n.deps == nil || n.deps.Settings == nil {
		return nil
	}
	return n.deps.Settings()
}

func (n *daemonNativeTools) providerModelsRefresh(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input providerModelsRefreshInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	providerID, err := nativeProviderModelProviderID(req.ToolID, input.ProviderID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	sourceID, err := nativeProviderModelSourceID(req.ToolID, input.SourceID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	statuses, err := n.deps.ModelCatalog.Refresh(ctx, modelcatalog.RefreshOptions{
		ProviderID: providerID,
		SourceID:   sourceID,
		Force:      input.Force,
		RequestID:  strings.TrimSpace(input.RequestID),
		Now:        time.Now().UTC(),
	})
	payload := contract.ProviderModelRefreshResponse{
		Sources: core.SourceStatusPayloadsFromStatuses(statuses),
	}
	if err != nil {
		if len(payload.Sources) == 0 {
			return toolspkg.ToolResult{}, nativeProviderModelToolError(req.ToolID, err)
		}
		payload.Error = modelcatalog.RedactString(err.Error())
	}
	return structuredResult(payload, fmt.Sprintf("%d provider model sources", len(payload.Sources)))
}

func (n *daemonNativeTools) providerModelsStatus(
	ctx context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input providerModelsStatusInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	providerID, err := nativeProviderModelProviderID(req.ToolID, input.ProviderID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	statuses, err := n.deps.ModelCatalog.ListSourceStatus(ctx, providerID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeProviderModelToolError(req.ToolID, err)
	}
	payload := contract.ProviderModelStatusResponse{
		Sources: core.SourceStatusPayloadsFromStatuses(statuses),
	}
	return structuredResult(payload, fmt.Sprintf("%d provider model sources", len(payload.Sources)))
}

func nativeProviderModelProviderID(id toolspkg.ToolID, providerID string) (string, error) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return "", nil
	}
	for idx, ch := range trimmed {
		valid := ch >= 'a' && ch <= 'z' ||
			ch >= '0' && ch <= '9' ||
			(idx > 0 && (ch == '-' || ch == '_'))
		if !valid {
			err := core.NewModelCatalogValidationError(
				fmt.Errorf("provider_id %q must match ^[a-z0-9][a-z0-9_-]*$", providerID),
			)
			return "", nativeProviderModelToolError(id, err)
		}
	}
	return trimmed, nil
}

func nativeProviderModelSourceID(id toolspkg.ToolID, sourceID string) (string, error) {
	trimmed := strings.TrimSpace(sourceID)
	if trimmed == "" {
		return "", nil
	}
	if err := modelcatalog.ValidateSourceID(trimmed); err != nil {
		return "", nativeProviderModelToolError(id, core.NewModelCatalogValidationError(err))
	}
	return trimmed, nil
}

func nativeProviderModelView(id toolspkg.ToolID, view string) (modelcatalog.CatalogView, error) {
	normalized := modelcatalog.CatalogView(strings.TrimSpace(view))
	if normalized == "" {
		return modelcatalog.CatalogViewCurated, nil
	}
	if normalized != modelcatalog.CatalogViewCurated && normalized != modelcatalog.CatalogViewAll {
		return "", nativeProviderModelToolError(
			id,
			core.NewModelCatalogValidationError(&modelcatalog.InvalidViewError{View: view}),
		)
	}
	return normalized, nil
}

func nativeProviderModelToolError(id toolspkg.ToolID, err error) error {
	if item, ok := diagnostics.ItemFromError(err); ok {
		switch item.Code {
		case diagnosticcontract.CodeModelNotFound:
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeModelNotFound,
				id,
				modelcatalog.RedactString(err.Error()),
				fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
				toolspkg.ReasonSchemaInvalid,
			)
		case diagnosticcontract.CodeReasoningEffortUnsupported:
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeReasoningEffortUnsupported,
				id,
				modelcatalog.RedactString(err.Error()),
				fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
				toolspkg.ReasonSchemaInvalid,
			)
		}
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, core.ErrModelCatalogValidation),
		errors.Is(err, modelcatalog.ErrSourceNotRegistered),
		errors.Is(err, settingspkg.ErrValidation):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			modelcatalog.RedactString(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	case errors.Is(err, core.ErrModelCatalogUnavailable),
		errors.Is(err, modelcatalog.ErrAllSourcesFailed):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			modelcatalog.RedactString(err.Error()),
			fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonBackendUnhealthy,
		)
	default:
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeBackendFailed,
			id,
			modelcatalog.RedactString(err.Error()),
			err,
			toolspkg.ReasonBackendUnhealthy,
		)
	}
}
