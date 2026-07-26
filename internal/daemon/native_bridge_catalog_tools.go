package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	nativeListPageKey    = "page"
	nativeBridgeScopeAll = "all"
)

type bridgeCatalogInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Search      string `json:"q,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Status      string `json:"status,omitempty"`
	Sort        string `json:"sort,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type bridgeStatusInput struct {
	BridgeID    string `json:"bridge_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Search      string `json:"q,omitempty"`
	Platform    string `json:"platform,omitempty"`
	Status      string `json:"status,omitempty"`
	Sort        string `json:"sort,omitempty"`
	Cursor      string `json:"cursor,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (i bridgeStatusInput) catalogInput() bridgeCatalogInput {
	return bridgeCatalogInput{
		WorkspaceID: i.WorkspaceID,
		Scope:       i.Scope,
		Search:      i.Search,
		Platform:    i.Platform,
		Status:      i.Status,
		Sort:        i.Sort,
		Cursor:      i.Cursor,
		Limit:       i.Limit,
	}
}

type nativeBridgeCatalogResponse struct {
	Bridges      []contract.BridgePayload                `json:"bridges"`
	BridgeHealth map[string]contract.BridgeHealthPayload `json:"bridge_health"`
	Facets       contract.BridgeCatalogFacetsPayload     `json:"facets"`
	Page         contract.CountedCursorPagePayload       `json:"page"`
	Redacted     bool                                    `json:"redacted"`
}

func (n *daemonNativeTools) bridgesList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input bridgeCatalogInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	response, page, err := n.bridgeCatalogPage(ctx, scope, req.ToolID, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(response, fmt.Sprintf("%d of %d bridges", len(response.Bridges), page.Total))
}

func (n *daemonNativeTools) bridgesStatus(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input bridgeStatusInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if strings.TrimSpace(input.BridgeID) != "" {
		return n.bridgeStatusByID(ctx, scope, req.ToolID, input)
	}
	response, page, err := n.bridgeCatalogPage(ctx, scope, req.ToolID, input.catalogInput())
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	return structuredResult(map[string]any{
		"bridges":              response.Bridges,
		"bridge_health":        response.BridgeHealth,
		"facets":               response.Facets,
		nativeListPageKey:      response.Page,
		"status_counts":        nativeBridgeStatusCounts(page.Facets.Statuses),
		nativeToolsRedactedKey: true,
	}, fmt.Sprintf("%d of %d bridges", len(response.Bridges), page.Total))
}

func (n *daemonNativeTools) bridgeStatusByID(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input bridgeStatusInput,
) (toolspkg.ToolResult, error) {
	query, err := nativeBridgeCatalogQuery(id, input.catalogInput(), scope)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	query, err = bridgepkg.NormalizeBridgeCatalogQuery(query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBridgeCatalogError(id, err)
	}
	query.Cursor = ""
	query.Limit = 1
	if _, err := n.bridgeCatalogObserver(); err != nil {
		return toolspkg.ToolResult{}, nativeBridgeCatalogError(id, err)
	}
	instance, err := n.deps.Bridges.GetInstance(ctx, strings.TrimSpace(input.BridgeID))
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record := bridgepkg.BridgeCatalogRecordFromInstance(*instance)
	statuses, err := n.bridgeEffectiveStatuses(ctx, []bridgepkg.BridgeCatalogRecord{record})
	if err != nil {
		return toolspkg.ToolResult{}, nativeBridgeCatalogError(id, err)
	}
	page, err := bridgepkg.BuildBridgeCatalogPage([]bridgepkg.BridgeInstance{*instance}, statuses, query)
	if err != nil {
		return toolspkg.ToolResult{}, nativeBridgeCatalogError(id, err)
	}
	if len(page.Items) != 1 {
		return toolspkg.ToolResult{}, bridgepkg.ErrBridgeInstanceNotFound
	}
	health, err := n.bridgeHealthFor(ctx, []bridgepkg.BridgeInstance{*instance})
	if err != nil {
		return toolspkg.ToolResult{}, nativeBridgeCatalogError(id, err)
	}
	item := page.Items[0]
	mergeBridgeCatalogHealth(health, item)
	return structuredResult(map[string]any{
		"bridge":               redactedBridgePayload(*instance),
		nativeToolsHealthKey:   health[strings.TrimSpace(instance.ID)],
		nativeToolsRedactedKey: true,
	}, string(item.EffectiveStatus))
}

