package udsapi

import (
	"bytes"
	"context"
	"encoding/json"
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
)

func TestAgentContextReturnsSituationPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return the situation payload for the caller session", func(t *testing.T) {
		t.Parallel()

		manager := activeAgentSessionManager(t)
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.AgentContextService = agentContextServiceFunc(
			func(_ context.Context, info *session.Info) (contract.AgentContextPayload, error) {
				if info.ID != "sess-agent" || info.AgentName != "coder" {
					t.Fatalf("ContextForSession() info = %#v, want caller session", info)
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
						CreatedAt:                    info.CreatedAt,
						UpdatedAt:                    info.UpdatedAt,
					},
					Task:                contract.AgentTaskContextPayload{Available: true},
					CoordinationChannel: contract.AgentCoordinationChannelContextPayload{Available: true},
					InboxSummary:        contract.AgentInboxSummaryPayload{},
					PeerRoster:          contract.AgentPeerRosterPayload{},
					Capabilities:        contract.AgentCapabilitySectionPayload{},
					Limits:              contract.AgentLimitsPayload{ContextSectionLimit: 20},
					Provenance: contract.AgentContextProvenancePayload{
						GeneratedAt: time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC),
						Source:      "test",
					},
				}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/context",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentContextResponse
		decodeJSONResponse(t, recorder, &response)
		if response.Context.Self.SessionID != "sess-agent" ||
			response.Context.Workspace.ID != "ws-1" ||
			!response.Context.Task.Available {
			t.Fatalf("context = %#v, want validated situation payload", response.Context)
		}
	})
}

func TestAgentCoordinatorRoleRouteReturnsResolvedPayload(t *testing.T) {
	t.Parallel()

	t.Run("Should return resolved workspace coordinator payload", func(t *testing.T) {
		t.Parallel()

		manager := activeAgentSessionManager(t)
		handlers := newTestHandlers(t, manager, stubObserver{}, newTestHomePaths(t))
		handlers.CoordinatorRole = agentCoordinatorRoleResolverFunc(
			func(_ context.Context, workspaceID string) (aghconfig.ResolvedCoordinatorRole, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ResolveCoordinatorRole() workspaceID = %q, want ws-1", workspaceID)
				}
				return aghconfig.ResolvedCoordinatorRole{
					Enabled:                       true,
					AgentName:                     "coordinator",
					Provider:                      "codex",
					Model:                         "gpt-4o",
					TTL:                           45 * time.Minute,
					MaxChildren:                   5,
					MaxActiveSessionsPerWorkspace: 5,
				}, nil
			},
		)
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/coordinator/config",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentCoordinatorConfigResponse
		decodeJSONResponse(t, recorder, &response)
		if !response.Coordinator.Enabled ||
			response.Coordinator.AgentName != "coordinator" ||
			response.Coordinator.DefaultTTLSeconds != 2700 ||
			response.Coordinator.Source != contract.CoordinatorConfigSourceWorkspace ||
			response.Coordinator.WorkspaceID != "ws-1" {
			t.Fatalf("coordinator = %#v, want workspace coordinator payload", response.Coordinator)
		}
	})
}

