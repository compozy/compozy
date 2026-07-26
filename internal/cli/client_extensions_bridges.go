package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"

	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) ListExtensions(ctx context.Context) ([]ExtensionRecord, error) {
	var response struct {
		Extensions []ExtensionRecord `json:"extensions"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/extensions", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Extensions, nil
}

func (c *unixSocketClient) InstallExtension(
	ctx context.Context,
	request InstallExtensionRequest,
) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/extensions", nil, request, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) UpdateExtension(
	ctx context.Context,
	name string,
	request UpdateExtensionRequest,
) (ExtensionUpdateRecord, error) {
	var response struct {
		Update ExtensionUpdateRecord `json:"update"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodPut,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		request,
		&response,
	); err != nil {
		return ExtensionUpdateRecord{}, err
	}
	return response.Update, nil
}

func (c *unixSocketClient) RemoveExtension(ctx context.Context, name string) (ManagedExtensionRemoveRecord, error) {
	var response struct {
		Extension ManagedExtensionRemoveRecord `json:"extension"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		nil,
		&response,
	); err != nil {
		return ManagedExtensionRemoveRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) EnableExtension(ctx context.Context, name string) (ExtensionRecord, error) {
	return c.extensionAction(ctx, strings.TrimSpace(name), "enable")
}

func (c *unixSocketClient) DisableExtension(ctx context.Context, name string) (ExtensionRecord, error) {
	return c.extensionAction(ctx, strings.TrimSpace(name), "disable")
}

func (c *unixSocketClient) ExtensionStatus(ctx context.Context, name string) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		nil,
		&response,
	); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) ExtensionProvenance(ctx context.Context, name string) (ExtensionProvenanceRecord, error) {
	var response struct {
		Provenance ExtensionProvenanceRecord `json:"provenance"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name))+"/provenance",
		nil,
		nil,
		&response,
	); err != nil {
		return ExtensionProvenanceRecord{}, err
	}
	return response.Provenance, nil
}

func (c *unixSocketClient) ListBundleCatalog(ctx context.Context) ([]BundleCatalogRecord, error) {
	var response struct {
		Bundles []BundleCatalogRecord `json:"bundles"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/bundles/catalog", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Bundles, nil
}

func (c *unixSocketClient) PreviewBundleActivation(
	ctx context.Context,
	request ActivateBundleRequest,
) (BundleActivationRecord, error) {
	var response struct {
		Activation BundleActivationRecord `json:"activation"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/bundles/preview", nil, request, &response); err != nil {
		return BundleActivationRecord{}, err
	}
	return response.Activation, nil
}

func (c *unixSocketClient) ActivateBundle(
	ctx context.Context,
	request ActivateBundleRequest,
) (BundleActivationRecord, error) {
	var response struct {
		Activation BundleActivationRecord `json:"activation"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/bundles/activations", nil, request, &response); err != nil {
		return BundleActivationRecord{}, err
	}
	return response.Activation, nil
}

func (c *unixSocketClient) ListBundleActivations(ctx context.Context) ([]BundleActivationRecord, error) {
	var response struct {
		Activations []BundleActivationRecord `json:"activations"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/bundles/activations", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Activations, nil
}

func (c *unixSocketClient) GetBundleActivation(ctx context.Context, id string) (BundleActivationRecord, error) {
	var response struct {
		Activation BundleActivationRecord `json:"activation"`
	}
	path := "/api/bundles/activations/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return BundleActivationRecord{}, err
	}
	return response.Activation, nil
}

func (c *unixSocketClient) UpdateBundleActivation(
	ctx context.Context,
	id string,
	request UpdateBundleActivationRequest,
) (BundleActivationRecord, error) {
	var response struct {
		Activation BundleActivationRecord `json:"activation"`
	}
	path := "/api/bundles/activations/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return BundleActivationRecord{}, err
	}
	return response.Activation, nil
}

