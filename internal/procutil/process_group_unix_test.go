//go:build !windows

package procutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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

// Invariant: Linux launches inherit priority before exec and immediate forks inherit it.
// Owner/canonical suite: procutil real-process group niceness.
func TestSetProcessGroupNiceness(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit launch priority before exec and through immediate forks", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS != "linux" {
			t.Skip("before-exec thread priority is Linux-specific")
		}
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		inherited, err := CurrentProcessNiceness()
		if err != nil {
			t.Fatal(err)
		}
		if inherited >= maxTestNiceness {
			t.Skip("inherited priority leaves no room to lower priority")
		}
		want := min(maxTestNiceness, max(7, inherited+3))
		const child = `sh -c 'echo $$; read hold <&3'`
		group := startNicenessProcessGroup(t, child+" & "+child+" & wait", want)
		first, second := group.readPID(t), group.readPID(t)
		for _, pid := range []int{group.pgid, first, second} {
			if got := testMemberNiceness(t, group.pgid, pid); got != want {
				t.Fatalf("pid %d niceness = %d, want %d", pid, got, want)
			}
		}
		if got, err := CurrentProcessNiceness(); err != nil || got != inherited {
			t.Fatalf("caller niceness = %d, error = %v, want unchanged %d", got, err, inherited)
		}
	})

	t.Run("Should lower every current and later member of the process group", func(t *testing.T) {
		t.Parallel()

		// Each child prints its own pid once it runs, so it has joined the group, and then
		// blocks reading fd 3, which only the test holds open, so it stays alive through
		// the assertions. The later fork waits for stdin, so no fork is in flight during
		// the call: setpriority(PRIO_PGRP) cannot reach a child that has not joined yet.
		const child = `sh -c 'echo $$; read hold <&3'`
		group := startNicenessProcessGroup(t, child+" & read go; "+child+" & wait")
		earlier := group.readPID(t)
		// Target above the group's own niceness so the call must change every member.
		// Linux niceness is per thread, so the group need not match the test goroutine.
		want := testProcessGroupNiceness(t, group.pgid) + 3
		if want > maxTestNiceness {
			t.Skipf("group niceness %d leaves no room to lower priority", want-3)
		}
		if err := SetProcessGroupNiceness(group.pgid, want); err != nil {
			t.Fatalf("SetProcessGroupNiceness() error = %v", err)
		}
		group.release(t)
		later := group.readPID(t)
		// The group's reported priority is its least-niced member. Each member is also
		// checked by pid, and the children cannot exit while fd 3 stays open.
		if got := testProcessGroupNiceness(t, group.pgid); got != want {
			t.Fatalf("process group niceness = %d, want %d", got, want)
		}
		for _, member := range []struct {
			name string
			pid  int
		}{
			{name: "shell", pid: group.pgid},
			{name: "earlier child", pid: earlier},
			{name: "later child", pid: later},
		} {
			if got := testMemberNiceness(t, group.pgid, member.pid); got != want {
				t.Fatalf("%s (pid %d) niceness = %d, want %d", member.name, member.pid, got, want)
			}
		}
	})

	t.Run("Should leave a group already niced above the target unchanged", func(t *testing.T) {
		t.Parallel()

		// The shell blocks on a builtin read of fd 3 until cleanup, so the group stays a
		// single live process: a fork in flight during the first call would keep the
		// old niceness and make the second call try to lower the group.
		pgid := startNicenessProcessGroup(t, "read hold <&3").pgid
		higher := max(9, testProcessGroupNiceness(t, pgid)+2)
		if higher > maxTestNiceness {
			t.Skipf("group niceness leaves no room above target %d", higher-2)
		}
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

// maxTestNiceness is the highest niceness every supported Unix accepts.
const maxTestNiceness = 19

// nicenessProcessGroup is a shell process group whose script reports progress on
// stdout and waits on stdin, so tests can order forks around a niceness change.
type nicenessProcessGroup struct {
	pgid   int
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

// startNicenessProcessGroup starts script as the leader of a new process group and
// kills the whole group on cleanup.
func startNicenessProcessGroup(t *testing.T, script string, niceness ...int) nicenessProcessGroup {
	t.Helper()
	// This helper owns child cleanup. t.Context is canceled before Cleanup runs;
	// racing that implicit kill with the group signal can return EPERM during exec on Darwin.
	command := exec.CommandContext(context.WithoutCancel(t.Context()), "sh", "-c", script)
	ConfigureCommandProcessGroup(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe() error = %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	// fd 3 lets script children block until cleanup: the test never writes to it.
	holdRead, holdWrite, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	command.ExtraFiles = []*os.File{holdRead}
	target := 0
	if len(niceness) > 0 {
		target = niceness[0]
	}
	priorityErr, startErr := StartWithNiceness(target, func() error {
		// This observation precedes Start, so a post-start adjustment cannot pass the test.
		if runtime.GOOS == "linux" && target > 0 {
			current, err := CurrentProcessNiceness()
			if err != nil {
				return err
			}
			if current < target {
				return fmt.Errorf("pre-exec thread niceness = %d, want at least %d", current, target)
			}
		}
		return command.Start()
	})
	if err := errors.Join(priorityErr, startErr); err != nil {
		closeHoldPipe(t, holdRead, holdWrite)
		t.Fatalf("Start() error = %v", err)
	}
	if err := holdRead.Close(); err != nil {
		t.Errorf("close hold pipe read end: %v", err)
	}
	pgid := command.Process.Pid
	t.Cleanup(func() {
		closeHoldPipe(t, holdWrite)
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
	return nicenessProcessGroup{pgid: pgid, stdin: stdin, stdout: bufio.NewReader(stdout)}
}

// readPID returns the next pid the script reports. It fails the subtest when the
// line does not arrive in time, so cleanup kills the group instead of the read
// blocking until the binary times out. Cleanup closes stdout through Wait, which
// ends the abandoned read.
func (g nicenessProcessGroup) readPID(t *testing.T) int {
	t.Helper()
	type readResult struct {
		line string
		err  error
	}
	result := make(chan readResult, 1)
	go func() {
		line, err := g.stdout.ReadString('\n')
		result <- readResult{line: line, err: err}
	}()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case read := <-result:
		if read.err != nil {
			t.Fatalf("read process group output: %v", read.err)
		}
		pid, err := strconv.Atoi(strings.TrimSpace(read.line))
		if err != nil || pid <= 0 {
			t.Fatalf("process group output = %q, want a child pid", read.line)
		}
		return pid
	case <-timer.C:
		t.Fatal("process group did not report a child pid within 10s")
		return 0
	}
}

func closeHoldPipe(t *testing.T, files ...*os.File) {
	t.Helper()
	for _, file := range files {
		if err := file.Close(); err != nil {
			t.Errorf("close hold pipe: %v", err)
		}
	}
}

// release lets the script continue past its stdin gate and fork the later child.
func (g nicenessProcessGroup) release(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(g.stdin, "go\n"); err != nil {
		t.Fatalf("release process group: %v", err)
	}
}

func testProcessGroupNiceness(t *testing.T, pgid int) int {
	t.Helper()
	niceness, err := ProcessGroupNiceness(pgid)
	if err != nil {
		t.Fatalf("ProcessGroupNiceness(%d) error = %v", pgid, err)
	}
	return niceness
}

// testMemberNiceness reports the niceness of a process after confirming it belongs
// to the process group.
func testMemberNiceness(t *testing.T, pgid int, pid int) int {
	t.Helper()
	memberGroup, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatalf("Getpgid(%d) error = %v, want live process group member", pid, err)
	}
	if memberGroup != pgid {
		t.Fatalf("Getpgid(%d) = %d, want %d", pid, memberGroup, pgid)
	}
	niceness, err := getNiceness(syscall.PRIO_PROCESS, pid)
	if err != nil {
		t.Fatalf("niceness of pid %d error = %v", pid, err)
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

// Invariant: joining an inspected process group never grants termination authority to the new member.
// Owning layer/canonical suite: procutil real-process signaling safety.
func TestScannedGroupMembershipFence(t *testing.T) {
	t.Parallel()
	t.Run("Should signal selected group identities without signaling a late joiner", func(t *testing.T) {
		t.Parallel()
		leader := startScannedGroupHelper(t, 0)
		snapshot, err := ScanProcesses(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		var target ScannedProcess
		for _, p := range snapshot.Processes {
			if p.PID == leader {
				target = p
				break
			}
		}
		members, valid := scannedSignalMembers(snapshot, target, false)
		if !valid || len(members) != 1 {
			t.Fatalf("members=%v valid=%t", members, valid)
		}
		late := startScannedGroupHelper(t, leader)
		if err := signalScannedMembers(members, syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		result := awaitGracefulScannedExit(t.Context(), members, time.Second)
		if result.Outcome != "terminated" {
			t.Fatalf("termination=%+v", result)
		}
		if !Alive(late) {
			t.Fatal("late group member received a signal without being selected")
		}
	})
}

func startScannedGroupHelper(t *testing.T, pgid int) int {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), executable, "-test.run=^TestScannedProcessTermination$")
	command.Env = append(
		os.Environ(),
		"COMPOZY_PROCSCAN_HELPER=1",
		"COMPOZY_SESSION_ID=sess-group-fence",
		"COMPOZY_HOME="+t.TempDir(),
	)
	ConfigureCommandProcessGroup(command)
	command.SysProcAttr.Pgid = pgid
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
	return command.Process.Pid
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
