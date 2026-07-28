package extensionpkg

import (
	"crypto/sha256"

	"encoding/hex"

	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ComputeDirectoryChecksum returns a deterministic SHA-256 checksum for an
// installed extension directory payload.
func ComputeDirectoryChecksum(path string) (string, error) {
	root := strings.TrimSpace(path)
	if root == "" {
		return "", errors.New("extension: extension directory is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("extension: resolve extension directory %q: %w", path, err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return "", fmt.Errorf("extension: stat extension directory %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extension: extension directory %q is not a directory", absRoot)
	}

	entries := make([]string, 0)
	err = filepath.WalkDir(absRoot, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entryPath == absRoot {
			return nil
		}
		if entry.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, entryPath)
		if err != nil {
			return err
		}
		entries = append(entries, relPath)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("extension: walk extension directory %q: %w", absRoot, err)
	}

	slices.Sort(entries)
	hasher := sha256.New()
	for _, relPath := range entries {
		if err := writeChecksumEntry(hasher, absRoot, relPath); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeChecksumEntry(hasher hash.Hash, root string, relPath string) error {
	normalizedPath := filepath.ToSlash(relPath)
	absPath := filepath.Join(root, relPath)

	info, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("extension: stat checksum path %q: %w", absPath, err)
	}

	if info.Mode().IsRegular() {
		content, err := os.ReadFile(absPath)
		if err != nil {
			return fmt.Errorf("extension: read checksum path %q: %w", absPath, err)
		}

		if err := writeChecksumString(
			hasher,
			fmt.Sprintf("file:%s\nmode:%#o\n", normalizedPath, info.Mode().Perm()),
		); err != nil {
			return err
		}
		if _, err := hasher.Write(content); err != nil {
			return fmt.Errorf("extension: hash regular file %q: %w", absPath, err)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return fmt.Errorf("extension: hash separator for %q: %w", absPath, err)
		}
		return nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(absPath)
		if err != nil {
			return fmt.Errorf("extension: read checksum symlink %q: %w", absPath, err)
		}
		normalizedTarget := filepath.ToSlash(filepath.Clean(target))
		return writeChecksumString(
			hasher,
			fmt.Sprintf("symlink:%s\nmode:%#o\ntarget:%s\n", normalizedPath, info.Mode().Perm(), normalizedTarget),
		)
	}

	return fmt.Errorf("extension: unsupported file type in extension payload %q", absPath)
}

func writeChecksumString(hasher hash.Hash, value string) error {
	if _, err := hasher.Write([]byte(value)); err != nil {
		return fmt.Errorf("extension: hash payload metadata: %w", err)
	}
	return nil
}
