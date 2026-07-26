package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCodegenOpenAPITempStorage(t *testing.T) {
	t.Run("Should check OpenAPI without writable temp storage", func(t *testing.T) {
		// not parallel: t.Setenv mutates the process-wide TMPDIR for this regression.
		if runtime.GOOS == "windows" {
			t.Skip("directory permission semantics differ on windows")
		}

		openapiPath := filepath.Join(t.TempDir(), "openapi", "agh.json")
		if err := writeOpenAPI(openapiPath); err != nil {
			t.Fatalf("writeOpenAPI(%q) error = %v", openapiPath, err)
		}

		lockedTemp := filepath.Join(t.TempDir(), "locked-tmp")
		if err := os.Mkdir(lockedTemp, 0o755); err != nil {
			t.Fatalf("os.Mkdir(%q) error = %v", lockedTemp, err)
		}
		if err := os.Chmod(lockedTemp, 0o500); err != nil {
			t.Fatalf("os.Chmod(%q) error = %v", lockedTemp, err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(lockedTemp, 0o700); err != nil {
				t.Fatalf("restore directory permissions: %v", err)
			}
		})
		t.Setenv("TMPDIR", lockedTemp)

		if err := checkOpenAPI(openapiPath); err != nil {
			t.Fatalf("checkOpenAPI() error = %v", err)
		}
	})
}
