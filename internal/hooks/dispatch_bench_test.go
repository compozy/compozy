package hooks

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"
)

func BenchmarkDispatchInputPreSubmitSync(b *testing.B) {
	hooks := newBenchmarkHooksRuntime(
		b,
		benchmarkSyncInputDecls(12),
		benchmarkSyncInputExecutors(12),
	)

	payload := benchmarkInputPayload()
	ctx := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := hooks.DispatchInputPreSubmit(ctx, payload); err != nil {
			b.Fatalf("DispatchInputPreSubmit() error = %v", err)
		}
	}
}

func BenchmarkSubmitAsyncHookInputPreSubmit(b *testing.B) {
	logger := benchmarkLogger()
	metrics := newHookMetrics()
	tasks := make(chan asyncTask, 1024)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for task := range tasks {
			_ = task
		}
	}()

	pool := &asyncPool{
		logger:  logger,
		metrics: metrics,
		tasks:   tasks,
		started: true,
	}
	runtime := &Hooks{
		logger:  logger,
		metrics: metrics,
		pool:    pool,
	}
	pipe := pipeline[InputPreSubmitPayload, InputPreSubmitPatch]{
		event:        HookInputPreSubmit,
		hooksRuntime: runtime,
		apply:        applyInputPreSubmitPatch,
		encode:       encodeJSON[InputPreSubmitPayload],
		decode:       decodeJSON[InputPreSubmitPatch],
	}
	hook := benchmarkAsyncResolvedHook("bench-async-submit")
	payload := benchmarkInputPayload()
	parent := context.Background()

	b.ReportAllocs()

	for b.Loop() {
		submitAsyncHook(parent, runtime, payload, 0, nil, hook, pipe)
	}
	b.StopTimer()

	close(tasks)
	<-drained

	metrics.mu.Lock()
	dropped := metrics.asyncDropCount
	metrics.mu.Unlock()
	if dropped != 0 {
		b.Fatalf("async submissions dropped = %d, want 0", dropped)
	}
}

func BenchmarkSubprocessProcessEnv(b *testing.B) {
	env := map[string]string{
		"AGH_AGENT":    "codex",
		"AGH_MODEL":    "gpt-5.4",
		"AGH_SESSION":  "session-bench",
		"AGH_WORKROOT": "/tmp/workspace",
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = subprocessProcessEnv(env)
	}
}

func benchmarkLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newBenchmarkHooksRuntime(
	tb testing.TB,
	decls []HookDecl,
	executors map[string]Executor,
	opts ...Option,
) *Hooks {
	tb.Helper()

	resolve := func(decl HookDecl) (Executor, error) {
		executor, ok := executors[decl.Name]
		if !ok {
			return nil, fmt.Errorf("benchmark executor not found for %q", decl.Name)
		}
		return executor, nil
	}

	baseOpts := []Option{
		WithLogger(benchmarkLogger()),
		WithExecutorResolver(resolve),
		WithNativeDeclarations(decls),
	}
	baseOpts = append(baseOpts, opts...)

	hooks := NewHooks(baseOpts...)
	tb.Cleanup(hooks.Close)

	if err := hooks.Rebuild(context.Background()); err != nil {
		tb.Fatalf("Rebuild() error = %v", err)
	}

	return hooks
}

func benchmarkInputPayload() InputPreSubmitPayload {
	return InputPreSubmitPayload{
		PayloadBase: PayloadBase{
			Event:     HookInputPreSubmit,
			Timestamp: time.Unix(1, 0).UTC(),
		},
		SessionContext: SessionContext{
			SessionID:    "session-bench",
			SessionType:  "interactive",
			AgentName:    "codex",
			WorkspaceID:  "workspace-bench",
			Workspace:    "/tmp/workspace-bench",
			CreatedAt:    time.Unix(1, 0).UTC(),
			UpdatedAt:    time.Unix(2, 0).UTC(),
			ACPSessionID: "acp-bench",
			State:        "running",
		},
		TurnContext: TurnContext{TurnID: "turn-bench"},
		InputClass:  "chat",
		Message:     "hello benchmark",
		ContextBlocks: []ContextBlock{
			{
				Kind: "note",
				Text: "context",
				Metadata: map[string]string{
					"scope": "bench",
				},
			},
		},
	}
}

func benchmarkSyncInputDecls(count int) []HookDecl {
	decls := make([]HookDecl, 0, count)
	for i := range count {
		decls = append(decls, HookDecl{
			Name:     fmt.Sprintf("bench-sync-%02d", i),
			Event:    HookInputPreSubmit,
			Mode:     HookModeSync,
			Priority: int32(count - i),
			Matcher: HookMatcher{
				AgentName:   "codex",
				WorkspaceID: "workspace-bench",
				InputClass:  "chat",
			},
		})
	}
	return decls
}

func benchmarkSyncInputExecutors(count int) map[string]Executor {
	executors := make(map[string]Executor, count)
	for i := range count {
		name := fmt.Sprintf("bench-sync-%02d", i)
		executors[name] = NewTypedNativeExecutor(
			func(_ context.Context, _ RegisteredHook, _ InputPreSubmitPayload) (InputPreSubmitPatch, error) {
				return InputPreSubmitPatch{}, nil
			},
		)
	}
	return executors
}

func benchmarkAsyncResolvedHook(name string) *ResolvedHook {
	executor := NewTypedNativeExecutor(
		func(_ context.Context, _ RegisteredHook, _ InputPreSubmitPayload) (InputPreSubmitPatch, error) {
			return InputPreSubmitPatch{}, nil
		},
	)

	return &ResolvedHook{
		RegisteredHook: RegisteredHook{
			Name:     name,
			Event:    HookInputPreSubmit,
			Source:   HookSourceNative,
			Mode:     HookModeAsync,
			Executor: executor,
		},
		Decl: HookDecl{
			Name:         name,
			Event:        HookInputPreSubmit,
			Source:       HookSourceNative,
			Mode:         HookModeAsync,
			ExecutorKind: HookExecutorNative,
		},
	}
}
