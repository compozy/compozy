//go:build !windows

package procutil

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestConfigureCommandSession(t *testing.T) {
	t.Parallel()

	t.Run("Should create a new session without claiming a controlling terminal", func(t *testing.T) {
		t.Parallel()
		command := exec.CommandContext(t.Context(), "true")

		ConfigureCommandSession(command)

		if command.SysProcAttr == nil || !command.SysProcAttr.Setsid || command.SysProcAttr.Setctty {
			t.Fatalf("session SysProcAttr = %#v, want Setsid without Setctty", command.SysProcAttr)
		}
	})

	t.Run("Should create a new session that claims its controlling terminal", func(t *testing.T) {
		t.Parallel()
		command := exec.CommandContext(t.Context(), "true")

		ConfigureCommandTerminalSession(command)

		if command.SysProcAttr == nil || !command.SysProcAttr.Setsid || !command.SysProcAttr.Setctty {
			t.Fatalf("terminal session SysProcAttr = %#v, want Setsid and Setctty", command.SysProcAttr)
		}
	})
}

func TestJoinProcessGroupKillResult(t *testing.T) {
	t.Parallel()

	waitErr := errors.New("wait for process group exit: deadline exceeded")
	testCases := []struct {
		name      string
		signalErr error
		waitErr   error
		wantNil   bool
		wantIs    []error
	}{
		{
			name:      "Should suppress EPERM when wait succeeds",
			signalErr: fmt.Errorf("signal process group (pid 123, sig killed): %w", syscall.EPERM),
			wantNil:   true,
		},
		{
			name:      "Should preserve wait failure when signal returns EPERM",
			signalErr: fmt.Errorf("signal process group (pid 123, sig killed): %w", syscall.EPERM),
			waitErr:   waitErr,
			wantIs:    []error{waitErr},
		},
		{
			name:      "Should preserve non-EPERM signal failure",
			signalErr: fmt.Errorf("signal process group members: %w", syscall.ESRCH),
			wantIs:    []error{syscall.ESRCH},
		},
		{
			name:      "Should join non-EPERM signal failure with wait failure",
			signalErr: fmt.Errorf("signal process group members: %w", syscall.ESRCH),
			waitErr:   waitErr,
			wantIs:    []error{syscall.ESRCH, waitErr},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := joinProcessGroupKillResult(tc.signalErr, tc.waitErr)
			if tc.wantNil {
				if err != nil {
					t.Fatalf("joinProcessGroupKillResult() error = %v, want nil", err)
				}
				return
			}
			for _, wantErr := range tc.wantIs {
				if !errors.Is(err, wantErr) {
					t.Fatalf("joinProcessGroupKillResult() error = %v, want wrapped %v", err, wantErr)
				}
			}
		})
	}
}

func TestSignalProcessGroupIDStrict(t *testing.T) {
	t.Parallel()

	t.Run("Should report a process group that disappeared before signal delivery", func(t *testing.T) {
		t.Parallel()

		const missingProcessGroupID = 1 << 30
		err := SignalProcessGroupIDStrict(missingProcessGroupID, syscall.SIGTERM)
		if !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("SignalProcessGroupIDStrict() error = %v, want ESRCH", err)
		}
	})

	t.Run("Should retain best-effort behavior for the general process-group helper", func(t *testing.T) {
		t.Parallel()

		const missingProcessGroupID = 1 << 30
		if err := SignalProcessGroupID(missingProcessGroupID, syscall.SIGTERM); err != nil {
			t.Fatalf("SignalProcessGroupID() error = %v, want nil", err)
		}
	})
}

func TestSetProcessGroupNiceness(t *testing.T) {
	t.Parallel()

	t.Run("Should lower every current and later member of the process group", func(t *testing.T) {
		t.Parallel()

		pgid := startNicenessProcessGroup(t, "sleep 30 & sleep 0.2; sleep 30")
		want := max(7, testCurrentNiceness(t))
		if err := SetProcessGroupNiceness(pgid, 7); err != nil {
			t.Fatalf("SetProcessGroupNiceness() error = %v", err)
		}
		// The group's reported priority is its least-niced member, so the target means
		// the shell, its background child, and the later-forked sleep all inherited it.
		time.Sleep(400 * time.Millisecond)
		if got := testProcessGroupNiceness(t, pgid); got != want {
			t.Fatalf("process group niceness = %d, want %d", got, want)
		}
	})

	t.Run("Should leave a group already niced above the target unchanged", func(t *testing.T) {
		t.Parallel()

		pgid := startNicenessProcessGroup(t, "sleep 30")
		higher := max(9, testCurrentNiceness(t)+2)
		if err := SetProcessGroupNiceness(pgid, higher); err != nil {
			t.Fatalf("SetProcessGroupNiceness(%d) error = %v", higher, err)
		}
		if err := SetProcessGroupNiceness(pgid, higher-2); err != nil {
			t.Fatalf("SetProcessGroupNiceness(%d) error = %v, want no-op without privileges", higher-2, err)
		}
		if got := testProcessGroupNiceness(t, pgid); got != higher {
			t.Fatalf("process group niceness = %d, want unchanged %d", got, higher)
		}
	})

	t.Run("Should reject a non-positive process group id", func(t *testing.T) {
		t.Parallel()

		if err := SetProcessGroupNiceness(0, 5); err == nil {
			t.Fatal("SetProcessGroupNiceness(0) error = nil, want validation error")
		}
	})

	t.Run("Should report a process group that does not exist", func(t *testing.T) {
		t.Parallel()

		const missingProcessGroupID = 1 << 30
		if err := SetProcessGroupNiceness(missingProcessGroupID, 5); !errors.Is(err, syscall.ESRCH) {
			t.Fatalf("SetProcessGroupNiceness() error = %v, want ESRCH", err)
		}
	})
}

func startNicenessProcessGroup(t *testing.T, script string) int {
	t.Helper()
	command := exec.CommandContext(t.Context(), "sh", "-c", script)
	ConfigureCommandProcessGroup(command)
	if err := command.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	pgid := command.Process.Pid
	t.Cleanup(func() {
		if err := SignalProcessGroupID(pgid, syscall.SIGKILL); err != nil {
			t.Errorf("cleanup SignalProcessGroupID() error = %v", err)
		}
		if err := command.Wait(); err != nil {
			if _, ok := errors.AsType[*exec.ExitError](err); ok {
				return
			}
			t.Errorf("cleanup Wait() error = %v", err)
		}
	})
	return pgid
}

func testProcessGroupNiceness(t *testing.T, pgid int) int {
	t.Helper()
	niceness, err := ProcessGroupNiceness(pgid)
	if err != nil {
		t.Fatalf("ProcessGroupNiceness(%d) error = %v", pgid, err)
	}
	return niceness
}

func testCurrentNiceness(t *testing.T) int {
	t.Helper()
	niceness, err := CurrentProcessNiceness()
	if err != nil {
		t.Fatalf("CurrentProcessNiceness() error = %v", err)
	}
	return niceness
}
