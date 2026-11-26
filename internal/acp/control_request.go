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
	rpcCtx, span := startRPCSpan(ctx, process.providerName, method)
	requestCtx, cancel := context.WithTimeout(rpcCtx, process.controlRequestTimeout(method))
	defer cancel()
	response, err := acpsdk.SendRequest[T](process.conn, requestCtx, method, params)
	endRPCSpan(ctx, span, err)
	if err != nil {
		cause := errors.Join(requestCtx.Err(), err)
		cause = classifyLoadSessionTimeout(ctx, requestCtx, method, cause)
		if method == acpsdk.AgentMethodInitialize && isNPX(process.Command) &&
			errors.Is(requestCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil && !IsAuthRequiredError(cause) {
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

// Loading a stored transcript can take as long as creating a provider session.
const defaultSessionLoadTimeout = 90 * time.Second

func classifyLoadSessionTimeout(
	callerCtx context.Context,
	requestCtx context.Context,
	method string,
	err error,
) error {
	if method != acpsdk.AgentMethodSessionLoad && method != acpsdk.AgentMethodSessionResume {
		return err
	}
	if callerCtx.Err() != nil {
		return err
	}
	if !errors.Is(requestCtx.Err(), context.DeadlineExceeded) && !requestErrorIndicatesDeadline(err) {
		return err
	}
	return errors.Join(ErrLoadSessionTimeout, err)
}

func (process *AgentProcess) controlRequestTimeout(method string) time.Duration {
	if process.controlTimeout > 0 {
		return process.controlTimeout
	}
	switch method {
	case acpsdk.AgentMethodSessionNew:
		return defaultSessionNewTimeout
	case acpsdk.AgentMethodSessionLoad, acpsdk.AgentMethodSessionResume:
		return defaultSessionLoadTimeout
	default:
		return defaultControlTimeout
	}
}
