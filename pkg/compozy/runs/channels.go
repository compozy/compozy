package runs

import (
	"context"

	"github.com/compozy/compozy/pkg/compozy/events"
)

func sendRunEvent(ctx context.Context, dst chan<- events.Event, ev events.Event) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case dst <- ev:
		return true
	}
}

func sendRunError(ctx context.Context, dst chan<- error, err error) bool {
	if err == nil {
		return true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case dst <- err:
		return true
	}
}
