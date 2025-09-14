// Package sqlite provides a SQLite driver implementation for the storage layer.
// It is optimized for the standalone/embedded mode using modernc.org/sqlite.
//
// Key features:
// - DSN with safe defaults (WAL, foreign_keys, busy_timeout)
// - Goose-based migrations (sqlite dialect) via embedded SQL files
// - Transaction-closure API compatible with store.Store contracts
// - JSON helpers for TEXT-backed columns
package sqlite
