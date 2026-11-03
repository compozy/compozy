package mcpbridge

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// progressRelay maps bridge-issued progress tokens back to the HTTP request
// that started each tool call, so child progress reaches the agent under the
// token the agent chose. Rewriting keeps concurrent agent tokens from colliding.
type progressRelay struct {
	logger *slog.Logger
	mu     sync.Mutex
	next   uint64
	calls  map[string]progressTarget
}

type progressTarget struct {
	// ctx is the in-flight HTTP request context; notifications sent with it
	// travel on that request's response stream.
	ctx     context.Context
	session *sdkmcp.ServerSession
	token   any
}

func (r *progressRelay) register(
	ctx context.Context,
	session *sdkmcp.ServerSession,
	token any,
) (string, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	relayToken := fmt.Sprintf("compozy-bridge-%d", r.next)
	r.calls[relayToken] = progressTarget{ctx: ctx, session: session, token: token}
	return relayToken, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.calls, relayToken)
	}
}

// relay forwards one child progress notification; unknown or finished calls are dropped.
func (r *progressRelay) relay(_ context.Context, params *sdkmcp.ProgressNotificationParams) {
	if params == nil {
		return
	}
	token, ok := params.ProgressToken.(string)
	if !ok {
		return
	}
	r.mu.Lock()
	target, found := r.calls[token]
	r.mu.Unlock()
	if !found || target.ctx.Err() != nil {
		return
	}
	// A failed notification only loses one keepalive; the call itself continues.
	if err := target.session.NotifyProgress(target.ctx, &sdkmcp.ProgressNotificationParams{
		ProgressToken: target.token,
		Message:       params.Message,
		Progress:      params.Progress,
		Total:         params.Total,
	}); err != nil && r.logger != nil {
		r.logger.Debug("mcpbridge: relay tool progress", "error", err)
	}
}
