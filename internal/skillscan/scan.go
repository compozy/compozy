// Package skillscan discovers skill definition files under filesystem roots.
package skillscan

import (
	"errors"
	"fmt"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/fileutil"
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
	rootPath, definitionPath, err := fileutil.ResolveExistingPathWithinRoot(resolvedRoot, candidate)
	if err == nil {
		return nil
	}
	if errors.Is(err, fileutil.ErrPathOutsideRoot) {
		return fmt.Errorf("definition %q escapes root %q: %w", definitionPath, rootPath, err)
	}
	return fmt.Errorf("validate definition %q within root %q: %w", candidate, resolvedRoot, err)
}
