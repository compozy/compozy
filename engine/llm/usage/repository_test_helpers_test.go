package usage

import "context"

func newRepositoryForTest(
	persistFn func(context.Context, *Finalized) error,
	queue chan *persistRequest,
) *Repository {
	if persistFn == nil {
		persistFn = func(context.Context, *Finalized) error { return nil }
	}
	return &Repository{
		persist:   persistFn,
		queue:     queue,
		metrics:   &repositoryMetrics{},
		queueSize: cap(queue),
	}
}
