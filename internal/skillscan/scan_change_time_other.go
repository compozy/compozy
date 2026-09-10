//go:build !darwin && !linux

package skillscan

import "io/fs"

// Platforms without reliable change metadata rediscover definitions before reuse.
func sameFileChangeTime(fs.FileInfo, fs.FileInfo) bool {
	return false
}
