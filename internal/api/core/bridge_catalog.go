package core

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

// ErrBridgeCatalogObserverUnavailable reports a missing bounded bridge catalog projection.
var ErrBridgeCatalogObserverUnavailable = errors.New("bridge catalog observer is not configured")

// ListBridges returns one bounded page from the shared public bridge catalog.
func (h *BaseHandlers) ListBridges(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}
	query, err := h.parseBridgeCatalogQuery(c)
	if err != nil {
		h.respondError(c, bridgeCatalogQueryErrorStatus(err), err)
		return
	}
	query, err = bridgepkg.NormalizeBridgeCatalogQuery(query)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	observer, err := h.bridgeCatalogObserver()
	if err != nil {
		h.respondError(c, bridgeCatalogReadErrorStatus(err), err)
		return
	}
	records, err := bridges.ListCatalogRecords(c.Request.Context(), query)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	effectiveStatuses, err := observer.QueryBridgeEffectiveStatuses(c.Request.Context(), records)
	if err != nil {
		h.respondError(c, bridgeCatalogReadErrorStatus(err), err)
		return
	}
	selection, err := bridgepkg.BuildBridgeCatalogRecordPage(records, effectiveStatuses, query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, bridgepkg.ErrBridgeCatalogQueryInvalid) ||
			errors.Is(err, bridgepkg.ErrBridgeCatalogCursorInvalid) {
			status = http.StatusBadRequest
		}
		h.respondError(c, status, err)
		return
	}
	instances, err := bridgeCatalogHydration(c.Request.Context(), bridges, selection.Items)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	page, err := bridgepkg.HydrateBridgeCatalogPage(selection, instances)
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}

	pageInstances := bridgeCatalogPageInstances(page.Items)
	health, err := bridgeCatalogHealthFromObserver(c.Request.Context(), observer, pageInstances)
	if err != nil {
		h.respondError(c, bridgeCatalogReadErrorStatus(err), err)
		return
	}
	health = h.enrichBridgeCatalogHealth(c.Request.Context(), bridges, page.Items, health)
	c.JSON(http.StatusOK, bridgeCatalogResponse(page, health))
}

func bridgeCatalogQueryErrorStatus(err error) int {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound),
		errors.Is(err, workspacepkg.ErrWorkspaceRootMissing),
		errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return StatusForWorkspaceError(err)
	default:
		return http.StatusBadRequest
	}
}

func bridgeCatalogReadErrorStatus(err error) int {
	if errors.Is(err, ErrBridgeCatalogObserverUnavailable) {
		return http.StatusServiceUnavailable
	}
	return http.StatusInternalServerError
}

func (h *BaseHandlers) parseBridgeCatalogQuery(c *gin.Context) (bridgepkg.BridgeCatalogQuery, error) {
	scope, err := h.parseBridgeScopeQuery(c.Request.Context(), c)
	if err != nil {
		return bridgepkg.BridgeCatalogQuery{}, err
	}
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return bridgepkg.BridgeCatalogQuery{}, err
	}
	return bridgepkg.BridgeCatalogQuery{
		Scope:       scope.scope,
		WorkspaceID: scope.workspaceID,
		Search:      c.Query("q"),
		Platform:    c.Query("platform"),
		Status:      bridgepkg.BridgeStatus(c.Query("status")),
		Sort:        c.Query("sort"),
		Cursor:      c.Query("cursor"),
		Limit:       limit,
	}, nil
}

func (h *BaseHandlers) bridgeCatalogObserver() (BridgeCatalogObserver, error) {
	if h == nil || h.Observer == nil {
		return nil, ErrBridgeCatalogObserverUnavailable
	}
	observer, ok := h.Observer.(BridgeCatalogObserver)
	if !ok {
		return nil, ErrBridgeCatalogObserverUnavailable
	}
	if err := observer.CheckBridgeCatalogReady(); err != nil {
		return nil, errors.Join(ErrBridgeCatalogObserverUnavailable, err)
	}
	return observer, nil
}

func bridgeCatalogHydration(
	ctx context.Context,
	bridges BridgeService,
	items []bridgepkg.BridgeCatalogRecordItem,
) ([]bridgepkg.BridgeInstance, error) {
	if len(items) == 0 {
		return []bridgepkg.BridgeInstance{}, nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Record.ID)
	}
	return bridges.ListInstancesByIDs(ctx, ids)
}

func bridgeCatalogHealthFromObserver(
	ctx context.Context,
	observer BridgeCatalogObserver,
	instances []bridgepkg.BridgeInstance,
) (map[string]contract.BridgeHealthPayload, error) {
	health := make(map[string]contract.BridgeHealthPayload, len(instances))
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
		if _, ok := requested[id]; ok {
			health[id] = BridgeHealthPayloadFromObserve(item)
		}
	}
	return health, nil
}

