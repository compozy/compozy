package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) (string, *git.Repository) {
	dir, err := os.MkdirTemp("", "git-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)
	// Create initial commit
	wt, err := repo.Worktree()
	require.NoError(t, err)
	testFile := filepath.Join(dir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err)
	_, err = wt.Add("test.txt")
	require.NoError(t, err)
	_, err = wt.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
		},
	})
	require.NoError(t, err)
	return dir, repo
}

func TestNewGitRepository(t *testing.T) {
	t.Run("Should create git repository for existing repo", func(t *testing.T) {
		dir, _ := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo, err := NewGitRepository()
		assert.NoError(t, err)
		assert.NotNil(t, gitRepo)
	})
	t.Run("Should return error for non-git directory", func(t *testing.T) {
		dir, err := os.MkdirTemp("", "non-git-*")
		require.NoError(t, err)
		defer os.RemoveAll(dir)
		oldPwd, _ := os.Getwd()
		err = os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo, err := NewGitRepository()
		assert.Error(t, err)
		assert.Nil(t, gitRepo)
	})
}

func TestGitRepository_LatestTag(t *testing.T) {
	t.Run("Should return latest tag when tags exist", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		// Create a tag
		head, err := repo.Head()
		require.NoError(t, err)
		_, err = repo.CreateTag("v1.0.0", head.Hash(), &git.CreateTagOptions{
			Message: "Release v1.0.0",
		})
		require.NoError(t, err)
		gitRepo := &gitRepository{repo: repo}
		tag, err := gitRepo.LatestTag(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "v1.0.0", tag)
	})
	t.Run("Should return empty string when no tags exist", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		tag, err := gitRepo.LatestTag(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "", tag)
	})
}

func TestGitRepository_CreateTag(t *testing.T) {
	t.Run("Should create tag successfully", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		err = gitRepo.CreateTag(context.Background(), "v1.0.0", "Release v1.0.0")
		assert.NoError(t, err)
		// Verify tag was created
		_, err = repo.Tag("v1.0.0")
		assert.NoError(t, err)
	})
	t.Run("Should return error for duplicate tag", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		err = gitRepo.CreateTag(context.Background(), "v1.0.0", "Release v1.0.0")
		require.NoError(t, err)
		err = gitRepo.CreateTag(context.Background(), "v1.0.0", "Release v1.0.0")
		assert.Error(t, err)
	})
}

func TestGitRepository_TagExists(t *testing.T) {
	t.Run("Should return true when tag exists", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		head, err := repo.Head()
		require.NoError(t, err)
		_, err = repo.CreateTag("v1.0.0", head.Hash(), nil)
		require.NoError(t, err)
		gitRepo := &gitRepository{repo: repo}
		exists, err := gitRepo.TagExists(context.Background(), "v1.0.0")
		assert.NoError(t, err)
		assert.True(t, exists)
	})
	t.Run("Should return false when tag does not exist", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		exists, err := gitRepo.TagExists(context.Background(), "v1.0.0")
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

func TestGitRepository_CreateBranch(t *testing.T) {
	t.Run("Should create branch successfully", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		err = gitRepo.CreateBranch(context.Background(), "feature/test")
		assert.NoError(t, err)
		// Verify branch was created
		ref, err := repo.Reference(plumbing.NewBranchReferenceName("feature/test"), false)
		assert.NoError(t, err)
		assert.NotNil(t, ref)
	})
	t.Run("Should return error for duplicate branch", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		err = gitRepo.CreateBranch(context.Background(), "feature/test")
		require.NoError(t, err)
		err = gitRepo.CreateBranch(context.Background(), "feature/test")
		assert.Error(t, err)
	})
}

func TestGitRepository_CommitsSinceTag(t *testing.T) {
	t.Run("Should count commits since tag", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		// Create a tag
		head, err := repo.Head()
		require.NoError(t, err)
		_, err = repo.CreateTag("v1.0.0", head.Hash(), nil)
		require.NoError(t, err)
		// Add more commits
		wt, err := repo.Worktree()
		require.NoError(t, err)
		testFile := filepath.Join(dir, "test2.txt")
		err = os.WriteFile(testFile, []byte("test content 2"), 0644)
		require.NoError(t, err)
		_, err = wt.Add("test2.txt")
		require.NoError(t, err)
		_, err = wt.Commit("Second commit", &git.CommitOptions{
			Author: &object.Signature{
				Name:  "Test User",
				Email: "test@example.com",
			},
		})
		require.NoError(t, err)
		gitRepo := &gitRepository{repo: repo}
		count, err := gitRepo.CommitsSinceTag(context.Background(), "v1.0.0")
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
	t.Run("Should return error for non-existent tag", func(t *testing.T) {
		dir, repo := setupTestRepo(t)
		oldPwd, _ := os.Getwd()
		err := os.Chdir(dir)
		require.NoError(t, err)
		defer os.Chdir(oldPwd)
		gitRepo := &gitRepository{repo: repo}
		count, err := gitRepo.CommitsSinceTag(context.Background(), "v999.0.0")
		assert.Error(t, err)
		assert.Equal(t, 0, count)
	})
}