func (c *unixSocketClient) DeactivateBundle(ctx context.Context, id string) error {
	path := "/api/bundles/activations/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) BundleNetworkSettings(ctx context.Context) (BundleNetworkSettingsRecord, error) {
	var response struct {
		Network BundleNetworkSettingsRecord `json:"network"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/bundles/network/settings", nil, nil, &response); err != nil {
		return BundleNetworkSettingsRecord{}, err
	}
	return response.Network, nil
}

func (c *unixSocketClient) CreateBridge(ctx context.Context, request CreateBridgeRequest) (BridgeRecord, error) {
	var response struct {
		Bridge BridgeRecord `json:"bridge"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/api/bridges", nil, request, &response); err != nil {
		return BridgeRecord{}, err
	}
	return response.Bridge, nil
}

func (c *unixSocketClient) GetBridge(ctx context.Context, id string) (BridgeRecord, error) {
	var response struct {
		Bridge BridgeRecord `json:"bridge"`
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return BridgeRecord{}, err
	}
	return response.Bridge, nil
}

func (c *unixSocketClient) UpdateBridge(
	ctx context.Context,
	id string,
	request UpdateBridgeRequest,
) (BridgeRecord, error) {
	var response struct {
		Bridge BridgeRecord `json:"bridge"`
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return BridgeRecord{}, err
	}
	return response.Bridge, nil
}

func (c *unixSocketClient) EnableBridge(ctx context.Context, id string) (BridgeRecord, error) {
	return c.bridgeAction(ctx, strings.TrimSpace(id), "enable")
}

func (c *unixSocketClient) DisableBridge(ctx context.Context, id string) (BridgeRecord, error) {
	return c.bridgeAction(ctx, strings.TrimSpace(id), "disable")
}

func (c *unixSocketClient) RestartBridge(ctx context.Context, id string) (BridgeRecord, error) {
	return c.bridgeAction(ctx, strings.TrimSpace(id), "restart")
}

func (c *unixSocketClient) BridgeRoutes(ctx context.Context, id string) ([]BridgeRouteRecord, error) {
	var response struct {
		Routes []BridgeRouteRecord `json:"routes"`
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/routes"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Routes, nil
}

func (c *unixSocketClient) BridgeTargets(
	ctx context.Context,
	id string,
	query string,
	limit int,
) (BridgeTargetsRecord, error) {
	values := url.Values{}
	if strings.TrimSpace(query) != "" {
		values.Set("q", strings.TrimSpace(query))
	}
	if limit > 0 {
		values.Set("limit", fmt.Sprintf("%d", limit))
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/targets"
	var response BridgeTargetsRecord
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return BridgeTargetsRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ResolveBridgeTarget(
	ctx context.Context,
	id string,
	name string,
) (record BridgeResolveTargetRecord, err error) {
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/resolve"
	var response BridgeResolveTargetRecord
	requestBody := BridgeResolveTargetRequest{Name: strings.TrimSpace(name)}
	httpResponse, err := c.doRequest(
		ctx,
		http.MethodPost,
		path,
		nil,
		requestBody,
	)
	if err != nil {
		return BridgeResolveTargetRecord{}, err
	}
	defer func() {
		if closeErr := httpResponse.Body.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("cli: close %s %s response: %w", http.MethodPost, path, closeErr))
		}
	}()
	if httpResponse.StatusCode >= 200 && httpResponse.StatusCode < 300 {
		if err := json.NewDecoder(httpResponse.Body).Decode(&response); err != nil {
			return BridgeResolveTargetRecord{}, fmt.Errorf("cli: decode %s %s response: %w", http.MethodPost, path, err)
		}
		return response, nil
	}
	if httpResponse.StatusCode == http.StatusNotFound || httpResponse.StatusCode == http.StatusUnprocessableEntity {
		body, readErr := io.ReadAll(io.LimitReader(httpResponse.Body, 1<<20))
		if readErr != nil {
			return BridgeResolveTargetRecord{}, fmt.Errorf("cli: read bridge target resolve response: %w", readErr)
		}
		if json.Unmarshal(body, &response) == nil && bridgeResolveTargetHasStructuredPayload(response) {
			return response, nil
		}
		return BridgeResolveTargetRecord{}, readAPIErrorBody(httpResponse.StatusCode, httpResponse.Status, body)
	}
	return BridgeResolveTargetRecord{}, readAPIError(httpResponse)
}

func bridgeResolveTargetHasStructuredPayload(response BridgeResolveTargetRecord) bool {
	return response.Diagnostic != nil ||
		response.Result.Match != nil ||
		response.Result.Ambiguous ||
		len(response.Result.Candidates) > 0 ||
		response.Result.Step != 0
}

func (c *unixSocketClient) ListNotificationPresets(
	ctx context.Context,
	query NotificationPresetQuery,
) (NotificationPresetListRecord, error) {
	var response NotificationPresetListRecord
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/notifications/presets",
		notificationPresetValues(query),
		nil,
		&response,
	); err != nil {
		return NotificationPresetListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetNotificationPreset(
	ctx context.Context,
	name string,
) (NotificationPresetRecord, error) {
	var response contract.NotificationPresetResponse
	path := "/api/notifications/presets/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return NotificationPresetRecord{}, err
	}
	return response.Preset, nil
}

func (c *unixSocketClient) CreateNotificationPreset(
	ctx context.Context,
	request CreateNotificationPresetRequest,
) (NotificationPresetRecord, error) {
	var response contract.NotificationPresetResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/notifications/presets", nil, request, &response); err != nil {
		return NotificationPresetRecord{}, err
	}
	return response.Preset, nil
}

