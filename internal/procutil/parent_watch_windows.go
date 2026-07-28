//go:build windows

package procutil

import (
	"context"
	"time"
)

// WatchParentExit is a no-op on Windows: the kill-on-close job object created by
// RegisterCommandProcessGroup terminates the child when the launcher exits.
func WatchParentExit(_ context.Context, _ time.Duration, _ func()) {}
