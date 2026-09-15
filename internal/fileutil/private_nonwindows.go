//go:build !windows

package fileutil

import (
	"errors"
	"os"
)

func checkPrivatePermissions(_ *os.File, info os.FileInfo) error {
	if info.Mode().Perm() != 0o600 {
		return errors.New("fileutil: private file permissions must be 0600")
	}
	return nil
}

func checkPrivateDirectory(*os.File) error { return nil }
