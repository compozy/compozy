//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package main

import (
	"context"
	"os"
	"os/signal"
)

func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}
