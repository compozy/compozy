package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryReturnsEmptyWhenNoExtensionsInstalled(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, _, _, _, _ := newTestDiscovery(t, false)

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Discovered) != 0 {
		t.Fatalf("len(Discovered) = %d, want 0", len(result.Discovered))
	}
	if len(result.Extensions) != 0 {
		t.Fatalf("len(Extensions) = %d, want 0", len(result.Extensions))
	}
	if len(result.Failures) != 0 {
		t.Fatalf("len(Failures) = %d, want 0", len(result.Failures))
	}
}

func TestDiscoveryReturnsBundledExtensionWhenOnlyBundledPopulated(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, _, _, _, bundledRoot := newTestDiscovery(t, false)
	writeManifestJSON(t, filepath.Join(bundledRoot, "bundled-only"), manifestFixture("bundled-only"))

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Discovered) != 1 {
		t.Fatalf("len(Discovered) = %d, want 1", len(result.Discovered))
	}
	if len(result.Extensions) != 1 {
		t.Fatalf("len(Extensions) = %d, want 1", len(result.Extensions))
	}
	if got := result.Extensions[0].Ref.Source; got != SourceBundled {
		t.Fatalf("Extensions[0].Ref.Source = %q, want %q", got, SourceBundled)
	}
	if !result.Extensions[0].Enabled {
		t.Fatal("Extensions[0].Enabled = false, want true")
	}
}

func TestDiscoveryUserOverridesBundled(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, store, homeDir, _, bundledRoot := newTestDiscovery(t, false)
	writeManifestJSON(t, filepath.Join(bundledRoot, "shared"), manifestFixture("shared"))
	writeManifestJSON(t, userExtensionDir(homeDir, "shared"), manifestFixture("shared"))
	enableUserExtension(t, store, "shared")

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(result.Discovered) != 2 {
		t.Fatalf("len(Discovered) = %d, want 2", len(result.Discovered))
	}
	if len(result.Extensions) != 1 {
		t.Fatalf("len(Extensions) = %d, want 1", len(result.Extensions))
	}
	if got := result.Extensions[0].Ref.Source; got != SourceUser {
		t.Fatalf("Extensions[0].Ref.Source = %q, want %q", got, SourceUser)
	}
	if len(result.Overrides) != 1 {
		t.Fatalf("len(Overrides) = %d, want 1", len(result.Overrides))
	}
	if got := result.Overrides[0].Winner.Source; got != SourceUser {
		t.Fatalf("Overrides[0].Winner.Source = %q, want %q", got, SourceUser)
	}
	if got := result.Overrides[0].Loser.Source; got != SourceBundled {
		t.Fatalf("Overrides[0].Loser.Source = %q, want %q", got, SourceBundled)
	}
}

func TestDiscoveryWorkspaceOverridesAllLevels(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, store, homeDir, workspaceRoot, bundledRoot := newTestDiscovery(t, false)
	writeManifestJSON(t, filepath.Join(bundledRoot, "shared"), manifestFixture("shared"))
	writeManifestJSON(t, userExtensionDir(homeDir, "shared"), manifestFixture("shared"))
	writeManifestJSON(t, workspaceExtensionDir(workspaceRoot, "shared-workspace"), manifestFixture("shared"))
	enableUserExtension(t, store, "shared")
	enableWorkspaceExtension(t, store, workspaceRoot, "shared")

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(result.Discovered) != 3 {
		t.Fatalf("len(Discovered) = %d, want 3", len(result.Discovered))
	}
	if len(result.Extensions) != 1 {
		t.Fatalf("len(Extensions) = %d, want 1", len(result.Extensions))
	}
	if got := result.Extensions[0].Ref.Source; got != SourceWorkspace {
		t.Fatalf("Extensions[0].Ref.Source = %q, want %q", got, SourceWorkspace)
	}
	if len(result.Overrides) != 2 {
		t.Fatalf("len(Overrides) = %d, want 2", len(result.Overrides))
	}
	if got := result.Overrides[0].Winner.Source; got != SourceWorkspace {
		t.Fatalf("Overrides[0].Winner.Source = %q, want %q", got, SourceWorkspace)
	}
	if got := result.Overrides[0].Loser.Source; got != SourceUser {
		t.Fatalf("Overrides[0].Loser.Source = %q, want %q", got, SourceUser)
	}
	if got := result.Overrides[1].Loser.Source; got != SourceBundled {
		t.Fatalf("Overrides[1].Loser.Source = %q, want %q", got, SourceBundled)
	}
}

