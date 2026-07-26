package hooks

import "context"

// WasmExecutor is the future execution seam for sandboxed hook runtimes.
type WasmExecutor struct{}

var _ Executor = (*WasmExecutor)(nil)

// Kind returns the executor type.
func (*WasmExecutor) Kind() HookExecutorKind {
	return HookExecutorWASM
}

// Execute reports that the Wasm executor seam is not implemented yet.
func (*WasmExecutor) Execute(_ context.Context, _ RegisteredHook, _ []byte) ([]byte, error) {
	return nil, ErrNotImplemented
}
