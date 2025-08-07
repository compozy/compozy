package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateReleaseBranchUseCase_Execute(t *testing.T) {
	t.Run("Should create and push branch successfully", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		branchName := "release/v1.0.0"
		gitRepo.On("CreateBranch", ctx, branchName).Return(nil)
		gitRepo.On("PushBranch", ctx, branchName).Return(nil)
		err := uc.Execute(ctx, branchName)
		require.NoError(t, err)
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when creating branch", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		branchName := "release/v1.0.0"
		expectedErr := errors.New("branch already exists")
		gitRepo.On("CreateBranch", ctx, branchName).Return(expectedErr)
		err := uc.Execute(ctx, branchName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create release branch")
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when pushing branch", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateReleaseBranchUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		branchName := "release/v1.0.0"
		expectedErr := errors.New("push failed")
		gitRepo.On("CreateBranch", ctx, branchName).Return(nil)
		gitRepo.On("PushBranch", ctx, branchName).Return(expectedErr)
		err := uc.Execute(ctx, branchName)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		gitRepo.AssertExpectations(t)
	})
}
