//go:build !windows

package subprocess

import (
	"errors"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/procutil"
)

func TestLaunchAppliesNiceness(t *testing.T) {
	// Invariant: the managed command starts niced before Launch can adjust its group.
	// Owner/canonical suite: subprocess launch priority with real shell descendants.
	t.Run("Should apply priority before a managed command executes", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "linux" {
			t.Skip("before-exec priority is Linux-specific")
		}
		cmd, stdin, stdout, _, _, err := startManagedCommand(LaunchConfig{
			Command: "sh", Args: []string{"-c", `ps -o ni= -p $$; sh -c 'ps -o ni= -p $$'`}, Niceness: 19,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := stdin.Close(); err != nil {
				t.Errorf("close stdin: %v", err)
			}
			if err := cmd.Wait(); err != nil {
				t.Errorf("wait: %v", err)
			}
		})
		output, err := io.ReadAll(stdout)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Fields(string(output)); len(got) != 2 || got[0] != "19" || got[1] != "19" {
			t.Fatalf("startup niceness = %q, want parent and immediate child at 19", output)
		}
	})

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