func TestAgentChannelSendUsesCallerIdentityAndRejectsRawClaimToken(t *testing.T) {
	t.Parallel()

	var seen network.SendRequest
	handlers := newAgentChannelHandlers(t, stubNetworkService{
		SendFn: func(_ context.Context, request network.SendRequest) (string, error) {
			seen = request
			return "msg-send", nil
		},
	})
	engine := newTestRouter(t, handlers)

	recorder := performAgentKernelRequest(
		t,
		engine,
		http.MethodPost,
		"/api/agent/channels/builders/send",
		[]byte(
			`{"body":{"text":"progress"},"metadata":{"task_id":"task-1","run_id":"run-1","workflow_id":"wf-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
		),
		agentKernelHeaders(),
	)
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
	}
	if seen.SessionID != "sess-agent" || seen.Channel != "builders" || seen.Kind != network.KindSay {
		t.Fatalf("send request = %#v, want caller-bound say on builders", seen)
	}
	metadata := decodeCoordinationExt(t, seen.Ext)
	if metadata.TaskID != "task-1" ||
		metadata.RunID != "run-1" ||
		metadata.WorkflowID != "wf-1" ||
		metadata.MessageKind != contract.CoordinationMessageStatus {
		t.Fatalf("metadata = %#v, want preserved coordination metadata", metadata)
	}

	for _, tt := range []struct {
		name       string
		body       []byte
		mustRedact string
	}{
		{
			name:       "payload body",
			body:       []byte(`{"body":{"claim_token":"agh_claim_UDS_CHANNEL_123"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`),
			mustRedact: "agh_claim_UDS_CHANNEL_123",
		},
		{
			name:       "metadata ext",
			body:       []byte(`{"body":{"text":"ok"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1","ext":{"claim_token":"agh_claim_UDS_CHANNEL_123"}}}`),
			mustRedact: "agh_claim_UDS_CHANNEL_123",
		},
		{
			name: "caller supplied verified-format identity",
			body: []byte(`{"body":{"text":"spoof"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"},"from":"alice@39f713d0a644253f04529421b9f51b9b"}`),
		},
	} {
		t.Run("Should reject unsafe "+tt.name, func(t *testing.T) {
			t.Parallel()

			handlers := newAgentChannelHandlers(t, stubNetworkService{
				SendFn: func(context.Context, network.SendRequest) (string, error) {
					t.Fatal("Send should not be called when claim_token is present")
					return "", nil
				},
			})
			resp := performAgentKernelRequest(
				t,
				newTestRouter(t, handlers),
				http.MethodPost,
				"/api/agent/channels/builders/send",
				tt.body,
				agentKernelHeaders(),
			)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
			}
			if tt.mustRedact != "" && strings.Contains(resp.Body.String(), tt.mustRedact) {
				t.Fatalf("unsafe send response leaked raw credential: %s", resp.Body.String())
			}
		})
	}

	denied := performAgentKernelRequest(
		t,
		engine,
		http.MethodPost,
		"/api/agent/channels/builders/send",
		[]byte(
			`{"body":{"text":"progress"},"metadata":{"task_id":"task-1","run_id":"run-1","channel_id":"builders","message_kind":"status","correlation_id":"run-1"}}`,
		),
		map[string]string{},
	)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("denied status = %d, want %d", denied.Code, http.StatusUnauthorized)
	}
}

func TestAgentChannelsListsCallerVisibleChannels(t *testing.T) {
	t.Parallel()

	t.Run("Should list caller-visible channels for the caller workspace", func(t *testing.T) {
		t.Parallel()

		handlers := newAgentChannelHandlers(t, stubNetworkService{
			ListChannelsFn: func(_ context.Context, workspaceID string) ([]network.ChannelInfo, error) {
				if workspaceID != "ws-1" {
					t.Fatalf("ListChannels() workspaceID = %q, want ws-1", workspaceID)
				}
				return []network.ChannelInfo{{Channel: "builders", PeerCount: 2}}, nil
			},
		})
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/channels",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}

		var response contract.AgentChannelsResponse
		decodeJSONResponse(t, recorder, &response)
		if len(response.Channels) != 1 ||
			response.Channels[0].ID != "builders" ||
			response.Channels[0].WorkspaceID != "ws-1" ||
			len(response.Channels[0].AllowedMessageKinds) != len(contract.CoordinationMessageKinds()) {
			t.Fatalf("channels = %#v, want caller workspace builders channel", response.Channels)
		}
	})
}

func TestAgentChannelRecvUsesInboxMode(t *testing.T) {
	t.Parallel()

	t.Run("Should use WaitInbox when wait is true", func(t *testing.T) {
		t.Parallel()

		var seenSession string
		var seenChannel string
		handlers := newAgentChannelHandlers(t, stubNetworkService{
			WaitInboxFn: func(_ context.Context, sessionID string, channel string) ([]network.Envelope, error) {
				seenSession = sessionID
				seenChannel = channel
				return []network.Envelope{
					agentChannelEnvelope(t, "msg-other", "other", contract.CoordinationMessageStatus),
					agentChannelEnvelope(t, "msg-builders", "builders", contract.CoordinationMessageResult),
				}, nil
			},
			InboxFn: func(context.Context, string) ([]network.Envelope, error) {
				t.Fatal("Inbox should not be called for wait receive")
				return nil, nil
			},
		})
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/channels/builders/recv?wait=true&limit=1",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if seenSession != "sess-agent" || seenChannel != "builders" {
			t.Fatalf("WaitInbox() args = %q/%q, want caller session builders", seenSession, seenChannel)
		}

		var response contract.AgentChannelMessagesResponse
		decodeJSONResponse(t, recorder, &response)
		if len(response.Messages) != 1 ||
			response.Messages[0].MessageID != "msg-builders" ||
			response.Messages[0].Metadata.MessageKind != contract.CoordinationMessageResult {
			t.Fatalf("messages = %#v, want one builders result message", response.Messages)
		}
	})

	t.Run("Should use Inbox when wait is false and ignore non-coordination extensions", func(t *testing.T) {
		t.Parallel()

		var seenSession string
		handlers := newAgentChannelHandlers(t, stubNetworkService{
			InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
				seenSession = sessionID
				return []network.Envelope{
					agentChannelEnvelope(t, "msg-other", "other", contract.CoordinationMessageStatus),
					agentChannelEnvelopeWithExt("msg-false-metadata", "builders", misleadingCoordinationExt()),
					agentChannelEnvelope(t, "msg-builders", "builders", contract.CoordinationMessageResult),
				}, nil
			},
			WaitInboxFn: func(context.Context, string, string) ([]network.Envelope, error) {
				t.Fatal("WaitInbox should not be called for non-wait receive")
				return nil, nil
			},
		})
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodGet,
			"/api/agent/channels/builders/recv",
			nil,
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if seenSession != "sess-agent" {
			t.Fatalf("Inbox() session = %q, want sess-agent", seenSession)
		}

		var response contract.AgentChannelMessagesResponse
		decodeJSONResponse(t, recorder, &response)
		if len(response.Messages) != 1 || response.Messages[0].MessageID != "msg-builders" {
			t.Fatalf("messages = %#v, want only explicit coordination metadata message", response.Messages)
		}
	})
}

