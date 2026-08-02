package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"path/filepath"
	"slices"

	"github.com/compozy/compozy/internal/fileutil"
)

func computeInstallChecksumDirectory(root *fileutil.Directory) (string, error) {
	if root == nil {
		return "", ErrArchiveRootRequired
	}
	hasher := sha256.New()
	if err := writeInstallChecksumDirectory(hasher, root, ""); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func writeInstallChecksumDirectory(hasher hash.Hash, directory *fileutil.Directory, prefix string) error {
	names, err := directory.ReadDir()
	if err != nil {
		return fmt.Errorf("registry: read checksum directory: %w", err)
	}
	slices.Sort(names)
	for _, name := range names {
		relativePath := name
		if prefix != "" {
			relativePath = filepath.Join(prefix, name)
		}
		child, openErr := directory.OpenDirectory(name)
		if openErr == nil {
			writeErr := writeInstallChecksumDirectory(hasher, child, relativePath)
			closeErr := child.Close()
			if writeErr != nil || closeErr != nil {
				return errors.Join(
					writeErr,
					closeChecksumDirectory(relativePath, closeErr),
				)
			}
			continue
		}
		if !errors.Is(openErr, fileutil.ErrNotDirectory) {
			return fmt.Errorf(
				"registry: open checksum directory %q: %w",
				relativePath,
				mapExtractionAccessError(openErr),
			)
		}
		if err := writeInstallChecksumFile(hasher, directory, name, relativePath); err != nil {
			return err
		}
	}
	return nil
}

func writeInstallChecksumFile(
	hasher hash.Hash,
	directory *fileutil.Directory,
	name string,
	relativePath string,
) (err error) {
	file, err := directory.OpenRegularFile(name)
	if err != nil {
		return fmt.Errorf("registry: open checksum path %q: %w", relativePath, mapExtractionAccessError(err))
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("registry: close checksum path %q: %w", relativePath, closeErr))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("registry: stat checksum path %q: %w", relativePath, err)
	}

	normalizedPath := filepath.ToSlash(relativePath)
	if err := writeInstallChecksumString(
		hasher,
		fmt.Sprintf("file:%s\nmode:%#o\n", normalizedPath, info.Mode().Perm()),
	); err != nil {
		return err
	}
	if _, err := io.Copy(hasher, file); err != nil {
		return fmt.Errorf("registry: hash regular file %q: %w", relativePath, err)
	}
	if _, err := hasher.Write([]byte{0}); err != nil {
		return fmt.Errorf("registry: hash separator for %q: %w", relativePath, err)
	}
	return nil
}

func closeChecksumDirectory(relativePath string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("registry: close checksum directory %q: %w", relativePath, err)
}

func writeInstallChecksumString(hasher hash.Hash, value string) error {
	if _, err := hasher.Write([]byte(value)); err != nil {
		return fmt.Errorf("registry: hash payload metadata: %w", err)
	}
	return nil
}