func TestDiscoveryMalformedManifestDoesNotAbortScan(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, store, homeDir, workspaceRoot, bundledRoot := newTestDiscovery(t, false)
	writeManifestJSON(t, filepath.Join(bundledRoot, "bundled-valid"), manifestFixture("bundled-valid"))
	writeManifestJSON(t, userExtensionDir(homeDir, "user-valid"), manifestFixture("user-valid"))
	enableUserExtension(t, store, "user-valid")
	writeTestFile(
		t,
		filepath.Join(workspaceExtensionDir(workspaceRoot, "broken-workspace"), ManifestFileNameTOML),
		`
[extension]
name = "broken"
version = "1.0.0"
description = "Broken workspace extension"
min_compozy_version = "1.0.0"

[security]
capabilities = ["prompt.not_real"]
`,
	)

	logBuf := captureDefaultLogger(t)

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Extensions) != 2 {
		t.Fatalf("len(Extensions) = %d, want 2", len(result.Extensions))
	}
	if len(result.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(result.Failures))
	}
	if got := result.Failures[0].Source; got != SourceWorkspace {
		t.Fatalf("Failures[0].Source = %q, want %q", got, SourceWorkspace)
	}

	var validationErr *ManifestValidationError
	if !errors.As(result.Failures[0], &validationErr) {
		t.Fatalf("Failures[0].Err = %T, want ManifestValidationError", result.Failures[0].Err)
	}

	records := decodeLogRecords(t, logBuf)
	if len(records) != 1 {
		t.Fatalf("len(log records) = %d, want 1", len(records))
	}
	if got := records[0]["msg"]; got != "extension discovery failed" {
		t.Fatalf("log message = %v, want extension discovery failed", got)
	}
}

func TestDiscoveryEnablementFilter(t *testing.T) {
	withVersion(t, "1.5.0")

	enabledOnly, _, homeDir, workspaceRoot, bundledRoot := newTestDiscovery(t, false)
	writeManifestJSON(t, filepath.Join(bundledRoot, "bundled-default"), manifestFixture("bundled-default"))
	writeManifestJSON(t, userExtensionDir(homeDir, "user-disabled"), manifestFixture("user-disabled"))
	writeManifestJSON(
		t,
		workspaceExtensionDir(workspaceRoot, "workspace-disabled"),
		manifestFixture("workspace-disabled"),
	)

	result, err := enabledOnly.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Discovered) != 3 {
		t.Fatalf("len(Discovered) = %d, want 3", len(result.Discovered))
	}
	if len(result.Extensions) != 1 {
		t.Fatalf("len(Extensions) = %d, want 1", len(result.Extensions))
	}
	if got := result.Extensions[0].Ref.Name; got != "bundled-default" {
		t.Fatalf("Extensions[0].Ref.Name = %q, want %q", got, "bundled-default")
	}

	allDiscovered := enabledOnly
	allDiscovered.IncludeDisabled = true
	allResult, err := allDiscovered.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover(include disabled) error = %v", err)
	}
	if len(allResult.Extensions) != 3 {
		t.Fatalf("len(Extensions with IncludeDisabled) = %d, want 3", len(allResult.Extensions))
	}

	states := make(map[string]bool, len(allResult.Extensions))
	for _, entry := range allResult.Extensions {
		states[entry.Ref.Name] = entry.Enabled
	}
	if !states["bundled-default"] {
		t.Fatal("bundled-default enabled = false, want true")
	}
	if states["user-disabled"] {
		t.Fatal("user-disabled enabled = true, want false")
	}
	if states["workspace-disabled"] {
		t.Fatal("workspace-disabled enabled = true, want false")
	}
}

