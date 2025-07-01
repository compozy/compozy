package runtime

import (
	"context"
	"time"

	"github.com/compozy/compozy/engine/core"
)

// Runtime defines the interface for executing tools in different JavaScript runtimes.
// Implementations of this interface provide runtime-specific execution logic
// while maintaining a consistent API for the rest of the system.
type Runtime interface {
	// ExecuteTool executes a tool with the global timeout configuration.
	// It delegates to ExecuteToolWithTimeout using the runtime's default timeout.
	ExecuteTool(
		ctx context.Context,
		toolID string,
		toolExecID core.ID,
		input *core.Input,
		env core.EnvMap,
	) (*core.Output, error)

	// ExecuteToolWithTimeout executes a tool with a custom timeout.
	// The timeout parameter overrides the global timeout for this specific execution.
	// Returns the tool's output or an error if execution fails.
	ExecuteToolWithTimeout(
		ctx context.Context,
		toolID string,
		toolExecID core.ID,
		input *core.Input,
		env core.EnvMap,
		timeout time.Duration,
	) (*core.Output, error)

	// GetGlobalTimeout returns the configured global timeout for tool execution.
	// This timeout is used when ExecuteTool is called without a specific timeout.
	GetGlobalTimeout() time.Duration
}

// Compile-time check to ensure BunManager implements Runtime interface
var _ Runtime = (*BunManager)(nil)
