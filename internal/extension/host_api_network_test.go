package extensionpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	apicontract "github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	extensioncontract "github.com/compozy/agh/internal/extension/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

const hostAPINetworkWorkspaceID = "ws-host-network"

func TestHostAPIHandlerNetworkMethodsShouldRejectMissingCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		method   string
		params   json.RawMessage
		security []string
	}{
		{
			name:     "ShouldRejectReadMethodWithoutNetworkRead",
			method:   string(extensioncontract.HostAPIMethodNetworkStatus),
			params:   json.RawMessage(`{}`),
			security: []string{"network.write"},
		},
		{
			name:     "ShouldRejectUsageWithoutNetworkRead",
			method:   string(extensioncontract.HostAPIMethodNetworkUsage),
			params:   json.RawMessage(`{"workspace_id":"ws-host-network"}`),
			security: []string{"network.write"},
		},
		{
			name:   "ShouldRejectSendWithoutNetworkWrite",
			method: string(extensioncontract.HostAPIMethodNetworkSend),
			params: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"body":{"text":"hello"}
			}`),
			security: []string{"network.read"},
		},
		{
			name:   "ShouldRejectDirectResolveWithoutNetworkWrite",
			method: string(extensioncontract.HostAPIMethodNetworkDirectResolve),
			params: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"channel":"builders",
				"session_id":"sess-local",
				"peer_id":"peer.remote"
			}`),
			security: []string{"network.read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, checker := newHostAPINetworkTestHandler(t, &hostAPINetworkServiceStub{}, nil)
			checker.Register("ext-network", SourceUser, &Manifest{
				Actions:  ActionsConfig{Requires: []string{tt.method}},
				Security: SecurityConfig{Capabilities: append([]string(nil), tt.security...)},
			})

			_, err := handler.Handle(testutil.Context(t), "ext-network", tt.method, tt.params)
			assertCapabilityDenied(t, err, tt.method)
		})
	}
}

func TestHostAPIHandlerNetworkSendShouldPreservePublicValidationParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		payload  json.RawMessage
		fragment string
		secret   string
	}{
		{
			name: "ShouldRejectLegacyInteractionID",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"interaction_id":"old-work",
				"body":{"text":"hello"}
			}`),
			fragment: "interaction_id",
		},
		{
			name: "ShouldRejectDirectKind",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"direct",
				"direct_id":"direct_0123456789abcdef0123456789abcdef",
				"kind":"direct",
				"body":{"text":"hello"}
			}`),
			fragment: "kind direct",
		},
		{
			name: "ShouldRejectRawClaimTokenFields",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"body":{"claim_token":"agh_claim_secret"}
			}`),
			fragment: "raw claim_token",
			secret:   "agh_claim_secret",
		},
		{
			name: "ShouldRejectCallerSuppliedVerifiedFormatIdentity",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"from":"alice@39f713d0a644253f04529421b9f51b9b",
				"body":{"text":"hello"}
			}`),
			fragment: "sender identity and proof are daemon-derived",
		},
		{
			name: "ShouldRejectConversationFieldsWithoutSurface",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"body":{"text":"hello"}
			}`),
			fragment: "surface is required",
		},
		{
			name: "ShouldRejectGreetWithConversationFields",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"greet",
				"body":{"summary":"hello"}
			}`),
			fragment: "cannot carry conversation",
		},
		{
			name: "ShouldRejectCapabilityWithoutWorkID",
			payload: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"capability",
				"body":{"id":"cap.review"}
			}`),
			fragment: "work_id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &hostAPINetworkServiceStub{}
			handler, _ := newAuthorizedHostAPINetworkHandler(
				t,
				service,
				nil,
				[]string{string(extensioncontract.HostAPIMethodNetworkSend)},
				[]string{"network.write"},
			)

			_, err := handler.Handle(
				testutil.Context(t),
				"ext-network",
				string(extensioncontract.HostAPIMethodNetworkSend),
				tt.payload,
			)
			assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
			assertErrorContains(t, err, tt.fragment)
			if tt.secret != "" && strings.Contains(err.Error(), tt.secret) {
				t.Fatalf("Host API validation error leaked raw credential: %v", err)
			}
			if got := len(service.sentRequests()); got != 0 {
				t.Fatalf("Network.Send calls = %d, want 0 for invalid Host API send", got)
			}
		})
	}
}

func TestHostAPIHandlerNetworkSendShouldRejectLocalSession(t *testing.T) {
	t.Parallel()

	t.Run("Should return not_participating before dispatching a Local send", func(t *testing.T) {
		t.Parallel()

		service := &hostAPINetworkServiceStub{}
		handler, checker := newHostAPINetworkTestHandler(t, service, nil)
		handler.sessions = promptSessionManagerStub{
			statusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{
					ID:                   "sess-local",
					WorkspaceID:          hostAPINetworkWorkspaceID,
					NetworkParticipation: participation.LocalSpec(),
				}, nil
			},
		}
		checker.Register("ext-network", SourceUser, &Manifest{
			Actions:  ActionsConfig{Requires: []string{string(extensioncontract.HostAPIMethodNetworkSend)}},
			Security: SecurityConfig{Capabilities: []string{"network.write"}},
		})

		_, err := handler.Handle(
			t.Context(),
			"ext-network",
			string(extensioncontract.HostAPIMethodNetworkSend),
			json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
				"surface":"thread",
				"thread_id":"thread_alpha01",
				"kind":"say",
				"body":{"text":"hello"}
			}`),
		)
		assertRPCErrorCode(t, err, HostAPIUnavailableCode)
		assertErrorContains(t, err, participation.ErrNotParticipating.Error())
		if got := len(service.sentRequests()); got != 0 {
			t.Fatalf("Network.Send calls = %d, want 0", got)
		}
	})
}

