package store

import "context"

// Store defines the transactional boundary for data access. Implementations
// manage begin/commit/rollback internally and expose repository access through
// a repositories factory provided to the transactional closure.
type Store interface {
	// WithTransaction executes fn within a single transaction. If fn returns
	// an error, the transaction is rolled back; otherwise it is committed.
	WithTransaction(ctx context.Context, fn func(Repositories) error) error

	// ReadOnly returns a repositories accessor suitable for non-mutating
	// operations that do not require a transaction.
	ReadOnly(ctx context.Context) Repositories
}

// Repositories is an abstract accessor for repository instances valid within
// the scope they were created (transaction-scoped for WithTransaction, or
// read-only when obtained via ReadOnly). Implementations should provide
// strongly-typed accessors in their concrete packages.
type Repositories any
