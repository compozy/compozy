package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupLegacyTransferredAssetsRemovesProjectScopeInstalls(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()

	legacyPaths := []string{
		filepath.Join(projectDir, ".agents", "skills", "cy-idea-factory", "SKILL.md"),
		filepath.Join(projectDir, ".claude", "skills", "cy-idea-factory", "SKILL.md"),
		filepath.Join(projectDir, ".compozy", "agents", "architect-advisor", "AGENT.md"),
	}
	for i := range legacyPaths {
		writeLegacyCleanupFixture(t, legacyPaths[i])
	}

	unrelatedPaths := []string{
		filepath.Join(projectDir, ".agents", "skills", "cy-create-prd", "SKILL.md"),
		filepath.Join(projectDir, ".compozy", "agents", "custom-agent", "AGENT.md"),
	}
	for i := range unrelatedPaths {
		writeLegacyCleanupFixture(t, unrelatedPaths[i])
	}

	result, err := CleanupLegacyTransferredAssets(LegacyAssetCleanupConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Global: false,
	})
	if err != nil {
		t.Fatalf("CleanupLegacyTransferredAssets() error = %v", err)
	}

	if len(result.Removed) != 3 {
		t.Fatalf("expected 3 removed legacy paths, got %#v", result.Removed)
	}

	for i := range legacyPaths {
		assertPathMissing(t, filepath.Dir(legacyPaths[i]))
	}
	for i := range unrelatedPaths {
		assertFileExists(t, unrelatedPaths[i])
	}
}

func TestCleanupLegacyTransferredAssetsRemovesGlobalScopeInstalls(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()

	legacyPaths := []string{
		filepath.Join(homeDir, ".agents", "skills", "cy-idea-factory", "SKILL.md"),
		filepath.Join(homeDir, ".claude", "skills", "cy-idea-factory", "SKILL.md"),
		filepath.Join(homeDir, ".compozy", "agents", "product-mind", "AGENT.md"),
	}
	for i := range legacyPaths {
		writeLegacyCleanupFixture(t, legacyPaths[i])
	}

	result, err := CleanupLegacyTransferredAssets(LegacyAssetCleanupConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Global: true,
	})
	if err != nil {
		t.Fatalf("CleanupLegacyTransferredAssets() error = %v", err)
	}

	if len(result.Removed) != 3 {
		t.Fatalf("expected 3 removed legacy paths, got %#v", result.Removed)
	}

	for i := range legacyPaths {
		assertPathMissing(t, filepath.Dir(legacyPaths[i]))
	}
}

func TestLegacyTransferredAssetRemovalsSkipsAgentsThatDoNotSupportScope(t *testing.T) {
	t.Parallel()

	env := resolvedEnvironment{
		cwd:     t.TempDir(),
		homeDir: t.TempDir(),
	}

	removals, err := legacyTransferredAssetRemovals(env, []Agent{
		{
			Name:           "project-only",
			DisplayName:    "Project Only",
			ProjectRootDir: ".project/skills",
		},
		{
			Name:           "claude-code",
			DisplayName:    "Claude Code",
			ProjectRootDir: ".claude/skills",
			GlobalRootDir:  filepath.Join(env.homeDir, ".claude", "skills"),
		},
	}, true)
	if err != nil {
		t.Fatalf("legacyTransferredAssetRemovals() error = %v", err)
	}

	want := 2 + len(legacyTransferredReusableAgentNames)
	if len(removals) != want {
		t.Fatalf("expected %d removals after skipping unsupported agent, got %#v", want, removals)
	}
}

func writeLegacyCleanupFixture(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("legacy"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err = %v", path, err)
	}
}
