//go:build !windows

package procutil

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

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
