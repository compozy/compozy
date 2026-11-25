package store

import "context"

// MutationCommitFence revalidates an authenticated mutation at its commit boundary.
type MutationCommitFence func(context.Context) error

type mutationCommitFenceContextKey struct{}
type mutationCommitGuardContextKey struct{}

// MutationCommitGuard serializes the last authorization check with the durable commit.
type MutationCommitGuard func(context.Context) (release func(), err error)

func ContextWithMutationCommitGuard(ctx context.Context, guard MutationCommitGuard) context.Context {
	return context.WithValue(ctx, mutationCommitGuardContextKey{}, guard)
}

// WithoutMutationCommitGuard marks work that has passed into runtime ownership.
func WithoutMutationCommitGuard(ctx context.Context) context.Context {
	return ContextWithMutationCommitGuard(ctx, nil)
}

// ContextWithMutationCommitFence carries a commit-time authorization check through service layers.
func ContextWithMutationCommitFence(ctx context.Context, fence MutationCommitFence) context.Context {
	if ctx == nil || fence == nil {
		return ctx
	}
	return context.WithValue(ctx, mutationCommitFenceContextKey{}, fence)
}

func validateMutationCommitFence(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	fence, ok := ctx.Value(mutationCommitFenceContextKey{}).(MutationCommitFence)
	if !ok || fence == nil {
		return nil
	}
	return fence(ctx)
}
