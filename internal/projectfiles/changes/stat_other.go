//go:build !darwin && !linux && !windows

package changes

import "os"

func statIdentity(_ *os.File, info os.FileInfo) (string, error) { return info.Name(), nil }
