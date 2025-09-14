package store

import "errors"

// Shared, driver-neutral sentinel errors used across the codebase. Keeping
// these in the contracts package prevents duplication in driver packages and
// avoids leaking driver-specific error types to callers.

// ErrTaskNotFound is returned when a task state cannot be found.
var ErrTaskNotFound = errors.New("task state not found")

// ErrWorkflowNotFound is returned when a workflow state cannot be found.
var ErrWorkflowNotFound = errors.New("workflow state not found")

// ErrWorkflowNotReady indicates the workflow cannot be completed because it
// still has running tasks.
var ErrWorkflowNotReady = errors.New("workflow not ready for completion")
