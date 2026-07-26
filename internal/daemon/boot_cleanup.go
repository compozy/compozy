package daemon

import (
	"context"
	"errors"
	"slices"
)

type bootCleanup struct {
	fns []func(context.Context) error
}

func (c *bootCleanup) add(fn func(context.Context) error) {
	if fn == nil {
		return
	}
	c.fns = append(c.fns, fn)
}

func (c *bootCleanup) run(ctx context.Context, err *error) {
	if err == nil || *err == nil {
		return
	}
	if ctx == nil {
		ctx = context.WithoutCancel(context.TODO())
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()

	var cleanupErrs []error
	for _, cleanup := range slices.Backward(c.fns) {
		if cleanupErr := cleanup(cleanupCtx); cleanupErr != nil {
			cleanupErrs = append(cleanupErrs, cleanupErr)
		}
	}
	*err = errors.Join(*err, errors.Join(cleanupErrs...))
}
