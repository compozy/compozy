package extensionpkg

import (
	"strings"
	"testing"
)

func TestManifestResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve AGH executable template", func(t *testing.T) {
		t.Parallel()

		resolved, err := resolveManifestString(
			"/tmp/ext",
			"{{agh_executable}} __internal",
			nil,
			func() (string, error) { return "/usr/local/bin/agh", nil },
		)
		if err != nil {
			t.Fatalf("resolveManifestString(agh_executable) error = %v", err)
		}
		if got, want := resolved, "/usr/local/bin/agh __internal"; got != want {
			t.Fatalf("resolveManifestString(agh_executable) = %q, want %q", got, want)
		}
	})

	t.Run("Should reject Go test binary for AGH executable template", func(t *testing.T) {
		t.Parallel()

		_, err := resolveManifestString(
			"/tmp/ext",
			"{{agh_executable}} __internal",
			nil,
			func() (string, error) { return "/tmp/daemon.test", nil },
		)
		if err == nil || !strings.Contains(err.Error(), "not an agh executable") {
			t.Fatalf("resolveManifestString(test binary) error = %v, want test binary rejection", err)
		}
	})
}
