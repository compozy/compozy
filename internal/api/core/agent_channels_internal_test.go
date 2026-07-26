package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/gin-gonic/gin"
)

func TestAgentChannelCoreHandlersUseIdentityAndCoordinationMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should use caller identity and coordination metadata for channel operations", func(t *testing.T) {
		t.Parallel()

		var sent []network.SendRequest
		source := agentCoreEnvelope(t, "msg-source", "builders", contract.CoordinationMessageRequest)
		source.Channel = " builders "
		source.From = " reviewer.sess-peer "
		peerSessionID := "sess-peer"
		networkService := &agentCoreNetworkService{
			ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
				if workspaceID != "ws-1" || channel != "builders" {
					t.Fatalf("ListPeers() = %q/%q, want ws-1/builders", workspaceID, channel)
				}
				return []network.PeerInfo{{
					SessionID:   &peerSessionID,
					PeerID:      "reviewer.sess-peer",
					WorkspaceID: "ws-1",
					Channel:     "builders",
					Local:       true,
				}}, nil
			},
			ListChannelsFn: func(_ context.Context, workspaceID string) ([]network.ChannelInfo, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ListChannels() workspaceID = %q, want ws-1", workspaceID)
				}
				return []network.ChannelInfo{{Channel: "builders", PeerCount: 2}}, nil
			},
			SendFn: func(_ context.Context, request network.SendRequest) (string, error) {
				sent = append(sent, request)
				if len(sent) == 1 {
					return "msg-send", nil
				}
				return "msg-reply", nil
			},
			WaitInboxFn: func(_ context.Context, sessionID string, channel string) ([]network.Envelope, error) {
				if sessionID != "sess-agent" || channel != "builders" {
					t.Fatalf("WaitInbox() = %q/%q, want caller session builders", sessionID, channel)
				}
				return []network.Envelope{
					agentCoreEnvelope(t, "msg-other", "other", contract.CoordinationMessageStatus),
					agentCoreEnvelope(t, "msg-builders", "builders", contract.CoordinationMessageResult),
				}, nil
			},
			InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
				if sessionID != "sess-agent" {
					t.Fatalf("Inbox() sessionID = %q, want sess-agent", sessionID)
				}
				return []network.Envelope{source}, nil
			},
		}
		engine := newAgentCoreTestRouter(t, networkService)

		contextResp := performAgentCoreRequest(t, engine, http.MethodGet, "/agent/context", nil, agentCoreHeaders())
		if contextResp.Code != http.StatusOK {
			t.Fatalf(
				"context status = %d, want %d; body=%s",
				contextResp.Code,
				http.StatusOK,
				contextResp.Body.String(),
			)
		}
		var contextPayload contract.AgentContextResponse
		decodeAgentCoreResponse(t, contextResp, &contextPayload)
		if contextPayload.Context.Self.SessionID != "sess-agent" ||
			contextPayload.Context.Workspace.ID != "ws-1" {
			t.Fatalf("context = %#v, want caller identity", contextPayload.Context)
		}
		if contextPayload.Context.Session.Lineage == nil ||
			contextPayload.Context.Session.Lineage.ParentSessionID != "sess-root" ||
			contextPayload.Context.Session.Lineage.RootSessionID != "sess-root" ||
			contextPayload.Context.Session.Lineage.SpawnDepth != 1 {
			t.Fatalf(
				"context session lineage = %#v, want propagated caller lineage",
				contextPayload.Context.Session.Lineage,
			)
		}

		channelsResp := performAgentCoreRequest(t, engine, http.MethodGet, "/agent/channels", nil, agentCoreHeaders())
		if channelsResp.Code != http.StatusOK {
			t.Fatalf(
				"channels status = %d, want %d; body=%s",
				channelsResp.Code,
				http.StatusOK,
				channelsResp.Body.String(),
			)
		}
		var channels contract.AgentChannelsResponse
		decodeAgentCoreResponse(t, channelsResp, &channels)
		if len(channels.Channels) != 1 ||
			channels.Channels[0].ID != "builders" ||
			len(channels.Channels[0].AllowedMessageKinds) != len(contract.CoordinationMessageKinds()) {
			t.Fatalf("channels = %#v, want builders with MVP kinds", channels.Channels)
		}

		sendResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/builders/send",
			[]byte(
				`{"body":{"text":"status"},"metadata":{"task_id":"task-1","run_id":"run-1","workflow_id":"wf-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
			),
			agentCoreHeaders(),
		)
		if sendResp.Code != http.StatusAccepted {
			t.Fatalf("send status = %d, want %d; body=%s", sendResp.Code, http.StatusAccepted, sendResp.Body.String())
		}
		if len(sent) != 1 ||
			sent[0].SessionID != "sess-agent" ||
			sent[0].Channel != "builders" ||
			sent[0].Kind != network.KindSay {
			t.Fatalf("sent requests = %#v, want caller-bound say", sent)
		}
		if metadata := decodeAgentCoreCoordinationExt(
			t,
			sent[0].Ext,
		); metadata.MessageKind != contract.CoordinationMessageStatus ||
			metadata.WorkflowID != "wf-1" {
			t.Fatalf("send metadata = %#v, want status wf-1", metadata)
		}

		recvResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodGet,
			"/agent/channels/builders/recv?wait=true&limit=1",
			nil,
			agentCoreHeaders(),
		)
		if recvResp.Code != http.StatusOK {
			t.Fatalf("recv status = %d, want %d; body=%s", recvResp.Code, http.StatusOK, recvResp.Body.String())
		}
		var messages contract.AgentChannelMessagesResponse
		decodeAgentCoreResponse(t, recvResp, &messages)
		if len(messages.Messages) != 1 ||
			messages.Messages[0].MessageID != "msg-builders" ||
			messages.Messages[0].Metadata.MessageKind != contract.CoordinationMessageResult {
			t.Fatalf("messages = %#v, want builders result message", messages.Messages)
		}

		queuedResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodGet,
			"/agent/channels/builders/recv",
			nil,
			agentCoreHeaders(),
		)
		if queuedResp.Code != http.StatusOK {
			t.Fatalf(
				"queued recv status = %d, want %d; body=%s",
				queuedResp.Code,
				http.StatusOK,
				queuedResp.Body.String(),
			)
		}
		var queuedMessages contract.AgentChannelMessagesResponse
		decodeAgentCoreResponse(t, queuedResp, &queuedMessages)
		if len(queuedMessages.Messages) != 1 || queuedMessages.Messages[0].MessageID != "msg-source" {
			t.Fatalf("queued messages = %#v, want source inbox message", queuedMessages.Messages)
		}
		replyResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/reply",
			[]byte(
				`{"reply_to_message_id":"msg-source","body":{"text":"ack"},"metadata":{"task_id":"","run_id":"","channel_id":"","message_kind":"","correlation_id":""}}`,
			),
			agentCoreHeaders(),
		)
		if replyResp.Code != http.StatusAccepted {
			t.Fatalf(
				"reply status = %d, want %d; body=%s",
				replyResp.Code,
				http.StatusAccepted,
				replyResp.Body.String(),
			)
		}
		if len(sent) != 2 ||
			sent[1].SessionID != "sess-agent" ||
			sent[1].Kind != network.KindSay ||
			sent[1].Surface == nil ||
			*sent[1].Surface != network.SurfaceDirect ||
			sent[1].Channel != "builders" ||
			sent[1].DirectID != nil ||
			sent[1].To == nil ||
			*sent[1].To != "reviewer.sess-peer" ||
			sent[1].ReplyTo == nil ||
			*sent[1].ReplyTo != "msg-source" {
			t.Fatalf("reply request = %#v, want direct reply to source", sent)
		}
		if metadata := decodeAgentCoreCoordinationExt(
			t,
			sent[1].Ext,
		); metadata.MessageKind != contract.CoordinationMessageReply ||
			metadata.TaskID != "task-1" ||
			metadata.RunID != "run-1" {
			t.Fatalf("reply metadata = %#v, want inherited reply metadata", metadata)
		}

		badReplyKind := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/reply",
			[]byte(
				`{"reply_to_message_id":"msg-source","body":{"text":"ack"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
			),
			agentCoreHeaders(),
		)
		if badReplyKind.Code != http.StatusBadRequest {
			t.Fatalf(
				"bad reply kind status = %d, want %d; body=%s",
				badReplyKind.Code,
				http.StatusBadRequest,
				badReplyKind.Body.String(),
			)
		}
		if len(sent) != 2 {
			t.Fatalf("sent request count after bad reply kind = %d, want unchanged 2", len(sent))
		}
	})
}

