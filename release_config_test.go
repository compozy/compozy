package looper_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoReleaserConfigSupportsFirstRelease(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	text := string(content)

	if strings.Contains(text, "use: github") {
		t.Fatal(
			"expected goreleaser changelog generation to avoid the GitHub compare API so the first release works without a previous remote tag",
		)
	}

	if !strings.Contains(text, "use: git") {
		t.Fatal("expected goreleaser changelog generation to use git history for first-release compatibility")
	}

	if !strings.Contains(text, "{{- if .PreviousTag }}") {
		t.Fatal("expected release notes to guard previous-tag links for the first release")
	}

	if !strings.Contains(text, "compare/{{ .PreviousTag }}...{{ .Tag }}") {
		t.Fatal("expected release notes to keep the compare link when a previous tag exists")
	}

	if !strings.Contains(text, "tree/{{ .Tag }}") {
		t.Fatal("expected release notes to include a first-release fallback link when no previous tag exists")
	}
}

func TestSetupReleaseActionUsesSupportedCosignVersionCommand(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "actions", "setup-release", "action.yml"))
	if err != nil {
		t.Fatalf("read setup-release action: %v", err)
	}

	text := string(content)

	if strings.Contains(text, "cosign version --short") {
		t.Fatal("expected setup-release to avoid the unsupported `cosign version --short` command")
	}

	if !strings.Contains(text, "echo \"Cosign version:\"") {
		t.Fatal("expected setup-release to print a cosign version header before running the standalone version command")
	}

	if !strings.Contains(text, "\n          cosign version\n") {
		t.Fatal(
			"expected setup-release to run `cosign version` as a standalone command so failures are not hidden inside command substitution",
		)
	}
}
