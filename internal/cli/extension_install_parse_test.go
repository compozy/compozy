package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

func TestParseExtensionInstallPlan(t *testing.T) {
	t.Parallel()

	t.Run("Should parse an explicit GitHub source with a ref", func(t *testing.T) {
		t.Parallel()

		plan, err := parseExtensionInstallPlan("github:acme/x@v1", "", "", true)
		if err != nil {
			t.Fatalf("parseExtensionInstallPlan() error = %v", err)
		}
		want := []contract.InstallExtensionRequest{{
			Source:          contract.InstallExtensionSourceGitHub,
			Ref:             "acme/x@v1",
			AllowUnverified: true,
		}}
		if !reflect.DeepEqual(plan.Attempts, want) {
			t.Fatalf("plan.Attempts = %#v, want %#v", plan.Attempts, want)
		}
	})

	t.Run("Should try curated before GitHub for a bare repository", func(t *testing.T) {
		t.Parallel()

		plan, err := parseExtensionInstallPlan("acme/x", "v2", "darwin-arm64", true)
		if err != nil {
			t.Fatalf("parseExtensionInstallPlan() error = %v", err)
		}
		want := []contract.InstallExtensionRequest{
			{
				Source:          contract.InstallExtensionSourceCurated,
				Ref:             "acme/x",
				Version:         "v2",
				Asset:           "darwin-arm64",
				AllowUnverified: true,
			},
			{
				Source:          contract.InstallExtensionSourceGitHub,
				Ref:             "acme/x",
				Version:         "v2",
				Asset:           "darwin-arm64",
				AllowUnverified: true,
			},
		}
		if !reflect.DeepEqual(plan.Attempts, want) {
			t.Fatalf("plan.Attempts = %#v, want %#v", plan.Attempts, want)
		}
	})

	t.Run("Should parse an absolute filesystem path", func(t *testing.T) {
		t.Parallel()

		path := t.TempDir()
		plan, err := parseExtensionInstallPlan(path, "", "", true)
		if err != nil {
			t.Fatalf("parseExtensionInstallPlan() error = %v", err)
		}
		want := []contract.InstallExtensionRequest{{
			Source:          contract.InstallExtensionSourceLocalPath,
			Ref:             filepath.Clean(path),
			AllowUnverified: true,
		}}
		if !reflect.DeepEqual(plan.Attempts, want) {
			t.Fatalf("plan.Attempts = %#v, want %#v", plan.Attempts, want)
		}
	})

	t.Run("Should reject a missing relative path without treating it as a slug", func(t *testing.T) {
		t.Parallel()

		missing := "./missing-extension-dir"
		_, err := parseExtensionInstallPlan(missing, "", "", true)
		if err == nil {
			t.Fatal("parseExtensionInstallPlan() error = nil, want missing path error")
		}
		if !strings.Contains(err.Error(), missing) || !strings.Contains(err.Error(), "does not exist") {
			t.Fatalf("parseExtensionInstallPlan() error = %q, want path-specific diagnostic", err)
		}
	})

	t.Run("Should parse an explicit git URL with a ref", func(t *testing.T) {
		t.Parallel()

		plan, err := parseExtensionInstallPlan("git:https://example.com/r.git@v2", "", "", true)
		if err != nil {
			t.Fatalf("parseExtensionInstallPlan() error = %v", err)
		}
		want := []contract.InstallExtensionRequest{{
			Source:          contract.InstallExtensionSourceGit,
			Ref:             "https://example.com/r.git@v2",
			AllowUnverified: true,
		}}
		if !reflect.DeepEqual(plan.Attempts, want) {
			t.Fatalf("plan.Attempts = %#v, want %#v", plan.Attempts, want)
		}
	})

	t.Run("Should reject credentials embedded in an HTTP git URL", func(t *testing.T) {
		t.Parallel()

		_, err := parseExtensionInstallPlan("git:https://operator:secret@example.com/r.git@v2", "", "", true)
		if err == nil || !strings.Contains(err.Error(), "must not include credentials") {
			t.Fatalf("parseExtensionInstallPlan() error = %v, want credential rejection", err)
		}
	})
}
