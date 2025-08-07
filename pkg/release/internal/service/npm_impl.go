package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// npmService is the implementation of the NpmService interface.
type npmService struct {
	// timeout for command execution
	timeout time.Duration
}

// NewNpmService creates a new NpmService.
func NewNpmService() NpmService {
	return &npmService{
		timeout: 60 * time.Second, // Default 60 second timeout for npm operations
	}
}

// resolvePathWithSymlinks resolves a path and evaluates symlinks.
func (s *npmService) resolvePathWithSymlinks(path string) (string, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	// Resolve any symlinks
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If the path doesn't exist yet, EvalSymlinks will fail
		// Try to resolve the parent directory instead
		if os.IsNotExist(err) {
			parentDir := filepath.Dir(absPath)
			if parentDir != "." && parentDir != "/" {
				resolvedParent, err := filepath.EvalSymlinks(parentDir)
				if err != nil {
					return "", fmt.Errorf("failed to resolve parent directory symlinks: %w", err)
				}
				// Return the path with resolved parent
				return filepath.Join(resolvedParent, filepath.Base(absPath)), nil
			}
			return absPath, nil
		}
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}
	return resolvedPath, nil
}

// validatePathSecurity checks if a path is within the project directory.
func (s *npmService) validatePathSecurity(absPath, cwd string) error {
	// Use path separator to ensure we're checking complete directory names
	if !strings.HasPrefix(absPath, cwd+string(os.PathSeparator)) && absPath != cwd {
		return fmt.Errorf("path traversal detected: path must be within project directory")
	}
	return nil
}

// sanitizePath validates and sanitizes a filesystem path to prevent path traversal attacks.
func (s *npmService) sanitizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	// Clean the path to resolve any . or .. elements
	cleanPath := filepath.Clean(path)
	// SECURITY: Resolve symlinks to prevent symlink-based path traversal
	absPath, err := s.resolvePathWithSymlinks(cleanPath)
	if err != nil {
		return "", err
	}
	// Get the current working directory and resolve its symlinks
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory symlinks: %w", err)
	}
	// Validate that the path is within the project directory
	if err := s.validatePathSecurity(absPath, cwd); err != nil {
		return "", err
	}
	// Check if the path exists
	if _, err := os.Stat(absPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("path does not exist: %s", absPath)
		}
		return "", fmt.Errorf("failed to check path: %w", err)
	}
	// Check if package.json exists in the directory
	packageJSONPath := filepath.Join(absPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("package.json not found in directory: %s", absPath)
		}
		return "", fmt.Errorf("failed to check package.json: %w", err)
	}
	return absPath, nil
}

// executeCommand runs a command with timeout and proper resource cleanup.
func (s *npmService) executeCommand(ctx context.Context, dir string, name string, args ...string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	// Ensure NPM authentication works with both NPM_TOKEN and NODE_AUTH_TOKEN
	// GitHub Actions setup-node with registry-url uses NODE_AUTH_TOKEN
	// Standard npm CLI uses NPM_TOKEN
	cmd.Env = os.Environ()
	if npmToken := os.Getenv("NPM_TOKEN"); npmToken != "" && os.Getenv("NODE_AUTH_TOKEN") == "" {
		// If NPM_TOKEN is set but NODE_AUTH_TOKEN is not, set NODE_AUTH_TOKEN
		// This ensures compatibility with GitHub Actions setup-node
		cmd.Env = append(cmd.Env, "NODE_AUTH_TOKEN="+npmToken)
	}

	// Capture both stdout and stderr for better error handling
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("command timed out after %v", s.timeout)
		}
		// Include stderr in error message for debugging
		errMsg := stderr.String()
		if errMsg != "" {
			return fmt.Errorf("command failed: %w (stderr: %s)", err, errMsg)
		}
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

// Publish publishes an NPM package.
func (s *npmService) Publish(ctx context.Context, path string) error {
	// Sanitize and validate the path to prevent path traversal and command injection
	safePath, err := s.sanitizePath(path)
	if err != nil {
		return fmt.Errorf("invalid package path: %w", err)
	}

	// NPM_TOKEN is expected to be set as an environment variable
	// The npm CLI will automatically use it for authentication
	// Alternatively, ensure .npmrc is properly configured in CI

	// Execute npm publish with timeout and proper error handling
	if err := s.executeCommand(ctx, safePath, "npm", "publish", "--access", "public"); err != nil {
		return fmt.Errorf("failed to publish npm package at %s: %w", safePath, err)
	}

	return nil
}
