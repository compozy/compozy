//go:build mage

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveOrBuildLaneBinary(t *testing.T) {
	// KEEP: AGH_TEST_DAEMON_BIN is the operator-facing override that lets E2E
	// lanes reuse a prebuilt daemon binary without leaking generated artifacts.
	// not parallel: these cases mutate process-wide environment variables with t.Setenv.
	t.Run("Should return executable override path without cleanup side effects", func(t *testing.T) {
		envVar := "AGH_TEST_DAEMON_BIN"
		overridePath := filepath.Join(t.TempDir(), laneBinaryName("agh"))
		writeExecutableFile(t, overridePath)
		t.Setenv(envVar, overridePath)

		got, cleanup, err := resolveOrBuildLaneBinary(envVar, func(string) error {
			t.Fatal("resolveOrBuildLaneBinary() invoked build for an override path")
			return nil
		}, "agh")
		if err != nil {
			t.Fatalf("resolveOrBuildLaneBinary() error = %v", err)
		}
		if got != overridePath {
			t.Fatalf("resolveOrBuildLaneBinary() path = %q, want %q", got, overridePath)
		}
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
		if _, err := os.Stat(overridePath); err != nil {
			t.Fatalf("override path after cleanup stat error = %v", err)
		}
	})

	t.Run("Should reject missing override paths", func(t *testing.T) {
		envVar := "AGH_TEST_DAEMON_BIN"
		missingPath := filepath.Join(t.TempDir(), laneBinaryName("missing-agh"))
		t.Setenv(envVar, missingPath)

		if _, _, err := resolveOrBuildLaneBinary(envVar, func(string) error {
			t.Fatal("resolveOrBuildLaneBinary() invoked build for a missing override path")
			return nil
		}, "agh"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("resolveOrBuildLaneBinary() error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should reject directory override paths", func(t *testing.T) {
		envVar := "AGH_TEST_DAEMON_BIN"
		overridePath := t.TempDir()
		t.Setenv(envVar, overridePath)

		_, _, err := resolveOrBuildLaneBinary(envVar, func(string) error {
			t.Fatal("resolveOrBuildLaneBinary() invoked build for a directory override path")
			return nil
		}, "agh")
		if err == nil {
			t.Fatal("resolveOrBuildLaneBinary() error = nil, want non-nil")
		}
		if !errors.Is(err, errLaneBinaryOverrideDirectory) {
			t.Fatalf("resolveOrBuildLaneBinary() error = %v, want directory sentinel", err)
		}
	})

	if runtime.GOOS != "windows" {
		t.Run("Should reject non-executable override paths", func(t *testing.T) {
			envVar := "AGH_TEST_DAEMON_BIN"
			overridePath := filepath.Join(t.TempDir(), "agh")
			if err := os.WriteFile(overridePath, []byte("binary"), 0o644); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", overridePath, err)
			}
			t.Setenv(envVar, overridePath)

			_, _, err := resolveOrBuildLaneBinary(envVar, func(string) error {
				t.Fatal("resolveOrBuildLaneBinary() invoked build for a non-executable override path")
				return nil
			}, "agh")
			if err == nil {
				t.Fatal("resolveOrBuildLaneBinary() error = nil, want non-nil")
			}
			if !errors.Is(err, errLaneBinaryOverrideNotExecutable) {
				t.Fatalf("resolveOrBuildLaneBinary() error = %v, want non-executable sentinel", err)
			}
		})
	}

	t.Run("Should remove generated binary directories during cleanup", func(t *testing.T) {
		envVar := "AGH_TEST_DAEMON_BIN"
		t.Setenv(envVar, "")
		var buildDir string

		got, cleanup, err := resolveOrBuildLaneBinary(envVar, func(outputPath string) error {
			buildDir = filepath.Dir(outputPath)
			if err := os.WriteFile(outputPath, []byte("binary"), 0o755); err != nil {
				return err
			}
			return nil
		}, "agh")
		if err != nil {
			t.Fatalf("resolveOrBuildLaneBinary() error = %v", err)
		}
		if buildDir == "" {
			t.Fatal("resolveOrBuildLaneBinary() did not provide an output path to build")
		}
		if got != filepath.Join(buildDir, laneBinaryName("agh")) {
			t.Fatalf("resolveOrBuildLaneBinary() path = %q, want output inside %q", got, buildDir)
		}
		if _, err := os.Stat(buildDir); err != nil {
			t.Fatalf("build dir before cleanup stat error = %v", err)
		}
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup() error = %v", err)
		}
		if _, err := os.Stat(buildDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("build dir after cleanup stat error = %v, want os.ErrNotExist", err)
		}
	})
}

func writeExecutableFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("binary"), 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("os.Chmod(%q) error = %v", path, err)
	}
}
