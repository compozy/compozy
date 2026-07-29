// Package skillscan discovers skill definition files under filesystem roots.
package skillscan

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
)

const (
	// SkillFileName is the canonical skill definition filename.
	SkillFileName = "SKILL.md"
	// MaxCandidates bounds the number of definitions discovered from one root.
	MaxCandidates = 300
	// MaxEntries bounds the number of filesystem entries visited under one root.
	MaxEntries = 20_000
)

var (
	errCandidateLimitReached = errors.New("skillscan: candidate limit reached")
	errEntryLimitReached     = errors.New("skillscan: entry limit reached")
)

// DirectoryResult is the filesystem discovery result for one root.
type DirectoryResult struct {
	Paths     []string
	Snapshots map[string]filesnap.Snapshot
}

func emptyDirectoryResult() DirectoryResult {
	return DirectoryResult{Paths: []string{}, Snapshots: map[string]filesnap.Snapshot{}}
}

func isTraversalLimit(err error) bool {
	return errors.Is(err, errCandidateLimitReached) || errors.Is(err, errEntryLimitReached)
}

func shouldSkipDirectory(name string) bool {
	return name == ".git" || name == "node_modules" || (name != compozyconfig.DirName && strings.HasPrefix(name, "."))
}

func pathWithinRoot(resolvedRoot string, candidate string) error {
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("resolve definition %q: %w", candidate, err)
	}
	relativePath, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return fmt.Errorf("relate definition %q to root %q: %w", resolvedCandidate, resolvedRoot, err)
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return fmt.Errorf("definition %q escapes root %q", resolvedCandidate, resolvedRoot)
	}
	return nil
}
