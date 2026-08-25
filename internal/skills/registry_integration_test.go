//go:build integration

package skills

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRegistryIntegrationSourceIsolationAndRealpathDedup(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()
	agentsDir := filepath.Join(homeDir, ".agents", "skills")
	claudeDir := filepath.Join(homeDir, ".claude", "skills")
	agentsSkillDir := filepath.Join(agentsDir, "shared-global")
	writeSkillFile(
		t,
		agentsDir,
		filepath.Join("shared-global", skillFileName),
		skillWithDescription("shared-global", "Global agents skill"),
	)
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(claude root) error = %v", err)
	}
	if err := os.Symlink(agentsSkillDir, filepath.Join(claudeDir, "shared-global")); err != nil {
		t.Skipf("Symlink(vercel-labs alias) unavailable: %v", err)
	}

	sourceConfig := compozyconfig.SkillsConfig{
		Sources: []string{compozyconfig.SkillSourceAgents, compozyconfig.SkillSourceClaude},
	}
	registry := newTestRegistry(t, RegistryConfig{
		GlobalSkillRoots: compozyconfig.ResolveGlobalSkillRoots(&sourceConfig, compozyconfig.HomePaths{
			SkillsDir:       filepath.Join(homeDir, compozyconfig.DirName, compozyconfig.SkillsDirName),
			OperatorHomeDir: homeDir,
		}),
	})
	if err := registry.LoadAll(t.Context()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}
	global := registry.List()
	sharedGlobalCount := 0
	for _, skill := range global {
		if skill != nil && skill.Meta.Name == "shared-global" {
			sharedGlobalCount++
		}
	}
	if sharedGlobalCount != 1 {
		t.Fatalf("shared-global count = %d, want one realpath-deduplicated skill", sharedGlobalCount)
	}
	if got := findSkill(t, global, "shared-global").Origin; got != compozyconfig.SkillSourceAgents {
		t.Fatalf("shared-global origin = %q, want %q", got, compozyconfig.SkillSourceAgents)
	}

	workspaceARoot := t.TempDir()
	workspaceBRoot := t.TempDir()
	writeSkillFile(
		t,
		filepath.Join(workspaceARoot, ".agents", "skills"),
		filepath.Join("workspace-agents", skillFileName),
		skillWithDescription("workspace-agents", "Workspace A agents skill"),
	)
	writeSkillFile(
		t,
		filepath.Join(workspaceARoot, ".claude", "skills"),
		filepath.Join("workspace-claude", skillFileName),
		skillWithDescription("workspace-claude", "Workspace A claude skill"),
	)
	writeSkillFile(
		t,
		filepath.Join(workspaceBRoot, ".agents", "skills"),
		filepath.Join("workspace-agents", skillFileName),
		skillWithDescription("workspace-agents", "Workspace B agents skill"),
	)

	workspaceA := resolvedWorkspaceForTest("workspace-a", workspaceARoot)
	workspaceA.Config.Skills.Sources = []string{
		compozyconfig.SkillSourceAgents,
		compozyconfig.SkillSourceClaude,
	}
	workspaceB := resolvedWorkspaceForTest("workspace-b", workspaceBRoot)
	workspaceB.Config.Skills.Sources = []string{compozyconfig.SkillSourceAgents}

	beforeA, err := registry.ForWorkspace(t.Context(), &workspaceA)
	if err != nil {
		t.Fatalf("ForWorkspace(A before override) error = %v", err)
	}
	beforeB, err := registry.ForWorkspace(t.Context(), &workspaceB)
	if err != nil {
		t.Fatalf("ForWorkspace(B before override) error = %v", err)
	}
	if got := findSkill(t, beforeA, "workspace-claude").Origin; got != compozyconfig.SkillSourceClaude {
		t.Fatalf("workspace A claude origin = %q, want %q", got, compozyconfig.SkillSourceClaude)
	}
	if hasSkillNamed(beforeB, "workspace-claude") {
		t.Fatal("workspace B contains workspace A claude skill, want isolated effective roots")
	}
	if got := findSkill(t, beforeB, "workspace-agents").Meta.Description; got != "Workspace B agents skill" {
		t.Fatalf("workspace B agents skill = %q, want isolated workspace B contribution", got)
	}

	cacheA := cacheEntryForWorkspace(t, registry, &workspaceA)
	cacheB := cacheEntryForWorkspace(t, registry, &workspaceB)
	workspaceA.Config.Skills.Sources = []string{compozyconfig.SkillSourceAgents}
	afterA, err := registry.ForWorkspace(t.Context(), &workspaceA)
	if err != nil {
		t.Fatalf("ForWorkspace(A after override) error = %v", err)
	}
	if hasSkillNamed(afterA, "workspace-claude") {
		t.Fatal("workspace A contains removed claude source after generation change")
	}
	if got := cacheEntryForWorkspace(t, registry, &workspaceA); got == nil || got == cacheA {
		t.Fatalf("workspace A cache entry = %p, want a new source-generation entry distinct from %p", got, cacheA)
	}

	afterB, err := registry.ForWorkspace(t.Context(), &workspaceB)
	if err != nil {
		t.Fatalf("ForWorkspace(B after A override) error = %v", err)
	}
	if got := cacheEntryForWorkspace(t, registry, &workspaceB); got != cacheB {
		t.Fatalf("workspace B cache entry = %p, want untouched entry %p", got, cacheB)
	}
	if got := findSkill(t, afterB, "workspace-agents").Meta.Description; got != "Workspace B agents skill" {
		t.Fatalf("workspace B agents skill after A override = %q, want unchanged projection", got)
	}
}

