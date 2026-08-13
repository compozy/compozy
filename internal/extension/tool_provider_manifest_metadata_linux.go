//go:build linux

package extensionpkg

import (
	"fmt"
	"os"
	"syscall"
)

func extensionManifestPlatformChangeToken(info os.FileInfo) (string, bool) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return "", false
	}
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Ctim.Sec, stat.Ctim.Nsec), true
}
