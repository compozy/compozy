package test

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

func TestGoReleaserConfigUsesReadableChangelogTitlesAndFiltersReleaseCommits(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	text := string(content)

	expectedTitles := []string{
		`title: "🎉 Features"`,
		`title: "🐛 Bug Fixes"`,
		`title: "⚡ Performance Improvements"`,
		`title: "🔒 Security"`,
		`title: "📚 Documentation"`,
		`title: "♻️ Refactoring"`,
		`title: "📦 Dependencies"`,
		`title: "🧪 Testing"`,
		`title: "Other Changes"`,
	}

	for _, title := range expectedTitles {
		title := title
		t.Run("Should include readable title "+title, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(text, title) {
				t.Fatalf("expected goreleaser changelog config to include readable group title %q", title)
			}
		})
	}

	unexpectedTitles := []string{
		`title: "\U0001F389"`,
		`title: "\U0001F41B"`,
		`title: "⚡"`,
		`title: "\U0001F510"`,
		`title: "\U0001F4DA"`,
		`title: "\U0001F527"`,
		`title: "\U0001F4E6"`,
		`title: "\U0001F9EA"`,
		`title: "\U0001F504"`,
	}

	for _, title := range unexpectedTitles {
		title := title
		t.Run("Should avoid emoji-only title "+title, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(text, title) {
				t.Fatalf("expected goreleaser changelog config to avoid emoji-only group title %q", title)
			}
		})
	}

	expectedFilters := []string{
		`- "^ci\\(release\\): "`,
		`- "^chore\\(release\\): "`,
	}

	for _, filter := range expectedFilters {
		filter := filter
		t.Run("Should exclude release automation filter "+filter, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(text, filter) {
				t.Fatalf(
					"expected goreleaser changelog config to exclude release automation commits with filter %q",
					filter,
				)
			}
		})
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
