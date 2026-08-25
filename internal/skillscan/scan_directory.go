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
	"syscall"

	"github.com/compozy/compozy/internal/filesnap"
)

const (
	skippedLinkCycleReason  = "cycle"
	skippedLinkEscapeReason = "escape"
)

// ScanDirectory discovers skill definitions below root. It resolves the root
// itself, never follows nested directory symlinks, and excludes definitions
// whose resolved path escapes root.
func ScanDirectory(root string) (DirectoryResult, error) {
	return ScanDirectoryWithin(root, []string{root})
}

// ScanDirectoryWithin discovers definitions while allowing first-level links
// to resolve only inside the supplied projection roots.
func ScanDirectoryWithin(root string, trustedRoots []string) (DirectoryResult, error) {
	scanner, err := newDirectoryScanner(root, trustedRoots)
	if err != nil {
		return DirectoryResult{}, err
	}
	if scanner.skipTraversal {
		return scanner.result, nil
	}
	if err := scanner.scanBase(); err != nil {
		return DirectoryResult{}, err
	}
	if err := scanner.followFirstLevelLinks(); err != nil {
		return DirectoryResult{}, err
	}
	slices.Sort(scanner.result.Paths)
	return scanner.result, nil
}

type directoryScanner struct {
	root          string
	walkRoot      string
	resolvedRoot  string
	result        DirectoryResult
	budget        *scanBudget
	skipTraversal bool
	trustedRoots  []string
	seenRealPaths map[string]struct{}
}

type scanBudget struct {
	entries    int
	candidates int
}

func newDirectoryScanner(root string, trustedRoots []string) (*directoryScanner, error) {
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
		if errors.Is(err, fs.ErrPermission) {
			result := emptyDirectoryResult()
			result.Stats.Exists = true
			result.Stats.Readable = false
			return &directoryScanner{root: absRoot, result: result, skipTraversal: true}, nil
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
		root:          absRoot,
		walkRoot:      resolvedRoot,
		resolvedRoot:  resolvedRoot,
		trustedRoots:  canonicalTrustedRoots(trustedRoots, resolvedRoot),
		seenRealPaths: make(map[string]struct{}),
		budget:        &scanBudget{},
		result: DirectoryResult{
			Paths:     make([]string, 0, MaxCandidates),
			Snapshots: make(map[string]filesnap.Snapshot, MaxCandidates),
			RealPaths: make(map[string]string, MaxCandidates),
			Stats:     RootScanStats{Exists: true, Readable: true},
		},
	}, nil
}

func (s *directoryScanner) scanBase() error {
	walkErr := filepath.WalkDir(s.walkRoot, s.visit)
	if walkErr != nil && !isTraversalLimit(walkErr) {
		return fmt.Errorf("skillscan: walk root %q: %w", s.root, walkErr)
	}
	return nil
}

