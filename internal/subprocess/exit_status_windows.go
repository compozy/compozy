//go:build windows

package subprocess

import "os"

func processStateSignal(_ *os.ProcessState) string {
	return ""
}
