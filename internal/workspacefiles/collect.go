package workspacefiles

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
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
			idx.logger.Warn("workspacefiles.git_fallback", "root_hash", rootHash(root), "error", err)
		}
	}
	files, truncated, err := idx.walkFiles(ctx, root)
	if err != nil {
		return nil, "", errors.Join(gitErr, err)
	}
	return newSnapshot(files, truncated), sourceWalk, nil
}

// walkFiles lists regular files and symlinks under root without following
// symlinked directories, skipping .git and node_modules directories.
func (idx *index) walkFiles(ctx context.Context, root string) ([]string, bool, error) {
	files := make([]string, 0, 256)
	truncated := false
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() {
			if _, skip := skippedWalkDirs[entry.Name()]; skip && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() && entry.Type()&fs.ModeSymlink == 0 {
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

// indexablePath rejects paths a mention token cannot carry.
func indexablePath(path string) bool {
	return path != "" && !strings.ContainsAny(path, "\"\n\r")
}
