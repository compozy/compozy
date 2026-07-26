//go:build !windows

package procutil

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/execabs"
)

const psStartedAtLayout = "Mon Jan _2 15:04:05 2006"
const processStartLookupTimeout = 5 * time.Second

// StartedAt reports the observed start time for pid using the host process table.
func StartedAt(pid int) (time.Time, error) {
	if pid <= 0 {
		return time.Time{}, fmt.Errorf("procutil: invalid process pid %d", pid)
	}

	ctx, cancel := context.WithTimeout(context.Background(), processStartLookupTimeout)
	defer cancel()

	cmd := execabs.CommandContext(
		ctx,
		"ps",
		"-o",
		"lstart=",
		"-p",
		strconv.Itoa(pid),
	)
	cmd.Env = startedAtEnv()
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, fmt.Errorf("procutil: read process %d start time: %w", pid, err)
	}

	startedAtText := strings.TrimSpace(string(output))
	if startedAtText == "" {
		return time.Time{}, fmt.Errorf("procutil: process %d start time is empty", pid)
	}

	startedAt, err := time.ParseInLocation(psStartedAtLayout, startedAtText, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf(
			"procutil: parse process %d start time %q: %w",
			pid,
			startedAtText,
			err,
		)
	}

	return startedAt.UTC(), nil
}

func startedAtEnv() []string {
	return withEnvVar(os.Environ(), "LC_ALL", "C")
}

func withEnvVar(env []string, key string, value string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered, prefix+value)
}
