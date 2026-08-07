package core

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	"github.com/compozy/compozy/internal/gateway"
)

func (h *BaseHandlers) authorizeBridgeIngressProjection(ctx context.Context, bridgeID string) error {
	if h == nil || h.Gateway == nil {
		return nil
	}
	_, err := h.Gateway.ProjectIngress(ctx, gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectBridgeInstance,
		ID:   strings.TrimSpace(bridgeID),
	})
	if errors.Is(err, gateway.ErrIngressSubjectNotFound) {
		return nil
	}
	return err
}

func (h *BaseHandlers) bridgeIngressPayload(
	ctx context.Context,
	bridgeID string,
) (*contract.GatewayIngressPayload, error) {
	if h == nil || h.Gateway == nil {
		return nil, nil
	}
	projection, err := h.Gateway.ProjectIngress(ctx, gateway.IngressSubjectRef{
		Kind: gateway.IngressSubjectBridgeInstance, ID: bridgeID,
	})
	if errors.Is(err, gateway.ErrIngressSubjectNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	payload := GatewayIngressPayload(projection)
	return &payload, nil
}

func (h *BaseHandlers) bridgePayload(
	ctx context.Context,
	instance bridgepkg.BridgeInstance,
) (contract.BridgePayload, error) {
	payload := BridgePayloadFromBridgeInstance(instance)
	ingress, err := h.bridgeIngressPayload(ctx, instance.ID)
	if err != nil {
		return contract.BridgePayload{}, err
	}
	payload.GatewayIngress = ingress
	return payload, nil
}

func (h *BaseHandlers) enrichBridgeHealthIngress(
	ctx context.Context,
	health contract.BridgeHealthPayload,
) (contract.BridgeHealthPayload, error) {
	ingress, err := h.bridgeIngressPayload(ctx, health.BridgeInstanceID)
	if err != nil {
		return contract.BridgeHealthPayload{}, err
	}
	if ingress != nil && (ingress.ConfirmedEndpointGeneration > 0 ||
		ingress.Reachability == string(gateway.IngressReachabilityBroken)) {
		health.Ingress = ingress
	}
	return health, nil
}