func TestAgentChannelCoreHandlersRejectCallerOwnedAndLegacyFields(t *testing.T) {
	t.Parallel()

	validMetadata := `{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}`
	tests := []struct {
		name        string
		path        string
		body        string
		wantMessage string
	}{
		{
			name:        "Should reject caller supplied from on send",
			path:        "/agent/channels/builders/send",
			body:        `{"body":{"text":"status"},"metadata":` + validMetadata + `,"from":"spoofed-peer"}`,
			wantMessage: "sender identity and proof are daemon-derived",
		},
		{
			name:        "Should reject caller supplied proof on send",
			path:        "/agent/channels/builders/send",
			body:        `{"body":{"text":"status"},"metadata":` + validMetadata + `,"proof":"spoofed-proof"}`,
			wantMessage: "sender identity and proof are daemon-derived",
		},
		{
			name:        "Should reject removed coordination channel on send",
			path:        "/agent/channels/builders/send",
			body:        `{"body":{"text":"status"},"metadata":` + validMetadata + `,"coordination_channel_id":"legacy"}`,
			wantMessage: "coordination_channel_id",
		},
		{
			name: "Should reject removed nested coordination channel on send",
			path: "/agent/channels/builders/send",
			body: `{"body":{"text":"status"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders",` +
				`"coordination_channel_id":"legacy","message_kind":"status","correlation_id":"run-1"}}`,
			wantMessage: "coordination_channel_id",
		},
		{
			name:        "Should reject caller supplied from on reply",
			path:        "/agent/channels/reply",
			body:        `{"reply_to_message_id":"msg-source","body":{"text":"ack"},"from":"spoofed-peer"}`,
			wantMessage: "sender identity and proof are daemon-derived",
		},
		{
			name:        "Should reject caller supplied proof on reply",
			path:        "/agent/channels/reply",
			body:        `{"reply_to_message_id":"msg-source","body":{"text":"ack"},"proof":"spoofed-proof"}`,
			wantMessage: "sender identity and proof are daemon-derived",
		},
		{
			name:        "Should reject removed coordination channel on reply",
			path:        "/agent/channels/reply",
			body:        `{"reply_to_message_id":"msg-source","body":{"text":"ack"},"coordination_channel_id":"legacy"}`,
			wantMessage: "coordination_channel_id",
		},
		{
			name: "Should reject removed nested coordination channel on reply",
			path: "/agent/channels/reply",
			body: `{"reply_to_message_id":"msg-source","body":{"text":"ack"},"metadata":{"task_id":"task-1",` +
				`"run_id":"run-1","channel_id":"builders","coordination_channel_id":"legacy",` +
				`"message_kind":"reply","correlation_id":"run-1"}}`,
			wantMessage: "coordination_channel_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sendCalls := 0
			engine := newAgentCoreTestRouter(t, &agentCoreNetworkService{
				SendFn: func(context.Context, network.SendRequest) (string, error) {
					sendCalls++
					return "msg-unexpected", nil
				},
			})
			response := performAgentCoreRequest(
				t,
				engine,
				http.MethodPost,
				tt.path,
				[]byte(tt.body),
				agentCoreHeaders(),
			)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, http.StatusBadRequest, response.Body.String())
			}
			if !strings.Contains(response.Body.String(), tt.wantMessage) {
				t.Fatalf("body = %s, want %q", response.Body.String(), tt.wantMessage)
			}
			if sendCalls != 0 {
				t.Fatalf("Send() calls = %d, want 0", sendCalls)
			}
		})
	}
}

