package procutil

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestProcessToken(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, stat string
		readErr    error
		want       string
		wantErr    string
	}{
		{name: "Should parse ticks after a command containing parentheses", stat: "42 (a ) b) S " + strings.Repeat("0 ", 18) + "98765 0", want: "boot-id:98765"},
		{name: "Should reject a truncated process stat", stat: "42 (a) S 0", wantErr: "invalid process identity"},
		{name: "Should reject malformed ticks", stat: "42 (a) S " + strings.Repeat("0 ", 18) + "oops", wantErr: "invalid process start ticks"},
		{name: "Should reject a vanished process", readErr: os.ErrNotExist, wantErr: "read process identity"},
		{name: "Should recognize a process exiting during stat read", readErr: syscall.ESRCH, wantErr: "read process identity"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := processToken(42, func(path string) ([]byte, error) {
				if path == "/proc/sys/kernel/random/boot_id" {
					return []byte("boot-id\n"), nil
				}
				if tc.readErr != nil {
					return nil, tc.readErr
				}
				return []byte(tc.stat), nil
			})
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("token = %q, %v", got, err)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want %q", err, tc.wantErr)
			}
			if tc.readErr != nil && (!errors.Is(err, tc.readErr) || !errors.Is(err, ErrProcessIdentityGone)) {
				t.Fatal(err)
			}
		})
	}
}

// Invariant: scanner and durable token reads identify the same live kernel process.
// Owner/canonical suite: procutil Linux identity.
func TestScannedKernelToken(t *testing.T) {
	t.Parallel()
	clock, err := linuxProcessClock()
	if err != nil {
		t.Fatal(err)
	}
	original, err := linuxProcess(os.Getpid(), clock)
	if err != nil {
		t.Fatal(err)
	}
	token, err := ProcessToken(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	clock.boot += 3600
	shifted, err := linuxProcess(os.Getpid(), clock)
	if err != nil {
		t.Fatal(err)
	}
	if original.KernelToken != token || shifted.KernelToken != token || original.StartedAt.Equal(shifted.StartedAt) {
		t.Fatalf("original=%+v shifted=%+v token=%q", original, shifted, token)
	}
}
