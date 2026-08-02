//go:build !linux

package sessiondb

import "os"

func sessionDBFileChangeToken(info os.FileInfo) string {
	return info.ModTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
}
