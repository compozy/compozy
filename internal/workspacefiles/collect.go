package workspacefiles

import (
	"bytes"
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

var errNotRepository = errors.New("workspacefiles: root is not inside a git repository")

// gitFiles returns tracked and untracked-not-ignored files, minus deleted ones,
// relative to root.
func (idx *index) gitFiles(ctx context.Context, root string) ([]string, bool, error) {
	gitCtx, cancel := context.WithTimeout(ctx, idx.gitTimeout)
	defer cancel()
	listed, err := idx.runGit(gitCtx, root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, false, err
	}
	deletedOut, err := idx.runGit(gitCtx, root, "ls-files", "--deleted", "-z")
	if err != nil {
		return nil, false, err
	}
	deletedNames, _ := splitNUL(deletedOut)
	deleted := make(map[string]struct{}, len(deletedNames))
	for _, name := range deletedNames {
		deleted[name] = struct{}{}
	}
	names, partial := splitNUL(listed)
	seen := make(map[string]struct{}, len(names))
	files := make([]string, 0, min(len(names), idx.maxFiles))
	truncated := partial
	for _, name := range names {
		if _, gone := deleted[name]; gone || !indexablePath(name) {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		if len(files) == idx.maxFiles {
			truncated = true
			break
		}
		files = append(files, name)
	}
	return files, truncated, nil
}

func (idx *index) runGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	stdout, stderr, err := idx.git.Run(ctx, root, args...)
	if err == nil {
		return stdout, nil
	}
	if strings.Contains(string(stderr), "not a git repository") ||
		strings.Contains(err.Error(), "not a git repository") {
		return nil, errNotRepository
	}
	return nil, fmt.Errorf("workspacefiles: git %s: %w", args[0], err)
}

// splitNUL splits NUL-terminated records. A trailing record without its NUL was
// cut by the runner's output cap; it is dropped and reported as partial.
func splitNUL(output []byte) ([]string, bool) {
	if len(output) == 0 {
		return nil, false
	}
	partial := output[len(output)-1] != 0
	records := bytes.Split(output, []byte{0})
	records = records[:len(records)-1]
	names := make([]string, 0, len(records))
	for _, record := range records {
		if len(record) > 0 {
			names = append(names, string(record))
		}
	}
	return names, partial
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
