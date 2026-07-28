// Package scheduler implements the daemon-owned mechanical task scheduler.
//
// The scheduler is intentionally narrow: it sweeps expired task-run leases
// through the task service, selects eligible idle sessions for queued work, and
// emits wake notifications. It never claims task runs directly; sessions remain
// responsible for calling the task claim API.
package scheduler
