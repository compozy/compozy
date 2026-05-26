//go:build !windows

package daemon

import (
	"fmt"
	"os"
)

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("daemon: open directory %q for sync: %w", path, err)
	}
	defer func() {
		_ = dir.Close()
	}()

	if err := dir.Sync(); err != nil {
		return fmt.Errorf("daemon: sync directory %q: %w", path, err)
	}
	return nil
}
