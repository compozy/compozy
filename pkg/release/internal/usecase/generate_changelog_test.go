package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateChangelogUseCase_Execute(t *testing.T) {
	t.Run("Should generate changelog for initial release", func(t *testing.T) {
		cliffSvc := new(mockCliffService)
		uc := &GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		version := "v1.0.0"
		mode := "initial"
		expectedChangelog := "# Changelog\n\n## v1.0.0\n\n- Initial release"
		cliffSvc.On("GenerateChangelog", ctx, version, mode).Return(expectedChangelog, nil)
		changelog, err := uc.Execute(ctx, version, mode)
		require.NoError(t, err)
		assert.Equal(t, expectedChangelog, changelog)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should generate changelog for release mode", func(t *testing.T) {
		cliffSvc := new(mockCliffService)
		uc := &GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		version := "v2.0.0"
		mode := "release"
		expectedChangelog := "## v2.0.0\n\n### Features\n- New feature"
		cliffSvc.On("GenerateChangelog", ctx, version, mode).Return(expectedChangelog, nil)
		changelog, err := uc.Execute(ctx, version, mode)
		require.NoError(t, err)
		assert.Equal(t, expectedChangelog, changelog)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should generate changelog for update mode", func(t *testing.T) {
		cliffSvc := new(mockCliffService)
		uc := &GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		version := "v1.1.0"
		mode := "update"
		expectedChangelog := "## v1.1.0\n\n### Bug Fixes\n- Fixed bug"
		cliffSvc.On("GenerateChangelog", ctx, version, mode).Return(expectedChangelog, nil)
		changelog, err := uc.Execute(ctx, version, mode)
		require.NoError(t, err)
		assert.Equal(t, expectedChangelog, changelog)
		cliffSvc.AssertExpectations(t)
	})
	t.Run("Should handle error from cliff service", func(t *testing.T) {
		cliffSvc := new(mockCliffService)
		uc := &GenerateChangelogUseCase{
			CliffSvc: cliffSvc,
		}
		ctx := context.Background()
		version := "v1.0.0"
		mode := "initial"
		expectedErr := errors.New("cliff error")
		cliffSvc.On("GenerateChangelog", ctx, version, mode).Return("", expectedErr)
		changelog, err := uc.Execute(ctx, version, mode)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Empty(t, changelog)
		cliffSvc.AssertExpectations(t)
	})
}
