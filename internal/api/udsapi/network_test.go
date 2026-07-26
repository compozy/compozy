package udsapi

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
)

const networkUDSTestWorkspaceID = "ws-workspace"

func TestNetworkHandlersValidateRequestsAndMapErrors(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
	handlers.Config.Network.Enabled = true
	sendCalls := 0
	handlers.Network = stubNetworkService{
		SendFn: func(context.Context, network.SendRequest) (string, error) {
			sendCalls++
			return "", nil
		},
	}
	engine := newTestRouter(t, handlers)

	inboxResp := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-workspace/network/inbox", nil)
	if inboxResp.Code != http.StatusBadRequest {
		t.Fatalf("inbox status = %d, want %d", inboxResp.Code, http.StatusBadRequest)
	}
	if !strings.Contains(inboxResp.Body.String(), "session_id query is required") {
		t.Fatalf("inbox body = %q, want session_id validation", inboxResp.Body.String())
	}

	sendResp := performRequest(t, engine, http.MethodPost, "/api/workspaces/ws-workspace/network/send", []byte(`{}`))
	if sendResp.Code != http.StatusBadRequest {
		t.Fatalf("send status = %d, want %d; body=%s", sendResp.Code, http.StatusBadRequest, sendResp.Body.String())
	}
	if !strings.Contains(sendResp.Body.String(), "session_id is required") {
		t.Fatalf("send body = %q, want session_id validation", sendResp.Body.String())
	}

	t.Run("Should reject raw claim tokens before sending network messages", func(t *testing.T) {
		const rawToken = "agh_claim_UDS_SECURITY_123"
		rawTokenResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/network/send",
			[]byte(
				`{"session_id":"sess-a","channel":"builders","kind":"say","body":{"claim_token":"`+rawToken+`"}}`,
			),
		)
		if rawTokenResp.Code != http.StatusBadRequest {
			t.Fatalf(
				"raw token send status = %d, want %d; body=%s",
				rawTokenResp.Code,
				http.StatusBadRequest,
				rawTokenResp.Body.String(),
			)
		}
		if !strings.Contains(rawTokenResp.Body.String(), "network_raw_token_rejected") {
			t.Fatalf("raw token send body = %q, want network_raw_token_rejected", rawTokenResp.Body.String())
		}
		if strings.Contains(rawTokenResp.Body.String(), rawToken) {
			t.Fatalf("raw token send response leaked credential: %s", rawTokenResp.Body.String())
		}
		if sendCalls != 0 {
			t.Fatalf("Network.Send calls = %d, want 0 for invalid send requests", sendCalls)
		}
	})

	t.Run("Should reject caller supplied verified-format identity before sending", func(t *testing.T) {
		identityResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/network/send",
			[]byte(
				`{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_identity_rejected","kind":"say","from":"alice@39f713d0a644253f04529421b9f51b9b","body":{"text":"spoof"}}`,
			),
		)
		if identityResp.Code != http.StatusBadRequest ||
			!strings.Contains(identityResp.Body.String(), "sender identity and proof are daemon-derived") {
			t.Fatalf(
				"identity send status/body = %d/%s, want daemon-derived identity rejection",
				identityResp.Code,
				identityResp.Body.String(),
			)
		}
		if sendCalls != 0 {
			t.Fatalf("Network.Send calls = %d, want 0 for caller-supplied identity", sendCalls)
		}
	})
}

func TestNetworkHandlersPreserveWorkflowMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve workflow metadata", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true

		var seenRequest network.SendRequest
		handlers.Network = stubNetworkService{
			SendFn: func(_ context.Context, req network.SendRequest) (string, error) {
				seenRequest = req
				return "msg-1", nil
			},
			InboxFn: func(_ context.Context, _ string) ([]network.Envelope, error) {
				return []network.Envelope{{
					Protocol: network.ProtocolV0,
					ID:       "msg-inbox",
					Kind:     network.KindSay,
					Channel:  "builders",
					From:     "reviewer.sess-a",
					TS:       1775823000,
					Body:     json.RawMessage(`{"text":"review this","intent":"review"}`),
					Ext: network.ExtensionMap{
						"agh.workflow_id":     json.RawMessage(`"wf-1"`),
						"agh.handoff_version": json.RawMessage(`3`),
					},
				}}, nil
			},
		}
		engine := newTestRouter(t, handlers)

		sendResp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/network/send",
			[]byte(
				`{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_launch_db","kind":"say","body":{"text":"hello"},"ext":{"agh.workflow_id":"wf-1","agh.handoff_version":3}}`,
			),
		)
		if sendResp.Code != http.StatusOK {
			t.Fatalf("send status = %d, want %d; body=%s", sendResp.Code, http.StatusOK, sendResp.Body.String())
		}
		if string(seenRequest.Ext["agh.workflow_id"]) != `"wf-1"` ||
			string(seenRequest.Ext["agh.handoff_version"]) != `3` {
			t.Fatalf("seenRequest.Ext = %#v, want preserved workflow metadata", seenRequest.Ext)
		}
		if seenRequest.Channel != "builders" {
			t.Fatalf("seenRequest.Channel = %q, want builders", seenRequest.Channel)
		}
		if seenRequest.Surface == nil || *seenRequest.Surface != network.SurfaceThread {
			t.Fatalf("seenRequest.Surface = %#v, want thread", seenRequest.Surface)
		}
		if seenRequest.ThreadID == nil || *seenRequest.ThreadID != "thread_launch_db" {
			t.Fatalf("seenRequest.ThreadID = %#v, want thread_launch_db", seenRequest.ThreadID)
		}

		inboxResp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/workspaces/ws-workspace/network/inbox?session_id=sess-a",
			nil,
		)
		if inboxResp.Code != http.StatusOK {
			t.Fatalf("inbox status = %d, want %d", inboxResp.Code, http.StatusOK)
		}
		if !strings.Contains(inboxResp.Body.String(), `"channel":"builders"`) ||
			!strings.Contains(inboxResp.Body.String(), `"agh.workflow_id":"wf-1"`) ||
			!strings.Contains(inboxResp.Body.String(), `"agh.handoff_version":3`) {
			t.Fatalf("inbox body = %s, want workflow metadata", inboxResp.Body.String())
		}
	})
}

