package store

import "context"

type ReconcileResult struct {
	Indexed  []string
	Orphaned []string
}

// SessionChangeReconciler reconciles the catalog against a session directory whose unchanged
// members were proven byte-identical to an earlier reconcile instead of being read again. Catalog
// sessions that are neither changed nor unchanged are marked orphaned.
type SessionChangeReconciler interface {
	ReconcileSessionChanges(ctx context.Context, changed []SessionInfo, unchanged []string) (ReconcileResult, error)
}