func TestDiscoveryIntegrationThreeLevelFixture(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, store, homeDir, workspaceRoot, bundledRoot := newTestDiscovery(t, false)

	bundled := manifestFixture("bundled-only")
	bundled.Resources.Skills = []string{"skills/*"}
	bundled.Providers = ProvidersConfig{
		Model: []ProviderEntry{{Name: "bundled-model", Command: "bin/bundled-model"}},
	}
	writeManifestJSON(t, filepath.Join(bundledRoot, "bundled-only"), bundled)
	writeSkillPack(t, filepath.Join(bundledRoot, "bundled-only"), "skills", "bundled-pack")

	user := manifestFixture("shared")
	user.Resources.Skills = []string{"skills/*"}
	user.Providers = ProvidersConfig{
		Review: []ProviderEntry{{Name: "shared-review", Command: "bin/shared-review"}},
	}
	writeManifestJSON(t, userExtensionDir(homeDir, "shared"), user)
	writeSkillPack(t, userExtensionDir(homeDir, "shared"), "skills", "user-pack")
	enableUserExtension(t, store, "shared")

	workspace := manifestFixture("shared")
	workspace.Resources.Skills = []string{"skills/*"}
	workspace.Providers = ProvidersConfig{
		IDE: []ProviderEntry{{Name: "workspace-ide", Command: "bin/workspace-ide"}},
	}
	writeManifestJSON(t, workspaceExtensionDir(workspaceRoot, "shared-workspace"), workspace)
	writeSkillPack(t, workspaceExtensionDir(workspaceRoot, "shared-workspace"), "skills", "workspace-pack")
	enableWorkspaceExtension(t, store, workspaceRoot, "shared")

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}

	if len(result.Discovered) != 3 {
		t.Fatalf("len(Discovered) = %d, want 3", len(result.Discovered))
	}
	if len(result.Extensions) != 2 {
		t.Fatalf("len(Extensions) = %d, want 2", len(result.Extensions))
	}
	if len(result.Overrides) != 1 {
		t.Fatalf("len(Overrides) = %d, want 1", len(result.Overrides))
	}
	if got := result.Overrides[0].Winner.Source; got != SourceWorkspace {
		t.Fatalf("Overrides[0].Winner.Source = %q, want %q", got, SourceWorkspace)
	}
	if got := result.Overrides[0].Loser.Source; got != SourceUser {
		t.Fatalf("Overrides[0].Loser.Source = %q, want %q", got, SourceUser)
	}
	if len(result.SkillPacks.Packs) != 2 {
		t.Fatalf("len(SkillPacks.Packs) = %d, want 2", len(result.SkillPacks.Packs))
	}
	if filepath.Base(result.SkillPacks.Packs[0].ResolvedPath) != "bundled-pack" {
		t.Fatalf(
			"SkillPacks.Packs[0].ResolvedPath = %q, want bundled-pack first",
			result.SkillPacks.Packs[0].ResolvedPath,
		)
	}
	if filepath.Base(result.SkillPacks.Packs[1].ResolvedPath) != "workspace-pack" {
		t.Fatalf(
			"SkillPacks.Packs[1].ResolvedPath = %q, want workspace-pack second",
			result.SkillPacks.Packs[1].ResolvedPath,
		)
	}
	if len(result.Providers.IDE) != 1 {
		t.Fatalf("len(Providers.IDE) = %d, want 1", len(result.Providers.IDE))
	}
	if len(result.Providers.Model) != 1 {
		t.Fatalf("len(Providers.Model) = %d, want 1", len(result.Providers.Model))
	}
	if len(result.Providers.Review) != 0 {
		t.Fatalf(
			"len(Providers.Review) = %d, want 0 because workspace override suppresses user assets",
			len(result.Providers.Review),
		)
	}
}

