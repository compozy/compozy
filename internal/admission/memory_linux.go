package admission

import "os"

func sampleHostMemory() (MemorySnapshot, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return MemorySnapshot{}, err
	}
	return parseMeminfo(string(data))
}
