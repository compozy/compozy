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
	if !pathWithin(root, joined) {
		return nil, fmt.Errorf("path outside CWD: %s", rel)
	}
	f, err := os.Open(joined)
	if err != nil {
		return nil, fmt.Errorf("open failed: %w", err)
	}
	defer f.Close()
	head := make([]byte, 512)
	n, rerr := f.Read(head)
	if rerr != nil && rerr != io.EOF {
		return nil, fmt.Errorf("read failed: %w", rerr)
	}
	mime := detectMIME(head[:n])
	if !isAllowedByType(kind, mime) {
		return nil, fmt.Errorf("mime not allowed for %s: %s", kind, mime)
	}
	logger.FromContext(ctx).Debug("Resolved local attachment", "path", joined, "mime", mime)
	return &resolvedFile{path: joined, mime: mime, temp: false}, nil
}

func pathWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false
	}
	return true
}