func TestAgentChannelReplyResolvesSourceMessageMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve source metadata into a direct reply request", func(t *testing.T) {
		t.Parallel()

		var seen network.SendRequest
		source := agentChannelEnvelope(t, "msg-source", "builders", contract.CoordinationMessageRequest)
		source.From = "reviewer.sess-peer"
		peerSessionID := "sess-peer"
		directID := "direct_reply_01"
		source.Surface = new(network.SurfaceDirect)
		source.DirectID = &directID
		handlers := newAgentChannelHandlers(t, stubNetworkService{
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
			InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
				if sessionID != "sess-agent" {
					t.Fatalf("Inbox() sessionID = %q, want sess-agent", sessionID)
				}
				return []network.Envelope{source}, nil
			},
			SendFn: func(_ context.Context, request network.SendRequest) (string, error) {
				seen = request
				return "msg-reply", nil
			},
		})
		engine := newTestRouter(t, handlers)

		recorder := performAgentKernelRequest(
			t,
			engine,
			http.MethodPost,
			"/api/agent/channels/reply",
			[]byte(`{"reply_to_message_id":"msg-source","body":{"text":"ack"}}`),
			agentKernelHeaders(),
		)
		if recorder.Code != http.StatusAccepted {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusAccepted, recorder.Body.String())
		}
		if seen.SessionID != "sess-agent" ||
			seen.Channel != "builders" ||
			seen.Kind != network.KindSay ||
			seen.Surface == nil ||
			*seen.Surface != network.SurfaceDirect ||
			seen.DirectID != nil ||
			seen.To == nil ||
			*seen.To != "reviewer.sess-peer" ||
			seen.ReplyTo == nil ||
			*seen.ReplyTo != "msg-source" {
			t.Fatalf("reply send request = %#v, want direct reply to source peer/message", seen)
		}
		metadata := decodeCoordinationExt(t, seen.Ext)
		if metadata.TaskID != "task-1" ||
			metadata.RunID != "run-1" ||
			metadata.ChannelID != "builders" ||
			metadata.MessageKind != contract.CoordinationMessageReply {
			t.Fatalf("reply metadata = %#v, want inherited source metadata with reply kind", metadata)
		}
	})
}

