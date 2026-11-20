//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package session

import (
	"os"
	"syscall"
)

type platformFileID struct {
	device uint64
	inode  uint64
}

func platformFileIDOf(info os.FileInfo) platformFileID {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return platformFileID{}
	}
	device, inode := uint64(stat.Dev), uint64(stat.Ino) //nolint:unconvert // Widths vary by platform.
	return platformFileID{device: device, inode: inode}
}
