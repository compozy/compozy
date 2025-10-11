package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/require"
)

type staticInvoker struct {
	resp *llmadapter.LLMResponse
	err  error
}

func (s *staticInvoker) Invoke(
	context.Context,
	llmadapter.LLMClient,
	*llmadapter.LLMRequest,
	Request,
) (*llmadapter.LLMResponse, error) {
	return s.resp, s.err
}

type noOpExec struct {
	results     []llmadapter.ToolResult
	err         error
	execCalls   int
	budgetCalls int
}

func (n *noOpExec) Execute(context.Context, []llmadapter.ToolCall) ([]llmadapter.ToolResult, error) {
	n.execCalls++
	return n.results, n.err
}
func (n *noOpExec) UpdateBudgets(context.Context, []llmadapter.ToolResult, *loopState) error {
	n.budgetCalls++
	return nil
}

type passHandler struct{}

func (passHandler) HandleNoToolCalls(
	context.Context,
	*llmadapter.LLMResponse,
	Request,
	*llmadapter.LLMRequest,
	*loopState,
) (*core.Output, bool, error) {
	return nil, false, nil
}

type recordingMemoryManager struct {
	storeCalls   int
	compactCalls int
}

type failingMemoryManager struct {
	*recordingMemoryManager
	err error
}

func newFailingMemoryManager(err error) *failingMemoryManager {
	return &failingMemoryManager{
		recordingMemoryManager: &recordingMemoryManager{},
		err:                    err,
	}
}

//nolint:gocritic // Test double mirrors production interface which consumes Request by value.
func (r *recordingMemoryManager) Prepare(_ context.Context, _ Request) *MemoryContext {
	return &MemoryContext{}
}

func (r *recordingMemoryManager) Inject(
	_ context.Context,
	base []llmadapter.Message,
	_ *MemoryContext,
) []llmadapter.Message {
	return base
}

//nolint:gocritic // Test double mirrors production interface which consumes Request by value.
func (r *recordingMemoryManager) StoreAsync(
	_ context.Context,
	_ *MemoryContext,
	_ *llmadapter.LLMResponse,
	_ []llmadapter.Message,
	_ Request,
) {
	r.storeCalls++
}

//nolint:gocritic // Test double mirrors production interface which consumes Request by value.
func (r *recordingMemoryManager) StoreFailureEpisode(
	_ context.Context,
	_ *MemoryContext,
	_ Request,
	_ FailureEpisode,
) {
}

func (r *recordingMemoryManager) Compact(
	_ context.Context,
	_ *LoopContext,
	_ telemetry.ContextUsage,
) error {
	r.compactCalls++
	return nil
}

func (f *failingMemoryManager) Compact(
	_ context.Context,
	_ *LoopContext,
	_ telemetry.ContextUsage,
) error {
	f.compactCalls++
	if f.err != nil {
		return f.err
	}
	return ErrCompactionIncomplete
}

func TestConversationLoop_NoProgressDetection(t *testing.T) {
	cfg := &settings{maxToolIterations: 5, enableProgressTracking: true, noProgressThreshold: 2}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "t"}}}}
	exec := &noOpExec{results: []llmadapter.ToolResult{{ID: "1", Name: "t", Content: `{"ok":true}`}}}
	h := passHandler{}
	memory := &recordingMemoryManager{}
	loop := newConversationLoop(cfg, exec, h, inv, memory)
	llmReq := &llmadapter.LLMRequest{}
	state := newLoopState(cfg, &MemoryContext{}, &agent.ActionConfig{ID: "a"})
	req := Request{Agent: &agent.Config{ID: "ag"}, Action: &agent.ActionConfig{ID: "a"}}
	_, _, err := loop.Run(context.Background(), nil, llmReq, req, state, nil)
	require.Error(t, err)
	require.Equal(t, 3, exec.execCalls)
	require.Equal(t, 3, exec.budgetCalls)
	require.Equal(t, 0, memory.storeCalls)
}

