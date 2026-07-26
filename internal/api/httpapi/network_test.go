package httpapi

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	apispec "github.com/compozy/agh/internal/api/spec"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

const networkHTTPTestWorkspaceID = "ws-workspace"

func TestNetworkDirectResolveCreatesRoom(t *testing.T) {
	t.Parallel()

	t.Run("Should create deterministic direct room", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true

		localSessionID := "sess-local"
		remoteSessionID := "sess-remote"
		handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
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
			networkHTTPTestWorkspaceID,
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

func TestNetworkPeerMessagesPreserveConversationRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve routing metadata in timeline payloads", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true
		handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, ""; got != want {
					t.Fatalf("ListPeers() channel = %q, want %q", got, want)
				}
				return []network.PeerInfo{{
					PeerID:  "reviewer.sess-remote",
					Channel: "builders",
				}}, nil
			},
		}
		handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.PeerID, "reviewer.sess-remote"; got != want {
					t.Fatalf("ListNetworkMessages() peer_id = %q, want %q", got, want)
				}
				return []store.NetworkMessageEntry{{
					MessageID: "msg-direct-01",
					Channel:   "builders",
					Surface:   "direct",
					DirectID:  "direct_test_01",
					Kind:      "say",
					Direction: "sent",
					PeerFrom:  "coder.sess-local",
					PeerTo:    "reviewer.sess-remote",
					Text:      "hello",
					Body:      []byte(`{"text":"hello"}`),
					Timestamp: time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC),
				}}, nil
			},
		}

		engine := gin.New()
		engine.GET("/api/workspaces/:workspace_id/network/peers/:peer_id/messages", handlers.NetworkPeerMessages)

		resp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("peer messages status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload contract.NetworkPeerMessagesResponse
		decodeJSONResponse(t, resp, &payload)
		if got, want := len(payload.Messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := payload.Messages[0].Surface, "direct"; got != want {
			t.Fatalf("messages[0].Surface = %q, want %q", got, want)
		}
		if got, want := payload.Messages[0].DirectID, "direct_test_01"; got != want {
			t.Fatalf("messages[0].DirectID = %q, want %q", got, want)
		}
	})
}

func TestNetworkPeerMessagesKeepPresenceEpisodesScopedByRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should keep peer presence episodes separate across routing containers", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 12, 12, 0, 0, 0, time.UTC)
		homePaths := newTestHomePaths(t)
		handlers := newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths)
		handlers.Config.Network.Enabled = true
		handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, ""; got != want {
					t.Fatalf("ListPeers() channel = %q, want %q", got, want)
				}
				return []network.PeerInfo{{
					PeerID:  "reviewer.sess-remote",
					Channel: "builders",
				}}, nil
			},
		}
		handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.PeerID, "reviewer.sess-remote"; got != want {
					t.Fatalf("ListNetworkMessages() peer_id = %q, want %q", got, want)
				}
				return []store.NetworkMessageEntry{
					{
						MessageID: "msg-thread-01",
						Channel:   "builders",
						Surface:   "thread",
						ThreadID:  "thread_alpha",
						Kind:      "greet",
						Direction: "received",
						PeerFrom:  "reviewer.sess-remote",
						Body: []byte(
							`{"peer_id":"reviewer.sess-remote","display_name":"Reviewer","capability_summary":"Review pull requests","summary":""}`,
						),
						Timestamp: recordedAt,
					},
					{
						MessageID: "msg-thread-02",
						Channel:   "builders",
						Surface:   "thread",
						ThreadID:  "thread_alpha",
						Kind:      "greet",
						Direction: "received",
						PeerFrom:  "reviewer.sess-remote",
						Body: []byte(
							`{"peer_id":"reviewer.sess-remote","display_name":"Reviewer","capability_summary":"Review pull requests","summary":""}`,
						),
						Timestamp: recordedAt.Add(20 * time.Second),
					},
					{
						MessageID: "msg-direct-01",
						Channel:   "builders",
						Surface:   "direct",
						DirectID:  "direct_99401d24bee62651d189e5a561785466",
						Kind:      "greet",
						Direction: "received",
						PeerFrom:  "reviewer.sess-remote",
						Body: []byte(
							`{"peer_id":"reviewer.sess-remote","display_name":"Reviewer","capability_summary":"Review pull requests","summary":""}`,
						),
						Timestamp: recordedAt.Add(25 * time.Second),
					},
				}, nil
			},
		}

		engine := gin.New()
		engine.GET("/api/workspaces/:workspace_id/network/peers/:peer_id/messages", handlers.NetworkPeerMessages)
		resp := performRequest(
			t,
			engine,
			http.MethodGet,
			"/api/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("peer messages status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}

		var payload contract.NetworkPeerMessagesResponse
		decodeJSONResponse(t, resp, &payload)
		if got, want := len(payload.Messages), 3; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}

		for index, messageID := range []string{"msg-thread-01", "msg-thread-02", "msg-direct-01"} {
			if got := payload.Messages[index].MessageID; got != messageID {
				t.Fatalf("messages[%d].message_id = %q, want %q", index, got, messageID)
			}
		}

		directEpisode := payload.Messages[2]
		if got, want := directEpisode.MessageID, "msg-direct-01"; got != want {
			t.Fatalf("direct episode message_id = %q, want %q", got, want)
		}
		if got, want := directEpisode.Surface, "direct"; got != want {
			t.Fatalf("direct episode surface = %q, want %q", got, want)
		}
		if got, want := directEpisode.DirectID, "direct_99401d24bee62651d189e5a561785466"; got != want {
			t.Fatalf("direct episode direct_id = %q, want %q", got, want)
		}
		if got := directEpisode.ThreadID; got != "" {
			t.Fatalf("direct episode thread_id = %q, want empty", got)
		}
	})
}

func TestRegisterNetworkRoutesMatchDocumentedHTTPSurface(t *testing.T) {
	t.Parallel()

	t.Run("Should match documented HTTP network routes", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		engine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

		got := registeredNetworkRoutesFromEngine(engine.Routes())
		want := documentedNetworkRoutesForTransport(apispec.TransportHTTP)
		for _, route := range []string{
			"GET /api/workspaces/:workspace_id/network/peers",
			"POST /api/workspaces/:workspace_id/network/send",
		} {
			if !slices.Contains(got, route) {
				t.Fatalf("registered network routes missing %q in %v", route, got)
			}
			if !slices.Contains(want, route) {
				t.Fatalf("documented network routes missing %q in %v", route, want)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("network routes = %v, want documented %s routes %v", got, apispec.TransportHTTP, want)
		}
	})
}

func registeredNetworkRoutesFromEngine(routes gin.RoutesInfo) []string {
	filtered := make([]string, 0)
	for _, route := range routes {
		if strings.HasPrefix(route.Path, "/api/network") ||
			strings.HasPrefix(route.Path, "/api/workspaces/:workspace_id/network") {
			filtered = append(filtered, route.Method+" "+route.Path)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func documentedNetworkRoutesForTransport(transport apispec.Transport) []string {
	routes := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if !slices.Contains(operation.Transports, transport) {
			continue
		}
		if !strings.HasPrefix(operation.Path, "/api/network") &&
			!strings.HasPrefix(operation.Path, "/api/workspaces/{workspace_id}/network") {
			continue
		}
		routes = append(routes, operation.Method+" "+normalizeNetworkSpecRoutePath(operation.Path))
	}
	sort.Strings(routes)
	return routes
}

func normalizeNetworkSpecRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[i] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}
