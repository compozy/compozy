//go:build !windows

package daemon

import (
	"errors"
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
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return loopGateSignalReportsGone(err)
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

func loopGateSignalReportsGone(err error) bool {
	return errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH)
}

// Invariant: permission failure cannot prove process absence. Owner: daemon
// adapter process-probe assertions; canonical Unix probe suite.
func TestLoopGateProcessProbeShouldRequireConfirmedAbsence(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		gone bool
	}{
		{name: "live"},
		{name: "permission denied", err: os.NewSyscallError("signal", syscall.EPERM)},
		{name: "missing", err: os.NewSyscallError("signal", syscall.ESRCH), gone: true},
		{name: "reaped", err: os.ErrProcessDone, gone: true},
	} {
		t.Run("Should classify "+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := loopGateSignalReportsGone(tc.err); got != tc.gone {
				t.Fatalf("process gone = %v, want %v for %v", got, tc.gone, tc.err)
			}
		})
	}
}
