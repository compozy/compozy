package usecase

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePackageVersionsUseCase_Execute(t *testing.T) {
	t.Run("Should update version in all package.json files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		toolsDir := "tools"
		// Create directory structure
		require.NoError(t, fs.MkdirAll(filepath.Join(toolsDir, "tool1"), 0755))
		require.NoError(t, fs.MkdirAll(filepath.Join(toolsDir, "tool2"), 0755))
		// Create package.json files
		pkg1 := domain.Package{Name: "tool1", Version: "1.0.0"}
		data1, _ := json.MarshalIndent(pkg1, "", "  ")
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "tool1", "package.json"), data1, 0644))
		pkg2 := domain.Package{Name: "tool2", Version: "1.0.0"}
		data2, _ := json.MarshalIndent(pkg2, "", "  ")
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "tool2", "package.json"), data2, 0644))
		uc := &UpdatePackageVersionsUseCase{
			FsRepo:   fs,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		newVersion := "2.0.0"
		err := uc.Execute(ctx, newVersion)
		require.NoError(t, err)
		// Verify versions were updated
		content1, err := afero.ReadFile(fs, filepath.Join(toolsDir, "tool1", "package.json"))
		require.NoError(t, err)
		var updatedPkg1 domain.Package
		require.NoError(t, json.Unmarshal(content1, &updatedPkg1))
		assert.Equal(t, newVersion, updatedPkg1.Version)
		content2, err := afero.ReadFile(fs, filepath.Join(toolsDir, "tool2", "package.json"))
		require.NoError(t, err)
		var updatedPkg2 domain.Package
		require.NoError(t, json.Unmarshal(content2, &updatedPkg2))
		assert.Equal(t, newVersion, updatedPkg2.Version)
	})
	t.Run("Should skip non-package.json files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		toolsDir := "tools"
		require.NoError(t, fs.MkdirAll(toolsDir, 0755))
		// Create non-package.json files
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "README.md"), []byte("# README"), 0644))
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "config.json"), []byte("{}"), 0644))
		uc := &UpdatePackageVersionsUseCase{
			FsRepo:   fs,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		err := uc.Execute(ctx, "1.0.0")
		require.NoError(t, err)
		// Verify other files remain unchanged
		content, err := afero.ReadFile(fs, filepath.Join(toolsDir, "README.md"))
		require.NoError(t, err)
		assert.Equal(t, "# README", string(content))
	})
	t.Run("Should handle empty tools directory", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		toolsDir := "tools"
		require.NoError(t, fs.MkdirAll(toolsDir, 0755))
		uc := &UpdatePackageVersionsUseCase{
			FsRepo:   fs,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		err := uc.Execute(ctx, "1.0.0")
		assert.NoError(t, err)
	})
	t.Run("Should handle nested package.json files", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		toolsDir := "tools"
		// Create nested directory structure
		nestedPath := filepath.Join(toolsDir, "tool1", "submodule")
		require.NoError(t, fs.MkdirAll(nestedPath, 0755))
		// Create nested package.json
		pkg := domain.Package{Name: "submodule", Version: "0.5.0"}
		data, _ := json.MarshalIndent(pkg, "", "  ")
		require.NoError(t, afero.WriteFile(fs, filepath.Join(nestedPath, "package.json"), data, 0644))
		uc := &UpdatePackageVersionsUseCase{
			FsRepo:   fs,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		newVersion := "1.0.0"
		err := uc.Execute(ctx, newVersion)
		require.NoError(t, err)
		// Verify nested package.json was updated
		content, err := afero.ReadFile(fs, filepath.Join(nestedPath, "package.json"))
		require.NoError(t, err)
		var updatedPkg domain.Package
		require.NoError(t, json.Unmarshal(content, &updatedPkg))
		assert.Equal(t, newVersion, updatedPkg.Version)
	})
	t.Run("Should preserve package.json formatting", func(t *testing.T) {
		fs := afero.NewMemMapFs()
		toolsDir := "tools"
		require.NoError(t, fs.MkdirAll(toolsDir, 0755))
		// Create package.json with specific formatting
		pkg := domain.Package{
			Name:    "test-tool",
			Version: "1.0.0",
			Private: true,
			Path:    filepath.Join(toolsDir, "package.json"),
		}
		data, _ := json.MarshalIndent(pkg, "", "  ")
		require.NoError(t, afero.WriteFile(fs, filepath.Join(toolsDir, "package.json"), data, 0644))
		uc := &UpdatePackageVersionsUseCase{
			FsRepo:   fs,
			ToolsDir: toolsDir,
		}
		ctx := context.Background()
		err := uc.Execute(ctx, "2.0.0")
		require.NoError(t, err)
		// Verify formatting is preserved
		content, err := afero.ReadFile(fs, filepath.Join(toolsDir, "package.json"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "  \"name\":")
		assert.Contains(t, string(content), "  \"version\": \"2.0.0\"")
		assert.Contains(t, string(content), "  \"private\":")
	})
}
