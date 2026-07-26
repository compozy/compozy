package store

import "path/filepath"

const (
	// SessionDatabaseName is the filename for per-session event storage.
	SessionDatabaseName = "events.db"
	// GlobalDatabaseName is the filename for the global AGH index database.
	GlobalDatabaseName = "agh.db"
	// SessionMetaName is the filename for quick session metadata lookups.
	SessionMetaName = "meta.json"
)

// SessionDBFile returns the canonical events database path for a session directory.
func SessionDBFile(sessionDir string) string {
	return filepath.Join(sessionDir, SessionDatabaseName)
}

// SessionMetaFile returns the canonical metadata file path for a session directory.
func SessionMetaFile(sessionDir string) string {
	return filepath.Join(sessionDir, SessionMetaName)
}
