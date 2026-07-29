//go:build windows

package daemon

// syncDir is a no-op on Windows: FlushFileBuffers requires write access on
// directory handles, but os.Open opens them read-only. Directory entry
// durability after an atomic rename is handled by the NTFS journal.
func syncDir(_ string) error {
	return nil
}
