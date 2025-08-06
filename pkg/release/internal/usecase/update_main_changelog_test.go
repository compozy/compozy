package usecase

import (
	"context"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateMainChangelogUseCase_Execute(t *testing.T) {
	t.Run("Should prepend new changelog to existing content", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		changelogPath := "CHANGELOG.md"
		existingContent := "# Changelog\n\n## v0.9.0\n- Previous release"
		err := afero.WriteFile(fs, changelogPath, []byte(existingContent), 0644)
		require.NoError(t, err)
		uc := &UpdateMainChangelogUseCase{
			FsRepo:        fs,
			ChangelogPath: changelogPath,
		}
		ctx := context.Background()
		newChangelog := "## v1.0.0\n- New release\n\n"
		err = uc.Execute(ctx, newChangelog)
		require.NoError(t, err)
		// Verify the content was prepended
		content, err := afero.ReadFile(fs, changelogPath)
		require.NoError(t, err)
		expectedContent := newChangelog + existingContent
		assert.Equal(t, expectedContent, string(content))
	})
	t.Run("Should handle empty existing changelog", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		changelogPath := "CHANGELOG.md"
		err := afero.WriteFile(fs, changelogPath, []byte(""), 0644)
		require.NoError(t, err)
		uc := &UpdateMainChangelogUseCase{
			FsRepo:        fs,
			ChangelogPath: changelogPath,
		}
		ctx := context.Background()
		newChangelog := "# Changelog\n\n## v1.0.0\n- Initial release"
		err = uc.Execute(ctx, newChangelog)
		require.NoError(t, err)
		// Verify the content was written
		content, err := afero.ReadFile(fs, changelogPath)
		require.NoError(t, err)
		assert.Equal(t, newChangelog, string(content))
	})
	t.Run("Should create changelog file if it doesn't exist", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		changelogPath := "CHANGELOG.md"
		uc := &UpdateMainChangelogUseCase{
			FsRepo:        fs,
			ChangelogPath: changelogPath,
		}
		ctx := context.Background()
		newChangelog := "## v1.0.0\n- New release"
		err := uc.Execute(ctx, newChangelog)
		require.NoError(t, err)
		// Verify the file was created with header and content
		content, err := afero.ReadFile(fs, changelogPath)
		require.NoError(t, err)
		expectedContent := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n## v1.0.0\n- New release"
		assert.Equal(t, expectedContent, string(content))
	})
	t.Run("Should preserve file permissions", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		changelogPath := "CHANGELOG.md"
		existingContent := "# Changelog"
		err := afero.WriteFile(fs, changelogPath, []byte(existingContent), 0644)
		require.NoError(t, err)
		uc := &UpdateMainChangelogUseCase{
			FsRepo:        fs,
			ChangelogPath: changelogPath,
		}
		ctx := context.Background()
		newChangelog := "## v1.0.0\n\n"
		err = uc.Execute(ctx, newChangelog)
		require.NoError(t, err)
		// Verify file still exists and is readable
		info, err := fs.Stat(changelogPath)
		require.NoError(t, err)
		assert.Equal(t, "-rw-r--r--", info.Mode().String())
	})
}
