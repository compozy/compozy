package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (c *daemonClient) ListExtensionsScoped(
	ctx context.Context,
	workspaceRef string,
) ([]ExtensionRecord, error) {
	var response struct {
		Extensions []ExtensionRecord `json:"extensions"`
	}
	query := url.Values{extensionWorkspaceQueryKey: []string{strings.TrimSpace(workspaceRef)}}
	if err := c.doJSON(ctx, http.MethodGet, "/api/extensions", query, nil, &response); err != nil {
		return nil, err
	}
	return response.Extensions, nil
}

func (c *daemonClient) ExtensionStatusScoped(
	ctx context.Context,
	workspaceRef string,
	name string,
) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	query := url.Values{extensionWorkspaceQueryKey: []string{strings.TrimSpace(workspaceRef)}}
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}
