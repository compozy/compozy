package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func readOptionalRegularFile(path string, label string) ([]byte, bool, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return nil, false, fmt.Errorf("config: %s path is required", label)
	}

	content, _, err := fileutil.ReadRegularFile(normalizedPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if errors.Is(err, fileutil.ErrSymlink) {
			return nil, false, fmt.Errorf(
				"config: %s %q must be a regular file, not a symlink",
				label,
				normalizedPath,
			)
		}
		if errors.Is(err, fileutil.ErrDirectory) {
			return nil, false, fmt.Errorf(
				"config: %s %q must be a regular file, not a directory",
				label,
				normalizedPath,
			)
		}
		if errors.Is(err, fileutil.ErrNotRegular) {
			return nil, false, fmt.Errorf("config: %s %q must be a regular file", label, normalizedPath)
		}
		return nil, false, fmt.Errorf("config: read %s %q: %w", label, normalizedPath, err)
	}
	return content, true, nil
}

func readOptionalRegularFileFromDirectory(
	directory *fileutil.Directory,
	name string,
	path string,
	label string,
) ([]byte, bool, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return nil, false, fmt.Errorf("config: %s path is required", label)
	}

	content, _, err := directory.ReadRegularFile(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		if errors.Is(err, fileutil.ErrSymlink) {
			return nil, false, fmt.Errorf(
				"config: %s %q must be a regular file, not a symlink",
				label,
				normalizedPath,
			)
		}
		if errors.Is(err, fileutil.ErrDirectory) {
			return nil, false, fmt.Errorf(
				"config: %s %q must be a regular file, not a directory",
				label,
				normalizedPath,
			)
		}
		if errors.Is(err, fileutil.ErrNotRegular) {
			return nil, false, fmt.Errorf("config: %s %q must be a regular file", label, normalizedPath)
		}
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
	directory, name, err := fileutil.OpenOrCreateParentDirectory(path, privateDirMode)
	if err != nil {
		return fmt.Errorf("open config parent directory for %q: %w", path, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close config parent directory for %q: %w", path, closeErr))
		}
	}()
	return writePersistedFileInDirectory(directory, name, path, contents, replace)
}

func writePersistedFileInDirectory(
	directory *fileutil.Directory,
	name string,
	path string,
	contents []byte,
	replace bool,
) error {
	if err := directory.AtomicWriteFile(name, contents, privateFileMode, replace); err != nil {
		return fmt.Errorf("write config file %q: %w", path, err)
	}
	return nil
}

func samePath(left string, right string) bool {
	return strings.TrimSpace(left) == strings.TrimSpace(right)
}
