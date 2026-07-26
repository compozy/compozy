package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func readOptionalRegularFile(path string, label string) ([]byte, bool, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return nil, false, fmt.Errorf("config: %s path is required", label)
	}

	info, err := os.Lstat(normalizedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("config: stat %s %q: %w", label, normalizedPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, false, fmt.Errorf(
			"config: %s %q must be a regular file, not a symlink",
			label,
			normalizedPath,
		)
	}
	if info.IsDir() {
		return nil, false, fmt.Errorf(
			"config: %s %q must be a regular file, not a directory",
			label,
			normalizedPath,
		)
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("config: %s %q must be a regular file", label, normalizedPath)
	}

	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, false, fmt.Errorf("config: read %s %q: %w", label, normalizedPath, err)
	}
	return content, true, nil
}

func writePersistedFile(path string, contents []byte) (err error) {
	return writePersistedFileMode(path, contents, true)
}

func writePersistedFileExclusive(path string, contents []byte) (err error) {
	return writePersistedFileMode(path, contents, false)
}

func writePersistedFileMode(path string, contents []byte, replace bool) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, privateDirMode); err != nil {
		return fmt.Errorf("create config directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp config file in %q: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove temp config file %q: %w", tmpPath, removeErr))
		}
	}()

	if err := tmpFile.Chmod(privateFileMode); err != nil {
		return closeFileAfterError(tmpFile, tmpPath, fmt.Errorf("chmod temp config file %q: %w", tmpPath, err))
	}
	if _, err := tmpFile.Write(contents); err != nil {
		return closeFileAfterError(tmpFile, tmpPath, fmt.Errorf("write temp config file %q: %w", tmpPath, err))
	}
	if err := tmpFile.Sync(); err != nil {
		return closeFileAfterError(tmpFile, tmpPath, fmt.Errorf("sync temp config file %q: %w", tmpPath, err))
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp config file %q: %w", tmpPath, err)
	}
	if replace {
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("replace config file %q: %w", path, err)
		}
	} else if err := os.Link(tmpPath, path); err != nil {
		return fmt.Errorf("create config file %q: %w", path, err)
	}
	if err := syncPersistedDir(dir); err != nil {
		return err
	}
	return nil
}

func samePath(left string, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}

func syncPersistedDir(dir string) (err error) {
	handle, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open config directory %q for sync: %w", dir, err)
	}
	defer func() {
		if closeErr := handle.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close config directory %q: %w", dir, closeErr))
		}
	}()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("sync config directory %q: %w", dir, err)
	}
	return nil
}

func closeFileAfterError(file *os.File, path string, cause error) error {
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(cause, fmt.Errorf("close file %q after error: %w", path, closeErr))
	}
	return cause
}
