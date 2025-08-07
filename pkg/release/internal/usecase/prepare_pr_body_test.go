package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreparePRBodyUseCase_Execute(t *testing.T) {
	t.Run("Should prepare PR body with release information", func(t *testing.T) {
		uc := &PreparePRBodyUseCase{}
		ctx := context.Background()
		version, _ := domain.NewVersion("v1.0.0")
		release := &domain.Release{
			Version:   version,
			Changelog: "### Features\n- New feature\n### Bug Fixes\n- Fixed bug",
		}
		body, err := uc.Execute(ctx, release)
		require.NoError(t, err)
		assert.Contains(t, body, "Release v1.0.0")
		assert.Contains(t, body, "This PR prepares the release of version v1.0.0")
		assert.Contains(t, body, "### Features")
		assert.Contains(t, body, "- New feature")
		assert.Contains(t, body, "### Bug Fixes")
		assert.Contains(t, body, "- Fixed bug")
	})
	t.Run("Should handle empty changelog", func(t *testing.T) {
		uc := &PreparePRBodyUseCase{}
		ctx := context.Background()
		version, _ := domain.NewVersion("v0.1.0")
		release := &domain.Release{
			Version:   version,
			Changelog: "",
		}
		body, err := uc.Execute(ctx, release)
		require.NoError(t, err)
		assert.Contains(t, body, "Release v0.1.0")
		assert.Contains(t, body, "### Changelog")
		// Check that after Changelog header there's no content
		lines := strings.Split(body, "\n")
		changelogIndex := -1
		for i, line := range lines {
			if strings.Contains(line, "### Changelog") {
				changelogIndex = i
				break
			}
		}
		assert.NotEqual(t, -1, changelogIndex)
		// Next line after changelog should be empty or end of string
		if changelogIndex+1 < len(lines) {
			assert.Equal(t, "", strings.TrimSpace(lines[changelogIndex+1]))
		}
	})
	t.Run("Should format multi-line changelog correctly", func(t *testing.T) {
		uc := &PreparePRBodyUseCase{}
		ctx := context.Background()
		version, _ := domain.NewVersion("v2.0.0")
		release := &domain.Release{
			Version: version,
			Changelog: `## [2.0.0] - 2024-01-01

### Added
- New API endpoints
- Documentation improvements

### Changed
- Updated dependencies
- Refactored core module

### Fixed
- Memory leak in worker process
- Race condition in cache`,
		}
		body, err := uc.Execute(ctx, release)
		require.NoError(t, err)
		assert.Contains(t, body, "Release v2.0.0")
		assert.Contains(t, body, "### Added")
		assert.Contains(t, body, "- New API endpoints")
		assert.Contains(t, body, "### Changed")
		assert.Contains(t, body, "- Updated dependencies")
		assert.Contains(t, body, "### Fixed")
		assert.Contains(t, body, "- Memory leak in worker process")
	})
}