type agentContextServiceFunc func(context.Context, *session.Info) (contract.AgentContextPayload, error)

func (f agentContextServiceFunc) ContextForSession(
	ctx context.Context,
	info *session.Info,
) (contract.AgentContextPayload, error) {
	return f(ctx, info)
}

type agentCoordinatorRoleResolverFunc func(context.Context, string) (aghconfig.ResolvedCoordinatorRole, error)

func (f agentCoordinatorRoleResolverFunc) ResolveCoordinatorRole(
	ctx context.Context,
	workspaceID string,
) (aghconfig.ResolvedCoordinatorRole, error) {
	return f(ctx, workspaceID)
}

func newAgentChannelHandlers(t *testing.T, networkService stubNetworkService) *Handlers {
	t.Helper()

	handlers := newTestHandlers(t, activeAgentSessionManager(t), stubObserver{}, newTestHomePaths(t))
	handlers.Config.Network.Enabled = true
	handlers.Network = networkService
	return handlers
}

func activeAgentSessionManager(t *testing.T) stubSessionManager {
	t.Helper()

	return stubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
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
				NetworkParticipation: udsTestLiveParticipation("ws-1", "builders"),
				Type:                 session.SessionTypeUser,
				State:                session.StateActive,
				CreatedAt:            now,
				UpdatedAt:            now,
			}, nil
		},
	}
}

func performAgentKernelRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	body []byte,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, path, bytesReader(body))
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)
	return recorder
}

func agentKernelHeaders() map[string]string {
	return map[string]string{
		agentidentity.HeaderSessionID:   "sess-agent",
		agentidentity.HeaderAgent:       "coder",
		agentidentity.HeaderWorkspaceID: "ws-1",
	}
}

func agentChannelEnvelope(
	t *testing.T,
	messageID string,
	channel string,
	kind contract.CoordinationMessageKind,
) network.Envelope {
	t.Helper()

	return agentChannelEnvelopeWithExt(messageID, channel, network.ExtensionMap{
		"coordination": mustCoordinationMetadata(t, kind),
	})
}

func agentChannelEnvelopeWithExt(
	messageID string,
	channel string,
	ext network.ExtensionMap,
) network.Envelope {
	return network.Envelope{
		Protocol:    network.ProtocolV0,
		WorkspaceID: "ws-1",
		ID:          messageID,
		Kind:        network.KindSay,
		Channel:     channel,
		From:        "coder.sess-peer",
		TS:          time.Date(2026, 4, 26, 10, 1, 0, 0, time.UTC).Unix(),
		Body:        json.RawMessage(`{"text":"coordination"}`),
		Ext:         ext,
	}
}

func misleadingCoordinationExt() network.ExtensionMap {
	return network.ExtensionMap{
		"task_id":        json.RawMessage(`"task-1"`),
		"run_id":         json.RawMessage(`"run-1"`),
		"workflow_id":    json.RawMessage(`"wf-1"`),
		"channel_id":     json.RawMessage(`"builders"`),
		"message_kind":   json.RawMessage(`"result"`),
		"correlation_id": json.RawMessage(`"run-1"`),
	}
}

func mustCoordinationMetadata(t *testing.T, kind contract.CoordinationMessageKind) json.RawMessage {
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

func decodeCoordinationExt(t *testing.T, ext network.ExtensionMap) contract.CoordinationMessageMetadataPayload {
	t.Helper()

	var metadata contract.CoordinationMessageMetadataPayload
	if err := json.Unmarshal(ext["coordination"], &metadata); err != nil {
		t.Fatalf("json.Unmarshal(coordination ext) error = %v", err)
	}
	return metadata
}

func bytesReader(body []byte) *bytes.Reader {
	if body == nil {
		return bytes.NewReader(nil)
	}
	return bytes.NewReader(body)
}
