// Suite: extension skill packaging conventions
// Invariant: setup installs the declared skills and their shipped references retain required security defaults.
// Boundary IN: real skill directories and content, setup installation, and setup verification in temporary projects.
// Boundary OUT: extension enablement and CLI dispatch, owned by CLI and workflow E2E suites.
package packaging

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/setup"
)

func TestInstallExtensionSkillPacksLandsAllSkills(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	packs := extensionSkillPacks(t)

	result := installExtensionSkillPacks(t, projectDir, homeDir, packs)
	requireSuccessfulSkillCount(t, result, len(expectedSkillNames))
	requireInstalledSkillDirectories(t, projectDir)
}

func TestInstallExtensionSkillPacksIsIdempotent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	packs := extensionSkillPacks(t)

	firstResult := installExtensionSkillPacks(t, projectDir, homeDir, packs)
	requireSuccessfulSkillCount(t, firstResult, len(expectedSkillNames))
	secondResult := installExtensionSkillPacks(t, projectDir, homeDir, packs)
	requireSuccessfulSkillCount(t, secondResult, len(expectedSkillNames))
	requireInstalledSkillDirectories(t, projectDir)

	verification, err := setup.VerifyExtensionSkillPacks(setup.ExtensionVerifyConfig{
		ResolverOptions: setup.ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:     packs,
		AgentName: "codex",
		ScopeHint: setup.InstallScopeProject,
	})
	if err != nil {
		t.Fatalf("verify extension skill packs: %v", err)
	}
	if verification.HasMissing() {
		t.Fatalf("installed skills are missing: %#v", verification.MissingSkillNames())
	}
	if verification.HasDrift() {
		t.Fatalf("installed skills have drift: %#v", verification.DriftedSkillNames())
	}
	if len(verification.Skills) != len(expectedSkillNames) {
		t.Fatalf("unexpected verified skill count: got %d want %d", len(verification.Skills), len(expectedSkillNames))
	}
	for _, skill := range verification.Skills {
		if skill.State != setup.VerifyStateCurrent {
			t.Fatalf("skill %q is not current: %q", skill.Skill.Name, skill.State)
		}
	}
}

func TestHTMLReportScaffoldUsesStrictMermaidSecurity(t *testing.T) {
	t.Parallel()

	reportPath := filepath.Join(
		extensionRoot(t),
		"skills",
		"cy-improve-architecture",
		"references",
		"html-report.md",
	)
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read HTML report reference: %v", err)
	}

	content := string(report)
	if !strings.Contains(content, `securityLevel: "strict"`) {
		t.Fatal("HTML report scaffold does not require Mermaid strict security")
	}
	if strings.Contains(content, `securityLevel: "loose"`) {
		t.Fatal("HTML report scaffold permits Mermaid loose security")
	}
}

func extensionSkillPacks(t *testing.T) []setup.SkillPackSource {
	t.Helper()

	root := extensionRoot(t)
	manifestPath := filepath.Join(root, "extension.toml")
	skills := make([]setup.Skill, 0, len(expectedSkillNames))
	for _, skillName := range expectedSkillNames {
		skills = append(skills, setup.Skill{
			Name:            skillName,
			Origin:          setup.AssetOriginExtension,
			ExtensionName:   "cy-improve-architecture",
			ExtensionSource: "bundled",
			ManifestPath:    manifestPath,
			ResolvedPath:    filepath.Join(root, "skills", skillName),
		})
	}

	packs := setup.ExtensionSkillPackSources(skills)
	if len(packs) != len(expectedSkillNames) {
		t.Fatalf("unexpected extension skill pack count: got %d want %d", len(packs), len(expectedSkillNames))
	}
	return packs
}

func installExtensionSkillPacks(
	t *testing.T,
	projectDir string,
	homeDir string,
	packs []setup.SkillPackSource,
) *setup.ExtensionResult {
	t.Helper()

	result, err := setup.InstallExtensionSkillPacks(setup.ExtensionInstallConfig{
		ResolverOptions: setup.ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:      packs,
		AgentNames: []string{"codex"},
		Mode:       setup.InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install extension skill packs: %v", err)
	}
	return result
}

func requireSuccessfulSkillCount(t *testing.T, result *setup.ExtensionResult, want int) {
	t.Helper()

	if len(result.Failed) != 0 {
		t.Fatalf("extension skill installation failures: %#v", result.Failed)
	}
	if len(result.Successful) != want {
		t.Fatalf("unexpected installed skill count: got %d want %d", len(result.Successful), want)
	}
}

func requireInstalledSkillDirectories(t *testing.T, projectDir string) {
	t.Helper()

	installedRoot := filepath.Join(projectDir, ".agents", "skills")
	entries, err := os.ReadDir(installedRoot)
	if err != nil {
		t.Fatalf("read installed skills directory: %v", err)
	}

	actualNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			actualNames = append(actualNames, entry.Name())
		}
	}
	if !slices.Equal(actualNames, expectedSkillNames) {
		t.Fatalf("unexpected installed skill directories: got %#v want %#v", actualNames, expectedSkillNames)
	}

	for _, skillName := range expectedSkillNames {
		skillPath := filepath.Join(installedRoot, skillName, "SKILL.md")
		info, statErr := os.Stat(skillPath)
		if statErr != nil {
			t.Fatalf("stat installed skill %q: %v", skillPath, statErr)
		}
		if info.IsDir() {
			t.Fatalf("installed SKILL.md is a directory: %q", skillPath)
		}
	}
}
