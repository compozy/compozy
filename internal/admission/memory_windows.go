package admission

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var globalMemoryStatusEx = windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")

type memoryStatusEx struct {
	length, memoryLoad                               uint32
	totalPhys, availPhys                             uint64
	totalPageFile, availPageFile                     uint64
	totalVirtual, availVirtual, availExtendedVirtual uint64
}

func sampleHostMemory() (MemorySnapshot, error) {
	status := memoryStatusEx{length: uint32(unsafe.Sizeof(memoryStatusEx{}))}
	ok, _, err := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return MemorySnapshot{}, err
	}
	return MemorySnapshot{Total: status.totalPhys, Available: status.availPhys}, nil
}
