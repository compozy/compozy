package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock for GitRepository
type mockGitRepository struct {
	mock.Mock
}

func (m *mockGitRepository) LatestTag(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}

func (m *mockGitRepository) CommitsSinceTag(ctx context.Context, tag string) (int, error) {
	args := m.Called(ctx, tag)
	return args.Int(0), args.Error(1)
}

func (m *mockGitRepository) TagExists(ctx context.Context, tag string) (bool, error) {
	args := m.Called(ctx, tag)
	return args.Bool(0), args.Error(1)
}

func (m *mockGitRepository) CreateBranch(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *mockGitRepository) CreateTag(ctx context.Context, tag, msg string) error {
	args := m.Called(ctx, tag, msg)
	return args.Error(0)
}

func (m *mockGitRepository) PushTag(ctx context.Context, tag string) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *mockGitRepository) PushBranch(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

// Mock for CliffService
type mockCliffService struct {
	mock.Mock
}

func (m *mockCliffService) GenerateChangelog(ctx context.Context, version, mode string) (string, error) {
	args := m.Called(ctx, version, mode)
	return args.String(0), args.Error(1)
}

func (m *mockCliffService) CalculateNextVersion(ctx context.Context, currentVersion string) (*domain.Version, error) {
	args := m.Called(ctx, currentVersion)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Version), args.Error(1)
}

func TestCheckChangesUseCase_Execute(t *testing.T) {
	t.Run("Should detect changes when commits exist since tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		nextVer, _ := domain.NewVersion("v1.1.0")
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		gitRepo.On("CommitsSinceTag", ctx, "v1.0.0").Return(5, nil)
		cliffSvc.On("CalculateNextVersion", ctx, "v1.0.0").Return(nextVer, nil)
		hasChanges, latestTag, err := uc.Execute(ctx)
		require.NoError(t, err)
		assert.True(t, hasChanges)
		assert.Equal(t, "v1.0.0", latestTag)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should detect no changes when no commits since tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		gitRepo.On("CommitsSinceTag", ctx, "v1.0.0").Return(0, nil)
		hasChanges, latestTag, err := uc.Execute(ctx)
		require.NoError(t, err)
		assert.False(t, hasChanges)
		assert.Equal(t, "v1.0.0", latestTag)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should detect changes for initial release", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		gitRepo.On("LatestTag", ctx).Return("", nil)
		hasChanges, latestTag, err := uc.Execute(ctx)
		require.NoError(t, err)
		assert.True(t, hasChanges)
		assert.Equal(t, "", latestTag)
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should handle error when getting latest tag", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		expectedErr := errors.New("git error")
		gitRepo.On("LatestTag", ctx).Return("", expectedErr)
		hasChanges, latestTag, err := uc.Execute(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get latest tag")
		assert.Contains(t, err.Error(), "git error")
		assert.False(t, hasChanges)
		assert.Equal(t, "", latestTag)
		gitRepo.AssertExpectations(t)
	})
	t.Run("Should handle error when counting commits", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		expectedErr := errors.New("commit count error")
		gitRepo.On("CommitsSinceTag", ctx, "v1.0.0").Return(0, expectedErr)
		hasChanges, latestTag, err := uc.Execute(ctx)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.False(t, hasChanges)
		assert.Equal(t, "v1.0.0", latestTag)
		gitRepo.AssertExpectations(t)
	})
}
