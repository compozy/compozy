package admission

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// MemorySnapshot holds physical memory in bytes. A zero total means unavailable.
type MemorySnapshot struct {
	Total     uint64
	Available uint64
}

var errHostMemoryUnavailable = errors.New("admission: host memory statistics are unavailable")

func parseMeminfo(input string) (MemorySnapshot, error) {
	var memory MemorySnapshot
	var haveAvailable bool
	for line := range strings.SplitSeq(input, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok || (key != "MemTotal" && key != "MemAvailable") {
			continue
		}
		fields := strings.Fields(value)
		if len(fields) != 2 || fields[1] != "kB" {
			return MemorySnapshot{}, errHostMemoryUnavailable
		}
		amount, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil || amount > math.MaxUint64/1024 {
			return MemorySnapshot{}, errHostMemoryUnavailable
		}
		if key == "MemTotal" {
			memory.Total = amount * 1024
		} else {
			memory.Available = amount * 1024
			haveAvailable = true
		}
	}
	if memory.Total == 0 || !haveAvailable || memory.Available > memory.Total {
		return MemorySnapshot{}, errHostMemoryUnavailable
	}
	return memory, nil
}
