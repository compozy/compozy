package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/network/participation"
	observepkg "github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

const networkCoreTestWorkspaceID = "ws-network-core"

type strictBundleHandlerService struct {
	activateCalls int
	updateCalls   int
}

func (s *strictBundleHandlerService) Catalog(context.Context) ([]bundlepkg.CatalogEntry, error) {
	return nil, nil
}

func (s *strictBundleHandlerService) PreviewActivation(
	context.Context,
	bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (s *strictBundleHandlerService) Activate(
	context.Context,
	bundlepkg.ActivateRequest,
) (bundlepkg.ActivationPreview, error) {
	s.activateCalls++
	return bundlepkg.ActivationPreview{}, nil
}

func (s *strictBundleHandlerService) ListActivations(context.Context) ([]bundlepkg.ActivationPreview, error) {
	return nil, nil
}

func (s *strictBundleHandlerService) GetActivation(
	context.Context,
	string,
) (bundlepkg.ActivationPreview, error) {
	return bundlepkg.ActivationPreview{}, nil
}

func (s *strictBundleHandlerService) UpdateActivation(
	context.Context,
	bundlepkg.UpdateActivationRequest,
) (bundlepkg.ActivationPreview, error) {
	s.updateCalls++
	return bundlepkg.ActivationPreview{}, nil
}

func (s *strictBundleHandlerService) Deactivate(context.Context, string) error { return nil }

func (s *strictBundleHandlerService) NetworkSettings(context.Context) (bundlepkg.NetworkSettings, error) {
	return bundlepkg.NetworkSettings{}, nil
}

func TestBundleHandlersRejectUnknownFields(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		method string
		path   string
		body   string
		call   func(*BaseHandlers, *gin.Context)
	}{
		{
			name:   "Should reject removed activation input",
			method: http.MethodPost,
			path:   "/api/bundles/activations",
			body:   `{"extension_name":"ext","bundle_name":"bundle","profile_name":"default","bind_primary_channel_as_default":true}`,
			call:   (*BaseHandlers).ActivateBundle,
		},
		{
			name:   "Should reject removed confirmation input",
			method: http.MethodPatch,
			path:   "/api/bundles/activations/act-1",
			body:   `{"expected_version":1,"confirm_network_requirement":true,"confirmed_by":"caller"}`,
			call:   (*BaseHandlers).UpdateBundleActivation,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &strictBundleHandlerService{}
			handlers := NewBaseHandlers(&BaseHandlerConfig{TransportName: "core-test", Bundles: service})
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequestWithContext(
				t.Context(),
				testCase.method,
				testCase.path,
				bytes.NewBufferString(testCase.body),
			)
			ctx.Request.Header.Set("Content-Type", "application/json")
			if testCase.method == http.MethodPatch {
				ctx.Params = gin.Params{{Key: "id", Value: "act-1"}}
			}

			testCase.call(handlers, ctx)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s, want 400", recorder.Code, recorder.Body.String())
			}
			if service.activateCalls != 0 || service.updateCalls != 0 {
				t.Fatalf(
					"service calls = activate:%d update:%d, want zero",
					service.activateCalls,
					service.updateCalls,
				)
			}
		})
	}
}

