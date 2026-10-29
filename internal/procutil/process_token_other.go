//go:build !linux

package procutil

// ProcessToken leaves native timestamp identity unchanged outside Linux.
func ProcessToken(_ int) (string, error) { return "", nil }
