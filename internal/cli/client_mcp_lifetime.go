package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// Allow the longest finite tool wait and a preceding approval to finish.
const hostedMCPLongestFiniteWait = compozyconfig.MaxToolsClarifyTimeout +
	time.Duration(compozyconfig.MaxToolsApprovalTimeoutSeconds)*time.Second

var errHostedMCPCallFallback = fmt.Errorf("cli: hosted MCP call: %w", context.DeadlineExceeded)

func withHostedMCPCallFallback(ctx context.Context, timeout time.Duration) (context.Context, func()) {
	ctx, cancel := context.WithCancelCause(ctx)
	var mu sync.Mutex
	deadline := time.Now().Add(timeout)
	timer := time.AfterFunc(timeout, func() {
		mu.Lock()
		defer mu.Unlock()
		// Reset can leave a superseded callback already running.
		if !time.Now().Before(deadline) {
			cancel(errHostedMCPCallFallback)
		}
	})
	trace := &httptrace.ClientTrace{Got1xxResponse: func(status int, _ textproto.MIMEHeader) error {
		if status == http.StatusProcessing {
			mu.Lock()
			if now := time.Now(); ctx.Err() == nil && now.Before(deadline) {
				deadline = now.Add(timeout)
				timer.Reset(time.Until(deadline))
			}
			mu.Unlock()
		}
		return nil
	}}
	return httptrace.WithClientTrace(ctx, trace), func() {
		mu.Lock()
		defer mu.Unlock()
		cancel(nil)
		timer.Stop()
	}
}
