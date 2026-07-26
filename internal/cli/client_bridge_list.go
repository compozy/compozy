package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

// BridgeListQuery captures the shared bridge catalog query parameters.
type BridgeListQuery struct {
	Scope       string
	WorkspaceID string
	Workspace   string
	Search      string
	Platform    string
	Status      string
	Sort        string
	Cursor      string
	Limit       int
}

// BridgeListRecord is one bounded bridge catalog response.
type BridgeListRecord = contract.BridgesResponse

func (c *unixSocketClient) ListBridges(
	ctx context.Context,
	query BridgeListQuery,
) (BridgeListRecord, error) {
	values := bridgeListQueryValues(query)
	var response BridgeListRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/bridges", values, nil, &response); err != nil {
		return BridgeListRecord{}, err
	}
	return response, nil
}

func bridgeListQueryValues(query BridgeListQuery) url.Values {
	values := make(url.Values)
	setBridgeListQueryValue(values, "scope", query.Scope)
	setBridgeListQueryValue(values, "workspace_id", query.WorkspaceID)
	setBridgeListQueryValue(values, "workspace", query.Workspace)
	setBridgeListQueryValue(values, "q", query.Search)
	setBridgeListQueryValue(values, "platform", query.Platform)
	setBridgeListQueryValue(values, "status", query.Status)
	setBridgeListQueryValue(values, "sort", query.Sort)
	setBridgeListQueryValue(values, "cursor", query.Cursor)
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func setBridgeListQueryValue(values url.Values, key string, value string) {
	if normalized := strings.TrimSpace(value); normalized != "" {
		values.Set(key, normalized)
	}
}
