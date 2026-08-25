//go:build !windows

package procutil

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
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
