package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReleaseOrchestrator_Execute(t *testing.T) {
	t.Run("Should successfully create a release with all steps", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "ci(release): prepare release v1.2.0")
		t.Setenv("TOOLS_DIR", "tools")
		version := "v1.2.0"
		changelog := "## v1.2.0\n\n### Features\n- feat: new awesome feature\n\n### Bug Fixes\n- fix: critical bug resolved"
		// Setup expectations in execution order (use mock.Anything for context due to timeout wrapper)
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return(changelog, nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create test tools directory structure for NPM publishing
		err := fsRepo.MkdirAll("tools/tool1", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "tools/tool1/package.json", []byte(`{"name": "tool1", "version": "1.0.0"}`), 0644)
		require.NoError(t, err)
		err = fsRepo.MkdirAll("tools/tool2", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "tools/tool2/package.json", []byte(`{"name": "tool2", "version": "1.0.0"}`), 0644)
		require.NoError(t, err)
		npmSvc.On("Publish", mock.Anything, filepath.Join("tools", "tool1")).Return(nil).Once()
		npmSvc.On("Publish", mock.Anything, filepath.Join("tools", "tool2")).Return(nil).Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err = orch.Execute(ctx, ReleaseConfig{
			SkipTag:       false,
			SkipNPM:       false,
			SkipChangelog: false,
			CIOutput:      false,
			DryRun:        false,
		})
		// Assertions
		require.NoError(t, err)
		// Verify RELEASE_NOTES.md was created
		exists, err := afero.Exists(afero.NewOsFs(), "RELEASE_NOTES.md")
		require.NoError(t, err)
		assert.True(t, exists)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		// Verify all mocks were called as expected
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
		goreleaserSvc.AssertExpectations(t)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should skip release when tag already exists (idempotency)", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "ci(release): prepare release v1.1.0")
		version := "v1.1.0"
		// Tag already exists - should skip creation but not fail
		gitRepo.On("TagExists", mock.Anything, version).Return(true, nil).Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err := orch.Execute(ctx, ReleaseConfig{
			SkipTag:       false,
			SkipNPM:       false,
			SkipChangelog: false,
			CIOutput:      false,
			DryRun:        false,
		})
		// Should succeed (idempotent)
		require.NoError(t, err)
		// Verify only TagExists was called
		gitRepo.AssertExpectations(t)
		gitRepo.AssertNotCalled(t, "CreateTag", mock.Anything, mock.Anything, mock.Anything)
		gitRepo.AssertNotCalled(t, "PushTag", mock.Anything, mock.Anything)
		cliffSvc.AssertNotCalled(t, "GenerateChangelog", mock.Anything, mock.Anything, mock.Anything)
		npmSvc.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
	})
	t.Run("Should fail when no version found in commit message", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment without version in commit message
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "ci(release): prepare release without version")
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err := orch.Execute(ctx, ReleaseConfig{})
		// Should fail
		require.Error(t, err)
		assert.ErrorContains(t, err, "no version found in commit message")
	})
	t.Run("Should fail when tag creation fails", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "ci(release): prepare release v1.3.0")
		version := "v1.3.0"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		// The retry logic doesn't retry by default unless error is marked as retryable
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).
			Return(fmt.Errorf("permission denied")).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err := orch.Execute(ctx, ReleaseConfig{})
		// Should fail
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create tag")
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should generate release changelog correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v2.0.0")
		version := "v2.0.0"
		expectedChangelog := "## v2.0.0\n\n### Breaking Changes\n- breaking: major API redesign"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return(expectedChangelog, nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute with NPM skipped to simplify test
		err := orch.Execute(ctx, ReleaseConfig{
			SkipNPM: true,
		})
		// Assertions
		require.NoError(t, err)
		// Verify RELEASE_NOTES.md was created with expected content
		content, err := os.ReadFile("RELEASE_NOTES.md")
		require.NoError(t, err)
		assert.Equal(t, expectedChangelog, string(content))
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
		goreleaserSvc.AssertExpectations(t)
	})
	t.Run("Should update main changelog after tag creation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v1.5.0")
		version := "v1.5.0"
		changelog := "## v1.5.0\n\n### Features\n- feat: amazing feature"
		// Create initial CHANGELOG.md in filesystem
		err := afero.WriteFile(fsRepo, "CHANGELOG.md", []byte("# Changelog\n\n## v1.4.0\n- Previous release"), 0644)
		require.NoError(t, err)
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return(changelog, nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute with NPM skipped
		err = orch.Execute(ctx, ReleaseConfig{
			SkipNPM: true,
		})
		// Assertions
		require.NoError(t, err)
		// Verify CHANGELOG.md was updated
		content, err := afero.ReadFile(fsRepo, "CHANGELOG.md")
		require.NoError(t, err)
		expectedContent := "# Changelog\n\n## v1.5.0\n\n### Features\n- feat: amazing feature\n\n## v1.4.0\n- Previous release"
		assert.Equal(t, expectedContent, string(content))
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should publish NPM packages when not skipped", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v1.6.0")
		version := "v1.6.0"
		// Create test NPM packages
		err := fsRepo.MkdirAll("tools/package-a", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(
			fsRepo,
			"tools/package-a/package.json",
			[]byte(`{"name": "@compozy/package-a", "version": "1.0.0"}`),
			0644,
		)
		require.NoError(t, err)
		err = fsRepo.MkdirAll("tools/package-b", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(
			fsRepo,
			"tools/package-b/package.json",
			[]byte(`{"name": "@compozy/package-b", "version": "1.0.0"}`),
			0644,
		)
		require.NoError(t, err)
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return("## v1.6.0\n- Changes", nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		npmSvc.On("Publish", mock.Anything, filepath.Join("tools", "package-a")).Return(nil).Once()
		npmSvc.On("Publish", mock.Anything, filepath.Join("tools", "package-b")).Return(nil).Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err = orch.Execute(ctx, ReleaseConfig{
			SkipNPM: false,
		})
		// Assertions
		require.NoError(t, err)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		npmSvc.AssertExpectations(t)
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should skip NPM publishing when SkipNpm is true", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment (no NPM_TOKEN needed when skipping)
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v1.7.0")
		version := "v1.7.0"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return("## v1.7.0\n- Changes", nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute with NPM skipped
		err := orch.Execute(ctx, ReleaseConfig{
			SkipNPM: true,
		})
		// Assertions
		require.NoError(t, err)
		// Verify NPM service was NOT called
		npmSvc.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle errors in changelog generation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v1.8.0")
		version := "v1.8.0"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").
			Return("", fmt.Errorf("cliff command failed")).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err := orch.Execute(ctx, ReleaseConfig{})
		// Should fail
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to generate changelog")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should handle errors in NPM publishing", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v1.9.0")
		version := "v1.9.0"
		// Create test NPM package
		err := fsRepo.MkdirAll("tools/failing-package", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(
			fsRepo,
			"tools/failing-package/package.json",
			[]byte(`{"name": "@compozy/failing", "version": "1.0.0"}`),
			0644,
		)
		require.NoError(t, err)
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return("## v1.9.0\n- Changes", nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// The retry logic doesn't retry by default unless error is marked as retryable
		npmSvc.On("Publish", mock.Anything, filepath.Join("tools", "failing-package")).
			Return(fmt.Errorf("npm publish failed")).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err = orch.Execute(ctx, ReleaseConfig{
			SkipNPM: false,
		})
		// Should fail
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to publish NPM packages")
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should handle CI output mode correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v2.1.0")
		version := "v2.1.0"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, version, fmt.Sprintf("Release %s", version)).Return(nil).Once()
		gitRepo.On("PushTag", mock.Anything, version).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").Return("## v2.1.0\n- CI test", nil).Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute with CI output enabled
		err := orch.Execute(ctx, ReleaseConfig{
			CIOutput: true,
			SkipNPM:  true,
		})
		// Assertions
		require.NoError(t, err)
		// In CI mode, version should be printed (we can't easily capture stdout in tests)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle dry run mode correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		t.Setenv("NPM_TOKEN", "test-npm-token")
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v3.0.0")
		version := "v3.0.0"
		// Setup expectations - only check tag exists and generate changelog
		gitRepo.On("TagExists", mock.Anything, version).Return(false, nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, version, "release").
			Return("## v3.0.0\n- Dry run test", nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute in dry run mode
		err := orch.Execute(ctx, ReleaseConfig{
			DryRun: true,
		})
		// Assertions
		require.NoError(t, err)
		// Verify that CreateTag, PushTag, and GoReleaser were NOT called
		gitRepo.AssertNotCalled(t, "CreateTag", mock.Anything, mock.Anything, mock.Anything)
		gitRepo.AssertNotCalled(t, "PushTag", mock.Anything, mock.Anything)
		goreleaserSvc.AssertNotCalled(t, "Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		npmSvc.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should validate environment variables correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment without required tokens
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: v4.0.0")
		// Unset required variables
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("NPM_TOKEN")
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute
		err := orch.Execute(ctx, ReleaseConfig{
			SkipNPM: false,
		})
		// Should fail on environment validation
		require.Error(t, err)
		assert.ErrorContains(t, err, "environment validation failed")
	})
	t.Run("Should accept provided version instead of extracting from commit", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)
		goreleaserSvc := new(mockGoReleaserService)
		// Setup environment
		t.Setenv("GITHUB_TOKEN", "test-github-token")
		// No version in commit message
		t.Setenv("GITHUB_EVENT_HEAD_COMMIT_MESSAGE", "release: prepare production release")
		providedVersion := "v5.0.0"
		// Setup expectations
		gitRepo.On("TagExists", mock.Anything, providedVersion).Return(false, nil).Once()
		gitRepo.On("CreateTag", mock.Anything, providedVersion, fmt.Sprintf("Release %s", providedVersion)).
			Return(nil).
			Once()
		gitRepo.On("PushTag", mock.Anything, providedVersion).Return(nil).Once()
		cliffSvc.On("GenerateChangelog", mock.Anything, providedVersion, "release").
			Return("## v5.0.0\n- Provided version test", nil).
			Once()
		goreleaserSvc.On("Run", mock.Anything, "release", "--clean", "--release-notes=RELEASE_NOTES.md").
			Return(nil).
			Once()
		// Create orchestrator
		orch := NewReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc, goreleaserSvc)
		// Execute with provided version
		err := orch.Execute(ctx, ReleaseConfig{
			Version: providedVersion,
			SkipNPM: true,
		})
		// Assertions
		require.NoError(t, err)
		// Cleanup
		_ = os.Remove("RELEASE_NOTES.md")
		gitRepo.AssertExpectations(t)
	})
}
