package orchestrator

import (
	"context"
	"encoding/json"
	"testing"

	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/stretchr/testify/require"
)

type stubPromptTemplate struct {
	output string
	err    error
}

func (s stubPromptTemplate) Render(context.Context, PromptDynamicContext) (string, error) {
	return s.output, s.err
}

func TestRefreshUserPrompt(t *testing.T) {
	t.Run("ShouldRespectMemoryInjection", func(t *testing.T) {
		t.Helper()
		loop := &conversationLoop{}
		memoryMsg := llmadapter.Message{Role: roleUser, Content: "memory-transcript"}
		userMsg := llmadapter.Message{
			Role:    roleUser,
			Content: "original",
			Parts:   []llmadapter.ContentPart{llmadapter.TextPart{Text: "attachment"}},
		}
		request := &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{memoryMsg, userMsg},
		}
		loopCtx := &LoopContext{
			LLMRequest:       request,
			baseMessageCount: len(request.Messages),
			PromptTemplate:   stubPromptTemplate{output: "rendered"},
		}
		err := loop.refreshUserPrompt(context.Background(), loopCtx)
		require.NoError(t, err)
		require.Equal(t, "memory-transcript", loopCtx.LLMRequest.Messages[0].Content)
		require.Equal(t, "rendered", loopCtx.LLMRequest.Messages[1].Content)
	})
	t.Run("ShouldHandleMissingUserMessage", func(t *testing.T) {
		t.Helper()
		loop := &conversationLoop{}
		request := &llmadapter.LLMRequest{
			Messages: []llmadapter.Message{
				{Role: roleAssistant, Content: "assistant response"},
			},
		}
		loopCtx := &LoopContext{
			LLMRequest:     request,
			PromptTemplate: stubPromptTemplate{output: "ignored"},
		}
		err := loop.refreshUserPrompt(context.Background(), loopCtx)
		require.NoError(t, err)
	})
}

func TestRestartLoop(t *testing.T) {
	t.Run("ShouldDeepCopyBaseMessages", func(t *testing.T) {
		t.Helper()
		cfg := settings{
			enableProgressTracking: true,
			enableLoopRestarts:     true,
			restartAfterStall:      1,
			maxLoopRestarts:        2,
		}
		loop := &conversationLoop{cfg: cfg}
		rawArgs := json.RawMessage(`{"foo":"bar"}`)
		jsonResult := json.RawMessage(`{"ok":true}`)
		baseMessages := []llmadapter.Message{
			{
				Role: roleAssistant,
				ToolCalls: []llmadapter.ToolCall{
					{ID: "tc-1", Name: "call", Arguments: append(json.RawMessage(nil), rawArgs...)},
				},
			},
			{
				Role: roleTool,
				ToolResults: []llmadapter.ToolResult{
					{
						ID:          "tr-1",
						Name:        "result",
						Content:     "payload",
						JSONContent: append(json.RawMessage(nil), jsonResult...),
					},
				},
				Parts: []llmadapter.ContentPart{
					llmadapter.TextPart{Text: "text"},
					llmadapter.ImageURLPart{URL: "http://example.com"},
					llmadapter.BinaryPart{MIMEType: "application/octet-stream", Data: []byte{0x1, 0x2}},
				},
			},
		}
		originalSnapshot, err := llmadapter.CloneMessages(baseMessages)
		require.NoError(t, err)
		clonedBase, err := llmadapter.CloneMessages(baseMessages)
		require.NoError(t, err)
		request := &llmadapter.LLMRequest{
			Messages: append(
				clonedBase,
				llmadapter.Message{Role: roleAssistant, Content: "runtime-added"},
			),
		}
		before := request.Messages
		loopCtx := &LoopContext{
			LLMRequest:       request,
			baseMessageCount: len(baseMessages),
			State:            newLoopState(&cfg, nil, nil),
		}
		loop.restartLoop(context.Background(), loopCtx, 1)
		require.Len(t, loopCtx.LLMRequest.Messages, len(baseMessages))
		require.Equal(t, originalSnapshot, loopCtx.LLMRequest.Messages)
		loopCtx.LLMRequest.Messages[0].ToolCalls[0].Arguments[2] = 'X'
		require.Equal(t, json.RawMessage(`{"foo":"bar"}`), originalSnapshot[0].ToolCalls[0].Arguments)
		require.Equal(t, json.RawMessage(`{"foo":"bar"}`), before[0].ToolCalls[0].Arguments)
		loopCtx.LLMRequest.Messages[1].ToolResults[0].JSONContent[5] = '0'
		require.Equal(t, jsonResult, originalSnapshot[1].ToolResults[0].JSONContent)
		require.Equal(t, jsonResult, before[1].ToolResults[0].JSONContent)
		binaryPart := loopCtx.LLMRequest.Messages[1].Parts[2].(llmadapter.BinaryPart)
		binaryPart.Data[0] = 0x9
		origBinary := originalSnapshot[1].Parts[2].(llmadapter.BinaryPart)
		require.Equal(t, []byte{0x1, 0x2}, origBinary.Data)
		beforeBinary := before[1].Parts[2].(llmadapter.BinaryPart)
		require.Equal(t, []byte{0x1, 0x2}, beforeBinary.Data)
	})
}
