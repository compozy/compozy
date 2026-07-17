package packaging

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/setup"
)

// IT-002: importing .compozy/DECISIONS.md into an agent-memory file surfaces the
// terse index text, while the rich decisions/AD-NNN.md bodies are NOT pulled in by
// that import. This is the two-tier consumption contract (ADR-001, US-011.EC-3):
// only the index loads into every session; bodies are read on demand.
//
// The @import directive is resolved by the coding agent, not by Compozy, so this
// test models the documented semantics directly: @path inlines exactly that one
// file. It is driven by the exact directive the README publishes, tying the doc to
// the behavior.
func TestIndexImportSurfacesIndexNotBodies(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const indexLine = "AD-001 | Event-sourcing for orders | proven | [orders, async] | audit + replay | feat-orders"
	const bodySentinel = "SENTINEL_BODY_ONLY_RECONCILIATION_TEXT"
	writeUnder(t, root, ".compozy/DECISIONS.md",
		"# Project Decisions (active, proven)\n\n"+indexLine+"\n")
	writeUnder(t, root, ".compozy/decisions/AD-001.md",
		"---\nid: AD-001\nstatus: proven\n---\n\n## Reconciliation\n\n"+bodySentinel+"\n")

	directive := extractImportDirective(t)
	writeUnder(t, root, "CLAUDE.md", "# Project memory\n\n"+directive+"\n")

	assembled := resolveMemoryImports(t, root, filepath.Join(root, "CLAUDE.md"))
	if !strings.Contains(assembled, indexLine) {
		t.Fatal("resolved agent memory does not surface the proven index line")
	}
	if strings.Contains(assembled, bodySentinel) {
		t.Fatal("resolved agent memory pulled in a decisions/ body; only the index should be imported")
	}
}

// IT-003: the README's durability negations flip the log from ignored to tracked in
// a repo that ignores .compozy/**; with no .gitignore the log is tracked by default.
// Exercised against real git so the assertion validates git's actual behavior, not a
// re-implementation of gitignore rules.
func TestGitignoreNegationsFlipLogToTracked(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git is required for this test but was not found: %v", err)
	}
	negations := extractGitignoreNegations(t)
	const index = ".compozy/DECISIONS.md"
	const body = ".compozy/decisions/AD-001.md"

	t.Run("negations re-include an ignored log", func(t *testing.T) {
		t.Parallel()
		repo := initRepo(t)
		writeUnder(t, repo, ".gitignore", ".compozy/**\n")
		writeUnder(t, repo, index, "index\n")
		writeUnder(t, repo, body, "body\n")
		if !gitIgnored(t, repo, index) || !gitIgnored(t, repo, body) {
			t.Fatal("precondition failed: .compozy/** should ignore both the index and the body")
		}
		appendFile(t, filepath.Join(repo, ".gitignore"), strings.Join(negations, "\n")+"\n")
		if gitIgnored(t, repo, index) {
			t.Fatal("index is still ignored after applying the README negations")
		}
		if gitIgnored(t, repo, body) {
			t.Fatal("body is still ignored after applying the README negations")
		}
	})

	t.Run("no .gitignore tracks the log by default", func(t *testing.T) {
		t.Parallel()
		repo := initRepo(t)
		writeUnder(t, repo, index, "index\n")
		writeUnder(t, repo, body, "body\n")
		if gitIgnored(t, repo, index) || gitIgnored(t, repo, body) {
			t.Fatal("the log should be tracked by default when no .gitignore exists")
		}
	})
}

