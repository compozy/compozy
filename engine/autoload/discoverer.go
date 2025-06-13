package autoload

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/compozy/compozy/engine/core"
)

// FileDiscoverer interface for discovering configuration files
type FileDiscoverer interface {
	Discover(includes, excludes []string) ([]string, error)
}

// fsDiscoverer implements FileDiscoverer using the filesystem
type fsDiscoverer struct {
	root string
}

// NewFileDiscoverer creates a new file discoverer
func NewFileDiscoverer(root string) FileDiscoverer {
	return &fsDiscoverer{root: root}
}

// Discover finds all files matching include patterns and filters out exclude patterns
func (d *fsDiscoverer) Discover(includes, excludes []string) ([]string, error) {
	if len(includes) == 0 {
		return []string{}, nil
	}

	discoveredFiles := make(map[string]bool)

	// Process each include pattern
	for _, pattern := range includes {
		// Validate pattern for security
		if err := d.validatePattern(pattern); err != nil {
			return nil, err
		}

		// Build full pattern path
		fullPattern := filepath.Join(d.root, pattern)

		// Use doublestar.FilepathGlob for better pattern support
		// Note: doublestar does not follow symbolic links by default
		matches, err := doublestar.FilepathGlob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
		}

		// Add matches to set (deduplicates)
		for _, match := range matches {
			// Ensure file is within project root
			if !strings.HasPrefix(match, d.root) {
				return nil, core.NewError(nil, "PATH_ESCAPE_ATTEMPT", map[string]any{
					"file": match,
					"root": d.root,
				})
			}
			discoveredFiles[match] = true
		}
	}

	// Convert map to slice
	files := make([]string, 0, len(discoveredFiles))
	for file := range discoveredFiles {
		files = append(files, file)
	}

	// Apply exclude patterns
	files = d.applyExcludes(files, excludes)

	return files, nil
}

// validatePattern validates a pattern for security issues
func (d *fsDiscoverer) validatePattern(pattern string) error {
	// Clean the pattern
	cleanPattern := filepath.Clean(pattern)

	// Reject absolute paths
	if filepath.IsAbs(cleanPattern) {
		return fmt.Errorf("INVALID_PATTERN: absolute paths not allowed: %s", pattern)
	}

	// Reject parent directory references
	if strings.Contains(cleanPattern, "..") {
		return fmt.Errorf("INVALID_PATTERN: parent directory references not allowed: %s", pattern)
	}

	return nil
}

// applyExcludes filters out files matching exclude patterns
func (d *fsDiscoverer) applyExcludes(files []string, excludes []string) []string {
	if len(excludes) == 0 && len(DefaultExcludes) == 0 {
		return files
	}

	// Combine default excludes with user excludes
	allExcludes := make([]string, 0, len(DefaultExcludes)+len(excludes))
	allExcludes = append(allExcludes, DefaultExcludes...)
	allExcludes = append(allExcludes, excludes...)

	filtered := make([]string, 0, len(files))
	for _, file := range files {
		excluded := false

		// Make file path relative for pattern matching
		relFile, err := filepath.Rel(d.root, file)
		if err != nil {
			// If we can't make it relative, include it to be safe
			filtered = append(filtered, file)
			continue
		}

		// Convert to forward slashes for consistent matching
		relFile = filepath.ToSlash(relFile)

		// Check each exclude pattern
		for _, pattern := range allExcludes {
			// Convert pattern to forward slashes
			pattern = filepath.ToSlash(pattern)

			// Use doublestar for pattern matching
			matched, err := doublestar.Match(pattern, relFile)
			if err != nil {
				// Invalid pattern, skip it
				continue
			}

			if matched {
				excluded = true
				break
			}

			// Also check against the basename for patterns like *.bak
			matched, err = doublestar.Match(pattern, filepath.Base(file))
			if err == nil && matched {
				excluded = true
				break
			}
		}

		if !excluded {
			filtered = append(filtered, file)
		}
	}

	return filtered
}
