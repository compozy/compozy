package loop

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/loop/dsl"
	storepkg "github.com/compozy/agh/internal/store"
)

func TestStoreChannelResultHarvesterShouldReadDurableConversationResults(t *testing.T) {
	t.Parallel()

	t.Run("Should harvest trace completed result after the sent message cursor", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{
			messages: []storepkg.NetworkConversationMessage{
				{
					MessageID: "msg-chat",
					PeerFrom:  "worker",
					Kind:      storepkg.NetworkKindSay,
					Intent:    "discussion",
					Body:      json.RawMessage(`{"text":"still discussing","intent":"discussion"}`),
				},
				{
					MessageID: "msg-result",
					PeerFrom:  "reviewer",
					Kind:      storepkg.NetworkKindTrace,
					WorkID:    "work-1",
					Body: json.RawMessage(
						`{"state":"completed","message":"approved","result":{"decision":"ship"}}`,
					),
				},
			},
		}
		harvester := mustStoreChannelResultHarvester(t, conversations)

		result, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision","work_id":"work-1"}`,
			"50ms",
			"",
			"json_path:decision",
		))
		if err != nil {
			t.Fatalf("HarvestChannelResult() error = %v", err)
		}
		if !result.Found || result.MessageID != "msg-result" || result.Text != "approved" {
			t.Fatalf("HarvestChannelResult() = %#v, want msg-result approved", result)
		}
		var structured map[string]string
		if err := json.Unmarshal(result.Structured, &structured); err != nil {
			t.Fatalf("unmarshal structured result error = %v", err)
		}
		if structured["decision"] != "ship" {
			t.Fatalf("structured decision = %q, want ship", structured["decision"])
		}
		query := conversations.mustSingleQuery(t)
		if query.ref.WorkspaceID != "ws-1" ||
			query.ref.Channel != "builders" ||
			query.ref.Surface != storepkg.NetworkSurfaceThread ||
			query.ref.ThreadID != "thread_decision" {
			t.Fatalf("conversation ref = %#v, want thread decision ref", query.ref)
		}
		if query.query.AfterMessageID != "msg-request" || query.query.WorkID != "work-1" {
			t.Fatalf("query = %#v, want cursor msg-request and work-1", query.query)
		}
	})

	t.Run("Should apply responder and content rules before selecting result", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{
			pages: [][]storepkg.NetworkConversationMessage{
				{
					{
						MessageID: "msg-other",
						PeerFrom:  "other",
						Kind:      storepkg.NetworkKindSay,
						Intent:    "result",
						Text:      "approved by the wrong peer",
						Body:      json.RawMessage(`{"text":"approved by the wrong peer","intent":"result"}`),
					},
					{
						MessageID: "msg-reviewer",
						PeerFrom:  "reviewer",
						Kind:      storepkg.NetworkKindSay,
						Intent:    "result",
						Text:      "approved with caveats",
						Body: json.RawMessage(
							`{"text":"approved with caveats","intent":"result","result":{"decision":"approved"}}`,
						),
					},
				},
			},
		}
		harvester := mustStoreChannelResultHarvester(t, conversations)

		result, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision"}`,
			"50ms",
			"reviewer",
			"contains:approved",
		))
		if err != nil {
			t.Fatalf("HarvestChannelResult() error = %v", err)
		}
		if !result.Found || result.MessageID != "msg-reviewer" {
			t.Fatalf("HarvestChannelResult() = %#v, want reviewer result", result)
		}
	})

	t.Run("Should advance the durable cursor past filtered pages before matching text JSON", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{
			pages: [][]storepkg.NetworkConversationMessage{
				{
					{
						MessageID: "msg-plain",
						PeerFrom:  "reviewer",
						Kind:      storepkg.NetworkKindSay,
						Intent:    "result",
						Text:      "plain result without structured verdict",
						Body: json.RawMessage(
							`{"text":"plain result without structured verdict","intent":"result"}`,
						),
					},
				},
				{
					{
						MessageID: "msg-json",
						PeerFrom:  "reviewer",
						Kind:      storepkg.NetworkKindSay,
						Intent:    "result",
						Text:      `Final verdict: {"decision":"ship","reason":"tests passed"}`,
						Body: json.RawMessage(
							`{"text":"Final verdict: {\"decision\":\"ship\",\"reason\":\"tests passed\"}","intent":"result"}`,
						),
					},
				},
			},
		}
		harvester := mustStoreChannelResultHarvester(t, conversations)

		result, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision"}`,
			"50ms",
			"reviewer",
			"json_path:decision",
		))
		if err != nil {
			t.Fatalf("HarvestChannelResult() error = %v", err)
		}
		if !result.Found || result.MessageID != "msg-json" {
			t.Fatalf("HarvestChannelResult() = %#v, want msg-json", result)
		}
		if got, want := conversations.queries[1].query.AfterMessageID, "msg-plain"; got != want {
			t.Fatalf("second query cursor = %q, want %q", got, want)
		}
	})

	t.Run("Should harvest a direct room result body when no dedicated result field exists", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{
			messages: []storepkg.NetworkConversationMessage{
				{
					MessageID: "msg-direct-result",
					PeerFrom:  "reviewer",
					Kind:      storepkg.NetworkKindSay,
					Intent:    "result",
					Body:      json.RawMessage(`{"text":"approved","intent":"result"}`),
				},
			},
		}
		harvester := mustStoreChannelResultHarvester(t, conversations)

		result, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"direct",`+
				`"direct_id":"direct_0123456789abcdef0123456789abcdef"}`,
			"50ms",
			"",
			"json",
		))
		if err != nil {
			t.Fatalf("HarvestChannelResult() error = %v", err)
		}
		if !result.Found || result.MessageID != "msg-direct-result" {
			t.Fatalf("HarvestChannelResult() = %#v, want direct result", result)
		}
		query := conversations.mustSingleQuery(t)
		if query.ref.Surface != storepkg.NetworkSurfaceDirect ||
			query.ref.DirectID != "direct_0123456789abcdef0123456789abcdef" {
			t.Fatalf("conversation ref = %#v, want direct room ref", query.ref)
		}
	})

	t.Run("Should return not found when the result is silent through the window", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{}
		harvester := mustStoreChannelResultHarvester(t, conversations)
		harvester.pollInterval = time.Millisecond

		result, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision"}`,
			"1ms",
			"",
			"",
		))
		if err != nil {
			t.Fatalf("HarvestChannelResult() error = %v", err)
		}
		if result.Found {
			t.Fatalf("HarvestChannelResult().Found = true, want false")
		}
		if conversations.queryCount() == 0 {
			t.Fatal("ListConversationMessages calls = 0, want at least one durable store read")
		}
	})

	t.Run("Should reject missing durable conversation metadata", func(t *testing.T) {
		t.Parallel()

		harvester := mustStoreChannelResultHarvester(t, &fakeChannelResultConversationStore{})
		_, err := harvester.HarvestChannelResult(context.Background(), &ChannelResultHarvestRequest{
			Window: "50ms",
			Raw: ActionRawResult{
				Structured: json.RawMessage(`{"message_id":"msg-request"}`),
			},
		})
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("HarvestChannelResult() error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject unsupported content rules before matching a result", func(t *testing.T) {
		t.Parallel()

		conversations := &fakeChannelResultConversationStore{
			messages: []storepkg.NetworkConversationMessage{
				{
					MessageID: "msg-result",
					PeerFrom:  "reviewer",
					Kind:      storepkg.NetworkKindSay,
					Intent:    "result",
					Body:      json.RawMessage(`{"intent":"result","result":{"decision":"ship"}}`),
				},
			},
		}
		harvester := mustStoreChannelResultHarvester(t, conversations)

		_, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision"}`,
			"50ms",
			"",
			"jsonpath:decision",
		))
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("HarvestChannelResult() invalid content_rule error = %v, want ErrValidation", err)
		}
	})

	t.Run("Should reject nil store and invalid windows", func(t *testing.T) {
		t.Parallel()

		if _, err := NewStoreChannelResultHarvester(nil); !errors.Is(err, ErrActionDependencyMissing) {
			t.Fatalf("NewStoreChannelResultHarvester(nil) error = %v, want ErrActionDependencyMissing", err)
		}
		harvester := mustStoreChannelResultHarvester(t, &fakeChannelResultConversationStore{})
		_, err := harvester.HarvestChannelResult(context.Background(), channelResultHarvestRequestForTest(
			`{"message_id":"msg-request"}`,
			`{"workspace_id":"ws-1","channel":"builders","surface":"thread","thread_id":"thread_decision"}`,
			"0s",
			"",
			"",
		))
		if !errors.Is(err, ErrValidation) {
			t.Fatalf("HarvestChannelResult() invalid window error = %v, want ErrValidation", err)
		}
	})
}

