package workspacefiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

const (
	sourceGit  = "git"
	sourceWalk = "walk"
)

var skippedWalkDirs = map[string]struct{}{".git": {}, "node_modules": {}}

// collect lists root's files from git, falling back to a bounded walk outside a
// repository or when git fails or times out.
func (idx *index) collect(ctx context.Context, root string) (*snapshot, string, error) {
	var gitErr error
	if idx.git != nil {
		files, truncated, err := idx.gitFiles(ctx, root)
		if err == nil {
			return newSnapshot(files, truncated), sourceGit, nil
		}
		gitErr = err
		if !errors.Is(err, errNotRepository) {
			idx.logger.Warn(
				"workspacefiles.git_fallback", "root_hash", rootHash(root), "error", redactRoot(root, err),
			)
		}
	}
	files, truncated, err := idx.walkFiles(ctx, root)
	if err != nil {
		return nil, "", errors.Join(gitErr, err)
	}
	return newSnapshot(files, truncated), sourceWalk, nil
}

// walkFiles lists regular files and classified symlinks under root without
// following symlinked directories, skipping .git and node_modules directories.
// The walk stops and reports truncation at the file cap, after visiting
// maxWalkEntries directory entries (folders count), or at the build deadline.
func (idx *index) walkFiles(ctx context.Context, root string) ([]string, bool, error) {
	files := make([]string, 0, 256)
	truncated := false
	visited := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if path != root {
			visited++
			if visited > idx.maxWalkEntries {
				truncated = true
				return fs.SkipAll
			}
		}
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				truncated = true
				return fs.SkipAll
			}
			return err
		}
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if _, skip := skippedWalkDirs[entry.Name()]; skip && path != root {
				return fs.SkipDir
			}
			return nil
		}
		symlink := entry.Type()&fs.ModeSymlink != 0
		if !entry.Type().IsRegular() && !symlink {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !indexablePath(rel) {
			return nil
		}
		if symlink {
			var keep bool
			if rel, keep = resolveSymlink(root, rel); !keep {
				return nil
			}
		}
		if len(files) == idx.maxFiles {
			truncated = true
			return fs.SkipAll
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("workspacefiles: walk root: %w", err)
	}
	return files, truncated, nil
}

// classifyListed applies the symlink rule to one git-listed path. A path that
// no longer exists (a deleted tracked file) is skipped; a listed directory (a
// submodule gitlink, listed without a trailing slash) is a folder, as mention
// resolution classifies it; other paths that are not symlinks, or cannot
// otherwise be inspected, are kept as listed.
func classifyListed(root string, rel string) (string, bool) {
	info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	if errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	switch {
	case err != nil:
		return rel, true
	case info.IsDir():
		return strings.TrimSuffix(rel, "/") + "/", true
	case info.Mode()&fs.ModeSymlink == 0:
		return rel, true
	default:
		return resolveSymlink(root, rel)
	}
}

// resolveSymlink classifies a symlink by its target, as mention resolution
// does: a directory inside root lists as a folder ("rel/"), a regular file
// inside root as a file, and a broken, escaping, or special target is skipped.
func resolveSymlink(root string, rel string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	inside, err := fileutil.PathWithinRoot(root, resolved)
	if err != nil || !inside {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false
	}
	switch {
	case info.IsDir():
		return rel + "/", true
	case info.Mode().IsRegular():
		return rel, true
	default:
		return "", false
	}
}

// indexablePath rejects paths a mention token cannot carry.
func indexablePath(path string) bool {
	return path != "" && !strings.ContainsAny(path, "\"\n\r")
}
