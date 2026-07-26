package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

const providerIdentityCheck = "provider.identity"

// ProviderIdentityCheckRecord maps a provider identity outcome to one safe,
// actionable check without copying provider error text into the public record.
func ProviderIdentityCheckRecord(err error, secretBindings ...string) bridgepkg.BridgeCheckRecord {
	if err == nil {
		return bridgepkg.PassCheck(providerIdentityCheck)
	}
	if errors.Is(err, context.Canceled) {
		return providerIdentityWarning(
			"Provider identity verification was canceled. Run bridge verification again.",
		)
	}

	bindings := normalizedSecretBindings(secretBindings)
	switch ClassifyError(err).Class {
	case ErrorClassRateLimit:
		return providerIdentityWarning(
			"The provider rate-limited identity verification. Wait before running bridge verification again.",
		)
	case ErrorClassTimeout:
		return providerIdentityWarning(
			"The provider identity check timed out. Check provider connectivity, then run bridge verification again.",
		)
	case ErrorClassOverloaded, ErrorClassServerError, ErrorClassTransient:
		return providerIdentityWarning(
			"The provider identity service is temporarily unavailable. Run bridge verification again.",
		)
	case ErrorClassAuth:
		return providerIdentityFailure(fmt.Sprintf(
			"Replace the required bridge secret bindings (%s), then run bridge verification again.",
			bindings,
		))
	default:
		return providerIdentityFailure(fmt.Sprintf(
			"Check provider configuration and the required bridge secret bindings (%s), "+
				"then run bridge verification again.",
			bindings,
		))
	}
}

func providerIdentityWarning(remediation string) bridgepkg.BridgeCheckRecord {
	return bridgepkg.BridgeCheckRecord{
		Check:       providerIdentityCheck,
		Status:      bridgepkg.BridgeCheckStatusWarn,
		Remediation: remediation,
	}
}

func providerIdentityFailure(remediation string) bridgepkg.BridgeCheckRecord {
	return bridgepkg.BridgeCheckRecord{
		Check:       providerIdentityCheck,
		Status:      bridgepkg.BridgeCheckStatusFail,
		Remediation: remediation,
	}
}

func normalizedSecretBindings(values []string) string {
	bindings := make([]string, 0, len(values))
	for _, value := range values {
		if binding := strings.TrimSpace(value); binding != "" {
			bindings = append(bindings, binding)
		}
	}
	slices.Sort(bindings)
	bindings = slices.Compact(bindings)
	if len(bindings) == 0 {
		return "provider credentials"
	}
	return strings.Join(bindings, ", ")
}
