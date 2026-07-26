package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	networkusage "github.com/compozy/agh/internal/network/usage"
	"github.com/compozy/agh/internal/store"
)

func (h *HostAPIHandler) handleNetworkStatus(ctx context.Context, raw json.RawMessage) (any, error) {
	if err := decodeHostAPIParamsStrict(raw, &struct{}{}); err != nil {
		return nil, err
	}
	service, err := h.requireHostAPINetworkService()
	if err != nil {
		return nil, err
	}
	status, err := service.Status(ctx)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return hostAPINetworkStatusPayload(status), nil
}

func (h *HostAPIHandler) handleNetworkUsage(ctx context.Context, raw json.RawMessage) (any, error) {
	var params extensioncontract.NetworkUsageParams
	if err := decodeHostAPIParamsStrict(raw, &params); err != nil {
		return nil, err
	}
	usageStore, err := h.requireHostAPINetworkUsageStore()
	if err != nil {
		return nil, err
	}
	workspaceID, err := h.hostAPINetworkWorkspaceID(ctx, params.WorkspaceID)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	setHostAPIUsageQueryValue(values, "owner_kind", params.OwnerKind)
	setHostAPIUsageQueryValue(values, "owner_id", params.OwnerID)
	setHostAPIUsageQueryValue(values, "run_id", params.RunID)
	setHostAPIUsageQueryValue(values, "channel", params.Channel)
	setHostAPIUsageQueryValue(values, "cursor", params.Cursor)
	if params.Limit != nil {
		values.Set("limit", strconv.Itoa(*params.Limit))
	}
	query, err := networkusage.ParseQuery(workspaceID, values)
	if err != nil {
		return nil, invalidParamsRPCError(err)
	}
	report, err := usageStore.GetNetworkUsage(ctx, query)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	payload, err := networkusage.ResponseFromReport(workspaceID, report)
	if err != nil {
		return nil, mapHostAPINetworkRPCError(err)
	}
	return payload, nil
}

func (h *HostAPIHandler) requireHostAPINetworkUsageStore() (store.NetworkUsageStore, error) {
	if h == nil || h.networkUsage == nil {
		return nil, unavailableRPCError(errors.New("extension: network usage store is not configured"))
	}
	return h.networkUsage, nil
}

func setHostAPIUsageQueryValue(values url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		values.Set(key, trimmed)
	}
}
