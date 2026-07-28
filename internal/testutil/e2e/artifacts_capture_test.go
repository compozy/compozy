package e2e

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactCollectorCaptureFilesDuplicateBasenamesContract(t *testing.T) {
	t.Parallel()

	t.Run("Should write distinct artifacts for duplicate source basenames", func(t *testing.T) {
		t.Parallel()

		collector := NewArtifactCollector(t)
		first := filepath.Join(t.TempDir(), "screenshot.png")
		second := filepath.Join(t.TempDir(), "screenshot.png")
		if err := os.WriteFile(first, []byte("one"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", first, err)
		}
		if err := os.WriteFile(second, []byte("two"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", second, err)
		}

		if err := collector.CaptureFiles(
			ArtifactKindBrowserScreenshots,
			[]string{first, second},
			"image/png",
		); err != nil {
			t.Fatalf("CaptureFiles(browser_screenshots) error = %v", err)
		}
		screenshotDir, ok := collector.ArtifactPath(ArtifactKindBrowserScreenshots)
		if !ok {
			t.Fatal("ArtifactPath(browser_screenshots) = missing, want present")
		}
		contents := map[string]string{
			"screenshot.png":     "one",
			"002-screenshot.png": "two",
		}
		for name, want := range contents {
			path := filepath.Join(screenshotDir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v", path, err)
			}
			if got := string(data); got != want {
				t.Fatalf("os.ReadFile(%q) = %q, want %q", path, got, want)
			}
		}
	})
}
