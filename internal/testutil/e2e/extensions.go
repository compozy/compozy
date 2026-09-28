package e2e

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
)

// ListExtensions fetches the installed extension projection through the daemon operator surface.
func (h *RuntimeHarness) ListExtensions(ctx context.Context) ([]compozycontract.ExtensionPayload, error) {
	var response compozycontract.ExtensionsResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/extensions", nil, &response); err != nil {
		return nil, err
	}
	return response.Extensions, nil
}

// GetExtension fetches one installed extension snapshot.
func (h *RuntimeHarness) GetExtension(
	ctx context.Context,
	name string,
) (compozycontract.ExtensionPayload, error) {
	var response compozycontract.ExtensionResponse
	if err := h.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name)),
		nil,
		&response,
	); err != nil {
		return compozycontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// InstallExtension installs one local extension bundle through the daemon operator surface.
func (h *RuntimeHarness) InstallExtension(
	ctx context.Context,
	request compozycontract.InstallExtensionRequest,
) (compozycontract.ExtensionPayload, error) {
	var response compozycontract.ExtensionResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/extensions", request, &response); err != nil {
		return compozycontract.ExtensionPayload{}, err
	}
	return response.Extension, nil
}

// SetExtensionEnablement changes one installed extension in a named profile.
func (h *RuntimeHarness) SetExtensionEnablement(
	ctx context.Context,
	name string,
	profile string,
	enabled bool,
) (compozycontract.ExtensionEnablementPayload, error) {
	var response compozycontract.ExtensionEnablementPayload
	if err := h.UDSJSON(
		ctx,
		http.MethodPut,
		"/api/extensions/"+url.PathEscape(strings.TrimSpace(name))+"/enablement",
		compozycontract.SetExtensionEnablementRequest{
			Profile: strings.TrimSpace(profile), Enabled: enabled,
		},
		&response,
	); err != nil {
		return compozycontract.ExtensionEnablementPayload{}, err
	}
	return response, nil
}
