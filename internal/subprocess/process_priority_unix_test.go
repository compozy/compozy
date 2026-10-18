//go:build !windows

package subprocess

import (
	"errors"
	"io"
	"testing"

	"github.com/compozy/compozy/internal/procutil"
)

func TestLaunchAppliesNiceness(t *testing.T) {
	t.Run("Should lower the launched process group to the configured niceness", func(t *testing.T) {
		process := launchHelperProcess(t, "raw_echo", LaunchConfig{DisableTransport: true, Niceness: 6})
		defer shutdownProcess(t, process)

		want := max(6, daemonNiceness(t))
		if got := launchedGroupNiceness(t, process.PID()); got != want {
			t.Fatalf("process group niceness = %d, want %d", got, want)
		}
	})

	t.Run("Should keep the daemon priority when niceness is zero", func(t *testing.T) {
		want := daemonNiceness(t)
		process := launchHelperProcess(t, "raw_echo", LaunchConfig{DisableTransport: true})
		defer shutdownProcess(t, process)

		if got := launchedGroupNiceness(t, process.PID()); got != want {
			t.Fatalf("process group niceness = %d, want inherited %d", got, want)
		}
	})

	// Not parallel: swaps the package-level priority seam.
	t.Run("Should start the process when lowering priority fails", func(t *testing.T) {
		original := setProcessGroupNiceness
		calls := 0
		setProcessGroupNiceness = func(int, int) error {
			calls++
			return errors.New("operation not permitted")
		}
		t.Cleanup(func() { setProcessGroupNiceness = original })

		process := launchHelperProcess(t, "raw_echo", LaunchConfig{DisableTransport: true, Niceness: 10})
		defer shutdownProcess(t, process)

		if calls != 1 {
			t.Fatalf("niceness setter calls = %d, want 1", calls)
		}
		if _, err := io.WriteString(process.Stdin(), "ping\n"); err != nil {
			t.Fatalf("WriteString(stdin) error = %v, want a running process", err)
		}
	})
}

func launchedGroupNiceness(t *testing.T, pgid int) int {
	t.Helper()
	niceness, err := procutil.ProcessGroupNiceness(pgid)
	if err != nil {
		t.Fatalf("ProcessGroupNiceness(%d) error = %v", pgid, err)
	}
	return niceness
}

// daemonNiceness is the niceness the launched group inherits from this process.
func daemonNiceness(t *testing.T) int {
	t.Helper()
	niceness, err := procutil.CurrentProcessNiceness()
	if err != nil {
		t.Fatalf("CurrentProcessNiceness() error = %v", err)
	}
	return niceness
}