func TestDiscoveryExtractsReusableAgentsFromEnabledExtensions(t *testing.T) {
	t.Run("Should extract reusable agents from enabled extensions", func(t *testing.T) {
		withVersion(t, "1.5.0")

		discovery, store, _, workspaceRoot, _ := newTestDiscovery(t, false)

		manifest := manifestFixture("agents-ext")
		manifest.Subprocess = nil
		manifest.Hooks = nil
		manifest.Providers = ProvidersConfig{}
		manifest.Resources.Skills = nil
		manifest.Security.Capabilities = []Capability{CapabilityAgentsShip}
		manifest.Resources.Agents = []string{"agents/*"}
		writeManifestJSON(t, workspaceExtensionDir(workspaceRoot, "agents-ext"), manifest)
		disabledManifest := manifestFixture("agents-disabled")
		disabledManifest.Subprocess = nil
		disabledManifest.Hooks = nil
		disabledManifest.Providers = ProvidersConfig{}
		disabledManifest.Resources.Skills = nil
		disabledManifest.Security.Capabilities = []Capability{CapabilityAgentsShip}
		disabledManifest.Resources.Agents = []string{"agents/*"}
		writeManifestJSON(t, workspaceExtensionDir(workspaceRoot, "agents-disabled"), disabledManifest)
		writeTestFile(
			t,
			filepath.Join(workspaceExtensionDir(workspaceRoot, "agents-ext"), "agents", "product-scout", "AGENT.md"),
			"---\ntitle: Product Scout\ndescription: Workspace reusable agent\n---\n",
		)
		writeTestFile(
			t,
			filepath.Join(
				workspaceExtensionDir(workspaceRoot, "agents-disabled"),
				"agents",
				"should-not-load",
				"AGENT.md",
			),
			"---\ntitle: Should Not Load\ndescription: Disabled reusable agent\n---\n",
		)
		enableWorkspaceExtension(t, store, workspaceRoot, "agents-ext")

		result, err := discovery.Discover(context.Background())
		if err != nil {
			t.Fatalf("Discover() error = %v", err)
		}
		if len(result.ReusableAgents.Agents) != 1 {
			t.Fatalf("len(ReusableAgents.Agents) = %d, want 1", len(result.ReusableAgents.Agents))
		}
		if got := result.ReusableAgents.Agents[0].Extension.Name; got != "agents-ext" {
			t.Fatalf("ReusableAgents.Agents[0].Extension.Name = %q, want %q", got, "agents-ext")
		}
		if got := filepath.Base(result.ReusableAgents.Agents[0].ResolvedPath); got != "product-scout" {
			t.Fatalf("ReusableAgents.Agents[0].ResolvedPath = %q, want product-scout", got)
		}
	})
}

func TestDiscoveryUsesDefaultStoreAndBundledFS(t *testing.T) {
	withVersion(t, "1.5.0")

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	store, err := NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	writeManifestJSON(t, userExtensionDir(homeDir, "user-enabled"), manifestFixture("user-enabled"))
	enableUserExtension(t, store, "user-enabled")

	discovery := Discovery{
		HomeDir:       homeDir,
		WorkspaceRoot: workspaceRoot,
	}

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Extensions) != 1 {
		t.Fatalf("len(Extensions) = %d, want 1", len(result.Extensions))
	}
	if got := result.Extensions[0].Ref.Source; got != SourceUser {
		t.Fatalf("Extensions[0].Ref.Source = %q, want %q", got, SourceUser)
	}
}

