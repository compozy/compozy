package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func computeInstallChecksum(path string) (string, error) {
	root := strings.TrimSpace(path)
	if root == "" {
		return "", errors.New("registry: install directory is required")
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("registry: resolve install directory %q: %w", path, err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return "", fmt.Errorf("registry: stat install directory %q: %w", absRoot, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("registry: install directory %q is not a directory", absRoot)
	}

	hasher := sha256.New()
	entries := make([]string, 0)
	err = filepath.WalkDir(absRoot, func(entryPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entryPath == absRoot || entry.IsDir() {
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
		return "", fmt.Errorf("registry: walk install directory %q: %w", absRoot, err)
	}

	slices.Sort(entries)
	for _, relPath := range entries {
		if err := writeInstallChecksumEntry(hasher, absRoot, relPath); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeInstallChecksumEntry(hasher hash.Hash, root string, relPath string) error {
	normalizedPath := filepath.ToSlash(relPath)
	absPath := filepath.Join(root, relPath)

	info, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("registry: stat checksum path %q: %w", absPath, err)
	}

	if info.Mode().IsRegular() {
		file, err := os.Open(absPath)
		if err != nil {
			return fmt.Errorf("registry: open checksum path %q: %w", absPath, err)
		}
		if err := writeInstallChecksumString(
			hasher,
			fmt.Sprintf("file:%s\nmode:%#o\n", normalizedPath, info.Mode().Perm()),
		); err != nil {
			closeErr := file.Close()
			if closeErr != nil {
				return errors.Join(err, fmt.Errorf("registry: close checksum path %q: %w", absPath, closeErr))
			}
			return err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			copyErr := fmt.Errorf("registry: hash regular file %q: %w", absPath, err)
			if closeErr := file.Close(); closeErr != nil {
				copyErr = errors.Join(
					copyErr,
					fmt.Errorf("registry: close checksum path %q after read failure: %w", absPath, closeErr),
				)
			}
			return copyErr
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("registry: close checksum path %q: %w", absPath, err)
		}
		if _, err := hasher.Write([]byte{0}); err != nil {
			return fmt.Errorf("registry: hash separator for %q: %w", absPath, err)
		}
		return nil
	}

	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(absPath)
		if err != nil {
			return fmt.Errorf("registry: read checksum symlink %q: %w", absPath, err)
		}
		normalizedTarget := filepath.ToSlash(target)
		return writeInstallChecksumString(
			hasher,
			fmt.Sprintf("symlink:%s\nmode:%#o\ntarget:%s\n", normalizedPath, info.Mode().Perm(), normalizedTarget),
		)
	}

	return fmt.Errorf("registry: unsupported file type in install payload %q", absPath)
}

func writeInstallChecksumString(hasher hash.Hash, value string) error {
	if _, err := hasher.Write([]byte(value)); err != nil {
		return fmt.Errorf("registry: hash payload metadata: %w", err)
	}
	return nil
}
