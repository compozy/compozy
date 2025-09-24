package orchestrator

import (
	"context"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/contracts"
	"github.com/stretchr/testify/assert"
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
		out := mm.Inject(context.Background(), base, nil)
		assert.Equal(t, base, out)
	})
	t.Run("Should inject memory messages before base", func(t *testing.T) {
		mem := &readOnlyMemory{msgs: []contracts.Message{{Role: contracts.MessageRoleUser, Content: "m1"}}}
		ctxData := &MemoryContext{memories: map[string]contracts.Memory{"mem": mem}}
		out := mm.Inject(context.Background(), base, ctxData)
		assert.Equal(t, 2, len(out))
		assert.Equal(t, "user", out[0].Role)
		assert.Equal(t, "m1", out[0].Content)
		assert.Equal(t, "prompt", out[1].Content)
	})
}
