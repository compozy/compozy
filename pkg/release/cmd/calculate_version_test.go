package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/usecase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCalculateVersionCmd(t *testing.T) {
	t.Run("Should execute calculate version command successfully", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}

		expectedVersion, _ := domain.NewVersion("v1.1.0")
		gitRepo.On("LatestTag", mock.Anything).Return("v1.0.0", nil)
		cliffSvc.On("CalculateNextVersion", mock.Anything, "v1.0.0").Return(expectedVersion, nil)

		cmd := NewCalculateVersionCmd(uc)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Next version: v1.1.0")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should handle error when calculating version fails", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}

		gitRepo.On("LatestTag", mock.Anything).Return("", assert.AnError)

		cmd := NewCalculateVersionCmd(uc)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		gitRepo.AssertExpectations(t)
	})
}

func TestCalculateVersionCmd_Context(t *testing.T) {
	t.Run("Should use provided context", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		cliffSvc := new(mockCliffService)
		uc := &usecase.CalculateVersionUseCase{
			GitRepo:  gitRepo,
			CliffSvc: cliffSvc,
		}

		ctx := context.Background()
		expectedVersion, _ := domain.NewVersion("v2.0.0")

		gitRepo.On("LatestTag", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		})).Return("v1.9.0", nil)

		cliffSvc.On("CalculateNextVersion", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "v1.9.0").Return(expectedVersion, nil)

		cmd := NewCalculateVersionCmd(uc)
		cmd.SetContext(ctx)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "Next version: v2.0.0")
		gitRepo.AssertExpectations(t)
		cliffSvc.AssertExpectations(t)
	})
}
