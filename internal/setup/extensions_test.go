package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestInstallExtensionSkillPacksCopiesDeclaredSkillIntoAgentDirectory(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	skillDir := writeExtensionSkillPack(t, t.TempDir(), "ext-review-skill", map[string]string{
		"references/notes.md": "# Notes\n",
	})

	result, err := InstallExtensionSkillPacks(ExtensionInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs: []SkillPackSource{
			{
				ExtensionName: "workspace-ext",
				ManifestPath:  filepath.Join(projectDir, ".compozy", "extensions", "workspace-ext", "extension.json"),
				ResolvedPath:  skillDir,
			},
		},
		AgentNames: []string{"codex"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install extension skill packs: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", result.Failed)
	}

	targetDir := filepath.Join(projectDir, ".agents", "skills", "ext-review-skill")
	assertFileExists(t, filepath.Join(targetDir, "SKILL.md"))
	assertFileExists(t, filepath.Join(targetDir, "references", "notes.md"))
}

func TestVerifyExtensionSkillPacksReportsDriftWhenInstalledContentChanges(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	skillDir := writeExtensionSkillPack(t, t.TempDir(), "ext-review-skill", nil)
	packs := []SkillPackSource{
		{
			ExtensionName: "workspace-ext",
			ManifestPath:  filepath.Join(projectDir, ".compozy", "extensions", "workspace-ext", "extension.json"),
			ResolvedPath:  skillDir,
		},
	}

	if _, err := InstallExtensionSkillPacks(ExtensionInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:      packs,
		AgentNames: []string{"codex"},
		Mode:       InstallModeCopy,
	}); err != nil {
		t.Fatalf("install extension skill packs: %v", err)
	}

	installedSkillPath := filepath.Join(projectDir, ".agents", "skills", "ext-review-skill", "SKILL.md")
	if err := os.WriteFile(
		installedSkillPath,
		[]byte("---\nname: ext-review-skill\ndescription: changed\n---\n"),
		0o644,
	); err != nil {
		t.Fatalf("write drifted installed skill: %v", err)
	}

	result, err := VerifyExtensionSkillPacks(ExtensionVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:     packs,
		AgentName: "codex",
	})
	if err != nil {
		t.Fatalf("verify extension skill packs: %v", err)
	}
	if !result.HasDrift() {
		t.Fatal("expected extension skill drift")
	}
	if got := result.DriftedSkillNames(); len(got) != 1 || got[0] != "ext-review-skill" {
		t.Fatalf("unexpected drifted skills: %#v", got)
	}
}

func TestVerifyExtensionSkillPacksDetectsCurrentGlobalInstallFromProvidedSourceFS(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	packs := []SkillPackSource{
		{
			ExtensionName: "workspace-ext",
			ManifestPath:  filepath.Join(projectDir, ".compozy", "extensions", "workspace-ext", "extension.json"),
			SourceFS: fstest.MapFS{
				"ext-pack/SKILL.md":        {Data: []byte("---\nname: ext-pack\ndescription: Fixture skill\n---\n")},
				"ext-pack/references/a.md": {Data: []byte("# Notes\n")},
			},
			SourceDir: "ext-pack",
		},
	}

	if _, err := InstallExtensionSkillPacks(ExtensionInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:      packs,
		AgentNames: []string{"codex"},
		Global:     true,
	}); err != nil {
		t.Fatalf("install global extension skill packs: %v", err)
	}

	result, err := VerifyExtensionSkillPacks(ExtensionVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		Packs:     packs,
		AgentName: "codex",
	})
	if err != nil {
		t.Fatalf("verify extension skill packs: %v", err)
	}

	if result.Scope != InstallScopeGlobal {
		t.Fatalf("expected global scope, got %q", result.Scope)
	}
	if result.Mode != InstallModeSymlink {
		t.Fatalf("expected symlink mode, got %q", result.Mode)
	}
	if result.HasMissing() {
		t.Fatalf("expected no missing skills, got %#v", result.MissingSkillNames())
	}
	if result.HasDrift() {
		t.Fatalf("expected no drift, got %#v", result.DriftedSkillNames())
	}
	if len(result.Skills) != 1 || result.Skills[0].State != VerifyStateCurrent {
		t.Fatalf("expected one current skill result, got %#v", result.Skills)
	}
}

