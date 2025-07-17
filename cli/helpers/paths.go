package helpers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

// loadEnvironmentFile loads environment variables from a file with security validation
// This function is context-independent and loads .env files into OS environment
// before configuration system initialization
func LoadEnvironmentFile(cmd *cobra.Command) error {
	// Get env file path from command flag
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		return fmt.Errorf("failed to get env-file flag: %w", err)
	}

	// If no env file specified, try default .env
	if envFile == "" {
		envFile = ".env"
	}

	// Get the current working directory for path resolution
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Resolve env file path relative to current working directory
	if !filepath.IsAbs(envFile) {
		envFile = filepath.Join(pwd, envFile)
	}

	// Security: Validate the resolved path to prevent directory traversal
	cleanPath := filepath.Clean(envFile)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve env file path: %w", err)
	}

	// Ensure the file is within the project directory or its subdirectories
	// This prevents accessing files like /etc/passwd via ../../../etc/passwd
	if !isPathWithinDirectory(absPath, pwd) {
		return fmt.Errorf("env file path '%s' is outside the project directory", envFile)
	}

	// Check if file exists and is a regular file
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, which is allowed
			return nil
		}
		return fmt.Errorf("failed to stat env file: %w", err)
	}

	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("env file path '%s' is not a regular file", envFile)
	}

	// Load environment variables into OS environment
	if err := godotenv.Load(absPath); err != nil {
		return fmt.Errorf("failed to load env file %s: %w", absPath, err)
	}

	return nil
}

// isPathWithinDirectory checks if a given path is within the specified directory
func isPathWithinDirectory(path, dir string) bool {
	absPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return false
	}
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return false
	}

	// Use filepath.Rel for more robust validation
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}

	// Check if the relative path starts with ".." (outside directory)
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
