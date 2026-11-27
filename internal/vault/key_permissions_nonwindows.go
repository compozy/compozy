//go:build !windows

package vault

import (
	"fmt"
	"os"
)

func validateKeyFilePermissions(path string, perm os.FileMode) error {
	if perm&0o077 != 0 {
		return fmt.Errorf("vault: key file %q permissions %o must not grant group or other permissions", path, perm)
	}
	return nil
}