func TestAgentChannelCoreHandlersUsePersistedReplySourceMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should inherit reply metadata from persisted source messages", func(t *testing.T) {
		t.Parallel()

		var sent []network.SendRequest
		networkService := &agentCoreNetworkService{
			ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
				if workspaceID != "ws-1" || channel != "builders" {
					t.Fatalf("ListPeers() = %q/%q, want ws-1/builders", workspaceID, channel)
				}
				sessionID := "sess-peer"
				return []network.PeerInfo{{
					SessionID:   &sessionID,
					PeerID:      "reviewer.sess-peer",
					WorkspaceID: "ws-1",
					Channel:     "builders",
					Local:       true,
				}}, nil
			},
			InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
				if sessionID != "sess-agent" {
					t.Fatalf("Inbox() sessionID = %q, want sess-agent", sessionID)
				}
				return nil, nil
			},
			SendFn: func(_ context.Context, request network.SendRequest) (string, error) {
				sent = append(sent, request)
				return "msg-reply", nil
			},
		}
		networkStore := agentCoreNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if query.WorkspaceID != "ws-1" || query.SessionID != "sess-agent" || query.MessageID != "msg-source" ||
					query.Limit != 1 {
					t.Fatalf("ListNetworkMessages() query = %#v, want persisted source lookup", query)
				}
				return []store.NetworkMessageEntry{{
					MessageID:   "msg-source",
					SessionID:   "sess-agent",
					WorkspaceID: "ws-1",
					Channel:     "builders",
					Surface:     store.NetworkSurfaceThread,
					ThreadID:    "thread_agent_core",
					Direction:   "received",
					PeerFrom:    "sess-peer",
					PeerTo:      "coder.sess-agent",
					Kind:        store.NetworkKindSay,
					Body:        json.RawMessage(`{"text":"coordination"}`),
					ExtJSON:     agentCoreCoordinationExtJSON(t, contract.CoordinationMessageRequest),
					Timestamp:   time.Date(2026, 4, 26, 10, 2, 0, 0, time.UTC),
				}}, nil
			},
		}
		engine := newAgentCoreTestRouterWithNetworkStore(t, networkService, networkStore)

		replyResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/reply",
			[]byte(
				`{"reply_to_message_id":"msg-source","body":{"text":"ack"},"metadata":{"task_id":"","run_id":"","channel_id":"","message_kind":"","correlation_id":""}}`,
			),
			agentCoreHeaders(),
		)

		if replyResp.Code != http.StatusAccepted {
			t.Fatalf(
				"reply status = %d, want %d; body=%s",
				replyResp.Code,
				http.StatusAccepted,
				replyResp.Body.String(),
			)
		}
		if len(sent) != 1 || sent[0].ReplyTo == nil || *sent[0].ReplyTo != "msg-source" {
			t.Fatalf("sent requests = %#v, want one reply to persisted source", sent)
		}
		if sent[0].To == nil || *sent[0].To != "reviewer.sess-peer" {
			t.Fatalf("sent target = %#v, want resolved peer reviewer.sess-peer", sent[0].To)
		}
		var response contract.AgentChannelMessageResponse
		decodeAgentCoreResponse(t, replyResp, &response)
		if response.Message.ToSessionID != "sess-peer" {
			t.Fatalf("reply response target = %q, want durable session sess-peer", response.Message.ToSessionID)
		}
		if metadata := decodeAgentCoreCoordinationExt(
			t,
			sent[0].Ext,
		); metadata.MessageKind != contract.CoordinationMessageReply ||
			metadata.TaskID != "task-1" ||
			metadata.RunID != "run-1" ||
			metadata.ChannelID != "builders" {
			t.Fatalf("reply metadata = %#v, want inherited persisted metadata", metadata)
		}
	})

	t.Run("Should surface persisted source ext decode failures", func(t *testing.T) {
		t.Parallel()

		networkService := &agentCoreNetworkService{
			InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
				if sessionID != "sess-agent" {
					t.Fatalf("Inbox() sessionID = %q, want sess-agent", sessionID)
				}
				return nil, nil
			},
		}
		networkStore := agentCoreNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if query.MessageID != "msg-source" {
					t.Fatalf("ListNetworkMessages() query = %#v, want persisted source lookup", query)
				}
				return []store.NetworkMessageEntry{{
					MessageID:   "msg-source",
					SessionID:   "sess-agent",
					WorkspaceID: "ws-1",
					Channel:     "builders",
					PeerFrom:    "reviewer.sess-peer",
					Body:        json.RawMessage(`{"text":"coordination"}`),
					ExtJSON:     json.RawMessage(`{"agh.coordination":`),
					Timestamp:   time.Date(2026, 4, 26, 10, 2, 0, 0, time.UTC),
				}}, nil
			},
		}
		engine := newAgentCoreTestRouterWithNetworkStore(t, networkService, networkStore)

		replyResp := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/reply",
			[]byte(
				`{"reply_to_message_id":"msg-source","body":{"text":"ack"},"metadata":{"task_id":"","run_id":"","channel_id":"","message_kind":"","correlation_id":""}}`,
			),
			agentCoreHeaders(),
		)

		if replyResp.Code != http.StatusInternalServerError {
			t.Fatalf(
				"reply status = %d, want %d; body=%s",
				replyResp.Code,
				http.StatusInternalServerError,
				replyResp.Body.String(),
			)
		}
		if !strings.Contains(replyResp.Body.String(), "decode network message ext_json") {
			t.Fatalf("reply body = %s, want persisted ext decode error", replyResp.Body.String())
		}
	})
}

