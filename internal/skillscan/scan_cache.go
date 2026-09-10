package skillscan

import (
	"context"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/filesnap"
)

type directorySnapshot struct {
	root         string
	trustedRoots []string
	files        map[string]filesnap.Snapshot
	directories  map[string][]directoryEntry
	resolved     map[string]string
	complete     bool
}

type directoryEntry struct {
	name     string
	typeBits fs.FileMode
}

// newDirectorySnapshot binds discovery evidence to the original root, resolved path, and trusted roots.
func newDirectorySnapshot(root, resolved string, trustedRoots []string) *directorySnapshot {
	return &directorySnapshot{
		root: root, trustedRoots: slices.Clone(trustedRoots), complete: true,
		files: make(map[string]filesnap.Snapshot), resolved: map[string]string{root: resolved},
		directories: make(map[string][]directoryEntry),
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
	return snapshot.unchangedPaths(ctx) && snapshot.unchangedFiles(ctx) && snapshot.unchangedDirectories(ctx) &&
		r.unchangedSkippedLinks(ctx, snapshot.trustedRoots)
}

// unchangedDirectories detects membership or type changes even when directory timestamps are preserved.
func (s *directorySnapshot) unchangedDirectories(ctx context.Context) bool {
	for path, expected := range s.directories {
		if ctx.Err() != nil {
			return false
		}
		current, err := os.ReadDir(path)
		if err != nil || len(current) != len(expected) {
			return false
		}
		for index, entry := range current {
			if entry.Name() != expected[index].name || entry.Type() != expected[index].typeBits {
				return false
			}
		}
	}
	return true
}

// unchangedSkippedLinks requires each rejected link to retain its recorded failure or containment reason.
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

// unchangedPaths invalidates discovery when a root or followed symlink resolves to a different target.
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

// unchangedFiles revalidates every recorded file and directory against its captured snapshot.
func (s *directorySnapshot) unchangedFiles(ctx context.Context) bool {
	for path, expected := range s.files {
		if ctx.Err() != nil {
			return false
		}
		current, err := os.Lstat(path)
		if err != nil || !expected.Equal(filesnap.FromInfo(path, current)) {
			return false
		}
	}
	return true
}

// trackLink records symlink metadata so replacement or retargeting invalidates discovery.
func (s *directorySnapshot) trackLink(path string, entry fs.DirEntry) {
	if entry.Type()&os.ModeSymlink != 0 {
		s.track(path, entry)
	}
}

// trackEntry captures the listing that drove traversal, including ignored children.
func (s *directorySnapshot) trackEntry(path string, entry fs.DirEntry) {
	parent := filepath.Dir(path)
	s.directories[parent] = append(s.directories[parent], directoryEntry{
		name: entry.Name(), typeBits: entry.Type(),
	})
}

// track marks the snapshot incomplete when entry metadata cannot be captured safely.
func (s *directorySnapshot) track(path string, entry fs.DirEntry) {
	info, err := entry.Info()
	if err != nil {
		s.complete = false
		return
	}
	s.files[path] = filesnap.FromInfo(path, info)
}

// merge combines linked discovery evidence and rejects incomplete or contradictory directory listings.
func (s *directorySnapshot) merge(other *directorySnapshot) {
	if other == nil || !other.complete {
		s.complete = false
		return
	}
	for path, entries := range other.directories {
		if previous, exists := s.directories[path]; exists && !slices.Equal(previous, entries) {
			s.complete = false
		}
	}
	maps.Copy(s.files, other.files)
	maps.Copy(s.directories, other.directories)
	maps.Copy(s.resolved, other.resolved)
}
