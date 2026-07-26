package bridges

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
)

// BridgeDiagnosticKind identifies one operator-actionable bridge diagnostic.
type BridgeDiagnosticKind string

const (
	// BridgeDiagnosticKindUnknownDestination reports that no route/default target
	// can identify where outbound delivery should go.
	BridgeDiagnosticKindUnknownDestination BridgeDiagnosticKind = "unknown_destination"
	// BridgeDiagnosticKindMissingToken reports a required provider secret slot that
	// has no persisted binding.
	BridgeDiagnosticKindMissingToken BridgeDiagnosticKind = "missing_token"
	// BridgeDiagnosticKindPermissionDenied reports auth/permission evidence from
	// bridge status, degradation, or observed delivery auth failures.
	BridgeDiagnosticKindPermissionDenied BridgeDiagnosticKind = "permission_denied"
	// BridgeDiagnosticKindUnsupportedCapability reports a provider/capability shape
	// that cannot support this bridge instance.
	BridgeDiagnosticKindUnsupportedCapability BridgeDiagnosticKind = "unsupported_capability"
	// BridgeDiagnosticKindTransientDeliveryFailure reports delivery failure evidence
	// that should be treated as retryable/transient by operators.
	BridgeDiagnosticKindTransientDeliveryFailure BridgeDiagnosticKind = "transient_delivery_failure"
)

// BridgeDiagnosticSeverity identifies how strongly an operator should react to a
// bridge diagnostic.
type BridgeDiagnosticSeverity string

const (
	// BridgeDiagnosticSeverityInfo reports informational bridge state.
	BridgeDiagnosticSeverityInfo BridgeDiagnosticSeverity = "info"
	// BridgeDiagnosticSeverityWarning reports degraded but potentially recoverable bridge state.
	BridgeDiagnosticSeverityWarning BridgeDiagnosticSeverity = "warning"
	// BridgeDiagnosticSeverityError reports a bridge state that blocks reliable delivery.
	BridgeDiagnosticSeverityError BridgeDiagnosticSeverity = "error"
)

// BridgeDiagnostic exposes a bridge management diagnostic derived from canonical
// bridge route, provider, secret, status, degradation, and delivery telemetry.
type BridgeDiagnostic struct {
	Kind              BridgeDiagnosticKind     `json:"kind"`
	Severity          BridgeDiagnosticSeverity `json:"severity"`
	Source            string                   `json:"source"`
	Message           string                   `json:"message"`
	NextAction        string                   `json:"next_action,omitempty"`
	BridgeInstanceID  string                   `json:"bridge_instance_id,omitempty"`
	SecretSlot        string                   `json:"secret_slot,omitempty"`
	Status            BridgeStatus             `json:"status,omitempty"`
	DegradationReason BridgeDegradationReason  `json:"degradation_reason,omitempty"`
}

// BridgeDiagnosticsInput carries the existing bridge facts used to derive
// diagnostics without probing or inventing runtime health.
type BridgeDiagnosticsInput struct {
	Instance                 BridgeInstance
	Provider                 *BridgeProvider
	ProviderCatalogAvailable bool
	SecretBindings           []BridgeSecretBinding
	RouteCount               int
	DeliveryBacklog          int
	DeliveryFailuresTotal    int
	AuthFailuresTotal        int
	LastError                string
}

// BuildBridgeDiagnostics derives actionable bridge diagnostics from existing
// canonical bridge facts.
func BuildBridgeDiagnostics(input BridgeDiagnosticsInput) []BridgeDiagnostic {
	instance := input.Instance.normalize()
	bridgeDiagnostics := make([]BridgeDiagnostic, 0, 5)
	bridgeDiagnostics = append(bridgeDiagnostics, providerDiagnostics(instance, input)...)
	bridgeDiagnostics = append(bridgeDiagnostics, missingTokenDiagnostics(instance, input)...)
	bridgeDiagnostics = append(bridgeDiagnostics, destinationDiagnostics(instance, input)...)
	if diagnostic, ok := permissionDiagnostic(instance, input); ok {
		bridgeDiagnostics = append(bridgeDiagnostics, diagnostic)
	}
	if diagnostic, ok := transientDeliveryDiagnostic(instance, input); ok {
		bridgeDiagnostics = append(bridgeDiagnostics, diagnostic)
	}
	return bridgeDiagnostics
}

