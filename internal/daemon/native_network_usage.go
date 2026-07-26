package daemon

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	core "github.com/compozy/agh/internal/api/core"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type networkUsageInput struct {
	WorkspaceID string `json:"workspace_id"`
	OwnerKind   string `json:"owner_kind,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       *int   `json:"limit,omitempty"`
}

func (n *daemonNativeTools) networkUsage(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input networkUsageInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	workspaceID, err := n.nativeNetworkWorkspaceID(ctx, req.ToolID, input.WorkspaceID, scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	values := url.Values{}
	setNetworkUsageQueryValue(values, "owner_kind", input.OwnerKind)
	setNetworkUsageQueryValue(values, "owner_id", input.OwnerID)
	setNetworkUsageQueryValue(values, "run_id", input.RunID)
	setNetworkUsageQueryValue(values, "channel", input.Channel)
	setNetworkUsageQueryValue(values, "cursor", input.Cursor)
	if input.Limit != nil {
		values.Set("limit", strconv.Itoa(*input.Limit))
	}
	query, err := core.ParseNetworkUsageQuery(workspaceID, values)
	if err != nil {
		return toolspkg.ToolResult{}, nativeNetworkInputError(req.ToolID, err)
	}
	report, err := n.deps.NetworkUsage.GetNetworkUsage(ctx, query)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	payload, err := core.NetworkUsageResponseFromReport(workspaceID, report)
	if err != nil {
		return toolspkg.ToolResult{}, fmt.Errorf("daemon: convert network usage response: %w", err)
	}
	return structuredNetworkResult(
		payload,
		fmt.Sprintf("%d network wake usage rows", len(payload.Details)),
	)
}

func setNetworkUsageQueryValue(values url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		values.Set(key, trimmed)
	}
}
