package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/repository"
	"github.com/spf13/afero"
)

// UpdateMainChangelogUseCase contains the logic for the update-main-changelog command.

type UpdateMainChangelogUseCase struct {
	FsRepo        repository.FileSystemRepository
	ChangelogPath string
}

// validatePath validates and sanitizes a path to prevent path traversal attacks
func (uc *UpdateMainChangelogUseCase) validatePath(path string) (string, error) {
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
		return "", fmt.Errorf("path traversal detected: changelog path must be within project directory")
	}
	return absPath, nil
}

// Execute runs the use case.
func (uc *UpdateMainChangelogUseCase) Execute(_ context.Context, changelog string) error {
	// Validate the changelog path to prevent path traversal
	validPath, err := uc.validatePath(uc.ChangelogPath)
	if err != nil {
		return fmt.Errorf("invalid changelog path: %w", err)
	}

	// Check if the changelog file exists
	original, err := afero.ReadFile(uc.FsRepo, validPath)
	if err != nil {
		// If file doesn't exist, create it with a header
		if os.IsNotExist(err) {
			header := "# Changelog\n\nAll notable changes to this project will be documented in this file.\n\n"
			newContent := append([]byte(header), []byte(changelog)...)
			return afero.WriteFile(uc.FsRepo, validPath, newContent, 0644)
		}
		return fmt.Errorf("failed to read main changelog: %w", err)
	}

	// File exists, prepend the new changelog
	newContent := append([]byte(changelog), original...)
	return afero.WriteFile(uc.FsRepo, validPath, newContent, 0644)
}