func TestExtensionVerificationHelpersSelectScopeAndSortSources(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	globalDir := t.TempDir()
	projectSkillPath := filepath.Join(projectDir, "ext-pack", "SKILL.md")
	writeSkillTestFile(t, projectSkillPath, "---\nname: ext-pack\ndescription: Fixture\n---\n")
	globalSkillPath := filepath.Join(globalDir, "ext-pack", "SKILL.md")
	writeSkillTestFile(t, globalSkillPath, "---\nname: ext-pack\ndescription: Fixture\n---\n")

	projectEntries := []extensionVerificationEntry{{TargetPath: projectSkillPath}}
	globalEntries := []extensionVerificationEntry{{TargetPath: globalSkillPath}}

	tests := []struct {
		name      string
		scopeHint InstallScope
		project   []extensionVerificationEntry
		global    []extensionVerificationEntry
		wantScope InstallScope
	}{
		{
			name:      "project scope hint wins",
			scopeHint: InstallScopeProject,
			project:   projectEntries,
			global:    globalEntries,
			wantScope: InstallScopeProject,
		},
		{
			name:      "global scope hint wins",
			scopeHint: InstallScopeGlobal,
			project:   projectEntries,
			global:    globalEntries,
			wantScope: InstallScopeGlobal,
		},
		{
			name:      "project install wins without hint",
			project:   projectEntries,
			global:    globalEntries,
			wantScope: InstallScopeProject,
		},
		{
			name:      "global install wins when project absent",
			project:   nil,
			global:    globalEntries,
			wantScope: InstallScopeGlobal,
		},
		{
			name:      "unknown scope when nothing installed",
			project:   []extensionVerificationEntry{{TargetPath: filepath.Join(projectDir, "missing", "SKILL.md")}},
			global:    []extensionVerificationEntry{{TargetPath: filepath.Join(globalDir, "missing", "SKILL.md")}},
			wantScope: InstallScopeUnknown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotScope, _ := selectExtensionVerificationEntries(tc.project, tc.global, tc.scopeHint)
			if gotScope != tc.wantScope {
				t.Fatalf("unexpected scope: got %q want %q", gotScope, tc.wantScope)
			}
		})
	}

	alphaPath := writeExtensionSkillPack(t, filepath.Join(projectDir, "skills"), "alpha-pack", nil)
	betaPath := writeExtensionSkillPack(t, filepath.Join(projectDir, "skills"), "beta-pack", nil)
	sources, err := loadExtensionSkillSources([]SkillPackSource{
		{
			ExtensionName: "zeta",
			ManifestPath:  filepath.Join(projectDir, "zeta.json"),
			ResolvedPath:  betaPath,
		},
		{
			ExtensionName: "alpha",
			ManifestPath:  filepath.Join(projectDir, "alpha.json"),
			ResolvedPath:  alphaPath,
		},
	})
	if err != nil {
		t.Fatalf("load extension skill sources: %v", err)
	}

	if got := []string{sources[0].Skill.Name, sources[1].Skill.Name}; got[0] != "alpha-pack" || got[1] != "beta-pack" {
		t.Fatalf("unexpected source order: %#v", got)
	}
}

func TestExtensionVerifyResultHelpersAndSkillSourceOrdering(t *testing.T) {
	t.Parallel()

	result := ExtensionVerifyResult{
		Skills: []ExtensionVerifiedSkill{
			{VerifiedSkill: VerifiedSkill{Skill: Skill{Name: "zeta"}, State: VerifyStateCurrent}},
			{VerifiedSkill: VerifiedSkill{Skill: Skill{Name: "beta"}, State: VerifyStateMissing}},
			{VerifiedSkill: VerifiedSkill{Skill: Skill{Name: "alpha"}, State: VerifyStateMissing}},
		},
	}

	if got := result.MissingSkillNames(); len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("unexpected missing skill names: %#v", got)
	}
	if !result.HasMissing() {
		t.Fatal("expected missing extension skills")
	}

	left := extensionSkillSource{
		Pack: SkillPackSource{
			ExtensionName: "alpha",
			ManifestPath:  "/tmp/alpha.json",
			ResolvedPath:  "/tmp/alpha-pack",
		},
		Skill: Skill{Name: "alpha"},
	}
	right := extensionSkillSource{
		Pack: SkillPackSource{
			ExtensionName: "beta",
			ManifestPath:  "/tmp/beta.json",
			ResolvedPath:  "/tmp/beta-pack",
		},
		Skill: Skill{Name: "beta"},
	}
	if got := compareExtensionSkillSource(left, right); got >= 0 {
		t.Fatalf("expected alpha to sort before beta, got %d", got)
	}
	if got := compareExtensionSkillSource(right, left); got <= 0 {
		t.Fatalf("expected beta to sort after alpha, got %d", got)
	}
}

func TestVerifyExtensionSkillPacksReturnsErrorForMissingPackSourcePath(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing-pack")
	_, err := VerifyExtensionSkillPacks(ExtensionVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     t.TempDir(),
			HomeDir: t.TempDir(),
		},
		Packs: []SkillPackSource{
			{
				ExtensionName: "workspace-ext",
				ManifestPath:  "/tmp/workspace-ext/extension.json",
				ResolvedPath:  missingPath,
			},
		},
		AgentName: "codex",
	})
	if err == nil || !strings.Contains(err.Error(), "stat extension skill pack") {
		t.Fatalf("expected missing source path stat error, got %v", err)
	}
}

func writeExtensionSkillPack(t *testing.T, root string, name string, files map[string]string) string {
	t.Helper()

	skillDir := filepath.Join(root, name)
	writeSkillTestFile(t, filepath.Join(skillDir, "SKILL.md"), "---\nname: "+name+"\ndescription: Fixture skill\n---\n")
	for relativePath, content := range files {
		writeSkillTestFile(t, filepath.Join(skillDir, filepath.FromSlash(relativePath)), content)
	}
	return skillDir
}

func writeSkillTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
