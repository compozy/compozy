//go:build !windows

package cli

import (
	"errors"
	"fmt"
	"os"
)

func syncAppDiagnosticBundleDirectory(path string) (err error) {
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open bundle output directory: %w", err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			closeErr = fmt.Errorf("close bundle output directory: %w", closeErr)
			if err == nil {
				err = closeErr
			} else {
				err = errors.Join(err, closeErr)
			}
		}
	}()
	if err := directory.Sync(); err != nil {
		return fmt.Errorf("sync bundle output directory: %w", err)
	}
	return nil
}
