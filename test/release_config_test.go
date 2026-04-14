package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goReleaserConfig struct {
	Archives      []goReleaserArchive      `yaml:"archives"`
	HomebrewCasks []goReleaserHomebrewCask `yaml:"homebrew_casks"`
}

type goReleaserArchive struct {
	ID              string `yaml:"id"`
	WrapInDirectory bool   `yaml:"wrap_in_directory"`
}

type goReleaserHomebrewCask struct {
	Name string   `yaml:"name"`
	IDs  []string `yaml:"ids"`
}

func TestGoReleaserConfigSupportsFirstRelease(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	configText := string(content)

	if strings.Contains(configText, "use: github") {
		t.Fatal(
			"expected goreleaser changelog generation to avoid the GitHub compare API so the first release works without a previous remote tag",
		)
	}

	if !strings.Contains(configText, "use: git") {
		t.Fatal("expected goreleaser changelog generation to use git history for first-release compatibility")
	}

	footerContent, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.release-footer.md.tmpl"))
	if err != nil {
		t.Fatalf("read goreleaser release footer template: %v", err)
	}

	footerText := string(footerContent)

	if !strings.Contains(footerText, "{{- if .PreviousTag }}") {
		t.Fatal("expected release notes to guard previous-tag links for the first release")
	}

	if !strings.Contains(footerText, "compare/{{ .PreviousTag }}...{{ .Tag }}") {
		t.Fatal("expected release notes to keep the compare link when a previous tag exists")
	}

	if !strings.Contains(footerText, "tree/{{ .Tag }}") {
		t.Fatal("expected release notes to include a first-release fallback link when no previous tag exists")
	}

	workflowContent, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}

	if !strings.Contains(string(workflowContent), "--release-footer-tmpl=.goreleaser.release-footer.md.tmpl") {
		t.Fatal("expected the release workflow to pass the first-release footer template to goreleaser")
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

func TestGoReleaserConfigKeepsHomebrewCaskArchivesUnwrapped(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	var cfg goReleaserConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal goreleaser config: %v", err)
	}

	if len(cfg.HomebrewCasks) == 0 {
		t.Fatal("expected goreleaser config to define at least one Homebrew cask")
	}

	archiveByID := make(map[string]goReleaserArchive, len(cfg.Archives))
	archiveIDs := make([]string, 0, len(cfg.Archives))
	for _, archive := range cfg.Archives {
		if strings.TrimSpace(archive.ID) == "" {
			continue
		}
		archiveByID[archive.ID] = archive
		archiveIDs = append(archiveIDs, archive.ID)
	}

	if len(archiveByID) == 0 {
		t.Fatal("expected goreleaser config to define archive IDs")
	}

	for _, cask := range cfg.HomebrewCasks {
		cask := cask
		t.Run(cask.Name, func(t *testing.T) {
			t.Parallel()

			targetIDs := cask.IDs
			if len(targetIDs) == 0 {
				targetIDs = archiveIDs
			}

			for _, id := range targetIDs {
				archive, ok := archiveByID[id]
				if !ok {
					t.Fatalf("expected Homebrew cask %q to reference a known archive id %q", cask.Name, id)
				}
				if archive.WrapInDirectory {
					t.Fatalf(
						"expected Homebrew cask archive %q to keep the binary at the archive root so brew does not depend on rename",
						id,
					)
				}
			}
		})
	}
}
