package telemetry_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// The privacy boundary is a leaf: the instrumented runtime packages import telemetry, so telemetry importing one of
// them would either close an import cycle or pull runtime state into the sanitizer. Registries are injected
// functions instead. The SDK direction (only telemetry builds providers and exporters) is a depguard rule.
func TestPackageBoundary(t *testing.T) {
	t.Parallel()

	t.Run("Should not import the instrumented runtime packages", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "go", "list", "-f", "{{range .Imports}}{{println .}}{{end}}", ".").Output()
		if err != nil {
			t.Fatalf("go list internal/telemetry imports error = %v", err)
		}

		const internalPrefix = "github.com/compozy/compozy/internal/"
		forbidden := map[string]struct{}{
			"session": {}, "acp": {}, "mcp": {}, "api": {}, "observe": {}, "store": {}, "daemon": {},
		}
		for importPath := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			relative, internal := strings.CutPrefix(importPath, internalPrefix)
			if !internal {
				continue
			}
			owner, _, _ := strings.Cut(relative, "/")
			if _, denied := forbidden[owner]; denied {
				t.Fatalf(
					"internal/telemetry imports %s, want no import of the %s runtime package tree",
					importPath,
					owner,
				)
			}
		}
	})
}
