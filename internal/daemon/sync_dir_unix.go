//go:build !windows

package daemon

import (
	"errors"
	"fmt"
	"os"
)

// syncDir fsyncs the directory entry so an atomic rename becomes durable.
// Non-Windows platforms allow opening a directory read-only and calling Sync;
// the Windows sibling in sync_dir_windows.go is a no-op because
// FlushFileBuffers requires a writable handle.
func syncDir(path string) (returnErr error) {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("daemon: open directory %q for sync: %w", path, err)
	}
	defer func() {
		if err := dir.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("daemon: close directory %q after sync: %w", path, err))
		}
	}()

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("daemon: sync directory %q: %w", path, err)
	}
	return nil
}
