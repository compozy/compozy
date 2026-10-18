//go:build darwin

package admission

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

// darwinLoadavgSize is sizeof(struct loadavg): fixpt_t ldavg[3], padding, long fscale.
const darwinLoadavgSize = 24

func sampleLoadAverage() (float64, error) {
	raw, err := unix.SysctlRaw("vm.loadavg")
	if err != nil {
		return 0, fmt.Errorf("admission: read vm.loadavg: %w", err)
	}
	if len(raw) < darwinLoadavgSize {
		return 0, fmt.Errorf("admission: vm.loadavg returned %d bytes, want %d", len(raw), darwinLoadavgSize)
	}
	load := binary.NativeEndian.Uint32(raw[0:4])
	scale := binary.NativeEndian.Uint64(raw[16:24])
	if scale == 0 {
		return 0, fmt.Errorf("admission: vm.loadavg reported zero fscale")
	}
	return float64(load) / float64(scale), nil
}
