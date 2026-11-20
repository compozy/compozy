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
	// Dev and Ino widths vary by platform, so the conversions are not redundant everywhere.
	return platformFileID{device: uint64(stat.Dev), inode: uint64(stat.Ino)} //nolint:unconvert
}
