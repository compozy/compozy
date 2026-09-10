//go:build darwin

package filesnap

import (
	"io/fs"
	"syscall"
)

type changeToken struct {
	device    int32
	inode     uint64
	changed   syscall.Timespec
	available bool
}

// fileChangeToken captures Darwin identity and change time without reading file content.
func fileChangeToken(info fs.FileInfo) changeToken {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return changeToken{}
	}
	return changeToken{
		device: stat.Dev, inode: stat.Ino, changed: stat.Ctimespec, available: true,
	}
}
