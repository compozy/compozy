package utils

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// -----
// Common Pointer Utilities
// -----

// StringPtr returns a pointer to the string value
func StringPtr(s string) *string {
	return &s
}

// IDPtr creates a pointer to a core.ID for test convenience
func IDPtr(id string) *core.ID {
	coreID := core.ID(id)
	return &coreID
}

// -----
// Test Data Generation
// -----

// GenerateUniqueTestID creates a unique test identifier for data isolation
func GenerateUniqueTestID(testName string) string {
	return fmt.Sprintf("test-%s-%d", testName, time.Now().UnixNano())
}

// -----
// Environment Utilities
// -----

var (
	envLoadOnce sync.Once
	envLoadErr  error
)

// loadBranchEnv loads the branch-specific environment file if available
func loadBranchEnv() error {
	// Get current git branch
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		// If we can't get the branch, that's ok - fall back to defaults
		return nil
	}
	branchName := strings.TrimSpace(string(output))
	if branchName == "" {
		return nil
	}
	// Sanitize branch name to match docker script logic
	branchName = strings.ReplaceAll(branchName, "/", "_")
	branchName = strings.ReplaceAll(branchName, "-", "_")

	// Look for branch-specific env file
	projectRoot, err := FindProjectRoot()
	if err != nil {
		return nil // Not critical, use defaults
	}
	envFile := filepath.Join(projectRoot, fmt.Sprintf(".env.%s", branchName))
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return nil // No branch env file, use defaults
	}
	// Load the environment file
	file, err := os.Open(envFile)
	if err != nil {
		return nil // Not critical, use defaults
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Only set if not already set in environment
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
	return scanner.Err()
}

// GetTestEnvOrDefault returns the test environment variable value or a default value
// It automatically loads branch-specific environment if available
func GetTestEnvOrDefault(key, defaultValue string) string {
	envLoadOnce.Do(func() {
		envLoadErr = loadBranchEnv()
	})
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// -----
// Project Utilities
// -----

// FindProjectRoot finds the project root by looking for go.mod file
func FindProjectRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			return projectRoot, nil
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return "", fmt.Errorf("could not find project root (go.mod not found)")
		}
		projectRoot = parent
	}
}

// -----
// Test Constants
// -----

const (
	// DefaultActionID is the default action ID used in tests
	DefaultActionID = "default_action"

	// DefaultTestTimeout is the default timeout for test operations
	DefaultTestTimeout = 5 * time.Second

	// DefaultPollInterval is the default polling interval for eventually assertions
	DefaultPollInterval = 100 * time.Millisecond
)
