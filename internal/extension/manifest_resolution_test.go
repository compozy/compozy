package extensionpkg

import (
	"strings"
	"testing"
)

func TestManifestResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve Compozy executable template", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveManifestString(
			"/tmp/ext",
			"{{compozy_executable}} __internal",
			nil,
			func() (string, error) { return "/usr/local/bin/compozy", nil },
		)
		if err != nil {
			t.Fatalf("resolveManifestString(compozy_executable) error = %v", err)
		}
		if got, want := resolved, "/usr/local/bin/compozy __internal"; got != want {
			t.Fatalf("resolveManifestString(compozy_executable) = %q, want %q", got, want)
		}
	})

	t.Run("Should reject Go test binary for Compozy executable template", func(t *testing.T) {
		t.Parallel()

		_, err := resolveManifestString(
			"/tmp/ext",
			"{{compozy_executable}} __internal",
			nil,
			func() (string, error) { return "/tmp/daemon.test", nil },
		)
		if err == nil || !strings.Contains(err.Error(), "not a compozy executable") {
			t.Fatalf("resolveManifestString(test binary) error = %v, want test binary rejection", err)
		}
	})
}
