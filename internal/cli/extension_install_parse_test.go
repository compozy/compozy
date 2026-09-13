package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
)

// Invariant: invalid command input is refused without echoing supplied values.
// Owner: CLI input decoding. Canonical suite: extension_install_parse_test.go.
func TestExtensionInputParsing(t *testing.T) {
	t.Parallel()
	t.Run("Should reject malformed typed flags without exposing their values", func(t *testing.T) {
		t.Parallel()
		for _, value := range []string{"unknown=private-marker", "enabled=private-marker", "token=" + strings.Repeat("x", 8193)} {
			_, err := mergeExtensionInputFlags(
				nil,
				[]string{value},
				map[string]string{"enabled": "boolean", "token": "secret"},
			)
			if err == nil || strings.Contains(err.Error(), "private-marker") ||
				strings.Contains(err.Error(), strings.Repeat("x", 50)) {
				t.Fatal("invalid typed input must fail without echoing its supplied value")
			}
		}
	})
	t.Run("Should reject ambiguous or malformed file envelopes without exposing values", func(t *testing.T) {
		t.Parallel()
		for _, data := range []string{
			`{"token":{"value":"private-marker","vault_ref":"vault:mcp/shared/TOKEN"}}`,
			`{"token":{"unknown":"private-marker"}}`,
			`{"token":{"value":"private-marker"}} {}`,
			`{"token":{"value":private-marker}}`,
			`null`,
		} {
			path := filepath.Join(t.TempDir(), "inputs.json")
			if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readExtensionInputFile(path)
			if err == nil || strings.Contains(err.Error(), "private-marker") {
				t.Fatal("invalid input file must fail without echoing its content")
			}
		}
	})
}

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

	t.Run("Should name a missing bare local path instead of attempting repository lookup", func(t *testing.T) {
		t.Parallel()

		missing := "missing-extension-dir"
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

	t.Run("Should reject git transports outside public HTTPS", func(t *testing.T) {
		t.Parallel()

		for _, input := range []string{
			"git:http://example.com/r.git",
			"git:ssh://git@example.com/r.git",
			"git:git@example.com:acme/r.git",
			"git:https://169.254.169.254/r.git",
		} {
			_, err := parseExtensionInstallPlan(input, "", "", true)
			if err == nil {
				t.Fatalf("parseExtensionInstallPlan(%q) error = nil, want HTTPS public destination rejection", input)
			}
		}
	})
}
