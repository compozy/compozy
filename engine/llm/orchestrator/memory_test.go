package orchestrator

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/compozy/compozy/engine/llm/telemetry"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type readOnlyMemory struct{ msgs []contracts.Message }

func (m *readOnlyMemory) Append(context.Context, contracts.Message) error       { return nil }
func (m *readOnlyMemory) AppendMany(context.Context, []contracts.Message) error { return nil }
func (m *readOnlyMemory) Read(context.Context) ([]contracts.Message, error)     { return m.msgs, nil }
func (m *readOnlyMemory) GetID() string                                         { return "mem" }

func TestMemoryManager_Inject(t *testing.T) {
	mm := NewMemoryManager(nil, nil, nil)
	base := []llmadapter.Message{{Role: "user", Content: "prompt"}}
	t.Run("Should return base when no memory", func(t *testing.T) {
		out := mm.Inject(t.Context(), base, nil)
		assert.Equal(t, base, out)
	})
	t.Run("Should inject memory messages before base", func(t *testing.T) {
		mem := &readOnlyMemory{msgs: []contracts.Message{{Role: contracts.MessageRoleUser, Content: "m1"}}}
		ctxData := &MemoryContext{memories: map[string]contracts.Memory{"mem": mem}}
		out := mm.Inject(t.Context(), base, ctxData)
		assert.Equal(t, 2, len(out))
		assert.Equal(t, "user", out[0].Role)
		assert.Equal(t, "m1", out[0].Content)
		assert.Equal(t, "prompt", out[1].Content)
	})
}

type stubMemoryProvider struct {
	memory contracts.Memory
}

func (s *stubMemoryProvider) GetMemory(
	context.Context,
	string,
	string,
) (contracts.Memory, error) {
	return s.memory, nil
}

type capturingMemory struct {
	mu       sync.Mutex
	messages []contracts.Message
	id       string
}

func newCapturingMemory(id string) *capturingMemory {
	return &capturingMemory{id: id}
}

func (m *capturingMemory) Append(_ context.Context, msg contracts.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	return nil
}

func (m *capturingMemory) AppendMany(_ context.Context, msgs []contracts.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msgs...)
	return nil
}

func (m *capturingMemory) Read(context.Context) ([]contracts.Message, error) {
	return nil, nil
}

func (m *capturingMemory) GetID() string { return m.id }

func (m *capturingMemory) snapshot() []contracts.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]contracts.Message, len(m.messages))
	copy(out, m.messages)
	return out
}

type waitingHook struct {
	wg    sync.WaitGroup
	error error
}

func newWaitingHook() *waitingHook {
	h := &waitingHook{}
	h.wg.Add(1)
	return h
}

func (h *waitingHook) OnMemoryStoreComplete(err error) {
	h.error = err
	h.wg.Done()
}

func (h *waitingHook) wait(t *testing.T) {
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for memory store completion")
	}
}

func TestMemoryManager_StoreAsync(t *testing.T) {
	t.Run("Should store assistant and user messages in memory", func(t *testing.T) {
		memory := newCapturingMemory("mem-1")
		provider := &stubMemoryProvider{memory: memory}
		hook := newWaitingHook()

		manager := NewMemoryManager(provider, nil, hook)

		agentCfg := &agent.Config{
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{{ID: "mem-1", Mode: core.MemoryModeReadWrite}},
			},
		}
		req := Request{Agent: agentCfg, Action: &agent.ActionConfig{}}
		ctx := t.Context()

		memoryCtx := manager.Prepare(ctx, req)
		require.NotNil(t, memoryCtx)

		messages := []llmadapter.Message{
			{Role: "user", Content: "hello"},
		}
		response := &llmadapter.LLMResponse{Content: "world"}

		manager.StoreAsync(ctx, memoryCtx, response, messages, req)
		hook.wait(t)
		require.NoError(t, hook.error)

		stored := memory.snapshot()
		require.Len(t, stored, 2)
		assert.Equal(t, contracts.MessageRoleUser, stored[0].Role)
		assert.Equal(t, "hello", stored[0].Content)
		assert.Equal(t, contracts.MessageRoleAssistant, stored[1].Role)
		assert.Equal(t, "world", stored[1].Content)
	})
}

