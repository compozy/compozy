//go:build mage

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

const (
	verifyLockEnvVar       = "AGH_VERIFY_LOCK"
	verifyLockDirName      = "agh-dev"
	verifyLockFileName     = "verify.lock"
	verifyLockPollInterval = 500 * time.Millisecond
	verifyLockWaitLogEvery = 15 * time.Second
)

type verifyLockHolder struct {
	PID        int    `json:"pid"`
	WorkingDir string `json:"working_dir"`
	StartedAt  string `json:"started_at"`
}

// acquireVerifyLock serializes full verify runs machine-wide: a second worktree
// queues (with progress messages) instead of oversubscribing the machine until
// both runs stall (L-030). The lock is capacity management, not correctness —
// every failure path warns and proceeds without the lock.
func acquireVerifyLock() func() {
	if verifyLockDisabled() {
		return func() {}
	}
	lockPath, err := verifyLockPath()
	if err != nil {
		fmt.Printf("Warning: verify lock unavailable (%v); continuing without machine-wide serialization\n", err)
		return func() {}
	}
	return acquireVerifyLockAt(lockPath)
}

func verifyLockDisabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(verifyLockEnvVar))) {
	case "0", "off", "false", "disable", "disabled":
		return true
	}
	return false
}

func verifyLockPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolve user cache dir: %w", err)
	}
	lockDir := filepath.Join(cacheDir, verifyLockDirName)
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return "", fmt.Errorf("create verify lock dir %q: %w", lockDir, err)
	}
	return filepath.Join(lockDir, verifyLockFileName), nil
}

func acquireVerifyLockAt(lockPath string) func() {
	fileLock := flock.New(lockPath)
	waiting := false
	lastLog := time.Time{}
	ticker := time.NewTicker(verifyLockPollInterval)
	defer ticker.Stop()
	for {
		locked, err := fileLock.TryLock()
		if err != nil {
			fmt.Printf(
				"Warning: verify lock %q failed (%v); continuing without machine-wide serialization\n",
				lockPath,
				err,
			)
			return func() {}
		}
		if locked {
			if waiting {
				fmt.Println("Verify lock acquired — resuming verification pipeline")
			}
			writeVerifyLockHolder(lockPath)
			return func() {
				if err := os.WriteFile(lockPath, nil, 0o644); err != nil {
					fmt.Printf("Warning: clear verify lock holder %q: %v\n", lockPath, err)
				}
				if err := fileLock.Unlock(); err != nil {
					fmt.Printf("Warning: release verify lock %q: %v\n", lockPath, err)
				}
				if err := fileLock.Close(); err != nil {
					fmt.Printf("Warning: close verify lock %q: %v\n", lockPath, err)
				}
			}
		}
		if !waiting || time.Since(lastLog) >= verifyLockWaitLogEvery {
			fmt.Printf(
				"make verify is queued: %s holds the machine-wide verify lock (%s). Waiting... (set %s=off to bypass)\n",
				describeVerifyLockHolder(lockPath),
				lockPath,
				verifyLockEnvVar,
			)
			waiting = true
			lastLog = time.Now()
		}
		<-ticker.C
	}
}

func writeVerifyLockHolder(lockPath string) {
	workingDir, err := os.Getwd()
	if err != nil {
		workingDir = ""
	}
	holder := verifyLockHolder{
		PID:        os.Getpid(),
		WorkingDir: workingDir,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	payload, err := json.Marshal(holder)
	if err != nil {
		fmt.Printf("Warning: encode verify lock holder: %v\n", err)
		return
	}
	if err := os.WriteFile(lockPath, append(payload, '\n'), 0o644); err != nil {
		fmt.Printf("Warning: record verify lock holder %q: %v\n", lockPath, err)
	}
}

func describeVerifyLockHolder(lockPath string) string {
	data, err := os.ReadFile(lockPath)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return "another make verify run"
	}
	var holder verifyLockHolder
	if err := json.Unmarshal(data, &holder); err != nil || holder.PID <= 0 {
		return "another make verify run"
	}
	if holder.WorkingDir == "" {
		return fmt.Sprintf("pid %d", holder.PID)
	}
	return fmt.Sprintf("pid %d (%s)", holder.PID, holder.WorkingDir)
}
