//go:build !windows

package daemon

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// loopGateProcessReaped reports a pid that can no longer run. A zombie still answers signal 0, so
// Linux consults its procfs state instead of waiting on the reaper that inherited it.
func loopGateProcessReaped(t *testing.T, pid int) bool {
	t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil || process.Signal(syscall.Signal(0)) != nil {
		return true
	}
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	_, state, found := strings.CutLast(string(data), ") ")
	if !found {
		return false
	}
	return strings.HasPrefix(state, "Z")
}
