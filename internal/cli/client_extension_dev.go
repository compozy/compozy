package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const extensionWorkspaceQueryKey = "workspace"

func (c *unixSocketClient) DevExtension(
	ctx context.Context,
	workspaceRef string,
	request DevLinkExtensionRequest,
) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	query := url.Values{extensionWorkspaceQueryKey: []string{strings.TrimSpace(workspaceRef)}}
	if err := c.doJSON(ctx, http.MethodPost, "/api/extensions/dev", query, request, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) ReloadDevExtension(
	ctx context.Context,
	workspaceRef string,
	name string,
	request ReloadExtensionRequest,
) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	query := url.Values{extensionWorkspaceQueryKey: []string{strings.TrimSpace(workspaceRef)}}
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name)) + "/reload"
	if err := c.doJSON(ctx, http.MethodPost, path, query, request, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) ExtensionLogs(
	ctx context.Context,
	workspaceRef string,
	name string,
	after int64,
) ([]ExtensionLogRecord, error) {
	var response struct {
		Logs []ExtensionLogRecord `json:"logs"`
	}
	query := extensionLogsQuery(workspaceRef, after, false)
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name)) + "/logs"
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return nil, err
	}
	return response.Logs, nil
}

func (c *unixSocketClient) StreamExtensionLogs(
	ctx context.Context,
	workspaceRef string,
	name string,
	after int64,
	handler SSEHandler,
) error {
	query := extensionLogsQuery(workspaceRef, after, true)
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name)) + "/logs"
	return c.doSSE(ctx, path, query, "", handler)
}

func (c *unixSocketClient) RemoveDevExtension(
	ctx context.Context,
	workspaceRef string,
	name string,
) (ManagedExtensionRemoveRecord, error) {
	var response struct {
		Extension ManagedExtensionRemoveRecord `json:"extension"`
	}
	query := url.Values{extensionWorkspaceQueryKey: []string{strings.TrimSpace(workspaceRef)}}
	path := "/api/extensions/" + url.PathEscape(strings.TrimSpace(name))
	if err := c.doJSON(ctx, http.MethodDelete, path, query, nil, &response); err != nil {
		return ManagedExtensionRemoveRecord{}, err
	}
	return response.Extension, nil
}

func extensionLogsQuery(workspaceRef string, after int64, follow bool) url.Values {
	query := url.Values{}
	if workspace := strings.TrimSpace(workspaceRef); workspace != "" {
		query.Set(extensionWorkspaceQueryKey, workspace)
	}
	if after > 0 {
		query.Set("after", strconv.FormatInt(after, 10))
	}
	if follow {
		query.Set("follow", "1")
	}
	return query
}
