//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package daemon

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func notifyDetachedDaemonContext(base context.Context) (context.Context, context.CancelFunc, error) {
	ctx, cancel := signal.NotifyContext(base, os.Interrupt, syscall.SIGTERM)
	return ctx, cancel, nil
}