func TestBundleCatalogPayloadsAndDeclaredChannels(t *testing.T) {
	t.Parallel()

	t.Run("Should build bundle catalog and declared channel payloads", func(t *testing.T) {
		t.Parallel()

		catalog := BundleCatalogPayloads([]bundlepkg.CatalogEntry{{
			ExtensionName: " ext-bundle ",
			Bundle: extensionpkg.BundleSpec{
				Name:        " ops ",
				Description: " Operations bundle ",
				Profiles: []extensionpkg.BundleProfile{{
					Name:        " default ",
					Description: " Primary profile ",
					Channels: extensionpkg.BundleChannelsConfig{
						Primary: "primary",
						Items: []extensionpkg.BundleChannel{
							{Name: " primary ", Description: " Main channel "},
							{Name: " secondary ", Description: " Backup channel "},
						},
					},
					Layouts: []extensionpkg.BundleLayout{{Path: "layouts/ops.json"}},
					Agents: []extensionpkg.BundleAgent{{
						Path:  "agents/planner",
						Agent: aghconfig.AgentDef{Name: "planner", Prompt: "Plan work."},
					}},
					Jobs:     []extensionpkg.BundleJob{{Name: "job-a"}},
					Triggers: []extensionpkg.BundleTrigger{{Name: "trigger-a"}},
					Bridges:  []extensionpkg.BundleBridgePreset{{Name: "bridge-a"}},
				}},
			},
		}})

		if got, want := len(catalog), 1; got != want {
			t.Fatalf("len(catalog) = %d, want %d", got, want)
		}
		if catalog[0].ExtensionName != "ext-bundle" || catalog[0].BundleName != "ops" ||
			catalog[0].Profiles[0].PrimaryChannel != "primary" {
			t.Fatalf("catalog payload = %#v", catalog[0])
		}
		if got, want := len(catalog[0].Profiles[0].Channels), 2; got != want {
			t.Fatalf("len(profile channels) = %d, want %d", got, want)
		}
		if got, want := catalog[0].Profiles[0].AgentCount, 1; got != want {
			t.Fatalf("profile.AgentCount = %d, want %d", got, want)
		}
		if got, want := catalog[0].Profiles[0].LayoutCount, 1; got != want {
			t.Fatalf("profile.LayoutCount = %d, want %d", got, want)
		}
		if !catalog[0].Profiles[0].Channels[0].Primary || catalog[0].Profiles[0].Channels[1].Primary {
			t.Fatalf("channel primary flags = %#v", catalog[0].Profiles[0].Channels)
		}

		declared := DeclaredNetworkChannelPayloads([]bundlepkg.DeclaredChannel{{
			ActivationID:  " act-1 ",
			ExtensionName: " ext-bundle ",
			BundleName:    " ops ",
			ProfileName:   " default ",
			WorkspaceID:   " ws-1 ",
			Name:          " builders ",
			Description:   " Build channel ",
			Primary:       true,
		}})
		if got, want := len(declared), 1; got != want {
			t.Fatalf("len(declared) = %d, want %d", got, want)
		}
		if declared[0].ActivationID != "act-1" || declared[0].Name != "builders" || !declared[0].Primary {
			t.Fatalf("declared payload = %#v", declared[0])
		}
	})
}

func TestStatusForBundleErrorAndChannelHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "nil", err: nil, want: http.StatusOK},
		{name: "activation missing", err: bundlepkg.ErrActivationNotFound, want: http.StatusNotFound},
		{name: "bundle missing", err: bundlepkg.ErrBundleNotFound, want: http.StatusNotFound},
		{name: "profile missing", err: bundlepkg.ErrProfileNotFound, want: http.StatusNotFound},
		{name: "extension missing", err: extensionpkg.ErrExtensionNotFound, want: http.StatusNotFound},
		{name: "agent conflict", err: bundlepkg.ErrAgentConflict, want: http.StatusConflict},
		{
			name: "agent reference missing",
			err:  bundlepkg.ErrAgentReferenceNotFound,
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "Should map a reserved agent name to unprocessable entity",
			err:  fmt.Errorf("bundle agent: %w", aghconfig.ErrAgentNameReserved),
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "extension has active bundles",
			err:  extensionpkg.ErrExtensionHasActiveBundles,
			want: http.StatusConflict,
		},
		{
			name: "live network requirement needs confirmation",
			err:  bundlepkg.ErrNetworkRequirementConfirmationRequired,
			want: http.StatusConflict,
		},
		{name: "stale activation version", err: resources.ErrConflict, want: http.StatusConflict},
		{name: "webhook unsupported", err: bundlepkg.ErrWebhookUnsupported, want: http.StatusBadRequest},
		{name: "workspace missing", err: workspacepkg.ErrWorkspaceNotFound, want: http.StatusNotFound},
		{name: "workspace root missing", err: workspacepkg.ErrWorkspaceRootMissing, want: http.StatusGone},
		{name: "default", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := StatusForBundleError(tt.err); got != tt.want {
				t.Fatalf("StatusForBundleError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}

	t.Run("Should report visible session channels", func(t *testing.T) {
		t.Parallel()

		sessions := []*session.Info{
			{
				ID:                   "sess-visible",
				WorkspaceID:          networkCoreTestWorkspaceID,
				NetworkParticipation: coreTestLiveParticipation(networkCoreTestWorkspaceID, "builders"),
				State:                session.StateActive,
			},
			{
				ID:                   "sess-stopped",
				WorkspaceID:          networkCoreTestWorkspaceID,
				NetworkParticipation: coreTestLiveParticipation(networkCoreTestWorkspaceID, "builders"),
				State:                session.StateStopped,
			},
		}
		peers := []network.PeerInfo{{PeerID: "peer-1", WorkspaceID: networkCoreTestWorkspaceID, Channel: "operators"}}
		if !networkChannelExists(sessions, peers, nil, networkCoreTestWorkspaceID, "builders") {
			t.Fatal("networkChannelExists() = false, want true for visible session channel")
		}
	})

	t.Run("Should report peer-backed channels", func(t *testing.T) {
		t.Parallel()

		if !networkChannelExists(nil, []network.PeerInfo{{
			PeerID:      "peer-2",
			WorkspaceID: networkCoreTestWorkspaceID,
			Channel:     "match",
		}}, nil, networkCoreTestWorkspaceID, "match") {
			t.Fatal("networkChannelExists() = false, want true for peer channel")
		}
	})

	t.Run("Should report persisted metadata channels", func(t *testing.T) {
		t.Parallel()

		metadata := &store.NetworkChannelEntry{WorkspaceID: networkCoreTestWorkspaceID, Channel: "builders"}
		if !networkChannelExists(nil, nil, metadata, networkCoreTestWorkspaceID, "builders") {
			t.Fatal("networkChannelExists() = false, want true for persisted metadata")
		}
	})

	t.Run("Should report missing channels as absent", func(t *testing.T) {
		t.Parallel()

		peers := []network.PeerInfo{{PeerID: "peer-1", WorkspaceID: networkCoreTestWorkspaceID, Channel: "operators"}}
		if networkChannelExists(nil, peers, nil, networkCoreTestWorkspaceID, "missing") {
			t.Fatal("networkChannelExists() = true, want false for missing channel")
		}
	})

	t.Run("Should recognize network channel not-found errors", func(t *testing.T) {
		t.Parallel()

		if !isNetworkChannelNotFound(errNetworkChannelNotFound) {
			t.Fatal("isNetworkChannelNotFound() = false, want true")
		}
	})
}

func coreTestLiveParticipation(workspaceID string, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
		Source:          participation.SourceExplicitRequest,
	}
}

func TestNetworkChannelAggregateKeepsConversationActivitySeparateFromMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should keep conversation activity separate from metadata", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		metadataAt := recordedAt.Add(10 * time.Minute)
		aggregates := make(map[string]*networkChannelAggregate)

		applyNetworkChannelMetadata(aggregates, networkCoreTestWorkspaceID, []store.NetworkChannelEntry{{
			WorkspaceID: networkCoreTestWorkspaceID,
			Channel:     "builders",
			UpdatedAt:   metadataAt,
		}})
		applyNetworkChannelMessages(aggregates, []store.NetworkMessageEntry{{
			Channel:   "builders",
			Text:      "hello from text",
			Timestamp: recordedAt,
		}})

		aggregate := aggregates["builders"]
		if aggregate == nil {
			t.Fatal("aggregate = nil, want builders aggregate")
			return
		}
		if aggregate.lastActivityAt == nil || !aggregate.lastActivityAt.Equal(recordedAt) {
			t.Fatalf("aggregate.lastActivityAt = %#v, want %s", aggregate.lastActivityAt, recordedAt)
		}
		if aggregate.lastMessageAt == nil || !aggregate.lastMessageAt.Equal(recordedAt) {
			t.Fatalf("aggregate.lastMessageAt = %#v, want %s", aggregate.lastMessageAt, recordedAt)
		}
		if got, want := aggregate.lastMessagePreview, "hello from text"; got != want {
			t.Fatalf("aggregate.lastMessagePreview = %q, want %q", got, want)
		}
	})
}

func TestSortedNetworkChannelPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should sort channels by effective recency when activity is missing", func(t *testing.T) {
		t.Parallel()

		olderPresence := time.Date(2026, 4, 28, 6, 34, 55, 0, time.UTC)
		newerPresence := olderPresence.Add(2 * time.Minute)
		channels := sortedNetworkChannelPayloads(map[string]*networkChannelAggregate{
			"scope-older": {
				channel:                    "scope-older",
				lastPresenceAt:             &olderPresence,
				presenceCount:              2,
				historicalParticipantCount: 2,
			},
			"scope-newer": {
				channel:                    "scope-newer",
				lastPresenceAt:             &newerPresence,
				presenceCount:              2,
				historicalParticipantCount: 2,
			},
		})

		if got, want := len(channels), 2; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := channels[0].Channel, "scope-newer"; got != want {
			t.Fatalf("channels[0].Channel = %q, want %q", got, want)
		}
		if got, want := channels[1].Channel, "scope-older"; got != want {
			t.Fatalf("channels[1].Channel = %q, want %q", got, want)
		}
	})

	t.Run("Should prefer fresher presence over stale activity when channels are reactivated", func(t *testing.T) {
		t.Parallel()

		staleActivity := time.Date(2026, 4, 28, 7, 30, 0, 0, time.UTC)
		freshPresence := staleActivity.Add(20 * time.Minute)
		recentActivity := staleActivity.Add(10 * time.Minute)
		channels := sortedNetworkChannelPayloads(map[string]*networkChannelAggregate{
			"launch-room": {
				channel:                    "launch-room",
				lastActivityAt:             &staleActivity,
				lastPresenceAt:             &freshPresence,
				messageCount:               4,
				presenceCount:              8,
				historicalParticipantCount: 7,
			},
			"coord-core": {
				channel:                    "coord-core",
				lastActivityAt:             &recentActivity,
				messageCount:               6,
				presenceCount:              2,
				historicalParticipantCount: 2,
			},
		})

		if got, want := len(channels), 2; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := channels[0].Channel, "launch-room"; got != want {
			t.Fatalf("channels[0].Channel = %q, want %q", got, want)
		}
		if got, want := channels[1].Channel, "coord-core"; got != want {
			t.Fatalf("channels[1].Channel = %q, want %q", got, want)
		}
	})

	t.Run("Should prefer higher message counts when effective recency ties", func(t *testing.T) {
		t.Parallel()

		recency := time.Date(2026, 4, 28, 6, 40, 0, 0, time.UTC)
		channels := sortedNetworkChannelPayloads(map[string]*networkChannelAggregate{
			"scope-low": {
				channel:        "scope-low",
				lastPresenceAt: &recency,
				messageCount:   1,
			},
			"scope-high": {
				channel:        "scope-high",
				lastPresenceAt: &recency,
				messageCount:   3,
			},
		})

		if got, want := len(channels), 2; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := channels[0].Channel, "scope-high"; got != want {
			t.Fatalf("channels[0].Channel = %q, want %q", got, want)
		}
		if got, want := channels[1].Channel, "scope-low"; got != want {
			t.Fatalf("channels[1].Channel = %q, want %q", got, want)
		}
	})
}

func TestSortedNetworkPeerPayloads(t *testing.T) {
	t.Parallel()

	t.Run("Should sort local peers by joined-at recency when last-seen is missing", func(t *testing.T) {
		t.Parallel()

		olderJoinedAt := time.Date(2026, 4, 28, 7, 17, 11, 0, time.UTC)
		newerJoinedAt := olderJoinedAt.Add(3 * time.Second)
		peers := []contract.NetworkPeerPayload{
			{
				PeerID:      "backend.sess-older",
				DisplayName: "Backend",
				Channel:     "builders",
				Local:       true,
				JoinedAt:    &olderJoinedAt,
			},
			{
				PeerID:      "coder.sess-newer",
				DisplayName: "Coder",
				Channel:     "builders",
				Local:       true,
				JoinedAt:    &newerJoinedAt,
			},
		}

		sortNetworkPeerPayloads(peers)

		if got, want := peers[0].PeerID, "coder.sess-newer"; got != want {
			t.Fatalf("peers[0].peer_id = %q, want %q", got, want)
		}
		if got, want := peers[1].PeerID, "backend.sess-older"; got != want {
			t.Fatalf("peers[1].peer_id = %q, want %q", got, want)
		}
	})
}

func TestNetworkPayloadHelpersCloneAndNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should clone and normalize network payload helpers", func(t *testing.T) {
		t.Parallel()

		joinedAt := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		displayName := " Support Bot "
		sessionID := "sess-1"
		ext := map[string]json.RawMessage{"role": json.RawMessage(`"support"`)}
		peerPayloads := NetworkPeerPayloadsFromInfos([]network.PeerInfo{{
			SessionID:     &sessionID,
			PeerID:        "peer-1",
			Channel:       "builders",
			Local:         true,
			PresenceState: network.PresenceStateLocal,
			PeerCard: network.PeerCard{
				PeerID:              "peer-1",
				DisplayName:         &displayName,
				ProfilesSupported:   []string{"default"},
				Capabilities:        []string{"chat"},
				ArtifactsSupported:  []string{"text"},
				TrustModesSupported: []string{"strict"},
				Ext:                 ext,
			},
			JoinedAt: &joinedAt,
		}})

		if got, want := len(peerPayloads), 1; got != want {
			t.Fatalf("len(peerPayloads) = %d, want %d", got, want)
		}
		if peerPayloads[0].DisplayName != "Support Bot" {
			t.Fatalf("DisplayName = %q, want %q", peerPayloads[0].DisplayName, "Support Bot")
		}
		if peerPayloads[0].PeerCard.Ext["role"] == nil {
			t.Fatalf("PeerCard.Ext = %#v, want copied metadata", peerPayloads[0].PeerCard.Ext)
		}
		if got := peerPayloads[0].PresenceState; got != contract.NetworkPresenceLocal {
			t.Fatalf("PresenceState = %q, want %q", got, contract.NetworkPresenceLocal)
		}

		displayName = "mutated"
		ext["role"][0] = '['
		if peerPayloads[0].DisplayName != "Support Bot" ||
			string(peerPayloads[0].PeerCard.Ext["role"]) != `"support"` {
			t.Fatalf("peer payload mutated with source data = %#v", peerPayloads[0])
		}

		channelPayloads := NetworkChannelPayloadsFromInfos([]network.ChannelInfo{{Channel: "builders", PeerCount: 2}})
		if got, want := len(channelPayloads), 1; got != want {
			t.Fatalf("len(channelPayloads) = %d, want %d", got, want)
		}
		if channelPayloads[0].Channel != "builders" || channelPayloads[0].PeerCount != 2 {
			t.Fatalf("channel payload = %#v", channelPayloads[0])
		}
	})
}

func TestCoreConversionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should convert core payload helpers", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		later := now.Add(5 * time.Minute)

		usage := TokenUsagePayloadFromUsage(&acp.TokenUsage{
			TurnID:           "turn-1",
			InputTokens:      new(int64(10)),
			OutputTokens:     new(int64(20)),
			TotalTokens:      new(int64(30)),
			ThoughtTokens:    new(int64(3)),
			CacheReadTokens:  new(int64(4)),
			CacheWriteTokens: new(int64(5)),
			ContextUsed:      new(int64(6)),
			ContextSize:      new(int64(7)),
			CostAmount:       new(1.23),
			CostCurrency:     new("USD"),
			Timestamp:        now,
		})
		if usage == nil || usage.TotalTokens == nil || *usage.TotalTokens != 30 || usage.CostCurrency == nil ||
			*usage.CostCurrency != "USD" {
			t.Fatalf("TokenUsagePayloadFromUsage() = %#v", usage)
		}
		if TokenUsagePayloadFromUsage(nil) != nil {
			t.Fatal("TokenUsagePayloadFromUsage(nil) != nil")
		}

		health := BridgeHealthPayloadFromObserve(observepkg.BridgeInstanceHealth{
			BridgeInstanceID:        "brg-1",
			Status:                  bridgepkg.BridgeStatusDegraded,
			RouteCount:              2,
			DeliveryBacklog:         1,
			DeliveryDroppedTotal:    3,
			DeliveryDroppedByReason: map[string]int{"rate_limit": 2},
			DeliveryFailuresTotal:   4,
			AuthFailuresTotal:       5,
			LastSuccessAt:           now,
			LastError:               "timeout",
			LastErrorAt:             later,
		})
		if health.LastSuccessAt == nil ||
			health.LastErrorAt == nil ||
			health.DeliveryDroppedByReason["rate_limit"] != 2 {
			t.Fatalf("BridgeHealthPayloadFromObserve() = %#v", health)
		}

		if got := string(PayloadJSON("  ")); got != "null" {
			t.Fatalf("PayloadJSON(blank) = %s, want null", got)
		}
		if got := string(PayloadJSON(`{"ok":true}`)); got != `{"ok":true}` {
			t.Fatalf("PayloadJSON(valid json) = %s", got)
		}
		if got := string(PayloadJSON("not-json")); got != `"not-json"` {
			t.Fatalf("PayloadJSON(string) = %s, want quoted string", got)
		}

		if workspaceID, workspace := sessionWorkspaceFromInfo(
			&session.Info{WorkspaceID: " ws-1 ", Workspace: " /tmp/ws "},
		); workspaceID != "ws-1" ||
			workspace != "/tmp/ws" {
			t.Fatalf("sessionWorkspaceFromInfo() = %q/%q", workspaceID, workspace)
		}
		if workspaceID, workspace := sessionWorkspaceFromInfo(nil); workspaceID != "" || workspace != "" {
			t.Fatalf("sessionWorkspaceFromInfo(nil) = %q/%q", workspaceID, workspace)
		}

		if got := laterTimePtr(nil, now); got == nil || !got.Equal(now) {
			t.Fatalf("laterTimePtr(nil, now) = %#v", got)
		}
		if got := laterTimePtr(&later, now); got == nil || !got.Equal(later) {
			t.Fatalf("laterTimePtr(later, earlier) = %#v", got)
		}
		if got := laterTimePtr(&later, time.Time{}); got == nil || !got.Equal(later) {
			t.Fatalf("laterTimePtr(later, zero) = %#v", got)
		}

		role := json.RawMessage(`"support"`)
		proof := network.Proof{"role": role}
		clonedProof := cloneProofPtr(&proof)
		if clonedProof == nil || string(clonedProof["role"]) != `"support"` {
			t.Fatalf("cloneProofPtr() = %#v", clonedProof)
		}
		proof["role"][0] = '['
		if string(clonedProof["role"]) != `"support"` {
			t.Fatalf("cloneProofPtr() mutated with source proof = %#v", clonedProof)
		}
		if cloneProofPtr(nil) != nil {
			t.Fatal("cloneProofPtr(nil) != nil")
		}

		peerSessionID := "sess-1"
		peerName := "Peer Display"
		if got := networkPeerDisplayName(network.PeerInfo{
			SessionID: &peerSessionID,
			PeerID:    "peer-1",
			PeerCard:  network.PeerCard{DisplayName: &peerName},
		}, nil); got != "Peer Display" {
			t.Fatalf("networkPeerDisplayName(peer card) = %q", got)
		}

		if got := networkPeerDisplayName(network.PeerInfo{
			SessionID: &peerSessionID,
			PeerID:    "peer-1",
		}, map[string]*session.Info{
			"sess-1": {Name: "Session Name", AgentName: "coder"},
		}); got != "Session Name" {
			t.Fatalf("networkPeerDisplayName(session name) = %q", got)
		}

		if got := networkPeerDisplayName(network.PeerInfo{
			SessionID: &peerSessionID,
			PeerID:    "peer-1",
		}, map[string]*session.Info{
			"sess-1": {AgentName: "coder"},
		}); got != "coder" {
			t.Fatalf("networkPeerDisplayName(agent fallback) = %q", got)
		}

		if got := networkPeerDisplayName(network.PeerInfo{PeerID: " peer-1 "}, nil); got != "peer-1" {
			t.Fatalf("networkPeerDisplayName(peer id fallback) = %q", got)
		}
	})
}

func TestCoreTimeAndSessionHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize time and session helpers", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.FixedZone("offset", -3*60*60))
		got := timePointerFromMap(map[string]*time.Time{"sess-1": &now}, "sess-1")
		if got == nil || !got.Equal(now.UTC()) || got.Location() != time.UTC {
			t.Fatalf("timePointerFromMap() = %#v, want UTC copy", got)
		}
		if timePointerFromMap(nil, "sess-1") != nil {
			t.Fatal("timePointerFromMap(nil) != nil")
		}
		if timePointerFromMap(map[string]*time.Time{"sess-1": nil}, "sess-1") != nil {
			t.Fatal("timePointerFromMap(nil entry) != nil")
		}
		if networkChannelSessionVisible(nil) {
			t.Fatal("networkChannelSessionVisible(nil) = true, want false")
		}
		if networkChannelSessionVisible(&session.Info{
			State:                session.StateStopped,
			NetworkParticipation: coreTestLiveParticipation(networkCoreTestWorkspaceID, "builders"),
		}) {
			t.Fatal("networkChannelSessionVisible(stopped) = true, want false")
		}
		if !networkChannelSessionVisible(&session.Info{
			State:                session.StateActive,
			NetworkParticipation: coreTestLiveParticipation(networkCoreTestWorkspaceID, "builders"),
		}) {
			t.Fatal("networkChannelSessionVisible(active) = false, want true")
		}
	})
}