func TestAgentMeCoreHandlerEnrichesContextAndChannels(t *testing.T) {
	t.Parallel()

	t.Run("Should enrich agent me with context and visible channels", func(t *testing.T) {
		t.Parallel()

		engine := newAgentCoreTestRouter(t, &agentCoreNetworkService{
			ListChannelsFn: func(_ context.Context, workspaceID string) ([]network.ChannelInfo, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ListChannels() workspaceID = %q, want ws-1", workspaceID)
				}
				return []network.ChannelInfo{{Channel: "builders", PeerCount: 1}}, nil
			},
		})

		recorder := performAgentCoreRequest(t, engine, http.MethodGet, "/agent/me", nil, agentCoreHeaders())
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentMeResponse
		decodeAgentCoreResponse(t, recorder, &response)
		if response.Me.Self.SessionID != "sess-agent" ||
			response.Me.Workspace.ID != "ws-1" ||
			len(response.Me.Capabilities) != 1 ||
			len(response.Me.Channels) != 1 ||
			len(response.Me.ActiveTaskLeases) != 1 ||
			!response.Me.Coordinator.Enabled ||
			response.Me.Coordinator.AgentName != "coordinator" ||
			response.Me.Coordinator.WorkspaceID != "ws-1" ||
			response.Me.Limits.MaxChildren != 2 {
			t.Fatalf("agent me = %#v, want context and channel enrichment", response.Me)
		}
	})
}

