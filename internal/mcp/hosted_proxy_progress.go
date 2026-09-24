package mcp

import (
	"context"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// hostedToolProgressInterval paces hosted tool call progress notifications at
// a 2x margin under the ~60s default request timeout enforced by MCP clients
// such as opencode, whose callTool runs with resetTimeoutOnProgress and aborts
// blocked calls that receive no progress. Not a config key by design.
const hostedToolProgressInterval = 30 * time.Second

// startHostedToolProgress emits standard notifications/progress while one
// hosted tool call blocks, resetting the client-side request timer so long
// waits such as a held clarification survive instead of surfacing as a
// canceled call. Clients that did not send a progress token receive nothing;
// delivery failures are advisory and the next tick retries.
func startHostedToolProgress(ctx context.Context, req *sdkmcp.CallToolRequest) func() {
	if req == nil || req.Session == nil || req.Params == nil || req.Params.GetProgressToken() == nil {
		return func() {}
	}
	session := req.Session
	token := req.Params.GetProgressToken()
	progressCtx, cancel := context.WithCancel(ctx)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(hostedToolProgressInterval)
		defer ticker.Stop()
		for progress := float64(1); ; progress++ {
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
			ping := &sdkmcp.ProgressNotificationParams{
				ProgressToken: token,
				Progress:      progress,
				Message:       "compozy: tool call still running",
			}
			// Delivery failures are advisory; the next tick retries.
			_ = session.NotifyProgress(progressCtx, ping) //nolint:errcheck // delivery is advisory; next tick retries.
		}
	}()
	return func() {
		close(stop)
		cancel()
		<-done
	}
}
