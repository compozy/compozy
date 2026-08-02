//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package fileutil

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

type directoryMoveIdentity struct {
	device uint64
	inode  uint64
	typeID uint64
}

type directoryMoveIdentityInteger interface {
	int | int8 | int16 | int32 | int64 | uint | uint8 | uint16 | uint32 | uint64 | uintptr
}

func directoryMoveIdentityFromFile(file *os.File) (directoryMoveIdentity, error) {
	fileDescriptor, err := unixFileDescriptor(file)
	if err != nil {
		return directoryMoveIdentity{}, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fileDescriptor, &stat); err != nil {
		return directoryMoveIdentity{}, fmt.Errorf("fileutil: stat move source handle: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFDIR {
		return directoryMoveIdentity{}, ErrNotDirectory
	}
	return directoryMoveIdentity{
		device: directoryMoveIdentityValue(stat.Dev),
		inode:  directoryMoveIdentityValue(stat.Ino),
		typeID: directoryMoveIdentityValue(stat.Mode & unix.S_IFMT),
	}, nil
}

func directoryMoveIdentityValue[T directoryMoveIdentityInteger](value T) uint64 {
	return uint64(value)
}

func directoryMoveIdentityMatches(want directoryMoveIdentity, file *os.File) (bool, error) {
	got, err := directoryMoveIdentityFromFile(file)
	if err != nil {
		return false, err
	}
	return want == got, nil
}

func unixFileLinkCount(file *os.File) (uint64, error) {
	fileDescriptor, err := unixFileDescriptor(file)
	if err != nil {
		return 0, err
	}
	var stat unix.Stat_t
	if err := unix.Fstat(fileDescriptor, &stat); err != nil {
		return 0, fmt.Errorf("fileutil: stat retained entry link count: %w", err)
	}
	return directoryMoveIdentityValue(stat.Nlink), nil
}
