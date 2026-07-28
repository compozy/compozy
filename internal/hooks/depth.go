package hooks

import (
	"context"
	"errors"
	"fmt"
)

const maxDispatchDepth = 3

type dispatchDepthContextKey struct{}
type dispatchChainContextKey struct{}

var ErrDispatchDepthExceeded = errors.New("hooks: dispatch depth exceeded")

func enterDispatch(ctx context.Context, event HookEvent) (context.Context, int, error) {
	depth := currentDispatchDepth(ctx) + 1
	if depth > maxDispatchDepth {
		return ctx, currentDispatchDepth(ctx), fmt.Errorf(
			"%w for event %q: depth %d exceeds max %d",
			ErrDispatchDepthExceeded,
			event,
			depth,
			maxDispatchDepth,
		)
	}

	nextChain := append(currentDispatchChain(ctx), event)
	nextCtx := context.WithValue(ctx, dispatchDepthContextKey{}, depth)
	nextCtx = context.WithValue(nextCtx, dispatchChainContextKey{}, nextChain)
	return nextCtx, depth, nil
}

func currentDispatchDepth(ctx context.Context) int {
	if ctx == nil {
		return 0
	}

	depth, ok := ctx.Value(dispatchDepthContextKey{}).(int)
	if !ok {
		return 0
	}
	return depth
}

func currentDispatchChain(ctx context.Context) []HookEvent {
	if ctx == nil {
		return nil
	}

	chain, ok := ctx.Value(dispatchChainContextKey{}).([]HookEvent)
	if !ok {
		return nil
	}
	if len(chain) == 0 {
		return nil
	}

	cloned := make([]HookEvent, len(chain))
	copy(cloned, chain)
	return cloned
}
