//go:build !windows

package daemon

// lockPIDPath returns the path used to persist the daemon PID.
// On non-Windows platforms, advisory locks don't block writes from other file
// descriptors, so the PID can be stored inside the lock file itself.
func lockPIDPath(lockPath string) string {
	return lockPath
}
