package udsapi

import (
	"context"
	"net/http"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const hostedWaitProgressWriteTimeout = compozyconfig.DefaultClientRequestTimeout

func withHostedWaitProgress(ctx context.Context, writer http.ResponseWriter) context.Context {
	return toolspkg.WithHostedWaitProgress(ctx, func() func() {
		progressCtx, cancel := context.WithCancel(ctx)
		done := make(chan struct{})
		go func() {
			defer close(done)
			reportHostedWaitProgress(progressCtx, writer)
		}()
		return func() {
			cancel()
			<-done
		}
	})
}

func reportHostedWaitProgress(ctx context.Context, writer http.ResponseWriter) {
	ticker := time.NewTicker(mcppkg.HostedToolProgressInterval)
	defer ticker.Stop()
	controller := http.NewResponseController(writer)
	for ctx.Err() == nil {
		if err := controller.SetWriteDeadline(time.Now().Add(hostedWaitProgressWriteTimeout)); err != nil {
			return
		}
		writer.WriteHeader(http.StatusProcessing)
		if err := controller.SetWriteDeadline(time.Time{}); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
