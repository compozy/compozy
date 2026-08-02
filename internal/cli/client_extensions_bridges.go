package cli

import (
	"context"
	"fmt"

	"net/http"
	"net/url"

	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

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
	bridgeID, err := requiredBridgeControlID(id)
	if err != nil {
		return BridgeTestDeliveryRecord{}, err
	}
	if _, err := request.ToResolveDeliveryTargetRequest(bridgeID); err != nil {
		return BridgeTestDeliveryRecord{}, fmt.Errorf("cli: invalid bridge test-delivery request: %w", err)
	}
	var response BridgeTestDeliveryRecord
	path := "/api/bridges/" + url.PathEscape(bridgeID) + "/test-delivery"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return BridgeTestDeliveryRecord{}, err
	}
	return response, nil
}
