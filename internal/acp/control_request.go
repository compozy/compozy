package acp

import (
	"context"
	"errors"
	"fmt"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
)

var errSessionNewTimeout = errors.New("acp: session/new control deadline exceeded")

func (process *AgentProcess) sendControlRequest[T any](ctx context.Context, method string, params any) (T, error) {
	requestCtx, cancel := context.WithTimeout(ctx, process.controlRequestTimeout(method))
	defer cancel()
	response, err := acpsdk.SendRequest[T](process.conn, requestCtx, method, params)
	if err != nil {
		cause := errors.Join(requestCtx.Err(), err)
		if method == acpsdk.AgentMethodInitialize && isNPX(process.Command) &&
			errors.Is(requestCtx.Err(), context.DeadlineExceeded) && !IsAuthRequiredError(cause) {
			cause = npxDiagnostic("initialize", cause)
		}
		if method == acpsdk.AgentMethodSessionNew && errors.Is(requestCtx.Err(), context.DeadlineExceeded) &&
			ctx.Err() == nil {
			cause = errors.Join(errSessionNewTimeout, cause)
		}
		return response, fmt.Errorf(
			"acp: %s for provider %q agent %q: %w",
			method,
			process.providerName,
			process.AgentName,
			cause,
		)
	}
	return response, nil
}

// Session creation loads provider plugins, hooks and MCP servers before replying.
const defaultSessionNewTimeout = 90 * time.Second

func (process *AgentProcess) controlRequestTimeout(method string) time.Duration {
	if process.controlTimeout > 0 {
		return process.controlTimeout
	}
	if method == acpsdk.AgentMethodSessionNew {
		return defaultSessionNewTimeout
	}
	return defaultControlTimeout
}
