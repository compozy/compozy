// Suite: generated bridge control SDK roots
// Invariant: every typed provider control payload is exported to extension SDK generators exactly once.
// Boundary IN: canonical SDK root registry.
// Boundary OUT: language-specific rendering, owned by codegen suites.
package contract

import "testing"

func TestSDKRootTypesIncludeBridgeControlContracts(t *testing.T) {
	t.Run("Should expose every bridge control contract exactly once", func(t *testing.T) {
		t.Parallel()

		counts := make(map[string]int)
		for _, named := range SDKRootTypes() {
			counts[named.Name]++
		}
		for _, name := range []string{
			"BridgeRuntimePurpose",
			"ControlMethod",
			"BridgeCheckStatus",
			"BridgeCheckRecord",
			"BridgeCheckRequest",
			"BridgeCheckResponse",
			"BridgeWebhookRegistrationRequest",
			"BridgeWebhookRegistrationResponse",
		} {
			if got := counts[name]; got != 1 {
				t.Fatalf("SDK root %q count = %d, want 1", name, got)
			}
		}
	})
}
