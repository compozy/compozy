package hooks

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrNotImplemented reports that the requested executor seam is reserved for
	// future work.
	ErrNotImplemented = errors.New("hooks: executor not implemented")
	// ErrNativeCallbackRequired reports that a native executor lacks a callback.
	ErrNativeCallbackRequired = errors.New("hooks: native executor callback is required")
	// ErrSubprocessCommandRequired reports that a subprocess executor lacks a
	// command to start.
	ErrSubprocessCommandRequired = errors.New("hooks: subprocess executor command is required")
	// ErrInvalidHookExecutorKind reports an unsupported executor kind.
	ErrInvalidHookExecutorKind = errors.New("hooks: invalid hook executor kind")
)

// HookExecutorKind identifies the execution boundary for a hook.
type HookExecutorKind string

const (
	HookExecutorNative     HookExecutorKind = "native"
	HookExecutorSubprocess HookExecutorKind = "subprocess"
	HookExecutorWASM       HookExecutorKind = "wasm"
)

// Validate ensures the executor kind is supported.
func (k HookExecutorKind) Validate() error {
	switch k {
	case HookExecutorNative, HookExecutorSubprocess, HookExecutorWASM:
		return nil
	default:
		return fmt.Errorf("hooks: invalid hook executor kind %q: %w", k, ErrInvalidHookExecutorKind)
	}
}

// Executor is the execution seam for hook implementations.
type Executor interface {
	Kind() HookExecutorKind
	Execute(ctx context.Context, hook RegisteredHook, payload []byte) ([]byte, error)
}
