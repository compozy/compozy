//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package cli

import "syscall"

func daemonLaunchSysProcAttr() *syscall.SysProcAttr {
	return nil
}
