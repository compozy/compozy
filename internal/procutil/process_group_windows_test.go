//go:build windows

package procutil

import (
	"os"
	"os/exec"
	"testing"
)

func TestWindowsProcessJobIdentity(t *testing.T) {
	t.Run("Should not resolve a reused PID to a different command owner", func(t *testing.T) {
		const pid = 987654
		first := &exec.Cmd{Process: &os.Process{Pid: pid}}
		second := &exec.Cmd{Process: &os.Process{Pid: pid}}
		job := &windowsProcessJob{command: first, pid: pid}
		windowsProcessJobs.Store(pid, job)
		t.Cleanup(func() { windowsProcessJobs.CompareAndDelete(pid, job) })

		if got, ok := windowsJobForCommand(first); !ok || got != job {
			t.Fatalf("windowsJobForCommand(owner) = (%p, %t), want (%p, true)", got, ok, job)
		}
		if got, ok := windowsJobForCommand(second); ok || got != nil {
			t.Fatalf("windowsJobForCommand(reused PID) = (%p, %t), want (nil, false)", got, ok)
		}
	})
}
