package vault

import "os"

func validateKeyFilePermissions(_ string, _ os.FileMode) error {
	// Go exposes synthetic POSIX mode bits on Windows. Filesystem ACLs govern access.
	return nil
}
