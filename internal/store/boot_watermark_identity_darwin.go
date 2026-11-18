//go:build darwin

package store

import (
	"fmt"
	"os"
	"syscall"
)

// fileStampIdentity names the inode a stamp was taken from and the last time it changed.
func fileStampIdentity(info os.FileInfo) (string, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return "", false
	}
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Ctimespec.Sec, stat.Ctimespec.Nsec), true
}
