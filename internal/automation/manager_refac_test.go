package automation

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/agh/internal/testutil"
)

type mergedRuntimeContextKey string

func TestMergedRuntimeContextNilParent(t *testing.T) {
	t.Parallel()

	t.Run("Should use runtime context values when parent is nil", func(t *testing.T) {
		t.Parallel()

		key := mergedRuntimeContextKey("runtime-key")
		runtimeBase := context.WithValue(testutil.Context(t), key, "runtime-value")
		runtimeCtx, cancelRuntime := context.WithCancel(runtimeBase)
		t.Cleanup(cancelRuntime)

		mergedCtx, cancelMerged := mergedRuntimeContext(nilContextForMergedRuntimeTest(), runtimeCtx)
		t.Cleanup(cancelMerged)

		if mergedCtx == nil {
			t.Fatal("mergedRuntimeContext(nil, runtimeCtx) returned nil context")
		}
		if got, want := mergedCtx.Value(key), "runtime-value"; got != want {
			t.Fatalf("merged context value = %v, want %v", got, want)
		}
	})

	t.Run("Should follow runtime cancellation when parent is nil", func(t *testing.T) {
		t.Parallel()

		runtimeCtx, cancelRuntime := context.WithCancel(testutil.Context(t))
		t.Cleanup(cancelRuntime)
		mergedCtx, cancelMerged := mergedRuntimeContext(nilContextForMergedRuntimeTest(), runtimeCtx)
		t.Cleanup(cancelMerged)

		if mergedCtx == nil {
			t.Fatal("mergedRuntimeContext(nil, runtimeCtx) returned nil context")
		}
		cancelRuntime()
		waitForContextDone(mergedCtx, t)
	})

	t.Run("Should return nil context when parent and runtime are nil", func(t *testing.T) {
		t.Parallel()

		nilParent := nilContextForMergedRuntimeTest()
		nilRuntime := nilContextForMergedRuntimeTest()
		mergedCtx, cancelMerged := mergedRuntimeContext(nilParent, nilRuntime)
		t.Cleanup(cancelMerged)

		if mergedCtx != nil {
			t.Fatalf("mergedRuntimeContext(nil, nil) = %v, want nil", mergedCtx)
		}
	})
}

func nilContextForMergedRuntimeTest() context.Context {
	return nil
}

func waitForContextDone(ctx context.Context, t *testing.T) {
	t.Helper()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not canceled")
	}
}
