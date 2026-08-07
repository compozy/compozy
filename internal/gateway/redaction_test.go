package gateway_test

import (
	"encoding/json"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/gateway"
)

func TestGatewayOutputRedaction(t *testing.T) {
	t.Parallel()

	t.Run("Should remove raw claim tokens from every audit payload field [UT-120]", func(t *testing.T) {
		t.Parallel()

		const secret = "compozy_claim_gateway-output-secret"
		data := marshalAuditPayload(t, auditReportWithSecret(secret))
		if strings.Contains(string(data), secret) {
			t.Fatalf("gateway audit payload leaked claim token: %s", data)
		}
		if !strings.Contains(string(data), "[REDACTED]") {
			t.Fatalf("gateway audit payload omitted redaction marker: %s", data)
		}
	})

	t.Run("Should remove device credentials pairing artifacts and stream tickets [UT-121]", func(t *testing.T) {
		t.Parallel()

		for _, secret := range []string{
			"cpz_gwd_device-credential-material-123456",
			"cpz_gwp_pairing-artifact-material-123456",
			"cpz_gwt_stream-ticket-material-123456",
		} {
			secret := secret
			t.Run("Should redact "+secret[:7], func(t *testing.T) {
				t.Parallel()
				data := marshalAuditPayload(t, auditReportWithSecret(secret))
				if strings.Contains(string(data), secret) {
					t.Fatalf("gateway audit payload leaked gateway secret: %s", data)
				}
			})
		}
	})

	t.Run("Should redact secret-shaped values even when stored in the wrong field [UT-122]", func(t *testing.T) {
		t.Parallel()

		const secret = "sk-gateway-wrong-field-secret-value"
		data := marshalAuditPayload(t, auditReportWithSecret(secret))
		if strings.Contains(string(data), secret) {
			t.Fatalf("gateway audit payload leaked wrong-field secret: %s", data)
		}
		unbind, err := json.Marshal(core.GatewayIngressUnbindPayload(gateway.UnbindResult{
			Subject: gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: secret},
			Changed: true,
		}))
		if err != nil {
			t.Fatalf("json.Marshal(GatewayIngressUnbindPayload()) error = %v", err)
		}
		if strings.Contains(string(unbind), secret) {
			t.Fatalf("gateway ingress unbind payload leaked wrong-field secret: %s", unbind)
		}
	})
}

func auditReportWithSecret(secret string) gateway.AuditReport {
	status := gateway.Status{
		Enabled: true,
		Providers: []gateway.ProviderStatus{{
			Name: secret, Tier: gateway.TierPrivate, Desired: gateway.DesiredEnabled,
			Observed: gateway.ProviderDegraded, Health: gateway.HealthDegraded, Cause: secret,
		}},
		Addresses: []gateway.AddressStatus{{
			Tier: gateway.TierPrivate, Address: "https://gateway.test/" + secret, Live: true,
		}},
		Devices: []gateway.DeviceSession{{
			ID: "dev-safe", Name: secret, PairingOrigin: secret,
		}},
		Bindings: []gateway.IngressBindingStatus{{
			Subject:     gateway.IngressSubjectRef{Kind: gateway.IngressSubjectWebhookTrigger, ID: secret},
			WorkspaceID: secret, URL: "https://gateway.test/" + secret,
			Reachability: gateway.IngressReachabilityReconfirmation,
		}},
		Refusal: &gateway.Refusal{Cause: secret, Fix: "remove " + secret},
	}
	return gateway.AuditReport{
		Ran: true, Status: status,
		Auth:             gateway.AuditAuthPosture{Mode: secret, RequiredRemotely: true},
		DeviceHighlights: gateway.AuditDeviceHighlights{StaleDeviceIDs: []string{secret}},
		Findings: []gateway.AuditFinding{{
			ID: "gateway.test", Severity: gateway.AuditSeverityCritical,
			Summary: secret, Remediation: "remove " + secret, Resource: secret,
		}},
	}
}

func marshalAuditPayload(t *testing.T, report gateway.AuditReport) []byte {
	t.Helper()
	data, err := json.Marshal(core.GatewayAuditPayload(report))
	if err != nil {
		t.Fatalf("json.Marshal(GatewayAuditPayload()) error = %v", err)
	}
	return data
}
