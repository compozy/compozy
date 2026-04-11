package run

import (
	"context"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/run/internal/acpshared"
)

// SwapNewAgentClientForTest lets higher-level package tests replace ACP client
// construction without importing lower-level internal runtime packages.
func SwapNewAgentClientForTest(
	fn func(context.Context, agent.ClientConfig) (agent.Client, error),
) func() {
	return acpshared.SwapNewAgentClientForTest(fn)
}
