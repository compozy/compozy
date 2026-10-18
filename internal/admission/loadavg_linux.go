//go:build linux

package admission

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// linuxLoadScale is 1 << SI_LOAD_SHIFT for sysinfo(2) load averages.
const linuxLoadScale = 1 << 16

func sampleLoadAverage() (float64, error) {
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0, fmt.Errorf("admission: sysinfo: %w", err)
	}
	return float64(info.Loads[0]) / linuxLoadScale, nil
}