func TestHostAPIHandlerNetworkSendShouldForwardValidPayload(t *testing.T) {
	t.Parallel()

	service := &hostAPINetworkServiceStub{sendID: "msg-host-api"}
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		service,
		nil,
		[]string{string(extensioncontract.HostAPIMethodNetworkSend)},
		[]string{"network.write"},
	)

	result, err := handler.Handle(
		testutil.Context(t),
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkSend),
		json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
			"surface":"thread",
			"thread_id":"thread_alpha01",
			"kind":"say",
			"work_id":"work-alpha",
			"body":{"text":"hello"}
		}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/send) error = %v, want nil", err)
	}

	var payload apicontract.NetworkSendPayload
	decodeResult(t, result, &payload)
	if payload.ID != "msg-host-api" {
		t.Fatalf("network/send id = %q, want msg-host-api", payload.ID)
	}
	sent := service.sentRequests()
	if len(sent) != 1 {
		t.Fatalf("network.Send calls = %d, want 1", len(sent))
	}
	if sent[0].ThreadID == nil || *sent[0].ThreadID != "thread_alpha01" {
		t.Fatalf("sent ThreadID = %#v, want thread_alpha01", sent[0].ThreadID)
	}
}

func TestHostAPIHandlerNetworkSendShouldForwardOptionalMetadata(t *testing.T) {
	t.Parallel()

	expiresAt := int64(1900000000)
	service := &hostAPINetworkServiceStub{sendID: "msg-direct-receipt"}
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		service,
		nil,
		[]string{string(extensioncontract.HostAPIMethodNetworkSend)},
		[]string{"network.write"},
	)

	result, err := handler.Handle(
		testutil.Context(t),
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkSend),
		json.RawMessage(fmt.Sprintf(`{
				"id":"msg-client",
				"workspace_id":"ws-host-network",
				"session_id":"sess-local",
				"channel":"builders",
			"surface":"direct",
			"direct_id":"direct_0123456789abcdef0123456789abcdef",
			"kind":"receipt",
			"to":"peer.remote",
			"mentions":[" reviewer.sess-peer ","","observer.sess-peer"],
			"work_id":"work-alpha",
			"reply_to":"msg-parent",
			"trace_id":"trace-alpha",
			"causation_id":"cause-alpha",
			"expires_at":%d,
			"body":{"status":"accepted"},
			"ext":{"safe":{"note":"ok"}}
		}`, expiresAt)),
	)
	if err != nil {
		t.Fatalf("Handle(network/send optional) error = %v, want nil", err)
	}

	var payload apicontract.NetworkSendPayload
	decodeResult(t, result, &payload)
	if payload.ID != "msg-direct-receipt" || payload.ExpiresAt == nil || *payload.ExpiresAt != expiresAt {
		t.Fatalf("network/send optional payload = %#v, want assigned id and expires_at", payload)
	}
	if got, want := strings.Join(payload.Mentions, ","), "reviewer.sess-peer,observer.sess-peer"; got != want {
		t.Fatalf("network/send payload mentions = %q, want %q", got, want)
	}
	sent := service.sentRequests()
	if len(sent) != 1 {
		t.Fatalf("network.Send calls = %d, want 1", len(sent))
	}
	req := sent[0]
	if req.DirectID == nil || *req.DirectID != "direct_0123456789abcdef0123456789abcdef" ||
		req.WorkID == nil || *req.WorkID != "work-alpha" ||
		req.To == nil || *req.To != "peer.remote" ||
		req.ExpiresAt == nil || *req.ExpiresAt != expiresAt ||
		req.ID == nil || *req.ID != "msg-client" ||
		len(req.Ext) != 1 {
		t.Fatalf("network.Send request = %#v, want optional metadata forwarded", req)
	}
	if got, want := strings.Join(req.Mentions, ","), "reviewer.sess-peer,observer.sess-peer"; got != want {
		t.Fatalf("network.Send request mentions = %q, want %q", got, want)
	}
	req.Mentions[0] = "mutated-after-dispatch"
	if payload.Mentions[0] != "reviewer.sess-peer" {
		t.Fatalf("network/send payload mentions alias runtime request: %#v", payload.Mentions)
	}
}

