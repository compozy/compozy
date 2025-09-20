package memory

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/memory/testutil"
	"github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const integrationTimeout = 5 * time.Second

func integrationContext(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	ctx = logger.ContextWithLogger(ctx, logger.NewForTests())
	ctx = config.ContextWithManager(ctx, config.NewManager(config.NewService()))
	return ctx, cancel
}

func TestMemoryResolverIntegration(t *testing.T) {
	t.Run("Should resolve memory using Redis manager", func(t *testing.T) {
		ctx, cancel := integrationContext(t)
		defer cancel()
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()
		_ = setup.CreateTestMemoryInstance(t, "session-memory")
		resolver := uc.NewMemoryResolver(
			setup.Manager,
			tplengine.NewEngine(tplengine.FormatText),
			map[string]any{"session": "abc"},
		)
		memory, err := resolver.GetMemory(ctx, "session-memory", "session-{{ .session }}")
		require.NoError(t, err)
		require.NotNil(t, memory)
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "hello"}
		require.NoError(t, memory.Append(ctx, msg))
		messages, err := memory.Read(ctx)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, msg.Content, messages[0].Content)
		assert.NotEmpty(t, memory.GetID())
	})

	t.Run("Should resolve agent memories with distinct Redis keys", func(t *testing.T) {
		ctx, cancel := integrationContext(t)
		defer cancel()
		setup := testutil.SetupTestRedis(t)
		defer setup.Cleanup()
		_ = setup.CreateTestMemoryInstance(t, "user-memory")
		resolver := uc.NewMemoryResolver(
			setup.Manager,
			tplengine.NewEngine(tplengine.FormatText),
			map[string]any{"user": "42"},
		)
		agentCfg := &agent.Config{
			ID: "agent-1",
			LLMProperties: agent.LLMProperties{
				Memory: []core.MemoryReference{{ID: "user-memory", Key: "user-{{ .user }}"}},
			},
		}
		memories, err := resolver.ResolveAgentMemories(ctx, agentCfg)
		require.NoError(t, err)
		require.NotNil(t, memories)
		require.Contains(t, memories, "user-memory")
		mem := memories["user-memory"]
		msg := llm.Message{Role: llm.MessageRoleUser, Content: "hi"}
		require.NoError(t, mem.Append(ctx, msg))
		messages, err := mem.Read(ctx)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		assert.Equal(t, "hi", messages[0].Content)
	})
}
