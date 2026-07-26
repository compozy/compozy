package cli

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
)

func TestNetworkCommandsAndFormatting(t *testing.T) {
	t.Parallel()

	t.Run("Should explain how the first public thread is opened", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
		stdout, _, err := executeRootCommand(t, deps, "network", "send", "--help")
		if err != nil {
			t.Fatalf("network send --help error = %v", err)
		}
		for _, phrase := range []string{
			"The first valid send opens a new public thread",
			"^thread_[a-z0-9][a-z0-9_-]{2,95}$",
			"required for capability and say with --work",
		} {
			if !strings.Contains(stdout, phrase) {
				t.Fatalf("network send --help output = %q, want phrase %q", stdout, phrase)
			}
		}
	})

	t.Run("Should format status peers channels send and inbox", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC).Unix()
		surfaceDirect := "direct"
		directID := "direct_99401d24bee62651d189e5a561785466"
		workID := "work_1"
		var seenPeersQuery NetworkPeersQuery
		var seenCreateRequest CreateNetworkChannelRequest
		var seenSendRequest NetworkSendRequest

		client := &stubClient{
			networkStatusFn: func(context.Context) (NetworkStatusRecord, error) {
				return NetworkStatusRecord{
					Enabled:              true,
					Status:               "active",
					LocalPeers:           1,
					Channels:             1,
					MessagesSent:         4,
					MessagesReceived:     5,
					MessagesRejected:     1,
					MessagesDelivered:    3,
					WorkflowTaggedEvents: 2,
					HandoffTaggedEvents:  1,
					KindMetrics: []NetworkKindMetricRecord{{
						Kind:      "say",
						Sent:      4,
						Received:  5,
						Rejected:  1,
						Delivered: 3,
					}},
				}, nil
			},
			networkPeersFn: func(_ context.Context, query NetworkPeersQuery) ([]NetworkPeerRecord, error) {
				seenPeersQuery = query
				displayName := "Reviewer"
				sessionID := "sess-a"
				return []NetworkPeerRecord{{
					PeerID:        "reviewer.sess-a",
					SessionID:     &sessionID,
					Channel:       "builders",
					Local:         true,
					PresenceState: contract.NetworkPresenceLocal,
					PeerCard: NetworkPeerCardRecord{
						PeerID:            "reviewer.sess-a",
						DisplayName:       &displayName,
						ProfilesSupported: []string{"v0"},
						Capabilities: []contract.NetworkCapabilityBriefPayload{{
							ID:      "send",
							Summary: "Send direct messages",
						}},
						ArtifactsSupported:  []string{"text"},
						TrustModesSupported: []string{"untrusted"},
					},
					JoinedAt: &fixedTestNow,
				}}, nil
			},
			networkChannelsFn: func(_ context.Context, workspaceRef string) ([]NetworkChannelRecord, error) {
				if workspaceRef != "ws-alpha" {
					t.Fatalf("NetworkChannels() workspace = %q, want ws-alpha", workspaceRef)
				}
				return []NetworkChannelRecord{{Channel: "builders", PeerCount: 2}}, nil
			},
			createNetworkChannelFn: func(
				_ context.Context,
				workspaceRef string,
				request CreateNetworkChannelRequest,
			) (NetworkChannelDetailRecord, error) {
				if workspaceRef != "ws-alpha" {
					t.Fatalf("CreateNetworkChannel() workspace = %q, want ws-alpha", workspaceRef)
				}
				seenCreateRequest = request
				return NetworkChannelDetailRecord{
					Channel:      request.Channel,
					WorkspaceID:  workspaceRef,
					Purpose:      request.Purpose,
					CreatedBy:    request.AgentNames[0],
					PeerCount:    len(request.AgentNames),
					SessionCount: len(request.AgentNames),
				}, nil
			},
			networkSendFn: func(_ context.Context, request NetworkSendRequest) (NetworkSendRecord, error) {
				seenSendRequest = request
				return NetworkSendRecord{
					ID:          "msg-1",
					SessionID:   request.SessionID,
					Channel:     request.Channel,
					Surface:     request.Surface,
					ThreadID:    request.ThreadID,
					DirectID:    request.DirectID,
					Kind:        request.Kind,
					WorkID:      request.WorkID,
					TraceID:     request.TraceID,
					CausationID: request.CausationID,
					ReplyTo:     request.ReplyTo,
					ExpiresAt:   request.ExpiresAt,
					Ext:         request.Ext,
				}, nil
			},
			networkInboxFn: func(_ context.Context, workspaceRef string, _ string) ([]NetworkEnvelopeRecord, error) {
				if workspaceRef != "ws-alpha" {
					t.Fatalf("NetworkInbox() workspace = %q, want ws-alpha", workspaceRef)
				}
				replyTo := "msg-root"
				traceID := "trace-1"
				causationID := "cause-1"
				return []NetworkEnvelopeRecord{{
					Protocol:    "agh-network/v0",
					ID:          "msg-inbox",
					Kind:        "say",
					Channel:     "builders",
					Surface:     &surfaceDirect,
					DirectID:    &directID,
					From:        "reviewer.sess-a",
					WorkID:      &workID,
					ReplyTo:     &replyTo,
					TraceID:     &traceID,
					CausationID: &causationID,
					TS:          fixedTestNow.Unix(),
					Body:        mustJSON(t, map[string]any{"text": "review this", "intent": "review"}),
					Ext: map[string]json.RawMessage{
						"agh.workflow_id":     mustJSON(t, "wf-1"),
						"agh.handoff_version": mustJSON(t, 3),
					},
				}}, nil
			},
		}
		deps := newTestDeps(t, client)

		statusOut, _, err := executeRootCommand(t, deps, "network", "--workspace", "ws-alpha", "status", "-o", "human")
		if err != nil {
			t.Fatalf("network status error = %v", err)
		}
		if !strings.Contains(statusOut, "Network") || !strings.Contains(statusOut, "Live Participants") ||
			!strings.Contains(statusOut, "Kind Metrics") {
			t.Fatalf("network status output = %q, want summary and metrics", statusOut)
		}

		peersOut, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"peers",
			"builders",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network peers error = %v", err)
		}
		if seenPeersQuery.WorkspaceRef != "ws-alpha" || seenPeersQuery.Channel != "builders" {
			t.Fatalf("seenPeersQuery = %#v, want ws-alpha/builders", seenPeersQuery)
		}
		var peers []NetworkPeerRecord
		if err := json.Unmarshal([]byte(peersOut), &peers); err != nil {
			t.Fatalf("json.Unmarshal(network peers) error = %v", err)
		}
		if len(peers) != 1 || peers[0].PeerID != "reviewer.sess-a" {
			t.Fatalf("peers = %#v, want one reviewer peer", peers)
		}
		if peers[0].PresenceState != contract.NetworkPresenceLocal {
			t.Fatalf("peer presence = %q, want local", peers[0].PresenceState)
		}

		peersHumanOut, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"peers",
			"builders",
			"-o",
			"human",
		)
		if err != nil {
			t.Fatalf("network peers human error = %v", err)
		}
		if !strings.Contains(peersHumanOut, "Presence") || !strings.Contains(peersHumanOut, "local") {
			t.Fatalf("network peers human output = %q, want local Presence", peersHumanOut)
		}

		channelsOut, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"channels",
			"-o",
			"toon",
		)
		if err != nil {
			t.Fatalf("network channels error = %v", err)
		}
		if !strings.Contains(channelsOut, "network_channels[1]{channel,peer_count}:") {
			t.Fatalf("network channels toon = %q, want TOON list", channelsOut)
		}

		createOut, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"channels",
			"create",
			"launch_ops",
			"--purpose",
			"Coordinate launch work",
			"--agent",
			"site_copywriter",
			"--agent",
			"growth_marketer",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network channels create error = %v", err)
		}
		if seenCreateRequest.Channel != "launch_ops" || seenCreateRequest.Purpose != "Coordinate launch work" ||
			!slices.Equal(seenCreateRequest.AgentNames, []string{"site_copywriter", "growth_marketer"}) {
			t.Fatalf("seenCreateRequest = %#v, want launch channel with two agents", seenCreateRequest)
		}
		var created NetworkChannelDetailRecord
		if err := json.Unmarshal([]byte(createOut), &created); err != nil {
			t.Fatalf("json.Unmarshal(network channels create) error = %v", err)
		}
		if created.Channel != "launch_ops" || created.SessionCount != 2 {
			t.Fatalf("created channel = %#v, want launch_ops with two sessions", created)
		}

		_, _, err = executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"channels",
			"create",
			"launch_ops",
			"--purpose",
			"   ",
			"--agent",
			"site_copywriter",
		)
		if err == nil || err.Error() != "cli: --purpose cannot be empty" {
			t.Fatalf("network channels create whitespace purpose error = %v, want cli purpose validation", err)
		}

		sendOut, _, err := executeRootCommand(t, deps,
			"network", "--workspace", "ws-alpha", "send",
			"--session", "sess-a",
			"--channel", "builders",
			"--surface", "thread",
			"--thread", "thread_123",
			"--kind", "say",
			"--to", "reviewer.sess-a",
			"--body", `{"text":"hello"}`,
			"--work", "work_1",
			"--reply-to", "msg-root",
			"--trace-id", "trace-1",
			"--causation-id", "cause-1",
			"--expires-at", "2026-04-11T18:00:00Z",
			"--ext", `{"agh.workflow_id":"wf-1","agh.handoff_version":3}`,
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("network send error = %v", err)
		}
		if string(seenSendRequest.Body) != `{"text":"hello"}` {
			t.Fatalf("seenSendRequest.Body = %s, want body JSON", string(seenSendRequest.Body))
		}
		if seenSendRequest.ExpiresAt == nil || *seenSendRequest.ExpiresAt != expiresAt {
			t.Fatalf("seenSendRequest.ExpiresAt = %#v, want %d", seenSendRequest.ExpiresAt, expiresAt)
		}
		if string(seenSendRequest.Ext["agh.workflow_id"]) != `"wf-1"` ||
			string(seenSendRequest.Ext["agh.handoff_version"]) != `3` {
			t.Fatalf("seenSendRequest.Ext = %#v, want workflow metadata", seenSendRequest.Ext)
		}
		if seenSendRequest.WorkspaceID != "ws-alpha" || seenSendRequest.Surface != "thread" ||
			seenSendRequest.ThreadID != "thread_123" ||
			seenSendRequest.WorkID != "work_1" || seenSendRequest.To != "reviewer.sess-a" {
			t.Fatalf("seenSendRequest = %#v, want workspace-qualified thread surface payload", seenSendRequest)
		}
		var sent NetworkSendRecord
		if err := json.Unmarshal([]byte(sendOut), &sent); err != nil {
			t.Fatalf("json.Unmarshal(network send) error = %v", err)
		}
		if sent.ID != "msg-1" || sent.TraceID != "trace-1" {
			t.Fatalf("sent = %#v, want sent payload", sent)
		}

		sendOut, _, err = executeRootCommand(t, deps,
			"network", "--workspace", "ws-alpha", "send",
			"--session", "sess-a",
			"--channel", "builders",
			"--surface", "direct",
			"--direct", directID,
			"--kind", "say",
			"--to", "reviewer.sess-a",
			"--body", `{"text":"direct hello"}`,
			"--work", "work_2",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("network send direct error = %v", err)
		}
		if seenSendRequest.WorkspaceID != "ws-alpha" || seenSendRequest.Surface != "direct" ||
			seenSendRequest.DirectID != directID ||
			seenSendRequest.WorkID != "work_2" ||
			seenSendRequest.To != "reviewer.sess-a" {
			t.Fatalf("seenSendRequest = %#v, want workspace-qualified direct surface payload", seenSendRequest)
		}
		if err := json.Unmarshal([]byte(sendOut), &sent); err != nil {
			t.Fatalf("json.Unmarshal(network send direct) error = %v", err)
		}
		if sent.ID != "msg-1" || sent.Surface != "direct" || sent.DirectID != directID {
			t.Fatalf("sent = %#v, want direct sent payload", sent)
		}

		inboxOut, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"inbox",
			"--session",
			"sess-a",
			"-o",
			"human",
		)
		if err != nil {
			t.Fatalf("network inbox error = %v", err)
		}
		if !strings.Contains(inboxOut, "Channel") || !strings.Contains(inboxOut, "builders") ||
			!strings.Contains(inboxOut, "wf-1") ||
			!strings.Contains(inboxOut, "3") {
			t.Fatalf("network inbox output = %q, want channel and workflow/handoff metadata", inboxOut)
		}
	})
}

