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

func TestNewGenerateChangelogCmd(t *testing.T) {
	t.Run("Should execute generate changelog command successfully", func(t *testing.T) {
		// Arrange
		cliffSvc := new(mockCliffService)
		uc := &usecase.GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}

		expectedChangelog := "## [1.0.0] - 2024-01-01\n### Added\n- New feature"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.0.0", "unreleased").Return(expectedChangelog, nil)

		cmd := NewGenerateChangelogCmd(uc)
		cmd.SetArgs([]string{"--version", "v1.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "## [1.0.0]")
		assert.Contains(t, buf.String(), "New feature")
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should use release mode when specified", func(t *testing.T) {
		// Arrange
		cliffSvc := new(mockCliffService)
		uc := &usecase.GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}

		expectedChangelog := "## [2.0.0] - 2024-01-01\n### Breaking Changes\n- Major update"
		cliffSvc.On("GenerateChangelog", mock.Anything, "v2.0.0", "release").Return(expectedChangelog, nil)

		cmd := NewGenerateChangelogCmd(uc)
		cmd.SetArgs([]string{"--version", "v2.0.0", "--mode", "release"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "## [2.0.0]")
		assert.Contains(t, buf.String(), "Breaking Changes")
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should require version flag", func(t *testing.T) {
		// Arrange
		cliffSvc := new(mockCliffService)
		uc := &usecase.GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}

		cmd := NewGenerateChangelogCmd(uc)
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		assert.Contains(t, err.Error(), "required flag(s) \"version\" not set")
	})

	t.Run("Should handle error when generating changelog fails", func(t *testing.T) {
		// Arrange
		cliffSvc := new(mockCliffService)
		uc := &usecase.GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}

		cliffSvc.On("GenerateChangelog", mock.Anything, "v1.0.0", "unreleased").Return("", assert.AnError)

		cmd := NewGenerateChangelogCmd(uc)
		cmd.SetArgs([]string{"--version", "v1.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.Error(t, err)
		cliffSvc.AssertExpectations(t)
	})

	t.Run("Should use provided context", func(t *testing.T) {
		// Arrange
		cliffSvc := new(mockCliffService)
		uc := &usecase.GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}

		ctx := context.Background()
		expectedChangelog := "## [3.0.0] - 2024-01-01"
		cliffSvc.On("GenerateChangelog", mock.MatchedBy(func(c context.Context) bool {
			return c != nil
		}), "v3.0.0", "unreleased").Return(expectedChangelog, nil)

		cmd := NewGenerateChangelogCmd(uc)
		cmd.SetContext(ctx)
		cmd.SetArgs([]string{"--version", "v3.0.0"})
		var buf bytes.Buffer
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)

		// Act
		err := cmd.Execute()

		// Assert
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "## [3.0.0]")
		cliffSvc.AssertExpectations(t)
	})
}