func mustStoreChannelResultHarvester(
	t *testing.T,
	conversations ChannelResultConversationStore,
) *StoreChannelResultHarvester {
	t.Helper()

	harvester, err := NewStoreChannelResultHarvester(conversations)
	if err != nil {
		t.Fatalf("NewStoreChannelResultHarvester() error = %v", err)
	}
	return harvester
}

func channelResultHarvestRequestForTest(
	structured string,
	params string,
	window string,
	responder string,
	contentRule string,
) *ChannelResultHarvestRequest {
	return &ChannelResultHarvestRequest{
		Window:      window,
		Responder:   responder,
		ContentRule: contentRule,
		Raw: ActionRawResult{
			Structured:     json.RawMessage(structured),
			RenderedParams: json.RawMessage(params),
		},
		Node: dsl.Node{ID: "ask", Class: dsl.NodeClassAction, Kind: "agh__network_send"},
	}
}

type fakeChannelResultConversationStore struct {
	pages    [][]storepkg.NetworkConversationMessage
	messages []storepkg.NetworkConversationMessage
	queries  []fakeChannelResultQuery
}

type fakeChannelResultQuery struct {
	ref   storepkg.NetworkConversationRef
	query storepkg.NetworkConversationMessageQuery
}

func (s *fakeChannelResultConversationStore) ListConversationMessages(
	_ context.Context,
	ref storepkg.NetworkConversationRef,
	query storepkg.NetworkConversationMessageQuery,
) ([]storepkg.NetworkConversationMessage, error) {
	s.queries = append(s.queries, fakeChannelResultQuery{ref: ref, query: query})
	if len(s.pages) > 0 {
		page := s.pages[0]
		s.pages = s.pages[1:]
		return append([]storepkg.NetworkConversationMessage(nil), page...), nil
	}
	return append([]storepkg.NetworkConversationMessage(nil), s.messages...), nil
}

func (s *fakeChannelResultConversationStore) mustSingleQuery(t *testing.T) fakeChannelResultQuery {
	t.Helper()

	if len(s.queries) != 1 {
		t.Fatalf("ListConversationMessages calls = %d, want 1", len(s.queries))
	}
	return s.queries[0]
}

func (s *fakeChannelResultConversationStore) queryCount() int {
	return len(s.queries)
}