func TestNetworkDirectResolveCreatesRoom(t *testing.T) {
	t.Parallel()

	t.Run("Should create deterministic direct room", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true

		localSessionID := "sess-local"
		remoteSessionID := "sess-remote"
		handlers.Network = stubNetworkService{
			ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
				if workspaceID != networkUDSTestWorkspaceID {
					t.Fatalf("ListPeers() workspaceID = %q, want %q", workspaceID, networkUDSTestWorkspaceID)
				}
				if channel != "builders" {
					t.Fatalf("ListPeers() channel = %q, want builders", channel)
				}
				return []network.PeerInfo{
					{
						PeerID:    "coder.sess-abc",
						Channel:   "builders",
						Local:     true,
						SessionID: &localSessionID,
					},
					{
						PeerID:    "reviewer.sess-xyz",
						Channel:   "builders",
						SessionID: &remoteSessionID,
					},
				}, nil
			},
		}

		wantDirectID, wantSessionA, wantSessionB, err := store.NetworkDirectRoomIdentity(
			networkUDSTestWorkspaceID,
			"builders",
			localSessionID,
			remoteSessionID,
		)
		if err != nil {
			t.Fatalf("NetworkDirectRoomIdentity() error = %v", err)
		}
		handlers.NetworkStore = testutil.StubNetworkStore{
			ResolveDirectRoomFn: func(
				_ context.Context,
				entry store.NetworkDirectRoomEntry,
			) (store.NetworkDirectRoomSummary, error) {
				if entry.Channel != "builders" ||
					entry.DirectID != wantDirectID ||
					entry.SessionA != wantSessionA ||
					entry.SessionB != wantSessionB {
					t.Fatalf("ResolveDirectRoom() entry = %#v, want deterministic direct-room identity", entry)
				}
				return store.NetworkDirectRoomSummary{
					Channel:        entry.Channel,
					DirectID:       entry.DirectID,
					SessionA:       entry.SessionA,
					SessionB:       entry.SessionB,
					OpenedAt:       time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
					LastActivityAt: time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC),
				}, nil
			},
		}

		engine := newTestRouter(t, handlers)
		resp := performRequest(
			t,
			engine,
			http.MethodPost,
			"/api/workspaces/ws-workspace/network/channels/builders/directs/resolve",
			[]byte(`{"session_id":"sess-local","peer_id":"reviewer.sess-xyz"}`),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("direct resolve status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload contract.NetworkDirectRoomResponse
		decodeJSONResponse(t, resp, &payload)
		if payload.Direct.DirectID != wantDirectID ||
			payload.Direct.SessionA != wantSessionA ||
			payload.Direct.SessionB != wantSessionB {
			t.Fatalf("direct resolve payload = %#v, want deterministic room", payload.Direct)
		}
	})
}

func TestNetworkHandlersExposeTypedCapabilityPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should expose typed capability payloads", func(t *testing.T) {
		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true
		handlers.Network = stubNetworkService{
			ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
				if workspaceID != networkUDSTestWorkspaceID {
					t.Fatalf("ListPeers() workspaceID = %q, want %q", workspaceID, networkUDSTestWorkspaceID)
				}
				if channel != "" {
					t.Fatalf("ListPeers() channel = %q, want empty", channel)
				}
				return []network.PeerInfo{{
					PeerID:  "reviewer.sess-a",
					Channel: "builders",
					Local:   true,
					PeerCard: network.PeerCard{
						PeerID:              "reviewer.sess-a",
						Capabilities:        []string{"review-pr"},
						ProfilesSupported:   []string{network.ProtocolV0},
						ArtifactsSupported:  []string{"capability"},
						TrustModesSupported: []string{"untrusted"},
						Ext: network.ExtensionMap{
							"agh.capabilities_brief": json.RawMessage(
								`[{"id":"review-pr","summary":"Review pull requests"}]`,
							),
							"agh.workflow_id": json.RawMessage(`"wf-1"`),
						},
					},
				}}, nil
			},
		}
		engine := newTestRouter(t, handlers)

		resp := performRequest(t, engine, http.MethodGet, "/api/workspaces/ws-workspace/network/peers", nil)
		if resp.Code != http.StatusOK {
			t.Fatalf("peers status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload contract.NetworkPeersResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
			t.Fatalf("json.Unmarshal(peers) error = %v", err)
		}
		if got, want := payload.Peers[0].PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{{
			ID:      "review-pr",
			Summary: "Review pull requests",
		}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("peer capabilities = %#v, want %#v", got, want)
		}
		if _, ok := payload.Peers[0].PeerCard.Ext["agh.capabilities_brief"]; ok {
			t.Fatalf("capability brief ext should be stripped: %#v", payload.Peers[0].PeerCard.Ext)
		}
		if got, want := string(payload.Peers[0].PeerCard.Ext["agh.workflow_id"]), `"wf-1"`; got != want {
			t.Fatalf("workflow ext = %q, want %q", got, want)
		}
	})
}
