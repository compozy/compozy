package builtin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// NormalizeRoot cleans and absolutizes the configured sandbox root.
func NormalizeRoot(root string) (string, error) {
	if root == "" {
		return "", errors.New("native tools root directory is empty")
	}
	clean := filepath.Clean(root)
	if !filepath.IsAbs(clean) {
		abs, err := filepath.Abs(clean)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute root: %w", err)
		}
		clean = abs
	}
	return clean, nil
}

// ResolvePath joins parts under root ensuring the result remains inside the sandbox.
func ResolvePath(root string, parts ...string) (string, error) {
	normalizedRoot, err := NormalizeRoot(root)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(append([]string{normalizedRoot}, parts...)...)
	cleaned := filepath.Clean(joined)
	relative, err := filepath.Rel(normalizedRoot, cleaned)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}
	if relative == ".." || relative == "." {
		if cleaned != normalizedRoot {
			return "", fmt.Errorf("path escapes native tools root: %s", cleaned)
		}
		return cleaned, nil
	}
	if len(relative) >= 3 && relative[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("path escapes native tools root: %s", cleaned)
	}
	return cleaned, nil
}

// EnsureWithinRoot verifies a candidate path resolves within the normalized root.
func EnsureWithinRoot(root string, candidate string) error {
	_, err := ResolvePath(root, candidate)
	return err
}

// RejectSymlink returns an error when the provided file info represents a symlink.
func RejectSymlink(info fs.FileInfo) error {
	if info == nil {
		return errors.New("file info is required")
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return errors.New("symbolic links are not permitted")
	}
	return nil
}

// CheckContext returns the context error if cancellation has been signaled.
func CheckContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
