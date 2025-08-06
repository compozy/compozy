package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCreateReleaseBranchCmd(t *testing.T) {
	t.Run("Should execute create release branch command successfully", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}

		gitRepo.On("CreateBranch", mock.Anything, "release/v1.0.0").Return(nil)
		gitRepo.On("PushBranch", mock.Anything, "release/v1.0.0").Return(nil)

		cmd := NewCreateReleaseBranchCmd(uc)
		cmd.SetArgs([]string{"--branch-name", "release/v1.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		gitRepo.AssertExpectations(t)
	})

	t.Run("Should require branch-name flag", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}

		cmd := NewCreateReleaseBranchCmd(uc)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required flag(s) \"branch-name\" not set")
	})

	t.Run("Should handle error when creating branch fails", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}

		gitRepo.On("CreateBranch", mock.Anything, "release/v1.0.0").Return(assert.AnError)

		cmd := NewCreateReleaseBranchCmd(uc)
		cmd.SetArgs([]string{"--branch-name", "release/v1.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		gitRepo.AssertExpectations(t)
	})

	t.Run("Should use provided context", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}

		ctx := context.Background()
		gitRepo.On("CreateBranch", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "hotfix/critical").Return(nil)
		gitRepo.On("PushBranch", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "hotfix/critical").Return(nil)

		cmd := NewCreateReleaseBranchCmd(uc)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"--branch-name", "hotfix/critical"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		gitRepo.AssertExpectations(t)
	})
}
