package admission

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func sampleHostMemory() (MemorySnapshot, error) {
	total, err := unix.SysctlUint64("hw.memsize")
	if err != nil {
		return MemorySnapshot{}, err
	}
	// vm_stat reads host_statistics64 even in releases built without cgo.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/vm_stat").Output()
	if err != nil {
		return MemorySnapshot{}, err
	}
	var availablePages uint64
	var pageCounters int
	for line := range strings.SplitSeq(string(out), "\n") {
		label, value, ok := strings.Cut(line, ":")
		if !ok || (label != "Pages free" && label != "Pages inactive" && label != "Pages purgeable") {
			continue
		}
		pages, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimSpace(value), "."), 10, 64)
		if err != nil {
			return MemorySnapshot{}, err
		}
		pageCounters++
		availablePages += pages
	}
	if pageCounters != 3 {
		return MemorySnapshot{}, errHostMemoryUnavailable
	}
	return MemorySnapshot{Total: total, Available: min(total, availablePages*uint64(os.Getpagesize()))}, nil
}
