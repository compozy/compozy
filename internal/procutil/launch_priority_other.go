//go:build !linux

package procutil

// StartWithNiceness leaves non-Linux priority to the post-start process-group adjustment.
func StartWithNiceness(_ int, start func() error) (priorityErr, startErr error) {
	return nil, start()
}
