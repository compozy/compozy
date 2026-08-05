package registry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/fileutil"
)

func openExtractionDirectory(
	root *fileutil.Directory,
	entryName string,
	create bool,
	mode os.FileMode,
) (_ *fileutil.Directory, err error) {
	if root == nil {
		return nil, ErrArchiveRootRequired
	}
	cleaned, err := cleanExtractionRelativePath(entryName)
	if err != nil {
		return nil, err
	}
	if cleaned == "." {
		return root, nil
	}

	current := root
	ownedCurrent := false
	for component := range strings.SplitSeq(cleaned, string(filepath.Separator)) {
		var next *fileutil.Directory
		var openErr error
		if create {
			next, openErr = current.OpenDirectoryForMutation(component)
		} else {
			next, openErr = current.OpenDirectory(component)
		}
		if errors.Is(openErr, os.ErrNotExist) && create {
			next, openErr = current.CreateDirectory(component, mode)
		}
		if openErr != nil {
			if ownedCurrent {
				if closeErr := current.Close(); closeErr != nil {
					return nil, errors.Join(
						mapExtractionAccessError(openErr),
						fmt.Errorf("close extraction directory after open failure: %w", closeErr),
					)
				}
			}
			return nil, mapExtractionAccessError(openErr)
		}
		if ownedCurrent {
			if closeErr := current.Close(); closeErr != nil {
				if nextCloseErr := next.Close(); nextCloseErr != nil {
					return nil, errors.Join(
						fmt.Errorf("close extraction parent directory: %w", closeErr),
						fmt.Errorf("close extraction child directory: %w", nextCloseErr),
					)
				}
				return nil, fmt.Errorf("close extraction parent directory: %w", closeErr)
			}
		}
		current = next
		ownedCurrent = true
	}
	return current, nil
}

func openExtractionParent(
	root *fileutil.Directory,
	entryName string,
	create bool,
	mode os.FileMode,
) (*fileutil.Directory, string, error) {
	cleaned, err := cleanExtractionRelativePath(entryName)
	if err != nil {
		return nil, "", err
	}
	parentPath := filepath.Dir(cleaned)
	name := filepath.Base(cleaned)
	parent, err := openExtractionDirectory(root, parentPath, create, mode)
	if err != nil {
		return nil, "", err
	}
	return parent, name, nil
}

func closeOwnedExtractionDirectory(root *fileutil.Directory, directory *fileutil.Directory, baseErr error) error {
	if directory == nil || directory == root {
		return baseErr
	}
	if closeErr := directory.Close(); closeErr != nil {
		return errors.Join(baseErr, fmt.Errorf("close extraction directory: %w", closeErr))
	}
	return baseErr
}

func mapExtractionAccessError(err error) error {
	if errors.Is(err, fileutil.ErrSymlink) {
		return fmt.Errorf("%w: extraction path", ErrPathTraversesSymlink)
	}
	return err
}