func (c *unixSocketClient) UpdateNotificationPreset(
	ctx context.Context,
	name string,
	request UpdateNotificationPresetRequest,
) (NotificationPresetRecord, error) {
	var response contract.NotificationPresetResponse
	path := "/api/notifications/presets/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return NotificationPresetRecord{}, err
	}
	return response.Preset, nil
}

func (c *unixSocketClient) DeleteNotificationPreset(ctx context.Context, name string) error {
	path := "/api/notifications/presets/" + url.PathEscape(strings.TrimSpace(name))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func notificationPresetValues(query NotificationPresetQuery) url.Values {
	values := url.Values{}
	if query.Enabled != nil {
		values.Set("enabled", strconv.FormatBool(*query.Enabled))
	}
	if query.BuiltIn != nil {
		values.Set("built_in", strconv.FormatBool(*query.BuiltIn))
	}
	if strings.TrimSpace(query.Name) != "" {
		values.Set("name", strings.TrimSpace(query.Name))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func (c *unixSocketClient) ListBridgeSecretBindings(
	ctx context.Context,
	id string,
) ([]BridgeSecretBindingRecord, error) {
	var response struct {
		Bindings []BridgeSecretBindingRecord `json:"bindings"`
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/secret-bindings"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Bindings, nil
}

func (c *unixSocketClient) PutBridgeSecretBinding(
	ctx context.Context,
	id string,
	bindingName string,
	request BridgeSecretBindingRequest,
) (BridgeSecretBindingRecord, error) {
	var response struct {
		Binding BridgeSecretBindingRecord `json:"binding"`
	}
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) +
		"/secret-bindings/" + url.PathEscape(strings.TrimSpace(bindingName))
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return BridgeSecretBindingRecord{}, err
	}
	return response.Binding, nil
}

func (c *unixSocketClient) DeleteBridgeSecretBinding(ctx context.Context, id string, bindingName string) error {
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) +
		"/secret-bindings/" + url.PathEscape(strings.TrimSpace(bindingName))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) TestBridgeDelivery(
	ctx context.Context,
	id string,
	request BridgeTestDeliveryRequest,
) (BridgeTestDeliveryRecord, error) {
	var response BridgeTestDeliveryRecord
	path := "/api/bridges/" + url.PathEscape(strings.TrimSpace(id)) + "/test-delivery"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return BridgeTestDeliveryRecord{}, err
	}
	return response, nil
}