func TestAgentCoordinatorRoleCoreHandlerReturnsResolvedWorkspaceConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should return resolved coordinator config for the caller workspace", func(t *testing.T) {
		t.Parallel()

		engine := newAgentCoreTestRouter(t, &agentCoreNetworkService{})
		recorder := performAgentCoreRequest(
			t,
			engine,
			http.MethodGet,
			"/agent/coordinator/config",
			nil,
			agentCoreHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentCoordinatorConfigResponse
		decodeAgentCoreResponse(t, recorder, &response)
		if !response.Coordinator.Enabled ||
			response.Coordinator.AgentName != "coordinator" ||
			response.Coordinator.DefaultTTLSeconds != 2700 ||
			response.Coordinator.MaxChildren != 5 ||
			response.Coordinator.MaxActiveSessionsPerWorkspace != 5 ||
			response.Coordinator.Source != contract.CoordinatorConfigSourceWorkspace ||
			response.Coordinator.WorkspaceID != "ws-1" {
			t.Fatalf("coordinator config = %#v, want resolved workspace config", response.Coordinator)
		}
	})
}

func TestAgentChannelCoreHandlersRejectInvalidIdentityAndClaimToken(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid identity headers and raw claim tokens", func(t *testing.T) {
		t.Parallel()

		sendCalled := false
		engine := newAgentCoreTestRouter(t, &agentCoreNetworkService{
			SendFn: func(context.Context, network.SendRequest) (string, error) {
				sendCalled = true
				return "unexpected", nil
			},
		})

		denied := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/builders/send",
			[]byte(
				`{"body":{"text":"ok"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
			),
			map[string]string{},
		)
		if denied.Code != http.StatusUnauthorized {
			t.Fatalf("denied status = %d, want %d", denied.Code, http.StatusUnauthorized)
		}

		rawToken := performAgentCoreRequest(
			t,
			engine,
			http.MethodPost,
			"/agent/channels/builders/send",
			[]byte(
				`{"body":{"claim_token":"secret"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
			),
			agentCoreHeaders(),
		)
		if rawToken.Code != http.StatusBadRequest {
			t.Fatalf(
				"claim_token status = %d, want %d; body=%s",
				rawToken.Code,
				http.StatusBadRequest,
				rawToken.Body.String(),
			)
		}
		if sendCalled {
			t.Fatal("Send should not be called for denied/raw-token requests")
		}
	})
}

func TestAgentTaskClaimCriteriaIncludesSoulProvenance(t *testing.T) {
	t.Parallel()

	t.Run("Should copy session soul provenance into claim criteria", func(t *testing.T) {
		t.Parallel()

		handlers := &BaseHandlers{}
		caller := agentidentity.Caller{
			Session: agentidentity.SessionSnapshot{
				ID:                   "sess-agent",
				AgentName:            "coder",
				WorkspaceID:          "ws-1",
				NetworkParticipation: participation.CloneSpec(coreTestLiveParticipation("ws-1", "builders")),
				State:                session.StateActive,
				SoulSnapshotID:       "soul-snapshot-1",
				SoulDigest:           "sha256:resolved",
			},
			Actor: taskpkg.ActorContext{
				Actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-agent"},
			},
		}

		criteria, err := handlers.agentTaskClaimCriteria(
			context.Background(),
			contract.AgentTaskClaimNextRequest{LeaseSeconds: 60},
			caller,
		)
		if err != nil {
			t.Fatalf("agentTaskClaimCriteria() error = %v", err)
		}
		if criteria.Soul == nil {
			t.Fatal("ClaimCriteria.Soul = nil, want session provenance")
		}
		if criteria.Soul.SnapshotID != "soul-snapshot-1" ||
			criteria.Soul.Digest != "sha256:resolved" ||
			criteria.Soul.AgentName != "coder" {
			t.Fatalf("ClaimCriteria.Soul = %#v, want caller soul provenance", criteria.Soul)
		}
		if criteria.ParticipationChannel != "" {
			t.Fatalf(
				"ClaimCriteria.ParticipationChannel = %q, want no ordinary-claim channel gate",
				criteria.ParticipationChannel,
			)
		}
		wantParticipation := coreTestLiveParticipation("ws-1", "builders")
		if criteria.CallerNetworkParticipation == nil ||
			*criteria.CallerNetworkParticipation != wantParticipation {
			t.Fatalf(
				"ClaimCriteria.CallerNetworkParticipation = %#v, want %#v",
				criteria.CallerNetworkParticipation,
				wantParticipation,
			)
		}
	})
}

type agentCoreNetworkService struct {
	SendFn         func(context.Context, network.SendRequest) (string, error)
	ListPeersFn    func(context.Context, string, string) ([]network.PeerInfo, error)
	ListChannelsFn func(context.Context, string) ([]network.ChannelInfo, error)
	InboxFn        func(context.Context, string) ([]network.Envelope, error)
	WaitInboxFn    func(context.Context, string, string) ([]network.Envelope, error)
}

