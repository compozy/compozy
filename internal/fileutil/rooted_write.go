package fileutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const rootedTemporaryFileAttempts = 100

// OpenOrCreateDirectory opens a non-link directory chain, creating missing directories.
func OpenOrCreateDirectory(path string, perm os.FileMode) (*Directory, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}

	file, err := openOrCreateDirectoryNoFollow(path, perm)
	if err != nil {
		return nil, fmt.Errorf("fileutil: open or create directory: %w", err)
	}
	if err := ensureDirectory(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	return &Directory{file: file}, nil
}

// OpenParentDirectory opens the existing parent directory and returns the target basename.
func OpenParentDirectory(path string) (*Directory, string, error) {
	if err := validatePath(path); err != nil {
		return nil, "", err
	}

	name := filepath.Base(path)
	if err := validateChildName(name); err != nil {
		return nil, "", err
	}

	directory, err := openDirectory(filepath.Dir(path), openIntentMutate)
	if err != nil {
		return nil, "", err
	}
	return directory, name, nil
}

// OpenOrCreateParentDirectory opens or creates the parent directory and returns the target basename.
func OpenOrCreateParentDirectory(path string, perm os.FileMode) (*Directory, string, error) {
	if err := validatePath(path); err != nil {
		return nil, "", err
	}

	name := filepath.Base(path)
	if err := validateChildName(name); err != nil {
		return nil, "", err
	}

	directory, err := OpenOrCreateDirectory(filepath.Dir(path), perm)
	if err != nil {
		return nil, "", err
	}
	return directory, name, nil
}

