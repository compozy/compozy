//go:build !darwin && !linux

package admission

func sampleLoadAverage() (float64, error) {
	return 0, ErrLoadAverageUnsupported
}
