//go:build !linux && !darwin && !windows

package fileutil

import "os"

func moveDirectoryNoFollow(_ *os.File, _ *os.File, _ string, _ *os.File, _ string, replace bool) error {
	if replace {
		return ErrAtomicNoReplaceUnsupported
	}
	return ErrAtomicNoReplaceUnsupported
}