func TestConversationLoop_RestartOnNoProgress(t *testing.T) {
	cfg := &settings{
		maxToolIterations:      6,
		enableProgressTracking: true,
		noProgressThreshold:    3,
		enableLoopRestarts:     true,
		restartAfterStall:      1,
		maxLoopRestarts:        1,
	}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{ToolCalls: []llmadapter.ToolCall{{ID: "1", Name: "t"}}}}
	exec := &noOpExec{results: []llmadapter.ToolResult{{ID: "1", Name: "t", Content: `{"ok":true}`}}}
	memory := &recordingMemoryManager{}
	loop := newConversationLoop(cfg, exec, passHandler{}, inv, memory)
	llmReq := &llmadapter.LLMRequest{}
	state := newLoopState(cfg, &MemoryContext{}, &agent.ActionConfig{ID: "action"})
	req := Request{Agent: &agent.Config{ID: "agent"}, Action: &agent.ActionConfig{ID: "action"}}
	_, _, err := loop.Run(context.Background(), nil, llmReq, req, state, nil)
	require.Error(t, err)
	require.Equal(t, 1, state.Iteration.Restarts)
	require.GreaterOrEqual(t, exec.execCalls, 4)
}

func TestConversationLoop_CompactionTrigger(t *testing.T) {
	tempDir := t.TempDir()
	recorder, err := telemetry.NewRecorder(&telemetry.Options{
		ProjectRoot:              tempDir,
		ContextWarningThresholds: []float64{0.5},
	})
	require.NoError(t, err)
	ctx, run, err := recorder.StartRun(context.Background(), telemetry.RunMetadata{})
	require.NoError(t, err)
	defer func() {
		closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{})
		require.NoError(t, closeErr)
	}()

	cfg := &settings{
		maxToolIterations:   2,
		compactionThreshold: 0.5,
		compactionCooldown:  1,
	}
	memory := &recordingMemoryManager{}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{
		Content: "done",
		Usage:   &llmadapter.Usage{TotalTokens: 90, PromptTokens: 70, CompletionTokens: 20},
	}}
	exec := &noOpExec{}
	loop := newConversationLoop(cfg, exec, passHandler{}, inv, memory)
	llmReq := &llmadapter.LLMRequest{
		Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hi"}},
		Options:  llmadapter.CallOptions{MaxTokens: 100},
	}
	state := newLoopState(cfg, &MemoryContext{}, &agent.ActionConfig{ID: "action"})
	req := Request{Agent: &agent.Config{ID: "agent"}, Action: &agent.ActionConfig{ID: "action"}}
	output, _, err := loop.Run(ctx, nil, llmReq, req, state, nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, 1, memory.compactCalls)
}

func TestConversationLoop_CompactionFailureRespectsCooldown(t *testing.T) {
	ctx := logger.ContextWithLogger(context.Background(), logger.NewForTests())
	memMgr := newFailingMemoryManager(ErrCompactionIncomplete)
	loop := &conversationLoop{
		cfg: settings{
			compactionThreshold: 0.5,
			compactionCooldown:  2,
		},
		memory: memMgr,
	}
	state := newLoopState(&settings{}, &MemoryContext{}, &agent.ActionConfig{ID: "action"})
	state.markCompaction(0.5, 0.75)
	loopCtx := &LoopContext{
		State:     state,
		Iteration: 3,
	}
	loop.tryCompactMemory(ctx, loopCtx, telemetry.ContextUsage{PercentOfLimit: 0.8}, 0.5)
	require.Equal(t, 1, state.Memory.CompactionFailures)
	require.Equal(t, 3, state.Memory.LastCompactionIteration)
	require.True(t, state.Memory.CompactionSuggested)
	require.False(t, state.compactionPending(4, 2))
	require.True(t, state.compactionPending(5, 2))
}

