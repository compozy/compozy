package ref

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	// MaxResolvedDataSize is the maximum size of resolved data (50MB)
	MaxResolvedDataSize = 50 << 20
	// MaxURLRedirects is the maximum number of redirects allowed
	MaxURLRedirects = 5
)

// validateFilePath validates that a file path is safe and doesn't escape the allowed directory.
func validateFilePath(path, baseDir string) error {
	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// If the path is absolute, allow it but with basic validation
	if filepath.IsAbs(cleanPath) {
		return validateAbsolutePath(cleanPath)
	}

	// For relative paths, resolve against base directory
	resolvedPath, resolvedBase, err := resolveRelativePath(cleanPath, baseDir)
	if err != nil {
		return err
	}

	// Find the project root
	projectRoot := findProjectRoot(resolvedBase)

	// Allow paths within the project root
	if strings.HasPrefix(resolvedPath, projectRoot) {
		return validateProjectPath(resolvedPath, projectRoot, path)
	}

	// Check if the path is within blocked system directories
	return validateSystemPath(resolvedPath, path)
}

// validateAbsolutePath validates an absolute path
func validateAbsolutePath(cleanPath string) error {
	if cleanPath == "/" || cleanPath == "" {
		return errors.Errorf("invalid absolute path: %s", cleanPath)
	}
	return nil
}

// resolveRelativePath resolves a relative path against the base directory
func resolveRelativePath(cleanPath, baseDir string) (string, string, error) {
	fullPath := filepath.Join(baseDir, cleanPath)
	resolvedPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to resolve path: %s", cleanPath)
	}

	// Get the absolute base directory
	resolvedBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", "", errors.Wrapf(err, "failed to resolve base directory: %s", baseDir)
	}

	return resolvedPath, resolvedBase, nil
}

// findProjectRoot finds the project root directory
func findProjectRoot(resolvedBase string) string {
	projectRoot := resolvedBase
	for current := resolvedBase; current != "/" && current != ""; current = filepath.Dir(current) {
		// Check for common project root indicators
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, "compozy.yaml")); err == nil {
			return current
		}
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		// Stop if we've gone too far up
		if current == filepath.Dir(current) {
			break
		}
	}
	return projectRoot
}

// validateProjectPath validates a path within the project
func validateProjectPath(resolvedPath, projectRoot, originalPath string) error {
	// Additional check: prevent access to sensitive directories within project
	relativePath := strings.TrimPrefix(resolvedPath, projectRoot)
	lowerPath := strings.ToLower(relativePath)

	sensitivePatterns := []string{
		"/.git/",
		"/.ssh/",
		"/node_modules/",
		"/.env",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerPath, pattern) {
			return errors.Errorf("access to sensitive directory denied: %s", originalPath)
		}
	}

	return nil
}

// validateSystemPath checks if the path is within system directories
func validateSystemPath(resolvedPath, originalPath string) error {
	lowerPath := strings.ToLower(resolvedPath)
	blockedPrefixes := []string{
		"/etc/",
		"/sys/",
		"/proc/",
		"/private/etc/",
		"/private/var/",
		"/usr/bin/",
		"/usr/sbin/",
		"/bin/",
		"/sbin/",
	}

	for _, prefix := range blockedPrefixes {
		if strings.HasPrefix(lowerPath, prefix) {
			return errors.Errorf("access to system directory denied: %s", originalPath)
		}
	}

	// If the path is outside the project root but not in a system directory,
	// it might be a valid reference (e.g., to a shared library)
	// Log a warning but allow it
	return nil
}

// validateURL validates that a URL is safe to fetch.
func validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return errors.Wrapf(err, "invalid URL: %s", urlStr)
	}

	// Only allow HTTP and HTTPS schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	// Validate host is not empty
	if parsedURL.Host == "" {
		return errors.New("URL host cannot be empty")
	}

	// Check for localhost/private IPs (basic check)
	host := strings.ToLower(parsedURL.Host)
	if strings.Contains(host, "localhost") || strings.Contains(host, "127.0.0.1") || strings.Contains(host, "0.0.0.0") {
		return errors.Errorf("URL points to local host: %s", host)
	}

	// Check for private IP ranges (basic check - could be expanded)
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "172.") {
		return errors.Errorf("URL points to private network: %s", host)
	}

	return nil
}

// validateDataSize checks if the data size is within acceptable limits.
func validateDataSize(data []byte) error {
	if len(data) > MaxResolvedDataSize {
		return errors.Errorf("resolved data size %d exceeds maximum allowed size %d", len(data), MaxResolvedDataSize)
	}
	return nil
}
