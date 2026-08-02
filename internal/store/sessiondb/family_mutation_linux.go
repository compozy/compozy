//go:build linux

package sessiondb

import (
	"fmt"
	"os"
	"syscall"
)

func sessionDBFileChangeToken(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
	}
	return fmt.Sprintf("%d:%d", stat.Ctim.Sec, stat.Ctim.Nsec)
}
