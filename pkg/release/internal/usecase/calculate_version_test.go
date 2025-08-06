package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalculateVersionUseCase_Execute(t *testing.T) {
	t.Run("Should calculate next version from latest tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		expectedVer, _ := domain.NewVersion("v1.1.0")
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		cliffSvc.On("CalculateNextVersion", ctx, "v1.0.0").Return(expectedVer, nil)
		version, err := uc.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedVer, version)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should calculate initial version when no tag exists", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		expectedVer, _ := domain.NewVersion("v0.1.0")
		gitRepo.On("LatestTag", ctx).Return("", nil)
		cliffSvc.On("CalculateNextVersion", ctx, "").Return(expectedVer, nil)
		version, err := uc.Execute(ctx)
		require.NoError(t, err)
		assert.Equal(t, expectedVer, version)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should handle error when getting latest tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		expectedErr := errors.New("git error")
		gitRepo.On("LatestTag", ctx).Return("", expectedErr)
		version, err := uc.Execute(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get latest tag")
		assert.Nil(t, version)
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when calculating next version", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		expectedErr := errors.New("cliff error")
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		cliffSvc.On("CalculateNextVersion", ctx, "v1.0.0").Return((*domain.Version)(nil), expectedErr)
		version, err := uc.Execute(ctx)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, version)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
}
