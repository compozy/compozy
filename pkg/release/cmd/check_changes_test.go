package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
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

func TestNewCheckChangesCmd(t *testing.T) {
	t.Run("Should create check-changes command", func(t *testing.T) {
		uc := &usecase.CheckChangesUseCase{}
		cmd := NewCheckChangesCmd(uc)
		assert.NotNil(t, cmd)
		assert.Equal(t, "check-changes", cmd.Use)
		assert.Equal(t, "Check if there are pending changes for a new release", cmd.Short)
	})
	t.Run("Should execute successfully when changes exist", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		cmd := NewCheckChangesCmd(uc)
		ctx := context.Background()
		nextVer, _ := domain.NewVersion("v1.1.0")
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		gitRepo.On("CommitsSinceTag", ctx, "v1.0.0").Return(5, nil)
		cliffSvc.On("CalculateNextVersion", ctx, "v1.0.0").Return(nextVer, nil)
		// Capture output
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetContext(ctx)
		err := cmd.Execute()
		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "Has changes: true")
		assert.Contains(t, output, "Latest tag: v1.0.0")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should execute successfully when no changes exist", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		cmd := NewCheckChangesCmd(uc)
		ctx := context.Background()
		gitRepo.On("LatestTag", ctx).Return("v1.0.0", nil)
		gitRepo.On("CommitsSinceTag", ctx, "v1.0.0").Return(0, nil)
		// Capture output
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetContext(ctx)
		err := cmd.Execute()
		require.NoError(t, err)
		output := buf.String()
		assert.Contains(t, output, "Has changes: false")
		assert.Contains(t, output, "Latest tag: v1.0.0")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should handle error from use case", func(t *testing.T) {
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CheckChangesUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}
		cmd := NewCheckChangesCmd(uc)
		ctx := context.Background()
		expectedErr := errors.New("git error")
		gitRepo.On("LatestTag", ctx).Return("", expectedErr)
		cmd.SetContext(ctx)
		err := cmd.Execute()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "git error")
		gitRepo.AssertExpectations(t)
	})
}
