package procutil

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestAliveCurrentProcess(t *testing.T) {
	t.Parallel()

	if !Alive(os.Getpid()) {
		t.Fatal("Alive(current pid) = false, want true")
	}
}

func TestAliveRejectsNonPositivePIDs(t *testing.T) {
	t.Parallel()

	testCases := []int{0, -1}
	for _, pid := range testCases {
		t.Run(fmt.Sprintf("ShouldReturnFalseForPID_%d", pid), func(t *testing.T) {
			t.Parallel()
			if Alive(pid) {
				t.Fatalf("Alive(%d) = true, want false", pid)
			}
		})
	}
}

func TestSignalCurrentProcessWithSignalZero(t *testing.T) {
	t.Parallel()

	if err := Signal(os.Getpid(), syscall.Signal(0)); err != nil {
		t.Fatalf("Signal(current pid, 0) error = %v, want nil", err)
	}
}

func TestSignalRejectsNonPositivePID(t *testing.T) {
	t.Parallel()

	if err := Signal(0, syscall.SIGTERM); err == nil {
		t.Fatal("Signal(0, SIGTERM) error = nil, want non-nil")
	}
}

func TestSignalReturnsErrorForMissingProcess(t *testing.T) {
	t.Parallel()

	if err := Signal(999999, syscall.Signal(0)); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("Signal(missing pid, 0) error = %v, want ESRCH", err)
	}
}

func TestStartedAtCurrentProcess(t *testing.T) {
	t.Parallel()

	t.Run("ShouldReturnANonZeroPastTimestampForTheCurrentProcess", func(t *testing.T) {
		t.Parallel()

		startedAt, err := StartedAt(os.Getpid())
		if err != nil {
			t.Fatalf("StartedAt(current pid) error = %v", err)
		}
		if startedAt.IsZero() {
			t.Fatal("StartedAt(current pid) = zero, want non-zero start time")
		}
		if startedAt.After(time.Now().UTC().Add(time.Second)) {
			t.Fatalf("StartedAt(current pid) = %v, want a past timestamp", startedAt)
		}
	})
}

func TestMatchesStartTimeCurrentProcess(t *testing.T) {
	t.Parallel()

	startedAt, err := StartedAt(os.Getpid())
	if err != nil {
		t.Fatalf("StartedAt(current pid) error = %v", err)
	}

	testCases := []struct {
		name      string
		input     time.Time
		wantMatch bool
	}{
		{
			name:      "ShouldMatchTheCurrentProcessStartTime",
			input:     startedAt,
			wantMatch: true,
		},
		{
			name:      "ShouldRejectAMismatchedStartTime",
			input:     startedAt.Add(-time.Hour),
			wantMatch: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := MatchesStartTime(os.Getpid(), tc.input)
			if got != tc.wantMatch {
				t.Fatalf("MatchesStartTime(current pid, %v) = %v, want %v", tc.input, got, tc.wantMatch)
			}
		})
	}
}
