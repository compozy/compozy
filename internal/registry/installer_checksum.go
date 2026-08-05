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
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func computeInstallChecksumDirectory(root *fileutil.Directory) (string, error) {
	if root == nil {
		return "", ErrArchiveRootRequired
	}
	paths, err := collectInstallChecksumPaths(root, "", nil)
	if err != nil {
		return "", err
	}
	// Payloads are re-verified by a globally sorted walk, so traversal order must not reach the digest.
	slices.Sort(paths)
	hasher := sha256.New()
	if err := hashInstallChecksumPaths(hasher, root, paths); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func collectInstallChecksumPaths(
	directory *fileutil.Directory,
	prefix string,
	paths []string,
) ([]string, error) {
	names, err := directory.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("registry: read checksum directory: %w", err)
	}
	for _, name := range names {
		relativePath := name
		if prefix != "" {
			relativePath = filepath.Join(prefix, name)
		}
		child, openErr := directory.OpenDirectory(name)
		if openErr != nil {
			if !errors.Is(openErr, fileutil.ErrNotDirectory) {
				return nil, fmt.Errorf(
					"registry: open checksum directory %q: %w",
					relativePath,
					mapExtractionAccessError(openErr),
				)
			}
			paths = append(paths, relativePath)
			continue
		}
		nested, collectErr := collectInstallChecksumPaths(child, relativePath, paths)
		closeErr := child.Close()
		if collectErr != nil || closeErr != nil {
			return nil, errors.Join(collectErr, closeChecksumDirectory(relativePath, closeErr))
		}
		paths = nested
	}
	return paths, nil
}

func hashInstallChecksumPaths(hasher hash.Hash, root *fileutil.Directory, paths []string) (err error) {
	descent := installChecksumDescent{root: root}
	defer func() {
		err = errors.Join(err, descent.close())
	}()
	for _, relativePath := range paths {
		components := strings.Split(relativePath, string(filepath.Separator))
		leaf := len(components) - 1
		directory, descentErr := descent.parent(components[:leaf])
		if descentErr != nil {
			return descentErr
		}
		if writeErr := writeInstallChecksumFile(hasher, directory, components[leaf], relativePath); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

// installChecksumDescent reuses the open directory chain so a sorted payload opens each directory once.
type installChecksumDescent struct {
	root   *fileutil.Directory
	names  []string
	opened []*fileutil.Directory
}

func (d *installChecksumDescent) parent(components []string) (*fileutil.Directory, error) {
	shared := 0
	for shared < len(d.names) && shared < len(components) && d.names[shared] == components[shared] {
		shared++
	}
	if err := d.unwind(shared); err != nil {
		return nil, err
	}
	current := d.current()
	for index, name := range components[shared:] {
		child, err := current.OpenDirectory(name)
		if err != nil {
			return nil, fmt.Errorf(
				"registry: open checksum directory %q: %w",
				filepath.Join(components[:shared+index+1]...),
				mapExtractionAccessError(err),
			)
		}
		d.names = append(d.names, name)
		d.opened = append(d.opened, child)
		current = child
	}
	return current, nil
}

func (d *installChecksumDescent) current() *fileutil.Directory {
	if len(d.opened) == 0 {
		return d.root
	}
	return d.opened[len(d.opened)-1]
}

func (d *installChecksumDescent) unwind(depth int) error {
	var err error
	for len(d.opened) > depth {
		last := len(d.opened) - 1
		if closeErr := d.opened[last].Close(); closeErr != nil {
			err = errors.Join(err, closeChecksumDirectory(filepath.Join(d.names[:last+1]...), closeErr))
		}
		d.opened = d.opened[:last]
		d.names = d.names[:last]
	}
	return err
}

func (d *installChecksumDescent) close() error {
	return d.unwind(0)
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
