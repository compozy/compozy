//go:build !windows

package procutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
	// This helper owns child cleanup. t.Context is canceled before Cleanup runs;
	// racing that implicit kill with the group signal can return EPERM during exec on Darwin.
	command := exec.CommandContext(context.WithoutCancel(t.Context()), "sh", "-c", script)
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

func TestScannedProcessTermination(t *testing.T) {
	if os.Getenv("COMPOZY_PROCSCAN_HELPER") == "1" {
		timer := time.NewTimer(9999 * time.Second)
		defer timer.Stop()
		<-timer.C
		return
	}
	t.Parallel()
	t.Run("Should attribute a detached process and reject a stale identity before termination", func(t *testing.T) {
		t.Parallel()
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(t.Context(), executable, "-test.run=^TestScannedProcessTermination$")
		command.Env = append(
			os.Environ(),
			"COMPOZY_SESSION_ID=sess-procscan-test",
			"COMPOZY_PROCSCAN_HELPER=1",
			"COMPOZY_PROFILE=engineering",
			"COMPOZY_PROFILE_ID=profile-engineering",
			"COMPOZY_HOME="+t.TempDir(),
		)
		ConfigureCommandSession(command)
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			if err := command.Wait(); err != nil {
				if _, ok := errors.AsType[*exec.ExitError](err); !ok {
					t.Errorf("wait: %v", err)
				}
			}
		}()
		t.Cleanup(func() {
			if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				t.Errorf("cleanup: %v", err)
			}
			<-done
		})
		snapshot, err := ScanProcesses(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if !snapshot.Supported {
			t.Skip("process scanning unsupported")
		}
		var target ScannedProcess
		for _, p := range snapshot.Processes {
			if p.PID == command.Process.Pid {
				target = p
				break
			}
		}
		if target.SessionID != "sess-procscan-test" || target.PGID != target.PID || target.StartedAt.IsZero() {
			t.Fatalf("process not attributed: %#v", target)
		}
		if target.Profile != "profile-engineering" {
			t.Fatalf("process profile identity = %q, want profile-engineering", target.Profile)
		}
		// The owning real-process suite also verifies readable command arguments survive discovery.
		enriched := []ScannedProcess{target}
		EnrichProcesses(t.Context(), enriched)
		if !strings.Contains(enriched[0].Command, "-test.run=^TestScannedProcessTermination$") {
			t.Fatalf("command arguments missing: %q", enriched[0].Command)
		}
		stale := target
		stale.StartedAt = stale.StartedAt.Add(-time.Hour)
		if result := TerminateScannedProcess(t.Context(), stale, time.Second); result.Outcome != "identity_mismatch" {
			t.Fatalf("stale identity result: %#v", result)
		}
		if !Alive(target.PID) {
			t.Fatal("stale identity signaled the process")
		}
		if result := TerminateScannedProcess(t.Context(), target, 3*time.Second); result.Outcome != "terminated" {
			t.Fatalf("termination: %#v", result)
		}
		<-done
		if result := TerminateScannedProcess(t.Context(), target, time.Second); result.Outcome != "already_exited" {
			t.Fatalf("repeat termination: %#v", result)
		}
		t.Logf("scan duration=%s total processes=%d", snapshot.Duration, len(snapshot.Processes))
	})
}

// Measures both cadences on the actual host table; no fake process enumeration.
func BenchmarkProcessScan(b *testing.B) {
	for _, full := range []bool{false, true} {
		name := "tree"
		if full {
			name = "environment"
		}
		b.Run(name, func(b *testing.B) {
			var before, after syscall.Rusage
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &before); err != nil {
				b.Fatal(err)
			}
			count := 0
			for b.Loop() {
				var snap ProcessSnapshot
				var err error
				if full {
					snap, err = ScanProcesses(b.Context())
				} else {
					snap, err = ScanProcessTree(b.Context())
				}
				if err != nil {
					b.Fatal(err)
				}
				count = len(snap.Processes)
			}
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &after); err != nil {
				b.Fatal(err)
			}
			cpu := after.Utime.Nano() + after.Stime.Nano() - before.Utime.Nano() - before.Stime.Nano()
			b.ReportMetric(float64(cpu)/float64(b.N), "cpu-ns/op")
			b.ReportMetric(float64(count), "processes")
		})
	}
}
