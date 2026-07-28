//go:build !windows

package procutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	processGroupPollInterval  = 10 * time.Millisecond
	processGroupDrainDeadline = 250 * time.Millisecond
)

var errProcessGroupEnumerationUnavailable = errors.New("process group enumeration unavailable")

// ConfigureCommandProcessGroup starts the command in its own process group so
// callers can signal and observe the full descendant tree.
func ConfigureCommandProcessGroup(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// RegisterCommandProcessGroup keeps Unix parity with Windows job registration.
func RegisterCommandProcessGroup(_ *exec.Cmd) error {
	return nil
}

// SignalCommandProcessGroup delivers sig to the command's process group.
func SignalCommandProcessGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}

	pgid := cmd.Process.Pid
	return SignalProcessGroupID(pgid, sig)
}

// SignalProcessGroupID delivers sig to the process group identified by pgid.
func SignalProcessGroupID(pgid int, sig syscall.Signal) error {
	if pgid <= 0 {
		return fmt.Errorf("procutil: invalid process group id %d", pgid)
	}
	if runtime.GOOS == "linux" {
		err := signalProcessGroupMembersLinux(pgid, sig)
		switch {
		case err == nil:
			return nil
		case !errors.Is(err, errProcessGroupEnumerationUnavailable):
			return fmt.Errorf("signal process group members (pid %d, sig %v): %w", pgid, sig, err)
		}
	}

	if err := syscall.Kill(-pgid, sig); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return fmt.Errorf("signal process group (pid %d, sig %v): %w", pgid, sig, err)
	}
	return nil
}

// WaitForCommandProcessGroupExit blocks until the command's process group no
// longer exists, which ensures descendants are gone before returning.
func WaitForCommandProcessGroupExit(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}

	pgid := cmd.Process.Pid
	return WaitForProcessGroupIDExit(pgid, timeout)
}

// WaitForProcessGroupIDExit blocks until the process group identified by pgid no longer exists.
func WaitForProcessGroupIDExit(pgid int, timeout time.Duration) error {
	if pgid <= 0 {
		return fmt.Errorf("procutil: invalid process group id %d", pgid)
	}
	if timeout <= 0 {
		timeout = processGroupPollInterval
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(processGroupPollInterval)
	defer ticker.Stop()
	for {
		err := syscall.Kill(-pgid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("check process group (pid %d): %w", pgid, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("wait for process group exit (pid %d): deadline exceeded after %s", pgid, timeout)
		}
		<-ticker.C
	}
}

// KillCommandProcessGroupAndWait forcefully terminates any remaining members of
// the command's process group, then waits for the group to disappear.
func KillCommandProcessGroupAndWait(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		return nil
	}
	return KillProcessGroupIDAndWait(cmd.Process.Pid, timeout)
}

// KillProcessGroupIDAndWait forcefully terminates any remaining members of a process group.
func KillProcessGroupIDAndWait(pgid int, timeout time.Duration) error {
	signalErr := SignalProcessGroupID(pgid, syscall.SIGKILL)
	waitErr := WaitForProcessGroupIDExit(pgid, timeout)
	return joinProcessGroupKillResult(signalErr, waitErr)
}

func joinProcessGroupKillResult(signalErr error, waitErr error) error {
	// A best-effort SIGKILL can race with the process group exiting on its own.
	// If the follow-up wait proves the group is gone, an EPERM from the signal
	// attempt is stale noise rather than a real shutdown failure.
	if waitErr == nil && errors.Is(signalErr, syscall.EPERM) {
		return nil
	}
	return errors.Join(signalErr, waitErr)
}

func signalProcessGroupMembersLinux(pgid int, sig syscall.Signal) error {
	members, err := linuxProcessGroupMembers(pgid)
	if err != nil {
		return err
	}
	if len(members) == 0 {
		return errProcessGroupEnumerationUnavailable
	}

	for _, pid := range members {
		if pid == pgid {
			continue
		}
		if err := signalPID(pid, sig); err != nil {
			return err
		}
	}

	if len(members) > 1 {
		waitForLinuxDescendantsToExit(pgid, processGroupDrainDeadline)
	}

	return signalPID(pgid, sig)
}

func waitForLinuxDescendantsToExit(pgid int, timeout time.Duration) {
	if timeout <= 0 {
		return
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(processGroupPollInterval)
	defer ticker.Stop()

	for {
		members, err := linuxProcessGroupMembers(pgid)
		if err != nil {
			return
		}

		descendantsRemain := false
		for _, pid := range members {
			if pid != pgid {
				descendantsRemain = true
				break
			}
		}
		if !descendantsRemain || time.Now().After(deadline) {
			return
		}

		<-ticker.C
	}
}

func linuxProcessGroupMembers(pgid int) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, errProcessGroupEnumerationUnavailable
	}

	members := make([]int, 0, 4)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}

		candidatePGID, err := linuxProcessGroupID(pid)
		if err != nil {
			continue
		}
		if candidatePGID == pgid {
			members = append(members, pid)
		}
	}

	sort.Ints(members)
	return members, nil
}

func linuxProcessGroupID(pid int) (int, error) {
	statPath := fmt.Sprintf("/proc/%d/stat", pid)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, err
	}

	line := string(data)
	closing := strings.LastIndex(line, ")")
	if closing < 0 || closing+2 >= len(line) {
		return 0, fmt.Errorf("parse %s: malformed stat payload", statPath)
	}

	fields := strings.Fields(line[closing+2:])
	if len(fields) < 3 {
		return 0, fmt.Errorf("parse %s: missing pgid field", statPath)
	}

	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, fmt.Errorf("parse %s pgid %q: %w", statPath, fields[2], err)
	}
	return pgid, nil
}

func signalPID(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}

	if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return err
	}
	return nil
}
