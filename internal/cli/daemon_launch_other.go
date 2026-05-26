//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package cli

import "syscall"

func daemonLaunchSysProcAttr() *syscall.SysProcAttr {
	return nil
}
