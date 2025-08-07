package orchestrator

import "time"

// Timeout constants for different operations
const (
	// DefaultWorkflowTimeout is the standard timeout for PR and dry-run workflows
	DefaultWorkflowTimeout = 60 * time.Minute
	// ReleaseWorkflowTimeout is the extended timeout for release operations
	ReleaseWorkflowTimeout = 120 * time.Minute
	// RollbackTimeout is the timeout for rollback operations
	RollbackTimeout = 10 * time.Minute
	// DefaultRetryCount is the standard number of retries for operations
	DefaultRetryCount = 3
	// DefaultRetryDelay is the initial delay for exponential backoff
	DefaultRetryDelay = 1 * time.Second
)

// File permission constants
const (
	// FilePermissionsReadWrite is the standard permission for created files
	FilePermissionsReadWrite = 0644
	// FilePermissionsSecure is the secure permission for sensitive files
	FilePermissionsSecure = 0600
	// DirPermissionsDefault is the standard permission for created directories
	DirPermissionsDefault = 0755
)
