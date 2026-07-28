package store

import "context"

// NetworkAuditStore manages network message audit entries.
type NetworkAuditStore interface {
	WriteNetworkAudit(ctx context.Context, entry NetworkAuditEntry) error
	ListNetworkAudit(ctx context.Context, query NetworkAuditQuery) ([]NetworkAuditEntry, error)
}
