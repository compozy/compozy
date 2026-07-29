// Package skillscan discovers skill definition files under filesystem roots.
package skillscan

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
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

	result := DirectoryResult{
		Paths:     make([]string, 0, MaxCandidates),
		Snapshots: make(map[string]filesnap.Snapshot, MaxCandidates),
	}
	visitedEntries := 0
	walkErr := filepath.WalkDir(absRoot, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if visitedEntries >= MaxEntries {
			slog.Warn("skillscan: entry limit reached", "root", absRoot, "limit", MaxEntries)
			return errEntryLimitReached
		}
		visitedEntries++
		if walkErr != nil {
			slog.Warn("skillscan: skipping unreadable path", "path", candidate, "error", walkErr)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if candidate != absRoot && shouldSkipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != SkillFileName {
			return nil
		}
		if err := pathWithinRoot(resolvedRoot, candidate); err != nil {
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
		result.Paths = append(result.Paths, candidate)
		result.Snapshots[candidate] = snapshot
		if len(result.Paths) >= MaxCandidates {
			slog.Warn("skillscan: candidate limit reached", "root", absRoot, "limit", MaxCandidates)
			return errCandidateLimitReached
		}
		return nil
	})
	if walkErr != nil &&
		!errors.Is(walkErr, errCandidateLimitReached) &&
		!errors.Is(walkErr, errEntryLimitReached) {
		return DirectoryResult{}, fmt.Errorf("skillscan: walk root %q: %w", absRoot, walkErr)
	}
	slices.Sort(result.Paths)
	return result, nil
}

// ScanFS discovers skill definitions below root in an fs.FS source.
func ScanFS(fsys fs.FS, root string) ([]string, error) {
	if fsys == nil {
		return nil, errors.New("skillscan: filesystem is required")
	}
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return nil, errors.New("skillscan: filesystem root is required")
	}
	cleanRoot := path.Clean(trimmedRoot)
	if cleanRoot != "." && !fs.ValidPath(cleanRoot) {
		return nil, fmt.Errorf("skillscan: invalid filesystem root %q", root)
	}
	info, err := fs.Stat(fsys, cleanRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("skillscan: stat filesystem root %q: %w", cleanRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skillscan: filesystem root %q is not a directory", cleanRoot)
	}

	paths := make([]string, 0, MaxCandidates)
	visitedEntries := 0
	walkErr := fs.WalkDir(fsys, cleanRoot, func(candidate string, entry fs.DirEntry, walkErr error) error {
		if visitedEntries >= MaxEntries {
			slog.Warn("skillscan: entry limit reached", "root", cleanRoot, "limit", MaxEntries)
			return errEntryLimitReached
		}
		visitedEntries++
		if walkErr != nil {
			slog.Warn("skillscan: skipping unreadable filesystem path", "path", candidate, "error", walkErr)
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if candidate != cleanRoot && shouldSkipDirectory(path.Base(candidate)) {
				return fs.SkipDir
			}
			return nil
		}
		if path.Base(candidate) != SkillFileName {
			return nil
		}
		info, err := fs.Stat(fsys, candidate)
		if err != nil {
			slog.Warn("skillscan: skipping unreadable filesystem definition", "path", candidate, "error", err)
			return nil
		}
		if !info.Mode().IsRegular() {
			slog.Warn("skillscan: skipping non-regular filesystem definition", "path", candidate)
			return nil
		}
		paths = append(paths, candidate)
		if len(paths) >= MaxCandidates {
			slog.Warn("skillscan: candidate limit reached", "root", cleanRoot, "limit", MaxCandidates)
			return errCandidateLimitReached
		}
		return nil
	})
	if walkErr != nil &&
		!errors.Is(walkErr, errCandidateLimitReached) &&
		!errors.Is(walkErr, errEntryLimitReached) {
		return nil, fmt.Errorf("skillscan: walk filesystem root %q: %w", cleanRoot, walkErr)
	}
	slices.Sort(paths)
	return paths, nil
}

func emptyDirectoryResult() DirectoryResult {
	return DirectoryResult{Paths: []string{}, Snapshots: map[string]filesnap.Snapshot{}}
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
