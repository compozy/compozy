package fileutil

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// CheckPrivateFile validates permissions on the same regular-file handle used for I/O.
func CheckPrivateFile(file *os.File) error {
	info, err := ensureRegularFile(file)
	if err != nil {
		return err
	}
	return checkPrivatePermissions(file, info)
}

// CheckPrivatePermissions validates the held directory's Windows owner and ACL.
func (d *Directory) CheckPrivatePermissions() error {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	return checkPrivateDirectory(d.file)
}

// ReadPrivateFile refuses unsafe paths and permissions before reading bytes.
func ReadPrivateFile(path string) ([]byte, error) {
	file, err := OpenRegularFile(path)
	if err != nil {
		return nil, err
	}
	return readPrivateFile(file)
}

// ReadPrivateFile reads a private child from the held directory.
func (d *Directory) ReadPrivateFile(name string) ([]byte, error) {
	file, err := d.OpenRegularFile(name)
	if err != nil {
		return nil, err
	}
	return readPrivateFile(file)
}

func readPrivateFile(file *os.File) (contents []byte, err error) {
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close private file: %w", closeErr))
		}
	}()
	if err := CheckPrivateFile(file); err != nil {
		return nil, err
	}
	contents, err = io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("fileutil: read private file: %w", err)
	}
	return contents, nil
}

// AtomicWritePrivateFile validates inherited protection before writing a private temporary file.
func AtomicWritePrivateFile(path string, contents []byte) (err error) {
	directory, name, err := OpenParentDirectory(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, directory.Close()) }()
	if err := directory.CheckPrivatePermissions(); err != nil {
		return err
	}
	if err := directory.checkPrivateReplacement(name); err != nil {
		return err
	}
	return directory.atomicWriteFile(name, contents, 0o600, true, true)
}

func (d *Directory) checkPrivateReplacement(name string) (err error) {
	file, err := d.OpenRegularFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	return CheckPrivateFile(file)
}
