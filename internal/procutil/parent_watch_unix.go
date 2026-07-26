//go:build !windows

package procutil

import (
	"context"
	"os"
	"time"
)

const defaultParentWatchInterval = 500 * time.Millisecond

// WatchParentExit calls onExit once if this process is reparented because its
// original parent exited, then returns; it stops early when ctx is done. It lets
// a child self-terminate when its launcher dies abruptly (SIGKILL, test timeout,
// Ctrl-C) — Darwin has no Pdeathsig. On Windows the kill-on-close job object
// (RegisterCommandProcessGroup) handles this, so that build is a no-op.
func WatchParentExit(ctx context.Context, poll time.Duration, onExit func()) {
	watchParentExit(ctx, poll, os.Getppid, onExit)
}

// watchParentExit polls getppid and calls onExit once the parent pid differs
// from the value observed at call time. It returns without calling onExit when
// ctx is done.
func watchParentExit(ctx context.Context, poll time.Duration, getppid func() int, onExit func()) {
	if getppid == nil || onExit == nil {
		return
	}
	if poll <= 0 {
		poll = defaultParentWatchInterval
	}
	original := getppid()
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if getppid() != original {
				onExit()
				return
			}
		}
	}
}
