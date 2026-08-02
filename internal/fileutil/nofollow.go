package fileutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	// ErrSymlink reports a symlink or platform reparse point in a protected path.
	ErrSymlink = errors.New("fileutil: symlink or reparse point")
	// ErrDirectory reports a directory where a regular file is required.
	ErrDirectory = errors.New("fileutil: directory")
	// ErrNotDirectory reports a non-directory where a directory is required.
	ErrNotDirectory = errors.New("fileutil: not a directory")
	// ErrNotRegular reports a special file where a regular file is required.
	ErrNotRegular = errors.New("fileutil: not a regular file")
	// ErrDirectoryDurabilityUnconfirmed reports that the platform rejected a
	// directory metadata flush, so persistence cannot be confirmed.
	ErrDirectoryDurabilityUnconfirmed = errors.New("fileutil: directory metadata durability unconfirmed")
)

type entryExpectation uint8

const (
	entryAny entryExpectation = iota
	entryDirectory
	entryRegularFile
)

type openIntent uint8

const (
	openIntentRead openIntent = iota
	openIntentMutate
	openIntentMoveSource
)

// Directory is a directory acquired without following symlinks or reparse points.
type Directory struct {
	file *os.File
	move *directoryMoveBinding
}

// OpenRegularFile opens a regular file through a non-following directory chain.
func OpenRegularFile(path string) (*os.File, error) {
	file, _, err := openRegularFile(path)
	return file, err
}

// ReadRegularFile reads a regular file through a non-following directory chain.
// It returns the metadata observed through that same file handle.
func ReadRegularFile(path string) (contents []byte, info os.FileInfo, err error) {
	file, info, err := openRegularFile(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close regular file: %w", closeErr))
		}
	}()

	contents, err = io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("fileutil: read regular file: %w", err)
	}
	return contents, info, nil
}

// OpenDirectory opens a directory through a non-following directory chain.
func OpenDirectory(path string) (*Directory, error) {
	return openDirectory(path, openIntentRead)
}

// OpenDirectoryForMutation opens a directory with the rights required for
// child creation and atomic publication.
func OpenDirectoryForMutation(path string) (*Directory, error) {
	return openDirectory(path, openIntentMutate)
}

func openDirectory(path string, intent openIntent) (*Directory, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	file, err := openNoFollow(path, entryDirectory, intent)
	if err != nil {
		return nil, fmt.Errorf("fileutil: open directory: %w", err)
	}
	if err := ensureDirectory(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	return &Directory{file: file}, nil
}

// Close releases the directory handle.
func (d *Directory) Close() error {
	if d == nil || d.file == nil {
		return nil
	}
	file := d.file
	d.file = nil
	d.move = nil
	if err := file.Close(); err != nil {
		return fmt.Errorf("fileutil: close directory: %w", err)
	}
	return nil
}

// ReadDir returns the names enumerated from the held directory handle.
func (d *Directory) ReadDir() ([]string, error) {
	if d == nil || d.file == nil {
		return nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}

	entries, err := d.file.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("fileutil: read directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

// OpenDirectory opens a direct child directory from the held directory handle.
func (d *Directory) OpenDirectory(name string) (*Directory, error) {
	return d.openDirectory(name, openIntentRead)
}

// OpenDirectoryForMutation opens a direct child directory with the rights
// required for child creation and atomic publication.
func (d *Directory) OpenDirectoryForMutation(name string) (*Directory, error) {
	return d.openDirectory(name, openIntentMutate)
}

func (d *Directory) openDirectory(name string, intent openIntent) (*Directory, error) {
	file, err := d.openChild(name, entryDirectory, intent)
	if err != nil {
		return nil, err
	}
	if err := ensureDirectory(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	return &Directory{file: file}, nil
}

// OpenRegularFile opens a direct child regular file from the held directory handle.
func (d *Directory) OpenRegularFile(name string) (*os.File, error) {
	file, _, err := d.openRegularFile(name)
	return file, err
}

// ReadRegularFile reads a direct child regular file from the held directory handle.
func (d *Directory) ReadRegularFile(name string) (contents []byte, info os.FileInfo, err error) {
	file, info, err := d.openRegularFile(name)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close regular file: %w", closeErr))
		}
	}()

	contents, err = io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("fileutil: read regular file: %w", err)
	}
	return contents, info, nil
}

func openRegularFile(path string) (*os.File, os.FileInfo, error) {
	if err := validatePath(path); err != nil {
		return nil, nil, err
	}

	file, err := openNoFollow(path, entryRegularFile, openIntentRead)
	if err != nil {
		return nil, nil, fmt.Errorf("fileutil: open regular file: %w", err)
	}
	info, err := ensureRegularFile(file)
	if err != nil {
		return nil, nil, closeFileWithError(file, err)
	}
	return file, info, nil
}

func (d *Directory) openRegularFile(name string) (*os.File, os.FileInfo, error) {
	file, err := d.openChild(name, entryRegularFile, openIntentRead)
	if err != nil {
		return nil, nil, err
	}
	info, err := ensureRegularFile(file)
	if err != nil {
		return nil, nil, closeFileWithError(file, err)
	}
	return file, info, nil
}

func (d *Directory) openChild(name string, expectation entryExpectation, intent openIntent) (*os.File, error) {
	if d == nil || d.file == nil {
		return nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := ValidatePathComponent(name); err != nil {
		return nil, err
	}

	file, err := openNoFollowAt(d.file, name, expectation, intent)
	if err != nil {
		return nil, fmt.Errorf("fileutil: open directory child: %w", err)
	}
	return file, nil
}

func ensureRegularFile(file *os.File) (os.FileInfo, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("fileutil: stat regular file handle: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, ErrSymlink
	}
	if info.IsDir() {
		return nil, ErrDirectory
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	return info, nil
}

func ensureDirectory(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("fileutil: stat directory handle: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrSymlink
	}
	if !info.IsDir() {
		return ErrNotDirectory
	}
	return nil
}

func closeFileWithError(file *os.File, cause error) error {
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(cause, fmt.Errorf("fileutil: close file after validation: %w", closeErr))
	}
	return cause
}

func validatePath(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: path is required", ErrInvalidPath)
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("%w: path contains NUL byte", ErrInvalidPath)
	}
	return nil
}

func validateChildName(name string) error {
	return ValidatePathComponent(name)
}
