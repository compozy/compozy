package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

const (
	hostAPIBridgesDefaultKey = "default"
)

type hostAPIBridgeRegistry interface {
	GetInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error)
	UpdateInstanceState(
		ctx context.Context,
		req bridgepkg.UpdateInstanceStateRequest,
	) (*bridgepkg.BridgeInstance, error)
	BuildRoutingKey(ctx context.Context, key bridgepkg.RoutingKey) (bridgepkg.RoutingKey, error)
	ResolveRoute(ctx context.Context, key bridgepkg.RoutingKey) (*bridgepkg.BridgeRoute, error)
	ResolveOrCreateRoute(ctx context.Context, route bridgepkg.BridgeRoute) (*bridgepkg.BridgeRoute, bool, error)
	UpsertRoute(ctx context.Context, route bridgepkg.BridgeRoute) (*bridgepkg.BridgeRoute, error)
}

type hostAPIBridgeDedupStore interface {
	PutBridgeIngestDedup(ctx context.Context, record bridgepkg.IngestDedupRecord) error
	GetBridgeIngestDedup(
		ctx context.Context,
		idempotencyKey string,
		lookupAt time.Time,
	) (bridgepkg.IngestDedupRecord, error)
	DeleteExpiredBridgeIngestDedup(ctx context.Context, now time.Time) (int64, error)
}

func (h *HostAPIHandler) handleBridgesInstancesList(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.bridges == nil {
		return nil, unavailableRPCError(errors.New("bridge registry is not configured"))
	}

	var params struct{}
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	_, instances, err := h.authorizedBridgeInstances(ctx)
	if err != nil {
		return nil, err
	}
	return bridgeInstancesContract(instances), nil
}

func (h *HostAPIHandler) handleBridgesInstancesGet(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.bridges == nil {
		return nil, unavailableRPCError(errors.New("bridge registry is not configured"))
	}

	var params bridgecontract.BridgeInstanceTargetParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}

	instanceID, err := requireBridgeInstanceID(params.BridgeInstanceID)
	if err != nil {
		return nil, err
	}

	instance, err := h.authorizedBridgeInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	return bridgepkg.BridgeInstanceToContract(*instance), nil
}

func (h *HostAPIHandler) handleBridgesInstancesReportState(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.bridges == nil {
		return nil, unavailableRPCError(errors.New("bridge registry is not configured"))
	}

	var params bridgecontract.BridgesInstancesReportStateParams
	if err := decodeHostAPIParams(raw, &params); err != nil {
		return nil, err
	}
	instanceID, err := requireBridgeInstanceID(params.BridgeInstanceID)
	if err != nil {
		return nil, err
	}
	if err := params.Status.Validate(); err != nil {
		return nil, invalidParamsRPCError(err)
	}
	if params.Status.Normalize() == bridgecontract.BridgeStatusDisabled {
		return nil, invalidParamsRPCError(errors.New("bridge status disabled is operator-controlled"))
	}
	if params.ClearDegradation && params.Degradation != nil && !params.Degradation.IsZero() {
		return nil, invalidParamsRPCError(errors.New("bridge degradation cannot be cleared and set together"))
	}
	if params.Degradation != nil {
		if err := params.Degradation.Validate(); err != nil {
			return nil, invalidParamsRPCError(err)
		}
	}

	instance, err := h.authorizedBridgeInstance(ctx, instanceID)
	if err != nil {
		return nil, err
	}

	updated, err := h.bridges.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:               instance.ID,
		Enabled:          instance.Enabled,
		Status:           bridgeStatusDomain(params.Status),
		Degradation:      bridgeDegradationDomain(params.Degradation),
		ClearDegradation: params.ClearDegradation,
		UpdatedAt:        h.now(),
	})
	if err != nil {
		return nil, mapBridgeStateUpdateError(instance.ID, err)
	}

	if h.observer != nil {
		if sink, ok := h.observer.(BridgeTelemetrySink); ok {
			switch updated.Status.Normalize() {
			case bridgepkg.BridgeStatusAuthRequired:
				sink.RecordBridgeAuthFailure(updated.ID)
			case bridgepkg.BridgeStatusReady:
				sink.ClearBridgeRuntimeIssue(updated.ID)
			}
		}
	}
	return bridgepkg.BridgeInstanceToContract(*updated), nil
}

func (h *HostAPIHandler) handleBridgesMessagesIngest(ctx context.Context, raw json.RawMessage) (any, error) {
	if h.bridges == nil {
		return nil, unavailableRPCError(errors.New("bridge registry is not configured"))
	}
	if h.dedupStore == nil {
		return nil, unavailableRPCError(errors.New("bridge ingest dedup store is not configured"))
	}

	ingress, err := h.prepareBridgeIngress(ctx, raw)
	if err != nil {
		return nil, err
	}
	unlock := h.bridgeLocks.lock(ingress.lockKey)
	defer unlock()

	if err := h.maybeCleanupBridgeIngestDedup(ctx); err != nil {
		return nil, err
	}

	suppressedRoute, suppressed, err := h.suppressedBridgeIngressRoute(
		ctx,
		ingress.routingKey,
		*ingress.instance,
		strings.TrimSpace(ingress.params.IdempotencyKey),
	)
	if err != nil {
		return nil, err
	}
	if suppressed {
		return bridgecontract.BridgesMessagesIngestResult{
			SessionID:    suppressedRoute.SessionID,
			RouteCreated: false,
			RoutingKey:   bridgeRoutingKeyContract(ingress.routingKey),
		}, nil
	}
	if err := h.checkPromptDeliveryAdmission(ctx); err != nil {
		return nil, err
	}

	route, routeCreated, err := h.resolveBridgeIngressRoute(ctx, *ingress.instance, ingress.routingKey)
	if err != nil {
		return nil, err
	}

	promptBody := renderInboundMessagePrompt(ingress.params)
	route, err = h.promptBridgeRoute(
		ctx,
		*ingress.instance,
		ingress.routingKey,
		route,
		ingress.params,
		promptBody,
	)
	if err != nil {
		return nil, err
	}
	if err := h.recordBridgeIngressDedup(context.WithoutCancel(ctx), ingress.params, *ingress.instance); err != nil {
		return nil, err
	}

	return bridgecontract.BridgesMessagesIngestResult{
		SessionID:    route.SessionID,
		RouteCreated: routeCreated,
		RoutingKey:   bridgeRoutingKeyContract(ingress.routingKey),
	}, nil
}

func (h *HostAPIHandler) checkPromptDeliveryAdmission(ctx context.Context) error {
	if h.deliveryBroker == nil {
		return nil
	}
	checker, ok := h.deliveryBroker.(hostAPIDeliveryAdmissionChecker)
	if !ok {
		return nil
	}
	if err := checker.CheckPromptDeliveryAdmission(ctx); err != nil {
		return unavailableRPCError(fmt.Errorf("extension: bridge delivery admission: %w", err))
	}
	return nil
}
