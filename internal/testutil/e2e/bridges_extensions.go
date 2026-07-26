package e2e

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	aghcontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func bridgePath(bridgeID string) (string, error) {
	trimmed := strings.TrimSpace(bridgeID)
	if trimmed == "" {
		return "", errors.New("bridge id is required")
	}
	return "/api/bridges/" + url.PathEscape(trimmed), nil
}

// ListExtensions fetches the installed extension projection through the daemon operator surface.
func (h *RuntimeHarness) ListExtensions(ctx context.Context) ([]aghcontract.ExtensionPayload, error) {
	var response aghcontract.ExtensionsResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/extensions", nil, &response); err != nil {
		return nil, err
	}
	return response.Extensions, nil
}

// GetExtension fetches one installed extension snapshot.
func (h *RuntimeHarness) GetExtension(
	ctx context.Context,
	name string,
) (aghcontract.ExtensionPayload, error) {
	var response aghcontract.ExtensionResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		&response,
	); err != nil {
		return aghcontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// InstallExtension installs one local extension bundle through the daemon operator surface.
func (h *RuntimeHarness) InstallExtension(
	ctx context.Context,
	request aghcontract.InstallExtensionRequest,
) (aghcontract.ExtensionPayload, error) {
	var response aghcontract.ExtensionResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/extensions", request, &response); err != nil {
		return aghcontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// EnableExtension enables one installed extension.
func (h *RuntimeHarness) EnableExtension(
	ctx context.Context,
	name string,
) (aghcontract.ExtensionPayload, error) {
	var response aghcontract.ExtensionResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name))+"/enable",
		nil,
		&response,
	); err != nil {
		return aghcontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// DisableExtension disables one installed extension.
func (h *RuntimeHarness) DisableExtension(
	ctx context.Context,
	name string,
) (aghcontract.ExtensionPayload, error) {
	var response aghcontract.ExtensionResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name))+"/disable",
		nil,
		&response,
	); err != nil {
		return aghcontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// CreateBridge persists one bridge instance through the daemon operator surface.
func (h *RuntimeHarness) CreateBridge(
	ctx context.Context,
	request aghcontract.CreateBridgeRequest,
) (aghcontract.BridgeResponse, error) {
	var response aghcontract.BridgeResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/bridges", request, &response); err != nil {
		return aghcontract.BridgeResponse{}, err
	}
	return response, nil
}

// GetBridge fetches one bridge instance plus its health projection.
func (h *RuntimeHarness) GetBridge(
	ctx context.Context,
	bridgeID string,
) (aghcontract.BridgeResponse, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return aghcontract.BridgeResponse{}, err
	}

	var response aghcontract.BridgeResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path,
		nil,
		&response,
	); err != nil {
		return aghcontract.BridgeResponse{}, err
	}
	return response, nil
}

// EnableBridge starts one persisted bridge instance.
func (h *RuntimeHarness) EnableBridge(
	ctx context.Context,
	bridgeID string,
) (aghcontract.BridgeResponse, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return aghcontract.BridgeResponse{}, err
	}

	var response aghcontract.BridgeResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodPost,
		path+"/enable",
		nil,
		&response,
	); err != nil {
		return aghcontract.BridgeResponse{}, err
	}
	return response, nil
}

// RestartBridge restarts one bridge instance while keeping its route ownership.
func (h *RuntimeHarness) RestartBridge(
	ctx context.Context,
	bridgeID string,
) (aghcontract.BridgeResponse, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return aghcontract.BridgeResponse{}, err
	}

	var response aghcontract.BridgeResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodPost,
		path+"/restart",
		nil,
		&response,
	); err != nil {
		return aghcontract.BridgeResponse{}, err
	}
	return response, nil
}

// ListBridgeRoutes fetches the persisted route set for one bridge instance.
func (h *RuntimeHarness) ListBridgeRoutes(
	ctx context.Context,
	bridgeID string,
) ([]bridgepkg.BridgeRoute, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return nil, err
	}

	var response aghcontract.BridgeRoutesResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path+"/routes",
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Routes, nil
}

// PutBridgeSecretBinding upserts one daemon-owned bridge secret binding.
func (h *RuntimeHarness) PutBridgeSecretBinding(
	ctx context.Context,
	bridgeID string,
	bindingName string,
	request aghcontract.PutBridgeSecretBindingRequest,
) (bridgepkg.BridgeSecretBinding, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return bridgepkg.BridgeSecretBinding{}, err
	}

	var response aghcontract.BridgeSecretBindingResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodPut,
		path+"/secret-bindings/"+url.PathEscape(strings.TrimSpace(bindingName)),
		request,
		&response,
	); err != nil {
		return bridgepkg.BridgeSecretBinding{}, err
	}
	return response.Binding, nil
}

// ListBridgeSecretBindings fetches the persisted secret bindings for one bridge instance.
func (h *RuntimeHarness) ListBridgeSecretBindings(
	ctx context.Context,
	bridgeID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	path, err := bridgePath(bridgeID)
	if err != nil {
		return nil, err
	}

	var response aghcontract.BridgeSecretBindingsResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		path+"/secret-bindings",
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Bindings, nil
}

// CaptureBridgeRoutes stores the persisted route projection for one bridge instance.
func (h *RuntimeHarness) CaptureBridgeRoutes(ctx context.Context, bridgeID string) error {
	routes, err := h.ListBridgeRoutes(ctx, bridgeID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindBridgeRoutes, routes)
}

// CaptureBridgeDeliveryState stores one bridge detail snapshot with additive delivery health.
func (h *RuntimeHarness) CaptureBridgeDeliveryState(ctx context.Context, bridgeID string) error {
	bridge, err := h.GetBridge(ctx, bridgeID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindBridgeDeliveryState, bridge)
}

// CaptureBridgeSecretBindings stores the persisted secret binding set for one bridge instance.
func (h *RuntimeHarness) CaptureBridgeSecretBindings(ctx context.Context, bridgeID string) error {
	bindings, err := h.ListBridgeSecretBindings(ctx, bridgeID)
	if err != nil {
		return err
	}
	return h.Artifacts.CaptureJSON(ArtifactKindBridgeSecretBindings, bindings)
}
