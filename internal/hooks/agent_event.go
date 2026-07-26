package hooks

import "context"

// OnAgentEvent remains a compatibility no-op. The daemon translates streamed
// ACP events into concrete tool and permission hook payloads before dispatch.
func (h *Hooks) OnAgentEvent(_ context.Context, _ SessionContext, _ any) {}