func TestSessionAndNetworkMappingHelpers(t *testing.T) {
	t.Parallel()

	payload := SessionPayloadFromInfo(&session.Info{
		ID:          "sess-1",
		Name:        "Support session",
		AgentName:   "coder",
		Provider:    "fake",
		WorkspaceID: " ws-1 ",
		Workspace:   " /tmp/ws ",
		ACPCaps: acp.Caps{
			SupportsLoadSession: true,
			SupportedModes:      []string{"edit"},
		},
	})
	if payload.ID != "sess-1" || payload.Provider != "fake" ||
		payload.WorkspaceID != "ws-1" || payload.WorkspacePath != "/tmp/ws" ||
		payload.ACPCaps == nil {
		t.Fatalf("SessionPayloadFromInfo() = %#v", payload)
	}
	if zero := SessionPayloadFromInfo(nil); zero.ID != "" {
		t.Fatalf("SessionPayloadFromInfo(nil) = %#v", zero)
	}

	t.Run("Should derive network message previews", func(t *testing.T) {
		t.Parallel()

		if got, want := networkMessagePreview(store.NetworkMessageEntry{
			Text: "hello from text",
		}), "hello from text"; got != want {
			t.Fatalf("networkMessagePreview(text fallback) = %q, want %q", got, want)
		}
		if got, want := networkMessagePreview(store.NetworkMessageEntry{
			Text:        "hello from text",
			PreviewText: "hello from preview",
		}), "hello from preview"; got != want {
			t.Fatalf("networkMessagePreview(preview) = %q, want %q", got, want)
		}
	})

	t.Run("Should map network channel messages to payloads", func(t *testing.T) {
		t.Parallel()

		sessionsByID := sessionInfoMapByID([]*session.Info{
			{ID: " sess-1 ", Name: "Support"},
			nil,
		})
		if sessionsByID["sess-1"] == nil {
			t.Fatalf("sessionInfoMapByID() missing trimmed session id: %#v", sessionsByID)
		}

		payloadMessage := NetworkConversationMessagePayloadFromEntry(
			store.NetworkMessageEntry{
				MessageID:   "msg-1",
				Channel:     "builders",
				Direction:   network.AuditDirectionSent,
				PeerFrom:    "peer-1",
				PeerTo:      "peer-2",
				Kind:        string(network.KindSay),
				SessionID:   "sess-1",
				PreviewText: "hello from preview",
				Body:        json.RawMessage(`{"text":"hello from preview"}`),
				Timestamp:   time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
			},
			sessionsByID,
			map[string]network.PeerInfo{},
		)
		if got, want := payloadMessage.Direction, network.AuditDirectionSent; got != want {
			t.Fatalf("NetworkConversationMessagePayloadFromEntry().Direction = %q, want %q", got, want)
		}
		if got, want := payloadMessage.PeerFrom, "peer-1"; got != want {
			t.Fatalf("NetworkConversationMessagePayloadFromEntry().PeerFrom = %q, want %q", got, want)
		}
		if got, want := payloadMessage.DisplayName, "Support"; got != want {
			t.Fatalf("NetworkConversationMessagePayloadFromEntry().DisplayName = %q, want %q", got, want)
		}
	})
}

