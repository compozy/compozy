package cli

import "testing"

func TestCommandExitErrorHandlesNilUnderlyingError(t *testing.T) {
	t.Parallel()

	var nilReceiver *commandExitError
	if got := nilReceiver.Error(); got != "" {
		t.Fatalf("expected nil receiver to render empty error string, got %q", got)
	}

	err := &commandExitError{code: 2}
	if got := err.Error(); got != "" {
		t.Fatalf("expected missing inner error to render empty error string, got %q", got)
	}
}
