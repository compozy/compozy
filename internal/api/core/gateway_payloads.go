package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/gateway"
)

func GatewayStatusPayload(status gateway.Status) contract.GatewayStatusPayload {
	payload := contract.GatewayStatusPayload{
		Enabled: status.Enabled, Changed: status.Changed,
		Tiers:     make([]contract.GatewayTierPayload, 0, len(status.Tiers)),
		Surfaces:  make([]contract.GatewaySurfacePayload, 0, len(status.Surfaces)),
		Providers: make([]contract.GatewayProviderPayload, 0, len(status.Providers)),
		Addresses: make([]contract.GatewayAddressPayload, 0, len(status.Addresses)),
		Devices:   GatewayDevicePayloads(status.Devices),
		Bindings:  make([]contract.GatewayIngressPayload, 0, len(status.Bindings)),
	}
	for _, tier := range status.Tiers {
		payload.Tiers = append(payload.Tiers, contract.GatewayTierPayload{
			Tier: string(tier.Tier), Desired: string(tier.Desired),
			Observed: tier.Observed, ListenerAddress: tier.ListenerAddress,
			Advertised: tier.Advertised,
		})
	}
	for _, surface := range status.Surfaces {
		payload.Surfaces = append(payload.Surfaces, contract.GatewaySurfacePayload{
			Surface: string(surface.Surface), Tier: string(surface.Tier),
			Desired: string(surface.Desired), Observed: string(surface.Observed),
			Generation: surface.Generation,
		})
	}
	for _, provider := range status.Providers {
		payload.Providers = append(payload.Providers, contract.GatewayProviderPayload{
			Name: provider.Name, Tier: string(provider.Tier), Desired: string(provider.Desired),
			Observed: string(provider.Observed), Generation: provider.Generation,
			Health: string(provider.Health),
			Cause:  diagnostics.RedactAndBound(provider.Cause, maxDiagnosticPayloadBytes),
		})
	}
	for _, address := range status.Addresses {
		payload.Addresses = append(payload.Addresses, contract.GatewayAddressPayload{
			Tier: string(address.Tier), Address: address.Address, Live: address.Live,
		})
	}
	for _, binding := range status.Bindings {
		payload.Bindings = append(payload.Bindings, GatewayIngressPayload(binding))
	}
	if status.Refusal != nil {
		payload.Refusal = &contract.GatewayRefusalPayload{
			Cause: diagnostics.RedactAndBound(status.Refusal.Cause, maxDiagnosticPayloadBytes),
			Fix:   diagnostics.RedactAndBound(status.Refusal.Fix, maxDiagnosticPayloadBytes),
		}
	}
	return payload
}

func GatewayIngressPayload(projection gateway.IngressProjection) contract.GatewayIngressPayload {
	payload := contract.GatewayIngressPayload{
		SubjectKind: string(projection.Subject.Kind), SubjectID: projection.Subject.ID,
		ScopeKind: string(projection.Scope), WorkspaceID: projection.WorkspaceID,
		URL: projection.URL, Reachability: string(projection.Reachability),
		EndpointGeneration:          projection.EndpointGeneration,
		ConfirmedEndpointGeneration: projection.ConfirmedEndpointGeneration,
		EnablePath:                  projection.EnablePath,
	}
	if !projection.ConfirmedAt.IsZero() {
		confirmedAt := projection.ConfirmedAt
		payload.ConfirmedAt = &confirmedAt
	}
	return payload
}

func GatewayDevicePayloads(devices []gateway.DeviceSession) []contract.GatewayDevicePayload {
	payloads := make([]contract.GatewayDevicePayload, 0, len(devices))
	for _, device := range devices {
		payloads = append(payloads, GatewayDevicePayload(device))
	}
	return payloads
}

func GatewayDevicePayload(device gateway.DeviceSession) contract.GatewayDevicePayload {
	payload := contract.GatewayDevicePayload{
		ID: device.ID, Name: device.Name, ActorKind: string(device.ActorKind),
		PairingOrigin: device.PairingOrigin, RevokeEpoch: device.RevokeEpoch,
		CreatedAt: device.CreatedAt,
	}
	if !device.LastSeenAt.IsZero() {
		lastSeen := device.LastSeenAt
		payload.LastSeenAt = &lastSeen
	}
	if !device.RevokedAt.IsZero() {
		revoked := device.RevokedAt
		payload.RevokedAt = &revoked
	}
	return payload
}

func GatewayIssuedCredentialPayload(issued gateway.IssuedCredential) contract.GatewayIssuedCredentialPayload {
	return contract.GatewayIssuedCredentialPayload{
		Device: GatewayDevicePayload(issued.Device), Credential: issued.Credential,
	}
}

func GatewayPairingArtifactPayload(artifact gateway.PairingArtifact) contract.GatewayPairingArtifactPayload {
	return contract.GatewayPairingArtifactPayload{
		Artifact: artifact.Artifact, ExpiresAt: artifact.ExpiresAt,
	}
}

func GatewayStreamTicketPayload(ticket gateway.StreamTicket) contract.GatewayStreamTicketPayload {
	return contract.GatewayStreamTicketPayload{
		Ticket: ticket.Ticket, ExpiresAt: ticket.ExpiresAt,
	}
}
