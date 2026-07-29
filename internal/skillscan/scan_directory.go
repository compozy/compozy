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

// ScanDirectory discovers skill definitions below root. It resolves the root
// itself, never follows nested directory symlinks, and excludes definitions
// whose resolved path escapes root.
func ScanDirectory(root string) (DirectoryResult, error) {
	scanner, err := newDirectoryScanner(root)
	if err != nil {
		return DirectoryResult{}, err
	}
	if scanner.skipTraversal {
		return scanner.result, nil
	}

	walkErr := filepath.WalkDir(scanner.walkRoot, scanner.visit)
	if walkErr != nil && !isTraversalLimit(walkErr) {
		return DirectoryResult{}, fmt.Errorf("skillscan: walk root %q: %w", scanner.root, walkErr)
	}
	slices.Sort(scanner.result.Paths)
	return scanner.result, nil
}

type directoryScanner struct {
	root           string
	walkRoot       string
	resolvedRoot   string
	result         DirectoryResult
	visitedEntries int
	skipTraversal  bool
}

func newDirectoryScanner(root string) (*directoryScanner, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return nil, errors.New("skillscan: directory root is required")
	}

	absRoot, err := filepath.Abs(trimmedRoot)
	if err != nil {
		return nil, fmt.Errorf("skillscan: resolve root %q: %w", root, err)
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &directoryScanner{
				root:          absRoot,
				result:        emptyDirectoryResult(),
				skipTraversal: true,
			}, nil
		}
		return nil, fmt.Errorf("skillscan: stat root %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skillscan: root %q is not a directory", absRoot)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return nil, fmt.Errorf("skillscan: resolve root %q: %w", absRoot, err)
	}

	return &directoryScanner{
		root:         absRoot,
		walkRoot:     resolvedRoot,
		resolvedRoot: resolvedRoot,
		result: DirectoryResult{
			Paths:     make([]string, 0, MaxCandidates),
			Snapshots: make(map[string]filesnap.Snapshot, MaxCandidates),
		},
	}, nil
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
	reportedCandidate, err := s.reportedCandidate(candidate)
	if err != nil {
		return err
	}
	if entry.IsDir() {
		if candidate != s.walkRoot && shouldSkipDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		return nil
	}
	if entry.Name() != SkillFileName {
		return nil
	}
	if err := pathWithinRoot(s.resolvedRoot, reportedCandidate); err != nil {
		slog.Warn("skillscan: skipping definition outside root", "path", reportedCandidate, "error", err)
		return nil
	}
	info, err := os.Stat(reportedCandidate)
	if err != nil {
		slog.Warn("skillscan: skipping unreadable definition", "path", reportedCandidate, "error", err)
		return nil
	}
	if !info.Mode().IsRegular() {
		slog.Warn("skillscan: skipping non-regular definition", "path", reportedCandidate)
		return nil
	}
	snapshot := filesnap.Snapshot{ModTime: info.ModTime(), Size: info.Size()}
	s.result.Paths = append(s.result.Paths, reportedCandidate)
	s.result.Snapshots[reportedCandidate] = snapshot
	if len(s.result.Paths) >= MaxCandidates {
		slog.Warn("skillscan: candidate limit reached", "root", s.root, "limit", MaxCandidates)
		return errCandidateLimitReached
	}
	return nil
}

func (s *directoryScanner) reportedCandidate(candidate string) (string, error) {
	relativePath, err := filepath.Rel(s.walkRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("skillscan: relate candidate %q to root %q: %w", candidate, s.walkRoot, err)
	}
	return filepath.Join(s.root, relativePath), nil
}
