package runtime

import (
	"context"

	engineruntime "github.com/compozy/compozy/engine/runtime"
)

// NativeToolsBuilder configures the enablement of builtin native tools exposed to Bun runtimes.
type NativeToolsBuilder struct {
	config *engineruntime.NativeToolsConfig
}

// NewNativeTools creates a builder with all native tools disabled by default.
func NewNativeTools() *NativeToolsBuilder {
	return &NativeToolsBuilder{config: &engineruntime.NativeToolsConfig{}}
}

// WithCallAgents enables the call_agents native tool.
func (b *NativeToolsBuilder) WithCallAgents() *NativeToolsBuilder {
	if b == nil {
		return nil
	}
	b.config.CallAgents = true
	return b
}

// WithCallWorkflows enables the call_workflows native tool.
func (b *NativeToolsBuilder) WithCallWorkflows() *NativeToolsBuilder {
	if b == nil {
		return nil
	}
	b.config.CallWorkflows = true
	return b
}

// Build returns a copy of the native tools configuration.
// A non-nil context is required to keep parity with other SDK builders.
func (b *NativeToolsBuilder) Build(ctx context.Context) *engineruntime.NativeToolsConfig {
	if b == nil || ctx == nil {
		return nil
	}
	return &engineruntime.NativeToolsConfig{CallAgents: b.config.CallAgents, CallWorkflows: b.config.CallWorkflows}
}
