//go:build integration

package loop_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	storepkg "github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/tools"
)

func TestActionChannelResultIntegrationShouldHarvestResultAndStallWithFakeConversationStore(t *testing.T) {
	t.Parallel()

	t.Run("Should execute network send and harvest durable result independent of templates", func(t *testing.T) {
		t.Parallel()

		conversations := &integrationChannelResultStore{
			messages: []storepkg.NetworkConversationMessage{
				{
					MessageID: "msg-result",
					PeerFrom:  "reviewer",
					Kind:      storepkg.NetworkKindSay,
					Intent:    "result",
					Text:      "approved",
					Body:      json.RawMessage(`{"text":"approved","intent":"result","result":{"decision":"ship"}}`),
				},
			},
		}
		executor := resolveNetworkSendExecutorForIntegration(t, conversations)
		node := networkSendChannelResultNodeForIntegration("50ms")

		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			ToolScope:   tools.Scope{WorkspaceID: "ws-1", SessionID: "sess-1", AgentName: "agent-a"},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		output, err := executor.Harvest(context.Background(), raw, node)
		if err != nil {
			t.Fatalf("Harvest() error = %v", err)
		}
		got, ok := output.Value.(map[string]any)
		if !ok || got["decision"] != "ship" {
			t.Fatalf("Harvest().Value = %#v, want decision ship", output.Value)
		}
		query := conversations.mustSingleQuery(t)
		if query.AfterMessageID != "msg-request" {
			t.Fatalf("AfterMessageID = %q, want msg-request", query.AfterMessageID)
		}
	})

	t.Run("Should route silence within the window to stalled", func(t *testing.T) {
		t.Parallel()

		executor := resolveNetworkSendExecutorForIntegration(t, &integrationChannelResultStore{})
		node := networkSendChannelResultNodeForIntegration("1ms")
		raw, err := executor.Execute(context.Background(), node, loop.ActionExecutionInput{
			WorkspaceID: "ws-1",
			ToolScope:   tools.Scope{WorkspaceID: "ws-1", SessionID: "sess-1", AgentName: "agent-a"},
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		_, err = executor.Harvest(context.Background(), raw, node)
		if !errors.Is(err, loop.ErrActionStalled) {
			t.Fatalf("Harvest() error = %v, want ErrActionStalled", err)
		}
	})
}

func resolveNetworkSendExecutorForIntegration(
	t *testing.T,
	conversations loop.ChannelResultConversationStore,
) loop.ActionExecutor {
	t.Helper()

	registry := &fakeActionToolRegistry{
		views: map[tools.ToolID]tools.ToolView{tools.ToolIDNetworkSend: {}},
		callResult: tools.ToolResult{
			Structured: json.RawMessage(`{"message_id":"msg-request"}`),
		},
	}
	actions := newActionRegistryForTest(t, registry, loop.WithActionChannelResultStore(conversations))
	executor, err := actions.Resolve(
		context.Background(),
		tools.Scope{WorkspaceID: "ws-1"},
		tools.ToolIDNetworkSend.String(),
	)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	return executor
}

func networkSendChannelResultNodeForIntegration(window string) dsl.Node {
	return dsl.Node{
		ID:    "ask",
		Class: dsl.NodeClassAction,
		Kind:  tools.ToolIDNetworkSend.String(),
		Params: dsl.NodeParams{
			"workspace_id": "ws-1",
			"session_id":   "sess-1",
			"channel":      "builders",
			"surface":      storepkg.NetworkSurfaceThread,
			"thread_id":    "thread_decision",
			"kind":         storepkg.NetworkKindSay,
			"body": map[string]any{
				"text":   "please decide",
				"intent": "request",
			},
		},
		Harvest: &dsl.HarvestSpec{Kind: "channel_result", Window: window},
	}
}

type integrationChannelResultStore struct {
	messages []storepkg.NetworkConversationMessage
	queries  []storepkg.NetworkConversationMessageQuery
}

func (s *integrationChannelResultStore) ListConversationMessages(
	_ context.Context,
	_ storepkg.NetworkConversationRef,
	query storepkg.NetworkConversationMessageQuery,
) ([]storepkg.NetworkConversationMessage, error) {
	s.queries = append(s.queries, query)
	return append([]storepkg.NetworkConversationMessage(nil), s.messages...), nil
}

func (s *integrationChannelResultStore) mustSingleQuery(t *testing.T) storepkg.NetworkConversationMessageQuery {
	t.Helper()

	if len(s.queries) != 1 {
		t.Fatalf("ListConversationMessages calls = %d, want 1", len(s.queries))
	}
	return s.queries[0]
}