func TestObserveHealthPayloadConversions(t *testing.T) {
	t.Run("Should include runtime activity", func(t *testing.T) {
		t.Parallel()

		lastActivityAt := time.Date(2026, 4, 24, 12, 0, 0, 0, time.UTC)
		lastSweepAt := lastActivityAt.Add(-time.Hour)
		lastCutoffAt := lastSweepAt.AddDate(0, 0, -14)
		input := observepkg.Health{
			Status:         "ok",
			ActiveSessions: 1,
			Persistence: observepkg.PersistenceHealth{
				Status:             " degraded ",
				GlobalDBSizeBytes:  4096,
				SessionDBSizeBytes: 2048,
			},
			Retention: observepkg.RetentionHealth{
				Enabled:                  true,
				RetentionDays:            14,
				SweepIntervalSeconds:     86400,
				LastSweepStatus:          " ok ",
				LastSweepAt:              &lastSweepAt,
				LastCutoffAt:             &lastCutoffAt,
				LastSweepError:           " cleared ",
				DeletedEventSummaries:    3,
				DeletedTokenStats:        2,
				DeletedPermissionLogRows: 1,
			},
			Failures: observepkg.FailureHealth{
				Status: " degraded ",
				Total:  1,
				ByKind: map[store.FailureKind]int{store.FailureProtocol: 1},
				Recent: []observepkg.SessionFailureHealth{{
					SessionID:       " sess-protocol ",
					AgentName:       " coder ",
					Provider:        " claude ",
					WorkspaceID:     " ws-1 ",
					State:           " stopped ",
					FailureKind:     store.FailureProtocol,
					Summary:         "bad frame token=super-secret",
					CrashBundlePath: "/tmp/crash-token=super-secret.json",
					UpdatedAt:       lastActivityAt,
				}},
			},
			AgentProbes: []acp.ProbeResult{{
				AgentName:  " coder ",
				Provider:   " claude ",
				Command:    "agent --api-key=super-secret",
				Executable: " /usr/bin/agent ",
				Status:     " missing ",
				Error:      "resolve failed token=super-secret",
				CheckedAt:  lastActivityAt,
				DurationMS: 12,
			}},
			Activities: []observepkg.SessionActivityHealth{{
				SessionID:        " sess-activity ",
				TurnID:           " turn-activity ",
				LastActivityAt:   &lastActivityAt,
				LastActivityKind: "warning",
				CurrentTool:      "delegate_task",
				IdleSeconds:      900,
				Status:           "warning",
			}},
		}
		health := ObserveHealthPayloadFromHealth(&input)

		if got, want := len(health.Activities), 1; got != want {
			t.Fatalf("len(Activities) = %d, want %d", got, want)
		}
		activity := health.Activities[0]
		if activity.SessionID != "sess-activity" ||
			activity.TurnID != "turn-activity" ||
			activity.Status != "warning" ||
			activity.CurrentTool != "delegate_task" ||
			activity.IdleSeconds != 900 {
			t.Fatalf("Activities[0] = %#v, want trimmed runtime activity", activity)
		}
		if health.Persistence.Status != "degraded" ||
			health.Persistence.GlobalDBSizeBytes != 4096 ||
			health.Persistence.SessionDBSizeBytes != 2048 {
			t.Fatalf("Persistence = %#v, want typed persistence health", health.Persistence)
		}
		if !health.Retention.Enabled ||
			health.Retention.RetentionDays != 14 ||
			health.Retention.SweepIntervalSeconds != 86400 ||
			health.Retention.LastSweepStatus != "ok" ||
			health.Retention.LastSweepError != "cleared" ||
			health.Retention.DeletedEventSummaries != 3 ||
			health.Retention.DeletedTokenStats != 2 ||
			health.Retention.DeletedPermissionLogRows != 1 {
			t.Fatalf("Retention = %#v, want typed retention health", health.Retention)
		}
		if health.Retention.LastSweepAt == nil ||
			health.Retention.LastSweepAt == &lastSweepAt ||
			!health.Retention.LastSweepAt.Equal(lastSweepAt) {
			t.Fatalf("Retention.LastSweepAt = %#v, want cloned %s", health.Retention.LastSweepAt, lastSweepAt)
		}
		if health.Retention.LastCutoffAt == nil ||
			health.Retention.LastCutoffAt == &lastCutoffAt ||
			!health.Retention.LastCutoffAt.Equal(lastCutoffAt) {
			t.Fatalf("Retention.LastCutoffAt = %#v, want cloned %s", health.Retention.LastCutoffAt, lastCutoffAt)
		}
		if health.Failures.Status != "degraded" ||
			health.Failures.Total != 1 ||
			health.Failures.ByKind[store.FailureProtocol] != 1 ||
			len(health.Failures.Recent) != 1 {
			t.Fatalf("Failures = %#v, want classified lifecycle failure payload", health.Failures)
		}
		failure := health.Failures.Recent[0]
		if failure.SessionID != "sess-protocol" ||
			failure.FailureKind != store.FailureProtocol ||
			strings.Contains(failure.Summary, "super-secret") ||
			strings.Contains(failure.CrashBundlePath, "super-secret") {
			t.Fatalf("Failures.Recent[0] = %#v, want trimmed and redacted payload", failure)
		}
		if got, want := len(health.AgentProbes), 1; got != want {
			t.Fatalf("len(AgentProbes) = %d, want %d", got, want)
		}
		probe := health.AgentProbes[0]
		if probe.AgentName != "coder" ||
			probe.Provider != "claude" ||
			probe.Executable != "/usr/bin/agent" ||
			probe.Status != "missing" ||
			probe.DurationMS != 12 ||
			strings.Contains(probe.Command, "super-secret") ||
			strings.Contains(probe.Error, "super-secret") {
			t.Fatalf("AgentProbes[0] = %#v, want trimmed and redacted probe payload", probe)
		}
	})

	t.Run("Should encode task run statuses with their public names", func(t *testing.T) {
		t.Parallel()

		payload := TaskHealthPayloadFromObserve(observepkg.TaskHealth{
			StuckRuns: []observepkg.StuckTaskRun{{
				TaskID: "task-1",
				RunID:  "run-1",
				Status: taskpkg.TaskRunStatusRunning,
			}},
			RunTotals: []observepkg.TaskRunTotal{{
				Status: taskpkg.TaskRunStatusNeedsAttention,
				Count:  1,
			}},
		})

		if got, want := payload.StuckRuns[0].Status, taskpkg.TaskRunStatusRunning.String(); got != want {
			t.Fatalf("StuckRuns[0].Status = %q, want %q", got, want)
		}
		if got, want := payload.RunTotals[0].Status, taskpkg.TaskRunStatusNeedsAttention.String(); got != want {
			t.Fatalf("RunTotals[0].Status = %q, want %q", got, want)
		}
	})
}