// CreateDirectory creates and opens one direct child directory without following links.
func (d *Directory) CreateDirectory(name string, perm os.FileMode) (*Directory, error) {
	if d == nil || d.file == nil {
		return nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(name); err != nil {
		return nil, err
	}

	file, err := createDirectoryNoFollowAt(d.file, name, perm)
	if err != nil {
		return nil, fmt.Errorf("fileutil: create directory child: %w", err)
	}
	if err := ensureDirectory(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	return &Directory{file: file}, nil
}

// CreateRegularFile creates one direct regular-file child without following links.
func (d *Directory) CreateRegularFile(name string, perm os.FileMode) (*os.File, error) {
	if d == nil || d.file == nil {
		return nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(name); err != nil {
		return nil, err
	}

	file, err := createFileNoFollowAt(d.file, name, perm)
	if err != nil {
		return nil, fmt.Errorf("fileutil: create regular file child: %w", err)
	}
	if _, err := ensureRegularFile(file); err != nil {
		return nil, closeFileWithError(file, err)
	}
	return file, nil
}

// CreateTemporaryDirectory creates and opens a direct child directory with a
// cryptographically random suffix.
func (d *Directory) CreateTemporaryDirectory(prefix string, perm os.FileMode) (string, *Directory, error) {
	if d == nil || d.file == nil {
		return "", nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateTemporaryPrefix(prefix); err != nil {
		return "", nil, err
	}

	for range rootedTemporaryFileAttempts {
		var randomBytes [16]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return "", nil, fmt.Errorf("fileutil: generate temporary directory name: %w", err)
		}
		name := prefix + hex.EncodeToString(randomBytes[:])
		directory, err := d.CreateDirectory(name, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("fileutil: create temporary directory: %w", err)
		}
		return name, directory, nil
	}
	return "", nil, errors.New("fileutil: create temporary directory: exhausted unique names")
}

// Stat returns metadata from the held directory handle.
func (d *Directory) Stat() (os.FileInfo, error) {
	if d == nil || d.file == nil {
		return nil, fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	info, err := d.file.Stat()
	if err != nil {
		return nil, fmt.Errorf("fileutil: stat directory: %w", err)
	}
	return info, nil
}

// Chmod changes permissions through the held directory handle.
func (d *Directory) Chmod(mode os.FileMode) error {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := d.file.Chmod(mode); err != nil {
		return fmt.Errorf("fileutil: chmod directory: %w", err)
	}
	return nil
}

// AtomicWriteFile writes a direct child through a temporary file and publishes it atomically.
// When replace is false, publication fails if a child with name already exists.
func (d *Directory) AtomicWriteFile(name string, contents []byte, perm os.FileMode, replace bool) (err error) {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(name); err != nil {
		return err
	}

	temporaryName, temporary, err := d.createTemporaryFile(perm)
	if err != nil {
		return err
	}
	temporaryOwner := newCloseOwner(temporary.Close)
	cleanup := true
	defer func() {
		err = temporaryOwner.closeWithError(err, fmt.Sprintf("fileutil: close temporary file %q", temporaryName))
		if cleanup {
			if removeErr := removeNoFollowAt(d.file, temporaryName, false); removeErr != nil &&
				!errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("fileutil: remove temporary file %q: %w", temporaryName, removeErr))
			}
		}
	}()

	if err := writeAndSyncFile(temporary, contents, perm); err != nil {
		return fmt.Errorf("fileutil: prepare temporary file %q: %w", temporaryName, err)
	}
	if err := publishNoFollowAt(d.file, temporary, temporaryName, name, replace); err != nil {
		return fmt.Errorf("fileutil: publish file %q: %w", name, err)
	}
	cleanup = false
	if err := temporaryOwner.closeWithError(err, fmt.Sprintf("fileutil: close published file %q", name)); err != nil {
		return err
	}
	if err := d.Sync(); err != nil {
		return err
	}
	return nil
}

// RemoveRegularFile removes a direct regular-file child without following links.
func (d *Directory) RemoveRegularFile(name string) (err error) {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(name); err != nil {
		return err
	}

	file, err := d.OpenRegularFile(name)
	if err != nil {
		return err
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("fileutil: close regular file %q before remove: %w", name, closeErr)
	}
	if err := removeNoFollowAt(d.file, name, false); err != nil {
		return fmt.Errorf("fileutil: remove regular file %q: %w", name, err)
	}
	return d.Sync()
}

// RemoveAll removes a direct child tree without traversing links or reparse points.
func (d *Directory) RemoveAll(name string) (err error) {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := validateChildName(name); err != nil {
		return err
	}

	child, err := d.OpenDirectory(name)
	if err == nil {
		if err := child.removeContents(); err != nil {
			closeErr := child.Close()
			if closeErr != nil {
				return errors.Join(err, closeErr)
			}
			return err
		}
		if err := child.Close(); err != nil {
			return fmt.Errorf("fileutil: close directory %q before remove: %w", name, err)
		}
		if err := removeNoFollowAt(d.file, name, true); err != nil {
			return fmt.Errorf("fileutil: remove directory %q: %w", name, err)
		}
		return d.Sync()
	}
	if !errors.Is(err, ErrNotDirectory) {
		return err
	}
	return d.RemoveRegularFile(name)
}

// CopyContentsFrom copies every child from source into d, excluding one direct child name.
func (d *Directory) CopyContentsFrom(source *Directory, excludedName string) error {
	if d == nil || d.file == nil || source == nil || source.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if excludedName != "" {
		if err := validateChildName(excludedName); err != nil {
			return err
		}
	}
	if err := copyDirectoryContents(d, source, excludedName); err != nil {
		return err
	}
	return d.Sync()
}

// Sync flushes held directory metadata when the platform supports it. On
// Windows, the Directory must have been opened for mutation because the OS
// requires write access for FlushFileBuffers.
func (d *Directory) Sync() error {
	if d == nil || d.file == nil {
		return fmt.Errorf("%w: directory is closed", ErrInvalidPath)
	}
	if err := syncDirectoryHandle(d.file); err != nil {
		return fmt.Errorf("fileutil: sync directory: %w", err)
	}
	return nil
}

func (d *Directory) createTemporaryFile(perm os.FileMode) (string, *os.File, error) {
	for range rootedTemporaryFileAttempts {
		var randomBytes [16]byte
		if _, err := rand.Read(randomBytes[:]); err != nil {
			return "", nil, fmt.Errorf("fileutil: generate temporary file name: %w", err)
		}
		name := ".compozy-tmp-" + hex.EncodeToString(randomBytes[:])
		file, err := createFileForMoveNoFollowAt(d.file, name, perm)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, fmt.Errorf("fileutil: create temporary file: %w", err)
		}
		return name, file, nil
	}
	return "", nil, errors.New("fileutil: create temporary file: exhausted unique names")
}

func validateTemporaryPrefix(prefix string) error {
	if err := validateChildName(prefix + "x"); err != nil {
		return fmt.Errorf("fileutil: temporary directory prefix: %w", err)
	}
	return nil
}

func (d *Directory) removeContents() error {
	names, err := d.ReadDir()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := d.RemoveAll(name); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectoryContents(target *Directory, source *Directory, excludedName string) error {
	names, err := source.ReadDir()
	if err != nil {
		return err
	}
	for _, name := range names {
		if name == excludedName {
			continue
		}
		if err := copyDirectoryChild(target, source, name); err != nil {
			return err
		}
	}
	return nil
}

func copyDirectoryChild(target *Directory, source *Directory, name string) (err error) {
	sourceDirectory, err := source.OpenDirectory(name)
	if err == nil {
		defer func() {
			if closeErr := sourceDirectory.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("fileutil: close source directory %q: %w", name, closeErr))
			}
		}()

		targetDirectory, err := target.CreateDirectory(name, 0o700)
		if err != nil {
			return err
		}
		copyErr := copyDirectoryContents(targetDirectory, sourceDirectory, "")
		syncErr := targetDirectory.Sync()
		closeErr := targetDirectory.Close()
		if copyErr != nil || syncErr != nil || closeErr != nil {
			if cleanupErr := target.RemoveAll(name); cleanupErr != nil {
				return errors.Join(copyErr, syncErr, closeErr, cleanupErr)
			}
			return errors.Join(copyErr, syncErr, closeErr)
		}
		return nil
	}
	if !errors.Is(err, ErrNotDirectory) {
		return err
	}
	return copyRegularFile(target, source, name)
}

func copyRegularFile(target *Directory, source *Directory, name string) (err error) {
	sourceFile, info, err := source.openRegularFile(name)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("fileutil: close source file %q: %w", name, closeErr))
		}
	}()

	targetFile, err := createFileNoFollowAt(target.file, name, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("fileutil: create target file %q: %w", name, err)
	}
	targetOwner := newCloseOwner(targetFile.Close)
	cleanup := true
	defer func() {
		err = targetOwner.closeWithError(err, fmt.Sprintf("fileutil: close target file %q", name))
		if cleanup {
			removeErr := removeNoFollowAt(target.file, name, false)
			if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("fileutil: remove incomplete target file %q: %w", name, removeErr))
			}
		}
	}()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("fileutil: copy file %q: %w", name, err)
	}
	if err := targetFile.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("fileutil: set target file mode %q: %w", name, err)
	}
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("fileutil: sync target file %q: %w", name, err)
	}
	if err := targetOwner.closeWithError(err, fmt.Sprintf("fileutil: close target file %q", name)); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func writeAndSyncFile(file *os.File, contents []byte, perm os.FileMode) error {
	if err := file.Chmod(perm); err != nil {
		return fmt.Errorf("set file mode: %w", err)
	}
	written, err := file.Write(contents)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if written != len(contents) {
		return io.ErrShortWrite
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync file: %w", err)
	}
	return nil
}
