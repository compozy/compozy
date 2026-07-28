package store

type ReconcileResult struct {
	Indexed  []string
	Orphaned []string
}