func (s *agentCoreNetworkService) Send(ctx context.Context, req network.SendRequest) (string, error) {
	if s.SendFn != nil {
		return s.SendFn(ctx, req)
	}
	return "", nil
}

func (s *agentCoreNetworkService) ListPeers(
	ctx context.Context,
	workspaceID string,
	channel string,
) ([]network.PeerInfo, error) {
	if s.ListPeersFn != nil {
		return s.ListPeersFn(ctx, workspaceID, channel)
	}
	return nil, nil
}

func (s *agentCoreNetworkService) ListChannels(ctx context.Context, workspaceID string) ([]network.ChannelInfo, error) {
	if s.ListChannelsFn != nil {
		return s.ListChannelsFn(ctx, workspaceID)
	}
	return nil, nil
}

func (s *agentCoreNetworkService) Status(context.Context) (*network.Status, error) {
	return &network.Status{Enabled: true, Status: network.StatusReady}, nil
}

func (s *agentCoreNetworkService) Inbox(ctx context.Context, sessionID string) ([]network.Envelope, error) {
	if s.InboxFn != nil {
		return s.InboxFn(ctx, sessionID)
	}
	return nil, nil
}

func (s *agentCoreNetworkService) WaitInbox(
	ctx context.Context,
	sessionID string,
	channel string,
) ([]network.Envelope, error) {
	if s.WaitInboxFn != nil {
		return s.WaitInboxFn(ctx, sessionID, channel)
	}
	return s.Inbox(ctx, sessionID)
}

type agentCoreNetworkStore struct {
	ListNetworkMessagesFn func(context.Context, store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error)
}

func (s agentCoreNetworkStore) ResolveDirectRoom(
	context.Context,
	store.NetworkDirectRoomEntry,
) (store.NetworkDirectRoomSummary, error) {
	return store.NetworkDirectRoomSummary{}, nil
}

func (s agentCoreNetworkStore) WriteConversationMessage(
	context.Context,
	store.NetworkConversationMessage,
) (store.NetworkConversationWriteResult, error) {
	return store.NetworkConversationWriteResult{}, nil
}

func (s agentCoreNetworkStore) ListThreads(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkThreadQuery,
) (store.NetworkThreadPage, error) {
	return store.NetworkThreadPage{}, nil
}

func (s agentCoreNetworkStore) GetThread(
	context.Context,
	store.NetworkChannelRef,
	string,
) (store.NetworkThreadSummary, error) {
	return store.NetworkThreadSummary{}, nil
}

func (s agentCoreNetworkStore) ListThreadParticipants(
	context.Context,
	store.NetworkChannelRef,
	string,
) ([]store.NetworkThreadParticipant, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) UpdateNetworkThreadSessionTokenStats(
	context.Context,
	store.NetworkThreadSessionTokenStatsUpdate,
) error {
	return nil
}

func (s agentCoreNetworkStore) ListNetworkThreadSessionTokenStats(
	context.Context,
	store.NetworkThreadSessionTokenStatsQuery,
) ([]store.NetworkThreadSessionTokenStats, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) ListDirectRooms(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkDirectRoomQuery,
) (store.NetworkDirectRoomPage, error) {
	return store.NetworkDirectRoomPage{}, nil
}

func (s agentCoreNetworkStore) GetDirectRoom(
	context.Context,
	store.NetworkChannelRef,
	string,
) (store.NetworkDirectRoomSummary, error) {
	return store.NetworkDirectRoomSummary{}, nil
}

func (s agentCoreNetworkStore) ListConversationMessages(
	context.Context,
	store.NetworkConversationRef,
	store.NetworkConversationMessageQuery,
) ([]store.NetworkConversationMessage, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) GetWork(context.Context, string, string) (store.NetworkWorkEntry, error) {
	return store.NetworkWorkEntry{}, nil
}

func (s agentCoreNetworkStore) ListNetworkAudit(
	context.Context,
	store.NetworkAuditQuery,
) ([]store.NetworkAuditEntry, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) WriteNetworkAudit(context.Context, store.NetworkAuditEntry) error {
	return nil
}

func (s agentCoreNetworkStore) GetNetworkChannel(
	context.Context,
	store.NetworkChannelRef,
) (store.NetworkChannelEntry, error) {
	return store.NetworkChannelEntry{}, nil
}

func (s agentCoreNetworkStore) ListNetworkChannels(
	context.Context,
	store.NetworkChannelQuery,
) ([]store.NetworkChannelEntry, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) WriteNetworkChannel(context.Context, store.NetworkChannelEntry) error {
	return nil
}

func (s agentCoreNetworkStore) CreateNetworkChannel(context.Context, store.NetworkChannelEntry) error {
	return nil
}

func (s agentCoreNetworkStore) PatchNetworkChannel(
	context.Context,
	store.NetworkChannelRef,
	store.NetworkChannelPatch,
) error {
	return nil
}

