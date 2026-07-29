package skillscan

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strings"
)

type filesystemScanner struct {
	fsys           fs.FS
	root           string
	paths          []string
	visitedEntries int
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
	scanner := filesystemScanner{
		fsys:  fsys,
		root:  cleanRoot,
		paths: make([]string, 0, MaxCandidates),
	}
	if err := fs.WalkDir(fsys, cleanRoot, scanner.visit); err != nil && !isTraversalLimit(err) {
		return nil, fmt.Errorf("skillscan: walk filesystem root %q: %w", cleanRoot, err)
	}
	slices.Sort(scanner.paths)
	return scanner.paths, nil
}

func (s *filesystemScanner) visit(candidate string, entry fs.DirEntry, walkErr error) error {
	if s.visitedEntries >= MaxEntries {
		slog.Warn("skillscan: entry limit reached", "root", s.root, "limit", MaxEntries)
		return errEntryLimitReached
	}
	s.visitedEntries++
	if walkErr != nil {
		slog.Warn("skillscan: skipping unreadable filesystem path", "path", candidate, "error", walkErr)
		if entry != nil && entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	}
	if entry.IsDir() {
		if candidate != s.root && shouldSkipDirectory(path.Base(candidate)) {
			return fs.SkipDir
		}
		return nil
	}
	if path.Base(candidate) != SkillFileName {
		return nil
	}
	return s.addDefinition(candidate)
}

func (s *filesystemScanner) addDefinition(candidate string) error {
	info, err := fs.Stat(s.fsys, candidate)
	if err != nil {
		slog.Warn("skillscan: skipping unreadable filesystem definition", "path", candidate, "error", err)
		return nil
	}
	if !info.Mode().IsRegular() {
		slog.Warn("skillscan: skipping non-regular filesystem definition", "path", candidate)
		return nil
	}
	s.paths = append(s.paths, candidate)
	if len(s.paths) >= MaxCandidates {
		slog.Warn("skillscan: candidate limit reached", "root", s.root, "limit", MaxCandidates)
		return errCandidateLimitReached
	}
	return nil
}