func providerDiagnostics(instance BridgeInstance, input BridgeDiagnosticsInput) []BridgeDiagnostic {
	if !input.ProviderCatalogAvailable {
		return nil
	}
	if input.Provider == nil {
		return []BridgeDiagnostic{{
			Kind:             BridgeDiagnosticKindUnsupportedCapability,
			Severity:         BridgeDiagnosticSeverityError,
			Source:           "provider",
			BridgeInstanceID: strings.TrimSpace(instance.ID),
			Message: fmt.Sprintf(
				"bridge provider %q for platform %q is not installed",
				strings.TrimSpace(instance.ExtensionName),
				strings.TrimSpace(instance.Platform),
			),
			NextAction: "Install or enable a bridge provider that matches this instance platform and extension.",
			Status:     instance.Status.Normalize(),
		}}
	}
	provider := input.Provider
	if provider.Enabled {
		return nil
	}
	message := fmt.Sprintf(
		"bridge provider %q for platform %q is disabled",
		strings.TrimSpace(provider.ExtensionName),
		strings.TrimSpace(provider.Platform),
	)
	if healthMessage := strings.TrimSpace(provider.HealthMessage); healthMessage != "" {
		message += ": " + healthMessage
	}
	return []BridgeDiagnostic{{
		Kind:             BridgeDiagnosticKindUnsupportedCapability,
		Severity:         BridgeDiagnosticSeverityError,
		Source:           "provider",
		BridgeInstanceID: strings.TrimSpace(instance.ID),
		Message:          sanitizeBridgeDiagnosticMessage(message),
		NextAction:       "Enable or replace the bridge provider before routing through this instance.",
		Status:           instance.Status.Normalize(),
	}}
}

func missingTokenDiagnostics(instance BridgeInstance, input BridgeDiagnosticsInput) []BridgeDiagnostic {
	if input.Provider == nil {
		return nil
	}
	bindings := make(map[string]struct{}, len(input.SecretBindings))
	for _, binding := range input.SecretBindings {
		name := strings.TrimSpace(binding.BindingName)
		if name != "" {
			bindings[name] = struct{}{}
		}
	}
	provider := input.Provider
	diagnostics := make([]BridgeDiagnostic, 0, len(provider.SecretSlots))
	for _, slot := range provider.SecretSlots {
		normalized := slot.Normalize()
		if !normalized.Required || strings.TrimSpace(normalized.Name) == "" {
			continue
		}
		if _, ok := bindings[normalized.Name]; ok {
			continue
		}
		diagnostics = append(diagnostics, BridgeDiagnostic{
			Kind:             BridgeDiagnosticKindMissingToken,
			Severity:         BridgeDiagnosticSeverityError,
			Source:           "secret_binding",
			BridgeInstanceID: strings.TrimSpace(instance.ID),
			SecretSlot:       normalized.Name,
			Message:          fmt.Sprintf("required bridge secret %q is not bound", normalized.Name),
			NextAction:       "Bind the required bridge secret before enabling outbound delivery.",
			Status:           instance.Status.Normalize(),
		})
	}
	return diagnostics
}

