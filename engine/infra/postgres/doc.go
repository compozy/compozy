// Package postgres provides the PostgreSQL driver implementation for the
// storage layer. This package intentionally contains only driver-specific
// code (connection pool management, migrations and scanning helpers) and
// must not leak pgx or driver types outside of its public API.
//
// Note: Task 3.0 is blocked by Task 2.0 (contracts-only store package).
// The initial commit adds a compile-safe skeleton without changing
// behavior. Integration and refactors will follow once contracts are in
// place.
package postgres
