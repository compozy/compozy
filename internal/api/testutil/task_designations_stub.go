package testutil

import (
	"context"

	"github.com/compozy/compozy/internal/store"
)

// StubTaskDesignationStore captures the task-owned fan-out persistence boundary.
type StubTaskDesignationStore struct {
	PutTaskDesignationRollupFn   func(context.Context, store.TaskDesignationRollup) error
	ListTaskDesignationRollupsFn func(
		context.Context,
		store.TaskDesignationRollupQuery,
	) ([]store.TaskDesignationRollup, error)
}

func (s StubTaskDesignationStore) PutTaskDesignationRollup(
	ctx context.Context,
	rollup store.TaskDesignationRollup,
) error {
	if s.PutTaskDesignationRollupFn != nil {
		return s.PutTaskDesignationRollupFn(ctx, rollup)
	}
	return nil
}

func (s StubTaskDesignationStore) ListTaskDesignationRollups(
	ctx context.Context,
	query store.TaskDesignationRollupQuery,
) ([]store.TaskDesignationRollup, error) {
	if s.ListTaskDesignationRollupsFn != nil {
		return s.ListTaskDesignationRollupsFn(ctx, query)
	}
	return nil, nil
}
