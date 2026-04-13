package setup

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestSelectAgentsAcceptsClaudeAlias(t *testing.T) {
	t.Parallel()

	agents, err := SupportedAgents(ResolverOptions{
		CWD:     t.TempDir(),
		HomeDir: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("supported agents: %v", err)
	}

	selected, err := SelectAgents(agents, []string{"claude"})
	if err != nil {
		t.Fatalf("select agents: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("expected 1 selected agent, got %d", len(selected))
	}
	if selected[0].Name != "claude-code" {
		t.Fatalf("expected claude alias to resolve to claude-code, got %q", selected[0].Name)
	}
}

func TestInstallCopyModeCopiesBundledSkillIntoAgentDirectory(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md":               "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
		"cy-create-prd/references/template.md": "# Template\n",
		"cy-create-tasks/SKILL.md":             "---\nname: cy-create-tasks\ndescription: Create tasks\n---\n",
		"cy-create-tasks/references/tasks.md":  "# Tasks\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	result, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install copy mode: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", result.Failed)
	}

	skillDir := filepath.Join(projectDir, ".claude", "skills", "cy-create-prd")
	assertFileExists(t, filepath.Join(skillDir, "SKILL.md"))
	assertFileExists(t, filepath.Join(skillDir, "references", "template.md"))
}

func TestInstallSymlinkModeUsesCanonicalDirForUniversalProjectAgent(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md":               "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
		"cy-create-prd/references/template.md": "# Template\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	result, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"codex"},
		Mode:       InstallModeSymlink,
	})
	if err != nil {
		t.Fatalf("install symlink mode: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", result.Failed)
	}
	if len(result.Successful) != 1 {
		t.Fatalf("expected 1 success, got %d", len(result.Successful))
	}

	skillDir := filepath.Join(projectDir, ".agents", "skills", "cy-create-prd")
	info, err := os.Lstat(skillDir)
	if err != nil {
		t.Fatalf("lstat skill dir: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("expected canonical project install to be a directory, got symlink")
	}
	assertFileExists(t, filepath.Join(skillDir, "SKILL.md"))
	assertFileExists(t, filepath.Join(skillDir, "references", "template.md"))
}

func TestPreviewGlobalUniversalAgentUsesCanonicalHomeAgentsDir(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md": "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	items, err := Preview(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"codex"},
		Global:     true,
		Mode:       InstallModeSymlink,
	})
	if err != nil {
		t.Fatalf("preview global install: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 preview item, got %d", len(items))
	}

	want := filepath.Join(homeDir, ".agents", "skills", "cy-create-prd")
	if items[0].CanonicalPath != want {
		t.Fatalf("unexpected canonical path\nwant: %s\ngot:  %s", want, items[0].CanonicalPath)
	}
	if items[0].TargetPath != want {
		t.Fatalf("unexpected target path\nwant: %s\ngot:  %s", want, items[0].TargetPath)
	}
}

func TestReusableAgentFlowsRespectSelectedScope(t *testing.T) {
	t.Parallel()

	reusableAgents := testExtensionReusableAgents(t, "architect-advisor")

	tests := []struct {
		name     string
		global   bool
		wantRoot func(projectDir, homeDir string) string
	}{
		{
			name:   "Project scope",
			global: false,
			wantRoot: func(projectDir, _ string) string {
				return filepath.Join(projectDir, ".compozy", "agents")
			},
		},
		{
			name:   "Global scope",
			global: true,
			wantRoot: func(_ string, homeDir string) string {
				return filepath.Join(homeDir, ".compozy", "agents")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectDir := t.TempDir()
			homeDir := t.TempDir()
			wantRoot := tt.wantRoot(projectDir, homeDir)

			items, err := PreviewReusableAgentInstall(ReusableAgentInstallConfig{
				ResolverOptions: ResolverOptions{
					CWD:     projectDir,
					HomeDir: homeDir,
				},
				ReusableAgents: reusableAgents,
				Global:         tt.global,
			})
			if err != nil {
				t.Fatalf("preview reusable agents: %v", err)
			}
			if len(items) != len(reusableAgents) {
				t.Fatalf("expected %d reusable-agent preview items, got %d", len(reusableAgents), len(items))
			}

			for _, item := range items {
				want := filepath.Join(wantRoot, item.ReusableAgent.Name)
				if item.TargetPath != want {
					t.Fatalf(
						"unexpected reusable-agent target path for %q\nwant: %s\ngot:  %s",
						item.ReusableAgent.Name,
						want,
						item.TargetPath,
					)
				}
			}

			successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
				ResolverOptions: ResolverOptions{
					CWD:     projectDir,
					HomeDir: homeDir,
				},
				ReusableAgents: reusableAgents,
				Global:         tt.global,
			})
			if err != nil {
				t.Fatalf("install reusable agents: %v", err)
			}
			if len(failures) != 0 {
				t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
			}
			if len(successes) != len(reusableAgents) {
				t.Fatalf(
					"expected %d reusable-agent installation successes, got %d",
					len(reusableAgents),
					len(successes),
				)
			}

			for _, success := range successes {
				agentDir := filepath.Join(wantRoot, success.ReusableAgent.Name)
				if success.Path != agentDir {
					t.Fatalf(
						"unexpected success path for %q\nwant: %s\ngot:  %s",
						success.ReusableAgent.Name,
						agentDir,
						success.Path,
					)
				}
				assertFileExists(t, filepath.Join(agentDir, "AGENT.md"))
			}
		})
	}
}

