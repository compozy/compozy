package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) ListExtensions(ctx context.Context) ([]ExtensionRecord, error) {
	var response struct {
		Extensions []ExtensionRecord `json:"extensions"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/extensions", nil, nil, &response); err != nil {
		return nil, err
	}
	return response.Extensions, nil
}

func (c *daemonClient) SearchExtensions(
	ctx context.Context,
	request ExtensionSearchRequest,
) (ExtensionSearchRecord, error) {
	query := url.Values{
		"q":            {strings.TrimSpace(request.Query)},
		listLimitField: {strconv.Itoa(request.Limit)},
		//nolint:goconst // cursor is the external extension-search query field, not a presentation label.
		"cursor": {strings.TrimSpace(request.Cursor)},
	}
	if len(request.Sources) > 0 {
		query.Set("sources", strings.Join(request.Sources, ","))
	}
	var response ExtensionSearchRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/extensions/search", query, nil, &response); err != nil {
		return ExtensionSearchRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) InstallExtension(
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

func (c *daemonClient) PreviewExtensionInstall(
	ctx context.Context,
	request InstallExtensionRequest,
) (ExtensionInstallPreviewRecord, error) {
	var response ExtensionInstallPreviewRecord
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		"/api/extensions/preview-install",
		nil,
		request,
		&response,
	); err != nil {
		return ExtensionInstallPreviewRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) UpdateExtension(
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
		extensionPath(name, ""),
		nil,
		request,
		&response,
	); err != nil {
		return ExtensionUpdateRecord{}, err
	}
	return response.Update, nil
}

func (c *daemonClient) UpdateExtensions(
	ctx context.Context,
	request UpdateExtensionsRequest,
) ([]ExtensionUpdateRecord, error) {
	var response contract.ExtensionUpdateBatchResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/extensions/update", nil, request, &response); err != nil {
		return nil, err
	}
	return append([]ExtensionUpdateRecord(nil), response.Updates...), nil
}

func (c *daemonClient) RemoveExtension(ctx context.Context, name string) (ManagedExtensionRemoveRecord, error) {
	var response struct {
		Extension ManagedExtensionRemoveRecord `json:"extension"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodDelete,
		extensionPath(name, ""),
		nil,
		nil,
		&response,
	); err != nil {
		return ManagedExtensionRemoveRecord{}, err
	}
	return response.Extension, nil
}

func (c *daemonClient) EnableExtension(
	ctx context.Context,
	name string,
	request EnableExtensionRequest,
) (ExtensionEnableRecord, error) {
	var response ExtensionEnableRecord
	path := extensionPath(name, "enable")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ExtensionEnableRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) DisableExtension(ctx context.Context, name string) (ExtensionRecord, error) {
	return c.extensionAction(ctx, strings.TrimSpace(name), "disable")
}

func (c *daemonClient) SetExtensionEnablement(
	ctx context.Context,
	name string,
	profile string,
	enabled bool,
) (ExtensionEnablementRecord, error) {
	var response ExtensionEnablementRecord
	request := contract.SetExtensionEnablementRequest{Profile: strings.TrimSpace(profile), Enabled: enabled}
	if err := c.doJSON(ctx, http.MethodPut, extensionPath(name, "enablement"), nil, request, &response); err != nil {
		return ExtensionEnablementRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ExtensionStatus(ctx context.Context, name string) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		extensionPath(name, ""),
		nil,
		nil,
		&response,
	); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *daemonClient) ExtensionProvenance(ctx context.Context, name string) (ExtensionProvenanceRecord, error) {
	var response struct {
		Provenance ExtensionProvenanceRecord `json:"provenance"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		extensionPath(name, "provenance"),
		nil,
		nil,
		&response,
	); err != nil {
		return ExtensionProvenanceRecord{}, err
	}
	return response.Provenance, nil
}

func (c *daemonClient) ExtensionInventory(ctx context.Context, name string) (ExtensionInventoryRecord, error) {
	var response ExtensionInventoryRecord
	path := extensionPath(name, "inventory")
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return ExtensionInventoryRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) PreviewExtensionEnable(
	ctx context.Context,
	name string,
) (ExtensionEnablePreviewRecord, error) {
	var response ExtensionEnablePreviewRecord
	path := extensionPath(name, "preview")
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return ExtensionEnablePreviewRecord{}, err
	}
	return response, nil
}

func extensionPath(name string, suffix string) string {
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name))
	if suffix != "" {
		path += "/" + suffix
	}
	return path
}
