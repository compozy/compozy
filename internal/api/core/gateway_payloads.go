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
			Name: diagnostics.RedactAndBound(provider.Name, maxDiagnosticPayloadBytes),
			Tier: string(provider.Tier), Desired: string(provider.Desired),
			Observed: string(provider.Observed), Generation: provider.Generation,
			Health: string(provider.Health),
			Cause:  diagnostics.RedactAndBound(provider.Cause, maxDiagnosticPayloadBytes),
		})
	}
	for _, address := range status.Addresses {
		payload.Addresses = append(payload.Addresses, contract.GatewayAddressPayload{
			Tier:    string(address.Tier),
			Address: diagnostics.RedactAndBound(address.Address, maxDiagnosticPayloadBytes),
			Live:    address.Live,
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

func GatewayAuditPayload(report gateway.AuditReport) contract.GatewayAuditPayload {
	payload := contract.GatewayAuditPayload{
		Ran: report.Ran, NoFindings: report.NoFindings, LocalOnly: report.LocalOnly,
		Status: GatewayStatusPayload(report.Status),
		Auth: contract.GatewayAuditAuthPayload{
			Mode:              diagnostics.RedactAndBound(report.Auth.Mode, maxDiagnosticPayloadBytes),
			RequiredRemotely:  report.Auth.RequiredRemotely,
			ActiveDeviceCount: report.Auth.ActiveDeviceCount,
		},
		DeviceHighlights: contract.GatewayDeviceHighlights{
			Active: report.DeviceHighlights.Active, Revoked: report.DeviceHighlights.Revoked,
			Stale: report.DeviceHighlights.Stale, RecentlyPaired: report.DeviceHighlights.RecentlyPaired,
			StaleDeviceIDs: make([]string, 0, len(report.DeviceHighlights.StaleDeviceIDs)),
		},
		Findings: make([]contract.GatewayAuditFindingPayload, 0, len(report.Findings)),
	}
	for _, id := range report.DeviceHighlights.StaleDeviceIDs {
		payload.DeviceHighlights.StaleDeviceIDs = append(
			payload.DeviceHighlights.StaleDeviceIDs,
			diagnostics.RedactAndBound(id, maxDiagnosticPayloadBytes),
		)
	}
	for _, finding := range report.Findings {
		payload.Findings = append(payload.Findings, contract.GatewayAuditFindingPayload{
			ID:          diagnostics.RedactAndBound(finding.ID, maxDiagnosticPayloadBytes),
			Severity:    string(finding.Severity),
			Summary:     diagnostics.RedactAndBound(finding.Summary, maxDiagnosticPayloadBytes),
			Remediation: diagnostics.RedactAndBound(finding.Remediation, maxDiagnosticPayloadBytes),
			Tier:        string(finding.Tier),
			Resource:    diagnostics.RedactAndBound(finding.Resource, maxDiagnosticPayloadBytes),
		})
	}
	return payload
}

func GatewayIngressPayload(projection gateway.IngressProjection) contract.GatewayIngressPayload {
	payload := contract.GatewayIngressPayload{
		SubjectKind:                 string(projection.Subject.Kind),
		SubjectID:                   diagnostics.RedactAndBound(projection.Subject.ID, maxDiagnosticPayloadBytes),
		ScopeKind:                   string(projection.Scope),
		WorkspaceID:                 diagnostics.RedactAndBound(projection.WorkspaceID, maxDiagnosticPayloadBytes),
		URL:                         diagnostics.RedactAndBound(projection.URL, maxDiagnosticPayloadBytes),
		Reachability:                string(projection.Reachability),
		EndpointGeneration:          projection.EndpointGeneration,
		ConfirmedEndpointGeneration: projection.ConfirmedEndpointGeneration,
		EnablePath:                  diagnostics.RedactAndBound(projection.EnablePath, maxDiagnosticPayloadBytes),
	}
	if !projection.ConfirmedAt.IsZero() {
		confirmedAt := projection.ConfirmedAt
		payload.ConfirmedAt = &confirmedAt
	}
	return payload
}

func GatewayIngressUnbindPayload(result gateway.UnbindResult) contract.GatewayIngressUnbindResponse {
	return contract.GatewayIngressUnbindResponse{
		SubjectKind: string(result.Subject.Kind),
		SubjectID: diagnostics.RedactAndBound(
			result.Subject.ID,
			maxDiagnosticPayloadBytes,
		),
		Changed: result.Changed,
	}
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
		ID:            diagnostics.RedactAndBound(device.ID, maxDiagnosticPayloadBytes),
		Name:          diagnostics.RedactAndBound(device.Name, maxDiagnosticPayloadBytes),
		ActorKind:     string(device.ActorKind),
		PairingOrigin: diagnostics.RedactAndBound(device.PairingOrigin, maxDiagnosticPayloadBytes),
		RevokeEpoch:   device.RevokeEpoch,
		CreatedAt:     device.CreatedAt,
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
