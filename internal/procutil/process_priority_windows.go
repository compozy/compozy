//go:build windows

package procutil

// SetProcessGroupNiceness is a no-op on Windows; job objects do not expose
// Unix niceness, and the agent process tree keeps its default priority class.
func SetProcessGroupNiceness(_ int, _ int) error {
	return nil
}
