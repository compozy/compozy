package tailscale

import (
	"context"
	"errors"
	"fmt"
)

type operationGate struct {
	token chan struct{}
}

func newOperationGate() *operationGate {
	gate := &operationGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	return gate
}

func (g *operationGate) acquire(ctx context.Context) error {
	if g == nil {
		return errors.New("tailscale: operation gate is required")
	}
	if ctx == nil {
		return errors.New("tailscale: operation context is required")
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("tailscale: wait for provider operation: %w", ctx.Err())
	case <-g.token:
		return nil
	}
}

func (g *operationGate) release() {
	if g != nil {
		g.token <- struct{}{}
	}
}
