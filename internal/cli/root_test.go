// Suite: CLI execution error rendering
// Invariant: human diagnostics preserve the actionable message and suggested command carried by structured output.
// Boundary IN: root command execution error renderer.
// Boundary OUT: diagnostic authoring, owned by the originating domain packages.
package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func TestRenderHumanExecutionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{
			name: "Should render policy remediation and configuration command",
			err: extensionpkg.ValidateUnverifiedSideLoad(
				"acme/blocked",
				"local_path",
				false,
				true,
			),
			contains: []string{
				"error: Unverified extension install is blocked",
				"This side-load is disabled by extensions marketplace policy.",
				"try: compozy config set extensions.trust.allow_unverified true",
			},
		},
		{
			name: "Should render daemon remediation command",
			err: newDaemonUnavailableError(
				"/tmp/compozy.sock",
				"GET",
				"/api/extensions",
				errors.New("connection refused"),
			),
			contains: []string{
				"error: Daemon unavailable",
				"Compozy daemon is not reachable at /tmp/compozy.sock",
				"try: compozy daemon start",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			if exitCode := writeExecutionError(&stderr, nil, test.err); exitCode == 0 {
				t.Fatal("writeExecutionError() exit code = 0, want failure")
			}
			for _, want := range test.contains {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("writeExecutionError() = %q, want %q", stderr.String(), want)
				}
			}
		})
	}
}
