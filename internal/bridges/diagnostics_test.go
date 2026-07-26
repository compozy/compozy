package bridges

import (
	"strings"
	"testing"
)

func TestBuildBridgeDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should report route secret auth capability and transient facts", func(t *testing.T) {
		t.Parallel()

		provider := BridgeProvider{
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			Enabled:       false,
			HealthMessage: "platform disabled by extension config",
			SecretSlots: []BridgeSecretSlot{
				{Name: "bot_token", Required: true},
				{Name: "signing_key"},
			},
		}
		diagnostics := BuildBridgeDiagnostics(BridgeDiagnosticsInput{
			Instance: BridgeInstance{
				ID:            "brg-support",
				Scope:         ScopeGlobal,
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				DisplayName:   "Support",
				Enabled:       true,
				Status:        BridgeStatusAuthRequired,
				RoutingPolicy: RoutingPolicy{IncludePeer: true},
				Degradation: &BridgeDegradation{
					Reason:  BridgeDegradationReasonAuthFailed,
					Message: "provider rejected credentials",
				},
			},
			Provider:                 &provider,
			ProviderCatalogAvailable: true,
			RouteCount:               0,
			DeliveryFailuresTotal:    2,
			AuthFailuresTotal:        1,
			LastError:                "temporary gateway timeout",
		})

		byKind := bridgeDiagnosticsByKind(t, diagnostics)
		for _, kind := range []BridgeDiagnosticKind{
			BridgeDiagnosticKindUnsupportedCapability,
			BridgeDiagnosticKindMissingToken,
			BridgeDiagnosticKindUnknownDestination,
			BridgeDiagnosticKindPermissionDenied,
			BridgeDiagnosticKindTransientDeliveryFailure,
		} {
			if _, ok := byKind[kind]; !ok {
				t.Fatalf("diagnostics missing kind %q: %#v", kind, diagnostics)
			}
		}
		if got := byKind[BridgeDiagnosticKindMissingToken].SecretSlot; got != "bot_token" {
			t.Fatalf("missing token secret slot = %q, want bot_token", got)
		}
		if got := byKind[BridgeDiagnosticKindPermissionDenied].DegradationReason; got != BridgeDegradationReasonAuthFailed {
			t.Fatalf("permission degradation reason = %q, want auth_failed", got)
		}
	})

	t.Run("Should not report unknown destination when defaults identify a target", func(t *testing.T) {
		t.Parallel()

		diagnostics := BuildBridgeDiagnostics(BridgeDiagnosticsInput{
			Instance: BridgeInstance{
				ID:               "brg-default",
				Scope:            ScopeGlobal,
				Platform:         "telegram",
				ExtensionName:    "ext-telegram",
				DisplayName:      "Support",
				Enabled:          true,
				Status:           BridgeStatusReady,
				RoutingPolicy:    RoutingPolicy{IncludePeer: true},
				DeliveryDefaults: []byte("{\"peer_id\":\"peer-1\",\"mode\":\"direct-send\"}"),
			},
			Provider: &BridgeProvider{
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				Enabled:       true,
			},
			ProviderCatalogAvailable: true,
		})

		byKind := bridgeDiagnosticsByKind(t, diagnostics)
		if _, ok := byKind[BridgeDiagnosticKindUnknownDestination]; ok {
			t.Fatalf("diagnostics = %#v, did not want unknown destination", diagnostics)
		}
	})

	t.Run("Should redact sensitive provider and runtime error details from diagnostics", func(t *testing.T) {
		t.Parallel()

		provider := BridgeProvider{
			Platform:      "telegram",
			ExtensionName: "ext-telegram",
			Enabled:       false,
			HealthMessage: "claim_token=agh_claim_bridge_secret oauth_code=oauth-secret",
		}
		diagnostics := BuildBridgeDiagnostics(BridgeDiagnosticsInput{
			Instance: BridgeInstance{
				ID:            "brg-redacted",
				Platform:      "telegram",
				ExtensionName: "ext-telegram",
				Status:        BridgeStatusAuthRequired,
				RoutingPolicy: RoutingPolicy{IncludePeer: true},
				Degradation: &BridgeDegradation{
					Reason:  BridgeDegradationReasonAuthFailed,
					Message: "secret_binding=vault-ref",
				},
			},
			Provider:                 &provider,
			ProviderCatalogAvailable: true,
			AuthFailuresTotal:        1,
			DeliveryFailuresTotal:    1,
			LastError:                "mcp_auth_token=mcp-secret",
		})

		byKind := bridgeDiagnosticsByKind(t, diagnostics)
		for _, leaked := range []string{
			"agh_claim_bridge_secret",
			"oauth-secret",
			"vault-ref",
			"mcp-secret",
		} {
			for kind, diagnostic := range byKind {
				if strings.Contains(diagnostic.Message, leaked) {
					t.Fatalf("%s diagnostic leaked %q: %#v", kind, leaked, diagnostic)
				}
			}
		}
		if got := byKind[BridgeDiagnosticKindUnsupportedCapability].Message; !strings.Contains(
			got,
			"agh_claim_[REDACTED]",
		) {
			t.Fatalf("provider diagnostic = %q, want claim token placeholder", got)
		}
		if got := byKind[BridgeDiagnosticKindPermissionDenied].Message; !strings.Contains(
			got,
			"[REDACTED]",
		) {
			t.Fatalf("permission diagnostic = %q, want redacted placeholder", got)
		}
		if got := byKind[BridgeDiagnosticKindTransientDeliveryFailure].Message; !strings.Contains(
			got,
			"[REDACTED]",
		) {
			t.Fatalf("delivery diagnostic = %q, want redacted placeholder", got)
		}
	})
}

func bridgeDiagnosticsByKind(
	t *testing.T,
	diagnostics []BridgeDiagnostic,
) map[BridgeDiagnosticKind]BridgeDiagnostic {
	t.Helper()

	byKind := make(map[BridgeDiagnosticKind]BridgeDiagnostic, len(diagnostics))
	for _, diagnostic := range diagnostics {
		byKind[diagnostic.Kind] = diagnostic
	}
	return byKind
}
