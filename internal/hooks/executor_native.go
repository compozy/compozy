package hooks

import (
	"context"
	"fmt"
)

// NativeHookFunc executes an in-process hook without crossing a subprocess
// boundary.
type NativeHookFunc func(ctx context.Context, hook RegisteredHook, payload []byte) ([]byte, error)

// NativeExecutor runs hooks as direct Go callbacks.
type NativeExecutor struct {
	callback NativeHookFunc
}

// TypedNativeHookFunc executes a typed in-process hook without crossing a
// serialization boundary.
type TypedNativeHookFunc[P any, R any] func(ctx context.Context, hook RegisteredHook, payload P) (R, error)

// TypedNativeExecutor runs hooks as direct Go callbacks on typed payloads.
type TypedNativeExecutor[P any, R any] struct {
	callback TypedNativeHookFunc[P, R]
}

// NewNativeExecutor constructs a NativeExecutor bound to one callback.
func NewNativeExecutor(callback NativeHookFunc) *NativeExecutor {
	return &NativeExecutor{callback: callback}
}

// NewTypedNativeExecutor constructs a typed native executor for pipeline use.
func NewTypedNativeExecutor[P any, R any](callback TypedNativeHookFunc[P, R]) *TypedNativeExecutor[P, R] {
	return &TypedNativeExecutor[P, R]{callback: callback}
}

// Kind returns the executor type.
func (*NativeExecutor) Kind() HookExecutorKind {
	return HookExecutorNative
}

// Kind returns the executor type.
func (*TypedNativeExecutor[P, R]) Kind() HookExecutorKind {
	return HookExecutorNative
}

// Execute invokes the configured Go callback directly.
func (e *NativeExecutor) Execute(ctx context.Context, hook RegisteredHook, payload []byte) (result []byte, err error) {
	if e == nil || e.callback == nil {
		return nil, fmt.Errorf("hooks: hook %q: %w", hook.Name, ErrNativeCallbackRequired)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("hooks: hook %q native callback panic: %v", hook.Name, recovered)
			result = nil
		}
	}()

	return e.callback(ctx, hook, payload)
}

// Execute preserves the Executor contract but the typed native path is expected
// to be invoked through pipeline.execute.
func (e *TypedNativeExecutor[P, R]) Execute(_ context.Context, hook RegisteredHook, _ []byte) ([]byte, error) {
	if e == nil || e.callback == nil {
		return nil, fmt.Errorf("hooks: hook %q: %w", hook.Name, ErrNativeCallbackRequired)
	}

	return nil, fmt.Errorf("hooks: hook %q typed native executor must be invoked through pipeline", hook.Name)
}

// ExecuteTyped invokes the configured typed Go callback directly.
func (e *TypedNativeExecutor[P, R]) ExecuteTyped(
	ctx context.Context,
	hook RegisteredHook,
	payload P,
) (result R, err error) {
	if e == nil || e.callback == nil {
		return result, fmt.Errorf("hooks: hook %q: %w", hook.Name, ErrNativeCallbackRequired)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("hooks: hook %q native typed callback panic: %v", hook.Name, recovered)
			var zero R
			result = zero
		}
	}()

	return e.callback(ctx, hook, payload)
}
