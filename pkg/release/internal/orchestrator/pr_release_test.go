package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPRReleaseOrchestrator_Execute(t *testing.T) {
	t.Run("Should successfully create a new release PR when changes exist", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		// Set required environment variables
		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory structure in memory fs
		toolDirs := []string{"tools/fetch", "tools/write-file", "tools/list-files"}
		for _, dir := range toolDirs {
			err := fsRepo.MkdirAll(dir, 0755)
			require.NoError(t, err)
			// Create package.json files
			packageJSON := fmt.Sprintf(`{"name": %q, "version": "1.0.0"}`, filepath.Base(dir))
			err = afero.WriteFile(fsRepo, filepath.Join(dir, "package.json"), []byte(packageJSON), 0644)
			require.NoError(t, err)
		}

		// Setup expectations for checkChanges
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Once()

		// Setup expectations for calculateVersion (called again in prepareRelease)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Once()

		// Setup expectations for createReleaseBranch
		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		// Setup expectations for generateChangelog
		changelog := "## v1.1.0\n\n### Features\n- New feature added\n### Bug Fixes\n- Fixed critical bug"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").Return(changelog, nil).Once()

		// Setup expectations for commitChanges
		gitRepo.On("ConfigureUser", mock.Anything, "github-actions[bot]", "github-actions[bot]@users.noreply.github.com").
			Return(nil).
			Once()
		gitRepo.On("AddFiles", mock.Anything, "CHANGELOG.md").Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, "package.json").Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, "package-lock.json").Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, "tools/*/package.json").Return(nil).Once()
		gitRepo.On("Commit", mock.Anything, "ci(release): prepare release v1.1.0").Return(nil).Once()

		// Setup expectations for push and PR creation
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		githubRepo.On("CreateOrUpdatePR", mock.Anything, branchName, "main", "ci(release): Release v1.1.0",
			mock.MatchedBy(func(body string) bool {
				return strings.Contains(body, "Release v1.1.0") && strings.Contains(body, "### Features")
			}),
			[]string{"release-pending", "automated"}).Return(nil).Once()

		// Create orchestrator and execute
		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{
			ForceRelease: false,
			DryRun:       false,
			CIOutput:     false,
			SkipPR:       false,
		}

		err := orch.Execute(ctx, cfg)
		require.NoError(t, err)

		// Verify all expectations were met
		gitRepo.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)

		// Verify files were created
		changelogExists, _ := afero.Exists(fsRepo, "CHANGELOG.md")
		assert.True(t, changelogExists, "CHANGELOG.md should be created")
		releaseNotesExists, _ := afero.Exists(fsRepo, "RELEASE_NOTES.md")
		assert.True(t, releaseNotesExists, "RELEASE_NOTES.md should be created")
	})

	t.Run("Should skip PR creation when no changes exist and force flag is false", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")

		// Setup expectations - no version bump means no changes
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(0, nil).Once()

		// Create orchestrator and execute
		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{
			ForceRelease: false,
			DryRun:       false,
			CIOutput:     false,
			SkipPR:       false,
		}

		err := orch.Execute(ctx, cfg)
		require.NoError(t, err) // No error, just skips

		// Verify no further operations were performed
		gitRepo.AssertExpectations(t)
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
		cliffSvc.AssertNotCalled(t, "GenerateChangelog")
	})

	t.Run("Should force PR creation when force flag is set despite no changes", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)
		packageJSON := `{"name": "fetch", "version": "1.0.0"}`
		err = afero.WriteFile(fsRepo, "tools/fetch/package.json", []byte(packageJSON), 0644)
		require.NoError(t, err)

		// Setup expectations - no changes but force is true
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(0, nil).Once()

		// Even with no changes, force should trigger the flow
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		nextVersion, _ := domain.NewVersion("v1.0.1")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Once()

		// Setup remaining expectations for forced release
		branchName := "release/v1.0.1"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v1.0.1\n\n### Maintenance\n- Forced release"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.0.1", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		githubRepo.On("CreateOrUpdatePR", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()

		// Create orchestrator and execute with force flag
		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{
			ForceRelease: true,
			DryRun:       false,
			CIOutput:     false,
			SkipPR:       false,
		}

		err = orch.Execute(ctx, cfg)
		require.NoError(t, err)

		gitRepo.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should handle error when GITHUB_TOKEN is missing", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		// Explicitly unset GITHUB_TOKEN
		t.Setenv("GITHUB_TOKEN", "")

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err := orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "environment validation failed")
		assert.ErrorContains(t, err, "GITHUB_TOKEN")

		// Verify no operations were performed
		gitRepo.AssertNotCalled(t, "LatestTag")
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should handle error in version calculation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")

		// Setup expectations for checkChanges (use mock.Anything for context due to timeout wrapper)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()
		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Once()

		// Setup expectations for calculateVersion to fail (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("", errors.New("failed to get tag")).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err := orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to calculate version")

		gitRepo.AssertExpectations(t)
		// Verify PR was not created
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should handle error in changelog generation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)

		// Setup successful flow until changelog generation (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		// Fail on changelog generation (use mock.Anything for context)
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").
			Return("", errors.New("cliff failed")).
			Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err = orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to generate changelog")

		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
		// Verify PR was not created
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should handle error in PR creation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)
		packageJSON := `{"name": "fetch", "version": "1.0.0"}`
		err = afero.WriteFile(fsRepo, "tools/fetch/package.json", []byte(packageJSON), 0644)
		require.NoError(t, err)

		// Setup successful flow until PR creation (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Times(2) // Once for branch, once after commit
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v1.1.0\n\n### Features\n- New feature"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(nil).Once()

		// Fail on PR creation (use mock.Anything for context)
		// Note: The retry might not be happening for non-retryable errors
		githubRepo.On("CreateOrUpdatePR", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(errors.New("GitHub API error")).
			Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err = orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create pull request")

		gitRepo.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should skip PR creation when SkipPR flag is set", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)
		packageJSON := `{"name": "fetch", "version": "1.0.0"}`
		err = afero.WriteFile(fsRepo, "tools/fetch/package.json", []byte(packageJSON), 0644)
		require.NoError(t, err)

		// Setup expectations - normal flow but skip PR (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Times(2)
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v1.1.0\n\n### Features\n- New feature"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(nil).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{
			SkipPR: true,
		}

		err = orch.Execute(ctx, cfg)
		require.NoError(t, err)

		gitRepo.AssertExpectations(t)
		// Verify PR was not created
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should output CI format when CIOutput flag is set", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")

		// Capture output
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		// Setup expectations - no changes for simplicity (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(0, nil).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{
			CIOutput: true,
		}

		err := orch.Execute(ctx, cfg)
		require.NoError(t, err)

		// Restore stdout and read output
		w.Close()
		os.Stdout = oldStdout
		buf := make([]byte, 1024)
		n, _ := r.Read(buf)
		output := string(buf[:n])

		// Verify CI output format
		assert.Contains(t, output, "has_changes=false")
		assert.Contains(t, output, "latest_tag=v1.0.0")

		gitRepo.AssertExpectations(t)
	})

	t.Run("Should handle initial release when no tags exist", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)
		packageJSON := `{"name": "fetch", "version": "0.0.0"}`
		err = afero.WriteFile(fsRepo, "tools/fetch/package.json", []byte(packageJSON), 0644)
		require.NoError(t, err)

		// Setup expectations for initial release (no tags, use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("", nil).Once() // No tags exist

		// For calculateVersion when no tag exists (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("", nil).Once()
		initialVersion, _ := domain.NewVersion("v0.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v0.0.0").Return(initialVersion, nil).Once()

		branchName := "release/v0.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Times(2)
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v0.1.0\n\n### Features\n- Initial release"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v0.1.0", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(nil).Once()
		githubRepo.On("CreateOrUpdatePR", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err = orch.Execute(ctx, cfg)
		require.NoError(t, err)

		gitRepo.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should properly update package versions in tools directory", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create multiple tool packages
		tools := []string{"fetch", "write-file", "list-files", "grep"}
		for _, tool := range tools {
			dir := filepath.Join("tools", tool)
			err := fsRepo.MkdirAll(dir, 0755)
			require.NoError(t, err)
			packageJSON := fmt.Sprintf(`{"name": "@compozy/%s", "version": "1.0.0", "description": "Test tool"}`, tool)
			err = afero.WriteFile(fsRepo, filepath.Join(dir, "package.json"), []byte(packageJSON), 0644)
			require.NoError(t, err)
		}

		// Setup expectations (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(5, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Times(2)
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v1.1.0\n\n### Features\n- Updated tools"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(nil).Once()
		githubRepo.On("CreateOrUpdatePR", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err := orch.Execute(ctx, cfg)
		require.NoError(t, err)

		// Verify all tool package.json files were updated
		for _, tool := range tools {
			packagePath := filepath.Join("tools", tool, "package.json")
			content, err := afero.ReadFile(fsRepo, packagePath)
			require.NoError(t, err)
			assert.Contains(t, string(content), `"version": "1.1.0"`, "Package %s should have updated version", tool)
			assert.Contains(
				t,
				string(content),
				fmt.Sprintf(`"name": "@compozy/%s"`, tool),
				"Package name should be preserved",
			)
		}

		gitRepo.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should handle error when creating release branch fails", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")

		// Setup expectations (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		// Fail on branch creation (use mock.Anything for context)
		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(errors.New("branch already exists")).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err := orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create release branch")

		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
		// Verify no PR was created
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should handle commit errors gracefully", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")
		t.Setenv("TOOLS_DIR", "tools")

		// Create tools directory
		err := fsRepo.MkdirAll("tools/fetch", 0755)
		require.NoError(t, err)

		// Setup successful flow until commit (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Times(2)
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()

		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Times(2)

		branchName := "release/v1.1.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		changelog := "## v1.1.0\n\n### Features\n- New feature"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.1.0", "unreleased").Return(changelog, nil).Once()

		gitRepo.On("ConfigureUser", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", mock.Anything, mock.Anything).Return(nil).Times(4)
		// Fail on commit (use mock.Anything for context)
		gitRepo.On("Commit", mock.Anything, mock.Anything).Return(errors.New("nothing to commit")).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err = orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to commit changes")

		gitRepo.AssertExpectations(t)
		// Verify no PR was created
		githubRepo.AssertNotCalled(t, "CreateOrUpdatePR")
	})

	t.Run("Should validate version format correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		t.Setenv("GITHUB_TOKEN", "test-token")

		// Setup expectations for checkChanges (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		gitRepo.On("CommitsSinceTag", mock.Anything, "v1.0.0").Return(10, nil).Once()
		nextVersion, _ := domain.NewVersion("v1.1.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(nextVersion, nil).Once()

		// Setup expectations for calculateVersion to return nil version which will cause validation error
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		// Return nil to simulate an error case that will fail validation
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").
			Return(nil, errors.New("version calculation failed")).
			Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)
		cfg := PRReleaseConfig{}

		err := orch.Execute(ctx, cfg)
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to calculate version")

		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
}

