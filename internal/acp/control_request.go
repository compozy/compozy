package acp

import (
	"context"
	"errors"
	"fmt"

	acpsdk "github.com/coder/acp-go-sdk"
)

var errSessionNewTimeout = errors.New("acp: session/new control deadline exceeded")

func (process *AgentProcess) sendControlRequest[T any](ctx context.Context, method string, params any) (T, error) {
	timeout := process.controlTimeout
	if timeout <= 0 {
		timeout = defaultControlTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	response, err := acpsdk.SendRequest[T](process.conn, requestCtx, method, params)
	if err != nil {
		cause := errors.Join(requestCtx.Err(), err)
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
