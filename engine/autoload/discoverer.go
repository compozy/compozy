package autoload

import (
	"fmt"
	"path/filepath"
	"slices"
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
			// Ensure the discovered path is really inside the root directory.
			rel, err := filepath.Rel(d.root, match)
			if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
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
	if slices.Contains(strings.Split(cleanPattern, string(filepath.Separator)), "..") {
		return fmt.Errorf("INVALID_PATTERN: parent directory references not allowed: %s", pattern)
	}

	return nil
}

// applyExcludes filters out files matching exclude patterns
func (d *fsDiscoverer) applyExcludes(files []string, excludes []string) []string {
	patterns := d.combineExcludePatterns(excludes)
	if len(patterns) == 0 {
		return files
	}
	filtered := make([]string, 0, len(files))
	for _, file := range files {
		if d.shouldExcludeFile(file, patterns) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

// combineExcludePatterns merges and normalizes user and default exclude patterns.
func (d *fsDiscoverer) combineExcludePatterns(excludes []string) []string {
	total := len(DefaultExcludes) + len(excludes)
	if total == 0 {
		return nil
	}
	combined := make([]string, 0, total)
	combined = append(combined, DefaultExcludes...)
	combined = append(combined, excludes...)
	for i, pattern := range combined {
		combined[i] = filepath.ToSlash(pattern)
	}
	return combined
}

// shouldExcludeFile determines whether a file matches any exclude pattern.
func (d *fsDiscoverer) shouldExcludeFile(file string, patterns []string) bool {
	relFile, err := filepath.Rel(d.root, file)
	if err != nil {
		return false
	}
	relFile = filepath.ToSlash(relFile)
	base := filepath.Base(file)
	for _, pattern := range patterns {
		if matchesExcludePattern(pattern, relFile, base) {
			return true
		}
	}
	return false
}

// matchesExcludePattern checks a pattern against relative and base filenames.
func matchesExcludePattern(pattern string, relFile string, base string) bool {
	matched, err := doublestar.Match(pattern, relFile)
	if err == nil && matched {
		return true
	}
	matched, err = doublestar.Match(pattern, base)
	return err == nil && matched
}
