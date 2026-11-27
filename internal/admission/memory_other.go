//go:build !linux && !darwin && !windows

package admission

func sampleHostMemory() (MemorySnapshot, error) {
	return MemorySnapshot{}, errHostMemoryUnavailable
}
