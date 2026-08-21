//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

// Invariant: install preview and commit use one declared-profile seed, and the
// canonical profile vault write is the only action that clears needs-setup.
// Owner: daemon extension distribution integration.
// Canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes.
func testDaemonE2EExtensionDeclaredProfileSetup(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
		}},
	})
	sourceDir := filepath.Join(harness.WorkspaceRoot, "declared-profile-kit")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("create declared-profile extension directory: %v", err)
	}
	manifest := map[string]any{
		"extension": map[string]any{
			"description":         "Declared profile runtime fixture",
			"min_compozy_version": "0.0.0",
			"name":                "declared-profile-kit",
			"version":             "1.0.0",
		},
		"profiles": []map[string]any{{
			"color": "#5fbf85",
			"credentials": []map[string]string{{
				"provider": "openai",
				"slot":     "api_key",
			}},
			"icon": "chart-line",
			"name": "growth",
		}},
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("encode declared-profile manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "extension.json"), encoded, 0o600); err != nil {
		t.Fatalf("write declared-profile manifest: %v", err)
	}

	request := compozycontract.InstallExtensionRequest{
		Source:          compozycontract.InstallExtensionSourceLocalPath,
		Ref:             sourceDir,
		AllowUnverified: true,
	}
	var preview compozycontract.ExtensionInstallPreviewPayload
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		"/api/extensions/preview-install",
		request,
		&preview,
	); err != nil {
		t.Fatalf("preview declared-profile install: %v", err)
	}
	if preview.Name != "declared-profile-kit" || len(preview.DeclaredProfiles) != 1 ||
		!preview.DeclaredProfiles[0].Create || len(preview.DeclaredProfiles[0].Credentials) != 1 {
		t.Fatalf("declared-profile install preview = %#v, want growth creation and one credential", preview)
	}

	var installed compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&installed,
		"extension",
		"install",
		sourceDir,
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if installed.Name != "declared-profile-kit" || !installed.Enabled {
		t.Fatalf("installed extension = %#v, want default-on declared-profile-kit", installed)
	}

	var profiles []compozycontract.Profile
	runExtensionAuthoringCLI(t, ctx, harness, &profiles, "profile", "list", "-o", "json")
	growth := findProfileByName(profiles, "growth")
	if growth == nil || !growth.NeedsSetup || len(growth.CredentialRequirements) != 1 {
		t.Fatalf("growth profile after install = %#v, want one missing credential", growth)
	}
	runExtensionAuthoringCLI(t, ctx, harness, nil, "profile", "use", "growth", "-o", "json")

	const secret = "declared-profile-e2e-secret"
	stdout, stderr, err := harness.CLI.RunInDirWithInput(
		ctx,
		harness.WorkspaceRoot,
		strings.NewReader(secret+"\n"),
		"secret",
		"set",
		"providers/openai/api_key",
		"--value-stdin",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("set declared-profile credential error = %v; stderr=%s", err, stderr)
	}
	if strings.Contains(stdout, secret) || strings.Contains(stderr, secret) {
		t.Fatalf("secret set transcript leaked plaintext: stdout=%s stderr=%s", stdout, stderr)
	}
	var saved struct {
		Profile string `json:"profile"`
		Ref     string `json:"ref"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal([]byte(stdout), &saved); err != nil {
		t.Fatalf("decode profile secret output: %v; stdout=%s", err, stdout)
	}
	if saved.Profile != "growth" || saved.Ref != "vault:profiles/growth/providers/openai/api_key" ||
		saved.Status != "saved" {
		t.Fatalf("profile secret output = %#v, want growth vault ref", saved)
	}

	var completed compozycontract.Profile
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/profiles/growth", nil, &completed); err != nil {
		t.Fatalf("get completed growth profile: %v", err)
	}
	if completed.NeedsSetup || len(completed.CredentialRequirements) != 0 {
		t.Fatalf("growth profile after secret set = %#v, want setup complete", completed)
	}
}

func findProfileByName(profiles []compozycontract.Profile, name string) *compozycontract.Profile {
	for index := range profiles {
		if profiles[index].Name == name {
			return &profiles[index]
		}
	}
	return nil
}
