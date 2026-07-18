//go:build !linux

package cli

import "github.com/compozy/compozy/internal/daemon"

// killGuardianDaemonInstance force-kills the daemon recorded in info. Platforms
// without a pidfd-equivalent instance handle (e.g. darwin) verify ownership via
// the daemon's endpoint and signal info.PID directly; the probe-then-kill window
// is left as tight as the OS allows. See the linux build for the instance-safe
// pidfd path.
func killGuardianDaemonInstance(info daemon.Info) (bool, error) {
	return killGuardianDaemonByPID(info)
}
