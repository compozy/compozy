package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	orphanCleanupGraceWait = 2 * time.Second
	orphanCleanupPollWait  = 100 * time.Millisecond
)

type processInfo struct {
	PID  int
	PPID int
}

func (d *Daemon) cleanupOrphans(ctx context.Context, stalePID int) error {
	if stalePID <= 0 {
		return nil
	}

	processes, err := d.listProcesses(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, proc := range processes {
		if proc.PPID != stalePID || proc.PID <= 0 {
			continue
		}
		if err := d.signalProcess(proc.PID, syscall.SIGTERM); err != nil {
			errs = append(errs, fmt.Errorf("daemon: terminate orphan process %d: %w", proc.PID, err))
			continue
		}
		if d.waitForProcessExit(ctx, proc.PID) {
			continue
		}
		if d.processAlive(proc.PID) {
			if err := d.signalProcess(proc.PID, syscall.SIGKILL); err != nil {
				errs = append(errs, fmt.Errorf("daemon: kill orphan process %d: %w", proc.PID, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (d *Daemon) waitForProcessExit(ctx context.Context, pid int) bool {
	if pid <= 0 {
		return true
	}
	if !d.processAlive(pid) {
		return true
	}
	if d.orphanGraceWait <= 0 || d.orphanPollWait <= 0 {
		return !d.processAlive(pid)
	}

	timer := time.NewTimer(d.orphanGraceWait)
	ticker := time.NewTicker(d.orphanPollWait)
	defer timer.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return !d.processAlive(pid)
		case <-ticker.C:
			if !d.processAlive(pid) {
				return true
			}
		case <-timer.C:
			return !d.processAlive(pid)
		}
	}
}

func removeStaleSocket(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return nil
	}

	if err := os.Remove(cleanPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("daemon: remove stale socket %q: %w", cleanPath, err)
	}
	return nil
}

func listProcesses(ctx context.Context) ([]processInfo, error) {
	command := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("daemon: list processes: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	processes := make([]processInfo, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		processes = append(processes, processInfo{PID: pid, PPID: ppid})
	}

	return processes, nil
}
