package service

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/release/internal/domain"
)

// cliffService is the implementation of the CliffService interface.
type cliffService struct {
	// timeout for command execution
	timeout time.Duration
}

// NewCliffService creates a new CliffService.
func NewCliffService() CliffService {
	return &cliffService{
		timeout: DefaultCliffTimeout,
	}
}

// sanitizeTag validates and sanitizes a git tag to prevent command injection.
func (s *cliffService) sanitizeTag(tag string) error {
	if tag == "" {
		return nil // Empty tag is valid
	}
	// Allow only valid git tag characters: alphanumeric, dots, hyphens, underscores, slashes
	// and the 'v' prefix for version tags
	validTag := regexp.MustCompile(`^[a-zA-Z0-9._/\-]+$`)
	if !validTag.MatchString(tag) {
		return fmt.Errorf("invalid tag format: %s", tag)
	}
	// Prevent directory traversal
	if strings.Contains(tag, "..") {
		return fmt.Errorf("invalid tag: contains directory traversal")
	}
	// Limit tag length to prevent buffer overflow attacks
	if len(tag) > 255 {
		return fmt.Errorf("tag too long: maximum 255 characters")
	}
	return nil
}

// sanitizeVersion validates and sanitizes a version string.
func (s *cliffService) sanitizeVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	// Version should follow semantic versioning pattern
	validVersion := regexp.MustCompile(`^v?\d+\.\d+\.\d+(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	if !validVersion.MatchString(version) {
		return fmt.Errorf("invalid version format: %s", version)
	}
	if len(version) > 100 {
		return fmt.Errorf("version too long: maximum 100 characters")
	}
	return nil
}

// sanitizeMode validates the changelog generation mode.
func (s *cliffService) sanitizeMode(mode string) error {
	validModes := map[string]bool{
		"unreleased": true,
		"current":    true,
		"initial":    true,
		"release":    true,
		"update":     true,
		"":           true, // Empty mode defaults to current
	}
	if !validModes[mode] {
		return fmt.Errorf("invalid mode: %s", mode)
	}
	return nil
}

// executeCommand runs a command with timeout and proper resource cleanup.
func (s *cliffService) executeCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)

	// Capture both stdout and stderr for better error handling
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command timed out after %v", s.timeout)
		}
		// Include stderr in error message for debugging
		errMsg := stderr.String()
		if errMsg != "" {
			return nil, fmt.Errorf("command failed: %w (stderr: %s)", err, errMsg)
		}
		return nil, fmt.Errorf("command failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// CalculateNextVersion calculates the next version based on the commit history.
func (s *cliffService) CalculateNextVersion(ctx context.Context, latestTag string) (*domain.Version, error) {
	// Sanitize input to prevent command injection
	if err := s.sanitizeTag(latestTag); err != nil {
		return nil, fmt.Errorf("invalid latest tag: %w", err)
	}

	args := []string{"--bumped-version"}
	if latestTag != "" {
		args = append(args, "--tag", latestTag)
	}

	output, err := s.executeCommand(ctx, "git-cliff", args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute git-cliff: %w", err)
	}

	// Validate output before parsing
	versionStr := strings.TrimSpace(string(output))
	if versionStr == "" {
		return nil, fmt.Errorf("git-cliff returned empty version")
	}

	// Additional validation of the version string
	if err := s.sanitizeVersion(versionStr); err != nil {
		return nil, fmt.Errorf("git-cliff returned invalid version: %w", err)
	}

	return domain.NewVersion(versionStr)
}

// GenerateChangelog generates a changelog.
func (s *cliffService) GenerateChangelog(ctx context.Context, version, mode string) (string, error) {
	// Sanitize inputs to prevent command injection
	if version != "" {
		if err := s.sanitizeVersion(version); err != nil {
			return "", fmt.Errorf("invalid version: %w", err)
		}
	}
	if err := s.sanitizeMode(mode); err != nil {
		return "", fmt.Errorf("invalid mode: %w", err)
	}

	var args []string
	switch mode {
	case "unreleased", "update":
		// Generate changelog for unreleased changes (pending)
		args = []string{"--unreleased"}
	case "release":
		// Generate changelog for a specific release tag
		if version == "" {
			return "", fmt.Errorf("version required for release mode")
		}
		args = []string{"--tag", version}
	default:
		// Default to current/unreleased
		args = []string{"--unreleased"}
	}

	output, err := s.executeCommand(ctx, "git-cliff", args...)
	if err != nil {
		return "", fmt.Errorf("failed to execute git-cliff: %w", err)
	}

	// Validate output is not empty
	changelog := strings.TrimSpace(string(output))
	if changelog == "" {
		return "", fmt.Errorf("git-cliff returned empty changelog")
	}

	return changelog, nil
}
