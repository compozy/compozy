//go:build windows

package cli

import (
	"syscall"
)

func daemonLaunchSysProcAttr() *syscall.SysProcAttr {
	// CREATE_NEW_PROCESS_GROUP detaches the daemon from the parent console's
	// process group so that Ctrl+C events are not forwarded to the daemon.
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
