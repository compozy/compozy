package orchestrator

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	// versionRegex matches semantic versions with optional 'v' prefix
	versionRegex = regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?(\+[a-zA-Z0-9.]+)?$`)
	// branchNameRegex matches valid git branch names
	branchNameRegex = regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
)

// ValidateVersion validates a semantic version string.
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("invalid version format: %s (expected: v1.2.3 or 1.2.3)", version)
	}
	return nil
}

// ValidateBranchName validates a git branch name.
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if len(branch) > 255 {
		return fmt.Errorf("branch name too long: %d characters (max: 255)", len(branch))
	}
	// Check for invalid patterns
	if strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") {
		return fmt.Errorf("branch name cannot start or end with slash: %s", branch)
	}
	if strings.Contains(branch, "..") {
		return fmt.Errorf("branch name cannot contain consecutive dots: %s", branch)
	}
	if strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("branch name cannot end with .lock: %s", branch)
	}
	if !branchNameRegex.MatchString(branch) {
		return fmt.Errorf("invalid branch name format: %s", branch)
	}
	return nil
}

// ValidateEnvironmentVariables checks for required environment variables.
func ValidateEnvironmentVariables(requiredVars []string) error {
	var missing []string
	for _, v := range requiredVars {
		if value := os.Getenv(v); value == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
