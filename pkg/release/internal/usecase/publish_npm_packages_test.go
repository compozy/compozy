package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock for NpmService
type mockNpmService struct {
	mock.Mock
}

func (m *mockNpmService) Publish(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func TestPublishNpmPackagesUseCase_Execute(t *testing.T) {
	t.Run("Should publish all tool directories", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		npmSvc := new(mockNpmService)
		toolsDir := "tools"
		// Create directory structure
		tool1Dir := filepath.Join(toolsDir, "tool1")
		tool2Dir := filepath.Join(toolsDir, "tool2")
		require.NoError(t, fs.MkdirAll(tool1Dir, 0755))
		require.NoError(t, fs.MkdirAll(tool2Dir, 0755))
		// Create some files
		require.NoError(t, afero.WriteFile(fs, filepath.Join(tool1Dir, "package.json"), []byte("{}"), 0644))
		require.NoError(t, afero.WriteFile(fs, filepath.Join(tool2Dir, "package.json"), []byte("{}"), 0644))
		uc := &PublishNpmPackagesUseCase{
			FsRepo:   fs,
			NpmSvc:   npmSvc,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		// Set up expectations
		npmSvc.On("Publish", ctx, tool1Dir).Return(nil)
		npmSvc.On("Publish", ctx, tool2Dir).Return(nil)
		err := uc.Execute(ctx)
		require.NoError(t, err)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should skip root tools directory", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		npmSvc := new(mockNpmService)
		toolsDir := "tools"
		// Create only root directory with file
		require.NoError(t, fs.MkdirAll(toolsDir, 0755))
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "README.md"), []byte("# Tools"), 0644))
		uc := &PublishNpmPackagesUseCase{
			FsRepo:   fs,
			NpmSvc:   npmSvc,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		// Should not call Publish for root directory
		err := uc.Execute(ctx)
		require.NoError(t, err)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should handle publish error", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		npmSvc := new(mockNpmService)
		toolsDir := "tools"
		toolDir := filepath.Join(toolsDir, "failing-tool")
		require.NoError(t, fs.MkdirAll(toolDir, 0755))
		uc := &PublishNpmPackagesUseCase{
			FsRepo:   fs,
			NpmSvc:   npmSvc,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		expectedErr := errors.New("npm publish failed")
		npmSvc.On("Publish", ctx, toolDir).Return(expectedErr)
		err := uc.Execute(ctx)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to publish package")
		assert.Contains(t, err.Error(), toolDir)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should handle nested directories", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		npmSvc := new(mockNpmService)
		toolsDir := "tools"
		// Create nested structure
		tool1Dir := filepath.Join(toolsDir, "tool1")
		nestedDir := filepath.Join(tool1Dir, "nested")
		require.NoError(t, fs.MkdirAll(nestedDir, 0755))
		uc := &PublishNpmPackagesUseCase{
			FsRepo:   fs,
			NpmSvc:   npmSvc,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		// Set up expectations for all directories except root
		npmSvc.On("Publish", ctx, tool1Dir).Return(nil)
		npmSvc.On("Publish", ctx, nestedDir).Return(nil)
		err := uc.Execute(ctx)
		require.NoError(t, err)
		npmSvc.AssertExpectations(t)
	})
	t.Run("Should handle empty tools directory", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		npmSvc := new(mockNpmService)
		toolsDir := "tools"
		require.NoError(t, fs.MkdirAll(toolsDir, 0755))
		uc := &PublishNpmPackagesUseCase{
			FsRepo:   fs,
			NpmSvc:   npmSvc,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		err := uc.Execute(ctx)
		assert.NoError(t, err)
		npmSvc.AssertNotCalled(t, "Publish")
	})
}
