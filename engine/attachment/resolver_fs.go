package attachment

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
)

// resolveLocalFile safely resolves a path against CWD and verifies it remains
// within the project tree. It detects MIME from the file head and returns a
// resolvedFile handle without copying data. Cleanup is a no-op for local files.
func resolveLocalFile(ctx context.Context, cwd *core.PathCWD, rel string, kind Type) (*resolvedFile, error) {
	if cwd == nil || cwd.Path == "" {
		return nil, fmt.Errorf("cwd not set")
	}
	root := filepath.Clean(cwd.Path)
	joined := filepath.Clean(filepath.Join(root, rel))
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve CWD symlinks: %w", err)
	}
	resolvedJoined, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return nil, fmt.Errorf("resolve path symlinks: %w", err)
	}
	within, err := pathWithinResolved(resolvedRoot, resolvedJoined)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}
	if !within {
		return nil, fmt.Errorf("path outside CWD: %s", rel)
	}
	fi, statErr := os.Stat(resolvedJoined)
	if statErr != nil {
		return nil, fmt.Errorf("stat failed: %w", statErr)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file: %s", rel)
	}
	f, err := os.Open(resolvedJoined)
	if err != nil {
		return nil, fmt.Errorf("open failed: %w", err)
	}
	defer f.Close()
	mime, err := detectMIMEFromFile(ctx, f)
	if err != nil {
		return nil, err
	}
	if !isAllowedByTypeCtx(ctx, kind, mime) {
		return nil, errMimeDenied(kind, mime)
	}
	logger.FromContext(ctx).Debug("Resolved local attachment", "mime", mime)
	return &resolvedFile{path: resolvedJoined, mime: mime, temp: false}, nil
}

// pathWithinResolved validates that targetResolved is inside rootResolved.
// Preconditions: both arguments MUST be outputs of filepath.EvalSymlinks (or equivalent)
// to avoid bypass via symlinks. Performs only Rel() + traversal checks.
func pathWithinResolved(rootResolved, targetResolved string) (bool, error) {
	rel, err := filepath.Rel(filepath.Clean(rootResolved), filepath.Clean(targetResolved))
	if err != nil {
		return false, fmt.Errorf("compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}
	return true, nil
}

func pathWithin(root, target string) (bool, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false, fmt.Errorf("resolve root symlinks: %w", err)
	}
	resolvedRoot = filepath.Clean(resolvedRoot)
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("target path does not exist: %s", target)
		}
		return false, fmt.Errorf("resolve target symlinks: %w", err)
	}
	resolvedTarget = filepath.Clean(resolvedTarget)
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return false, fmt.Errorf("compute relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}
	return true, nil
}

func detectMIMEFromFile(ctx context.Context, f *os.File) (string, error) {
	// Note: this reads from the current file offset and advances it. Callers
	// that need to reread from the beginning should seek back after this call.
	head := make([]byte, mimeHeadLimit(ctx))
	n, err := f.Read(head)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read failed for %q: %w", filepath.Base(f.Name()), err)
	}
	return detectMIME(head[:n]), nil
}
