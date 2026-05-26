//go:build windows

package daemon

// lockPIDPath returns the path used to persist the daemon PID.
// On Windows, flock's LockFileEx acquires a byte-range lock that prevents
// other file handles — even within the same process — from writing to the
// locked file. A separate .pid sidecar avoids that conflict.
func lockPIDPath(lockPath string) string {
	return lockPath + ".pid"
}
