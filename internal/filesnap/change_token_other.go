//go:build !darwin && !linux

package filesnap

import "io/fs"

type changeToken struct {
	available bool
}

// fileChangeToken requires content fingerprints when the OS lacks authoritative change metadata.
func fileChangeToken(fs.FileInfo) changeToken {
	return changeToken{}
}
