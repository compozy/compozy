//go:build linux

package daemon

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func collectResidentMemory() (uint64, string, error) {
	raw, err := os.ReadFile("/proc/self/statm")
	if err != nil {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf("read /proc/self/statm: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) < 2 {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf("parse /proc/self/statm: expected resident page count")
	}
	residentPages, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf("parse /proc/self/statm resident pages: %w", err)
	}
	pageSize := os.Getpagesize()
	if pageSize <= 0 {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf(
			"get system page size: expected positive byte count, got %d",
			pageSize,
		)
	}
	pageSizeBytes := uint64(pageSize)
	if residentPages > math.MaxUint64/pageSizeBytes {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf(
			"calculate resident memory: %d pages of %d bytes overflow uint64",
			residentPages,
			pageSizeBytes,
		)
	}
	return residentPages * pageSizeBytes, runtimeMemoryResidentKindCurrent, nil
}
