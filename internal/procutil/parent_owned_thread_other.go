//go:build !linux

package procutil

import "os/exec"

func pinParentCommandThread(*exec.Cmd) func() { return func() {} }