func TestDiscoveryBundledJSONAndManifestlessDirectories(t *testing.T) {
	withVersion(t, "1.5.0")

	discovery, _, _, _, bundledRoot := newTestDiscovery(t, true)
	writeManifestJSON(t, filepath.Join(bundledRoot, "json-bundled"), manifestFixture("json-bundled"))
	if err := os.MkdirAll(filepath.Join(bundledRoot, "missing-manifest"), 0o755); err != nil {
		t.Fatalf("MkdirAll(missing-manifest) error = %v", err)
	}
	writeTestFile(t, filepath.Join(bundledRoot, "broken-bundled", ManifestFileNameJSON), "{")

	logBuf := captureDefaultLogger(t)

	result, err := discovery.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(result.Discovered) != 1 {
		t.Fatalf("len(Discovered) = %d, want 1", len(result.Discovered))
	}
	if len(result.Failures) != 1 {
		t.Fatalf("len(Failures) = %d, want 1", len(result.Failures))
	}
	if got := result.Failures[0].ManifestPath; got != pathJoin("builtin", "broken-bundled", ManifestFileNameJSON) {
		t.Fatalf("Failures[0].ManifestPath = %q, want bundled broken json path", got)
	}

	records := decodeLogRecords(t, logBuf)
	if len(records) != 2 {
		t.Fatalf("len(log records) = %d, want 2", len(records))
	}
	messages := make(map[string]int, len(records))
	for _, record := range records {
		msg, _ := record["msg"].(string)
		messages[msg]++
	}
	if messages["ignore bundled extension directory without manifest"] != 1 {
		t.Fatalf("warning count = %d, want 1", messages["ignore bundled extension directory without manifest"])
	}
	if messages["extension discovery failed"] != 1 {
		t.Fatalf("failure count = %d, want 1", messages["extension discovery failed"])
	}
}

func TestDiscoveryFailureErrorIncludesContext(t *testing.T) {
	failure := DiscoveryFailure{
		Source:       SourceWorkspace,
		ExtensionDir: "/tmp/workspace/.compozy/extensions/broken",
		Err:          errors.New("boom"),
	}

	got := failure.Error()
	if !strings.Contains(got, `discover workspace extension`) {
		t.Fatalf("Error() = %q, want source context", got)
	}
	if !strings.Contains(got, `boom`) {
		t.Fatalf("Error() = %q, want wrapped error text", got)
	}
}

func newTestDiscovery(
	t *testing.T,
	includeDisabled bool,
) (Discovery, *EnablementStore, string, string, string) {
	t.Helper()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	bundledRoot := t.TempDir()

	store, err := NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("NewEnablementStore() error = %v", err)
	}

	return Discovery{
		WorkspaceRoot:   workspaceRoot,
		HomeDir:         homeDir,
		IncludeDisabled: includeDisabled,
		Enablement:      store,
		BundledFS:       os.DirFS(bundledRoot),
	}, store, homeDir, workspaceRoot, bundledRoot
}

func manifestFixture(name string) *Manifest {
	manifest := validManifest()
	manifest.Extension.Name = name
	manifest.Extension.Description = "Fixture " + name
	return manifest
}

func writeManifestJSON(t *testing.T, dir string, manifest *Manifest) {
	t.Helper()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent() error = %v", err)
	}
	writeTestFile(t, filepath.Join(dir, ManifestFileNameJSON), string(data))
}

func writeSkillPack(t *testing.T, extensionDir string, elems ...string) {
	t.Helper()

	skillDir := filepath.Join(append([]string{extensionDir}, elems...)...)
	writeTestFile(t, filepath.Join(skillDir, "SKILL.md"), "# Skill\n")
}

func userExtensionDir(homeDir, name string) string {
	return filepath.Join(homeDir, ".compozy", "extensions", name)
}

func workspaceExtensionDir(workspaceRoot, name string) string {
	return filepath.Join(workspaceRoot, ".compozy", "extensions", name)
}

func enableUserExtension(t *testing.T, store *EnablementStore, name string) {
	t.Helper()

	if err := store.Enable(context.Background(), Ref{Name: name, Source: SourceUser}); err != nil {
		t.Fatalf("Enable(user %q) error = %v", name, err)
	}
}

func enableWorkspaceExtension(t *testing.T, store *EnablementStore, workspaceRoot string, name string) {
	t.Helper()

	if err := store.Enable(context.Background(), Ref{
		Name:          name,
		Source:        SourceWorkspace,
		WorkspaceRoot: workspaceRoot,
	}); err != nil {
		t.Fatalf("Enable(workspace %q) error = %v", name, err)
	}
}

func pathJoin(elems ...string) string {
	return filepath.ToSlash(filepath.Join(elems...))
}
