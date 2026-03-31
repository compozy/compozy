package setup

import (
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
		"create-prd/SKILL.md":               "---\nname: create-prd\ndescription: Create a PRD\n---\n",
		"create-prd/references/template.md": "# Template\n",
		"create-tasks/SKILL.md":             "---\nname: create-tasks\ndescription: Create tasks\n---\n",
		"create-tasks/references/tasks.md":  "# Tasks\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	result, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"create-prd"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install copy mode: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failures, got %#v", result.Failed)
	}

	skillDir := filepath.Join(projectDir, ".claude", "skills", "create-prd")
	assertFileExists(t, filepath.Join(skillDir, "SKILL.md"))
	assertFileExists(t, filepath.Join(skillDir, "references", "template.md"))
}

func TestInstallSymlinkModeUsesCanonicalDirForUniversalProjectAgent(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"create-prd/SKILL.md":               "---\nname: create-prd\ndescription: Create a PRD\n---\n",
		"create-prd/references/template.md": "# Template\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	result, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"create-prd"},
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

	skillDir := filepath.Join(projectDir, ".agents", "skills", "create-prd")
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
		"create-prd/SKILL.md": "---\nname: create-prd\ndescription: Create a PRD\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	items, err := Preview(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"create-prd"},
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

	want := filepath.Join(homeDir, ".agents", "skills", "create-prd")
	if items[0].CanonicalPath != want {
		t.Fatalf("unexpected canonical path\nwant: %s\ngot:  %s", want, items[0].CanonicalPath)
	}
	if items[0].TargetPath != want {
		t.Fatalf("unexpected target path\nwant: %s\ngot:  %s", want, items[0].TargetPath)
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