func TestInstallBundledSetupAssetsReturnsSkillResultsWhenReusableAgentInstallFails(t *testing.T) {
	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md": "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	previous := installBundledReusableAgents
	installBundledReusableAgents = func(
		ReusableAgentInstallConfig,
	) ([]ReusableAgentSuccessItem, []ReusableAgentFailureItem, error) {
		return nil, nil, errors.New("reusable agents unavailable")
	}
	t.Cleanup(func() {
		installBundledReusableAgents = previous
	})

	result, err := InstallBundledSetupAssets(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"codex"},
		Mode:       InstallModeCopy,
	})
	if err == nil {
		t.Fatal("expected reusable-agent install error")
	}
	if result == nil {
		t.Fatal("expected partial skill install result")
	}
	if len(result.Successful) != 1 {
		t.Fatalf("expected one successful skill install, got %#v", result.Successful)
	}
	if len(result.ReusableAgentsSuccessful) != 0 || len(result.ReusableAgentsFailed) != 0 {
		t.Fatalf("expected reusable-agent result slices to remain empty on phase-two error, got %#v", result)
	}
}

func TestInstallReusableAgentsPreservesExistingInstallWhenCopyFails(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	existingAgentDir := filepath.Join(homeDir, ".compozy", "agents", "architect-advisor")
	if err := os.MkdirAll(existingAgentDir, 0o755); err != nil {
		t.Fatalf("mkdir existing reusable agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(existingAgentDir, "AGENT.md"), []byte("existing"), 0o600); err != nil {
		t.Fatalf("write existing reusable agent: %v", err)
	}

	previous := copyReusableAgentBundleDirectory
	copyReusableAgentBundleDirectory = func(bundle fs.FS, rootDir, dest, subject string) error {
		if rootDir != "architect-advisor" {
			return previous(bundle, rootDir, dest, subject)
		}
		if err := os.WriteFile(filepath.Join(dest, "AGENT.md"), []byte("partial"), 0o600); err != nil {
			t.Fatalf("write staged reusable agent file: %v", err)
		}
		return errors.New("copy reusable agent failed")
	}
	t.Cleanup(func() {
		copyReusableAgentBundleDirectory = previous
	})

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: testExtensionReusableAgents(t, "architect-advisor", "product-mind"),
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install reusable agents: %v", err)
	}
	if len(successes) == 0 {
		t.Fatalf("expected unaffected reusable agents to keep installing, got %#v", successes)
	}
	if len(failures) == 0 {
		t.Fatal("expected one reusable-agent installation failure")
	}

	content, err := os.ReadFile(filepath.Join(existingAgentDir, "AGENT.md"))
	if err != nil {
		t.Fatalf("read preserved reusable agent install: %v", err)
	}
	if string(content) != "existing" {
		t.Fatalf("expected existing install to remain intact, got %q", content)
	}
	stagedPaths, err := filepath.Glob(filepath.Join(homeDir, ".compozy", "agents", "architect-advisor.tmp-*"))
	if err != nil {
		t.Fatalf("glob staged reusable agent dirs: %v", err)
	}
	if len(stagedPaths) != 0 {
		t.Fatalf("expected failed staged installs to be cleaned up, got %v", stagedPaths)
	}
}

func TestInstallReusableAgentsInProjectScopePreservesGlobalInstall(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	globalAgentDir := filepath.Join(homeDir, ".compozy", "agents", "architect-advisor")
	if err := os.MkdirAll(globalAgentDir, 0o755); err != nil {
		t.Fatalf("mkdir global reusable agent: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalAgentDir, "AGENT.md"), []byte("global-existing"), 0o600); err != nil {
		t.Fatalf("write global reusable agent: %v", err)
	}

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: testExtensionReusableAgents(t, "architect-advisor"),
		Global:         false,
	})
	if err != nil {
		t.Fatalf("install reusable agents in project scope: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected reusable-agent installation successes")
	}

	projectAgentDir := filepath.Join(projectDir, ".compozy", "agents", "architect-advisor")
	assertFileExists(t, filepath.Join(projectAgentDir, "AGENT.md"))

	content, err := os.ReadFile(filepath.Join(globalAgentDir, "AGENT.md"))
	if err != nil {
		t.Fatalf("read preserved global reusable agent: %v", err)
	}
	if string(content) != "global-existing" {
		t.Fatalf("expected global install to remain unchanged, got %q", content)
	}
}

func newTestBundle(t *testing.T, files map[string]string) fs.FS {
	t.Helper()

	root := t.TempDir()
	for relativePath, content := range files {
		absolutePath := filepath.Join(root, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", absolutePath, err)
		}
		if err := os.WriteFile(absolutePath, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", absolutePath, err)
		}
	}
	return os.DirFS(root)
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}
