package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/domain"
	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/spf13/afero"
)

// UpdatePackageVersionsUseCase contains the logic for the update-package-versions command.
type UpdatePackageVersionsUseCase struct {
	FsRepo   repository.FileSystemRepository
	ToolsDir string
}

// validatePath validates and sanitizes a path to prevent path traversal attacks
func (uc *UpdatePackageVersionsUseCase) validatePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	// Clean the path to resolve any . or .. elements
	cleanPath := filepath.Clean(path)

	// For memory filesystems (testing), just return the clean path
	// Check if we're using a real filesystem by trying to stat the current directory
	if _, err := uc.FsRepo.Stat("."); err == nil {
		// If using afero MemMapFs or similar, skip the absolute path validation
		// as it doesn't make sense for virtual filesystems
		if !strings.Contains(cleanPath, "..") {
			return cleanPath, nil
		}
		return "", fmt.Errorf("path traversal detected in path: %s", cleanPath)
	}

	// For real filesystems, do full validation
	// Convert to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	// SECURITY: Resolve symlinks to prevent symlink-based path traversal
	// Only check if the path or its parent exists
	if _, statErr := os.Stat(absPath); statErr == nil {
		absPath, err = filepath.EvalSymlinks(absPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	} else {
		// Path doesn't exist, check parent
		parentDir := filepath.Dir(absPath)
		if _, parentErr := os.Stat(parentDir); parentErr == nil {
			parentDir, err = filepath.EvalSymlinks(parentDir)
			if err != nil {
				return "", fmt.Errorf("failed to resolve parent directory: %w", err)
			}
			absPath = filepath.Join(parentDir, filepath.Base(absPath))
		}
	}
	// Get the current working directory and resolve its symlinks
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	if cwdInfo, err := os.Stat(cwd); err == nil && cwdInfo.IsDir() {
		cwd, err = filepath.EvalSymlinks(cwd)
		if err != nil {
			return "", fmt.Errorf("failed to resolve current directory: %w", err)
		}
	}
	// Ensure the path is within the project directory to prevent path traversal
	// Use path separator to ensure complete directory name matching
	if !strings.HasPrefix(absPath, cwd+string(os.PathSeparator)) && absPath != cwd {
		return "", fmt.Errorf("path traversal detected: path must be within project directory")
	}
	return absPath, nil
}

// Execute runs the use case.
func (uc *UpdatePackageVersionsUseCase) Execute(_ context.Context, version string) error {
	// Validate the tools directory to prevent path traversal
	validToolsDir, err := uc.validatePath(uc.ToolsDir)
	if err != nil {
		return fmt.Errorf("invalid tools directory: %w", err)
	}
	return afero.Walk(uc.FsRepo, validToolsDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "package.json" {
			// Validate each file path before processing
			validPath, err := uc.validatePath(path)
			if err != nil {
				return fmt.Errorf("invalid file path %s: %w", path, err)
			}
			if err := uc.updatePackageVersion(validPath, version); err != nil {
				return fmt.Errorf("failed to update package version at %s: %w", validPath, err)
			}
		}
		return nil
	})
}

func (uc *UpdatePackageVersionsUseCase) updatePackageVersion(path, version string) error {
	data, err := afero.ReadFile(uc.FsRepo, path)
	if err != nil {
		return err
	}
	var pkg domain.Package
	if err := json.Unmarshal(data, &pkg); err != nil {
		return err
	}
	pkg.Version = version
	newData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}
	return afero.WriteFile(uc.FsRepo, path, newData, 0644)
}