func TestConversationLoop_FinalizeStoresMemory(t *testing.T) {
	memory := &recordingMemoryManager{}
	inv := &staticInvoker{resp: &llmadapter.LLMResponse{Content: "done"}}
	exec := &noOpExec{}
	h := passHandler{}
	loop := newConversationLoop(&settings{maxToolIterations: 1}, exec, h, inv, memory)
	llmReq := &llmadapter.LLMRequest{Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hi"}}}
	memCtx := &MemoryContext{}
	state := newLoopState(&settings{}, memCtx, &agent.ActionConfig{ID: "action"})
	req := Request{Agent: &agent.Config{ID: "ag"}, Action: &agent.ActionConfig{ID: "action"}}
	output, _, err := loop.Run(context.Background(), nil, llmReq, req, state, nil)
	require.NoError(t, err)
	require.NotNil(t, output)
	require.Equal(t, 1, memory.storeCalls)
}

func TestConversationLoop_RestartEmitsTelemetry(t *testing.T) {
	tempDir := t.TempDir()
	recorder, err := telemetry.NewRecorder(&telemetry.Options{ProjectRoot: tempDir})
	require.NoError(t, err)
	ctx, run, err := recorder.StartRun(context.Background(), telemetry.RunMetadata{})
	require.NoError(t, err)

	loop := &conversationLoop{cfg: settings{maxLoopRestarts: 2}}
	loopCtx := &LoopContext{
		State:     newLoopState(&loop.cfg, &MemoryContext{}, &agent.ActionConfig{ID: "action"}),
		Iteration: 3,
		LLMRequest: &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{{Role: llmadapter.RoleUser, Content: "hi"}},
		},
		BaseSystemPrompt: "system",
		baseMessageCount: 1,
	}
	loop.restartLoop(ctx, loopCtx, 3)

	closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{Success: true})
	require.NoError(t, closeErr)

	events := readRunEvents(t, tempDir)
	evt, ok := findEventByStage(events, "loop_restart")
	require.True(t, ok)
	metadata, ok := evt["metadata"].(map[string]any)
	require.True(t, ok)
	restartIndex, ok := metadata["restart_index"].(float64)
	require.True(t, ok)
	require.Equal(t, 1, int(restartIndex))
	stallIterations, ok := metadata["stall_iterations"].(float64)
	require.True(t, ok)
	require.Equal(t, 3, int(stallIterations))
}

func TestConversationLoop_CompactionCooldownTelemetry(t *testing.T) {
	tempDir := t.TempDir()
	recorder, err := telemetry.NewRecorder(&telemetry.Options{ProjectRoot: tempDir})
	require.NoError(t, err)
	ctx, run, err := recorder.StartRun(context.Background(), telemetry.RunMetadata{})
	require.NoError(t, err)

	loop := &conversationLoop{cfg: settings{compactionThreshold: 0.5, compactionCooldown: 3}}
	loop.memory = &recordingMemoryManager{}
	state := newLoopState(&loop.cfg, &MemoryContext{}, &agent.ActionConfig{ID: "action"})
	state.markCompaction(0.5, 0.7)
	state.Memory.LastCompactionIteration = 2
	loopCtx := &LoopContext{
		State:     state,
		Iteration: 3,
	}
	loop.tryCompactMemory(ctx, loopCtx, telemetry.ContextUsage{PercentOfLimit: 0.7}, 0.5)

	closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{Success: true})
	require.NoError(t, closeErr)

	events := readRunEvents(t, tempDir)
	evt, ok := findEventByStage(events, "compaction_cooldown")
	require.True(t, ok)
	metadata, ok := evt["metadata"].(map[string]any)
	require.True(t, ok)
	remaining, ok := metadata["iterations_until_ready"].(float64)
	require.True(t, ok)
	require.Equal(t, 2, int(remaining))
}

func TestConversationLoop_RecordLLMResponseAddsTokenMetadata(t *testing.T) {
	tempDir := t.TempDir()
	recorder, err := telemetry.NewRecorder(
		&telemetry.Options{ProjectRoot: tempDir, CaptureContent: false, RedactContent: true},
	)
	require.NoError(t, err)
	ctx, run, err := recorder.StartRun(context.Background(), telemetry.RunMetadata{})
	require.NoError(t, err)

	loop := &conversationLoop{}
	loopCtx := &LoopContext{
		Iteration: 1,
		Request:   Request{},
		LLMRequest: &llmadapter.LLMRequest{
			Options: llmadapter.CallOptions{MaxTokens: 200},
		},
	}
	resp := &llmadapter.LLMResponse{
		Content: "sensitive",
		Usage:   &llmadapter.Usage{PromptTokens: 100, CompletionTokens: 40, TotalTokens: 140},
	}
	loop.recordLLMResponse(ctx, loopCtx, resp)

	closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{Success: true})
	require.NoError(t, closeErr)

	events := readRunEvents(t, tempDir)
	evt, ok := findEventByStage(events, "llm_response")
	require.True(t, ok)
	metadata, ok := evt["metadata"].(map[string]any)
	require.True(t, ok)
	promptTokens, ok := metadata["prompt_tokens"].(float64)
	require.True(t, ok)
	require.Equal(t, 100, int(promptTokens))
	totalTokens, ok := metadata["total_tokens"].(float64)
	require.True(t, ok)
	require.Equal(t, 140, int(totalTokens))
	payload, ok := evt["payload"].(map[string]any)
	require.True(t, ok)
	rawContent, ok := payload["raw_content"].(string)
	require.True(t, ok)
	require.Equal(t, telemetry.RedactedValue, rawContent)
}
