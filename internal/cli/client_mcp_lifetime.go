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
	timer := time.AfterFunc(timeout, func() { cancel(errHostedMCPCallFallback) })
	var mu sync.Mutex
	trace := &httptrace.ClientTrace{Got1xxResponse: func(status int, _ textproto.MIMEHeader) error {
		if status == http.StatusProcessing {
			mu.Lock()
			if ctx.Err() == nil {
				timer.Reset(timeout)
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
