package tools

import "context"

type immutableProjectionKey struct{}

// WithImmutableProjection borrows descriptor storage; consumers must never mutate returned descriptors.
func WithImmutableProjection(ctx context.Context) context.Context {
	return context.WithValue(ctx, immutableProjectionKey{}, true)
}

func projectionDescriptor(ctx context.Context, descriptor Descriptor) Descriptor {
	if borrowed, ok := ctx.Value(immutableProjectionKey{}).(bool); ok && borrowed {
		return descriptor
	}
	return cloneDescriptor(descriptor)
}