// IT-004: installing the extension's skill pack lands SKILL.md in an agent skill
// dir, and a repeat install (as `compozy setup` re-run) is idempotent — a single
// current install, with no duplicates and no drift. Uses the real setup install
// path so the assertion reflects how `compozy setup` actually installs the skill.
func TestInstallLandsSkillInAgentDirIdempotently(t *testing.T) {
	t.Parallel()
	skillDir, err := filepath.Abs(skillDirPath)
	if err != nil {
		t.Fatalf("resolve skill dir: %v", err)
	}
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	cfg := setup.ExtensionInstallConfig{
		ResolverOptions: setup.ResolverOptions{CWD: projectDir, HomeDir: homeDir},
		Packs: []setup.SkillPackSource{{
			ExtensionName: "cy-capture-decisions",
			ManifestPath: filepath.Join(
				projectDir, ".compozy", "extensions", "cy-capture-decisions", "extension.toml",
			),
			ResolvedPath: skillDir,
		}},
		AgentNames: []string{"codex"},
		Mode:       setup.InstallModeCopy,
	}

	install := func() {
		t.Helper()
		result, installErr := setup.InstallExtensionSkillPacks(cfg)
		if installErr != nil {
			t.Fatalf("install extension skill packs: %v", installErr)
		}
		if len(result.Failed) != 0 {
			t.Fatalf("install reported failures: %#v", result.Failed)
		}
	}

	install()
	installedSkill := filepath.Join(projectDir, ".agents", "skills", "cy-capture-decisions", "SKILL.md")
	assertExists(t, installedSkill)

	install() // re-running setup must not duplicate or break the existing install
	assertExists(t, installedSkill)

	entries, err := os.ReadDir(filepath.Join(projectDir, ".agents", "skills"))
	if err != nil {
		t.Fatalf("read installed skills dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "cy-capture-decisions" {
		t.Fatalf("expected a single cy-capture-decisions install, got %v", dirNames(entries))
	}

	verify, err := setup.VerifyExtensionSkillPacks(setup.ExtensionVerifyConfig{
		ResolverOptions: setup.ResolverOptions{CWD: projectDir, HomeDir: homeDir},
		Packs:           cfg.Packs,
		AgentName:       "codex",
	})
	if err != nil {
		t.Fatalf("verify extension skill packs: %v", err)
	}
	if verify.HasMissing() || verify.HasDrift() {
		t.Fatalf(
			"post-install verify was not clean: missing=%v drifted=%v",
			verify.MissingSkillNames(), verify.DriftedSkillNames(),
		)
	}
}

// extractImportDirective returns the standalone @.compozy/DECISIONS.md line the
// README publishes, failing if the README does not document it.
func extractImportDirective(t *testing.T) string {
	t.Helper()
	for _, line := range strings.Split(readFile(t, readmePath), "\n") {
		if strings.TrimSpace(line) == "@.compozy/DECISIONS.md" {
			return "@.compozy/DECISIONS.md"
		}
	}
	t.Fatal("README does not contain a standalone @.compozy/DECISIONS.md import directive")
	return ""
}

// extractGitignoreNegations returns every `!.compozy...` line the README's
// durability section instructs the user to add.
func extractGitignoreNegations(t *testing.T) []string {
	t.Helper()
	var negations []string
	for _, line := range strings.Split(readmeSection(t, "durable"), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "!.compozy") {
			negations = append(negations, trimmed)
		}
	}
	if len(negations) == 0 {
		t.Fatal("README durability section has no !.compozy negation lines")
	}
	return negations
}

// resolveMemoryImports models the coding agent's @path memory-import semantics:
// each @-prefixed token inlines exactly that one file's content (a directory or
// missing path contributes nothing, so index bodies are never pulled by importing
// the index file).
func resolveMemoryImports(t *testing.T, root, memoryFile string) string {
	t.Helper()
	content := readFile(t, memoryFile)
	var b strings.Builder
	b.WriteString(content)
	for _, token := range strings.Fields(content) {
		if !strings.HasPrefix(token, "@") {
			continue
		}
		rel := strings.TrimPrefix(token, "@")
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		b.WriteByte('\n')
		b.Write(data)
	}
	return b.String()
}

// gitIgnored reports whether git ignores path within repo. git check-ignore -q
// exits 0 when the path is ignored and 1 when it is not.
func gitIgnored(t *testing.T, repo, path string) bool {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", "-C", repo, "check-ignore", "-q", path)
	cmd.Env = gitEnv(repo)
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false
	}
	t.Fatalf("git check-ignore %s: %v", path, err)
	return false
}
