package utils

import (
	"fmt"
	"os"
	"path/filepath"
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

// GetTestEnvOrDefault returns the test environment variable value or a default value
func GetTestEnvOrDefault(key, defaultValue string) string {
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
