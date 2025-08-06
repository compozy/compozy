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

func TestNewCreateGitTagCmd(t *testing.T) {
	t.Run("Should execute create git tag command successfully", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateGitTagUseCase{
			GitRepo: gitRepo,
		}

		gitRepo.On("CreateTag", mock.Anything, "v1.0.0", "Release v1.0.0").Return(nil)
		gitRepo.On("PushTag", mock.Anything, "v1.0.0").Return(nil)

		cmd := NewCreateGitTagCmd(uc)
		cmd.SetArgs([]string{"--tag-name", "v1.0.0", "--message", "Release v1.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		gitRepo.AssertExpectations(t)
	})

	t.Run("Should require tag-name flag", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateGitTagUseCase{
			GitRepo: gitRepo,
		}

		cmd := NewCreateGitTagCmd(uc)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required flag(s) \"tag-name\" not set")
	})

	t.Run("Should handle error when creating tag fails", func(t *testing.T) {
		// Arrange
		gitRepo := new(mockGitRepository)
		uc := &usecase.CreateGitTagUseCase{
			GitRepo: gitRepo,
		}

		gitRepo.On("CreateTag", mock.Anything, "v1.0.0", "").Return(assert.AnError)

		cmd := NewCreateGitTagCmd(uc)
		cmd.SetArgs([]string{"--tag-name", "v1.0.0"})
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
		uc := &usecase.CreateGitTagUseCase{
			GitRepo: gitRepo,
		}

		ctx := context.Background()
		gitRepo.On("CreateTag", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "v2.0.0", "Major release").Return(nil)
		gitRepo.On("PushTag", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "v2.0.0").Return(nil)

		cmd := NewCreateGitTagCmd(uc)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"--tag-name", "v2.0.0", "--message", "Major release"})
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
