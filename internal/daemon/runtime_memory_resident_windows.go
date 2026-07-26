//go:build windows

package daemon

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getProcessMemoryInfo = windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

type processMemoryCounters struct {
	Size                    uint32
	PageFaultCount          uint32
	PeakWorkingSetSize      uintptr
	WorkingSetSize          uintptr
	QuotaPeakPagedPoolUsage uintptr
	QuotaPagedPoolUsage     uintptr
	QuotaPeakNonPagedPool   uintptr
	QuotaNonPagedPoolUsage  uintptr
	PagefileUsage           uintptr
	PeakPagefileUsage       uintptr
	PrivateUsage            uintptr
}

func collectResidentMemory() (uint64, string, error) {
	process, err := windows.GetCurrentProcess()
	if err != nil {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf("get current process: %w", err)
	}
	counters := processMemoryCounters{Size: uint32(unsafe.Sizeof(processMemoryCounters{}))}
	result, _, callErr := getProcessMemoryInfo.Call(
		uintptr(process),
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.Size),
	)
	if result == 0 {
		return 0, runtimeMemoryResidentKindCurrent, fmt.Errorf("get process memory info: %w", callErr)
	}
	return uint64(counters.WorkingSetSize), runtimeMemoryResidentKindCurrent, nil
}
