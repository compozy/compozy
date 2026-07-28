//go:build !darwin && !linux && !windows

package daemon

import "errors"

func collectResidentMemory() (uint64, string, error) {
	return 0, "unavailable", errors.New("resident memory collection is unavailable on this platform")
}
