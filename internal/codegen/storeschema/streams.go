// Package storeschema owns deterministic sqlc and Atlas generation for AGH stores.
package storeschema

import "path/filepath"

type stream struct {
	name          string
	schemaSource  schemaSource
	migrationsDir string
}

func streams(root string) []stream {
	return []stream{
		newStreamWithSource(root, "global", "internal/store/globaldb", "definitions"),
		newStream(root, "session", "internal/store/sessiondb"),
		newStream(root, "memory", "internal/memory"),
		newStream(root, "workspace", "internal/store/workspacedb"),
	}
}

func newStream(root, name, packagePath string) stream {
	return newStreamWithSource(root, name, packagePath, "schema.sql")
}

func newStreamWithSource(root, name, packagePath, sourceName string) stream {
	schemaDir := filepath.Join(root, packagePath, "schema")
	return stream{
		name:          name,
		schemaSource:  schemaSource{path: filepath.Join(schemaDir, sourceName)},
		migrationsDir: filepath.Join(schemaDir, "migrations"),
	}
}