func destinationDiagnostics(instance BridgeInstance, input BridgeDiagnosticsInput) []BridgeDiagnostic {
	if err := instance.RoutingPolicy.Validate(); err != nil {
		return []BridgeDiagnostic{{
			Kind:             BridgeDiagnosticKindUnsupportedCapability,
			Severity:         BridgeDiagnosticSeverityError,
			Source:           "routing_policy",
			BridgeInstanceID: strings.TrimSpace(instance.ID),
			Message:          sanitizeBridgeDiagnosticMessage(err.Error()),
			NextAction:       "Update the bridge routing policy to a supported peer/group/thread shape.",
			Status:           instance.Status.Normalize(),
		}}
	}
	hasDefaultTarget, err := deliveryDefaultsCarryDestination(instance.DeliveryDefaults)
	if err != nil {
		return []BridgeDiagnostic{{
			Kind:             BridgeDiagnosticKindUnsupportedCapability,
			Severity:         BridgeDiagnosticSeverityError,
			Source:           "delivery_defaults",
			BridgeInstanceID: strings.TrimSpace(instance.ID),
			Message:          sanitizeBridgeDiagnosticMessage(err.Error()),
			NextAction:       "Update bridge delivery_defaults to a supported delivery target mode and identity.",
			Status:           instance.Status.Normalize(),
		}}
	}
	if input.RouteCount > 0 || hasDefaultTarget {
		return nil
	}
	return []BridgeDiagnostic{{
		Kind:             BridgeDiagnosticKindUnknownDestination,
		Severity:         BridgeDiagnosticSeverityWarning,
		Source:           "route",
		BridgeInstanceID: strings.TrimSpace(instance.ID),
		Message:          "bridge has no canonical route and no default outbound destination",
		NextAction:       "Create a bridge route or configure delivery_defaults with a peer_id or group_id.",
		Status:           instance.Status.Normalize(),
	}}
}

func permissionDiagnostic(instance BridgeInstance, input BridgeDiagnosticsInput) (BridgeDiagnostic, bool) {
	reason := degradationReason(instance)
	if instance.Status.Normalize() != BridgeStatusAuthRequired &&
		reason != BridgeDegradationReasonAuthFailed &&
		input.AuthFailuresTotal == 0 {
		return BridgeDiagnostic{}, false
	}
	message := "bridge has authentication or permission failures"
	if instance.Degradation != nil && strings.TrimSpace(instance.Degradation.Message) != "" {
		message = strings.TrimSpace(instance.Degradation.Message)
	}
	return BridgeDiagnostic{
		Kind:              BridgeDiagnosticKindPermissionDenied,
		Severity:          BridgeDiagnosticSeverityError,
		Source:            "auth",
		BridgeInstanceID:  strings.TrimSpace(instance.ID),
		Message:           sanitizeBridgeDiagnosticMessage(message),
		NextAction:        "Refresh bridge credentials and confirm provider-side permissions.",
		Status:            instance.Status.Normalize(),
		DegradationReason: reason,
	}, true
}

func transientDeliveryDiagnostic(instance BridgeInstance, input BridgeDiagnosticsInput) (BridgeDiagnostic, bool) {
	reason := degradationReason(instance)
	if reason != BridgeDegradationReasonRateLimited &&
		reason != BridgeDegradationReasonProviderTimeout &&
		input.DeliveryFailuresTotal == 0 &&
		input.DeliveryBacklog == 0 {
		return BridgeDiagnostic{}, false
	}
	message := strings.TrimSpace(input.LastError)
	if message == "" && instance.Degradation != nil {
		message = strings.TrimSpace(instance.Degradation.Message)
	}
	if message == "" {
		message = "bridge delivery is delayed or retrying"
	}
	return BridgeDiagnostic{
		Kind:              BridgeDiagnosticKindTransientDeliveryFailure,
		Severity:          BridgeDiagnosticSeverityWarning,
		Source:            "delivery",
		BridgeInstanceID:  strings.TrimSpace(instance.ID),
		Message:           sanitizeBridgeDiagnosticMessage(message),
		NextAction:        "Inspect delivery backlog and retry after provider rate limits or timeouts recover.",
		Status:            instance.Status.Normalize(),
		DegradationReason: reason,
	}, true
}

func deliveryDefaultsCarryDestination(raw []byte) (bool, error) {
	defaults, err := decodeDeliveryTargetDefaults(raw)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(defaults.PeerID) != "" || strings.TrimSpace(defaults.GroupID) != "", nil
}

func degradationReason(instance BridgeInstance) BridgeDegradationReason {
	if instance.Degradation == nil {
		return ""
	}
	return instance.Degradation.Reason.Normalize()
}

func sanitizeBridgeDiagnosticMessage(text string) string {
	return strings.TrimSpace(diagnostics.Redact(text))
}
