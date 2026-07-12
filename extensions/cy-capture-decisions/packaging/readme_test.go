package packaging

import (
	"strings"
	"testing"
)

// UT-009: the README's durability section names the exact gitignore negations a
// user must add so the log stays committed in a repo that ignores .compozy/**.
func TestReadmeDocumentsGitignoreNegations(t *testing.T) {
	t.Parallel()
	section := readmeSection(t, "durable")
	for _, negation := range []string{"!.compozy/DECISIONS.md", "!.compozy/decisions/**"} {
		if !strings.Contains(section, negation) {
			t.Fatalf("durability section is missing gitignore negation %q", negation)
		}
	}
}

// UT-010: the README documents the @import consumption wiring and covers both
// agent-memory conventions (CLAUDE.md and AGENTS.md).
func TestReadmeDocumentsImportWiring(t *testing.T) {
	t.Parallel()
	readme := readFile(t, readmePath)
	for _, token := range []string{"@.compozy/DECISIONS.md", "CLAUDE.md", "AGENTS.md"} {
		if !strings.Contains(readme, token) {
			t.Fatalf("README is missing consumption-wiring token %q", token)
		}
	}
}

// Regression guard (reviews-004/issue_001): every documented `compozy ext install`
// invocation must carry a <source>. The CLI declares Args: cobra.ExactArgs(1) in
// internal/cli/extension/install.go, so a bare `compozy ext install` is rejected with
// "accepts 1 arg(s), received 0" before any install logic runs — a hard onboarding
// failure for a README that is this skill-only extension's entire product surface.
func TestReadmeInstallCommandsIncludeSource(t *testing.T) {
	t.Parallel()
	readme := readFile(t, readmePath)
	for i, line := range strings.Split(readme, "\n") {
		command := line
		if idx := strings.Index(command, "#"); idx >= 0 {
			command = command[:idx] // drop a trailing shell comment before matching
		}
		if strings.TrimSpace(command) == "compozy ext install" {
			t.Fatalf(
				"README line %d invokes `compozy ext install` with no <source>; the CLI requires exactly one positional argument",
				i+1,
			)
		}
	}
	// The fix must document the canonical first-party bundle source for this extension,
	// matching how the sibling cy-idea-factory extension is installed across the repo.
	const canonicalSource = "compozy/compozy --remote github --ref <tag> --subdir extensions/cy-capture-decisions"
	if !strings.Contains(readme, canonicalSource) {
		t.Fatalf("README does not document the canonical install source %q", canonicalSource)
	}
}

// UT-011: the README states capture's canonical flow position — run as the final
// step, after /cy-final-verify.
func TestReadmeStatesCanonicalFlowPosition(t *testing.T) {
	t.Parallel()
	readme := readFile(t, readmePath)
	if !strings.Contains(readme, "/cy-final-verify") {
		t.Fatal("README does not mention /cy-final-verify")
	}
	// The canonical flow line must place capture after final-verify.
	if !strings.Contains(readme, "/cy-final-verify → /cy-capture-decisions") {
		t.Fatal("README canonical flow does not run /cy-capture-decisions after /cy-final-verify")
	}
	// And the prose must state the ordering explicitly, not only in the diagram.
	if !strings.Contains(readme, "after `/cy-final-verify`") {
		t.Fatal("README does not state in prose that capture runs after /cy-final-verify")
	}
}