func (n *daemonNativeTools) bridgeCatalogPage(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	input bridgeCatalogInput,
) (nativeBridgeCatalogResponse, bridgepkg.BridgeCatalogPage, error) {
	query, err := nativeBridgeCatalogQuery(id, input, scope)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, err
	}
	if n == nil || n.deps == nil || n.deps.Bridges == nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, errors.New(
			"daemon: bridge service is required",
		)
	}
	query, err = bridgepkg.NormalizeBridgeCatalogQuery(query)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, nativeBridgeCatalogError(id, err)
	}
	if _, err := n.bridgeCatalogObserver(); err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, nativeBridgeCatalogError(id, err)
	}
	records, err := n.deps.Bridges.ListCatalogRecords(ctx, query)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, err
	}
	statuses, err := n.bridgeEffectiveStatuses(ctx, records)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, nativeBridgeCatalogError(id, err)
	}
	selection, err := bridgepkg.BuildBridgeCatalogRecordPage(records, statuses, query)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, nativeBridgeCatalogError(id, err)
	}
	instances, err := nativeBridgeCatalogHydration(ctx, n.deps.Bridges, selection.Items)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, err
	}
	page, err := bridgepkg.HydrateBridgeCatalogPage(selection, instances)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, err
	}
	pageInstances := make([]bridgepkg.BridgeInstance, 0, len(page.Items))
	for _, item := range page.Items {
		pageInstances = append(pageInstances, item.Instance)
	}
	health, err := n.bridgeHealthFor(ctx, pageInstances)
	if err != nil {
		return nativeBridgeCatalogResponse{}, bridgepkg.BridgeCatalogPage{}, nativeBridgeCatalogError(id, err)
	}
	payloads := make([]contract.BridgePayload, 0, len(page.Items))
	for _, item := range page.Items {
		payloads = append(payloads, redactedBridgePayload(item.Instance))
		mergeBridgeCatalogHealth(health, item)
	}
	response := nativeBridgeCatalogResponse{
		Bridges:      payloads,
		BridgeHealth: health,
		Facets: contract.BridgeCatalogFacetsPayload{
			Platforms: page.Facets.Platforms,
			Statuses:  nativeBridgeStatusCountsPayload(page.Facets.Statuses),
		},
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
		Redacted: true,
	}
	return response, page, nil
}

func nativeBridgeCatalogQuery(
	id toolspkg.ToolID,
	input bridgeCatalogInput,
	scope toolspkg.Scope,
) (bridgepkg.BridgeCatalogQuery, error) {
	requestedScope := strings.ToLower(strings.TrimSpace(input.Scope))
	requestedWorkspace := strings.TrimSpace(input.WorkspaceID)
	trustedWorkspace := strings.TrimSpace(scope.WorkspaceID)
	if !scope.Operator && trustedWorkspace == "" && requestedWorkspace != "" {
		return bridgepkg.BridgeCatalogQuery{}, nativeScopeMismatchError(id, "workspace_id")
	}
	workspaceID, err := nativeCallerWorkspaceInput(id, "workspace_id", requestedWorkspace, scope)
	if err != nil {
		return bridgepkg.BridgeCatalogQuery{}, err
	}
	if requestedScope == string(bridgepkg.ScopeGlobal) {
		if requestedWorkspace != "" {
			return bridgepkg.BridgeCatalogQuery{}, nativeBridgeCatalogError(
				id,
				fmt.Errorf("%w: global scope cannot include workspace id", bridgepkg.ErrBridgeCatalogQueryInvalid),
			)
		}
		workspaceID = ""
	}
	if !scope.Operator && trustedWorkspace == "" &&
		(requestedScope == "" || requestedScope == nativeBridgeScopeAll) {
		requestedScope = string(bridgepkg.ScopeGlobal)
	}
	return bridgepkg.BridgeCatalogQuery{
		Scope:       requestedScope,
		WorkspaceID: strings.TrimSpace(workspaceID),
		Search:      input.Search,
		Platform:    input.Platform,
		Status:      bridgepkg.BridgeStatus(input.Status),
		Sort:        input.Sort,
		Cursor:      input.Cursor,
		Limit:       input.Limit,
	}, nil
}

func nativeBridgeCatalogError(id toolspkg.ToolID, err error) error {
	if errors.Is(err, core.ErrBridgeCatalogObserverUnavailable) {
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
			toolspkg.ReasonDependencyMissing,
		)
	}
	if errors.Is(err, bridgepkg.ErrBridgeCatalogQueryInvalid) ||
		errors.Is(err, bridgepkg.ErrBridgeCatalogCursorInvalid) {
		return nativeNetworkInputError(id, err)
	}
	return err
}

func (n *daemonNativeTools) bridgeEffectiveStatuses(
	ctx context.Context,
	records []bridgepkg.BridgeCatalogRecord,
) (map[string]bridgepkg.BridgeStatus, error) {
	observer, err := n.bridgeCatalogObserver()
	if err != nil {
		return nil, err
	}
	return observer.QueryBridgeEffectiveStatuses(ctx, records)
}

