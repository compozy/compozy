package builtin

import "github.com/compozy/compozy/engine/core"

// Canonical error codes shared across builtin tool handlers.
const (
	CodeInvalidArgument   = "InvalidArgument"
	CodePermissionDenied  = "PermissionDenied"
	CodeFileNotFound      = "FileNotFound"
	CodeCommandNotAllowed = "CommandNotAllowed"
	CodeInternal          = "Internal"
	CodeDeadlineExceeded  = "DeadlineExceeded"
)

func newError(code string, err error, details map[string]any) *core.Error {
	return core.NewError(err, code, details)
}

// InvalidArgument wraps validation failures with the canonical error code.
func InvalidArgument(err error, details map[string]any) *core.Error {
	return newError(CodeInvalidArgument, err, details)
}

// PermissionDenied identifies sandbox permission violations.
func PermissionDenied(err error, details map[string]any) *core.Error {
	return newError(CodePermissionDenied, err, details)
}

// FileNotFound maps missing filesystem entries to the shared error catalog.
func FileNotFound(err error, details map[string]any) *core.Error {
	return newError(CodeFileNotFound, err, details)
}

// CommandNotAllowed describes exec allowlist violations.
func CommandNotAllowed(err error, details map[string]any) *core.Error {
	return newError(CodeCommandNotAllowed, err, details)
}

// Internal signals unexpected builtin failures.
func Internal(err error, details map[string]any) *core.Error {
	return newError(CodeInternal, err, details)
}

// DeadlineExceeded reports timeout conditions surfaced by builtin handlers.
func DeadlineExceeded(err error, details map[string]any) *core.Error {
	return newError(CodeDeadlineExceeded, err, details)
}
