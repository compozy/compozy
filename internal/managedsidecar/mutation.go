package managedsidecar

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/compozy/compozy/internal/fileutil"
)

// Read reads the managed regular file through a no-follow directory chain.
func (p Path) Read() (contents []byte, err error) {
	parent, name, err := p.openParent(false)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("managed sidecar: close read parent: %w", closeErr))
		}
	}()
	contents, _, err = parent.ReadRegularFile(name)
	if err != nil {
		return nil, fmt.Errorf("managed sidecar: read %q: %w", p.SourcePath(), err)
	}
	return contents, nil
}

// Write atomically replaces the managed file through a held no-follow parent.
func (p Path) Write(contents []byte, mode os.FileMode) (err error) {
	if issue := p.ValidateNoSymlinks(); issue != nil {
		return issue
	}
	parent, name, err := p.openParent(true)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("managed sidecar: close write parent: %w", closeErr))
		}
	}()
	if err := parent.AtomicWriteFile(name, contents, mode, true); err != nil {
		return fmt.Errorf("managed sidecar: write %q: %w", p.SourcePath(), err)
	}
	return nil
}

// Remove deletes the managed regular file through a held no-follow parent.
func (p Path) Remove() (err error) {
	parent, name, err := p.openParent(true)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := parent.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("managed sidecar: close remove parent: %w", closeErr))
		}
	}()
	if err := parent.RemoveRegularFile(name); err != nil {
		return fmt.Errorf("managed sidecar: remove %q: %w", p.SourcePath(), err)
	}
	return nil
}

func (p Path) openParent(mutate bool) (*fileutil.Directory, string, error) {
	components := pathComponents(p.relative)
	if len(components) == 0 {
		return nil, "", errors.New("managed sidecar: file path is required")
	}
	current, err := openRootDirectory(p.root, mutate)
	if err != nil {
		return nil, "", fmt.Errorf("managed sidecar: open root %q: %w", p.root, err)
	}
	for _, component := range components[:len(components)-1] {
		next, openErr := openChildDirectory(current, component, mutate)
		closeErr := current.Close()
		if openErr != nil {
			return nil, "", errors.Join(
				fmt.Errorf("managed sidecar: open parent %q: %w", filepath.ToSlash(component), openErr),
				closeErr,
			)
		}
		if closeErr != nil {
			return nil, "", errors.Join(closeErr, next.Close())
		}
		current = next
	}
	return current, components[len(components)-1], nil
}

func openRootDirectory(path string, mutate bool) (*fileutil.Directory, error) {
	if mutate {
		return fileutil.OpenDirectoryForMutation(path)
	}
	return fileutil.OpenDirectory(path)
}

func openChildDirectory(parent *fileutil.Directory, name string, mutate bool) (*fileutil.Directory, error) {
	if mutate {
		return parent.OpenDirectoryForMutation(name)
	}
	return parent.OpenDirectory(name)
}