func (n *daemonNativeTools) bridgeHealthFor(
	ctx context.Context,
	instances []bridgepkg.BridgeInstance,
) (map[string]contract.BridgeHealthPayload, error) {
	health := make(map[string]contract.BridgeHealthPayload, len(instances))
	observer, err := n.bridgeCatalogObserver()
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return health, nil
	}
	observed, err := observer.QueryBridgeHealthFor(ctx, instances)
	if err != nil {
		return nil, err
	}
	requested := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		requested[strings.TrimSpace(instance.ID)] = struct{}{}
	}
	for _, item := range observed {
		id := strings.TrimSpace(item.BridgeInstanceID)
		if _, ok := requested[id]; !ok {
			continue
		}
		payload := core.BridgeHealthPayloadFromObserve(item)
		payload.LastError = taskpkg.RedactClaimTokens(strings.TrimSpace(payload.LastError))
		health[id] = payload
	}
	return health, nil
}

func (n *daemonNativeTools) bridgeCatalogReady() bool {
	if n == nil || n.deps == nil || n.deps.Bridges == nil {
		return false
	}
	_, err := n.bridgeCatalogObserver()
	return err == nil
}

func (n *daemonNativeTools) bridgeCatalogObserver() (core.BridgeCatalogObserver, error) {
	if n == nil || n.deps == nil || n.deps.Observer == nil {
		return nil, core.ErrBridgeCatalogObserverUnavailable
	}
	observer, ok := n.deps.Observer.(core.BridgeCatalogObserver)
	if !ok {
		return nil, core.ErrBridgeCatalogObserverUnavailable
	}
	if err := observer.CheckBridgeCatalogReady(); err != nil {
		return nil, errors.Join(core.ErrBridgeCatalogObserverUnavailable, err)
	}
	return observer, nil
}

func nativeBridgeCatalogHydration(
	ctx context.Context,
	service core.BridgeService,
	items []bridgepkg.BridgeCatalogRecordItem,
) ([]bridgepkg.BridgeInstance, error) {
	if len(items) == 0 {
		return []bridgepkg.BridgeInstance{}, nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Record.ID)
	}
	return service.ListInstancesByIDs(ctx, ids)
}

func redactedBridgePayload(instance bridgepkg.BridgeInstance) contract.BridgePayload {
	payload := core.BridgePayloadFromBridgeInstance(instance)
	payload.ProviderConfig = nil
	if payload.Degradation != nil {
		payload.Degradation.Message = taskpkg.RedactClaimTokens(strings.TrimSpace(payload.Degradation.Message))
	}
	return payload
}

func mergeBridgeCatalogHealth(
	health map[string]contract.BridgeHealthPayload,
	item bridgepkg.BridgeCatalogItem,
) {
	instance := item.Instance
	key := strings.TrimSpace(instance.ID)
	value := health[key]
	value.BridgeInstanceID = key
	value.Status = item.EffectiveStatus
	if instance.Degradation != nil {
		degradation := *instance.Degradation
		degradation.Message = taskpkg.RedactClaimTokens(strings.TrimSpace(degradation.Message))
		value.Degradation = &degradation
	} else {
		value.Degradation = nil
	}
	health[key] = value
}

func nativeBridgeStatusCountsPayload(counts map[bridgepkg.BridgeStatus]int) contract.BridgeStatusCountsPayload {
	return contract.BridgeStatusCountsPayload{
		Disabled:     counts[bridgepkg.BridgeStatusDisabled],
		Starting:     counts[bridgepkg.BridgeStatusStarting],
		Ready:        counts[bridgepkg.BridgeStatusReady],
		Degraded:     counts[bridgepkg.BridgeStatusDegraded],
		AuthRequired: counts[bridgepkg.BridgeStatusAuthRequired],
		Error:        counts[bridgepkg.BridgeStatusError],
	}
}

func nativeBridgeStatusCounts(counts map[bridgepkg.BridgeStatus]int) map[string]int {
	payload := make(map[string]int)
	for _, status := range []bridgepkg.BridgeStatus{
		bridgepkg.BridgeStatusDisabled,
		bridgepkg.BridgeStatusStarting,
		bridgepkg.BridgeStatusReady,
		bridgepkg.BridgeStatusDegraded,
		bridgepkg.BridgeStatusAuthRequired,
		bridgepkg.BridgeStatusError,
	} {
		if count := counts[status]; count > 0 {
			payload[string(status)] = count
		}
	}
	return payload
}
