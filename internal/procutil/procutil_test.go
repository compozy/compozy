package procutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"syscall"
	"testing"
	"time"

	"github.com/gofrs/flock"
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

func TestVerifyProcessExit(t *testing.T) {
	t.Parallel()
	t.Run("Should distinguish the live process from a reused PID", func(t *testing.T) {
		t.Parallel()
		started, err := StartedAt(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		verified, err := VerifyProcessExit(os.Getpid(), started)
		if err != nil || verified {
			t.Fatalf("VerifyProcessExit(live) = %v, %v", verified, err)
		}
		verified, err = VerifyProcessExit(os.Getpid(), started.Add(-time.Hour))
		if err != nil || !verified {
			t.Fatalf("VerifyProcessExit(reused) = %v, %v", verified, err)
		}
	})
	t.Run("Should retain uncertainty when the process table lookup fails", func(t *testing.T) {
		t.Parallel()
		lookupErr := errors.New("process table unavailable")
		verified, err := verifyProcessExit(42, time.Unix(100, 0),
			func(int, syscall.Signal) error { return nil },
			func(int) (time.Time, error) { return time.Time{}, lookupErr },
		)
		if verified || !errors.Is(err, lookupErr) {
			t.Fatalf("verifyProcessExit() = %v, %v", verified, err)
		}
	})
	t.Run("Should retain uncertainty when the initial probe fails", func(t *testing.T) {
		t.Parallel()
		probeErr := errors.New("process probe unavailable")
		verified, err := verifyProcessExit(42, time.Unix(100, 0),
			func(int, syscall.Signal) error { return probeErr },
			func(int) (time.Time, error) { t.Fatal("unexpected lookup after failed probe"); return time.Time{}, nil },
		)
		if verified || !errors.Is(err, probeErr) {
			t.Fatalf("verifyProcessExit() = %v, %v", verified, err)
		}
	})
	t.Run("Should reject missing process identity", func(t *testing.T) {
		t.Parallel()
		for _, pid := range []int{0, -1, os.Getpid()} {
			if verified, err := VerifyProcessExit(pid, time.Time{}); verified || err == nil {
				t.Fatalf("VerifyProcessExit(%d, zero) = %v, %v", pid, verified, err)
			}
		}
	})
}

func TestFileLockIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, row string
		want      bool
	}{
		{"Should match the exclusive kernel lock owner", "1: FLOCK ADVISORY WRITE 42 00:1a:123 0 EOF", true},
		{"Should reject a different PID", "1: FLOCK ADVISORY WRITE 43 00:1a:123 0 EOF", false},
		{"Should reject another home inode", "1: FLOCK ADVISORY WRITE 42 00:1a:124 0 EOF", false},
		{"Should reject another device", "1: FLOCK ADVISORY WRITE 42 00:1b:123 0 EOF", false},
		{"Should reject shared locks", "1: FLOCK ADVISORY READ 42 00:1a:123 0 EOF", false},
		{"Should reject blocked waiters", "1: -> FLOCK ADVISORY WRITE 42 00:1a:123 0 EOF", false},
		{"Should reject POSIX locks", "1: POSIX ADVISORY WRITE 42 00:1a:123 0 EOF", false},
		{"Should reject malformed identities", "1: FLOCK ADVISORY WRITE 42 00:1a:bad 0 EOF", false},
		{"Should reject incomplete records", "1: FLOCK ADVISORY WRITE 42", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ownsFileLock(strings.NewReader(tc.row), 42, 0, 26, 123)
			if err != nil || got != tc.want {
				t.Fatalf("ownsFileLock() = %v, %v; want %v", got, err, tc.want)
			}
		})
	}
	t.Run("Should verify a real Linux flock and reject its released owner", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "linux" {
			t.Skip("kernel lock ownership is Linux-specific")
		}
		path := filepath.Join(t.TempDir(), "daemon.lock")
		lock := flock.New(path)
		t.Cleanup(func() {
			if err := lock.Close(); err != nil {
				t.Error(err)
			}
		})
		locked, err := lock.TryLock()
		if err != nil || !locked {
			t.Fatalf("TryLock() = %v, %v", locked, err)
		}
		owned, err := OwnsFileLock(os.Getpid(), path)
		if err != nil || !owned {
			t.Fatalf("OwnsFileLock() = %v, %v", owned, err)
		}
		if err := lock.Unlock(); err != nil {
			t.Fatal(err)
		}
		owned, err = OwnsFileLock(os.Getpid(), path)
		if err != nil || owned {
			t.Fatalf("released OwnsFileLock() = %v, %v", owned, err)
		}
	})
}

func TestMatchesProcessIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		recorded  string
		observed  string
		lookupErr error
		want      bool
	}{
		{name: "Should accept the same kernel identity despite wall clock drift", recorded: "boot:123", observed: "boot:123", want: true},
		{name: "Should reject a reused PID", recorded: "boot:123", observed: "boot:456"},
		{name: "Should reject an identity from a previous boot", recorded: "old:123", observed: "new:123"},
		{name: "Should reject a vanished process", recorded: "boot:123", lookupErr: os.ErrNotExist},
		{name: "Should retain timestamp verification for old records", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := matchesProcessIdentity(42, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), tc.recorded,
				func(int) (string, error) { return tc.observed, tc.lookupErr },
				func(int, time.Time) bool { return false },
			)
			if got != tc.want {
				t.Fatalf("identity match = %v, want %v", got, tc.want)
			}
		})
	}
}
