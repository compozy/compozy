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
	envFile, err := cmd.Flags().GetString("env-file")
	if err != nil {
		envFile = ""
	}
	if envFile == "" {
		envFile = ".env"
	}
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	absPath, err := resolveEnvAbsolutePath(pwd, envFile)
	if err != nil {
		return err
	}
	fileInfo, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat env file: %w", err)
	}
	if !fileInfo.Mode().IsRegular() {
		return fmt.Errorf("env file path '%s' is not a regular file", envFile)
	}
	if err := godotenv.Load(absPath); err != nil {
		return fmt.Errorf("failed to load env file %s: %w", absPath, err)
	}
	return nil
}

func resolveEnvAbsolutePath(pwd, envFile string) (string, error) {
	candidate := envFile
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(pwd, candidate)
	}
	cleanPath := filepath.Clean(candidate)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve env file path: %w", err)
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to resolve env file symlink: %w", err)
		}
		resolvedPath = absPath
	}
	if !isPathWithinDirectory(resolvedPath, pwd) {
		return "", fmt.Errorf("env file path '%s' resolves outside the project directory", envFile)
	}
	return absPath, nil
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
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}