func (s agentCoreNetworkStore) DeleteNetworkChannel(context.Context, store.NetworkChannelRef) error {
	return nil
}

func (s agentCoreNetworkStore) PutNetworkSubscription(
	context.Context,
	store.NetworkSubscriptionEntry,
) error {
	return nil
}

func (s agentCoreNetworkStore) PutNetworkSubscriptionWithChannel(
	context.Context,
	store.NetworkChannelEntry,
	store.NetworkSubscriptionEntry,
) error {
	return nil
}

func (s agentCoreNetworkStore) ListNetworkSubscriptions(
	context.Context,
	store.NetworkSubscriptionQuery,
) ([]store.NetworkSubscriptionEntry, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) DeleteNetworkSubscription(context.Context, store.NetworkSubscriptionRef) error {
	return nil
}

func (s agentCoreNetworkStore) PutNetworkTaskThreadOrigin(
	context.Context,
	store.NetworkTaskThreadOrigin,
) error {
	return nil
}

func (s agentCoreNetworkStore) ListNetworkTaskThreadOrigins(
	context.Context,
	store.NetworkTaskThreadOriginQuery,
) ([]store.NetworkTaskThreadOrigin, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) PutTaskDesignationRollup(context.Context, store.TaskDesignationRollup) error {
	return nil
}

func (s agentCoreNetworkStore) ListTaskDesignationRollups(
	context.Context,
	store.TaskDesignationRollupQuery,
) ([]store.TaskDesignationRollup, error) {
	return nil, nil
}

func (s agentCoreNetworkStore) WriteNetworkMessage(context.Context, store.NetworkMessageEntry) error {
	return nil
}

func (s agentCoreNetworkStore) ListNetworkMessages(
	ctx context.Context,
	query store.NetworkMessageQuery,
) ([]store.NetworkMessageEntry, error) {
	if s.ListNetworkMessagesFn != nil {
		return s.ListNetworkMessagesFn(ctx, query)
	}
	return nil, nil
}

type agentCoreContextService func(context.Context, *session.Info) (contract.AgentContextPayload, error)

func (f agentCoreContextService) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	return f(ctx, info)
}

type agentCoreCoordinatorRoleResolver struct{}

func (agentCoreCoordinatorRoleResolver) ResolveCoordinatorRole(
	_ context.Context,
	_ string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	return aghconfig.ResolvedCoordinatorRole{
		Enabled:                       true,
		AgentName:                     "coordinator",
		Provider:                      "codex",
		Model:                         "gpt-4o",
		TTL:                           45 * time.Minute,
		MaxChildren:                   5,
		MaxActiveSessionsPerWorkspace: 5,
	}, nil
}

func newAgentCoreTestRouter(t *testing.T, networkService *agentCoreNetworkService) *gin.Engine {
	t.Helper()

	return newAgentCoreTestRouterWithNetworkStore(t, networkService, nil)
}

