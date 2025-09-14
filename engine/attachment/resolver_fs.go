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

const mimeDetectReadLimit = 512

// resolveLocalFile safely resolves a path against CWD and verifies it remains
// within the project tree. It detects MIME from the file head and returns a
// resolvedFile handle without copying data. Cleanup is a no-op for local files.
func resolveLocalFile(ctx context.Context, cwd *core.PathCWD, rel string, kind Type) (*resolvedFile, error) {
	if cwd == nil || cwd.Path == "" {
		return nil, fmt.Errorf("cwd not set")
	}
	root := filepath.Clean(cwd.Path)
	joined := filepath.Clean(filepath.Join(root, rel))

	// Resolve symlinks to prevent symlink-escape path traversal
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, fmt.Errorf("resolve CWD symlinks: %w", err)
	}
	resolvedJoined, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return nil, fmt.Errorf("resolve path symlinks: %w", err)
	}
	within, err := pathWithin(resolvedRoot, resolvedJoined)
	if err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}
	if !within {
		return nil, fmt.Errorf("path outside CWD: %s", rel)
	}

	f, err := os.Open(resolvedJoined)
	if err != nil {
		return nil, fmt.Errorf("open failed: %w", err)
	}
	defer f.Close()
	head := make([]byte, mimeDetectReadLimit)
	n, rerr := f.Read(head)
	if rerr != nil && rerr != io.EOF {
		return nil, fmt.Errorf("read failed: %w", rerr)
	}
	mime := detectMIME(head[:n])
	if !isAllowedByTypeCtx(ctx, kind, mime) {
		return nil, errMimeDenied(kind, mime)
	}
	logger.FromContext(ctx).Debug("Resolved local attachment", "path", resolvedJoined, "mime", mime)
	return &resolvedFile{path: resolvedJoined, mime: mime, temp: false}, nil
}

func pathWithin(root, target string) (bool, error) {
	// Canonicalize root path by resolving symlinks and cleaning
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		// If root symlink resolution fails, treat as insecure (fail-safe)
		return false, fmt.Errorf("resolve root symlinks: %w", err)
	}
	resolvedRoot = filepath.Clean(resolvedRoot)

	// Canonicalize target path by resolving symlinks and cleaning
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		// If symlink resolution fails and file doesn't exist, treat as insecure
		if os.IsNotExist(err) {
			return false, fmt.Errorf("target path does not exist: %s", target)
		}
		// If symlink resolution fails for other reasons, treat as insecure (fail-safe)
		return false, fmt.Errorf("resolve target symlinks: %w", err)
	}
	resolvedTarget = filepath.Clean(resolvedTarget)

	// Compute relative path from resolved root to resolved target
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil {
		return false, fmt.Errorf("compute relative path: %w", err)
	}

	// Reject if relative path escapes the root
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}

	// Allow current directory and subdirectories
	return true, nil
}