func TestPRReleaseOrchestrator_prepareRelease(t *testing.T) {
	t.Run("Should validate branch name format", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		// Setup expectations - test with a normal version (use mock.Anything for context)
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil).Once()
		validVersion, _ := domain.NewVersion("v1.0.0")
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(validVersion, nil).Once()

		// Setup branch creation expectations (use mock.Anything for context)
		branchName := "release/v1.0.0"
		gitRepo.On("CreateBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("PushBranch", mock.Anything, branchName).Return(nil).Once()
		gitRepo.On("CheckoutBranch", mock.Anything, branchName).Return(nil).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)

		// This should succeed with valid branch name
		version, resultBranch, err := orch.prepareRelease(ctx, "v1.0.0", false)

		require.NoError(t, err)
		assert.Equal(t, "v1.0.0", version)
		assert.Equal(t, branchName, resultBranch)
		// Verify the branch name is within limits
		assert.LessOrEqual(t, len(resultBranch), 255)

		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
}

func TestPRReleaseOrchestrator_commitChanges(t *testing.T) {
	t.Run("Should configure git user correctly", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		expectedUser := "github-actions[bot]"
		expectedEmail := "github-actions[bot]@users.noreply.github.com"

		gitRepo.On("ConfigureUser", ctx, expectedUser, expectedEmail).Return(nil).Once()
		gitRepo.On("AddFiles", ctx, "CHANGELOG.md").Return(nil).Once()
		gitRepo.On("AddFiles", ctx, "package.json").Return(nil).Once()
		gitRepo.On("AddFiles", ctx, "package-lock.json").Return(nil).Once()
		gitRepo.On("AddFiles", ctx, "tools/*/package.json").Return(nil).Once()
		gitRepo.On("Commit", ctx, "ci(release): prepare release v1.2.0").Return(nil).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)

		err := orch.commitChanges(ctx, "v1.2.0")
		require.NoError(t, err)

		gitRepo.AssertExpectations(t)
	})

	t.Run("Should add all required files in correct order", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		npmSvc := new(mockNpmService)

		// Ensure files are added in the expected order
		var addedFiles []string
		gitRepo.On("ConfigureUser", ctx, mock.Anything, mock.Anything).Return(nil).Once()
		gitRepo.On("AddFiles", ctx, mock.Anything).Run(func(args mock.Arguments) {
			pattern := args.Get(1).(string)
			addedFiles = append(addedFiles, pattern)
		}).Return(nil).Times(4)
		gitRepo.On("Commit", ctx, mock.Anything).Return(nil).Once()

		orch := NewPRReleaseOrchestrator(gitRepo, githubRepo, fsRepo, cliffSvc, npmSvc)

		err := orch.commitChanges(ctx, "v1.2.0")
		require.NoError(t, err)

		// Verify files were added in correct order
		assert.Equal(t, []string{
			"CHANGELOG.md",
			"package.json",
			"package-lock.json",
			"tools/*/package.json",
		}, addedFiles)

		gitRepo.AssertExpectations(t)
	})
}
