package core

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/gateway"
)

func (h *BaseHandlers) triggerPayload(
	ctx context.Context,
	trigger automationpkg.Trigger,
) (contract.TriggerPayload, error) {
	payload := TriggerPayloadFromTrigger(trigger)
	if strings.TrimSpace(trigger.WebhookID) == "" || strings.TrimSpace(trigger.EndpointSlug) == "" || h.Gateway == nil {
		return payload, nil
	}
	ref := gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: trigger.ID}
	projection, err := h.Gateway.ProjectIngress(ctx, ref)
	if errors.Is(err, gateway.ErrIngressSubjectNotFound) {
		payload.Ingress = &contract.GatewayIngressPayload{
			SubjectKind: string(ref.Kind), SubjectID: ref.ID, ScopeKind: string(trigger.Scope),
			WorkspaceID: trigger.WorkspaceID, Reachability: string(gateway.IngressReachabilityOff),
			EnablePath: "/api/gateway/surfaces",
		}
		return payload, nil
	}
	if err != nil {
		return contract.TriggerPayload{}, err
	}
	ingress := GatewayIngressPayload(projection)
	payload.Ingress = &ingress
	return payload, nil
}

func (h *BaseHandlers) triggerPayloads(
	ctx context.Context,
	triggers []automationpkg.Trigger,
) ([]contract.TriggerPayload, error) {
	payloads := make([]contract.TriggerPayload, 0, len(triggers))
	for _, trigger := range triggers {
		payload, err := h.triggerPayload(ctx, trigger)
		if err != nil {
			return nil, err
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}
