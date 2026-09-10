package skillscan

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
)

type directorySnapshot struct {
	root         string
	trustedRoots []string
	files        map[string]fs.FileInfo
	resolved     map[string]string
	complete     bool
}

func newDirectorySnapshot(root, resolved string, trustedRoots []string) *directorySnapshot {
	return &directorySnapshot{
		root: root, trustedRoots: slices.Clone(trustedRoots), complete: true,
		files: make(map[string]fs.FileInfo), resolved: map[string]string{root: resolved},
	}
}

// Unchanged revalidates complete discovery inputs before callers reuse a directory result.
func (r DirectoryResult) Unchanged(ctx context.Context, root string, trustedRoots []string) bool {
	snapshot := r.discovery
	if ctx.Err() != nil || snapshot == nil || !snapshot.complete || r.Stats.Truncated ||
		!r.Stats.Readable {
		return false
	}
	absolute, err := filepath.Abs(root)
	if err != nil || absolute != snapshot.root {
		return false
	}
	if !slices.Equal(canonicalTrustedRoots(trustedRoots, snapshot.resolved[absolute]), snapshot.trustedRoots) {
		return false
	}
	return snapshot.unchangedPaths(ctx) && snapshot.unchangedFiles(ctx) &&
		r.unchangedSkippedLinks(ctx, snapshot.trustedRoots)
}

func (r DirectoryResult) unchangedSkippedLinks(ctx context.Context, trustedRoots []string) bool {
	for _, link := range r.Stats.SkippedLinks {
		if ctx.Err() != nil {
			return false
		}
		resolved, err := filepath.EvalSymlinks(link.Path)
		if err != nil {
			if symlinkFailureReason(link.Path, err) != link.Reason {
				return false
			}
			continue
		}
		if link.Reason != skippedLinkEscapeReason || pathWithinAnyTrustedRoot(resolved, trustedRoots) {
			return false
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			return false
		}
	}
	return true
}

func (s *directorySnapshot) unchangedPaths(ctx context.Context) bool {
	for path, expected := range s.resolved {
		if ctx.Err() != nil {
			return false
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != expected {
			return false
		}
	}
	return true
}

func (s *directorySnapshot) unchangedFiles(ctx context.Context) bool {
	for path, expected := range s.files {
		if ctx.Err() != nil {
			return false
		}
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(expected, current) || expected.Mode() != current.Mode() ||
			expected.Size() != current.Size() || !expected.ModTime().Equal(current.ModTime()) {
			return false
		}
	}
	return true
}

func (s *directorySnapshot) trackLink(path string, entry fs.DirEntry) {
	if entry.Type()&os.ModeSymlink != 0 {
		s.track(path, entry)
	}
}

func (s *directorySnapshot) track(path string, entry fs.DirEntry) {
	info, err := entry.Info()
	if err != nil {
		s.complete = false
		return
	}
	s.files[path] = info
}

func (s *directorySnapshot) merge(other *directorySnapshot) {
	if other == nil || !other.complete {
		s.complete = false
		return
	}
	maps.Copy(s.files, other.files)
	maps.Copy(s.resolved, other.resolved)
}