// Invariant: a Compozy canonical skill exposed into a provider root resolves as
// one Compozy-owned catalog entry and never creates a self-shadow diagnostic.
// Owning layer: registry realpath dedup. Canonical suite: registry_integration_test.go.
func TestRegistryIntegrationOwnExposureDoesNotSelfShadow(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve an owned provider link as one canonical Compozy skill", func(t *testing.T) {
		t.Parallel()
		homeDir := t.TempDir()
		canonicalRoot := filepath.Join(homeDir, compozyconfig.DirName, compozyconfig.SkillsDirName)
		canonicalDir := filepath.Join(canonicalRoot, "review")
		writeSkillFile(
			t,
			canonicalRoot,
			filepath.Join("review", skillFileName),
			skillWithDescription("review", "Canonical Compozy skill"),
		)
		agentsRoot := filepath.Join(homeDir, ".agents", "skills")
		if err := os.MkdirAll(agentsRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll(agents root) error = %v", err)
		}
		if err := os.Symlink(canonicalDir, filepath.Join(agentsRoot, "review")); err != nil {
			t.Skipf("Symlink(exposure) unavailable: %v", err)
		}
		config := compozyconfig.SkillsConfig{Sources: []string{compozyconfig.SkillSourceAgents}}
		registry := newTestRegistry(t, RegistryConfig{
			GlobalSkillRoots: compozyconfig.ResolveGlobalSkillRoots(&config, compozyconfig.HomePaths{
				SkillsDir: canonicalRoot, OperatorHomeDir: homeDir,
			}),
		})
		if err := registry.LoadAll(t.Context()); err != nil {
			t.Fatalf("LoadAll() error = %v", err)
		}
		count := 0
		for _, skill := range registry.List() {
			if skill != nil && skill.Meta.Name == "review" {
				count++
				if skill.Origin != "" {
					t.Fatalf("review origin = %q, want Compozy canonical origin", skill.Origin)
				}
				if len(skill.Diagnostics.ShadowedDefinitions) != 0 {
					t.Fatalf("review self-shadow diagnostics = %#v", skill.Diagnostics.ShadowedDefinitions)
				}
			}
		}
		if count != 1 {
			t.Fatalf("review count = %d, want 1", count)
		}
	})
}

