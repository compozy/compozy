//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func openOrCreateDirectoryNoFollow(path string, perm os.FileMode) (*os.File, error) {
	root, parts, err := unixPathParts(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, unixOpenError(err)
	}

	for _, part := range parts {
		nextFD, err := openOrCreateUnixDirectoryAt(fd, part, perm)
		if err != nil {
			if closeErr := unix.Close(fd); closeErr != nil {
				return nil, errors.Join(
					unixOpenError(err),
					fmt.Errorf("fileutil: close parent directory: %w", closeErr),
				)
			}
			return nil, unixOpenError(err)
		}
		if closeErr := unix.Close(fd); closeErr != nil {
			if childCloseErr := unix.Close(nextFD); childCloseErr != nil {
				return nil, errors.Join(
					fmt.Errorf("fileutil: close parent directory: %w", closeErr),
					fmt.Errorf("fileutil: close child directory: %w", childCloseErr),
				)
			}
			return nil, fmt.Errorf("fileutil: close parent directory: %w", closeErr)
		}
		fd = nextFD
	}

	return newUnixFile(fd, path)
}

func createDirectoryNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	parentFD, err := unixFileDescriptor(parent)
	if err != nil {
		return nil, err
	}
	if err := unix.Mkdirat(parentFD, name, uint32(perm.Perm())); err != nil {
		return nil, unixOpenError(err)
	}
	fd, err := openUnixAt(parentFD, name, entryDirectory)
	if err != nil {
		return nil, unixOpenError(err)
	}
	if err := ensureUnixDirectory(fd); err != nil {
		if closeErr := unix.Close(fd); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("fileutil: close created directory: %w", closeErr))
		}
		return nil, err
	}
	return newUnixFile(fd, name)
}

func createFileNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	parentFD, err := unixFileDescriptor(parent)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Openat(
		parentFD,
		name,
		unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,
		uint32(perm.Perm()),
	)
	if err != nil {
		return nil, unixOpenError(err)
	}
	return newUnixFile(fd, name)
}

func createFileForMoveNoFollowAt(parent *os.File, name string, perm os.FileMode) (*os.File, error) {
	return createFileNoFollowAt(parent, name, perm)
}

func publishNoFollowAt(parent *os.File, _ *os.File, sourceName string, targetName string, replace bool) error {
	parentFD, err := unixFileDescriptor(parent)
	if err != nil {
		return err
	}
	if replace {
		return unix.Renameat(parentFD, sourceName, parentFD, targetName)
	}
	if err := unix.Linkat(parentFD, sourceName, parentFD, targetName, 0); err != nil {
		return err
	}
	if err := unix.Unlinkat(parentFD, sourceName, 0); err != nil {
		return err
	}
	return nil
}

func removeNoFollowAt(parent *os.File, name string, directory bool) error {
	parentFD, err := unixFileDescriptor(parent)
	if err != nil {
		return err
	}
	flags := 0
	if directory {
		flags = unix.AT_REMOVEDIR
	}
	return unix.Unlinkat(parentFD, name, flags)
}

// unlinkat returns these when the entry may still be a directory needing removal of its contents.
func isDirectoryUnlinkRejection(err error) bool {
	return errors.Is(err, unix.EISDIR) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EACCES)
}

func syncDirectoryHandle(file *os.File) error {
	return file.Sync()
}

func openOrCreateUnixDirectoryAt(parentFD int, name string, perm os.FileMode) (int, error) {
	fd, err := openUnixAt(parentFD, name, entryDirectory)
	if err == nil {
		if ensureErr := ensureUnixDirectory(fd); ensureErr != nil {
			if closeErr := unix.Close(fd); closeErr != nil {
				return 0, errors.Join(ensureErr, fmt.Errorf("fileutil: close directory after validation: %w", closeErr))
			}
			return 0, ensureErr
		}
		return fd, nil
	}
	if !errors.Is(err, unix.ENOENT) {
		return 0, err
	}
	if err := unix.Mkdirat(parentFD, name, uint32(perm.Perm())); err != nil && !errors.Is(err, unix.EEXIST) {
		return 0, err
	}
	fd, err = openUnixAt(parentFD, name, entryDirectory)
	if err != nil {
		return 0, err
	}
	if ensureErr := ensureUnixDirectory(fd); ensureErr != nil {
		if closeErr := unix.Close(fd); closeErr != nil {
			return 0, errors.Join(ensureErr, fmt.Errorf("fileutil: close directory after creation: %w", closeErr))
		}
		return 0, ensureErr
	}
	return fd, nil
}
