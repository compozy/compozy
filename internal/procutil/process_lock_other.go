//go:build !linux

package procutil

// OwnsFileLock leaves process identity verification unchanged outside Linux.
func OwnsFileLock(_ int, _ string) (bool, error) {
	return false, nil
}
