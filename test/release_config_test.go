package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goReleaserConfig struct {
	Before        goReleaserBefore         `yaml:"before"`
	Archives      []goReleaserArchive      `yaml:"archives"`
	HomebrewCasks []goReleaserHomebrewCask `yaml:"homebrew_casks"`
}

type goReleaserBefore struct {
	Hooks []string `yaml:"hooks"`
}

type goReleaserArchive struct {
	ID              string `yaml:"id"`
	WrapInDirectory bool   `yaml:"wrap_in_directory"`
}

type goReleaserHomebrewCask struct {
	Name string   `yaml:"name"`
	IDs  []string `yaml:"ids"`
}

func TestReleaseWorkflowsUseScopedReleaseNotesGenerator(t *testing.T) {
	t.Parallel()

	const fixedModule = "github.com/compozy/releasepr@v0.0.19"
	brokenModules := []string{
		"github.com/compozy/releasepr@v0.0.17",
		"github.com/compozy/releasepr@v0.0.18",
	}
	workflowPaths := []string{
		filepath.Join(repoRoot(t), ".github", "workflows", "auto-docs.yml"),
		filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"),
	}

	for _, workflowPath := range workflowPaths {
		workflowPath := workflowPath
		t.Run(filepath.Base(workflowPath), func(t *testing.T) {
			t.Parallel()
			content, err := os.ReadFile(workflowPath)
			if err != nil {
				t.Fatalf("read release workflow: %v", err)
			}
			text := string(content)
			if !strings.Contains(text, "PR_RELEASE_MODULE: "+fixedModule) {
				t.Fatalf("expected workflow to use fixed releasepr module %q", fixedModule)
			}
			for _, brokenModule := range brokenModules {
				if strings.Contains(text, brokenModule) {
					t.Fatalf("expected workflow to avoid broken releasepr module %q", brokenModule)
				}
			}
		})
	}
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

func TestGoReleaserBuildsFrontendBundleBeforeBinaries(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join(repoRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read goreleaser config: %v", err)
	}

	var cfg goReleaserConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		t.Fatalf("unmarshal goreleaser config: %v", err)
	}

	foundFrontendBuild := false
	for _, hook := range cfg.Before.Hooks {
		if hook == "make frontend-build" {
			foundFrontendBuild = true
			break
		}
	}
	if !foundFrontendBuild {
		t.Fatal("expected GoReleaser to build the frontend bundle before compiling binaries")
	}

	workflowContent, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatalf("read release workflow: %v", err)
	}
	workflow := string(workflowContent)

	dryRunBlock := workflowJobBlock(t, workflow, "dry-run", "release")
	assertWorkflowStepBefore(
		t,
		dryRunBlock,
		"uses: ./.github/actions/setup-bun",
		"uses: ./.github/actions/setup-release",
		"expected release dry-run to install Bun before invoking GoReleaser",
	)

	releaseBlock := workflowJobBlock(t, workflow, "release", "")
	assertWorkflowStepBefore(
		t,
		releaseBlock,
		"uses: ./.github/actions/setup-bun",
		"uses: goreleaser/goreleaser-action@v6",
		"expected production release to install Bun before invoking GoReleaser",
	)
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

func workflowJobBlock(t *testing.T, workflow string, jobName string, nextJobName string) string {
	t.Helper()

	startNeedle := "\n  " + jobName + ":\n"
	start := strings.Index(workflow, startNeedle)
	if start == -1 {
		t.Fatalf("expected workflow to contain job %q", jobName)
	}
	start += len("\n")
	if nextJobName == "" {
		return workflow[start:]
	}
	endNeedle := "\n  " + nextJobName + ":\n"
	end := strings.Index(workflow[start:], endNeedle)
	if end == -1 {
		t.Fatalf("expected workflow job %q to be followed by %q", jobName, nextJobName)
	}
	return workflow[start : start+end]
}

func assertWorkflowStepBefore(t *testing.T, block string, first string, second string, message string) {
	t.Helper()

	firstIndex := strings.Index(block, first)
	if firstIndex == -1 {
		t.Fatalf("%s: missing %q", message, first)
	}
	secondIndex := strings.Index(block, second)
	if secondIndex == -1 {
		t.Fatalf("%s: missing %q", message, second)
	}
	if firstIndex > secondIndex {
		t.Fatalf("%s: %q appears after %q", message, first, second)
	}
}
