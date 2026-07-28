//go:build darwin

package daemon

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const runtimeMemoryResidentKindPeak = "peak"

func collectResidentMemory() (uint64, string, error) {
	var usage unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
		return 0, runtimeMemoryResidentKindPeak, fmt.Errorf("get process resource usage: %w", err)
	}
	if usage.Maxrss < 0 {
		return 0, runtimeMemoryResidentKindPeak, fmt.Errorf(
			"get process resource usage: negative max resident bytes: %d",
			usage.Maxrss,
		)
	}
	return uint64(usage.Maxrss), runtimeMemoryResidentKindPeak, nil
}
