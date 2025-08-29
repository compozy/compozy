package runtime

import (
	"fmt"
	"slices"

	"github.com/compozy/compozy/engine/core"
)

// Runtime type constants
const (
	// RuntimeTypeBun represents the Bun JavaScript runtime
	RuntimeTypeBun = "bun"
	// RuntimeTypeNode represents the Node.js runtime
	RuntimeTypeNode = "node"
)

// SupportedRuntimeTypes contains all supported runtime types
var SupportedRuntimeTypes = []string{
	RuntimeTypeBun,
	RuntimeTypeNode,
}

// IsValidRuntimeType checks if the given runtime type is valid
func IsValidRuntimeType(runtimeType string) bool {
	return slices.Contains(SupportedRuntimeTypes, runtimeType)
}

// ToolExecutionError provides structured error information with context
type ToolExecutionError struct {
	ToolID     string
	ToolExecID string
	Operation  string
	Err        error
}

func (e *ToolExecutionError) Error() string {
	return fmt.Sprintf("tool execution failed for tool %s (exec %s) during %s: %v",
		e.ToolID, e.ToolExecID, e.Operation, e.Err)
}

func (e *ToolExecutionError) Unwrap() error {
	return e.Err
}

// ProcessError provides structured error information for runtime process issues
type ProcessError struct {
	Operation string
	Err       error
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("runtime process %s failed: %v", e.Operation, e.Err)
}

func (e *ProcessError) Unwrap() error {
	return e.Err
}

// -----
// Types
// -----

// ToolExecuteParams represents the parameters for Tool.Execute method
type ToolExecuteParams struct {
	ToolID     string      `json:"tool_id"`
	ToolExecID string      `json:"tool_exec_id"`
	Input      *core.Input `json:"input"`
	Config     *core.Input `json:"config"`
	Env        core.EnvMap `json:"env"`
	// Optional per-execution timeout (milliseconds) propagated to the JS worker
	// When not provided, the worker applies its internal default (60s)
	TimeoutMs int64 `json:"timeout_ms,omitempty"`
}

// ToolExecuteResult represents the result of Tool.Execute method
// The tool output is returned directly as core.Output (map[string]any)
type ToolExecuteResult = core.Output
