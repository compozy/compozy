// Package terminal owns resident terminal processes, their byte streams, and control leases.
//
// Contracts live in contracts.go; registry and lifecycle in manager.go and session.go;
// output aggregation in coalescer.go; transport filtering in osc_filter.go; audit
// admission in audit_gate.go; screen state in vt/; and OS process handling in pty/.
package terminal
