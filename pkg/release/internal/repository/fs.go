package repository

import "github.com/spf13/afero"

// FileSystemRepository defines the interface for filesystem operations.

type FileSystemRepository interface {
	afero.Fs
}
