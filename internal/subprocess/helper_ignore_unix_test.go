//go:build !windows

package subprocess

import (
	"os"
	"os/signal"
	"syscall"
)

func configureIgnoreTermination() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	go func() {
		for sig := range signals {
			_ = sig
		}
	}()
}
