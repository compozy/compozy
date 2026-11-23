package fileutil

import "os"

// TryLockFile takes an exclusive, non-blocking OS lock on file. It reports
// false without an error when another open file description holds the lock.
func TryLockFile(file *os.File) (bool, error) {
	return tryLockFile(file)
}

// UnlockFile releases a lock taken by TryLockFile.
func UnlockFile(file *os.File) error {
	return unlockFile(file)
}
