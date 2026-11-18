package projectfiles

import (
	"errors"
	"os"
)

// OpenRegularFile confines the open and rejects special descriptors even after a path swap.
func OpenRegularFile(root *os.Root, path string) (*os.File, error) {
	info, err := root.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotAFile
	}
	file, err := root.OpenFile(path, os.O_RDONLY|regularOpenFlags, 0)
	if err != nil {
		return nil, err
	}
	info, err = file.Stat()
	if err == nil && info.Mode().IsRegular() {
		return file, nil
	}
	if err == nil {
		err = ErrNotAFile
	}
	return nil, errors.Join(err, file.Close())
}
