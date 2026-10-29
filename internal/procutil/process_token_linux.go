package procutil

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// ProcessToken identifies a process without converting kernel ticks to wall time.
func ProcessToken(pid int) (string, error) {
	return processToken(pid, os.ReadFile)
}

func processToken(pid int, readFile func(string) ([]byte, error)) (string, error) {
	if pid <= 0 {
		return "", fmt.Errorf("procutil: invalid process pid %d", pid)
	}
	boot, err := readFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", fmt.Errorf("procutil: read boot identity: %w", err)
	}
	stat, err := readFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ESRCH) {
			err = errors.Join(ErrProcessIdentityGone, err)
		}
		return "", fmt.Errorf("procutil: read process identity: %w", err)
	}
	_, tail, ok := strings.CutLast(string(stat), ")")
	fields := strings.Fields(tail)
	if !ok || len(fields) < 20 || strings.TrimSpace(string(boot)) == "" {
		return "", fmt.Errorf("procutil: invalid process identity for pid %d", pid)
	}
	if _, err := strconv.ParseUint(fields[19], 10, 64); err != nil {
		return "", fmt.Errorf("procutil: invalid process start ticks: %w", err)
	}
	return strings.TrimSpace(string(boot)) + ":" + fields[19], nil
}
