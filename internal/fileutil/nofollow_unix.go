//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openNoFollow(path string, expectation entryExpectation, _ openIntent) (*os.File, error) {
	root, parts, err := unixPathParts(path)
	if err != nil {
		return nil, err
	}
	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, unixOpenError(err)
	}

	for index, part := range parts {
		nextExpectation := entryDirectory
		if index == len(parts)-1 {
			nextExpectation = expectation
		}
		nextFD, err := openUnixAt(fd, part, nextExpectation)
		if err != nil {
			if closeErr := unix.Close(fd); closeErr != nil {
				return nil, errors.Join(
					unixOpenError(err),
					fmt.Errorf("fileutil: close parent directory: %w", closeErr),
				)
			}
			return nil, unixOpenError(err)
		}
		if index < len(parts)-1 {
			if err := ensureUnixDirectory(nextFD); err != nil {
				if closeErr := unix.Close(nextFD); closeErr != nil {
					if parentCloseErr := unix.Close(fd); parentCloseErr != nil {
						return nil, errors.Join(
							err,
							fmt.Errorf("fileutil: close child after directory validation: %w", closeErr),
							fmt.Errorf("fileutil: close parent directory: %w", parentCloseErr),
						)
					}
					return nil, errors.Join(
						err,
						fmt.Errorf("fileutil: close child after directory validation: %w", closeErr),
					)
				}
				if parentCloseErr := unix.Close(fd); parentCloseErr != nil {
					return nil, errors.Join(
						err,
						fmt.Errorf("fileutil: close parent directory: %w", parentCloseErr),
					)
				}
				return nil, err
			}
		}
		if closeErr := unix.Close(fd); closeErr != nil {
			if childCloseErr := unix.Close(nextFD); childCloseErr != nil {
				return nil, errors.Join(
					fmt.Errorf("fileutil: close parent directory: %w", closeErr),
					fmt.Errorf("fileutil: close child after parent close failure: %w", childCloseErr),
				)
			}
			return nil, fmt.Errorf("fileutil: close parent directory: %w", closeErr)
		}
		fd = nextFD
	}

	return newUnixFile(fd, path)
}

func openNoFollowAt(parent *os.File, name string, expectation entryExpectation, _ openIntent) (*os.File, error) {
	parentFD, err := unixFileDescriptor(parent)
	if err != nil {
		return nil, err
	}

	fd, err := openUnixAt(parentFD, name, expectation)
	if err != nil {
		return nil, unixOpenError(err)
	}

	return newUnixFile(fd, name)
}

func unixFileDescriptor(file *os.File) (int, error) {
	fd := file.Fd()
	if fd > ^uintptr(0)>>1 {
		return 0, fmt.Errorf("fileutil: file descriptor %d exceeds int range", fd)
	}
	return int(fd), nil
}

func newUnixFile(fd int, name string) (*os.File, error) {
	if fd < 0 {
		return nil, fmt.Errorf("fileutil: invalid file descriptor %d", fd)
	}

	file := os.NewFile(uintptr(fd), name)
	if file != nil {
		return file, nil
	}
	if closeErr := unix.Close(fd); closeErr != nil {
		return nil, errors.Join(
			errors.New("fileutil: create file handle"),
			fmt.Errorf("fileutil: close file descriptor: %w", closeErr),
		)
	}
	return nil, errors.New("fileutil: create file handle")
}

func openUnixAt(parentFD int, name string, _ entryExpectation) (int, error) {
	return unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
}

func ensureUnixDirectory(fd int) error {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return fmt.Errorf("fileutil: stat parent directory handle: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return ErrNotDirectory
	}
	return nil
}

func unixPathParts(path string) (string, []string, error) {
	cleaned, err := normalizeUnixSystemRootAlias(filepath.Clean(path))
	if err != nil {
		return "", nil, err
	}
	if filepath.IsAbs(cleaned) {
		trimmed := strings.TrimPrefix(cleaned, string(filepath.Separator))
		return string(filepath.Separator), splitUnixPath(trimmed), nil
	}
	return ".", splitUnixPath(cleaned), nil
}

func splitUnixPath(path string) []string {
	if path == "." || path == "" {
		return nil
	}
	return strings.Split(path, string(filepath.Separator))
}

func unixOpenError(err error) error {
	if errors.Is(err, unix.ELOOP) {
		return fmt.Errorf("%w: %w", ErrSymlink, err)
	}
	return err
}