func TestHostAPIHandlerNetworkReadMethodsShouldUseRuntimeAndStore(t *testing.T) {
	t.Parallel()

	storeDB := openHostAPINetworkTestStore(t)
	baseTime := time.Date(2026, 4, 10, 20, 0, 0, 0, time.UTC)
	_, err := storeDB.WriteConversationMessage(testutil.Context(t), store.NetworkConversationMessage{
		MessageID:   "msg-thread-root",
		SessionID:   "sess-local",
		WorkspaceID: hostAPINetworkWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    "thread_alpha01",
		Direction:   "sent",
		PeerFrom:    "agent.local",
		PeerTo:      "sess-remote",
		Kind:        store.NetworkKindSay,
		WorkID:      "work-alpha",
		Text:        "hello thread",
		PreviewText: "thread preview",
		Body:        json.RawMessage(`{"text":"hello thread"}`),
		Timestamp:   baseTime,
	})
	if err != nil {
		t.Fatalf("WriteConversationMessage(thread) error = %v", err)
	}
	directID, _, _, err := store.NetworkDirectRoomIdentity(
		hostAPINetworkWorkspaceID,
		"builders",
		"sess-local",
		"sess-remote",
	)
	if err != nil {
		t.Fatalf("DirectRoomIdentity() error = %v", err)
	}
	_, err = storeDB.WriteConversationMessage(testutil.Context(t), store.NetworkConversationMessage{
		MessageID:   "msg-direct-one",
		SessionID:   "sess-local",
		WorkspaceID: hostAPINetworkWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceDirect,
		DirectID:    directID,
		Direction:   "sent",
		PeerFrom:    "sess-local",
		PeerTo:      "sess-remote",
		Kind:        store.NetworkKindSay,
		Text:        "hello direct",
		PreviewText: "direct preview",
		Body:        json.RawMessage(`{"text":"hello direct"}`),
		Timestamp:   baseTime.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("WriteConversationMessage(direct) error = %v", err)
	}

	displayName := "Remote Agent"
	joinedAt := baseTime.Add(-time.Hour)
	service := &hostAPINetworkServiceStub{
		status: &network.Status{
			Enabled:         true,
			Status:          network.StatusActive,
			Channels:        2,
			OpenThreads:     1,
			OpenDirectRooms: 1,
			KindMetrics: []network.KindMetric{
				{Kind: network.KindSay, Sent: 2, Delivered: 2},
			},
		},
		channels: []network.ChannelInfo{
			{Channel: "ops", PeerCount: 1},
			{Channel: "builders", PeerCount: 2},
		},
		peers: []network.PeerInfo{
			{
				SessionID: hostAPINetworkStringPtr("sess-local"),
				PeerID:    "agent.local",
				Channel:   "builders",
				Local:     true,
				PeerCard:  network.PeerCard{PeerID: "agent.local"},
				JoinedAt:  &joinedAt,
			},
			{
				SessionID: hostAPINetworkStringPtr("sess-remote"),
				PeerID:    "peer.remote",
				Channel:   "builders",
				Local:     true,
				PeerCard: network.PeerCard{
					PeerID:              "peer.remote",
					DisplayName:         &displayName,
					Capabilities:        []string{"cap.review"},
					ProfilesSupported:   []string{"default"},
					ArtifactsSupported:  []string{"patch"},
					TrustModesSupported: []string{"signed"},
				},
				JoinedAt: &baseTime,
			},
		},
	}
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		service,
		storeDB,
		[]string{
			string(extensioncontract.HostAPIMethodNetworkStatus),
			string(extensioncontract.HostAPIMethodNetworkUsage),
			string(extensioncontract.HostAPIMethodNetworkChannels),
			string(extensioncontract.HostAPIMethodNetworkPeers),
			string(extensioncontract.HostAPIMethodNetworkThreads),
			string(extensioncontract.HostAPIMethodNetworkThreadGet),
			string(extensioncontract.HostAPIMethodNetworkDirects),
			string(extensioncontract.HostAPIMethodNetworkDirectMessages),
			string(extensioncontract.HostAPIMethodNetworkWorkGet),
		},
		[]string{"network.read"},
	)

	ctx := testutil.Context(t)
	statusResult, err := handler.Handle(ctx, "ext-network", string(extensioncontract.HostAPIMethodNetworkStatus), nil)
	if err != nil {
		t.Fatalf("Handle(network/status) error = %v, want nil", err)
	}
	var status apicontract.NetworkStatusPayload
	decodeResult(t, statusResult, &status)
	if status.Status != network.StatusActive || len(status.KindMetrics) != 1 {
		t.Fatalf("network/status = %#v, want active status with kind metrics", status)
	}

	usageResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkUsage),
		json.RawMessage(`{"workspace_id":"ws-host-network","owner_kind":"task_run","owner_id":"run-1","limit":25}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/usage) error = %v, want nil", err)
	}
	var usage apicontract.NetworkUsageResponse
	decodeResult(t, usageResult, &usage)
	if usage.WorkspaceID != hostAPINetworkWorkspaceID || usage.Total.WakeCount != 0 {
		t.Fatalf("network/usage = %#v, want empty workspace-scoped usage", usage)
	}

	channelsResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkChannels),
		json.RawMessage(`{"workspace_id":"ws-host-network"}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/channels) error = %v, want nil", err)
	}
	var channels []apicontract.NetworkChannelPayload
	decodeResult(t, channelsResult, &channels)
	if len(channels) != 2 || channels[0].Channel != "builders" || channels[1].Channel != "ops" {
		t.Fatalf("network/channels = %#v, want sorted channel payloads", channels)
	}

	peersResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkPeers),
		json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders"}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/peers) error = %v, want nil", err)
	}
	var peers []apicontract.NetworkPeerPayload
	decodeResult(t, peersResult, &peers)
	if len(peers) != 2 || !peers[0].Local || !peers[1].Local || peers[1].DisplayName != "Remote Agent" {
		t.Fatalf("network/peers = %#v, want two local peers with display names", peers)
	}

	threadsResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkThreads),
		json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders","limit":10}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/threads) error = %v, want nil", err)
	}
	var threads apicontract.NetworkThreadsResponse
	decodeResult(t, threadsResult, &threads)
	if len(threads.Threads) != 1 || threads.Threads[0].ThreadID != "thread_alpha01" ||
		threads.Threads[0].OpenWorkCount != 1 || threads.Page.Total != 1 || threads.Page.Limit != 10 {
		t.Fatalf("network/threads = %#v, want counted thread page with open work", threads)
	}

	threadResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkThreadGet),
		json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders","thread_id":"thread_alpha01"}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/thread/get) error = %v, want nil", err)
	}
	var thread apicontract.NetworkThreadSummaryPayload
	decodeResult(t, threadResult, &thread)
	if thread.RootMessageID != "msg-thread-root" {
		t.Fatalf("network/thread/get root = %q, want msg-thread-root", thread.RootMessageID)
	}

	directsResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkDirects),
		json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders","peer_id":"peer.remote","limit":10}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/directs) error = %v, want nil", err)
	}
	var directs apicontract.NetworkDirectRoomsResponse
	decodeResult(t, directsResult, &directs)
	if len(directs.Directs) != 1 || directs.Directs[0].DirectID != directID ||
		directs.Directs[0].MessageCount != 1 || directs.Page.Total != 1 || directs.Page.Limit != 10 {
		t.Fatalf("network/directs = %#v, want one counted direct summary page", directs)
	}

	directMessagesResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkDirectMessages),
		json.RawMessage(
			fmt.Sprintf(`{"workspace_id":"ws-host-network","channel":"builders","direct_id":%q,"limit":10}`, directID),
		),
	)
	if err != nil {
		t.Fatalf("Handle(network/direct/messages) error = %v, want nil", err)
	}
	var directMessages apicontract.NetworkDirectRoomMessagesResponse
	decodeResult(t, directMessagesResult, &directMessages)
	if len(directMessages.Messages) != 1 || directMessages.Messages[0].DirectID != directID ||
		directMessages.Page.Limit != 10 {
		t.Fatalf("network/direct/messages = %#v, want direct timeline message", directMessages)
	}

	workResult, err := handler.Handle(
		ctx,
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkWorkGet),
		json.RawMessage(`{"workspace_id":"ws-host-network","work_id":"work-alpha"}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/work/get) error = %v, want nil", err)
	}
	var work apicontract.NetworkWorkPayload
	decodeResult(t, workResult, &work)
	if work.WorkID != "work-alpha" || work.ThreadID != "thread_alpha01" ||
		work.State != store.NetworkWorkStateSubmitted {
		t.Fatalf("network/work/get = %#v, want submitted thread work", work)
	}
}

func TestHostAPIHandlerNetworkMethodsShouldRejectInvalidReadParams(t *testing.T) {
	t.Parallel()

	storeDB := openHostAPINetworkTestStore(t)
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		&hostAPINetworkServiceStub{},
		storeDB,
		[]string{
			string(extensioncontract.HostAPIMethodNetworkPeers),
			string(extensioncontract.HostAPIMethodNetworkUsage),
			string(extensioncontract.HostAPIMethodNetworkThreads),
			string(extensioncontract.HostAPIMethodNetworkThreadGet),
			string(extensioncontract.HostAPIMethodNetworkThreadMessages),
			string(extensioncontract.HostAPIMethodNetworkDirects),
			string(extensioncontract.HostAPIMethodNetworkDirectMessages),
			string(extensioncontract.HostAPIMethodNetworkWorkGet),
		},
		[]string{"network.read"},
	)

	tests := []struct {
		name   string
		method string
		params json.RawMessage
	}{
		{
			name:   "ShouldRejectUsageLimit",
			method: string(extensioncontract.HostAPIMethodNetworkUsage),
			params: json.RawMessage(`{"workspace_id":"ws-host-network","limit":0}`),
		},
		{
			name:   "ShouldRejectUsageTrailingJSON",
			method: string(extensioncontract.HostAPIMethodNetworkUsage),
			params: json.RawMessage(`{"workspace_id":"ws-host-network"}{"limit":25}`),
		},
		{
			name:   "ShouldRejectPeerChannelTraversal",
			method: string(extensioncontract.HostAPIMethodNetworkPeers),
			params: json.RawMessage(`{"workspace_id":"ws-host-network","channel":"bad/channel"}`),
		},
		{
			name:   "ShouldRejectThreadsLimit",
			method: string(extensioncontract.HostAPIMethodNetworkThreads),
			params: json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders","limit":-1}`),
		},
		{
			name:   "ShouldRejectThreadID",
			method: string(extensioncontract.HostAPIMethodNetworkThreadGet),
			params: json.RawMessage(
				`{"workspace_id":"ws-host-network","channel":"builders","thread_id":"provider-thread"}`,
			),
		},
		{
			name:   "ShouldRejectConflictingMessageCursors",
			method: string(extensioncontract.HostAPIMethodNetworkThreadMessages),
			params: json.RawMessage(`{
				"workspace_id":"ws-host-network",
				"channel":"builders",
				"thread_id":"thread_alpha01",
				"before":"msg-before",
				"after":"msg-after"
			}`),
		},
		{
			name:   "ShouldRejectDirectsLimit",
			method: string(extensioncontract.HostAPIMethodNetworkDirects),
			params: json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders","limit":-1}`),
		},
		{
			name:   "ShouldRejectDirectID",
			method: string(extensioncontract.HostAPIMethodNetworkDirectMessages),
			params: json.RawMessage(
				`{"workspace_id":"ws-host-network","channel":"builders","direct_id":"thread_alpha01"}`,
			),
		},
		{
			name:   "ShouldRejectWorkID",
			method: string(extensioncontract.HostAPIMethodNetworkWorkGet),
			params: json.RawMessage(`{"workspace_id":"ws-host-network","work_id":"bad/work"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := handler.Handle(testutil.Context(t), "ext-network", tt.method, tt.params)
			assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
		})
	}
}

func TestHostAPIHandlerNetworkMethodsShouldRejectMissingDependencies(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectRuntimeMethodWithoutNetworkService", func(t *testing.T) {
		t.Parallel()

		handler, _ := newAuthorizedHostAPINetworkHandler(
			t,
			nil,
			nil,
			[]string{string(extensioncontract.HostAPIMethodNetworkStatus)},
			[]string{"network.read"},
		)
		_, err := handler.Handle(
			testutil.Context(t),
			"ext-network",
			string(extensioncontract.HostAPIMethodNetworkStatus),
			nil,
		)
		assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	})

	t.Run("ShouldRejectStoreMethodWithoutNetworkStore", func(t *testing.T) {
		t.Parallel()

		handler, _ := newAuthorizedHostAPINetworkHandler(
			t,
			&hostAPINetworkServiceStub{},
			nil,
			[]string{string(extensioncontract.HostAPIMethodNetworkThreads)},
			[]string{"network.read"},
		)
		_, err := handler.Handle(
			testutil.Context(t),
			"ext-network",
			string(extensioncontract.HostAPIMethodNetworkThreads),
			json.RawMessage(`{"workspace_id":"ws-host-network","channel":"builders"}`),
		)
		assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	})

	t.Run("ShouldRejectUsageWithoutUsageStore", func(t *testing.T) {
		t.Parallel()

		handler, _ := newAuthorizedHostAPINetworkHandler(
			t,
			&hostAPINetworkServiceStub{},
			nil,
			[]string{string(extensioncontract.HostAPIMethodNetworkUsage)},
			[]string{"network.read"},
		)
		_, err := handler.Handle(
			testutil.Context(t),
			"ext-network",
			string(extensioncontract.HostAPIMethodNetworkUsage),
			json.RawMessage(`{"workspace_id":"ws-host-network"}`),
		)
		assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	})
}

func TestHostAPIHandlerNetworkDirectResolveShouldBeIdempotentUnderRace(t *testing.T) {
	t.Parallel()

	storeDB := openHostAPINetworkTestStore(t)
	service := &hostAPINetworkServiceStub{
		peers: []network.PeerInfo{
			{
				SessionID: hostAPINetworkStringPtr("sess-local"),
				PeerID:    "agent.local",
				Channel:   "builders",
				Local:     true,
				PeerCard:  network.PeerCard{PeerID: "agent.local"},
			},
			{
				SessionID: hostAPINetworkStringPtr("sess-remote"),
				PeerID:    "peer.remote",
				Channel:   "builders",
				Local:     false,
				PeerCard:  network.PeerCard{PeerID: "peer.remote"},
			},
		},
	}
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		service,
		storeDB,
		[]string{string(extensioncontract.HostAPIMethodNetworkDirectResolve)},
		[]string{"network.write"},
	)

	const calls = 8
	results := make(chan string, calls)
	errs := make(chan error, calls)
	var wg sync.WaitGroup
	for range calls {
		wg.Go(func() {
			result, err := handler.Handle(
				testutil.Context(t),
				"ext-network",
				string(extensioncontract.HostAPIMethodNetworkDirectResolve),
				json.RawMessage(`{
					"workspace_id":"ws-host-network",
					"channel":"builders",
					"session_id":"sess-local",
					"peer_id":"peer.remote"
				}`),
			)
			if err != nil {
				errs <- err
				return
			}
			var payload apicontract.NetworkDirectRoomPayload
			decodeResult(t, result, &payload)
			results <- payload.DirectID
		})
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("Handle(network/direct/resolve concurrent) error = %v", err)
	}
	expectedID, _, _, err := store.NetworkDirectRoomIdentity(
		hostAPINetworkWorkspaceID,
		"builders",
		"sess-local",
		"sess-remote",
	)
	if err != nil {
		t.Fatalf("DirectRoomIdentity() error = %v", err)
	}
	for directID := range results {
		if directID != expectedID {
			t.Fatalf("direct_id = %q, want %q", directID, expectedID)
		}
	}

	directs, err := storeDB.ListDirectRooms(testutil.Context(t), store.NetworkChannelRef{
		WorkspaceID: hostAPINetworkWorkspaceID,
		Channel:     "builders",
	}, store.NetworkDirectRoomQuery{Limit: 10})
	if err != nil {
		t.Fatalf("ListDirectRooms() error = %v", err)
	}
	if len(directs.Directs) != 1 {
		t.Fatalf("ListDirectRooms() len = %d, want 1", len(directs.Directs))
	}
}

func TestHostAPIHandlerNetworkThreadMessagesShouldUseConversationStore(t *testing.T) {
	t.Parallel()

	storeDB := openHostAPINetworkTestStore(t)
	baseTime := time.Date(2026, 4, 10, 18, 30, 0, 0, time.UTC)
	_, err := storeDB.WriteConversationMessage(testutil.Context(t), store.NetworkConversationMessage{
		MessageID:   "msg-thread-root",
		SessionID:   "sess-local",
		WorkspaceID: hostAPINetworkWorkspaceID,
		Channel:     "builders",
		Surface:     store.NetworkSurfaceThread,
		ThreadID:    "thread_alpha01",
		Direction:   "sent",
		PeerFrom:    "agent.local",
		PeerTo:      "peer.remote",
		Kind:        store.NetworkKindSay,
		Text:        "hello thread",
		Body:        json.RawMessage(`{"text":"hello thread"}`),
		Timestamp:   baseTime,
	})
	if err != nil {
		t.Fatalf("WriteConversationMessage() error = %v", err)
	}
	handler, _ := newAuthorizedHostAPINetworkHandler(
		t,
		&hostAPINetworkServiceStub{},
		storeDB,
		[]string{string(extensioncontract.HostAPIMethodNetworkThreadMessages)},
		[]string{"network.read"},
	)

	result, err := handler.Handle(
		testutil.Context(t),
		"ext-network",
		string(extensioncontract.HostAPIMethodNetworkThreadMessages),
		json.RawMessage(`{
			"workspace_id":"ws-host-network",
			"channel":"builders",
			"thread_id":"thread_alpha01",
			"limit":10
		}`),
	)
	if err != nil {
		t.Fatalf("Handle(network/thread/messages) error = %v, want nil", err)
	}

	var messages apicontract.NetworkThreadMessagesResponse
	decodeResult(t, result, &messages)
	if len(messages.Messages) != 1 {
		t.Fatalf("network/thread/messages len = %d, want 1", len(messages.Messages))
	}
	if messages.Messages[0].MessageID != "msg-thread-root" || messages.Page.Limit != 10 {
		t.Fatalf("network/thread/messages = %#v, want message with applied limit", messages)
	}
}

func TestBridgeHostAPINetworkMetaShouldUseExplicitConversationMapping(t *testing.T) {
	t.Parallel()

	t.Run("ShouldKeepProviderThreadIDOutOfAGHNetworkMeta", func(t *testing.T) {
		t.Parallel()

		meta := bridgePromptNetworkMeta(bridgepkg.InboundMessageEnvelope{
			PlatformMessageID: "platform-msg-1",
			PeerID:            "provider-peer",
			ThreadID:          "provider-thread",
			EventFamily:       bridgepkg.InboundEventFamilyMessage,
		})
		if meta.ThreadID != "" {
			t.Fatalf("bridgePromptNetworkMeta().ThreadID = %q, want empty without explicit conversation", meta.ThreadID)
		}
	})

	t.Run("ShouldMapExplicitConversationRefToAGHNetworkMeta", func(t *testing.T) {
		t.Parallel()

		meta := bridgePromptNetworkMeta(bridgepkg.InboundMessageEnvelope{
			PlatformMessageID: "platform-msg-1",
			PeerID:            "provider-peer",
			ThreadID:          "provider-thread",
			EventFamily:       bridgepkg.InboundEventFamilyMessage,
			Conversation: &bridgepkg.NetworkConversationRef{
				Channel:  "builders",
				Surface:  bridgepkg.NetworkConversationSurfaceThread,
				ThreadID: "thread_alpha01",
				WorkID:   "work-alpha",
			},
		})
		if meta.ThreadID != "thread_alpha01" || meta.Channel != "builders" ||
			meta.Surface != string(bridgepkg.NetworkConversationSurfaceThread) || meta.WorkID != "work-alpha" {
			t.Fatalf("bridgePromptNetworkMeta() = %#v, want explicit AGH conversation mapping", meta)
		}
	})

	t.Run("ShouldUseEditedMessageIdentityForEditNetworkMeta", func(t *testing.T) {
		t.Parallel()

		meta := bridgePromptNetworkMeta(bridgepkg.InboundMessageEnvelope{
			EventFamily: bridgepkg.InboundEventFamilyEdit,
			Edit: &bridgepkg.InboundEdit{
				MessageID:         "platform-msg-edited",
				NewText:           "corrected",
				OriginalTimestamp: time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC),
				Operation:         bridgepkg.InboundEditOperationUpdated,
			},
		})
		if got, want := meta.MessageID, "platform-msg-edited"; got != want {
			t.Fatalf("bridgePromptNetworkMeta().MessageID = %q, want %q", got, want)
		}
		if got, want := meta.Kind, string(bridgepkg.InboundEventFamilyEdit); got != want {
			t.Fatalf("bridgePromptNetworkMeta().Kind = %q, want %q", got, want)
		}
	})
}

type hostAPINetworkServiceStub struct {
	mu       sync.Mutex
	status   *network.Status
	channels []network.ChannelInfo
	peers    []network.PeerInfo
	sendID   string
	sendErr  error
	sent     []network.SendRequest
}

func (s *hostAPINetworkServiceStub) Send(_ context.Context, req network.SendRequest) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, req)
	if s.sendErr != nil {
		return "", s.sendErr
	}
	if s.sendID != "" {
		return s.sendID, nil
	}
	return "msg-network-stub", nil
}

func (s *hostAPINetworkServiceStub) ListPeers(
	_ context.Context,
	workspaceID string,
	channel string,
) ([]network.PeerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	trimmedChannel := strings.TrimSpace(channel)
	peers := make([]network.PeerInfo, 0, len(s.peers))
	for _, peer := range s.peers {
		if trimmedWorkspaceID != "" && peer.WorkspaceID != "" && peer.WorkspaceID != trimmedWorkspaceID {
			continue
		}
		if trimmedChannel != "" && peer.Channel != trimmedChannel {
			continue
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func (s *hostAPINetworkServiceStub) ListChannels(_ context.Context, workspaceID string) ([]network.ChannelInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmedWorkspaceID := strings.TrimSpace(workspaceID)
	channels := make([]network.ChannelInfo, 0, len(s.channels))
	for _, channel := range s.channels {
		if trimmedWorkspaceID != "" && channel.WorkspaceID != "" && channel.WorkspaceID != trimmedWorkspaceID {
			continue
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func (s *hostAPINetworkServiceStub) Status(context.Context) (*network.Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == nil {
		return &network.Status{Enabled: true, Status: network.StatusActive}, nil
	}
	copyStatus := *s.status
	return &copyStatus, nil
}

func (s *hostAPINetworkServiceStub) sentRequests() []network.SendRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]network.SendRequest(nil), s.sent...)
}

func newAuthorizedHostAPINetworkHandler(
	t testing.TB,
	service hostAPINetworkService,
	networkStore store.NetworkConversationStore,
	actions []string,
	security []string,
) (*HostAPIHandler, *CapabilityChecker) {
	t.Helper()

	handler, checker := newHostAPINetworkTestHandler(t, service, networkStore)
	checker.Register("ext-network", SourceUser, &Manifest{
		Actions:  ActionsConfig{Requires: append([]string(nil), actions...)},
		Security: SecurityConfig{Capabilities: append([]string(nil), security...)},
	})
	return handler, checker
}

func newHostAPINetworkTestHandler(
	t testing.TB,
	service hostAPINetworkService,
	networkStore store.NetworkConversationStore,
) (*HostAPIHandler, *CapabilityChecker) {
	t.Helper()

	checker := &CapabilityChecker{}
	options := []HostAPIOption{
		WithHostAPICapabilityChecker(checker),
		WithHostAPINetworkService(service),
		WithHostAPIRateLimit(1000, 1000),
		WithHostAPINow(func() time.Time {
			return time.Date(2026, 4, 10, 19, 0, 0, 0, time.UTC)
		}),
	}
	if networkStore != nil {
		options = append(options, WithHostAPINetworkStore(networkStore))
		if usageStore, ok := networkStore.(store.NetworkUsageStore); ok {
			options = append(options, WithHostAPINetworkUsageStore(usageStore))
		}
	}
	sessions := promptSessionManagerStub{
		statusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			sessionID := strings.TrimSpace(id)
			if sessionID == "" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:                   sessionID,
				WorkspaceID:          hostAPINetworkWorkspaceID,
				NetworkParticipation: hostAPINetworkLiveSpec("builders"),
			}, nil
		},
	}
	return NewHostAPIHandler(sessions, nil, nil, nil, options...), checker
}

func hostAPINetworkLiveSpec(channel string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     hostAPINetworkWorkspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       strings.TrimSpace(channel),
		Source:          participation.SourceExplicitRequest,
		Bounds: participation.Bounds{
			MaxWakes: 4, MaxWakeWallTime: "1m", MaxTotalWallTime: "5m",
			MaxInputTokens: 1000, MaxOutputTokens: 1000, MaxWakeDepth: 2,
			CoalesceWindow: "1s",
		},
	}
}

func openHostAPINetworkTestStore(t testing.TB) *globaldb.GlobalDB {
	t.Helper()

	db, err := globaldb.OpenGlobalDB(testutil.Context(t), t.TempDir()+"/host-api-network.db")
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Errorf("GlobalDB.Close() error = %v", err)
		}
	})
	now := time.Date(2026, 4, 10, 17, 0, 0, 0, time.UTC)
	if err := db.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        hostAPINetworkWorkspaceID,
		RootDir:   t.TempDir(),
		Name:      "host-api-network",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	for _, sessionInfo := range []store.SessionInfo{
		{
			ID:                  "sess-local",
			AgentName:           "local",
			WorkspaceID:         hostAPINetworkWorkspaceID,
			SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
			SessionType:         "system",
			State:               "stopped",
			CreatedAt:           now,
			UpdatedAt:           now,
		},
		{
			ID:                  "sess-remote",
			AgentName:           "remote",
			WorkspaceID:         hostAPINetworkWorkspaceID,
			SessionNetworkState: &store.SessionNetworkState{NetworkSpec: participation.LocalSpec()},
			SessionType:         "system",
			State:               "stopped",
			CreatedAt:           now,
			UpdatedAt:           now,
		},
	} {
		if err := db.RegisterSession(testutil.Context(t), sessionInfo); err != nil {
			t.Fatalf("RegisterSession(%s) error = %v", sessionInfo.ID, err)
		}
	}
	if err := db.WriteNetworkChannel(testutil.Context(t), store.NetworkChannelEntry{
		WorkspaceID: hostAPINetworkWorkspaceID,
		Channel:     "builders",
		Purpose:     "Host API test coordination",
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatalf("WriteNetworkChannel() error = %v", err)
	}
	return db
}

func hostAPINetworkStringPtr(value string) *string {
	copyValue := value
	return &copyValue
}