func TestNetworkConversationCommandsAndFormatting(t *testing.T) {
	t.Parallel()

	directID := "direct_99401d24bee62651d189e5a561785466"
	thread := NetworkThreadRecord{
		Channel:            "builders",
		ThreadID:           "thread_launch",
		RootMessageID:      "msg-root",
		Title:              "Launch review",
		OpenedByPeerID:     "coder.sess-a",
		OpenedSessionID:    "sess-a",
		OpenedAt:           &fixedTestNow,
		LastActivityAt:     &fixedTestNow,
		MessageCount:       2,
		ParticipantCount:   2,
		OpenWorkCount:      1,
		LastMessagePreview: "ready for review",
	}
	direct := NetworkDirectRoomRecord{
		Channel:            "builders",
		DirectID:           directID,
		SessionA:           "sess-a",
		SessionB:           "sess-b",
		OpenedAt:           &fixedTestNow,
		LastActivityAt:     &fixedTestNow,
		MessageCount:       1,
		OpenWorkCount:      1,
		LastMessagePreview: "please review",
	}
	threadMessage := NetworkConversationMessageRecord{
		MessageID:   "msg-thread-1",
		Channel:     "builders",
		Surface:     "thread",
		ThreadID:    "thread_launch",
		Kind:        "say",
		Direction:   "outbound",
		PeerFrom:    "coder.sess-a",
		WorkID:      "work_1",
		PreviewText: "ready for review",
		Body:        mustJSON(t, map[string]any{"text": "ready for review"}),
		Timestamp:   fixedTestNow,
	}
	directMessage := NetworkConversationMessageRecord{
		MessageID:   "msg-direct-1",
		Channel:     "builders",
		Surface:     "direct",
		DirectID:    directID,
		Kind:        "say",
		Direction:   "outbound",
		PeerFrom:    "coder.sess-a",
		PeerTo:      "reviewer.sess-b",
		WorkID:      "work_1",
		PreviewText: "please review",
		Body:        mustJSON(t, map[string]any{"text": "please review"}),
		Timestamp:   fixedTestNow,
	}
	work := NetworkWorkRecord{
		WorkID:          "work_1",
		Channel:         "builders",
		Surface:         "direct",
		DirectID:        directID,
		OpenedSessionID: "sess-a",
		TargetSessionID: "sess-b",
		State:           "open",
		OpenedAt:        &fixedTestNow,
		LastActivityAt:  &fixedTestNow,
	}

	t.Run("Should list threads as JSON response wrapper", func(t *testing.T) {
		t.Parallel()

		var seenQuery NetworkThreadsQuery
		deps := newTestDeps(t, &stubClient{
			networkThreadsFn: func(
				_ context.Context,
				query NetworkThreadsQuery,
			) (contract.NetworkThreadsResponse, error) {
				seenQuery = query
				return contract.NetworkThreadsResponse{
					Threads: []NetworkThreadRecord{thread},
					Page: contract.CountedCursorPagePayload{
						NextCursor: "thread_next",
						HasMore:    true,
						Total:      8,
						Limit:      2,
					},
				}, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"threads",
			"list",
			"--channel",
			"builders",
			"--query",
			"launch",
			"--session",
			"sess-b",
			"--sort",
			"alphabetical",
			"--has-work",
			"--limit",
			"2",
			"--after",
			"thread_0",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network threads list error = %v", err)
		}
		if seenQuery.WorkspaceRef != "ws-alpha" || seenQuery.Channel != "builders" ||
			seenQuery.Query != "launch" || seenQuery.SessionID != "sess-b" ||
			seenQuery.Sort != "alphabetical" || seenQuery.HasWork == nil || !*seenQuery.HasWork ||
			seenQuery.Limit != 2 || seenQuery.After != "thread_0" {
			t.Fatalf("seenQuery = %#v, want all thread list filters", seenQuery)
		}
		var response contract.NetworkThreadsResponse
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatalf("json.Unmarshal(network threads list) error = %v", err)
		}
		if len(response.Threads) != 1 || response.Threads[0].ThreadID != "thread_launch" {
			t.Fatalf("response = %#v, want one thread", response)
		}
		if response.Page.Total != 8 || response.Page.Limit != 2 || !response.Page.HasMore ||
			response.Page.NextCursor != "thread_next" {
			t.Fatalf("response.Page = %#v, want truthful thread page", response.Page)
		}
	})

	t.Run("Should show a thread as TOON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			networkThreadFn: func(_ context.Context, workspaceRef string, channel string, threadID string) (NetworkThreadRecord, error) {
				if workspaceRef != "ws-alpha" || channel != "builders" || threadID != "thread_launch" {
					t.Fatalf(
						"NetworkThread(%q, %q, %q), want ws-alpha/builders/thread_launch",
						workspaceRef,
						channel,
						threadID,
					)
				}
				return thread, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"threads",
			"show",
			"--channel",
			"builders",
			"--thread",
			"thread_launch",
			"-o",
			"toon",
		)
		if err != nil {
			t.Fatalf("network threads show error = %v", err)
		}
		if !strings.Contains(out, "network_thread{") || !strings.Contains(out, "thread_launch") {
			t.Fatalf("network threads show toon = %q, want thread object", out)
		}
	})

	t.Run("Should stream thread messages as JSONL", func(t *testing.T) {
		t.Parallel()

		var seenQuery NetworkConversationMessagesQuery
		deps := newTestDeps(t, &stubClient{
			networkThreadMessagesFn: func(
				_ context.Context,
				query NetworkConversationMessagesQuery,
			) (contract.NetworkThreadMessagesResponse, error) {
				seenQuery = query
				return contract.NetworkThreadMessagesResponse{
					Messages: []NetworkConversationMessageRecord{threadMessage},
					Page: contract.CursorPagePayload{
						Limit: 2,
					},
				}, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"threads",
			"messages",
			"--channel",
			"builders",
			"--thread",
			"thread_launch",
			"--limit",
			"2",
			"--before",
			"msg-3",
			"--after",
			"msg-1",
			"--kind",
			"say",
			"--work",
			"work_1",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("network threads messages error = %v", err)
		}
		if seenQuery.WorkspaceRef != "ws-alpha" || seenQuery.Channel != "builders" ||
			seenQuery.ThreadID != "thread_launch" ||
			seenQuery.Limit != 2 ||
			seenQuery.Before != "msg-3" ||
			seenQuery.After != "msg-1" ||
			seenQuery.Kind != "say" ||
			seenQuery.WorkID != "work_1" {
			t.Fatalf("seenQuery = %#v, want workspace thread message filters", seenQuery)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 1 {
			t.Fatalf("network threads messages jsonl lines = %d, want 1; output=%q", len(lines), out)
		}
		var decoded NetworkConversationMessageRecord
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(thread message line) error = %v", err)
		}
		if decoded.MessageID != "msg-thread-1" || decoded.Surface != "thread" {
			t.Fatalf("decoded = %#v, want thread message", decoded)
		}
	})

	t.Run("Should list directs as JSON response wrapper", func(t *testing.T) {
		t.Parallel()

		var seenQuery NetworkDirectsQuery
		deps := newTestDeps(t, &stubClient{
			networkDirectsFn: func(
				_ context.Context,
				query NetworkDirectsQuery,
			) (contract.NetworkDirectRoomsResponse, error) {
				seenQuery = query
				return contract.NetworkDirectRoomsResponse{
					Directs: []NetworkDirectRoomRecord{direct},
					Page: contract.CountedCursorPagePayload{
						Total: 1,
						Limit: 2,
					},
				}, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"directs",
			"list",
			"--channel",
			"builders",
			"--session",
			"sess-b",
			"--query",
			"review",
			"--sort",
			"created",
			"--has-work=false",
			"--limit",
			"2",
			"--after",
			"direct_0",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network directs list error = %v", err)
		}
		if seenQuery.WorkspaceRef != "ws-alpha" || seenQuery.Channel != "builders" ||
			seenQuery.Query != "review" || seenQuery.SessionID != "sess-b" ||
			seenQuery.Sort != "created" || seenQuery.HasWork == nil || *seenQuery.HasWork ||
			seenQuery.Limit != 2 || seenQuery.After != "direct_0" {
			t.Fatalf("seenQuery = %#v, want all direct list filters", seenQuery)
		}
		var response contract.NetworkDirectRoomsResponse
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatalf("json.Unmarshal(network directs list) error = %v", err)
		}
		if len(response.Directs) != 1 || response.Directs[0].DirectID != directID {
			t.Fatalf("response = %#v, want one direct room", response)
		}
		if response.Page.Total != 1 || response.Page.Limit != 2 || response.Page.HasMore ||
			response.Page.NextCursor != "" {
			t.Fatalf("response.Page = %#v, want truthful direct page", response.Page)
		}
	})

	t.Run("Should resolve a direct room", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			networkDirectResolveFn: func(
				_ context.Context,
				workspaceRef string,
				channel string,
				request NetworkDirectResolveRequest,
			) (NetworkDirectRoomRecord, error) {
				if workspaceRef != "ws-alpha" || channel != "builders" || request.SessionID != "sess-a" ||
					request.PeerID != "reviewer.sess-b" {
					t.Fatalf(
						"NetworkDirectResolve(%q, %q, %#v), want ws-alpha/builders/sess-a/reviewer.sess-b",
						workspaceRef,
						channel,
						request,
					)
				}
				return direct, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"directs",
			"resolve",
			"--session",
			"sess-a",
			"--channel",
			"builders",
			"--peer",
			"reviewer.sess-b",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network directs resolve error = %v", err)
		}
		var response contract.NetworkDirectRoomResponse
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatalf("json.Unmarshal(network directs resolve) error = %v", err)
		}
		if response.Direct.DirectID != directID {
			t.Fatalf("response = %#v, want resolved direct room", response)
		}
	})

	t.Run("Should show a direct room as TOON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			networkDirectFn: func(_ context.Context, workspaceRef string, channel string, gotDirectID string) (NetworkDirectRoomRecord, error) {
				if workspaceRef != "ws-alpha" || channel != "builders" || gotDirectID != directID {
					t.Fatalf(
						"NetworkDirect(%q, %q, %q), want ws-alpha/builders/%s",
						workspaceRef,
						channel,
						gotDirectID,
						directID,
					)
				}
				return direct, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"directs",
			"show",
			"--channel",
			"builders",
			"--direct",
			directID,
			"-o",
			"toon",
		)
		if err != nil {
			t.Fatalf("network directs show error = %v", err)
		}
		if !strings.Contains(out, "network_direct{") || !strings.Contains(out, directID) {
			t.Fatalf("network directs show toon = %q, want direct object", out)
		}
	})

	t.Run("Should stream direct messages as JSONL", func(t *testing.T) {
		t.Parallel()

		var seenQuery NetworkConversationMessagesQuery
		deps := newTestDeps(t, &stubClient{
			networkDirectMessagesFn: func(
				_ context.Context,
				query NetworkConversationMessagesQuery,
			) (contract.NetworkDirectRoomMessagesResponse, error) {
				seenQuery = query
				return contract.NetworkDirectRoomMessagesResponse{
					Messages: []NetworkConversationMessageRecord{directMessage},
					Page: contract.CursorPagePayload{
						Limit: 2,
					},
				}, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"directs",
			"messages",
			"--channel",
			"builders",
			"--direct",
			directID,
			"--limit",
			"2",
			"--work",
			"work_1",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("network directs messages error = %v", err)
		}
		if seenQuery.WorkspaceRef != "ws-alpha" || seenQuery.Channel != "builders" || seenQuery.DirectID != directID ||
			seenQuery.Limit != 2 || seenQuery.WorkID != "work_1" {
			t.Fatalf("seenQuery = %#v, want workspace direct message filters", seenQuery)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 1 {
			t.Fatalf("network directs messages jsonl lines = %d, want 1; output=%q", len(lines), out)
		}
		var decoded NetworkConversationMessageRecord
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(direct message line) error = %v", err)
		}
		if decoded.MessageID != "msg-direct-1" || decoded.Surface != "direct" {
			t.Fatalf("decoded = %#v, want direct message", decoded)
		}
	})

	t.Run("Should show work lookup as JSON response wrapper", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			networkWorkFn: func(_ context.Context, workspaceRef string, workID string) (NetworkWorkRecord, error) {
				if workspaceRef != "ws-alpha" || workID != "work_1" {
					t.Fatalf("NetworkWork(%q, %q), want ws-alpha/work_1", workspaceRef, workID)
				}
				return work, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"work",
			"lookup",
			"--work",
			"work_1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("network work lookup error = %v", err)
		}
		var response contract.NetworkWorkResponse
		if err := json.Unmarshal([]byte(out), &response); err != nil {
			t.Fatalf("json.Unmarshal(network work lookup) error = %v", err)
		}
		if response.Work.WorkID != "work_1" || response.Work.DirectID != directID {
			t.Fatalf("response = %#v, want work", response)
		}
	})

	t.Run("Should show work status as TOON", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			networkWorkFn: func(_ context.Context, workspaceRef string, workID string) (NetworkWorkRecord, error) {
				if workspaceRef != "ws-alpha" || workID != "work_1" {
					t.Fatalf("NetworkWork(%q, %q), want ws-alpha/work_1", workspaceRef, workID)
				}
				return work, nil
			},
		})
		out, _, err := executeRootCommand(
			t,
			deps,
			"network",
			"--workspace",
			"ws-alpha",
			"work",
			"status",
			"--work",
			"work_1",
			"-o",
			"toon",
		)
		if err != nil {
			t.Fatalf("network work status error = %v", err)
		}
		if !strings.Contains(out, "network_work{") || !strings.Contains(out, "work_1") {
			t.Fatalf("network work status toon = %q, want work object", out)
		}
	})
}

func TestNetworkSendParsersRejectInvalidFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "ShouldRejectInvalidBodyJSON",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `not-json`,
			},
			wantErr: "--body must be valid JSON",
		},
		{
			name: "ShouldRejectNonObjectExtJSON",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
				"--ext", `[]`,
			},
			wantErr: "--ext must be a JSON object",
		},
		{
			name: "ShouldRejectInvalidExpiresAtValues",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
				"--expires-at", `tomorrow`,
			},
			wantErr: "--expires-at must be unix seconds or RFC3339",
		},
		{
			name: "ShouldRejectRawClaimTokenBody",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `{"nested":{"claim_token":"agh_claim_cli"}}`,
			},
			wantErr: "network_raw_token_rejected",
		},
		{
			name: "ShouldRejectRawClaimTokenExt",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
				"--ext", `{"agh":{"claim_token":"agh_claim_cli"}}`,
			},
			wantErr: "network_raw_token_rejected",
		},
		{
			name: "ShouldRejectLegacyInteractionIDFlag",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "thread",
				"--thread", "thread_123",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
				"--interaction-id", "work_1",
			},
			wantErr: "unknown flag: --interaction-id",
		},
		{
			name: "ShouldRejectDirectKind",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "direct",
				"--direct", "direct_99401d24bee62651d189e5a561785466",
				"--kind", "direct",
				"--body", `{"text":"ok"}`,
			},
			wantErr: "--kind direct is not supported",
		},
		{
			name: "ShouldRejectConversationKindWithoutSurface",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
			},
			wantErr: "--surface is required for --kind say",
		},
		{
			name: "Should reject malformed public thread id locally",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "thread",
				"--thread", "launch-brief-0711",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
			},
			wantErr: "--thread must match ^thread_[a-z0-9][a-z0-9_-]{2,95}$",
		},
		{
			name: "Should reject capability without directed target",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "thread",
				"--thread", "thread_123",
				"--kind", "capability",
				"--work", "work_1",
				"--body", `{"capability":{"id":"review"}}`,
			},
			wantErr: "--kind capability requires --to",
		},
		{
			name: "Should reject lifecycle say without directed target",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "thread",
				"--thread", "thread_123",
				"--kind", "say",
				"--work", "work_1",
				"--body", `{"text":"ok"}`,
			},
			wantErr: "--kind say with --work requires --to",
		},
		{
			name: "ShouldRejectLegacyWorkIDFlag",
			args: []string{
				"network", "send",
				"--session", "sess-a",
				"--channel", "builders",
				"--surface", "thread",
				"--thread", "thread_123",
				"--kind", "say",
				"--body", `{"text":"ok"}`,
				"--work-id", "work_1",
			},
			wantErr: "unknown flag: --work-id",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps := newTestDeps(t, &stubClient{
				networkSendFn: func(context.Context, NetworkSendRequest) (NetworkSendRecord, error) {
					t.Fatal("NetworkSend() called for invalid network send flags")
					return NetworkSendRecord{}, nil
				},
			})

			if _, _, err := executeRootCommand(
				t,
				deps,
				tc.args...); err == nil ||
				!strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("executeRootCommand(%v) error = %v, want substring %q", tc.args, err, tc.wantErr)
			}
		})
	}
}
