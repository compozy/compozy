package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/modelcatalog"
)

// ProviderModelListQuery captures provider model catalog list filters.
type ProviderModelListQuery struct {
	ProviderID   string
	SourceID     string
	View         string
	Refresh      bool
	IncludeStale bool
}

// ProviderModelClient exposes provider-model catalog reads and mutations over the daemon API.
type ProviderModelClient interface {
	ListProviderModels(ctx context.Context, query ProviderModelListQuery) (ProviderModelListRecord, error)
	RefreshProviderModels(
		ctx context.Context,
		providerID string,
		request ProviderModelRefreshRequest,
	) (ProviderModelRefreshRecord, error)
	ProviderModelStatus(ctx context.Context, providerID string) (ProviderModelStatusRecord, error)
	CurateProviderModel(
		ctx context.Context,
		providerID string,
		request ProviderModelCurationRequest,
	) (ProviderModelCurationRecord, error)
}

// ProviderModelListRecord is the native provider model catalog list response.
type ProviderModelListRecord = contract.ProviderModelListResponse

// ProviderModelRecord is one native provider model catalog projection.
type ProviderModelRecord = contract.ProviderModelPayload

// ProviderModelRefreshRequest captures one provider model catalog refresh request.
type ProviderModelRefreshRequest = contract.ProviderModelRefreshRequest

// ProviderModelRefreshRecord is the native provider model catalog refresh response.
type ProviderModelRefreshRecord = contract.ProviderModelRefreshResponse

// ProviderModelCurationRequest captures one provider-model curation mutation.
type ProviderModelCurationRequest = contract.ProviderModelCurationRequest

// ProviderModelCurationRecord is the applied provider-model curation response.
type ProviderModelCurationRecord = contract.ProviderModelCurationResponse

// ProviderModelStatusRecord is the native provider model catalog source status response.
type ProviderModelStatusRecord = contract.ProviderModelStatusResponse

// ProviderModelSourceStatusRecord is one provider-scoped source status row.
type ProviderModelSourceStatusRecord = contract.ModelCatalogSourceStatusPayload

func (c *daemonClient) ListProviderModels(
	ctx context.Context,
	query ProviderModelListQuery,
) (ProviderModelListRecord, error) {
	path := providerModelsPath(query.ProviderID, "")
	var response ProviderModelListRecord
	values := providerModelListValues(ProviderModelListQuery{
		SourceID:     query.SourceID,
		View:         query.View,
		Refresh:      query.Refresh,
		IncludeStale: query.IncludeStale,
	})
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return ProviderModelListRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) CurateProviderModel(
	ctx context.Context,
	providerID string,
	request ProviderModelCurationRequest,
) (ProviderModelCurationRecord, error) {
	trimmedProvider := strings.TrimSpace(providerID)
	if trimmedProvider == "" {
		return ProviderModelCurationRecord{}, fmt.Errorf("provider model provider_id is required")
	}
	path := providerModelsPath(trimmedProvider, "curate")
	var response ProviderModelCurationRecord
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ProviderModelCurationRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) RefreshProviderModels(
	ctx context.Context,
	providerID string,
	request ProviderModelRefreshRequest,
) (ProviderModelRefreshRecord, error) {
	trimmedProvider := strings.TrimSpace(providerID)
	path := providerModelsPath(trimmedProvider, "refresh")
	var response ProviderModelRefreshRecord
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ProviderModelRefreshRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ProviderModelStatus(
	ctx context.Context,
	providerID string,
) (ProviderModelStatusRecord, error) {
	trimmedProvider := strings.TrimSpace(providerID)
	path := providerModelsPath(trimmedProvider, "status")
	var response ProviderModelStatusRecord
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return ProviderModelStatusRecord{}, err
	}
	return response, nil
}

func providerModelListValues(query ProviderModelListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.ProviderID); trimmed != "" {
		values.Set(providerModelsProviderIDKey, trimmed)
	}
	if trimmed := strings.TrimSpace(query.SourceID); trimmed != "" {
		values.Set("source_id", trimmed)
	}
	if trimmed := strings.TrimSpace(query.View); trimmed != "" {
		values.Set("view", trimmed)
	}
	if query.Refresh {
		values.Set("refresh", "true")
	}
	if query.IncludeStale {
		values.Set("include_stale", "true")
	}
	return values
}

func providerModelsPath(providerID string, action string) string {
	trimmedProvider := strings.TrimSpace(providerID)
	trimmedAction := strings.Trim(strings.TrimSpace(action), "/")
	if trimmedProvider == "" {
		if trimmedAction == "" {
			return "/api/model-catalog/models"
		}
		if trimmedAction == lifecycleStatusKey {
			return "/api/model-catalog/sources/status"
		}
		return "/api/model-catalog/models/" + url.PathEscape(trimmedAction)
	}
	path := "/api/model-catalog/providers/" + url.PathEscape(trimmedProvider) + "/models"
	if trimmedAction != "" {
		path += "/" + url.PathEscape(trimmedAction)
	}
	return path
}

type providerModelRefreshAPIError struct {
	statusCode int
	payload    ProviderModelRefreshRecord
}

func (e *providerModelRefreshAPIError) Error() string {
	details := []string{"Could not refresh the model list."}
	for _, source := range e.payload.Sources {
		if source.RefreshState != string(modelcatalog.RefreshStateFailed) {
			continue
		}
		if source.FailureReason == "" {
			source.FailureReason = "refresh_failed"
		}
		source = providerModelSourceStatusHumanSource(source)
		details = append(details, source.ProviderID+" ("+source.FailureReason+"): "+source.LastError)
	}
	return strings.Join(details, " ")
}

func (e *providerModelRefreshAPIError) cliExitCode() int {
	return apiStatusExitCode(e.statusCode)
}

func parseProviderModelRefreshAPIError(statusCode int, _ string, body []byte) (bool, error) {
	var payload ProviderModelRefreshRecord
	if json.Unmarshal(body, &payload) != nil || len(payload.Sources) == 0 || payload.Error == "" {
		return false, nil
	}
	for index, source := range payload.Sources {
		if source.SourceID == "" || source.ProviderID == "" || source.SourceKind == "" || source.RefreshState == "" {
			return false, nil
		}
		payload.Sources[index].LastError = redactToolDiagnostic(source.LastError)
	}
	payload.Error = redactToolDiagnostic(payload.Error)
	return true, &providerModelRefreshAPIError{statusCode: statusCode, payload: payload}
}
