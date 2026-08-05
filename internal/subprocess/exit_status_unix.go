//go:build darwin || linux

package subprocess

import (
	"os"
	"syscall"
)

func processStateSignal(state *os.ProcessState) string {
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() {
		return ""
	}
	return status.Signal().String()
}
