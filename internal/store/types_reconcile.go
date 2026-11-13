package store

import "context"

type ReconcileResult struct {
	Indexed  []string
	Orphaned []string
}

// SessionChangeReconcileResult is a change reconcile's outcome together with the catalog rows it
// wrote. Each row in Written was read back inside the transaction that wrote it, in the shape
// ListSessions returns, so a caller can tell that row apart from one another writer left later.
type SessionChangeReconcileResult struct {
	ReconcileResult
	Written []SessionInfo
}

// SessionChangeReconciler reconciles the catalog against a session directory whose unchanged
// members were proven identical to an earlier reconcile instead of being read again. Catalog
// sessions that are neither changed nor unchanged are marked orphaned.
type SessionChangeReconciler interface {
	ReconcileSessionChanges(
		ctx context.Context,
		changed []SessionInfo,
		unchanged []string,
	) (SessionChangeReconcileResult, error)
}
