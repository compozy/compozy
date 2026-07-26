package core

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondBridge(c *gin.Context, status int, instance bridgepkg.BridgeInstance) {
	resp, err := h.bridgeResponse(c.Request.Context(), instance)
	if err != nil {
		if h != nil && h.Logger != nil {
			h.Logger.Warn(
				"api: bridge health unavailable after successful bridge mutation; returning best-effort response",
				"bridge_id",
				strings.TrimSpace(instance.ID),
				"status",
				status,
				bridgesErrorKey,
				err,
			)
		}
		c.JSON(status, contract.BridgeResponse{
			Bridge: BridgePayloadFromBridgeInstance(instance),
			Health: contract.BridgeHealthPayload{
				BridgeInstanceID: strings.TrimSpace(instance.ID),
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
	health, err := h.bridgeHealthLookup(ctx, strings.TrimSpace(instance.ID))
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
	return &contract.BridgeResponse{
		Bridge: BridgePayloadFromBridgeInstance(instance),
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
		health[strings.TrimSpace(item.BridgeInstanceID)] = BridgeHealthPayloadFromObserve(item)
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

	return healthMap[strings.TrimSpace(bridgeInstanceID)], nil
}