func TestMemoryManager_CompactSummarisesAndStores(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	memory := newCapturingMemory("mem-1")
	provider := &stubMemoryProvider{memory: memory}
	manager := NewMemoryManager(provider, nil, nil)
	agentCfg := &agent.Config{
		ID: "agent",
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{{ID: "mem-1", Mode: core.MemoryModeReadWrite}},
		},
	}
	req := Request{Agent: agentCfg, Action: &agent.ActionConfig{ID: "action"}}
	memCtx := manager.Prepare(ctx, req)
	require.NotNil(t, memCtx)
	state := newLoopState(&settings{enableContextCompaction: true}, memCtx, req.Action)
	loopCtx := &LoopContext{
		Request: req,
		LLMRequest: &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{
				{Role: llmadapter.RoleSystem, Content: "system prompt"},
				{Role: llmadapter.RoleUser, Content: "initial question"},
				{
					Role:    llmadapter.RoleSystem,
					Content: "[memory-compaction] iteration 1 (ctx 75.0%)\n\n- Assistant: prior summary",
				},
				{Role: llmadapter.RoleAssistant, Content: "assistant proposes multi-step plan with long explanation"},
				{
					Role:        llmadapter.RoleTool,
					ToolResults: []llmadapter.ToolResult{{Name: "search", Content: "item1, item2, item3"}},
				},
				{Role: llmadapter.RoleUser, Content: "please continue"},
			},
		},
		State:     state,
		Iteration: 4,
	}
	loopCtx.baseMessageCount = 2
	err := manager.Compact(ctx, loopCtx, telemetry.ContextUsage{PercentOfLimit: 0.92})
	require.NoError(t, err)
	require.Len(t, loopCtx.LLMRequest.Messages, 3)
	summaryMsg := loopCtx.LLMRequest.Messages[2]
	require.Equal(t, llmadapter.RoleSystem, summaryMsg.Role)
	require.True(t, strings.HasPrefix(summaryMsg.Content, compactionSummaryPrefix))
	require.Contains(t, summaryMsg.Content, "prior summary")
	require.Contains(t, summaryMsg.Content, "\n\n-")
	stored := memory.snapshot()
	require.NotEmpty(t, stored)
	require.True(t, strings.HasPrefix(stored[len(stored)-1].Content, compactionSummaryPrefix))
	require.Contains(t, stored[len(stored)-1].Content, "prior summary")
}

func TestMemoryManager_CompactSkippedWhenNoConversationHistory(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	memory := newCapturingMemory("mem-1")
	provider := &stubMemoryProvider{memory: memory}
	manager := NewMemoryManager(provider, nil, nil)
	agentCfg := &agent.Config{
		LLMProperties: agent.LLMProperties{
			Memory: []core.MemoryReference{{ID: "mem-1", Mode: core.MemoryModeReadWrite}},
		},
	}
	req := Request{Agent: agentCfg, Action: &agent.ActionConfig{ID: "action"}}
	memCtx := manager.Prepare(ctx, req)
	require.NotNil(t, memCtx)
	state := newLoopState(&settings{}, memCtx, req.Action)
	loopCtx := &LoopContext{
		Request: req,
		LLMRequest: &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{
				{Role: llmadapter.RoleSystem, Content: "system prompt"},
				{Role: llmadapter.RoleUser, Content: "hello"},
			},
		},
		State:     state,
		Iteration: 1,
	}
	loopCtx.baseMessageCount = 2
	err := manager.Compact(ctx, loopCtx, telemetry.ContextUsage{PercentOfLimit: 0.9})
	require.ErrorIs(t, err, ErrCompactionSkipped)
	require.Len(t, loopCtx.LLMRequest.Messages, 2)
	require.Empty(t, memory.snapshot())
}

func TestMemoryManager_CompactWithoutMemoryContext(t *testing.T) {
	ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
	manager := NewMemoryManager(nil, nil, nil)
	loopCtx := &LoopContext{
		LLMRequest: &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{
				{Role: llmadapter.RoleSystem, Content: "system"},
				{Role: llmadapter.RoleUser, Content: "step one"},
				{Role: llmadapter.RoleAssistant, Content: "assistant provided detailed answer"},
			},
		},
		State:     &loopState{Memory: memoryState{}, runtime: runtimeState{}},
		Iteration: 2,
	}
	loopCtx.baseMessageCount = 1
	err := manager.Compact(ctx, loopCtx, telemetry.ContextUsage{PercentOfLimit: 0.9})
	require.NoError(t, err)
	require.Len(t, loopCtx.LLMRequest.Messages, 2)
	require.Contains(t, loopCtx.LLMRequest.Messages[1].Content, "step one")
}
