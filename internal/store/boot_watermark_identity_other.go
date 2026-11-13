//go:build !linux && !darwin

package store

import "os"

// Platforms without an inode identity and change time cannot prove a file unchanged, so every
// session there takes the full boot checks.
func fileStampIdentity(os.FileInfo) (string, bool) {
	return "", false
}
