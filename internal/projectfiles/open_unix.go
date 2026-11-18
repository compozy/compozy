//go:build !windows

package projectfiles

import "syscall"

const regularOpenFlags = syscall.O_NONBLOCK
const directoryOpenFlags = syscall.O_NONBLOCK | syscall.O_DIRECTORY
