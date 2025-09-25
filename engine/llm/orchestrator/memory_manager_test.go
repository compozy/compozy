package orchestrator

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	require.Eventually(t, func() bool {
		waitCh := make(chan struct{})
		go func() {
			h.wg.Wait()
			close(waitCh)
		}()
		select {
		case <-waitCh:
			return true
		case <-time.After(2 * time.Second):
			return false
		}
	}, time.Second*3, time.Millisecond*50)
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
		ctx := context.Background()

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
