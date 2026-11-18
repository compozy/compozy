package tools

import (
	"errors"
	"fmt"
	"testing"
)

type providerDisabledCarrier struct{}

func (providerDisabledCarrier) Error() string          { return "provider \"gemini\" is disabled" }
func (providerDisabledCarrier) DiagnosticCode() string { return "provider_disabled" }

func TestProviderDisabledToolErrorPromotion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
	}{
		{
			name: "Should promote a backend error carrying provider_disabled",
			err:  fmt.Errorf("start: %w", providerDisabledCarrier{}),
		},
		{
			name: "Should promote a domain tool error that wraps provider_disabled",
			err: NewToolError(
				ErrorCodeInvalidInput, "compozy__loop_run", "invalid", fmt.Errorf("x: %w", providerDisabledCarrier{}),
			),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, normalized := range []error{
				normalizeBackendError("compozy__session_create", tc.err),
				normalizeToolError("compozy__session_create", tc.err),
			} {
				toolErr, ok := errors.AsType[*ToolError](normalized)
				if !ok || toolErr.Code != ErrorCodeProviderDisabled {
					t.Fatalf("normalized error = %#v, want code provider_disabled", normalized)
				}
				if toolErr.PublicMessage != (providerDisabledCarrier{}).Error() {
					t.Fatalf("PublicMessage = %q, want the re-enable message", toolErr.PublicMessage)
				}
			}
		})
	}

	t.Run("Should leave unrelated errors on their existing code", func(t *testing.T) {
		t.Parallel()
		toolErr, ok := errors.AsType[*ToolError](normalizeBackendError("compozy__session_create", errors.New("boom")))
		if !ok || toolErr.Code != ErrorCodeBackendFailed {
			t.Fatalf("normalizeBackendError() = %#v, want tool_backend_failed", toolErr)
		}
	})
}
