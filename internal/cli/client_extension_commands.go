package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (c *unixSocketClient) ListExtensionCommands(
	ctx context.Context,
	extension string,
	workspaceID string,
) (ExtensionCommandsRecord, error) {
	query := url.Values{}
	if extension = strings.TrimSpace(extension); extension != "" {
		query.Set("extension", extension)
	}
	if workspaceID = strings.TrimSpace(workspaceID); workspaceID != "" {
		query.Set("workspace", workspaceID)
	}
	var response ExtensionCommandsRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/extensions/commands", query, nil, &response); err != nil {
		return ExtensionCommandsRecord{}, err
	}
	return response, nil
}
