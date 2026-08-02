package bridgesdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime/debug"
)

type rpcHandlerPanicError struct {
	panicType string
	stack     []byte
}

func (e *rpcHandlerPanicError) Error() string {
	return fmt.Sprintf("bridgesdk: rpc handler panicked with %s", e.panicType)
}

func invokeRPCHandler(
	ctx context.Context,
	handler RPCHandler,
	params json.RawMessage,
) (result any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = nil
			err = &rpcHandlerPanicError{
				panicType: fmt.Sprintf("%T", recovered),
				stack:     debug.Stack(),
			}
		}
	}()

	return handler(ctx, params)
}

func recordRPCHandlerPanic(ctx context.Context, method string, panicErr *rpcHandlerPanicError) {
	slog.ErrorContext(
		ctx,
		"bridge RPC handler panicked",
		slog.String("method", method),
		slog.String("panic_type", panicErr.panicType),
		slog.String("stack", string(panicErr.stack)),
	)
}
