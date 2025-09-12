package attachments

import (
	"path/filepath"
	"runtime"
)

func getTestDir() string {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(filename)
}