func (s *directoryScanner) visit(candidate string, entry fs.DirEntry, walkErr error) error {
	if s.budget.entries >= MaxEntries {
		slog.Warn("skillscan: entry limit reached", "root", s.root, "limit", MaxEntries)
		s.result.Stats.Truncated = true
		return errEntryLimitReached
	}
	s.budget.entries++
	if walkErr != nil {
		slog.Warn("skillscan: skipping unreadable path", "path", candidate, "error", walkErr)
		if candidate == s.walkRoot && errors.Is(walkErr, fs.ErrPermission) {
			s.result.Stats.Readable = false
		}
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
	realPath, err := filepath.EvalSymlinks(reportedCandidate)
	if err != nil {
		slog.Warn("skillscan: skipping unresolved definition", "path", reportedCandidate, "error", err)
		return nil
	}
	s.result.Stats.ScannedCount++
	s.budget.candidates++
	candidateLimitReached := s.budget.candidates >= MaxCandidates
	if _, exists := s.seenRealPaths[realPath]; exists {
		if candidateLimitReached {
			s.result.Stats.Truncated = true
			return errCandidateLimitReached
		}
		return nil
	}
	s.seenRealPaths[realPath] = struct{}{}
	snapshot := filesnap.Snapshot{ModTime: info.ModTime(), Size: info.Size()}
	s.result.Paths = append(s.result.Paths, reportedCandidate)
	s.result.Snapshots[reportedCandidate] = snapshot
	s.result.RealPaths[reportedCandidate] = realPath
	if candidateLimitReached {
		slog.Warn("skillscan: candidate limit reached", "root", s.root, "limit", MaxCandidates)
		s.result.Stats.Truncated = true
		return errCandidateLimitReached
	}
	return nil
}

func (s *directoryScanner) followFirstLevelLinks() error {
	if s.budget.candidates >= MaxCandidates || s.budget.entries >= MaxEntries {
		return nil
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrPermission) {
			s.result.Stats.Readable = false
			return nil
		}
		return fmt.Errorf("skillscan: read root %q: %w", s.root, err)
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		linkPath := filepath.Join(s.root, entry.Name())
		resolved, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			s.result.Stats.SkippedLinks = append(s.result.Stats.SkippedLinks, SkippedLink{
				Path: linkPath, Reason: symlinkFailureReason(linkPath, err),
			})
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}
		if !pathWithinAnyTrustedRoot(resolved, s.trustedRoots) {
			s.result.Stats.SkippedLinks = append(s.result.Stats.SkippedLinks, SkippedLink{
				Path: linkPath, Reason: skippedLinkEscapeReason,
			})
			continue
		}
		linked, err := newDirectoryScanner(linkPath, s.trustedRoots)
		if err != nil {
			return err
		}
		linked.budget = s.budget
		if !linked.skipTraversal {
			if err := linked.scanBase(); err != nil {
				return err
			}
		}
		s.mergeLinkedResult(linked.result)
		if s.budget.candidates >= MaxCandidates || s.budget.entries >= MaxEntries {
			s.result.Stats.Truncated = true
			break
		}
	}
	return nil
}

func (s *directoryScanner) mergeLinkedResult(result DirectoryResult) {
	s.result.Stats.ScannedCount += result.Stats.ScannedCount
	s.result.Stats.Truncated = s.result.Stats.Truncated || result.Stats.Truncated
	for _, path := range result.Paths {
		realPath := result.RealPaths[path]
		if _, exists := s.seenRealPaths[realPath]; exists {
			continue
		}
		if len(s.result.Paths) >= MaxCandidates {
			s.result.Stats.Truncated = true
			return
		}
		s.seenRealPaths[realPath] = struct{}{}
		s.result.Paths = append(s.result.Paths, path)
		s.result.Snapshots[path] = result.Snapshots[path]
		s.result.RealPaths[path] = realPath
	}
}

func canonicalTrustedRoots(roots []string, fallback string) []string {
	if len(roots) == 0 {
		roots = []string{fallback}
	}
	canonical := make([]string, 0, len(roots))
	for _, root := range roots {
		trimmed := strings.TrimSpace(root)
		if trimmed == "" {
			continue
		}
		absolute, err := filepath.Abs(trimmed)
		if err != nil {
			continue
		}
		resolved, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			resolved = filepath.Clean(absolute)
		}
		canonical = append(canonical, resolved)
	}
	return canonical
}

func pathWithinAnyTrustedRoot(path string, roots []string) bool {
	for _, root := range roots {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			continue
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func symlinkFailureReason(path string, err error) string {
	if errors.Is(err, syscall.ELOOP) || strings.Contains(strings.ToLower(err.Error()), "too many levels") ||
		symlinkChainHasCycle(path) {
		return skippedLinkCycleReason
	}
	return "dangling"
}

func symlinkChainHasCycle(path string) bool {
	seen := make(map[string]struct{})
	current := filepath.Clean(path)
	for range 64 {
		if _, exists := seen[current]; exists {
			return true
		}
		seen[current] = struct{}{}
		info, err := os.Lstat(current)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			return false
		}
		target, err := os.Readlink(current)
		if err != nil {
			return false
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(current), target)
		}
		current = filepath.Clean(target)
	}
	return true
}

func (s *directoryScanner) reportedCandidate(candidate string) (string, error) {
	relativePath, err := filepath.Rel(s.walkRoot, candidate)
	if err != nil {
		return "", fmt.Errorf("skillscan: relate candidate %q to root %q: %w", candidate, s.walkRoot, err)
	}
	return filepath.Join(s.root, relativePath), nil
}