func (h *BaseHandlers) enrichBridgeCatalogHealth(
	ctx context.Context,
	bridges BridgeService,
	items []bridgepkg.BridgeCatalogItem,
	health map[string]contract.BridgeHealthPayload,
) map[string]contract.BridgeHealthPayload {
	if health == nil {
		health = make(map[string]contract.BridgeHealthPayload, len(items))
	}
	providerCatalog := h.bridgeProviderCatalogForCatalog(ctx, bridges, len(items))
	bindingMap, bindingsAvailable := h.bridgeCatalogSecretBindings(ctx, bridges, items, providerCatalog)
	for _, item := range items {
		instance := item.Instance
		id := strings.TrimSpace(instance.ID)
		current := health[id]
		current.Status = item.EffectiveStatus
		current = bridgeBaseHealthPayload(instance, current)
		if providerCatalog != nil && bindingsAvailable {
			provider, catalogAvailable := providerCatalog.providerForInstance(instance)
			current.Diagnostics = bridgepkg.BuildBridgeDiagnostics(bridgepkg.BridgeDiagnosticsInput{
				Instance:                 instance,
				Provider:                 provider,
				ProviderCatalogAvailable: catalogAvailable,
				SecretBindings:           bindingMap[id],
				RouteCount:               current.RouteCount,
				DeliveryBacklog:          current.DeliveryBacklog,
				DeliveryFailuresTotal:    current.DeliveryFailuresTotal,
				AuthFailuresTotal:        current.AuthFailuresTotal,
				LastError:                current.LastError,
			})
		}
		health[id] = current
	}
	return health
}

func (h *BaseHandlers) bridgeProviderCatalogForCatalog(
	ctx context.Context,
	bridges BridgeService,
	itemCount int,
) *bridgeProviderCatalog {
	if itemCount == 0 {
		return nil
	}
	catalog, err := loadBridgeProviderCatalog(ctx, bridges)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn(
				"api: bridge diagnostics provider catalog unavailable; continuing with base health",
				bridgesErrorKey,
				err,
			)
		}
		return nil
	}
	return &catalog
}

func (h *BaseHandlers) bridgeCatalogSecretBindings(
	ctx context.Context,
	bridges BridgeService,
	items []bridgepkg.BridgeCatalogItem,
	providerCatalog *bridgeProviderCatalog,
) (map[string][]bridgepkg.BridgeSecretBinding, bool) {
	bindings := make(map[string][]bridgepkg.BridgeSecretBinding)
	if providerCatalog == nil {
		return bindings, true
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		provider, _ := providerCatalog.providerForInstance(item.Instance)
		if provider != nil && bridgeProviderHasRequiredSecretSlots(*provider) {
			ids = append(ids, strings.TrimSpace(item.Instance.ID))
		}
	}
	if len(ids) == 0 {
		return bindings, true
	}
	loaded, err := bridges.ListSecretBindingsForInstances(ctx, ids)
	if err != nil {
		h.logBridgeCatalogBindingError(err)
		return bindings, false
	}
	return loaded, true
}

func (h *BaseHandlers) logBridgeCatalogBindingError(err error) {
	if h != nil && h.Logger != nil {
		h.Logger.Warn(
			"api: bridge diagnostics bindings unavailable; continuing with base health",
			bridgesErrorKey,
			err,
		)
	}
}

func bridgeCatalogPageInstances(items []bridgepkg.BridgeCatalogItem) []bridgepkg.BridgeInstance {
	instances := make([]bridgepkg.BridgeInstance, 0, len(items))
	for _, item := range items {
		instances = append(instances, item.Instance)
	}
	return instances
}

func bridgeCatalogResponse(
	page bridgepkg.BridgeCatalogPage,
	health map[string]contract.BridgeHealthPayload,
) contract.BridgesResponse {
	payloads := make([]contract.BridgePayload, 0, len(page.Items))
	for _, item := range page.Items {
		payloads = append(payloads, BridgePayloadFromBridgeInstance(item.Instance))
	}
	return contract.BridgesResponse{
		Bridges:      payloads,
		BridgeHealth: health,
		Facets: contract.BridgeCatalogFacetsPayload{
			Platforms: page.Facets.Platforms,
			Statuses:  bridgeStatusCountsPayload(page.Facets.Statuses),
		},
		Page: contract.CountedCursorPagePayload{
			NextCursor: page.NextCursor,
			HasMore:    page.HasMore,
			Total:      page.Total,
			Limit:      page.Limit,
		},
	}
}

func bridgeStatusCountsPayload(counts map[bridgepkg.BridgeStatus]int) contract.BridgeStatusCountsPayload {
	return contract.BridgeStatusCountsPayload{
		Disabled:     counts[bridgepkg.BridgeStatusDisabled],
		Starting:     counts[bridgepkg.BridgeStatusStarting],
		Ready:        counts[bridgepkg.BridgeStatusReady],
		Degraded:     counts[bridgepkg.BridgeStatusDegraded],
		AuthRequired: counts[bridgepkg.BridgeStatusAuthRequired],
		Error:        counts[bridgepkg.BridgeStatusError],
	}
}
