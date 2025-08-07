package orchestrator

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDryRunOrchestrator_Execute(t *testing.T) {
	t.Run("Should successfully execute dry-run validation", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"
		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		// Setup expectations
		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)
		// Setup test environment
		t.Setenv("GITHUB_HEAD_REF", "release/v1.1.0")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_ISSUE_NUMBER", "123")

		// Create tools directory for NPM validation
		err := fsRepo.MkdirAll("tools", 0755)
		require.NoError(t, err)

		// Create mock metadata file that GoReleaser would generate
		metadata := `{"version":"v1.1.0","artifacts":[{"type":"Archive","goos":"linux","goarch":"amd64"}]}`
		err = afero.WriteFile(fsRepo, "dist/metadata.json", []byte(metadata), 0644)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "dist/checksums.txt", []byte("checksums"), 0644)
		require.NoError(t, err)

		githubRepo.On("AddComment", mock.Anything, 123, mock.MatchedBy(func(body string) bool {
			return strings.Contains(body, "Dry-Run Completed Successfully")
		})).Return(nil)

		// Execute
		cfg := DryRunConfig{CIOutput: false}
		err = orch.Execute(ctx, cfg)
		require.NoError(t, err)

		goreleaserSvc.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
	})

	t.Run("Should fail when GoReleaser dry-run fails", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		t.Setenv("GITHUB_HEAD_REF", "release/v1.1.0")

		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").
			Return(errors.New("dry-run failed"))

		err := orch.Execute(ctx, DryRunConfig{})
		assert.ErrorContains(t, err, "GoReleaser dry-run failed")
	})

	t.Run("Should fail when no version found in branch name", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		t.Setenv("GITHUB_HEAD_REF", "feature/no-version")
		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)

		err := orch.Execute(ctx, DryRunConfig{})
		assert.ErrorContains(t, err, "no version found in branch name")
	})

	t.Run("Should skip artifact upload when not in CI environment", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		// Create tools directory in memory filesystem
		err := fsRepo.MkdirAll(toolsDir, 0755)
		require.NoError(t, err)

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		t.Setenv("GITHUB_HEAD_REF", "release/v1.1.0")
		// Do NOT set GITHUB_ACTIONS - simulating local run

		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)

		err = orch.Execute(ctx, DryRunConfig{CIOutput: false})
		require.NoError(t, err)

		// Should NOT call AddComment since not in CI
		githubRepo.AssertNotCalled(t, "AddComment", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("Should handle invalid metadata.json gracefully", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		// Create tools directory in memory filesystem
		err := fsRepo.MkdirAll(toolsDir, 0755)
		require.NoError(t, err)

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		t.Setenv("GITHUB_HEAD_REF", "release/v1.1.0")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_ISSUE_NUMBER", "123")

		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)

		// Create dist directory and invalid metadata file
		err = fsRepo.MkdirAll("dist", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "dist/metadata.json", []byte("invalid json"), 0644)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "dist/checksums.txt", []byte("checksums"), 0644)
		require.NoError(t, err)

		// Execute should handle invalid JSON gracefully
		err = orch.Execute(ctx, DryRunConfig{CIOutput: false})
		assert.ErrorContains(t, err, "failed to parse metadata.json")

		goreleaserSvc.AssertExpectations(t)
	})

	t.Run("Should post comment to PR when in CI with issue number", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		// Create tools directory in memory filesystem
		err := fsRepo.MkdirAll(toolsDir, 0755)
		require.NoError(t, err)

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		// Setup CI environment
		t.Setenv("GITHUB_HEAD_REF", "release/v2.0.0")
		t.Setenv("GITHUB_ACTIONS", "true")
		t.Setenv("GITHUB_ISSUE_NUMBER", "456")
		t.Setenv("GITHUB_SHA", "abc123def456789")

		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)

		// Create metadata with multiple artifacts
		metadata := `{
			"version":"v2.0.0",
			"artifacts":[
				{"type":"Archive","goos":"linux","goarch":"amd64"},
				{"type":"Archive","goos":"darwin","goarch":"amd64"},
				{"type":"Archive","goos":"windows","goarch":"amd64"}
			]
		}`
		err = fsRepo.MkdirAll("dist", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "dist/metadata.json", []byte(metadata), 0644)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "dist/checksums.txt", []byte("checksums"), 0644)
		require.NoError(t, err)

		// Expect comment with proper formatting
		githubRepo.On("AddComment", mock.Anything, 456, mock.MatchedBy(func(body string) bool {
			return strings.Contains(body, "Dry-Run Completed Successfully") &&
				strings.Contains(body, "v2.0.0") &&
				strings.Contains(body, "linux/amd64") &&
				strings.Contains(body, "darwin/amd64") &&
				strings.Contains(body, "windows/amd64")
		})).Return(nil)

		err = orch.Execute(ctx, DryRunConfig{CIOutput: false})
		require.NoError(t, err)

		goreleaserSvc.AssertExpectations(t)
		githubRepo.AssertExpectations(t)
	})

	t.Run("Should validate NPM packages when tools directory exists", func(t *testing.T) {
		ctx := context.Background()
		fsRepo := afero.NewMemMapFs()
		toolsDir := "tools"

		gitRepo := new(mockGitExtendedRepository)
		githubRepo := new(mockGithubExtendedRepository)
		cliffSvc := new(mockCliffService)
		goreleaserSvc := new(mockGoReleaserService)

		orch := NewDryRunOrchestrator(gitRepo, githubRepo, cliffSvc, goreleaserSvc, fsRepo, toolsDir)

		t.Setenv("GITHUB_HEAD_REF", "release/v1.2.3")

		// Create tools directory with package.json files
		err := fsRepo.MkdirAll("tools/tool1", 0755)
		require.NoError(t, err)
		err = afero.WriteFile(fsRepo, "tools/tool1/package.json", []byte(`{"name":"tool1","version":"0.0.0"}`), 0644)
		require.NoError(t, err)

		goreleaserSvc.On("Run", mock.Anything, "release", "--snapshot", "--skip=publish", "--clean").Return(nil)

		err = orch.Execute(ctx, DryRunConfig{CIOutput: false})
		require.NoError(t, err)

		// Verify package.json was updated with correct version
		content, err := afero.ReadFile(fsRepo, "tools/tool1/package.json")
		require.NoError(t, err)
		assert.Contains(t, string(content), `"version": "1.2.3"`)

		goreleaserSvc.AssertExpectations(t)
	})
}
