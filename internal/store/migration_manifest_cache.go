package store

import (
	"embed"
	"sync"
)

type embeddedMigrationDirectoryKey struct {
	files embed.FS
	dir   string
	name  string
}

var embeddedMigrationDirectories = struct {
	sync.Mutex
	loads map[embeddedMigrationDirectoryKey]func() (migrationDirectory, error)
}{loads: make(map[embeddedMigrationDirectoryKey]func() (migrationDirectory, error))}

func loadMigrationDirectory(stream MigrationStream) (migrationDirectory, error) {
	files, immutable := stream.FS.(embed.FS)
	if !immutable {
		return readMigrationDirectory(stream)
	}
	// Embedded bytes cannot change during this process; mutable filesystems are validated on every call.
	key := embeddedMigrationDirectoryKey{files: files, dir: stream.Dir, name: stream.Name}
	embeddedMigrationDirectories.Lock()
	load := embeddedMigrationDirectories.loads[key]
	if load == nil {
		load = sync.OnceValues(func() (migrationDirectory, error) { return readMigrationDirectory(stream) })
		embeddedMigrationDirectories.loads[key] = load
	}
	embeddedMigrationDirectories.Unlock()
	return load()
}