func TestRegistryIntegrationRefreshPromotesSidecarBackedSkillToMarketplace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	content := skillWithDescription("installed", "Installed from marketplace")
	skillPath := writeSkillFile(t, userDir, filepath.Join("installed", skillFileName), content)

	registry := newTestRegistry(t, RegistryConfig{
		GlobalSkillRoots: testGlobalSkillRoots(userDir),
	})

	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	skill, ok := registry.Get("installed")
	if !ok {
		t.Fatal("Get(installed) ok = false, want initial user skill")
	}
	if skill.Source != SourceUser {
		t.Fatalf("initial Source = %v, want %v", skill.Source, SourceUser)
	}

	if err := WriteSidecar(filepath.Dir(skillPath), Provenance{
		Hash:        mustComputeDirectoryHash(t, filepath.Dir(skillPath)),
		Registry:    "clawhub",
		Slug:        "@author/installed",
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 4, 7, 14, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	if err := registry.RefreshGlobal(context.Background()); err != nil {
		t.Fatalf("RefreshGlobal() error = %v", err)
	}

	skill, ok = registry.Get("installed")
	if !ok {
		t.Fatal("Get(installed) ok = false after refresh, want marketplace skill")
	}
	if skill.Source != SourceMarketplace {
		t.Fatalf("refreshed Source = %v, want %v", skill.Source, SourceMarketplace)
	}
	if skill.InstalledFrom != "@author/installed" {
		t.Fatalf("InstalledFrom = %q, want %q", skill.InstalledFrom, "@author/installed")
	}
	if skill.Provenance == nil || skill.Provenance.Slug != "@author/installed" {
		t.Fatalf("Provenance = %#v, want loaded sidecar provenance", skill.Provenance)
	}
}

func TestRegistryIntegrationRefreshBlocksTamperedMarketplaceSkill(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	userDir := filepath.Join(root, "user")
	original := skillWithDescription("tampered-reload", "Original marketplace skill")
	tampered := skillWithDescription("tampered-reload", "Tampered marketplace skill")
	skillPath := writeSkillFile(t, userDir, filepath.Join("tampered-reload", skillFileName), original)
	originalHash := mustComputeDirectoryHash(t, filepath.Dir(skillPath))
	if err := WriteSidecar(filepath.Dir(skillPath), Provenance{
		Hash:        originalHash,
		Registry:    "clawhub",
		Slug:        "@author/tampered-reload",
		Version:     "1.0.0",
		InstalledAt: time.Date(2026, 4, 7, 14, 30, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("WriteSidecar() error = %v", err)
	}

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	registry := newTestRegistry(t, RegistryConfig{
		GlobalSkillRoots: testGlobalSkillRoots(userDir),
	}, WithLogger(logger))

	if err := registry.LoadAll(context.Background()); err != nil {
		t.Fatalf("LoadAll() error = %v", err)
	}

	rewriteSkillFile(t, skillPath, tampered)
	actualHash := mustComputeDirectoryHash(t, filepath.Dir(skillPath))

	if err := registry.RefreshGlobal(context.Background()); err != nil {
		t.Fatalf("RefreshGlobal() error = %v", err)
	}

	if _, ok := registry.Get("tampered-reload"); ok {
		t.Fatal("Get(tampered-reload) ok = true after tamper refresh, want marketplace skill blocked")
	}

	output := logs.String()
	if !strings.Contains(output, "marketplace skill hash mismatch") {
		t.Fatalf("logs = %q, want hash mismatch warning", output)
	}
	if !strings.Contains(output, "skill_name=tampered-reload") {
		t.Fatalf("logs = %q, want skill_name field", output)
	}
	if !strings.Contains(output, "expected_hash="+originalHash) {
		t.Fatalf("logs = %q, want expected hash", output)
	}
	if !strings.Contains(output, "actual_hash="+actualHash) {
		t.Fatalf("logs = %q, want actual hash", output)
	}
}
