package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateGitTagUseCase_Execute(t *testing.T) {
	t.Run("Should create and push tag successfully", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateGitTagUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		tagName := "v1.0.0"
		message := "Release v1.0.0"
		gitRepo.On("CreateTag", ctx, tagName, message).Return(nil)
		gitRepo.On("PushTag", ctx, tagName).Return(nil)
		err := uc.Execute(ctx, tagName, message)
		require.NoError(t, err)
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when creating tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateGitTagUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		tagName := "v1.0.0"
		message := "Release v1.0.0"
		expectedErr := errors.New("tag already exists")
		gitRepo.On("CreateTag", ctx, tagName, message).Return(expectedErr)
		err := uc.Execute(ctx, tagName, message)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create git tag")
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when pushing tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		uc := &CreateGitTagUseCase{
			GitRepo: gitRepo,
		}
		ctx := context.Background()
		tagName := "v1.0.0"
		message := "Release v1.0.0"
		expectedErr := errors.New("push failed")
		gitRepo.On("CreateTag", ctx, tagName, message).Return(nil)
		gitRepo.On("PushTag", ctx, tagName).Return(expectedErr)
		err := uc.Execute(ctx, tagName, message)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		gitRepo.AssertExpectations(t)
	})
}
