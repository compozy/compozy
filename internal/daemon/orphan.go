package daemon

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func removeStaleSocket(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}

	if err := os.Remove(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("daemon: remove stale socket %q: %w", cleanPath, err)
	}
	return nil
}