func newAgentCoreTestRouterWithNetworkStore(
	t *testing.T,
	networkService *agentCoreNetworkService,
	networkStore NetworkStore,
) *gin.Engine {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Network.Enabled = true
	handlers := NewBaseHandlers(&BaseHandlerConfig{
		TransportName:       "udsapi",
		Sessions:            agentCoreSessionManager(t),
		Network:             networkService,
		NetworkStore:        networkStore,
		AgentContextService: agentCoreContextService(agentCoreContextPayload),
		CoordinatorRole:     agentCoreCoordinatorRoleResolver{},
		Config:              cfg,
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		StreamDone:          make(chan struct{}),
		StartedAt:           time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
		Now: func() time.Time {
			return time.Date(2026, 4, 26, 10, 0, 1, 0, time.UTC)
		},
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/agent/me", handlers.AgentMe)
	engine.GET("/agent/context", handlers.AgentContext)
	engine.GET("/agent/coordinator/config", handlers.AgentCoordinatorRole)
	engine.GET("/agent/channels", handlers.AgentChannels)
	engine.GET("/agent/channels/:channel/recv", handlers.AgentChannelRecv)
	engine.POST("/agent/channels/:channel/send", handlers.AgentChannelSend)
	engine.POST("/agent/channels/reply", handlers.AgentChannelReply)
	return engine
}

func agentCoreSessionManager(t *testing.T) sessionManagerStub {
	t.Helper()

	return sessionManagerStub{
		status: func(_ context.Context, id string) (*session.Info, error) {
			if id != "sess-agent" {
				return nil, session.ErrSessionNotFound
			}
			now := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
			return &session.Info{
				ID:                   "sess-agent",
				Name:                 "worker",
				AgentName:            "coder",
				Provider:             "test-provider",
				WorkspaceID:          "ws-1",
				Workspace:            "/workspace/project",
				NetworkParticipation: coreTestLiveParticipation("ws-1", "builders"),
				Type:                 session.SessionTypeUser,
				Lineage: &store.SessionLineage{
					ParentSessionID: "sess-root",
					RootSessionID:   "sess-root",
					SpawnDepth:      1,
				},
				State:     session.StateActive,
				CreatedAt: now,
				UpdatedAt: now,
			}, nil
		},
	}
}

func agentCoreContextPayload(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
	channel := contract.CoordinationChannelPayload{
		ID:          "builders",
		DisplayName: "builders",
		WorkspaceID: info.WorkspaceID,
	}
	return contract.AgentContextPayload{
		Self: contract.AgentIdentityPayload{
			SessionID: info.ID,
			AgentName: info.AgentName,
			Provider:  info.Provider,
		},
		Workspace: contract.AgentWorkspacePayload{ID: info.WorkspaceID, RootDir: info.Workspace},
		Session: contract.AgentSessionPayload{
			ID:                           info.ID,
			State:                        info.State,
			ResolvedNetworkParticipation: participation.CloneSpec(info.NetworkParticipation),
			Lineage:                      contract.SessionLineagePayloadFromStore(info.Lineage),
			CreatedAt:                    info.CreatedAt,
			UpdatedAt:                    info.UpdatedAt,
		},
		Task: contract.AgentTaskContextPayload{
			Available: true,
			Lease: &contract.TaskRunLeaseSummaryPayload{
				TaskID:              "task-1",
				RunID:               "run-1",
				Status:              taskpkg.TaskRunStatusRunning,
				SessionID:           info.ID,
				CoordinationChannel: &channel,
			},
		},
		CoordinationChannel: contract.AgentCoordinationChannelContextPayload{
			Available: true,
			Channel:   &channel,
		},
		InboxSummary: contract.AgentInboxSummaryPayload{},
		PeerRoster:   contract.AgentPeerRosterPayload{},
		Capabilities: contract.AgentCapabilitySectionPayload{
			Capabilities: []contract.AgentCapabilityPayload{{
				ID:     "shell",
				Source: "test",
			}},
		},
		Limits: contract.AgentLimitsPayload{ContextSectionLimit: 20, MaxChildren: 2},
		Provenance: contract.AgentContextProvenancePayload{
			GeneratedAt: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
			Source:      "test",
		},
	}, nil
}

func performAgentCoreRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, path, bytes.NewReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func agentCoreHeaders() map[string]string {
	return map[string]string{
		agentidentity.HeaderSessionID:   "sess-agent",
		agentidentity.HeaderAgent:       "coder",
		agentidentity.HeaderWorkspaceID: "ws-1",
	}
}

func decodeAgentCoreResponse(t *testing.T, recorder *httptest.ResponseRecorder, dest any) {
	t.Helper()

	if err := json.Unmarshal(recorder.Body.Bytes(), dest); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, recorder.Body.String())
	}
}

func agentCoreEnvelope(
	t *testing.T,
	messageID string,
	channel string,
	kind contract.CoordinationMessageKind,
) network.Envelope {
	t.Helper()

	return network.Envelope{
		Protocol:    network.ProtocolV0,
		WorkspaceID: "ws-1",
		ID:          messageID,
		Kind:        network.KindSay,
		Channel:     channel,
		From:        "coder.sess-peer",
		TS:          time.Date(2026, 4, 26, 10, 1, 0, 0, time.UTC).Unix(),
		Body:        json.RawMessage(`{"text":"coordination"}`),
		Ext: network.ExtensionMap{
			"coordination": agentCoreCoordinationMetadata(t, kind),
		},
	}
}

func agentCoreCoordinationMetadata(t *testing.T, kind contract.CoordinationMessageKind) json.RawMessage {
	t.Helper()

	content, err := json.Marshal(contract.CoordinationMessageMetadataPayload{
		TaskID:        "task-1",
		RunID:         "run-1",
		WorkflowID:    "wf-1",
		ChannelID:     "builders",
		MessageKind:   kind,
		CorrelationID: "run-1",
	})
	if err != nil {
		t.Fatalf("json.Marshal(coordination metadata) error = %v", err)
	}
	return content
}

func agentCoreCoordinationExtJSON(t *testing.T, kind contract.CoordinationMessageKind) json.RawMessage {
	t.Helper()

	content, err := json.Marshal(map[string]json.RawMessage{
		agentCoordinationExtKey: agentCoreCoordinationMetadata(t, kind),
	})
	if err != nil {
		t.Fatalf("json.Marshal(coordination ext) error = %v", err)
	}
	return content
}

func decodeAgentCoreCoordinationExt(
	t *testing.T,
	ext network.ExtensionMap,
) contract.CoordinationMessageMetadataPayload {
	t.Helper()

	var metadata contract.CoordinationMessageMetadataPayload
	if err := json.Unmarshal(ext["coordination"], &metadata); err != nil {
		t.Fatalf("json.Unmarshal(coordination ext) error = %v", err)
	}
	return metadata
}
