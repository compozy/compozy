package core

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondBridge(c *gin.Context, status int, instance bridgepkg.BridgeInstance) {
	projectionCtx, err := h.gatewayIngressReadContext(c, "bridges.read")
	if err != nil {
		h.respondGatewayError(c, errors.Join(gateway.ErrIngressForbidden, err))
		return
	}
	resp, err := h.bridgeResponse(projectionCtx, instance)
	if err != nil {
		if h != nil && h.Logger != nil {
			h.Logger.Warn(
				"api: bridge health unavailable after successful bridge mutation; returning best-effort response",
				"bridge_id",
				instance.ID,
				"status",
				status,
				bridgesErrorKey,
				err,
			)
		}
		c.JSON(status, contract.BridgeResponse{
			Bridge: BridgePayloadFromBridgeInstance(instance),
			Health: contract.BridgeHealthPayload{
				BridgeInstanceID: instance.ID,
				Status:           instance.Status,
				Degradation:      cloneBridgeDegradation(instance.Degradation),
			},
		})
		return
	}
	c.JSON(status, *resp)
}

func (h *BaseHandlers) bridgeResponse(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
) (*contract.BridgeResponse, error) {
	health, err := h.bridgeHealthLookup(ctx, instance.ID)
	if err != nil {
		return nil, err
	}
	bridges, ok := h.bridgeService()
	if !ok {
		return nil, errBridgeServiceUnavailable
	}
	health, err = h.bridgeHealthPayloadForInstance(ctx, bridges, instance, health, nil)
	if err != nil {
		return nil, err
	}
	health, err = h.enrichBridgeHealthIngress(ctx, health)
	if err != nil {
		return nil, err
	}
	bridgePayload, err := h.bridgePayload(ctx, instance)
	if err != nil {
		return nil, err
	}
	return &contract.BridgeResponse{
		Bridge: bridgePayload,
		Health: health,
	}, nil
}

func (h *BaseHandlers) bridgeHealthMap(ctx context.Context) (map[string]contract.BridgeHealthPayload, error) {
	if h == nil || h.Observer == nil {
		return nil, nil
	}

	observed, err := h.Observer.QueryBridgeHealth(ctx)
	if err != nil {
		return nil, err
	}

	health := make(map[string]contract.BridgeHealthPayload, len(observed))
	for _, item := range observed {
		payload, projectErr := h.enrichBridgeHealthIngress(ctx, BridgeHealthPayloadFromObserve(item))
		if projectErr != nil {
			return nil, projectErr
		}
		health[item.BridgeInstanceID] = payload
	}
	return health, nil
}

func (h *BaseHandlers) bridgeHealthLookup(
	ctx context.Context,
	bridgeInstanceID string,
) (contract.BridgeHealthPayload, error) {
	healthMap, err := h.bridgeHealthMap(ctx)
	if err != nil {
		return contract.BridgeHealthPayload{}, err
	}

	return healthMap[bridgeInstanceID], nil
}
