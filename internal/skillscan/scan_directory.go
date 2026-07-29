package skillscan

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/filesnap"
)

type directoryScanner struct {
	root           string
	resolvedRoot   string
	result         DirectoryResult
	visitedEntries int
}

// ScanDirectory discovers skill definitions below root. It never follows
// directory symlinks and excludes definitions whose resolved path escapes root.
func ScanDirectory(root string) (DirectoryResult, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return DirectoryResult{}, errors.New("skillscan: directory root is required")
	}
	absRoot, err := filepath.Abs(trimmedRoot)
	if err != nil {
		return DirectoryResult{}, fmt.Errorf("skillscan: resolve root %q: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return emptyDirectoryResult(), nil
		}
		return DirectoryResult{}, fmt.Errorf("skillscan: stat root %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return DirectoryResult{}, fmt.Errorf("skillscan: root %q is not a directory", absRoot)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return DirectoryResult{}, fmt.Errorf("skillscan: resolve root %q: %w", absRoot, err)
	}
	scanner := directoryScanner{
		root:         absRoot,
		resolvedRoot: resolvedRoot,
		result: DirectoryResult{
			Paths:     make([]string, 0, MaxCandidates),
			Snapshots: make(map[string]filesnap.Snapshot, MaxCandidates),
		},
	}
	if err := filepath.WalkDir(absRoot, scanner.visit); err != nil && !isTraversalLimit(err) {
		return DirectoryResult{}, fmt.Errorf("skillscan: walk root %q: %w", absRoot, err)
	}
	slices.Sort(scanner.result.Paths)
	return scanner.result, nil
}

func (s *directoryScanner) visit(candidate string, entry fs.DirEntry, walkErr error) error {
	if s.visitedEntries >= MaxEntries {
		slog.Warn("skillscan: entry limit reached", "root", s.root, "limit", MaxEntries)
		return errEntryLimitReached
	}
	s.visitedEntries++
	if walkErr != nil {
		slog.Warn("skillscan: skipping unreadable path", "path", candidate, "error", walkErr)
		if entry != nil && entry.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	if entry.IsDir() {
		if candidate != s.root && shouldSkipDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if entry.Name() != SkillFileName {
		return nil
	}
	return s.addDefinition(candidate)
}

func (s *directoryScanner) addDefinition(candidate string) error {
	if err := pathWithinRoot(s.resolvedRoot, candidate); err != nil {
		slog.Warn("skillscan: skipping definition outside root", "path", candidate, "error", err)
		return nil
	}
	info, err := os.Stat(candidate)
	if err != nil {
		slog.Warn("skillscan: skipping unreadable definition", "path", candidate, "error", err)
		return nil
	}
	if !info.Mode().IsRegular() {
		slog.Warn("skillscan: skipping non-regular definition", "path", candidate)
		return nil
	}
	snapshot, err := filesnap.FromPath(candidate)
	if err != nil {
		slog.Warn("skillscan: skipping unreadable definition", "path", candidate, "error", err)
		return nil
	}
	s.result.Paths = append(s.result.Paths, candidate)
	s.result.Snapshots[candidate] = snapshot
	if len(s.result.Paths) >= MaxCandidates {
		slog.Warn("skillscan: candidate limit reached", "root", s.root, "limit", MaxCandidates)
		return errCandidateLimitReached
	}
	return nil
}
