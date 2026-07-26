package core_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	bundlepkg "github.com/compozy/agh/internal/bundles"
	aghconfig "github.com/compozy/agh/internal/config"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestNetworkConversionHelpersPreserveMetadata(t *testing.T) {
	t.Parallel()

	deadline := int64(1775823000)

	t.Run("Should map status metadata", func(t *testing.T) {
		t.Parallel()

		status := &network.Status{
			Enabled:              true,
			Status:               network.StatusActive,
			LocalPeers:           1,
			Channels:             1,
			MessagesSent:         4,
			MessagesReceived:     5,
			MessagesRejected:     1,
			MessagesDelivered:    3,
			WorkflowTaggedEvents: 2,
			HandoffTaggedEvents:  1,
			KindMetrics: []network.KindMetric{{
				Kind:      network.KindSay,
				Sent:      4,
				Received:  5,
				Rejected:  1,
				Delivered: 3,
			}},
		}

		payload := core.NetworkStatusPayloadFromStatus(status)
		if payload == nil || payload.Channels != 1 || payload.MessagesDelivered != 3 ||
			len(payload.KindMetrics) != 1 ||
			payload.KindMetrics[0].Kind != string(network.KindSay) {
			t.Fatalf("NetworkStatusPayloadFromStatus() = %#v", payload)
		}
	})

	t.Run("Should omit empty coordination cost payloads", func(t *testing.T) {
		t.Parallel()

		if got := core.NetworkCoordinationCostPayloadFromStore(store.NetworkThreadSummary{}); got != nil {
			t.Fatalf("NetworkCoordinationCostPayloadFromStore(empty) = %#v, want nil", got)
		}
		cost := core.NetworkCoordinationCostPayloadFromStore(store.NetworkThreadSummary{DeliveredCount: 1})
		if cost == nil || cost.DeliveredCount != 1 {
			t.Fatalf("NetworkCoordinationCostPayloadFromStore(non-empty) = %#v, want delivered count", cost)
		}
	})

	t.Run("Should convert NetworkSendRequest preserving metadata", func(t *testing.T) {
		t.Parallel()

		req := contract.NetworkSendRequest{
			WorkspaceID: "ws-workspace",
			SessionID:   " sess-a ",
			Channel:     " builders ",
			Surface:     " thread ",
			ThreadID:    " thread_launch_db ",
			Kind:        "say",
			To:          " reviewer.sess-b ",
			Body:        json.RawMessage(`{"text":"hello"}`),
			WorkID:      " work-1 ",
			ReplyTo:     " msg-root ",
			TraceID:     " trace-1 ",
			CausationID: " cause-1 ",
			ExpiresAt:   &deadline,
			ID:          " msg-1 ",
			Ext: map[string]json.RawMessage{
				"agh.workflow_id":     json.RawMessage(`"wf-1"`),
				"agh.handoff_version": json.RawMessage(`3`),
			},
		}

		converted, err := core.NetworkSendRequestFromPayload(req)
		if err != nil {
			t.Fatalf("NetworkSendRequestFromPayload() error = %v", err)
		}
		if converted.SessionID != "sess-a" || converted.Channel != "builders" ||
			converted.Kind != network.KindSay {
			t.Fatalf("converted request = %#v", converted)
		}
		if converted.To == nil || *converted.To != "reviewer.sess-b" {
			t.Fatalf("converted.To = %#v, want reviewer.sess-b", converted.To)
		}
		if converted.Surface == nil || *converted.Surface != network.SurfaceThread {
			t.Fatalf("converted.Surface = %#v, want thread", converted.Surface)
		}
		if converted.ThreadID == nil || *converted.ThreadID != "thread_launch_db" {
			t.Fatalf("converted.ThreadID = %#v, want thread_launch_db", converted.ThreadID)
		}
		if converted.WorkID == nil || *converted.WorkID != "work-1" {
			t.Fatalf("converted.WorkID = %#v, want work-1", converted.WorkID)
		}
		if converted.ExpiresAt == nil || *converted.ExpiresAt != deadline {
			t.Fatalf("converted.ExpiresAt = %#v, want %d", converted.ExpiresAt, deadline)
		}
		if string(converted.Body) != `{"text":"hello"}` {
			t.Fatalf("converted.Body = %s, want preserved JSON", string(converted.Body))
		}
		if string(converted.Ext["agh.workflow_id"]) != `"wf-1"` ||
			string(converted.Ext["agh.handoff_version"]) != `3` {
			t.Fatalf("converted.Ext = %#v, want preserved ext metadata", converted.Ext)
		}
	})

	t.Run("Should reject raw claim token fields in NetworkSendRequest", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			req  contract.NetworkSendRequest
		}{
			{
				name: "Should reject body claim token",
				req: contract.NetworkSendRequest{
					WorkspaceID: "ws-workspace",
					SessionID:   "sess-a",
					Channel:     "builders",
					Kind:        "say",
					Body:        json.RawMessage(`{"nested":{"CLAIM_TOKEN":"agh_claim_secret"}}`),
				},
			},
			{
				name: "Should reject extension claim token",
				req: contract.NetworkSendRequest{
					WorkspaceID: "ws-workspace",
					SessionID:   "sess-a",
					Channel:     "builders",
					Kind:        "say",
					Body:        json.RawMessage(`{"text":"ok"}`),
					Ext: map[string]json.RawMessage{
						"agh.metadata": json.RawMessage(`{"claim_token":"agh_claim_secret"}`),
					},
				},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, err := core.NetworkSendRequestFromPayload(tc.req)
				if err == nil {
					t.Fatal("NetworkSendRequestFromPayload() error = nil, want raw token rejection")
				}
				if !errors.Is(err, core.ErrNetworkValidation) ||
					!errors.Is(err, contract.ErrRawClaimTokenMetadata) ||
					!strings.Contains(err.Error(), "network_raw_token_rejected") {
					t.Fatalf(
						"NetworkSendRequestFromPayload() error = %v, want network_raw_token_rejected validation",
						err,
					)
				}
			})
		}
	})

	t.Run(
		"Should allow claim token hash and benign claim prose in NetworkSendRequest",
		func(t *testing.T) {
			t.Parallel()

			_, err := core.NetworkSendRequestFromPayload(contract.NetworkSendRequest{
				WorkspaceID: "ws-workspace",
				SessionID:   "sess-a",
				Channel:     "builders",
				Surface:     "thread",
				ThreadID:    "thread_claim_docs",
				Kind:        "say",
				Body: json.RawMessage(
					`{"claim_token_hash":"sha256:abc","description":"see agh_claim_token docs"}`,
				),
			})
			if err != nil {
				t.Fatalf("NetworkSendRequestFromPayload() error = %v, want nil", err)
			}
		},
	)

	t.Run(
		"Should enforce conversation surface validation in NetworkSendRequest",
		func(t *testing.T) {
			t.Parallel()

			validBody := json.RawMessage(`{"text":"hello"}`)
			tests := []struct {
				name    string
				req     contract.NetworkSendRequest
				wantErr bool
			}{
				{
					name: "Should accept matching thread container",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "say",
						Body:        validBody,
					},
				},
				{
					name: "Should accept matching direct container with work",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "direct",
						DirectID:    "direct_99401d24bee62651d189e5a561785466",
						Kind:        "receipt",
						WorkID:      "work-1",
						Body:        json.RawMessage(`{"status":"accepted"}`),
					},
				},
				{
					name: "Should accept directed capability with work",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "capability",
						To:          "reviewer.sess-b",
						WorkID:      "work-1",
						Body:        json.RawMessage(`{"capability":{"id":"review"}}`),
					},
				},
				{
					name: "Should reject missing surface",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Kind:        "say",
						Body:        validBody,
					},
					wantErr: true,
				},
				{
					name: "Should reject opposite container for thread surface",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						DirectID:    "direct_99401d24bee62651d189e5a561785466",
						Kind:        "say",
						Body:        validBody,
					},
					wantErr: true,
				},
				{
					name: "Should reject opposite container for direct surface",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "direct",
						ThreadID:    "thread_launch_db",
						Kind:        "say",
						Body:        validBody,
					},
					wantErr: true,
				},
				{
					name: "Should reject greet conversation fields",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "greet",
						Body:        json.RawMessage(`{"hello":"world"}`),
					},
					wantErr: true,
				},
				{
					name: "Should reject capability without directed target",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "capability",
						WorkID:      "work-1",
						Body:        json.RawMessage(`{"capability":{"id":"review"}}`),
					},
					wantErr: true,
				},
				{
					name: "Should reject lifecycle say without directed target",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "say",
						WorkID:      "work-1",
						Body:        validBody,
					},
					wantErr: true,
				},
				{
					name: "Should reject receipt without work",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "receipt",
						Body:        json.RawMessage(`{"status":"accepted"}`),
					},
					wantErr: true,
				},
				{
					name: "Should reject trace without work",
					req: contract.NetworkSendRequest{
						WorkspaceID: "ws-workspace",
						SessionID:   "sess-a",
						Channel:     "builders",
						Surface:     "thread",
						ThreadID:    "thread_launch_db",
						Kind:        "trace",
						Body:        json.RawMessage(`{"event":"progress"}`),
					},
					wantErr: true,
				},
			}

			for _, tc := range tests {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					_, err := core.NetworkSendRequestFromPayload(tc.req)
					if tc.wantErr {
						if err == nil {
							t.Fatal(
								"NetworkSendRequestFromPayload() error = nil, want validation error",
							)
						}
						if !errors.Is(err, core.ErrNetworkValidation) {
							t.Fatalf(
								"NetworkSendRequestFromPayload() error = %v, want network validation",
								err,
							)
						}
						return
					}
					if err != nil {
						t.Fatalf("NetworkSendRequestFromPayload() error = %v, want nil", err)
					}
				})
			}
		},
	)

	t.Run("Should convert Envelope preserving metadata", func(t *testing.T) {
		t.Parallel()

		proof := network.Proof{"sig": json.RawMessage(`"abc123"`)}
		traceID := "trace-1"
		causationID := "cause-1"
		replyTo := "msg-root"
		envelope := network.Envelope{
			Protocol:    network.ProtocolV0,
			ID:          "msg-1",
			Kind:        network.KindSay,
			Channel:     "builders",
			Surface:     new(network.SurfaceDirect),
			DirectID:    new("direct_0123456789abcdef0123456789abcdef"),
			From:        "reviewer.sess-b",
			ReplyTo:     &replyTo,
			TraceID:     &traceID,
			CausationID: &causationID,
			TS:          deadline,
			ExpiresAt:   &deadline,
			Body:        json.RawMessage(`{"text":"hello","intent":"review"}`),
			Proof:       &proof,
			Ext: network.ExtensionMap{
				"agh.workflow_id":     json.RawMessage(`"wf-1"`),
				"agh.handoff_version": json.RawMessage(`3`),
			},
		}

		envelopePayload := core.NetworkEnvelopePayloadFromEnvelope(envelope)
		if envelopePayload.TraceID == nil || *envelopePayload.TraceID != "trace-1" {
			t.Fatalf("TraceID = %#v, want trace-1", envelopePayload.TraceID)
		}
		if envelopePayload.ExpiresAt == nil || *envelopePayload.ExpiresAt != deadline {
			t.Fatalf("ExpiresAt = %#v, want %d", envelopePayload.ExpiresAt, deadline)
		}
		if string(envelopePayload.Proof["sig"]) != `"abc123"` {
			t.Fatalf("Proof = %#v, want cloned proof payload", envelopePayload.Proof)
		}
		if string(envelopePayload.Ext["agh.handoff_version"]) != `3` {
			t.Fatalf("Ext = %#v, want cloned ext payload", envelopePayload.Ext)
		}
	})

	t.Run("Should fall back to peer id when peer-card display name is blank", func(t *testing.T) {
		t.Parallel()

		blank := "   "
		payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
			PeerID:   "reviewer.sess-b",
			Channel:  "builders",
			Local:    true,
			PeerCard: network.PeerCard{PeerID: "reviewer.sess-b", DisplayName: &blank},
		})

		if got, want := payload.DisplayName, "reviewer.sess-b"; got != want {
			t.Fatalf("payload.DisplayName = %q, want %q", got, want)
		}
	})

	t.Run("Should expose local presence state", func(t *testing.T) {
		t.Parallel()

		payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
			PeerID:        "reviewer.sess-b",
			Channel:       "builders",
			Local:         true,
			PresenceState: network.PresenceStateLocal,
			PeerCard:      network.PeerCard{PeerID: "reviewer.sess-b"},
		})

		if got, want := payload.PresenceState, contract.NetworkPresenceLocal; got != want {
			t.Fatalf("payload.PresenceState = %q, want %q", got, want)
		}
	})

	t.Run("Should default missing presence to local", func(t *testing.T) {
		t.Parallel()

		payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
			PeerID:   "reviewer.sess-local",
			Channel:  "builders",
			Local:    true,
			PeerCard: network.PeerCard{PeerID: "reviewer.sess-local"},
		})
		if payload.PresenceState != contract.NetworkPresenceLocal {
			t.Fatalf("PresenceState = %q, want local", payload.PresenceState)
		}
	})

	t.Run("Should normalize empty peer-card arrays to empty slices", func(t *testing.T) {
		t.Parallel()

		payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
			PeerID:   "founder.sess-empty",
			Channel:  "builders",
			Local:    true,
			PeerCard: network.PeerCard{PeerID: "founder.sess-empty"},
		})

		if payload.PeerCard.ProfilesSupported == nil {
			t.Fatal("payload.PeerCard.ProfilesSupported = nil, want empty slice")
		}
		if got := len(payload.PeerCard.ProfilesSupported); got != 0 {
			t.Fatalf("len(payload.PeerCard.ProfilesSupported) = %d, want 0", got)
		}
		if payload.PeerCard.ArtifactsSupported == nil {
			t.Fatal("payload.PeerCard.ArtifactsSupported = nil, want empty slice")
		}
		if got := len(payload.PeerCard.ArtifactsSupported); got != 0 {
			t.Fatalf("len(payload.PeerCard.ArtifactsSupported) = %d, want 0", got)
		}
		if payload.PeerCard.TrustModesSupported == nil {
			t.Fatal("payload.PeerCard.TrustModesSupported = nil, want empty slice")
		}
		if got := len(payload.PeerCard.TrustModesSupported); got != 0 {
			t.Fatalf("len(payload.PeerCard.TrustModesSupported) = %d, want 0", got)
		}
	})

	t.Run("Should clone capability brief peer-card metadata", func(t *testing.T) {
		t.Parallel()

		brief := json.RawMessage(`[{"id":"review-pr","summary":"Review pull requests"}]`)
		payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
			PeerID:  "reviewer.sess-b",
			Channel: "builders",
			Local:   true,
			PeerCard: network.PeerCard{
				PeerID:              "reviewer.sess-b",
				ProfilesSupported:   []string{network.ProtocolV0},
				Capabilities:        []string{"review-pr"},
				ArtifactsSupported:  []string{},
				TrustModesSupported: []string{},
				Ext: network.ExtensionMap{
					"agh.capabilities_brief": brief,
				},
			},
		})

		brief[0] = '{'
		if got, want := payload.PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{{
			ID:      "review-pr",
			Summary: "Review pull requests",
		}}; !reflect.DeepEqual(got, want) {
			t.Fatalf("payload capability brief = %#v, want %#v", got, want)
		}
		if _, ok := payload.PeerCard.Ext["agh.capabilities_brief"]; ok {
			t.Fatalf("payload capability brief ext should be stripped: %#v", payload.PeerCard.Ext)
		}
	})

	t.Run(
		"Should preserve greet brief summaries when rich catalog is filtered",
		func(t *testing.T) {
			t.Parallel()

			payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
				PeerID:  "reviewer.sess-c",
				Channel: "builders",
				Local:   false,
				PeerCard: network.PeerCard{
					PeerID:       "reviewer.sess-c",
					Capabilities: []string{"review-pr", "ship-release"},
					Ext: network.ExtensionMap{
						"agh.capabilities_brief": json.RawMessage(`[
						{"id":"review-pr","summary":"Review pull requests"},
						{"id":"ship-release","summary":"Ship releases"}
					]`),
					},
				},
				CapabilityCatalog: []session.NetworkPeerCapability{{
					ID:      "review-pr",
					Summary: "Review pull requests",
					Outcome: "Actionable review findings",
				}},
				CapabilityCatalogKnown: true,
			})

			if got, want := payload.PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{
				{ID: "review-pr", Summary: "Review pull requests"},
				{ID: "ship-release", Summary: "Ship releases"},
			}; !reflect.DeepEqual(got, want) {
				t.Fatalf(
					"payload capability brief with filtered catalog = %#v, want %#v",
					got,
					want,
				)
			}
		},
	)

	t.Run(
		"Should ignore stale rich catalog entries when the catalog is not known",
		func(t *testing.T) {
			t.Parallel()

			payload := core.NetworkPeerPayloadFromInfo(network.PeerInfo{
				PeerID:  "reviewer.sess-d",
				Channel: "builders",
				Local:   false,
				PeerCard: network.PeerCard{
					PeerID:       "reviewer.sess-d",
					Capabilities: []string{"review-pr", "ship-release"},
					Ext: network.ExtensionMap{
						"agh.capabilities_brief": json.RawMessage(`[
						{"id":"review-pr","summary":"Review pull requests"},
						{"id":"ship-release","summary":"Ship releases"}
					]`),
					},
				},
				CapabilityCatalog: []session.NetworkPeerCapability{{
					ID:      "review-pr",
					Summary: "STALE SUMMARY",
					Outcome: "Actionable review findings",
				}},
				CapabilityCatalogKnown: false,
			})

			if got, want := payload.PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{
				{ID: "review-pr", Summary: "Review pull requests"},
				{ID: "ship-release", Summary: "Ship releases"},
			}; !reflect.DeepEqual(got, want) {
				t.Fatalf("payload capability brief with unknown catalog = %#v, want %#v", got, want)
			}
		},
	)

	t.Run("Should reject coordinator fanout without a coordinator peer before creating sessions", func(t *testing.T) {
		t.Parallel()

		manager := testutil.StubSessionManager{
			CreateFn: func(context.Context, session.CreateOpts) (*session.Session, error) {
				t.Fatal("Create() should not be called for invalid coordinator fanout")
				return nil, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-workspace" {
					t.Fatalf("Resolve() ref = %q, want ws-workspace", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-workspace", Name: "Workspace"},
					WorkspaceID: "ws-workspace",
					Agents:      []aghconfig.AgentDef{{Name: "coder"}},
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			WriteNetworkChannelFn: func(context.Context, store.NetworkChannelEntry) error {
				t.Fatal("WriteNetworkChannel() should not be called for invalid coordinator fanout")
				return nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels",
			[]byte(
				`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder"],"fanout_policy":"coordinator"}`,
			),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("create channel code = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}
	})
}

func networkTestSessionManager(
	workspaceID string,
	sessionIDs ...string,
) testutil.StubSessionManager {
	allowed := make(map[string]struct{}, len(sessionIDs))
	for _, id := range sessionIDs {
		allowed[strings.TrimSpace(id)] = struct{}{}
	}
	return testutil.StubSessionManager{
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			trimmedID := strings.TrimSpace(id)
			if _, ok := allowed[trimmedID]; !ok {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:          trimmedID,
				WorkspaceID: strings.TrimSpace(workspaceID),
				State:       session.StateActive,
			}, nil
		},
	}
}

func TestBundleActivationPayloadUsesMaterializedStableIDs(t *testing.T) {
	t.Parallel()

	t.Run("Should use materialized stable IDs", func(t *testing.T) {
		t.Parallel()

		preview := bundlepkg.ActivationPreview{
			Activation: bundlepkg.Activation{
				ID:            "act_marketing",
				ExtensionName: "marketing-team",
				BundleName:    "marketing",
				ProfileName:   "default",
				Scope:         bundlepkg.ScopeGlobal,
			},
			Bundle: extensionpkg.BundleSpec{
				Name: "marketing",
			},
			Profile: extensionpkg.BundleProfile{
				Name: "default",
				Layouts: []extensionpkg.BundleLayout{{
					Path: "layouts/two-up.json",
					Layout: windowmanager.LayoutResource{
						ID:               "two-up",
						DisplayName:      "Two up",
						AspectVariant:    windowmanager.LayoutAspectLandscape,
						ParticipantSlots: []windowmanager.WindowID{"primary", "secondary"},
						OverflowPolicy:   windowmanager.LayoutOverflowStack,
					},
				}},
				Agents: []extensionpkg.BundleAgent{{
					Path: "agents/planner",
					Agent: aghconfig.AgentDef{
						Name:   "planner",
						Model:  "sonnet",
						Prompt: "Plan campaign work.",
					},
					Soul: &extensionpkg.BundleAgentSidecar{
						SourcePath: "agents/planner/SOUL.md",
						Body:       "Lead planning.",
					},
				}},
				Jobs: []extensionpkg.BundleJob{{
					Name:      "daily-sync",
					AgentName: "planner",
				}},
				Triggers: []extensionpkg.BundleTrigger{{
					Name:      "session-opened",
					AgentName: "planner",
					Event:     "session.created",
				}},
				Bridges: []extensionpkg.BundleBridgePreset{{
					Name:        "telegram-main",
					DisplayName: "Marketing Telegram",
				}},
			},
		}

		payload := core.BundleActivationPayload(preview)
		if got, want := payload.Layouts[0].ID, bundleStableIDForTest(
			"lay",
			preview.Activation.ID,
			"two-up",
		); got != want {
			t.Fatalf("payload.Layouts[0].ID = %q, want %q", got, want)
		}
		if got, want := payload.Layouts[0].ParticipantSlots,
			[]string{"primary", "secondary"}; !slices.Equal(got, want) {
			t.Fatalf("payload.Layouts[0].ParticipantSlots = %#v, want %#v", got, want)
		}
		if got, want := payload.Agents[0].ID, bundleStableIDForTest(
			"agt",
			preview.Activation.ID,
			"planner",
		); got != want {
			t.Fatalf("payload.Agents[0].ID = %q, want %q", got, want)
		}
		if !payload.Agents[0].HasSoul || payload.Agents[0].HasHeartbeat {
			t.Fatalf("payload.Agents[0] sidecar flags = %#v", payload.Agents[0])
		}
		if got, want := payload.Jobs[0].ID, bundleStableIDForTest(
			"job",
			preview.Activation.ID,
			"daily-sync",
		); got != want {
			t.Fatalf("payload.Jobs[0].ID = %q, want %q", got, want)
		}
		if got, want := payload.Triggers[0].ID, bundleStableIDForTest(
			"trg",
			preview.Activation.ID,
			"session-opened",
		); got != want {
			t.Fatalf("payload.Triggers[0].ID = %q, want %q", got, want)
		}
		if got, want := payload.Bridges[0].ID, bundleStableIDForTest(
			"bri",
			preview.Activation.ID,
			"telegram-main",
		); got != want {
			t.Fatalf("payload.Bridges[0].ID = %q, want %q", got, want)
		}
	})
}

func TestStatusForBundleErrorDefaultsToInternalServerError(t *testing.T) {
	t.Parallel()

	t.Run("Should map known bundle errors and default unknown errors", func(t *testing.T) {
		t.Parallel()

		if got, want := core.StatusForBundleError(errors.New("store failed")),
			http.StatusInternalServerError; got != want {
			t.Fatalf("StatusForBundleError(unknown) = %d, want %d", got, want)
		}
		if got, want := core.StatusForBundleError(bundlepkg.ErrActivationNotFound), http.StatusNotFound; got != want {
			t.Fatalf("StatusForBundleError(ErrActivationNotFound) = %d, want %d", got, want)
		}
	})
}

func bundleStableIDForTest(prefix string, parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return prefix + "_" + hex.EncodeToString(sum[:8])
}

func TestBaseHandlersNetworkConversationReadPaths(t *testing.T) {
	t.Parallel()

	openedAt := time.Date(2026, 4, 3, 12, 0, 1, 0, time.UTC)
	threadID := "thread_launch_db"
	directID := "direct_99401d24bee62651d189e5a561785466"
	workID := "work-1"
	fixture := newHandlerFixture(
		t,
		networkTestSessionManager("ws-workspace", "sess-a"),
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.Config.Network.Enabled = true
	fixture.Handlers.Network = testutil.StubNetworkService{}
	fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
		ListThreadsFn: func(
			_ context.Context,
			ref store.NetworkChannelRef,
			query store.NetworkThreadQuery,
		) (store.NetworkThreadPage, error) {
			if ref.WorkspaceID == "" || ref.Channel != "builders" || query.Limit != 5 ||
				query.After != "thread_before" {
				t.Fatalf(
					"ListThreads() ref=%#v query=%#v, want workspace builders limit/after",
					ref,
					query,
				)
			}
			return store.NetworkThreadPage{Threads: []store.NetworkThreadSummary{{
				WorkspaceID:           ref.WorkspaceID,
				Channel:               ref.Channel,
				ThreadID:              threadID,
				RootMessageID:         "msg-root",
				Title:                 "Launch DB",
				OpenedByPeerID:        "coder.sess-abc",
				OpenedSessionID:       "sess-local",
				OpenedAt:              openedAt,
				LastActivityAt:        openedAt,
				MessageCount:          2,
				ParticipantCount:      2,
				OpenWorkCount:         1,
				DeliveredCount:        3,
				PromptSizeBytes:       1024,
				EstimatedPromptTokens: 256,
			}}, Total: 7, Limit: 5, HasMore: true, NextCursor: "opaque-thread-cursor"}, nil
		},
		GetThreadFn: func(_ context.Context, ref store.NetworkChannelRef, gotThreadID string) (store.NetworkThreadSummary, error) {
			if ref.WorkspaceID == "" || ref.Channel != "builders" || gotThreadID != threadID {
				t.Fatalf(
					"GetThread() ref=%#v threadID=%q, want workspace builders/%s",
					ref,
					gotThreadID,
					threadID,
				)
			}
			return store.NetworkThreadSummary{
				WorkspaceID:           ref.WorkspaceID,
				Channel:               ref.Channel,
				ThreadID:              gotThreadID,
				RootMessageID:         "msg-root",
				OpenedAt:              openedAt,
				LastActivityAt:        openedAt,
				DeliveredCount:        2,
				PromptSizeBytes:       640,
				EstimatedPromptTokens: 160,
			}, nil
		},
		ListNetworkThreadSessionTokenStatsFn: func(
			_ context.Context,
			query store.NetworkThreadSessionTokenStatsQuery,
		) ([]store.NetworkThreadSessionTokenStats, error) {
			if query.WorkspaceID == "" || query.Channel != "builders" || query.ThreadID != threadID {
				t.Fatalf("ListNetworkThreadPeerTokenStats() query=%#v, want builders/%s", query, threadID)
			}
			return []store.NetworkThreadSessionTokenStats{{
				WorkspaceID:           query.WorkspaceID,
				Channel:               query.Channel,
				ThreadID:              query.ThreadID,
				SessionID:             "sess-reviewer",
				DeliveredCount:        2,
				PromptSizeBytes:       640,
				EstimatedPromptTokens: 160,
				FirstDeliveredAt:      openedAt,
				LastDeliveredAt:       openedAt.Add(time.Minute),
			}}, nil
		},
		ListDirectRoomsFn: func(
			_ context.Context,
			ref store.NetworkChannelRef,
			query store.NetworkDirectRoomQuery,
		) (store.NetworkDirectRoomPage, error) {
			if ref.WorkspaceID == "" || ref.Channel != "builders" ||
				query.SessionID != "reviewer.sess-xyz" ||
				query.Limit != 3 {
				t.Fatalf(
					"ListDirectRooms() ref=%#v query=%#v, want workspace builders reviewer limit",
					ref,
					query,
				)
			}
			return store.NetworkDirectRoomPage{Directs: []store.NetworkDirectRoomSummary{{
				WorkspaceID:    ref.WorkspaceID,
				Channel:        ref.Channel,
				DirectID:       directID,
				SessionA:       "sess-coder",
				SessionB:       "sess-reviewer",
				OpenedAt:       openedAt,
				LastActivityAt: openedAt,
				MessageCount:   1,
				OpenWorkCount:  1,
			}}, Total: 4, Limit: 3, HasMore: true, NextCursor: "opaque-direct-cursor"}, nil
		},
		GetDirectRoomFn: func(_ context.Context, ref store.NetworkChannelRef, gotDirectID string) (store.NetworkDirectRoomSummary, error) {
			if ref.WorkspaceID == "" || ref.Channel != "builders" || gotDirectID != directID {
				t.Fatalf(
					"GetDirectRoom() ref=%#v directID=%q, want workspace builders/%s",
					ref,
					gotDirectID,
					directID,
				)
			}
			return store.NetworkDirectRoomSummary{
				WorkspaceID:    ref.WorkspaceID,
				Channel:        ref.Channel,
				DirectID:       gotDirectID,
				SessionA:       "sess-coder",
				SessionB:       "sess-reviewer",
				OpenedAt:       openedAt,
				LastActivityAt: openedAt,
			}, nil
		},
		ListConversationMessagesFn: func(
			_ context.Context,
			ref store.NetworkConversationRef,
			query store.NetworkConversationMessageQuery,
		) ([]store.NetworkConversationMessage, error) {
			if ref.Channel != "builders" || query.Kind != "say" || query.WorkID != workID {
				t.Fatalf(
					"ListConversationMessages() ref=%#v query=%#v, want builders say work",
					ref,
					query,
				)
			}
			switch ref.Surface {
			case store.NetworkSurfaceThread:
				if ref.ThreadID != threadID || ref.DirectID != "" {
					t.Fatalf("thread ref = %#v, want %s", ref, threadID)
				}
			case store.NetworkSurfaceDirect:
				if ref.DirectID != directID || ref.ThreadID != "" {
					t.Fatalf("direct ref = %#v, want %s", ref, directID)
				}
			default:
				t.Fatalf("ref.Surface = %q, want thread or direct", ref.Surface)
			}
			return []store.NetworkConversationMessage{{
				MessageID:   "msg-1",
				SessionID:   "sess-local",
				Channel:     "builders",
				Surface:     ref.Surface,
				ThreadID:    ref.ThreadID,
				DirectID:    ref.DirectID,
				Direction:   network.AuditDirectionReceived,
				PeerFrom:    "reviewer.sess-xyz",
				Kind:        string(network.KindSay),
				WorkID:      workID,
				PreviewText: "hello",
				SizeBytes:   123,
				Body:        json.RawMessage(`{"text":"hello"}`),
				Timestamp:   openedAt,
			}}, nil
		},
		GetWorkFn: func(_ context.Context, workspaceID string, gotWorkID string) (store.NetworkWorkEntry, error) {
			if workspaceID == "" || gotWorkID != workID {
				t.Fatalf(
					"GetWork() workspaceID=%q workID=%q, want workspace/%s",
					workspaceID,
					gotWorkID,
					workID,
				)
			}
			return store.NetworkWorkEntry{
				WorkID:         gotWorkID,
				WorkspaceID:    workspaceID,
				Channel:        "builders",
				Surface:        store.NetworkSurfaceThread,
				ThreadID:       threadID,
				State:          store.NetworkWorkStateSubmitted,
				OpenedAt:       openedAt,
				LastActivityAt: openedAt,
			}, nil
		},
	}

	t.Run("Should list threads", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/threads?limit=5&after=thread_before",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"threads status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkThreadsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Threads) != 1 || payload.Threads[0].ThreadID != threadID {
			t.Fatalf("threads payload = %#v, want thread %s", payload.Threads, threadID)
		}
		if payload.Threads[0].CoordinationCost == nil {
			t.Fatalf("threads[0].CoordinationCost = nil, want prompt cost payload")
		}
		if got, want := payload.Threads[0].CoordinationCost.PromptSizeBytes, int64(1024); got != want {
			t.Fatalf("threads[0].CoordinationCost.PromptSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Page.Total, 7; got != want || payload.Page.Limit != 5 || !payload.Page.HasMore ||
			payload.Page.NextCursor != "opaque-thread-cursor" {
			t.Fatalf("threads page = %#v, want total=%d limit=5 has_more cursor", payload.Page, want)
		}
	})

	t.Run("Should show thread", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/threads/"+threadID,
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"thread status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkThreadResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Thread.ThreadID != threadID {
			t.Fatalf("thread payload = %#v, want %s", payload.Thread, threadID)
		}
		if payload.Thread.CoordinationCost == nil {
			t.Fatalf("thread.CoordinationCost = nil, want prompt cost payload")
		}
		if got, want := payload.Thread.CoordinationCost.EstimatedPromptTokens, int64(160); got != want {
			t.Fatalf("thread.CoordinationCost.EstimatedPromptTokens = %d, want %d", got, want)
		}
		if len(payload.SessionCosts) != 1 || payload.SessionCosts[0].SessionID != "sess-reviewer" {
			t.Fatalf("thread session_costs = %#v, want reviewer session cost", payload.SessionCosts)
		}
		if got, want := payload.SessionCosts[0].PromptSizeBytes, int64(640); got != want {
			t.Fatalf("thread session_costs[0].PromptSizeBytes = %d, want %d", got, want)
		}
	})

	t.Run("Should list thread messages", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/threads/"+threadID+"/messages?kind=say&work_id="+workID,
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"thread messages status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkThreadMessagesResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Messages) != 1 || payload.Messages[0].Surface != store.NetworkSurfaceThread {
			t.Fatalf("thread messages payload = %#v, want thread message", payload.Messages)
		}
		if got, want := payload.Page.Limit, store.NetworkMessageDefaultLimit; got != want {
			t.Fatalf("thread messages page limit = %d, want %d", got, want)
		}
		if got, want := payload.Messages[0].SizeBytes, int64(123); got != want {
			t.Fatalf("thread messages[0].SizeBytes = %d, want %d", got, want)
		}
	})

	t.Run("Should list direct rooms", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/directs?session_id=reviewer.sess-xyz&limit=3",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"directs status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkDirectRoomsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Directs) != 1 || payload.Directs[0].DirectID != directID {
			t.Fatalf("directs payload = %#v, want direct %s", payload.Directs, directID)
		}
		if got, want := payload.Page.Total, 4; got != want || payload.Page.Limit != 3 || !payload.Page.HasMore ||
			payload.Page.NextCursor != "opaque-direct-cursor" {
			t.Fatalf("directs page = %#v, want total=%d limit=3 has_more cursor", payload.Page, want)
		}
	})

	t.Run("Should show direct room", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/directs/"+directID,
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"direct status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkDirectRoomResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Direct.DirectID != directID {
			t.Fatalf("direct payload = %#v, want %s", payload.Direct, directID)
		}
	})

	t.Run("Should list direct messages", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/directs/"+directID+"/messages?kind=say&work_id="+workID,
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"direct messages status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkDirectRoomMessagesResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Messages) != 1 || payload.Messages[0].Surface != store.NetworkSurfaceDirect {
			t.Fatalf("direct messages payload = %#v, want direct message", payload.Messages)
		}
	})

	t.Run("Should show work", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/work/"+workID,
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"work status = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkWorkResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Work.WorkID != workID || payload.Work.ThreadID != threadID {
			t.Fatalf(
				"work payload = %#v, want work %s on thread %s",
				payload.Work,
				workID,
				threadID,
			)
		}
	})
}

func TestBaseHandlersNetworkEndpoints(t *testing.T) {
	t.Parallel()

	directID, directSessionA, directSessionB, err := store.NetworkDirectRoomIdentity(
		"ws-workspace",
		"builders",
		"sess-a",
		"sess-remote",
	)
	if err != nil {
		t.Fatalf("DirectRoomIdentity() error = %v", err)
	}

	fixture := newHandlerFixture(
		t,
		networkTestSessionManager("ws-workspace", "sess-a"),
		testutil.StubObserver{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.Config.Network.Enabled = true

	fixedNow := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC)
	fixture.Handlers.Network = testutil.StubNetworkService{
		StatusFn: func(context.Context) (*network.Status, error) {
			return &network.Status{
				Enabled:              true,
				Status:               network.StatusActive,
				LocalPeers:           1,
				Channels:             1,
				MessagesSent:         4,
				MessagesReceived:     5,
				MessagesRejected:     1,
				MessagesDelivered:    3,
				WorkflowTaggedEvents: 2,
				HandoffTaggedEvents:  1,
				KindMetrics: []network.KindMetric{{
					Kind:      network.KindSay,
					Sent:      4,
					Received:  5,
					Rejected:  1,
					Delivered: 3,
				}},
			}, nil
		},
		ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
			if channel != "builders" && channel != "" {
				t.Fatalf("ListPeers() channel = %q, want builders or empty", channel)
			}
			displayName := "Reviewer"
			sessionID := "sess-a"
			peers := []network.PeerInfo{{
				SessionID: &sessionID,
				PeerID:    "reviewer.sess-a",
				Channel:   "builders",
				Local:     true,
				PeerCard: network.PeerCard{
					PeerID:              "reviewer.sess-a",
					DisplayName:         &displayName,
					ProfilesSupported:   []string{"v0"},
					Capabilities:        []string{"send"},
					ArtifactsSupported:  []string{"text"},
					TrustModesSupported: []string{"untrusted"},
					Ext: network.ExtensionMap{
						"agh.workflow_id": json.RawMessage(`"wf-1"`),
						"agh.capabilities_brief": json.RawMessage(
							`[{"id":"send","summary":"Send channel updates"}]`,
						),
					},
				},
				JoinedAt: new(fixedNow),
			}}
			if channel == "" {
				remoteDisplayName := "Coder"
				remoteSessionID := "sess-remote"
				peers = append(peers, network.PeerInfo{
					SessionID: &remoteSessionID,
					PeerID:    "coder.sess-remote",
					Channel:   "builders",
					Local:     true,
					PeerCard: network.PeerCard{
						PeerID:      "coder.sess-remote",
						DisplayName: &remoteDisplayName,
					},
					JoinedAt: new(fixedNow),
				})
			}
			return peers, nil
		},
		ListChannelsFn: func(context.Context, string) ([]network.ChannelInfo, error) {
			return []network.ChannelInfo{{Channel: "builders", PeerCount: 2}}, nil
		},
		SendFn: func(_ context.Context, req network.SendRequest) (string, error) {
			if req.Surface != nil && *req.Surface == network.SurfaceDirect {
				if req.SessionID != "sess-a" || req.Channel != "builders" ||
					req.Kind != network.KindSay {
					t.Fatalf("direct Send() req = %#v", req)
				}
				if req.DirectID == nil || *req.DirectID != directID {
					t.Fatalf("direct Send() direct_id = %#v, want %q", req.DirectID, directID)
				}
				if req.To == nil || *req.To != "coder.sess-remote" {
					t.Fatalf("direct Send() to = %#v, want coder.sess-remote", req.To)
				}
				return "msg-direct", nil
			}
			if req.SessionID != "sess-a" || req.Channel != "builders" ||
				req.Kind != network.KindSay {
				t.Fatalf("Send() req = %#v", req)
			}
			if string(req.Ext["agh.workflow_id"]) != `"wf-1"` ||
				string(req.Ext["agh.handoff_version"]) != `3` {
				t.Fatalf("Send() ext = %#v, want workflow/handoff metadata", req.Ext)
			}
			return "msg-1", nil
		},
		InboxFn: func(_ context.Context, sessionID string) ([]network.Envelope, error) {
			if sessionID != "sess-a" {
				t.Fatalf("Inbox() sessionID = %q, want sess-a", sessionID)
			}
			replyTo := "msg-root"
			traceID := "trace-1"
			proof := network.Proof{"sig": json.RawMessage(`"abc123"`)}
			return []network.Envelope{{
				Protocol: network.ProtocolV0,
				ID:       "msg-inbox",
				Kind:     network.KindSay,
				Channel:  "builders",
				Surface:  new(network.SurfaceDirect),
				DirectID: new("direct_0123456789abcdef0123456789abcdef"),
				From:     "reviewer.sess-a",
				ReplyTo:  &replyTo,
				TraceID:  &traceID,
				TS:       fixedNow.Unix(),
				Body:     json.RawMessage(`{"text":"follow up","intent":"review"}`),
				Proof:    &proof,
				Ext: network.ExtensionMap{
					"agh.workflow_id":     json.RawMessage(`"wf-1"`),
					"agh.handoff_version": json.RawMessage(`3`),
				},
			}}, nil
		},
	}
	networkChannelKey := func(workspaceID string, channel string) string {
		return strings.TrimSpace(workspaceID) + "\x00" + strings.TrimSpace(channel)
	}
	networkChannels := map[string]store.NetworkChannelEntry{
		networkChannelKey("ws-workspace", "builders"): {
			WorkspaceID:  "ws-workspace",
			Channel:      "builders",
			Purpose:      "Build coordination",
			FanoutPolicy: store.NetworkFanoutPolicyCapabilityMatch,
			CreatedBy:    "test",
			CreatedAt:    fixedNow,
			UpdatedAt:    fixedNow,
		},
	}
	networkSubscriptions := make([]store.NetworkSubscriptionEntry, 0)
	var networkStateMu sync.Mutex
	fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
		GetDirectRoomFn: func(_ context.Context, ref store.NetworkChannelRef, gotDirectID string) (store.NetworkDirectRoomSummary, error) {
			if ref.WorkspaceID != "ws-workspace" || ref.Channel != "builders" ||
				gotDirectID != directID {
				t.Fatalf(
					"GetDirectRoom() ref = %#v direct_id=%q, want ws-workspace/builders/%s",
					ref,
					gotDirectID,
					directID,
				)
			}
			return store.NetworkDirectRoomSummary{
				WorkspaceID: "ws-workspace",
				Channel:     "builders",
				DirectID:    directID,
				SessionA:    directSessionA,
				SessionB:    directSessionB,
			}, nil
		},
		GetNetworkChannelFn: func(_ context.Context, ref store.NetworkChannelRef) (store.NetworkChannelEntry, error) {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			entry, ok := networkChannels[networkChannelKey(ref.WorkspaceID, ref.Channel)]
			if !ok {
				return store.NetworkChannelEntry{}, sql.ErrNoRows
			}
			return entry, nil
		},
		WriteNetworkChannelFn: func(_ context.Context, entry store.NetworkChannelEntry) error {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			networkChannels[networkChannelKey(entry.WorkspaceID, entry.Channel)] = entry
			return nil
		},
		PatchNetworkChannelFn: func(_ context.Context, ref store.NetworkChannelRef, patch store.NetworkChannelPatch) error {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			key := networkChannelKey(ref.WorkspaceID, ref.Channel)
			entry, ok := networkChannels[key]
			if !ok {
				return sql.ErrNoRows
			}
			networkChannels[key] = patch.Apply(entry)
			return nil
		},
		ListNetworkChannelsFn: func(context.Context, store.NetworkChannelQuery) ([]store.NetworkChannelEntry, error) {
			return nil, nil
		},
		ListNetworkMessagesFn: func(context.Context, store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
			return nil, nil
		},
		PutNetworkSubscriptionFn: func(_ context.Context, entry store.NetworkSubscriptionEntry) error {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			networkSubscriptions = append(networkSubscriptions, entry)
			return nil
		},
		PutNetworkSubscriptionWithChannelFn: func(
			_ context.Context,
			channel store.NetworkChannelEntry,
			entry store.NetworkSubscriptionEntry,
		) error {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			key := networkChannelKey(channel.WorkspaceID, channel.Channel)
			if _, exists := networkChannels[key]; !exists {
				networkChannels[key] = channel
			}
			networkSubscriptions = append(networkSubscriptions, entry)
			return nil
		},
		ListNetworkSubscriptionsFn: func(
			_ context.Context,
			query store.NetworkSubscriptionQuery,
		) ([]store.NetworkSubscriptionEntry, error) {
			networkStateMu.Lock()
			defer networkStateMu.Unlock()
			out := make([]store.NetworkSubscriptionEntry, 0, len(networkSubscriptions))
			for _, entry := range networkSubscriptions {
				if entry.WorkspaceID != query.WorkspaceID ||
					entry.Channel != query.Channel ||
					entry.SessionID != query.SessionID {
					continue
				}
				if strings.TrimSpace(query.ThreadID) != "" && entry.ThreadID != query.ThreadID {
					continue
				}
				out = append(out, entry)
			}
			return out, nil
		},
	}

	t.Run("Should return network status", func(t *testing.T) {
		statusResp := performRequest(t, fixture.Engine, http.MethodGet, "/network/status", nil)
		if statusResp.Code != http.StatusOK {
			t.Fatalf("status code = %d, want %d", statusResp.Code, http.StatusOK)
		}

		var statusPayload contract.NetworkStatusResponse
		testutil.DecodeJSONResponse(t, statusResp, &statusPayload)
		if statusPayload.Network.Channels != 1 || statusPayload.Network.MessagesDelivered != 3 ||
			len(statusPayload.Network.KindMetrics) != 1 {
			t.Fatalf("status payload = %#v", statusPayload.Network)
		}
		if statusPayload.Network.KindMetrics[0].Sent != 4 ||
			statusPayload.Network.KindMetrics[0].Kind != string(network.KindSay) {
			t.Fatalf("kind metrics = %#v", statusPayload.Network.KindMetrics)
		}
	})

	t.Run("Should list network peers", func(t *testing.T) {
		peersResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers?channel=builders",
			nil,
		)
		if peersResp.Code != http.StatusOK {
			t.Fatalf("peers code = %d, want %d", peersResp.Code, http.StatusOK)
		}

		var peersPayload contract.NetworkPeersResponse
		testutil.DecodeJSONResponse(t, peersResp, &peersPayload)
		if len(peersPayload.Peers) != 1 || peersPayload.Peers[0].PeerCard.DisplayName == nil ||
			*peersPayload.Peers[0].PeerCard.DisplayName != "Reviewer" {
			t.Fatalf("peers payload = %#v", peersPayload.Peers)
		}
		if got, want := peersPayload.Peers[0].PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{{
			ID:      "send",
			Summary: "Send channel updates",
		}}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("peer list capability brief = %#v, want %#v", got, want)
		}
		if _, ok := peersPayload.Peers[0].PeerCard.Ext["agh.capabilities_brief"]; ok {
			t.Fatalf(
				"peer list capability brief ext should be stripped: %#v",
				peersPayload.Peers[0].PeerCard.Ext,
			)
		}
	})

	t.Run("Should list network channels", func(t *testing.T) {
		channelsResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if channelsResp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", channelsResp.Code, http.StatusOK)
		}

		var channelsPayload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
		if len(channelsPayload.Channels) != 1 ||
			channelsPayload.Channels[0].Channel != "builders" ||
			channelsPayload.Channels[0].PeerCount != 2 {
			t.Fatalf("channels payload = %#v", channelsPayload.Channels)
		}
	})

	t.Run("Should reject resolving a direct room against the same session", func(t *testing.T) {
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels/builders/directs/resolve",
			[]byte(`{"session_id":"sess-a","peer_id":"reviewer.sess-a"}`),
		)
		if response.Code != http.StatusBadRequest {
			t.Fatalf(
				"same-session direct resolve code = %d, want %d; body=%s",
				response.Code,
				http.StatusBadRequest,
				response.Body.String(),
			)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, response, &payload)
		if !strings.Contains(payload.Error, "direct room sessions must differ") {
			t.Fatalf("same-session direct resolve error = %q, want stable validation detail", payload.Error)
		}
	})

	t.Run("Should update network channel policy", func(t *testing.T) {
		t.Parallel()

		updateResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/workspaces/ws-workspace/network/channels/builders",
			[]byte(
				`{"purpose":"Pair reviews","fanout_policy":"coordinator","coordinator_peer_id":"reviewer.sess-a"}`,
			),
		)
		if updateResp.Code != http.StatusOK {
			t.Fatalf(
				"update channel code = %d, want %d; body=%s",
				updateResp.Code,
				http.StatusOK,
				updateResp.Body.String(),
			)
		}
		var updatePayload contract.NetworkChannelResponse
		testutil.DecodeJSONResponse(t, updateResp, &updatePayload)
		if updatePayload.Channel.FanoutPolicy != store.NetworkFanoutPolicyCoordinator ||
			updatePayload.Channel.CoordinatorPeerID != "reviewer.sess-a" ||
			updatePayload.Channel.Purpose != "Pair reviews" {
			t.Fatalf("updated channel payload = %#v", updatePayload.Channel)
		}
		networkStateMu.Lock()
		stored := networkChannels[networkChannelKey("ws-workspace", "builders")]
		networkStateMu.Unlock()
		if stored.FanoutPolicy != store.NetworkFanoutPolicyCoordinator ||
			stored.CoordinatorPeerID != "reviewer.sess-a" ||
			stored.Purpose != "Pair reviews" {
			t.Fatalf("stored channel = %#v", stored)
		}
	})

	t.Run("Should upsert and list network subscriptions with default channel metadata", func(t *testing.T) {
		t.Parallel()

		upsertResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/workspaces/ws-workspace/network/channels/quiet/subscriptions",
			[]byte(
				`{"thread_id":"thread_quiet","session_id":"sess-reviewer","mode":"mute"}`,
			),
		)
		if upsertResp.Code != http.StatusOK {
			t.Fatalf(
				"upsert subscription code = %d, want %d; body=%s",
				upsertResp.Code,
				http.StatusOK,
				upsertResp.Body.String(),
			)
		}
		var upsertPayload contract.NetworkSubscriptionResponse
		testutil.DecodeJSONResponse(t, upsertResp, &upsertPayload)
		if upsertPayload.Subscription.Channel != "quiet" ||
			upsertPayload.Subscription.ThreadID != "thread_quiet" ||
			upsertPayload.Subscription.SessionID != "sess-reviewer" ||
			upsertPayload.Subscription.Mode != store.NetworkSubscriptionModeMute {
			t.Fatalf("upsert subscription payload = %#v", upsertPayload.Subscription)
		}
		networkStateMu.Lock()
		_, ok := networkChannels[networkChannelKey("ws-workspace", "quiet")]
		networkStateMu.Unlock()
		if !ok {
			t.Fatal("subscription upsert did not materialize default channel metadata")
		}

		listResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/quiet/subscriptions?session_id=sess-reviewer&thread_id=thread_quiet",
			nil,
		)
		if listResp.Code != http.StatusOK {
			t.Fatalf(
				"list subscription code = %d, want %d; body=%s",
				listResp.Code,
				http.StatusOK,
				listResp.Body.String(),
			)
		}
		var listPayload contract.NetworkSubscriptionsResponse
		testutil.DecodeJSONResponse(t, listResp, &listPayload)
		if len(listPayload.Subscriptions) != 1 ||
			listPayload.Subscriptions[0].SessionID != "sess-reviewer" ||
			listPayload.Subscriptions[0].Mode != store.NetworkSubscriptionModeMute {
			t.Fatalf("list subscriptions payload = %#v", listPayload.Subscriptions)
		}
	})

	t.Run("Should reject unknown network subscription fields", func(t *testing.T) {
		t.Parallel()

		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPut,
			"/workspaces/ws-workspace/network/channels/quiet/subscriptions",
			[]byte(`{"session_id":"sess-reviewer","mode":"mute","legacy":true}`),
		)
		body := response.Body.String()
		if response.Code != http.StatusBadRequest ||
			!strings.Contains(body, "unknown_field") ||
			!strings.Contains(body, "legacy") {
			t.Fatalf(
				"subscription status = %d, want %d unknown_field response naming legacy; body=%s",
				response.Code,
				http.StatusBadRequest,
				body,
			)
		}
	})

	t.Run("Should send network messages", func(t *testing.T) {
		sendResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/send",
			[]byte(
				`{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_launch_db","kind":"say","body":{"text":"hello"},"ext":{"agh.workflow_id":"wf-1","agh.handoff_version":3}}`,
			),
		)
		if sendResp.Code != http.StatusOK {
			t.Fatalf(
				"send code = %d, want %d; body=%s",
				sendResp.Code,
				http.StatusOK,
				sendResp.Body.String(),
			)
		}

		var sendPayload contract.NetworkSendResponse
		testutil.DecodeJSONResponse(t, sendResp, &sendPayload)
		if sendPayload.Message.ID != "msg-1" ||
			string(sendPayload.Message.Ext["agh.workflow_id"]) != `"wf-1"` {
			t.Fatalf("send payload = %#v", sendPayload.Message)
		}
		if got := sendPayload.Message.ExpiresAt; got != nil {
			t.Fatalf("send expires_at = %#v, want nil without flag", got)
		}
	})

	t.Run("Should infer direct target when sending to a direct room", func(t *testing.T) {
		sendResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/send",
			fmt.Appendf(
				nil,
				"{\"session_id\":\"sess-a\",\"channel\":\"builders\",\"surface\":\"direct\",\"direct_id\":%q,\"kind\":\"say\",\"body\":{\"text\":\"hello direct\"}}",
				directID,
			),
		)
		if sendResp.Code != http.StatusOK {
			t.Fatalf(
				"direct send code = %d, want %d; body=%s",
				sendResp.Code,
				http.StatusOK,
				sendResp.Body.String(),
			)
		}

		var sendPayload contract.NetworkSendResponse
		testutil.DecodeJSONResponse(t, sendResp, &sendPayload)
		if sendPayload.Message.ID != "msg-direct" || sendPayload.Message.To != "coder.sess-remote" {
			t.Fatalf("direct send payload = %#v, want inferred target", sendPayload.Message)
		}
	})

	t.Run("Should return network inbox messages", func(t *testing.T) {
		inboxResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/inbox?session_id=sess-a",
			nil,
		)
		if inboxResp.Code != http.StatusOK {
			t.Fatalf("inbox code = %d, want %d", inboxResp.Code, http.StatusOK)
		}

		var inboxPayload contract.NetworkInboxResponse
		testutil.DecodeJSONResponse(t, inboxResp, &inboxPayload)
		if len(inboxPayload.Messages) != 1 {
			t.Fatalf("inbox payload len = %d, want 1", len(inboxPayload.Messages))
		}
		if string(inboxPayload.Messages[0].Proof["sig"]) != `"abc123"` ||
			string(inboxPayload.Messages[0].Ext["agh.handoff_version"]) != `3` {
			t.Fatalf("inbox payload = %#v", inboxPayload.Messages[0])
		}
		if got := inboxPayload.Messages[0].ExpiresAt; got != nil {
			t.Fatalf("ExpiresAt = %#v, want nil for non-expiring inbox message", got)
		}
	})
}

func TestBaseHandlersPromoteNetworkThreadTask(t *testing.T) {
	t.Parallel()

	t.Run("Should promote a thread message into a task", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 1, 17, 0, 0, 0, time.UTC)
		var createdSpec taskpkg.CreateTask
		var writtenOrigin store.NetworkTaskThreadOrigin
		tasks := &testutil.StubTaskManager{
			CreateTaskFn: func(_ context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
				createdSpec = spec
				return &taskpkg.Task{
					ID:          "task-promoted",
					Scope:       spec.Scope,
					WorkspaceID: spec.WorkspaceID,
					Title:       spec.Title,
					Description: spec.Description,
					Priority:    spec.Priority,
					Status:      taskpkg.TaskStatusReady,
					CreatedBy:   actor.Actor,
					Origin:      actor.Origin,
					CreatedAt:   now,
					UpdatedAt:   now,
					Metadata:    spec.Metadata,
				}, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListConversationMessagesFn: func(
				_ context.Context,
				ref store.NetworkConversationRef,
				query store.NetworkConversationMessageQuery,
			) ([]store.NetworkConversationMessage, error) {
				if ref.WorkspaceID != "ws-workspace" || ref.Channel != "builders" ||
					ref.ThreadID != "thread_promote" || query.Limit != 200 {
					t.Fatalf("ListConversationMessages ref=%#v query=%#v", ref, query)
				}
				return []store.NetworkConversationMessage{
					{
						MessageID: "msg-origin",
						PeerFrom:  "coordinator.sess-a",
						Text:      strings.Repeat("Investigate high token usage. ", 220),
					},
					{
						MessageID: "msg-followup",
						PeerFrom:  "reviewer.sess-b",
						Text:      "Check digest routing",
					},
				}, nil
			},
			GetThreadFn: func(
				_ context.Context,
				ref store.NetworkChannelRef,
				threadID string,
			) (store.NetworkThreadSummary, error) {
				if ref.WorkspaceID != "ws-workspace" || ref.Channel != "builders" || threadID != "thread_promote" {
					t.Fatalf("GetThread ref=%#v threadID=%q", ref, threadID)
				}
				return store.NetworkThreadSummary{
					WorkspaceID: "ws-workspace",
					Channel:     "builders",
					ThreadID:    "thread_promote",
					Title:       "Token optimization",
				}, nil
			},
			PutNetworkTaskThreadOriginFn: func(_ context.Context, origin store.NetworkTaskThreadOrigin) error {
				writtenOrigin = origin
				return nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels/builders/threads/thread_promote/promote-task",
			[]byte(`{"origin_message_id":"msg-followup","priority":"high","metadata":{"source":"test"}}`),
		)
		if resp.Code != http.StatusCreated {
			t.Fatalf("promote status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
		}
		var payload contract.PromoteNetworkThreadTaskResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if payload.Task.ID != "task-promoted" ||
			resolvedParticipationChannelID(payload.Task.ResolvedNetworkParticipation) != "" ||
			payload.Origin.OriginMessageID != "msg-followup" ||
			payload.Origin.TaskID != "task-promoted" {
			t.Fatalf("promote payload = %#v", payload)
		}
		if createdSpec.Scope != taskpkg.ScopeWorkspace ||
			createdSpec.WorkspaceID != "ws-workspace" ||
			createdSpec.Title != "Token optimization" ||
			createdSpec.Priority != taskpkg.PriorityHigh {
			t.Fatalf("created promoted task spec = %#v", createdSpec)
		}
		if writtenOrigin.TaskID != "task-promoted" ||
			writtenOrigin.ThreadID != "thread_promote" ||
			writtenOrigin.OriginMessageID != "msg-followup" ||
			!slices.Contains(writtenOrigin.SourceMessageIDs, "msg-followup") {
			t.Fatalf("written origin = %#v", writtenOrigin)
		}
	})
}

func TestBaseHandlersNetworkPeersUseBestEffortSessionEnrichment(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should enrich local peers and fall back to peer-card display names on lookup failures",
		func(t *testing.T) {
			localSessionID := "sess-local"
			brokenSessionID := "sess-broken"
			brokenDisplayName := "Broken peer"

			manager := testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{
						{
							ID:        localSessionID,
							Name:      "Reviewer",
							AgentName: "reviewer",
						},
					}, nil
				},
				StatusFn: func(context.Context, string) (*session.Info, error) {
					t.Fatal("Status() should not be called for peer enrichment")
					return nil, nil
				},
			}

			fixture := newHandlerFixture(
				t,
				manager,
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, "builders"; got != want {
						t.Fatalf("ListPeers() channel = %q, want %q", got, want)
					}
					return []network.PeerInfo{
						{
							SessionID: &localSessionID,
							PeerID:    "reviewer.sess-local",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "reviewer.sess-local"},
						},
						{
							SessionID: &brokenSessionID,
							PeerID:    "broken.sess-broken",
							Channel:   "builders",
							Local:     true,
							PeerCard: network.PeerCard{
								PeerID:      "broken.sess-broken",
								DisplayName: &brokenDisplayName,
							},
						},
					}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers?channel=builders",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("peers code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkPeersResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Peers), 2; got != want {
				t.Fatalf("len(peers) = %d, want %d", got, want)
			}
			displayNames := []string{payload.Peers[0].DisplayName, payload.Peers[1].DisplayName}
			sort.Strings(displayNames)
			if got, want := displayNames, []string{brokenDisplayName, "Reviewer"}; !reflect.DeepEqual(
				got,
				want,
			) {
				t.Fatalf("display names = %#v, want %#v", got, want)
			}
			if payload.Peers[0].PeerCard.ProfilesSupported == nil {
				t.Fatal("peers[0].peer_card.profiles_supported = nil, want empty slice")
			}
			if payload.Peers[0].PeerCard.ArtifactsSupported == nil {
				t.Fatal("peers[0].peer_card.artifacts_supported = nil, want empty slice")
			}
			if payload.Peers[0].PeerCard.TrustModesSupported == nil {
				t.Fatal("peers[0].peer_card.trust_modes_supported = nil, want empty slice")
			}
		},
	)
}

func TestBaseHandlersNetworkPeerOrderingUsesEffectiveRecency(t *testing.T) {
	t.Parallel()

	backendSessionID := "sess-backend"
	coderSessionID := "sess-coder"
	backendJoinedAt := time.Date(2026, 4, 28, 7, 17, 11, 0, time.UTC)
	coderJoinedAt := backendJoinedAt.Add(3 * time.Second)

	newFixture := func(t *testing.T) handlerFixture {
		t.Helper()

		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{
					{
						ID:                   backendSessionID,
						Name:                 "Backend",
						AgentName:            "backend",
						NetworkParticipation: testLiveParticipation("", "builders"),
						State:                session.StateActive,
					},
					{
						ID:                   coderSessionID,
						Name:                 "Coder",
						AgentName:            "coder",
						NetworkParticipation: testLiveParticipation("", "builders"),
						State:                session.StateActive,
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				switch channel {
				case "", "builders":
				default:
					t.Fatalf("ListPeers() channel = %q, want empty or builders", channel)
				}
				return []network.PeerInfo{
					{
						SessionID: &backendSessionID,
						PeerID:    "backend.sess-older",
						Channel:   "builders",
						Local:     true,
						PeerCard:  network.PeerCard{PeerID: "backend.sess-older"},
						JoinedAt:  &backendJoinedAt,
					},
					{
						SessionID: &coderSessionID,
						PeerID:    "coder.sess-newer",
						Channel:   "builders",
						Local:     true,
						PeerCard:  network.PeerCard{PeerID: "coder.sess-newer"},
						JoinedAt:  &coderJoinedAt,
					},
				}, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.Channel, "builders"; got != want {
					t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
				}
				return nil, nil
			},
		}
		return fixture
	}

	t.Run(
		"Should sort peer list by joined-at recency when last-seen is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t)

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers?channel=builders",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("peers code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkPeersResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Peers), 2; got != want {
				t.Fatalf("len(peers) = %d, want %d", got, want)
			}
			if got, want := payload.Peers[0].PeerID, "coder.sess-newer"; got != want {
				t.Fatalf("peers[0].peer_id = %q, want %q", got, want)
			}
			if got, want := payload.Peers[1].PeerID, "backend.sess-older"; got != want {
				t.Fatalf("peers[1].peer_id = %q, want %q", got, want)
			}
		},
	)

	t.Run(
		"Should sort channel detail peers by joined-at recency when last-seen is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newFixture(t)

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("channel detail code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkChannelResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Channel.Peers), 2; got != want {
				t.Fatalf("len(channel peers) = %d, want %d", got, want)
			}
			if got, want := payload.Channel.Peers[0].PeerID, "coder.sess-newer"; got != want {
				t.Fatalf("channel peers[0].peer_id = %q, want %q", got, want)
			}
			if got, want := payload.Channel.Peers[1].PeerID, "backend.sess-older"; got != want {
				t.Fatalf("channel peers[1].peer_id = %q, want %q", got, want)
			}
		},
	)
}

func TestBaseHandlersNetworkPeerMessages(t *testing.T) {
	t.Parallel()

	t.Run("Should return visible peer messages", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 11, 20, 0, 0, 0, time.UTC)
		localSessionID := "sess-coder"
		remoteSessionID := "sess-reviewer"
		fixture := newHandlerFixture(t, testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{
					{
						ID:        localSessionID,
						Name:      "Coder",
						AgentName: "coder",
						State:     session.StateActive,
					},
					{
						ID:        remoteSessionID,
						Name:      "Reviewer",
						AgentName: "reviewer",
						State:     session.StateActive,
					},
				}, nil
			},
		}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, ""; got != want {
					t.Fatalf("ListPeers() channel = %q, want empty peer detail lookup", got)
				}
				return []network.PeerInfo{
					{
						SessionID: &localSessionID,
						PeerID:    "coder.sess-coder",
						Channel:   "builders",
						Local:     true,
						PeerCard:  network.PeerCard{PeerID: "coder.sess-coder"},
					},
					{
						SessionID: &remoteSessionID,
						PeerID:    "reviewer.sess-reviewer",
						Channel:   "builders",
						Local:     false,
						PeerCard:  network.PeerCard{PeerID: "reviewer.sess-reviewer"},
					},
				}, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.PeerID, "reviewer.sess-reviewer"; got != want {
					t.Fatalf("ListNetworkMessages() PeerID = %q, want %q", got, want)
				}
				if !query.DirectedOnly {
					t.Fatal("ListNetworkMessages() DirectedOnly = false, want true")
				}
				return []store.NetworkMessageEntry{{
					MessageID:   "msg-direct-01",
					SessionID:   localSessionID,
					Channel:     "builders",
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-coder",
					PeerTo:      "reviewer.sess-reviewer",
					Kind:        "direct",
					Text:        "can you review this?",
					PreviewText: "can you review this?",
					Body:        json.RawMessage(`{"text":"can you review this?"}`),
					Timestamp:   recordedAt,
				}}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers/reviewer.sess-reviewer/messages?limit=25",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"peer messages code = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}

		var payload contract.NetworkPeerMessagesResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got, want := len(payload.Messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := payload.Messages[0].DisplayName, "Coder"; got != want {
			t.Fatalf("message display_name = %q, want %q", got, want)
		}
		if got, want := payload.Messages[0].PeerTo, "reviewer.sess-reviewer"; got != want {
			t.Fatalf("message peer_to = %q, want %q", got, want)
		}
	})
}

func TestBaseHandlersCreateNetworkChannelRollsBackPartialProvisioning(t *testing.T) {
	t.Parallel()

	t.Run("Should roll back created sessions when channel readback fails", func(t *testing.T) {
		t.Parallel()

		createdAt := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC)
		var operations []string
		channelPersisted := false
		manager := testutil.StubSessionManager{
			CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
				if !channelPersisted {
					t.Fatal("Create() called before durable channel authority was persisted")
				}
				channelID := ""
				if opts.NetworkParticipation != nil && opts.NetworkParticipation.ChannelID != nil {
					channelID = *opts.NetworkParticipation.ChannelID
				}
				return &session.Session{
					ID:                   "sess-" + opts.AgentName,
					Name:                 strings.ToUpper(opts.AgentName),
					AgentName:            opts.AgentName,
					WorkspaceID:          opts.Workspace,
					NetworkParticipation: testLiveParticipation(opts.Workspace, channelID),
					Type:                 session.SessionTypeUser,
					State:                session.StateActive,
					CreatedAt:            createdAt,
					UpdatedAt:            createdAt,
				}, nil
			},
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, errors.New("readback failed")
			},
			StopWithCauseFn: func(_ context.Context, id string, cause session.StopCause, detail string) error {
				if got, want := cause, session.CauseFailed; got != want {
					t.Fatalf("StopWithCause() cause = %v, want %v", got, want)
				}
				if got, want := detail, "rollback network channel creation"; got != want {
					t.Fatalf("StopWithCause() detail = %q, want %q", got, want)
				}
				operations = append(operations, "stop:"+id)
				return nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: ref, Name: "Workspace"},
					Agents: []aghconfig.AgentDef{
						{Name: "coder"},
						{Name: "reviewer"},
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(context.Context, string, string) ([]network.PeerInfo, error) {
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			CreateNetworkChannelFn: func(_ context.Context, _ store.NetworkChannelEntry) error {
				channelPersisted = true
				return nil
			},
			DeleteNetworkChannelFn: func(ctx context.Context, ref store.NetworkChannelRef) error {
				if err := ctx.Err(); err != nil {
					t.Fatalf("DeleteNetworkChannel() context error = %v", err)
				}
				if got, want := ref, (store.NetworkChannelRef{
					WorkspaceID: "ws-workspace",
					Channel:     "builders",
				}); got != want {
					t.Fatalf("DeleteNetworkChannel() ref = %#v, want %#v", got, want)
				}
				operations = append(operations, "delete:"+ref.WorkspaceID+"/"+ref.Channel)
				channelPersisted = false
				return nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels",
			[]byte(
				`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder","reviewer"]}`,
			),
		)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("create channel code = %d, want %d", resp.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(resp.Body.String(), "readback failed") {
			t.Fatalf("create channel body = %q, want readback failure", resp.Body.String())
		}

		if got, want := strings.Join(operations, ","),
			"stop:sess-coder,stop:sess-reviewer,delete:ws-workspace/builders"; got != want {
			t.Fatalf("rollback operations = %q, want %q", got, want)
		}
		if channelPersisted {
			t.Fatal("network channel still persisted after readback rollback")
		}
	})

	t.Run("Should remove channel authority when a member session fails", func(t *testing.T) {
		t.Parallel()

		channelPersisted := false
		createCalls := 0
		var operations []string
		manager := testutil.StubSessionManager{
			CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
				if !channelPersisted {
					t.Fatal("Create() called before durable channel authority was persisted")
				}
				createCalls++
				if createCalls == 2 {
					return nil, errors.New("member create failed")
				}
				return &session.Session{
					ID:                   "sess-" + opts.AgentName,
					AgentName:            opts.AgentName,
					WorkspaceID:          opts.Workspace,
					NetworkParticipation: testLiveParticipation(opts.Workspace, "builders"),
					Type:                 session.SessionTypeUser,
					State:                session.StateActive,
				}, nil
			},
			StopWithCauseFn: func(_ context.Context, id string, _ session.StopCause, _ string) error {
				operations = append(operations, "stop:"+id)
				return nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: ref, Name: "Workspace"},
					Agents: []aghconfig.AgentDef{
						{Name: "coder"},
						{Name: "reviewer"},
					},
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			CreateNetworkChannelFn: func(_ context.Context, _ store.NetworkChannelEntry) error {
				channelPersisted = true
				return nil
			},
			DeleteNetworkChannelFn: func(_ context.Context, ref store.NetworkChannelRef) error {
				operations = append(operations, "delete:"+ref.WorkspaceID+"/"+ref.Channel)
				channelPersisted = false
				return nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels",
			[]byte(
				`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder","reviewer"]}`,
			),
		)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("create channel code = %d, want %d", resp.Code, http.StatusInternalServerError)
		}
		if !strings.Contains(resp.Body.String(), "member create failed") {
			t.Fatalf("create channel body = %q, want member failure", resp.Body.String())
		}
		if channelPersisted {
			t.Fatal("network channel still persisted after member creation failure")
		}
		if got, want := strings.Join(operations, ","),
			"stop:sess-coder,delete:ws-workspace/builders"; got != want {
			t.Fatalf("rollback operations = %q, want %q", got, want)
		}
	})

	t.Run("Should complete compensation after the request is canceled", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		channelPersisted := false
		var operations []string
		entry := store.NetworkChannelEntry{
			WorkspaceID: "ws-workspace",
			Channel:     "builders",
			Purpose:     "Cross-agent coordination",
		}
		networkStore := testutil.StubNetworkStore{
			CreateNetworkChannelFn: func(_ context.Context, got store.NetworkChannelEntry) error {
				if channelPersisted {
					return store.ErrNetworkChannelExists
				}
				if got.WorkspaceID != entry.WorkspaceID || got.Channel != entry.Channel {
					t.Fatalf("CreateNetworkChannel() entry = %#v, want %#v", got, entry)
				}
				operations = append(operations, "create:"+got.WorkspaceID+"/"+got.Channel)
				channelPersisted = true
				return nil
			},
			DeleteNetworkChannelFn: func(cleanupCtx context.Context, ref store.NetworkChannelRef) error {
				if err := cleanupCtx.Err(); err != nil {
					t.Fatalf("DeleteNetworkChannel() inherited canceled context: %v", err)
				}
				if got, want := ref, (store.NetworkChannelRef{
					WorkspaceID: entry.WorkspaceID,
					Channel:     entry.Channel,
				}); got != want {
					t.Fatalf("DeleteNetworkChannel() ref = %#v, want %#v", got, want)
				}
				operations = append(operations, "delete:"+ref.WorkspaceID+"/"+ref.Channel)
				channelPersisted = false
				return nil
			},
		}
		manager := testutil.StubSessionManager{
			CreateFn: func(createCtx context.Context, _ session.CreateOpts) (*session.Session, error) {
				cancel()
				return nil, createCtx.Err()
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: ref, Name: "Workspace"},
					Agents:    []aghconfig.AgentDef{{Name: "coder"}},
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = networkStore
		request := httptest.NewRequestWithContext(
			ctx,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels",
			strings.NewReader(
				`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder"]}`,
			),
		)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		fixture.Engine.ServeHTTP(response, request)
		const statusClientClosedRequest = 499
		if response.Code != statusClientClosedRequest {
			t.Fatalf("create channel code = %d, want %d", response.Code, statusClientClosedRequest)
		}
		if !strings.Contains(response.Body.String(), context.Canceled.Error()) {
			t.Fatalf("create channel body = %q, want cancellation", response.Body.String())
		}
		if channelPersisted {
			t.Fatal("network channel still persisted after canceled provisioning")
		}
		if retryErr := networkStore.CreateNetworkChannel(t.Context(), entry); retryErr != nil {
			t.Fatalf("CreateNetworkChannel() retry error = %v", retryErr)
		}
		if got, want := strings.Join(operations, ","),
			"create:ws-workspace/builders,delete:ws-workspace/builders,create:ws-workspace/builders"; got != want {
			t.Fatalf("operations = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve primary stop and channel cleanup errors", func(t *testing.T) {
		t.Parallel()

		memberErr := errors.New("member create failed")
		stopErr := errors.New("stop failed")
		deleteErr := errors.New("channel delete failed")
		channelPersisted := false
		createCalls := 0
		var operations []string
		entry := store.NetworkChannelEntry{
			WorkspaceID: "ws-workspace",
			Channel:     "builders",
			Purpose:     "Cross-agent coordination",
		}
		networkStore := testutil.StubNetworkStore{
			CreateNetworkChannelFn: func(_ context.Context, got store.NetworkChannelEntry) error {
				if channelPersisted {
					return store.ErrNetworkChannelExists
				}
				operations = append(operations, "create:"+got.WorkspaceID+"/"+got.Channel)
				channelPersisted = true
				return nil
			},
			DeleteNetworkChannelFn: func(_ context.Context, ref store.NetworkChannelRef) error {
				operations = append(operations, "delete:"+ref.WorkspaceID+"/"+ref.Channel)
				channelPersisted = false
				return deleteErr
			},
		}
		manager := testutil.StubSessionManager{
			CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
				createCalls++
				if createCalls == 2 {
					return nil, memberErr
				}
				return &session.Session{ID: "sess-" + opts.AgentName}, nil
			},
			StopWithCauseFn: func(_ context.Context, id string, cause session.StopCause, detail string) error {
				if cause != session.CauseFailed || detail != "rollback network channel creation" {
					t.Fatalf("StopWithCause() cause/detail = %v/%q", cause, detail)
				}
				operations = append(operations, "stop:"+id)
				return stopErr
			},
		}
		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				return workspacepkg.ResolvedWorkspace{
					Workspace: workspacepkg.Workspace{ID: ref, Name: "Workspace"},
					Agents: []aghconfig.AgentDef{
						{Name: "coder"},
						{Name: "reviewer"},
					},
				}, nil
			},
		}
		fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = networkStore
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/channels",
			[]byte(
				`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder","reviewer"]}`,
			),
		)
		if response.Code != http.StatusInternalServerError {
			t.Fatalf("create channel code = %d, want %d", response.Code, http.StatusInternalServerError)
		}
		for name, wantErr := range map[string]error{
			"member": memberErr,
			"stop":   stopErr,
			"delete": deleteErr,
		} {
			if !strings.Contains(response.Body.String(), wantErr.Error()) {
				t.Fatalf("create channel body = %q, want %s error %v", response.Body.String(), name, wantErr)
			}
		}
		if channelPersisted {
			t.Fatal("network channel still persisted after cleanup returned an error")
		}
		if retryErr := networkStore.CreateNetworkChannel(t.Context(), entry); retryErr != nil {
			t.Fatalf("CreateNetworkChannel() retry error = %v", retryErr)
		}
		if got, want := strings.Join(operations, ","),
			"create:ws-workspace/builders,stop:sess-coder,delete:ws-workspace/builders,create:ws-workspace/builders"; got != want {
			t.Fatalf("operations = %q, want %q", got, want)
		}
	})
}

func TestBaseHandlersNetworkChannelsIncludeHistoryOnlyChannels(t *testing.T) {
	t.Parallel()

	t.Run("Should include history-only channels from persisted message logs", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 11, 18, 30, 0, 0, time.UTC)
		fixture := newHandlerFixture(t, testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, nil
			},
		}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(context.Context, string, string) ([]network.PeerInfo, error) {
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got := query.Channel; got != "" {
					t.Fatalf("ListNetworkMessages() channel = %q, want empty list query", got)
				}
				return []store.NetworkMessageEntry{{
					MessageID:   "msg-history-only",
					Channel:     "builders",
					Direction:   network.AuditDirectionReceived,
					PeerFrom:    "reviewer.sess-remote",
					Kind:        "say",
					Text:        "History survives runtime disconnects.",
					PreviewText: "History survives runtime disconnects.",
					Body: json.RawMessage(
						`{"text":"History survives runtime disconnects."}`,
					),
					Timestamp: recordedAt,
				}}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got, want := len(payload.Channels), 1; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := payload.Channels[0].Channel, "builders"; got != want {
			t.Fatalf("channel = %q, want %q", got, want)
		}
		if got, want := payload.Channels[0].MessageCount, 1; got != want {
			t.Fatalf("message_count = %d, want %d", got, want)
		}
	})

	t.Run("Should order equal-timestamp persisted channels by causal sequence", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 11, 18, 30, 0, 0, time.UTC)
		fixture := newHandlerFixture(t, testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, nil
			},
		}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(context.Context, string, string) ([]network.PeerInfo, error) {
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkChannelProjectionsFn: func(
				_ context.Context,
				query store.NetworkChannelProjectionQuery,
			) ([]store.NetworkChannelProjection, error) {
				if got, want := query.WorkspaceID, "ws-workspace"; got != want {
					t.Fatalf("workspace_id = %q, want %q", got, want)
				}
				return []store.NetworkChannelProjection{
					{
						WorkspaceID:          query.WorkspaceID,
						Channel:              "earlier-sequence",
						MessageCount:         1,
						LastActivityAt:       &recordedAt,
						LastActivitySequence: 10,
					},
					{
						WorkspaceID:          query.WorkspaceID,
						Channel:              "later-sequence",
						MessageCount:         1,
						LastActivityAt:       &recordedAt,
						LastActivitySequence: 20,
					},
				}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got, want := len(payload.Channels), 2; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := payload.Channels[0].Channel, "later-sequence"; got != want {
			t.Fatalf("channels[0].channel = %q, want %q", got, want)
		}
		if got, want := payload.Channels[1].Channel, "earlier-sequence"; got != want {
			t.Fatalf("channels[1].channel = %q, want %q", got, want)
		}
	})
}

func TestBaseHandlersNetworkChannelReturnsHistoryOnlyDetails(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should return history-only channel details from persisted message logs",
		func(t *testing.T) {
			recordedAt := time.Date(2026, 4, 11, 19, 0, 0, 0, time.UTC)
			fixture := newHandlerFixture(t, testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, "builders"; got != want {
						t.Fatalf("ListPeers() channel = %q, want %q", got, want)
					}
					return nil, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					if got, want := query.Channel, "builders"; got != want {
						t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
					}
					return []store.NetworkMessageEntry{{
						MessageID:   "msg-history-detail",
						Channel:     "builders",
						Direction:   network.AuditDirectionReceived,
						PeerFrom:    "reviewer.sess-remote",
						Kind:        "say",
						Text:        "Still visible from persisted history.",
						PreviewText: "Still visible from persisted history.",
						Body: json.RawMessage(
							`{"text":"Still visible from persisted history."}`,
						),
						Timestamp: recordedAt,
					}}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf(
					"channel detail code = %d, want %d body=%s",
					resp.Code,
					http.StatusOK,
					resp.Body.String(),
				)
			}

			var payload contract.NetworkChannelResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := payload.Channel.Channel, "builders"; got != want {
				t.Fatalf("channel = %q, want %q", got, want)
			}
			if got, want := payload.Channel.MessageCount, 1; got != want {
				t.Fatalf("message_count = %d, want %d", got, want)
			}
			if got, want := payload.Channel.SessionCount, 0; got != want {
				t.Fatalf("session_count = %d, want %d", got, want)
			}
			if got, want := payload.Channel.PeerCount, 0; got != want {
				t.Fatalf("peer_count = %d, want %d", got, want)
			}
		},
	)
}

func TestBaseHandlersNetworkChannelsSeparatePresenceFromConversation(t *testing.T) {
	t.Run("Should keep presence counts separate from conversation totals", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 11, 19, 0, 0, 0, time.UTC)
		fixture := newHandlerFixture(t, testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, nil
			},
		}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				switch channel {
				case "":
					return nil, nil
				case "builders":
					return nil, nil
				default:
					t.Fatalf("ListPeers() channel = %q, want builders or empty", channel)
					return nil, nil
				}
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(
				_ context.Context,
				query store.NetworkMessageQuery,
			) ([]store.NetworkMessageEntry, error) {
				messages := []store.NetworkMessageEntry{
					{
						MessageID: "msg-greet-01",
						Channel:   "builders",
						Direction: network.AuditDirectionReceived,
						PeerFrom:  "reviewer.sess-remote",
						Kind:      "greet",
						Body: greetBodyJSON(
							"reviewer.sess-remote",
							"Reviewer",
							"Review pull requests",
							"",
						),
						Timestamp: recordedAt,
					},
					{
						MessageID: "msg-greet-02",
						Channel:   "builders",
						Direction: network.AuditDirectionReceived,
						PeerFrom:  "reviewer.sess-remote",
						Kind:      "greet",
						Body: greetBodyJSON(
							"reviewer.sess-remote",
							"Reviewer",
							"Review pull requests",
							"",
						),
						Timestamp: recordedAt.Add(20 * time.Second),
					},
					{
						MessageID:   "msg-say-01",
						Channel:     "builders",
						Direction:   network.AuditDirectionReceived,
						PeerFrom:    "reviewer.sess-remote",
						Kind:        "say",
						Text:        "Rollout plan is locked.",
						PreviewText: "Rollout plan is locked.",
						Body:        json.RawMessage(`{"text":"Rollout plan is locked."}`),
						Timestamp:   recordedAt.Add(2 * time.Minute),
					},
				}
				if query.Channel == "" || query.Channel == "builders" {
					return messages, nil
				}
				t.Fatalf(
					"ListNetworkMessages() channel = %q, want builders or empty",
					query.Channel,
				)
				return nil, nil
			},
		}

		channelsResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if channelsResp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", channelsResp.Code, http.StatusOK)
		}

		var channelsPayload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
		if got, want := len(channelsPayload.Channels), 1; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		channel := channelsPayload.Channels[0]
		if got, want := channel.MessageCount, 1; got != want {
			t.Fatalf("message_count = %d, want %d", got, want)
		}
		if got, want := channel.PresenceCount, 2; got != want {
			t.Fatalf("presence_count = %d, want %d", got, want)
		}
		if got, want := channel.HistoricalParticipantCount, 1; got != want {
			t.Fatalf("historical_participant_count = %d, want %d", got, want)
		}
		if got, want := channel.LastMessagePreview, "Rollout plan is locked."; got != want {
			t.Fatalf("last_message_preview = %q, want %q", got, want)
		}
		if channel.LastActivityAt == nil ||
			!channel.LastActivityAt.Equal(recordedAt.Add(2*time.Minute)) {
			t.Fatalf(
				"last_activity_at = %#v, want %s",
				channel.LastActivityAt,
				recordedAt.Add(2*time.Minute),
			)
		}
		if channel.LastPresenceAt == nil ||
			!channel.LastPresenceAt.Equal(recordedAt.Add(20*time.Second)) {
			t.Fatalf(
				"last_presence_at = %#v, want %s",
				channel.LastPresenceAt,
				recordedAt.Add(20*time.Second),
			)
		}

		detailResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders",
			nil,
		)
		if detailResp.Code != http.StatusOK {
			t.Fatalf("detail code = %d, want %d", detailResp.Code, http.StatusOK)
		}

		var detailPayload contract.NetworkChannelResponse
		testutil.DecodeJSONResponse(t, detailResp, &detailPayload)
		if got, want := len(detailPayload.Channel.KindCounts), 1; got != want {
			t.Fatalf("len(kind_counts) = %d, want %d", got, want)
		}
		if got, want := detailPayload.Channel.KindCounts[0].Kind, "say"; got != want {
			t.Fatalf("kind_counts[0].Kind = %q, want %q", got, want)
		}
	})
}

func TestBaseHandlersNetworkChannelsTrackDistinctHistoricalPeerIdentities(
	t *testing.T,
) {
	t.Parallel()

	t.Run("Should track distinct historical peer identities", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 28, 6, 22, 0, 0, time.UTC)
		fixture := newHandlerFixture(t, testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return nil, nil
			},
		}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				switch channel {
				case "":
					return nil, nil
				case "founders":
					return nil, nil
				default:
					t.Fatalf("ListPeers() channel = %q, want founders or empty", channel)
					return nil, nil
				}
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(
				_ context.Context,
				query store.NetworkMessageQuery,
			) ([]store.NetworkMessageEntry, error) {
				messages := []store.NetworkMessageEntry{
					{
						MessageID: "msg-greet-founder-01",
						Channel:   "founders",
						Direction: network.AuditDirectionReceived,
						PeerFrom:  "founder.sess-old",
						Kind:      "greet",
						Body:      greetBodyJSON("founder.sess-old", "founder", "Founder", ""),
						Timestamp: recordedAt,
					},
					{
						MessageID: "msg-greet-founder-02",
						Channel:   "founders",
						Direction: network.AuditDirectionReceived,
						PeerFrom:  "founder.sess-new",
						Kind:      "greet",
						Body:      greetBodyJSON("founder.sess-new", "founder", "Founder", ""),
						Timestamp: recordedAt.Add(time.Minute),
					},
				}
				if query.Channel == "" || query.Channel == "founders" {
					return messages, nil
				}
				t.Fatalf(
					"ListNetworkMessages() channel = %q, want founders or empty",
					query.Channel,
				)
				return nil, nil
			},
		}

		channelsResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if channelsResp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", channelsResp.Code, http.StatusOK)
		}

		var channelsPayload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
		if got, want := len(channelsPayload.Channels), 1; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := channelsPayload.Channels[0].PresenceCount, 2; got != want {
			t.Fatalf("channels[0].PresenceCount = %d, want %d", got, want)
		}
		if got, want := channelsPayload.Channels[0].HistoricalParticipantCount, 2; got != want {
			t.Fatalf("channels[0].HistoricalParticipantCount = %d, want %d", got, want)
		}

		detailResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/founders",
			nil,
		)
		if detailResp.Code != http.StatusOK {
			t.Fatalf("detail code = %d, want %d", detailResp.Code, http.StatusOK)
		}

		var detailPayload contract.NetworkChannelResponse
		testutil.DecodeJSONResponse(t, detailResp, &detailPayload)
		if got, want := detailPayload.Channel.PresenceCount, 2; got != want {
			t.Fatalf("detail presence_count = %d, want %d", got, want)
		}
		if got, want := detailPayload.Channel.HistoricalParticipantCount, 2; got != want {
			t.Fatalf("detail historical_participant_count = %d, want %d", got, want)
		}
	})
}

func TestBaseHandlersNetworkChannelMessagesTogglePresenceEpisodes(t *testing.T) {
	t.Run(
		"Should coalesce presence-only channel episodes behind the presence toggle",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 11, 19, 0, 0, 0, time.UTC)
			fixture := newHandlerFixture(t, testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, "presence-only"; got != want {
						t.Fatalf("ListPeers() channel = %q, want %q", got, want)
					}
					return nil, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(
					_ context.Context,
					query store.NetworkMessageQuery,
				) ([]store.NetworkMessageEntry, error) {
					if got, want := query.Channel, "presence-only"; got != want {
						t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
					}
					return []store.NetworkMessageEntry{
						{
							MessageID: "msg-greet-01",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "reviewer.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"reviewer.sess-remote",
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt,
						},
						{
							MessageID: "msg-greet-02",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "planner.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"planner.sess-remote",
								"Planner",
								"Track launch milestones",
								"",
							),
							Timestamp: recordedAt.Add(10 * time.Second),
						},
						{
							MessageID: "msg-greet-03",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "reviewer.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"reviewer.sess-remote",
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt.Add(20 * time.Second),
						},
						{
							MessageID: "msg-greet-04",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "planner.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"planner.sess-remote",
								"Planner",
								"Track launch milestones",
								"",
							),
							Timestamp: recordedAt.Add(30 * time.Second),
						},
					}, nil
				},
			}

			defaultResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/presence-only/messages",
				nil,
			)
			if defaultResp.Code != http.StatusOK {
				t.Fatalf("default messages code = %d, want %d", defaultResp.Code, http.StatusOK)
			}
			var defaultPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, defaultResp, &defaultPayload)
			if got := len(defaultPayload.Messages); got != 0 {
				t.Fatalf("len(default messages) = %d, want 0", got)
			}

			presenceResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/presence-only/messages?include_presence=true",
				nil,
			)
			if presenceResp.Code != http.StatusOK {
				t.Fatalf("presence messages code = %d, want %d", presenceResp.Code, http.StatusOK)
			}
			var presencePayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, presenceResp, &presencePayload)
			if got, want := len(presencePayload.Messages), 4; got != want {
				t.Fatalf("len(presence messages) = %d, want %d", got, want)
			}
			for index, messageID := range []string{"msg-greet-01", "msg-greet-02", "msg-greet-03", "msg-greet-04"} {
				message := presencePayload.Messages[index]
				if message.MessageID != messageID {
					t.Fatalf("presence messages[%d] = %#v, want %s", index, message, messageID)
				}
			}
		},
	)

	t.Run(
		"Should paginate discrete presence messages",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 11, 20, 0, 0, 0, time.UTC)
			fixture := newHandlerFixture(t, testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, "presence-only"; got != want {
						t.Fatalf("ListPeers() channel = %q, want %q", got, want)
					}
					return nil, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(
					_ context.Context,
					query store.NetworkMessageQuery,
				) ([]store.NetworkMessageEntry, error) {
					if got, want := query.Channel, "presence-only"; got != want {
						t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
					}
					if query.Limit != 0 {
						t.Fatalf(
							"ListNetworkMessages() limit = %d, want 0 for raw fetch",
							query.Limit,
						)
					}
					if query.BeforeMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() before = %q, want empty raw cursor",
							query.BeforeMessageID,
						)
					}
					if query.AfterMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() after = %q, want empty raw cursor",
							query.AfterMessageID,
						)
					}
					return []store.NetworkMessageEntry{
						{
							MessageID: "msg-greet-01",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "reviewer.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"reviewer.sess-remote",
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt,
						},
						{
							MessageID: "msg-greet-02",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "planner.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"planner.sess-remote",
								"Planner",
								"Track launch milestones",
								"",
							),
							Timestamp: recordedAt.Add(10 * time.Second),
						},
						{
							MessageID: "msg-greet-03",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "reviewer.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"reviewer.sess-remote",
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt.Add(20 * time.Second),
						},
						{
							MessageID: "msg-greet-04",
							Channel:   "presence-only",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  "planner.sess-remote",
							Kind:      "greet",
							Body: greetBodyJSON(
								"planner.sess-remote",
								"Planner",
								"Track launch milestones",
								"",
							),
							Timestamp: recordedAt.Add(30 * time.Second),
						},
					}, nil
				},
			}

			limitResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/presence-only/messages?include_presence=true&limit=1",
				nil,
			)
			if limitResp.Code != http.StatusOK {
				t.Fatalf(
					"limit presence messages code = %d, want %d",
					limitResp.Code,
					http.StatusOK,
				)
			}
			var limitPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, limitResp, &limitPayload)
			if got, want := len(limitPayload.Messages), 1; got != want {
				t.Fatalf("len(limit presence messages) = %d, want %d", got, want)
			}
			if got, want := limitPayload.Messages[0].MessageID, "msg-greet-04"; got != want {
				t.Fatalf("limit presence messages[0].message_id = %q, want %q", got, want)
			}

			afterResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/presence-only/messages?include_presence=true&after=msg-greet-03&limit=1",
				nil,
			)
			if afterResp.Code != http.StatusOK {
				t.Fatalf(
					"after presence messages code = %d, want %d",
					afterResp.Code,
					http.StatusOK,
				)
			}
			var afterPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, afterResp, &afterPayload)
			if got, want := len(afterPayload.Messages), 1; got != want {
				t.Fatalf("len(after presence messages) = %d, want %d", got, want)
			}
			if got, want := afterPayload.Messages[0].MessageID, "msg-greet-04"; got != want {
				t.Fatalf("after presence messages[0].message_id = %q, want %q", got, want)
			}

			hiddenCursorResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/presence-only/messages?include_presence=true&after=msg-greet-01&limit=1",
				nil,
			)
			if hiddenCursorResp.Code != http.StatusOK {
				t.Fatalf(
					"discrete cursor presence messages code = %d, want %d; body=%s",
					hiddenCursorResp.Code,
					http.StatusOK,
					hiddenCursorResp.Body.String(),
				)
			}
			var hiddenCursorPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, hiddenCursorResp, &hiddenCursorPayload)
			if len(hiddenCursorPayload.Messages) != 1 || hiddenCursorPayload.Messages[0].MessageID != "msg-greet-02" {
				t.Fatalf("discrete cursor payload = %#v, want msg-greet-02", hiddenCursorPayload)
			}
		},
	)
}

func TestBaseHandlersNetworkChannelsHideDirectedTrafficFromPublicTimeline(t *testing.T) {
	t.Run(
		"Should keep directed traffic in peer rooms while hiding it from channel timelines and summaries",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
			channel := "builders"
			peerID := "reviewer.sess-remote"

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{
					ListAllFn: func(context.Context) ([]*session.Info, error) {
						return nil, nil
					},
				},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, requestedChannel string) ([]network.PeerInfo, error) {
					displayName := "Reviewer"
					peers := []network.PeerInfo{{
						PeerID:  peerID,
						Channel: channel,
						Local:   false,
						PeerCard: network.PeerCard{
							PeerID:      peerID,
							DisplayName: &displayName,
						},
					}}
					switch requestedChannel {
					case "", channel:
						return peers, nil
					default:
						return nil, nil
					}
				},
			}

			messages := []store.NetworkMessageEntry{
				{
					MessageID: "msg-greet-01",
					Channel:   channel,
					Direction: network.AuditDirectionReceived,
					PeerFrom:  peerID,
					Kind:      "greet",
					Body:      greetBodyJSON(peerID, "Reviewer", "Review pull requests", ""),
					Timestamp: recordedAt,
				},
				{
					MessageID:   "msg-say-01",
					Channel:     channel,
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-local",
					Kind:        "say",
					Text:        "Public rollout update.",
					PreviewText: "Public rollout update.",
					Body:        json.RawMessage(`{"text":"Public rollout update."}`),
					Timestamp:   recordedAt.Add(time.Minute),
				},
				{
					MessageID:   "msg-direct-01",
					Channel:     channel,
					Direction:   network.AuditDirectionReceived,
					PeerFrom:    peerID,
					PeerTo:      "coder.sess-local",
					Kind:        "direct",
					Text:        "Private review note.",
					PreviewText: "Private review note.",
					Body:        json.RawMessage(`{"text":"Private review note."}`),
					Timestamp:   recordedAt.Add(2 * time.Minute),
				},
				{
					MessageID:   "msg-receipt-01",
					Channel:     channel,
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-local",
					PeerTo:      peerID,
					Kind:        "receipt",
					Text:        "Received the private review note.",
					PreviewText: "Received the private review note.",
					Body: json.RawMessage(
						`{"status":"received","text":"Received the private review note."}`,
					),
					Timestamp: recordedAt.Add(3 * time.Minute),
				},
				{
					MessageID:   "msg-trace-01",
					Channel:     channel,
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-local",
					PeerTo:      peerID,
					Kind:        "trace",
					Text:        "Working on the requested review.",
					PreviewText: "Working on the requested review.",
					Body: json.RawMessage(
						`{"status":"working","text":"Working on the requested review."}`,
					),
					Timestamp: recordedAt.Add(4 * time.Minute),
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkChannelsFn: func(context.Context, store.NetworkChannelQuery) ([]store.NetworkChannelEntry, error) {
					return nil, nil
				},
				ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					switch {
					case query.Channel == channel:
						return messages, nil
					case query.PeerID == peerID:
						return messages, nil
					case query.Channel == "":
						return messages, nil
					default:
						return nil, nil
					}
				},
			}

			channelMessagesResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages?include_presence=true",
				nil,
			)
			if channelMessagesResp.Code != http.StatusOK {
				t.Fatalf(
					"channel messages code = %d, want %d",
					channelMessagesResp.Code,
					http.StatusOK,
				)
			}
			var channelMessagesPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, channelMessagesResp, &channelMessagesPayload)
			if got, want := len(channelMessagesPayload.Messages), 2; got != want {
				t.Fatalf("len(channel messages) = %d, want %d", got, want)
			}
			if got, want := channelMessagesPayload.Messages[0].Kind, "greet"; got != want {
				t.Fatalf("channel messages[0].Kind = %q, want %q", got, want)
			}
			if got, want := channelMessagesPayload.Messages[1].Kind, "say"; got != want {
				t.Fatalf("channel messages[1].Kind = %q, want %q", got, want)
			}

			peerMessagesResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true",
				nil,
			)
			if peerMessagesResp.Code != http.StatusOK {
				t.Fatalf("peer messages code = %d, want %d", peerMessagesResp.Code, http.StatusOK)
			}
			var peerMessagesPayload contract.NetworkPeerMessagesResponse
			testutil.DecodeJSONResponse(t, peerMessagesResp, &peerMessagesPayload)
			if got, want := len(peerMessagesPayload.Messages), 4; got != want {
				t.Fatalf("len(peer messages) = %d, want %d", got, want)
			}
			if got, want := peerMessagesPayload.Messages[1].Kind, "direct"; got != want {
				t.Fatalf("peer messages[1].Kind = %q, want %q", got, want)
			}
			if got, want := peerMessagesPayload.Messages[2].Kind, "receipt"; got != want {
				t.Fatalf("peer messages[2].Kind = %q, want %q", got, want)
			}
			if got, want := peerMessagesPayload.Messages[3].Kind, "trace"; got != want {
				t.Fatalf("peer messages[3].Kind = %q, want %q", got, want)
			}

			channelResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders",
				nil,
			)
			if channelResp.Code != http.StatusOK {
				t.Fatalf("channel detail code = %d, want %d", channelResp.Code, http.StatusOK)
			}
			var channelPayload contract.NetworkChannelResponse
			testutil.DecodeJSONResponse(t, channelResp, &channelPayload)
			if got, want := channelPayload.Channel.MessageCount, 1; got != want {
				t.Fatalf("channel detail message_count = %d, want %d", got, want)
			}
			if got, want := channelPayload.Channel.PresenceCount, 1; got != want {
				t.Fatalf("channel detail presence_count = %d, want %d", got, want)
			}
			if got, want := channelPayload.Channel.HistoricalParticipantCount, 2; got != want {
				t.Fatalf("channel detail historical_participant_count = %d, want %d", got, want)
			}
			if got, want := channelPayload.Channel.LastMessagePreview, "Public rollout update."; got != want {
				t.Fatalf("channel detail last_message_preview = %q, want %q", got, want)
			}
			if channelPayload.Channel.LastActivityAt == nil ||
				!channelPayload.Channel.LastActivityAt.Equal(recordedAt.Add(time.Minute)) {
				t.Fatalf(
					"channel detail last_activity_at = %#v, want %s",
					channelPayload.Channel.LastActivityAt,
					recordedAt.Add(time.Minute),
				)
			}
			if got, want := channelPayload.Channel.LastPresenceAt, new(
				recordedAt,
			); got == nil ||
				!got.Equal(*want) {
				t.Fatalf("channel detail last_presence_at = %#v, want %s", got, *want)
			}
			if got, want := channelPayload.Channel.KindCounts, []contract.NetworkChannelKindCountPayload{{
				Kind:  "say",
				Count: 1,
			}}; !reflect.DeepEqual(
				got,
				want,
			) {
				t.Fatalf("channel detail kind_counts = %#v, want %#v", got, want)
			}

			channelsResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels",
				nil,
			)
			if channelsResp.Code != http.StatusOK {
				t.Fatalf("channels list code = %d, want %d", channelsResp.Code, http.StatusOK)
			}
			var channelsPayload contract.NetworkChannelsResponse
			testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
			if got, want := len(channelsPayload.Channels), 1; got != want {
				t.Fatalf("len(channels) = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[0].MessageCount, 1; got != want {
				t.Fatalf("channels[0].MessageCount = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[0].PresenceCount, 1; got != want {
				t.Fatalf("channels[0].PresenceCount = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[0].HistoricalParticipantCount, 2; got != want {
				t.Fatalf("channels[0].HistoricalParticipantCount = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[0].LastMessagePreview, "Public rollout update."; got != want {
				t.Fatalf("channels[0].LastMessagePreview = %q, want %q", got, want)
			}
			if channelsPayload.Channels[0].LastActivityAt == nil ||
				!channelsPayload.Channels[0].LastActivityAt.Equal(recordedAt.Add(time.Minute)) {
				t.Fatalf(
					"channels[0].LastActivityAt = %#v, want %s",
					channelsPayload.Channels[0].LastActivityAt,
					recordedAt.Add(time.Minute),
				)
			}
		},
	)
}

func TestBaseHandlersNetworkChannelMessagesPaginateVisiblePublicTimeline(t *testing.T) {
	t.Run(
		"Should paginate against visible public messages instead of hidden greet or directed traffic",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{
					ListAllFn: func(context.Context) ([]*session.Info, error) {
						return nil, nil
					},
				},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, "builders"; got != want {
						t.Fatalf("ListPeers() channel = %q, want %q", got, want)
					}
					return nil, nil
				},
			}

			messages := []store.NetworkMessageEntry{
				{
					MessageID: "msg-greet-01",
					Channel:   "builders",
					Direction: network.AuditDirectionReceived,
					PeerFrom:  "reviewer.sess-remote",
					Kind:      "greet",
					Body: greetBodyJSON(
						"reviewer.sess-remote",
						"Reviewer",
						"Review pull requests",
						"",
					),
					Timestamp: recordedAt,
				},
				{
					MessageID:   "msg-say-01",
					Channel:     "builders",
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "founder.sess-local",
					Kind:        "say",
					Text:        "Public update one.",
					PreviewText: "Public update one.",
					Body:        json.RawMessage(`{"text":"Public update one."}`),
					Timestamp:   recordedAt.Add(time.Minute),
				},
				{
					MessageID:   "msg-direct-01",
					Channel:     "builders",
					Direction:   network.AuditDirectionReceived,
					PeerFrom:    "reviewer.sess-remote",
					PeerTo:      "coder.sess-local",
					Kind:        "direct",
					Text:        "Private handoff.",
					PreviewText: "Private handoff.",
					Body:        json.RawMessage(`{"text":"Private handoff."}`),
					Timestamp:   recordedAt.Add(2 * time.Minute),
				},
				{
					MessageID:   "msg-receipt-01",
					Channel:     "builders",
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-local",
					PeerTo:      "reviewer.sess-remote",
					Kind:        "receipt",
					Text:        "Receipt for private handoff.",
					PreviewText: "Receipt for private handoff.",
					Body: json.RawMessage(
						`{"status":"received","text":"Receipt for private handoff."}`,
					),
					Timestamp: recordedAt.Add(3 * time.Minute),
				},
				{
					MessageID:   "msg-say-02",
					Channel:     "builders",
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "founder.sess-local",
					Kind:        "say",
					Text:        "Public update two.",
					PreviewText: "Public update two.",
					Body:        json.RawMessage(`{"text":"Public update two."}`),
					Timestamp:   recordedAt.Add(4 * time.Minute),
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					if got, want := query.Channel, "builders"; got != want {
						t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
					}
					if query.Limit != 2 {
						t.Fatalf(
							"ListNetworkMessages() limit = %d, want one-row page plus overfetch",
							query.Limit,
						)
					}
					if !query.PublicOnly {
						t.Fatal("ListNetworkMessages() public_only = false, want store-owned visibility filter")
					}
					switch query.AfterMessageID {
					case "":
						return []store.NetworkMessageEntry{messages[1], messages[4]}, nil
					case "msg-say-01":
						return []store.NetworkMessageEntry{messages[4]}, nil
					case "msg-direct-01":
						return nil, sql.ErrNoRows
					default:
						t.Fatalf(
							"ListNetworkMessages() after = %q, want known visible/hidden cursor",
							query.AfterMessageID,
						)
						return nil, nil
					}
				},
			}

			limitResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages?limit=1",
				nil,
			)
			if limitResp.Code != http.StatusOK {
				t.Fatalf("limit messages code = %d, want %d", limitResp.Code, http.StatusOK)
			}
			var limitPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, limitResp, &limitPayload)
			if got, want := len(limitPayload.Messages), 1; got != want {
				t.Fatalf("len(limit messages) = %d, want %d", got, want)
			}
			if got, want := limitPayload.Messages[0].MessageID, "msg-say-02"; got != want {
				t.Fatalf("limit messages[0].message_id = %q, want %q", got, want)
			}

			afterResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages?after=msg-say-01&limit=1",
				nil,
			)
			if afterResp.Code != http.StatusOK {
				t.Fatalf("after messages code = %d, want %d", afterResp.Code, http.StatusOK)
			}
			var afterPayload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, afterResp, &afterPayload)
			if got, want := len(afterPayload.Messages), 1; got != want {
				t.Fatalf("len(after messages) = %d, want %d", got, want)
			}
			if got, want := afterPayload.Messages[0].MessageID, "msg-say-02"; got != want {
				t.Fatalf("after messages[0].message_id = %q, want %q", got, want)
			}

			hiddenCursorResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages?after=msg-direct-01&limit=1",
				nil,
			)
			if hiddenCursorResp.Code != http.StatusBadRequest {
				t.Fatalf(
					"hidden cursor messages code = %d, want %d; body=%s",
					hiddenCursorResp.Code,
					http.StatusBadRequest,
					hiddenCursorResp.Body.String(),
				)
			}
			var hiddenCursorPayload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, hiddenCursorResp, &hiddenCursorPayload)
			if !strings.Contains(hiddenCursorPayload.Error, "message cursor not found") {
				t.Fatalf(
					"hidden cursor payload = %#v, want message cursor not found error",
					hiddenCursorPayload,
				)
			}
		},
	)
}

func TestBaseHandlersNetworkPeerMessagesCanIncludePresenceWithoutBroadcasts(t *testing.T) {
	t.Run(
		"Should include peer presence history without reviving broadcast traffic",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 11, 19, 0, 0, 0, time.UTC)
			peerID := "reviewer.sess-remote"
			fixture := newHandlerFixture(t, testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, ""; got != want {
						t.Fatalf("ListPeers() channel = %q, want empty", got)
					}
					displayName := "Reviewer"
					return []network.PeerInfo{{
						PeerID:  peerID,
						Channel: "builders",
						Local:   false,
						PeerCard: network.PeerCard{
							PeerID:      peerID,
							DisplayName: &displayName,
						},
					}}, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(
					_ context.Context,
					query store.NetworkMessageQuery,
				) ([]store.NetworkMessageEntry, error) {
					if got, want := query.PeerID, peerID; got != want {
						t.Fatalf("ListNetworkMessages() peer_id = %q, want %q", got, want)
					}
					if query.IncludePresence && query.DirectedOnly {
						t.Fatal("include_presence query should not force directed_only")
					}
					if !query.IncludePresence && !query.DirectedOnly {
						t.Fatal("default peer timeline should stay directed_only")
					}
					return []store.NetworkMessageEntry{
						{
							MessageID: "msg-greet-01",
							Channel:   "builders",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  peerID,
							Kind:      "greet",
							Body: greetBodyJSON(
								peerID,
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt,
						},
						{
							MessageID:   "msg-direct-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    peerID,
							PeerTo:      "coder.sess-local",
							Kind:        "direct",
							Text:        "Please review the rollout.",
							PreviewText: "Please review the rollout.",
							Body:        json.RawMessage(`{"text":"Please review the rollout."}`),
							Timestamp:   recordedAt.Add(time.Minute),
						},
						{
							MessageID:   "msg-say-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    peerID,
							Kind:        "say",
							Text:        "This is a room-wide update.",
							PreviewText: "This is a room-wide update.",
							Body:        json.RawMessage(`{"text":"This is a room-wide update."}`),
							Timestamp:   recordedAt.Add(2 * time.Minute),
						},
					}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("peer messages code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkPeerMessagesResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Messages), 2; got != want {
				t.Fatalf("len(messages) = %d, want %d", got, want)
			}
			if got, want := payload.Messages[0].Kind, "greet"; got != want {
				t.Fatalf("messages[0].Kind = %q, want %q", got, want)
			}
			if got, want := payload.Messages[1].Kind, "direct"; got != want {
				t.Fatalf("messages[1].Kind = %q, want %q", got, want)
			}
		},
	)

	t.Run(
		"Should paginate coalesced peer presence episodes instead of raw greet rows",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 11, 21, 0, 0, 0, time.UTC)
			peerID := "reviewer.sess-remote"
			fixture := newHandlerFixture(t, testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			}, testutil.StubObserver{}, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, ""; got != want {
						t.Fatalf("ListPeers() channel = %q, want empty", got)
					}
					displayName := "Reviewer"
					return []network.PeerInfo{{
						PeerID:  peerID,
						Channel: "builders",
						Local:   false,
						PeerCard: network.PeerCard{
							PeerID:      peerID,
							DisplayName: &displayName,
						},
					}}, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(
					_ context.Context,
					query store.NetworkMessageQuery,
				) ([]store.NetworkMessageEntry, error) {
					if got, want := query.PeerID, peerID; got != want {
						t.Fatalf("ListNetworkMessages() peer_id = %q, want %q", got, want)
					}
					if query.DirectedOnly {
						t.Fatal(
							"include_presence peer query should fetch raw rows before visibility pagination",
						)
					}
					if query.Limit != 0 {
						t.Fatalf(
							"ListNetworkMessages() limit = %d, want 0 for raw fetch",
							query.Limit,
						)
					}
					if query.BeforeMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() before = %q, want empty raw cursor",
							query.BeforeMessageID,
						)
					}
					if query.AfterMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() after = %q, want empty raw cursor",
							query.AfterMessageID,
						)
					}
					return []store.NetworkMessageEntry{
						{
							MessageID: "msg-greet-01",
							Channel:   "builders",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  peerID,
							Kind:      "greet",
							Body: greetBodyJSON(
								peerID,
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt,
						},
						{
							MessageID: "msg-greet-02",
							Channel:   "builders",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  peerID,
							Kind:      "greet",
							Body: greetBodyJSON(
								peerID,
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt.Add(20 * time.Second),
						},
						{
							MessageID:   "msg-direct-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    peerID,
							PeerTo:      "coder.sess-local",
							Kind:        "direct",
							Text:        "Please review the rollout.",
							PreviewText: "Please review the rollout.",
							Body:        json.RawMessage(`{"text":"Please review the rollout."}`),
							Timestamp:   recordedAt.Add(time.Minute),
						},
					}, nil
				},
			}

			limitResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true&limit=1",
				nil,
			)
			if limitResp.Code != http.StatusOK {
				t.Fatalf("limit peer messages code = %d, want %d", limitResp.Code, http.StatusOK)
			}
			var limitPayload contract.NetworkPeerMessagesResponse
			testutil.DecodeJSONResponse(t, limitResp, &limitPayload)
			if got, want := len(limitPayload.Messages), 1; got != want {
				t.Fatalf("len(limit peer messages) = %d, want %d", got, want)
			}
			if got, want := limitPayload.Messages[0].MessageID, "msg-direct-01"; got != want {
				t.Fatalf("limit peer messages[0].message_id = %q, want %q", got, want)
			}

			afterResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true&after=msg-greet-02&limit=1",
				nil,
			)
			if afterResp.Code != http.StatusOK {
				t.Fatalf("after peer messages code = %d, want %d", afterResp.Code, http.StatusOK)
			}
			var afterPayload contract.NetworkPeerMessagesResponse
			testutil.DecodeJSONResponse(t, afterResp, &afterPayload)
			if got, want := len(afterPayload.Messages), 1; got != want {
				t.Fatalf("len(after peer messages) = %d, want %d", got, want)
			}
			if got, want := afterPayload.Messages[0].MessageID, "msg-direct-01"; got != want {
				t.Fatalf("after peer messages[0].message_id = %q, want %q", got, want)
			}
		},
	)
}

func TestBaseHandlersNetworkPeerMessagesPaginateVisiblePeerTimeline(t *testing.T) {
	t.Run(
		"Should paginate peer history against visible peer messages when presence is included",
		func(t *testing.T) {
			t.Parallel()

			recordedAt := time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC)
			peerID := "reviewer.sess-remote"
			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{
					ListAllFn: func(context.Context) ([]*session.Info, error) {
						return nil, nil
					},
				},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if got, want := channel, ""; got != want {
						t.Fatalf("ListPeers() channel = %q, want empty", got)
					}
					displayName := "Reviewer"
					return []network.PeerInfo{{
						PeerID:  peerID,
						Channel: "builders",
						Local:   false,
						PeerCard: network.PeerCard{
							PeerID:      peerID,
							DisplayName: &displayName,
						},
					}}, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(
					_ context.Context,
					query store.NetworkMessageQuery,
				) ([]store.NetworkMessageEntry, error) {
					if got, want := query.PeerID, peerID; got != want {
						t.Fatalf("ListNetworkMessages() peer_id = %q, want %q", got, want)
					}
					if query.DirectedOnly {
						t.Fatal(
							"include_presence peer query should fetch the raw lane before handler-side pagination",
						)
					}
					if query.Limit != 0 {
						t.Fatalf(
							"ListNetworkMessages() limit = %d, want handler-side pagination",
							query.Limit,
						)
					}
					if query.BeforeMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() before = %q, want empty raw fetch cursor",
							query.BeforeMessageID,
						)
					}
					if query.AfterMessageID != "" {
						t.Fatalf(
							"ListNetworkMessages() after = %q, want empty raw fetch cursor",
							query.AfterMessageID,
						)
					}
					return []store.NetworkMessageEntry{
						{
							MessageID: "msg-greet-01",
							Channel:   "builders",
							Direction: network.AuditDirectionReceived,
							PeerFrom:  peerID,
							Kind:      "greet",
							Body: greetBodyJSON(
								peerID,
								"Reviewer",
								"Review pull requests",
								"",
							),
							Timestamp: recordedAt,
						},
						{
							MessageID:   "msg-say-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    peerID,
							Kind:        "say",
							Text:        "Broadcast that should stay out of peer room.",
							PreviewText: "Broadcast that should stay out of peer room.",
							Body: json.RawMessage(
								`{"text":"Broadcast that should stay out of peer room."}`,
							),
							Timestamp: recordedAt.Add(time.Minute),
						},
						{
							MessageID:   "msg-direct-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    peerID,
							PeerTo:      "coder.sess-local",
							Kind:        "direct",
							Text:        "Direct handoff.",
							PreviewText: "Direct handoff.",
							Body:        json.RawMessage(`{"text":"Direct handoff."}`),
							Timestamp:   recordedAt.Add(2 * time.Minute),
						},
						{
							MessageID:   "msg-receipt-01",
							Channel:     "builders",
							Direction:   network.AuditDirectionSent,
							PeerFrom:    "coder.sess-local",
							PeerTo:      peerID,
							Kind:        "receipt",
							Text:        "Receipt for direct handoff.",
							PreviewText: "Receipt for direct handoff.",
							Body: json.RawMessage(
								`{"status":"received","text":"Receipt for direct handoff."}`,
							),
							Timestamp: recordedAt.Add(3 * time.Minute),
						},
					}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-remote/messages?include_presence=true&after=msg-greet-01&limit=2",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("peer messages code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkPeerMessagesResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Messages), 2; got != want {
				t.Fatalf("len(messages) = %d, want %d", got, want)
			}
			if got, want := payload.Messages[0].MessageID, "msg-direct-01"; got != want {
				t.Fatalf("messages[0].message_id = %q, want %q", got, want)
			}
			if got, want := payload.Messages[1].MessageID, "msg-receipt-01"; got != want {
				t.Fatalf("messages[1].message_id = %q, want %q", got, want)
			}
		},
	)
}

func TestBaseHandlersNetworkErrorsAndDisabledMode(t *testing.T) {
	t.Parallel()

	t.Run("Should return disabled status", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		disabledResp := performRequest(t, fixture.Engine, http.MethodGet, "/network/status", nil)
		if disabledResp.Code != http.StatusOK {
			t.Fatalf("disabled status code = %d, want %d", disabledResp.Code, http.StatusOK)
		}
		var disabledPayload contract.NetworkStatusResponse
		testutil.DecodeJSONResponse(t, disabledResp, &disabledPayload)
		if disabledPayload.Network.Enabled || disabledPayload.Network.Status != "disabled" {
			t.Fatalf("disabled payload = %#v, want disabled status", disabledPayload.Network)
		}
	})

	t.Run("Should return service unavailable when network service is missing", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers",
			nil,
		)
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"peers unavailable code = %d, want %d",
				resp.Code,
				http.StatusServiceUnavailable,
			)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "network service is required") {
			t.Fatalf("peers unavailable payload = %#v, want network service error", payload)
		}
	})

	t.Run(
		"Should return service unavailable for network detail handlers when service is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				networkTestSessionManager("ws-workspace", "sess-a"),
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true

			for _, path := range []string{"/workspaces/ws-workspace/network/peers/reviewer.sess-a", "/workspaces/ws-workspace/network/channels/builders"} {
				resp := performRequest(t, fixture.Engine, http.MethodGet, path, nil)
				if resp.Code != http.StatusServiceUnavailable {
					t.Fatalf(
						"%s code = %d, want %d",
						path,
						resp.Code,
						http.StatusServiceUnavailable,
					)
				}
			}
		},
	)

	t.Run(
		"Should return internal server error for peer detail when network store is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				networkTestSessionManager("ws-workspace", "sess-a"),
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if channel != "" {
						t.Fatalf("ListPeers() channel = %q, want empty filter", channel)
					}
					return []network.PeerInfo{{
						PeerID:   "reviewer.sess-a",
						Channel:  "builders",
						Local:    false,
						PeerCard: network.PeerCard{PeerID: "reviewer.sess-a"},
					}}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-a",
				nil,
			)
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf(
					"peer detail code = %d, want %d",
					resp.Code,
					http.StatusInternalServerError,
				)
			}
		},
	)

	t.Run("Should return bad request for blank network peer id", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers/%20",
			nil,
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("peer detail code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
	})

	t.Run(
		"Should return not found for missing network peer and channel details",
		func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{
					ListAllFn: func(context.Context) ([]*session.Info, error) {
						return nil, nil
					},
				},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(context.Context, string, string) ([]network.PeerInfo, error) {
					return nil, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkMessagesFn: func(context.Context, store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					return nil, nil
				},
				ListNetworkAuditFn: func(context.Context, store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error) {
					return nil, nil
				},
			}

			peerResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/peers/reviewer.sess-missing",
				nil,
			)
			if peerResp.Code != http.StatusNotFound {
				t.Fatalf("peer detail code = %d, want %d", peerResp.Code, http.StatusNotFound)
			}

			channelResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders",
				nil,
			)
			if channelResp.Code != http.StatusNotFound {
				t.Fatalf("channel detail code = %d, want %d", channelResp.Code, http.StatusNotFound)
			}
		},
	)

	t.Run(
		"Should return internal server error when channel messages store is missing",
		func(t *testing.T) {
			t.Parallel()

			fixture := newHandlerFixture(
				t,
				testutil.StubSessionManager{},
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages",
				nil,
			)
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf(
					"channel messages code = %d, want %d",
					resp.Code,
					http.StatusInternalServerError,
				)
			}
		},
	)

	t.Run("Should map network status error to 500", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			StatusFn: func(context.Context) (*network.Status, error) {
				return nil, errors.New("boom")
			},
		}

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/network/status", nil)
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status error code = %d, want %d", resp.Code, http.StatusInternalServerError)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "boom") {
			t.Fatalf("status error payload = %#v, want boom", payload)
		}
	})

	t.Run("Should map list channels error to 400", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(context.Context, string, string) ([]network.PeerInfo, error) {
				return nil, network.ErrInvalidField
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("channels error code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, network.ErrInvalidField.Error()) {
			t.Fatalf("channels error payload = %#v, want invalid field", payload)
		}
	})

	t.Run("Should return bad request on send decode", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/send",
			[]byte(`{`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("send decode code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "decode network send request") {
			t.Fatalf("send decode payload = %#v, want decode error", payload)
		}
	})

	t.Run("Should reject unsafe send content before the Network service", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			SendFn: func(context.Context, network.SendRequest) (string, error) {
				t.Fatal("Network.Send should not be called for unsafe public payloads")
				return "", nil
			},
		}

		const rawToken = "agh_claim_HTTP_SECURITY_123"
		tests := []struct {
			name       string
			body       string
			want       string
			mustRedact string
		}{
			{
				name:       "raw claim token",
				body:       `{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_token_rejected","kind":"say","body":{"claim_token":"` + rawToken + `"}}`,
				want:       "network_raw_token_rejected",
				mustRedact: rawToken,
			},
			{
				name: "caller supplied verified-format identity",
				body: `{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_identity_rejected","kind":"say","from":"alice@39f713d0a644253f04529421b9f51b9b","body":{"text":"spoof"}}`,
				want: "sender identity and proof are daemon-derived",
			},
		}
		for _, test := range tests {
			t.Run("Should reject "+test.name, func(t *testing.T) {
				resp := performRequest(
					t,
					fixture.Engine,
					http.MethodPost,
					"/workspaces/ws-workspace/network/send",
					[]byte(test.body),
				)
				if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), test.want) {
					t.Fatalf("unsafe send status/body = %d/%s, want %q", resp.Code, resp.Body.String(), test.want)
				}
				if test.mustRedact != "" && strings.Contains(resp.Body.String(), test.mustRedact) {
					t.Fatalf("unsafe send response leaked raw credential: %s", resp.Body.String())
				}
			})
		}
	})

	t.Run("Should map send target not found to 404", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			SendFn: func(context.Context, network.SendRequest) (string, error) {
				return "", network.ErrTargetPeerNotFound
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-workspace/network/send",
			[]byte(
				`{"session_id":"sess-a","channel":"builders","surface":"thread","thread_id":"thread_launch_db","kind":"say","body":{"text":"hello"}}`,
			),
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("send error code = %d, want %d", resp.Code, http.StatusNotFound)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, network.ErrTargetPeerNotFound.Error()) {
			t.Fatalf("send error payload = %#v, want target not found", payload)
		}
	})

	t.Run("Should return bad request when inbox is missing", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/inbox",
			nil,
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("inbox missing code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "session_id query is required") {
			t.Fatalf("inbox missing payload = %#v, want session_id validation error", payload)
		}
	})

	t.Run("Should map inbox invalid field to 400", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			InboxFn: func(context.Context, string) ([]network.Envelope, error) {
				return nil, network.ErrInvalidField
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/inbox?session_id=sess-a",
			nil,
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("inbox error code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, network.ErrInvalidField.Error()) {
			t.Fatalf("inbox error payload = %#v, want invalid field", payload)
		}
	})

	t.Run("Should preserve network error status mappings", func(t *testing.T) {
		t.Parallel()

		validationErr := core.NewNetworkValidationError(errors.New("missing session_id"))
		if !errors.Is(validationErr, core.ErrNetworkValidation) {
			t.Fatalf("validationErr = %v, want ErrNetworkValidation", validationErr)
		}
		if got := core.StatusForNetworkError(validationErr); got != http.StatusBadRequest {
			t.Fatalf("StatusForNetworkError(validation) = %d, want %d", got, http.StatusBadRequest)
		}
		if got := core.StatusForNetworkError(network.ErrTargetPeerNotFound); got != http.StatusNotFound {
			t.Fatalf("StatusForNetworkError(not found) = %d, want %d", got, http.StatusNotFound)
		}
		if got := core.StatusForNetworkError(network.ErrInvalidField); got != http.StatusBadRequest {
			t.Fatalf(
				"StatusForNetworkError(invalid field) = %d, want %d",
				got,
				http.StatusBadRequest,
			)
		}
	})
}

func TestValidationErrorHelpersPreserveInnerErrorChain(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve memory validation cause", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("bad memory payload")
		wrapped := core.NewMemoryValidationError(cause)
		if !errors.Is(wrapped, memory.ErrValidation) {
			t.Fatalf("NewMemoryValidationError() = %v, want memory.ErrValidation", wrapped)
		}
		if !errors.Is(wrapped, cause) {
			t.Fatalf("NewMemoryValidationError() = %v, want wrapped cause", wrapped)
		}
	})

	t.Run("Should preserve network validation cause", func(t *testing.T) {
		t.Parallel()

		cause := errors.New("missing session_id")
		wrapped := core.NewNetworkValidationError(cause)
		if !errors.Is(wrapped, core.ErrNetworkValidation) {
			t.Fatalf("NewNetworkValidationError() = %v, want ErrNetworkValidation", wrapped)
		}
		if !errors.Is(wrapped, cause) {
			t.Fatalf("NewNetworkValidationError() = %v, want wrapped cause", wrapped)
		}
	})
}

func TestBaseHandlersNetworkChannelEndpointsIgnoreStoppedSessions(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC)
	coderSessionID := "sess-coder"
	reviewerSessionID := "sess-reviewer"

	newFixture := func(t *testing.T) handlerFixture {
		t.Helper()

		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{
					{
						ID:                   coderSessionID,
						Name:                 "Coder",
						AgentName:            "coder",
						WorkspaceID:          "ws-workspace",
						NetworkParticipation: testLiveParticipation("ws-workspace", "builders"),
						Type:                 session.SessionTypeUser,
						State:                session.StateActive,
						CreatedAt:            createdAt,
						UpdatedAt:            createdAt,
					},
					{
						ID:                   reviewerSessionID,
						Name:                 "Reviewer",
						AgentName:            "reviewer",
						WorkspaceID:          "ws-workspace",
						NetworkParticipation: testLiveParticipation("ws-workspace", "retro"),
						Type:                 session.SessionTypeUser,
						State:                session.StateStopped,
						CreatedAt:            createdAt.Add(time.Minute),
						UpdatedAt:            createdAt.Add(time.Minute),
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				switch channel {
				case "":
					return []network.PeerInfo{
						{
							SessionID: &coderSessionID,
							PeerID:    "coder.sess-coder",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "coder.sess-coder"},
							JoinedAt:  new(createdAt),
						},
					}, nil
				case "builders":
					return []network.PeerInfo{
						{
							SessionID: &coderSessionID,
							PeerID:    "coder.sess-coder",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "coder.sess-coder"},
							JoinedAt:  new(createdAt),
						},
					}, nil
				case "retro":
					return nil, nil
				default:
					t.Fatalf("unexpected ListPeers() channel %q", channel)
					return nil, nil
				}
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkAuditFn: func(_ context.Context, query store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error) {
				switch query.Channel {
				case "builders":
					return []store.NetworkAuditEntry{{
						ID:        "naud-builders-01",
						SessionID: coderSessionID,
						Direction: network.AuditDirectionSent,
						Kind:      "say",
						Channel:   "builders",
						PeerFrom:  "coder.sess-coder",
						MessageID: "msg-builders-01",
						Size:      1,
						Timestamp: createdAt.Add(2 * time.Minute),
					}}, nil
				case "retro":
					return []store.NetworkAuditEntry{{
						ID:        "naud-retro-01",
						SessionID: reviewerSessionID,
						Direction: network.AuditDirectionSent,
						Kind:      "say",
						Channel:   "retro",
						PeerFrom:  "reviewer.sess-reviewer",
						MessageID: "msg-retro-01",
						Size:      1,
						Timestamp: createdAt.Add(3 * time.Minute),
					}}, nil
				default:
					return nil, nil
				}
			},
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				switch query.Channel {
				case "":
					return []store.NetworkMessageEntry{
						{
							MessageID:   "msg-builders-01",
							SessionID:   coderSessionID,
							Channel:     "builders",
							Direction:   network.AuditDirectionSent,
							PeerFrom:    "coder.sess-coder",
							Kind:        "say",
							Intent:      "announce",
							Text:        "hello builders",
							PreviewText: "hello builders",
							Body: json.RawMessage(
								`{"text":"hello builders","intent":"announce"}`,
							),
							Timestamp: createdAt.Add(2 * time.Minute),
						},
						{
							MessageID:   "msg-retro-01",
							SessionID:   reviewerSessionID,
							Channel:     "retro",
							Direction:   network.AuditDirectionSent,
							PeerFrom:    "reviewer.sess-reviewer",
							Kind:        "say",
							Text:        "retro note",
							PreviewText: "retro note",
							Body:        json.RawMessage(`{"text":"retro note"}`),
							Timestamp:   createdAt.Add(3 * time.Minute),
						},
					}, nil
				case "builders":
					return []store.NetworkMessageEntry{{
						MessageID:   "msg-builders-01",
						SessionID:   coderSessionID,
						Channel:     "builders",
						Direction:   network.AuditDirectionSent,
						PeerFrom:    "coder.sess-coder",
						Kind:        "say",
						Intent:      "announce",
						Text:        "hello builders",
						PreviewText: "hello builders",
						Body: json.RawMessage(
							`{"text":"hello builders","intent":"announce"}`,
						),
						Timestamp: createdAt.Add(2 * time.Minute),
					}}, nil
				case "retro":
					return []store.NetworkMessageEntry{{
						MessageID:   "msg-retro-01",
						SessionID:   reviewerSessionID,
						Channel:     "retro",
						Direction:   network.AuditDirectionSent,
						PeerFrom:    "reviewer.sess-reviewer",
						Kind:        "say",
						Text:        "retro note",
						PreviewText: "retro note",
						Body:        json.RawMessage(`{"text":"retro note"}`),
						Timestamp:   createdAt.Add(3 * time.Minute),
					}}, nil
				default:
					return nil, nil
				}
			},
		}
		return fixture
	}

	t.Run(
		"Should keep stopped sessions out of the channel list while preserving history-only channels",
		func(t *testing.T) {
			fixture := newFixture(t)

			channelsResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels",
				nil,
			)
			if channelsResp.Code != http.StatusOK {
				t.Fatalf("channels code = %d, want %d", channelsResp.Code, http.StatusOK)
			}

			var channelsPayload contract.NetworkChannelsResponse
			testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
			if got, want := len(channelsPayload.Channels), 2; got != want {
				t.Fatalf("len(channels) = %d, want %d", got, want)
			}
			sort.Slice(channelsPayload.Channels, func(i, j int) bool {
				return channelsPayload.Channels[i].Channel < channelsPayload.Channels[j].Channel
			})
			if got, want := channelsPayload.Channels[0].Channel, "builders"; got != want {
				t.Fatalf("channels[0].Channel = %q, want %q", got, want)
			}
			if got, want := channelsPayload.Channels[0].SessionCount, 1; got != want {
				t.Fatalf("channels[0].SessionCount = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[1].Channel, "retro"; got != want {
				t.Fatalf("channels[1].Channel = %q, want %q", got, want)
			}
			if got, want := channelsPayload.Channels[1].SessionCount, 0; got != want {
				t.Fatalf("channels[1].SessionCount = %d, want %d", got, want)
			}
			if got, want := channelsPayload.Channels[1].MessageCount, 1; got != want {
				t.Fatalf("channels[1].MessageCount = %d, want %d", got, want)
			}
		},
	)

	t.Run("Should exclude stopped sessions from active channel details", func(t *testing.T) {
		fixture := newFixture(t)

		channelResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders",
			nil,
		)
		if channelResp.Code != http.StatusOK {
			t.Fatalf("channel detail code = %d, want %d", channelResp.Code, http.StatusOK)
		}

		var channelPayload contract.NetworkChannelResponse
		testutil.DecodeJSONResponse(t, channelResp, &channelPayload)
		if got, want := channelPayload.Channel.Channel, "builders"; got != want {
			t.Fatalf("channel detail channel = %q, want %q", got, want)
		}
		if got, want := channelPayload.Channel.Peers[0].DisplayName, "Coder"; got != want {
			t.Fatalf("channel detail peer display = %q, want %q", got, want)
		}
		if got, want := channelPayload.Channel.MessageCount, 1; got != want {
			t.Fatalf("channel detail message count = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve active local authors in channel message history", func(t *testing.T) {
		fixture := newFixture(t)

		messagesResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/messages",
			nil,
		)
		if messagesResp.Code != http.StatusOK {
			t.Fatalf("channel messages code = %d, want %d", messagesResp.Code, http.StatusOK)
		}

		var messagesPayload contract.NetworkChannelMessagesResponse
		testutil.DecodeJSONResponse(t, messagesResp, &messagesPayload)
		if got, want := len(messagesPayload.Messages), 1; got != want {
			t.Fatalf("len(messages) = %d, want %d", got, want)
		}
		if got, want := messagesPayload.Messages[0].DisplayName, "Coder"; got != want {
			t.Fatalf("message display_name = %q, want %q", got, want)
		}
		if !messagesPayload.Messages[0].Local {
			t.Fatal("message local = false, want true")
		}
	})

	t.Run(
		"Should return history-only channel details without reviving stopped sessions",
		func(t *testing.T) {
			fixture := newFixture(t)

			historyResp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/retro",
				nil,
			)
			if historyResp.Code != http.StatusOK {
				t.Fatalf(
					"history-only channel detail code = %d, want %d",
					historyResp.Code,
					http.StatusOK,
				)
			}

			var historyPayload contract.NetworkChannelResponse
			testutil.DecodeJSONResponse(t, historyResp, &historyPayload)
			if got, want := historyPayload.Channel.Channel, "retro"; got != want {
				t.Fatalf("history-only channel = %q, want %q", got, want)
			}
			if got, want := historyPayload.Channel.SessionCount, 0; got != want {
				t.Fatalf("history-only session count = %d, want %d", got, want)
			}
			if got, want := historyPayload.Channel.PeerCount, 0; got != want {
				t.Fatalf("history-only peer count = %d, want %d", got, want)
			}
			if got, want := historyPayload.Channel.MessageCount, 1; got != want {
				t.Fatalf("history-only message count = %d, want %d", got, want)
			}
		},
	)

	t.Run("Should preserve history-only direct channels as public-empty rooms", func(t *testing.T) {
		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{
					{
						ID:                   "sess-founder",
						Name:                 "Founder",
						AgentName:            "founder",
						WorkspaceID:          "ws-workspace",
						NetworkParticipation: testLiveParticipation("ws-workspace", "handoff"),
						Type:                 session.SessionTypeUser,
						State:                session.StateStopped,
						CreatedAt:            createdAt,
						UpdatedAt:            createdAt,
					},
					{
						ID:                   "sess-coder",
						Name:                 "Coder",
						AgentName:            "coder",
						WorkspaceID:          "ws-workspace",
						NetworkParticipation: testLiveParticipation("ws-workspace", "handoff"),
						Type:                 session.SessionTypeUser,
						State:                session.StateStopped,
						CreatedAt:            createdAt.Add(time.Minute),
						UpdatedAt:            createdAt.Add(time.Minute),
					},
				}, nil
			},
		}

		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, "handoff"; got != want && got != "" {
					t.Fatalf("ListPeers() channel = %q, want %q or empty", got, want)
				}
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.Channel, "handoff"; got != want && got != "" {
					t.Fatalf("ListNetworkMessages() channel = %q, want %q or empty", got, want)
				}
				messages := []store.NetworkMessageEntry{
					{
						MessageID: "msg-handoff-greet-founder",
						SessionID: "sess-founder",
						Channel:   "handoff",
						Direction: network.AuditDirectionSent,
						PeerFrom:  "founder.sess-founder",
						Kind:      "greet",
						Body:      greetBodyJSON("founder.sess-founder", "Founder", "", ""),
						Timestamp: createdAt,
					},
					{
						MessageID: "msg-handoff-greet-coder",
						SessionID: "sess-coder",
						Channel:   "handoff",
						Direction: network.AuditDirectionSent,
						PeerFrom:  "coder.sess-coder",
						Kind:      "greet",
						Body: greetBodyJSON(
							"coder.sess-coder",
							"Coder",
							"Implement a fix",
							"",
						),
						Timestamp: createdAt.Add(time.Minute),
					},
					{
						MessageID:   "msg-handoff-direct",
						SessionID:   "sess-founder",
						Channel:     "handoff",
						Direction:   network.AuditDirectionSent,
						PeerFrom:    "founder.sess-founder",
						PeerTo:      "coder.sess-coder",
						Kind:        "direct",
						Text:        "Private handoff.",
						PreviewText: "Private handoff.",
						Body:        json.RawMessage(`{"text":"Private handoff."}`),
						Timestamp:   createdAt.Add(2 * time.Minute),
					},
				}
				if query.Channel == "" {
					return messages, nil
				}
				return messages, nil
			},
		}

		channelsResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels",
			nil,
		)
		if channelsResp.Code != http.StatusOK {
			t.Fatalf("channels code = %d, want %d", channelsResp.Code, http.StatusOK)
		}
		var channelsPayload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, channelsResp, &channelsPayload)
		if got, want := len(channelsPayload.Channels), 1; got != want {
			t.Fatalf("len(channels) = %d, want %d", got, want)
		}
		if got, want := channelsPayload.Channels[0].Channel, "handoff"; got != want {
			t.Fatalf("channels[0].Channel = %q, want %q", got, want)
		}
		if got, want := channelsPayload.Channels[0].MessageCount, 0; got != want {
			t.Fatalf("channels[0].MessageCount = %d, want %d", got, want)
		}
		if got, want := channelsPayload.Channels[0].PresenceCount, 2; got != want {
			t.Fatalf("channels[0].PresenceCount = %d, want %d", got, want)
		}
		if got, want := channelsPayload.Channels[0].HistoricalParticipantCount, 2; got != want {
			t.Fatalf("channels[0].HistoricalParticipantCount = %d, want %d", got, want)
		}

		historyResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/handoff",
			nil,
		)
		if historyResp.Code != http.StatusOK {
			t.Fatalf(
				"history-only direct channel detail code = %d, want %d",
				historyResp.Code,
				http.StatusOK,
			)
		}
		var historyPayload contract.NetworkChannelResponse
		testutil.DecodeJSONResponse(t, historyResp, &historyPayload)
		if got, want := historyPayload.Channel.Channel, "handoff"; got != want {
			t.Fatalf("history-only direct channel = %q, want %q", got, want)
		}
		if got, want := historyPayload.Channel.SessionCount, 0; got != want {
			t.Fatalf("history-only direct session count = %d, want %d", got, want)
		}
		if got, want := historyPayload.Channel.PeerCount, 0; got != want {
			t.Fatalf("history-only direct peer count = %d, want %d", got, want)
		}
		if got, want := historyPayload.Channel.MessageCount, 0; got != want {
			t.Fatalf("history-only direct message count = %d, want %d", got, want)
		}
		if got, want := historyPayload.Channel.PresenceCount, 2; got != want {
			t.Fatalf("history-only direct presence count = %d, want %d", got, want)
		}
		if got, want := historyPayload.Channel.HistoricalParticipantCount, 2; got != want {
			t.Fatalf("history-only direct historical participants = %d, want %d", got, want)
		}

		publicResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/handoff/messages",
			nil,
		)
		if publicResp.Code != http.StatusOK {
			t.Fatalf(
				"history-only direct public messages code = %d, want %d",
				publicResp.Code,
				http.StatusOK,
			)
		}
		var publicPayload contract.NetworkChannelMessagesResponse
		testutil.DecodeJSONResponse(t, publicResp, &publicPayload)
		if got, want := len(publicPayload.Messages), 0; got != want {
			t.Fatalf("len(public messages) = %d, want %d", got, want)
		}

		presenceResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/handoff/messages?include_presence=true",
			nil,
		)
		if presenceResp.Code != http.StatusOK {
			t.Fatalf(
				"history-only direct presence messages code = %d, want %d",
				presenceResp.Code,
				http.StatusOK,
			)
		}
		var presencePayload contract.NetworkChannelMessagesResponse
		testutil.DecodeJSONResponse(t, presenceResp, &presencePayload)
		if got, want := len(presencePayload.Messages), 2; got != want {
			t.Fatalf("len(presence messages) = %d, want %d", got, want)
		}
		for index, message := range presencePayload.Messages {
			if got, want := message.Kind, "greet"; got != want {
				t.Fatalf("presence messages[%d].Kind = %q, want %q", index, got, want)
			}
		}
	})
}

func TestBaseHandlersNetworkChannelMessagesPreserveRemoteAuthors(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should preserve remote author identity while keeping local session metadata intact",
		func(t *testing.T) {
			createdAt := time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC)
			localSessionID := "sess-coder"
			remotePeerID := "reviewer.sess-remote"

			manager := testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return []*session.Info{{
						ID:                   localSessionID,
						Name:                 "Coder",
						AgentName:            "coder",
						WorkspaceID:          "ws-workspace",
						NetworkParticipation: testLiveParticipation("ws-workspace", "builders"),
						Type:                 session.SessionTypeUser,
						State:                session.StateActive,
						CreatedAt:            createdAt,
						UpdatedAt:            createdAt,
					}}, nil
				},
			}

			fixture := newHandlerFixture(
				t,
				manager,
				testutil.StubObserver{},
				testutil.StubWorkspaceService{},
				nil,
				nil,
			)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
					if channel != "builders" {
						t.Fatalf("ListPeers() channel = %q, want builders", channel)
					}
					displayName := "Reviewer"
					return []network.PeerInfo{
						{
							SessionID: &localSessionID,
							PeerID:    "coder.sess-coder",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "coder.sess-coder"},
						},
						{
							PeerID:  remotePeerID,
							Channel: "builders",
							Local:   false,
							PeerCard: network.PeerCard{
								PeerID:      remotePeerID,
								DisplayName: &displayName,
							},
						},
					}, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				ListNetworkAuditFn: func(_ context.Context, query store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error) {
					if got, want := query.Channel, "builders"; got != want {
						t.Fatalf("ListNetworkAudit() channel = %q, want %q", got, want)
					}
					return []store.NetworkAuditEntry{
						{
							ID:        "naud-1",
							SessionID: localSessionID,
							Direction: network.AuditDirectionReceived,
							Kind:      "say",
							Channel:   "builders",
							PeerFrom:  remotePeerID,
							MessageID: "msg-remote-01",
							Size:      1,
							Timestamp: createdAt.Add(time.Minute),
						},
						{
							ID:        "naud-2",
							SessionID: localSessionID,
							Direction: network.AuditDirectionDelivered,
							Kind:      "say",
							Channel:   "builders",
							PeerFrom:  remotePeerID,
							MessageID: "msg-remote-01",
							Size:      1,
							Timestamp: createdAt.Add(2 * time.Minute),
						},
						{
							ID:        "naud-3",
							SessionID: localSessionID,
							Direction: network.AuditDirectionSent,
							Kind:      "say",
							Channel:   "builders",
							PeerFrom:  "coder.sess-coder",
							MessageID: "msg-local-01",
							Size:      1,
							Timestamp: createdAt.Add(3 * time.Minute),
						},
					}, nil
				},
				ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					if got, want := query.Channel, "builders"; got != want {
						t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
					}
					return []store.NetworkMessageEntry{
						{
							MessageID:   "msg-remote-01",
							SessionID:   localSessionID,
							Channel:     "builders",
							Direction:   network.AuditDirectionReceived,
							PeerFrom:    remotePeerID,
							Kind:        "say",
							Intent:      "review",
							Text:        "Please double-check the rollout.",
							PreviewText: "Please double-check the rollout.",
							Body: json.RawMessage(
								`{"text":"Please double-check the rollout.","intent":"review"}`,
							),
							Timestamp: createdAt.Add(time.Minute),
						},
						{
							MessageID:   "msg-local-01",
							SessionID:   localSessionID,
							Channel:     "builders",
							Direction:   network.AuditDirectionSent,
							PeerFrom:    "coder.sess-coder",
							Kind:        "say",
							Intent:      "announce",
							Text:        "Starting rollout now.",
							PreviewText: "Starting rollout now.",
							Body: json.RawMessage(
								`{"text":"Starting rollout now.","intent":"announce"}`,
							),
							Timestamp: createdAt.Add(3 * time.Minute),
						},
					}, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodGet,
				"/workspaces/ws-workspace/network/channels/builders/messages",
				nil,
			)
			if resp.Code != http.StatusOK {
				t.Fatalf("channel messages code = %d, want %d", resp.Code, http.StatusOK)
			}

			var payload contract.NetworkChannelMessagesResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := len(payload.Messages), 2; got != want {
				t.Fatalf("len(messages) = %d, want %d", got, want)
			}

			if got, want := payload.Messages[0].DisplayName, "Reviewer"; got != want {
				t.Fatalf("remote display_name = %q, want %q", got, want)
			}
			if payload.Messages[0].Local {
				t.Fatal("remote message local = true, want false")
			}
			if got := payload.Messages[0].SessionID; got != "" {
				t.Fatalf("remote session_id = %q, want empty", got)
			}

			if got, want := payload.Messages[1].DisplayName, "Coder"; got != want {
				t.Fatalf("local display_name = %q, want %q", got, want)
			}
			if !payload.Messages[1].Local {
				t.Fatal("local message local = false, want true")
			}
			if got, want := payload.Messages[1].SessionID, localSessionID; got != want {
				t.Fatalf("local session_id = %q, want %q", got, want)
			}
		},
	)

	t.Run("Should return not found when a channel has no presence or history", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, "builders"; got != want {
					t.Fatalf("ListPeers() channel = %q, want %q", got, want)
				}
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.Channel, "builders"; got != want {
					t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
				}
				if !query.PublicOnly {
					if query.AfterMessageID != "" || query.Limit != 0 {
						t.Fatalf("projection ListNetworkMessages() query = %#v, want unbounded channel lookup", query)
					}
					return nil, nil
				}
				if query.AfterMessageID != "msg-missing" {
					t.Fatalf(
						"ListNetworkMessages() after = %q, want forwarded public cursor",
						query.AfterMessageID,
					)
				}
				if !query.PublicOnly || query.Limit != 11 {
					t.Fatalf("ListNetworkMessages() query = %#v, want public overfetch limit 11", query)
				}
				return nil, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/messages?after=msg-missing&limit=10",
			nil,
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("channel messages code = %d, want %d", resp.Code, http.StatusNotFound)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, "network channel not found") {
			t.Fatalf(
				"channel messages payload = %#v, want network channel not found error",
				payload,
			)
		}
	})

	t.Run("Should return an empty cursor page for a history-only channel", func(t *testing.T) {
		t.Parallel()

		recordedAt := time.Date(2026, 4, 12, 13, 0, 0, 0, time.UTC)
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if got, want := channel, "builders"; got != want {
					t.Fatalf("ListPeers() channel = %q, want %q", got, want)
				}
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if got, want := query.Channel, "builders"; got != want {
					t.Fatalf("ListNetworkMessages() channel = %q, want %q", got, want)
				}
				if query.AfterMessageID == "msg-last" {
					if !query.PublicOnly || query.Limit != 11 {
						t.Fatalf("ListNetworkMessages() query = %#v, want public overfetch limit 11", query)
					}
					return nil, nil
				}
				if query.AfterMessageID != "" {
					t.Fatalf(
						"ListNetworkMessages() after = %q, want empty projection lookup cursor",
						query.AfterMessageID,
					)
				}
				return []store.NetworkMessageEntry{{
					MessageID:   "msg-last",
					Channel:     "builders",
					Direction:   network.AuditDirectionSent,
					PeerFrom:    "coder.sess-local",
					Kind:        "say",
					Text:        "Past update.",
					PreviewText: "Past update.",
					Body:        json.RawMessage(`{"text":"Past update."}`),
					Timestamp:   recordedAt,
				}}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/messages?after=msg-last&limit=10",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"channel messages code = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
		var payload contract.NetworkChannelMessagesResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got := len(payload.Messages); got != 0 {
			t.Fatalf("len(channel messages) = %d, want empty cursor page", got)
		}
	})

	t.Run("Should reject invalid channel message limits", func(t *testing.T) {
		t.Parallel()

		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/channels/builders/messages?limit=abc",
			nil,
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("channel messages code = %d, want %d", resp.Code, http.StatusBadRequest)
		}
		var payload contract.ErrorPayload
		testutil.DecodeJSONResponse(t, resp, &payload)
		if !strings.Contains(payload.Error, `invalid integer "abc"`) {
			t.Fatalf("channel messages payload = %#v, want invalid integer error", payload)
		}
	})
}

func TestBaseHandlersCreateNetworkChannelCreatesSessionsPerAgent(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should create one session per requested agent and return the aggregated channel payload",
		func(t *testing.T) {
			var createCalls []session.CreateOpts
			channelPersisted := false
			manager := testutil.StubSessionManager{
				CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
					if !channelPersisted {
						t.Fatal("Create() called before durable channel authority was persisted")
					}
					createCalls = append(createCalls, opts)
					channelID := testParticipationRequestChannel(opts.NetworkParticipation)
					return &session.Session{
						ID:                   "sess-" + opts.AgentName,
						Name:                 strings.ToUpper(opts.AgentName),
						AgentName:            opts.AgentName,
						WorkspaceID:          opts.Workspace,
						NetworkParticipation: testLiveParticipation(opts.Workspace, channelID),
						Type:                 session.SessionTypeUser,
						State:                session.StateActive,
						CreatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
						UpdatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
					}, nil
				},
				ListAllFn: func(_ context.Context) ([]*session.Info, error) {
					infos := make([]*session.Info, 0, len(createCalls))
					for _, call := range createCalls {
						channelID := testParticipationRequestChannel(call.NetworkParticipation)
						infos = append(infos, &session.Info{
							ID:                   "sess-" + call.AgentName,
							Name:                 strings.ToUpper(call.AgentName),
							AgentName:            call.AgentName,
							WorkspaceID:          call.Workspace,
							NetworkParticipation: testLiveParticipation(call.Workspace, channelID),
							Type:                 session.SessionTypeUser,
							State:                session.StateActive,
							CreatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
							UpdatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
						})
					}
					return infos, nil
				},
			}
			workspaces := testutil.StubWorkspaceService{
				ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
					if ref != "ws-workspace" {
						t.Fatalf("Resolve() ref = %q, want ws-workspace", ref)
					}
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "ws-workspace", Name: "Workspace"},
						WorkspaceID: "ws-stable",
						Agents: []aghconfig.AgentDef{
							{Name: "coder"},
							{Name: "reviewer"},
						},
					}, nil
				},
			}
			fixture := newHandlerFixture(t, manager, testutil.StubObserver{}, workspaces, nil, nil)
			fixture.Handlers.Config.Network.Enabled = true
			fixture.Handlers.Network = testutil.StubNetworkService{
				ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
					if workspaceID != "ws-workspace" {
						t.Fatalf("ListPeers() workspaceID = %q, want ws-workspace", workspaceID)
					}
					if channel != "builders" {
						return nil, nil
					}
					coderSessionID := "sess-coder"
					reviewerSessionID := "sess-reviewer"
					return []network.PeerInfo{
						{
							SessionID: &coderSessionID,
							PeerID:    "coder.sess-coder",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "coder.sess-coder"},
						},
						{
							SessionID: &reviewerSessionID,
							PeerID:    "reviewer.sess-reviewer",
							Channel:   "builders",
							Local:     true,
							PeerCard:  network.PeerCard{PeerID: "reviewer.sess-reviewer"},
						},
					}, nil
				},
			}
			fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
				CreateNetworkChannelFn: func(_ context.Context, entry store.NetworkChannelEntry) error {
					if got, want := entry.WorkspaceID, "ws-workspace"; got != want {
						t.Fatalf("CreateNetworkChannel() workspace_id = %q, want %q", got, want)
					}
					channelPersisted = true
					return nil
				},
				ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
					if query.Channel != "builders" {
						return nil, nil
					}
					return nil, nil
				},
			}

			resp := performRequest(
				t,
				fixture.Engine,
				http.MethodPost,
				"/workspaces/ws-workspace/network/channels",
				[]byte(
					`{"channel":"builders","workspace_id":"ws-workspace","purpose":"Cross-agent coordination","agent_names":["coder","reviewer"]}`,
				),
			)
			if resp.Code != http.StatusCreated {
				t.Fatalf(
					"create channel code = %d, want %d; body=%s",
					resp.Code,
					http.StatusCreated,
					resp.Body.String(),
				)
			}

			if got, want := len(createCalls), 2; got != want {
				t.Fatalf("len(createCalls) = %d, want %d", got, want)
			}
			expectedAgents := map[string]struct{}{
				"coder":    {},
				"reviewer": {},
			}
			for _, call := range createCalls {
				if _, ok := expectedAgents[call.AgentName]; !ok {
					t.Fatalf("Create() agent = %q, want coder/reviewer", call.AgentName)
				}
				delete(expectedAgents, call.AgentName)
				if got, want := call.Workspace, "ws-workspace"; got != want {
					t.Fatalf("Create() workspace = %q, want %q", got, want)
				}
				if got := call.Provider; got != "" {
					t.Fatalf("Create() provider = %q, want explicit empty provider", got)
				}
				if got, want := testParticipationRequestChannel(call.NetworkParticipation), "builders"; got != want {
					t.Fatalf("Create() channel = %q, want %q", got, want)
				}
			}
			if len(expectedAgents) != 0 {
				t.Fatalf("missing Create() calls for agents: %#v", expectedAgents)
			}

			var payload contract.CreateNetworkChannelResponse
			testutil.DecodeJSONResponse(t, resp, &payload)
			if got, want := payload.Channel.SessionCount, 2; got != want {
				t.Fatalf("payload.Channel.SessionCount = %d, want %d", got, want)
			}
			if got, want := len(payload.Channel.Peers), 2; got != want {
				t.Fatalf("len(payload.Channel.Peers) = %d, want %d", got, want)
			}
			for index, peer := range payload.Channel.Peers {
				if peer.PeerCard.ProfilesSupported == nil {
					t.Fatalf(
						"payload.Channel.Peers[%d].PeerCard.ProfilesSupported = nil, want empty slice",
						index,
					)
				}
				if peer.PeerCard.ArtifactsSupported == nil {
					t.Fatalf(
						"payload.Channel.Peers[%d].PeerCard.ArtifactsSupported = nil, want empty slice",
						index,
					)
				}
				if peer.PeerCard.TrustModesSupported == nil {
					t.Fatalf(
						"payload.Channel.Peers[%d].PeerCard.TrustModesSupported = nil, want empty slice",
						index,
					)
				}
			}
		},
	)
}

func TestBaseHandlersNetworkUsesRegistryWorkspaceIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should list persisted channels with the registry workspace id", func(t *testing.T) {
		t.Parallel()

		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-stable" {
					t.Fatalf("Resolve() ref = %q, want ws-stable", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-workspace", Name: "Workspace"},
					WorkspaceID: "ws-stable",
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			workspaces,
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, workspaceID string, channel string) ([]network.PeerInfo, error) {
				if workspaceID != "ws-workspace" {
					t.Fatalf("ListPeers() workspaceID = %q, want ws-workspace", workspaceID)
				}
				if channel != "" {
					t.Fatalf("ListPeers() channel = %q, want empty list filter", channel)
				}
				return nil, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkChannelsFn: func(_ context.Context, query store.NetworkChannelQuery) ([]store.NetworkChannelEntry, error) {
				if query.WorkspaceID != "ws-workspace" {
					t.Fatalf(
						"ListNetworkChannels() workspace_id = %q, want ws-workspace",
						query.WorkspaceID,
					)
				}
				return []store.NetworkChannelEntry{{
					WorkspaceID: "ws-workspace",
					Channel:     "builders",
					Purpose:     "Coordinate builders",
					CreatedBy:   "general",
					CreatedAt:   time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
					UpdatedAt:   time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
				}}, nil
			},
			ListNetworkMessagesFn: func(_ context.Context, query store.NetworkMessageQuery) ([]store.NetworkMessageEntry, error) {
				if query.WorkspaceID != "ws-workspace" {
					t.Fatalf(
						"ListNetworkMessages() workspace_id = %q, want ws-workspace",
						query.WorkspaceID,
					)
				}
				return nil, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-stable/network/channels",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"channels code = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}

		var payload contract.NetworkChannelsResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if len(payload.Channels) != 1 || payload.Channels[0].Channel != "builders" {
			t.Fatalf("channels payload = %#v, want builders channel", payload.Channels)
		}
		if got, want := payload.Channels[0].WorkspaceID, "ws-workspace"; got != want {
			t.Fatalf("channel workspace_id = %q, want %q", got, want)
		}
	})

	t.Run("Should send messages with the registry workspace id", func(t *testing.T) {
		t.Parallel()

		workspaces := testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "ws-stable" {
					t.Fatalf("Resolve() ref = %q, want ws-stable", ref)
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "ws-workspace", Name: "Workspace"},
					WorkspaceID: "ws-stable",
				}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			networkTestSessionManager("ws-workspace", "sess-a"),
			testutil.StubObserver{},
			workspaces,
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			SendFn: func(_ context.Context, req network.SendRequest) (string, error) {
				if req.WorkspaceID != "ws-workspace" {
					t.Fatalf("Send() workspace_id = %q, want ws-workspace", req.WorkspaceID)
				}
				return "msg-1", nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/workspaces/ws-stable/network/send",
			[]byte(
				"{\"workspace_id\":\"ws-stable\",\"session_id\":\"sess-a\",\"channel\":\"builders\",\"surface\":\"thread\",\"thread_id\":\"thread_launch_db\",\"kind\":\"say\",\"body\":{\"text\":\"hello\"}}",
			),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf(
				"send code = %d, want %d; body=%s",
				resp.Code,
				http.StatusOK,
				resp.Body.String(),
			)
		}
	})
}

func TestBaseHandlersNetworkPeerDetailUsesAuditMetrics(t *testing.T) {
	t.Parallel()

	t.Run("Should derive peer metrics from persisted audit history", func(t *testing.T) {
		coderSessionID := "sess-coder"
		manager := testutil.StubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{{
					ID:                   coderSessionID,
					Name:                 "Coder",
					AgentName:            "coder",
					WorkspaceID:          "ws-workspace",
					NetworkParticipation: testLiveParticipation("ws-workspace", "builders"),
					Type:                 session.SessionTypeUser,
					State:                session.StateActive,
					CreatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
					UpdatedAt:            time.Date(2026, 4, 11, 18, 0, 0, 0, time.UTC),
				}}, nil
			},
		}
		fixture := newHandlerFixture(
			t,
			manager,
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if channel != "" {
					t.Fatalf("ListPeers() channel = %q, want empty filter", channel)
				}
				return []network.PeerInfo{{
					SessionID: &coderSessionID,
					PeerID:    "coder.sess-coder",
					Channel:   "builders",
					Local:     true,
					PeerCard: network.PeerCard{
						PeerID:              "coder.sess-coder",
						Capabilities:        []string{"review-pr"},
						ProfilesSupported:   []string{network.ProtocolV0},
						ArtifactsSupported:  []string{},
						TrustModesSupported: []string{},
						Ext: network.ExtensionMap{
							"agh.capabilities_brief": json.RawMessage(
								`[{"id":"review-pr","summary":"Review pull requests"}]`,
							),
						},
					},
					CapabilityCatalog: []session.NetworkPeerCapability{{
						ID:                "review-pr",
						Summary:           "Review pull requests",
						Outcome:           "Actionable review findings",
						Version:           "1.0.0",
						Digest:            "sha256:review-pr-v1",
						ContextNeeded:     []string{"pull request link"},
						ArtifactsExpected: []string{"review summary"},
						Requirements:      []string{"workspace-read"},
					}},
					CapabilityCatalogKnown: true,
				}}, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkAuditFn: func(_ context.Context, query store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error) {
				if query.SessionID != coderSessionID {
					t.Fatalf(
						"ListNetworkAudit() session_id = %q, want %q",
						query.SessionID,
						coderSessionID,
					)
				}
				return []store.NetworkAuditEntry{
					{
						SessionID: coderSessionID,
						Direction: network.AuditDirectionSent,
						Kind:      "say",
						Channel:   "builders",
						PeerFrom:  "coder.sess-coder",
						MessageID: "msg-1",
						Size:      11,
					},
					{
						SessionID: coderSessionID,
						Direction: network.AuditDirectionReceived,
						Kind:      "direct",
						Channel:   "builders",
						PeerFrom:  "reviewer.sess-remote",
						MessageID: "msg-2",
						Size:      22,
					},
					{
						SessionID: coderSessionID,
						Direction: network.AuditDirectionDelivered,
						Kind:      "say",
						Channel:   "builders",
						PeerFrom:  "coder.sess-coder",
						MessageID: "msg-1",
						Size:      33,
					},
					{
						SessionID: coderSessionID,
						Direction: network.AuditDirectionRejected,
						Kind:      "receipt",
						Channel:   "builders",
						PeerFrom:  "reviewer.sess-remote",
						MessageID: "msg-3",
						Reason:    "busy",
						Size:      44,
					},
				}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers/coder.sess-coder",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("peer detail code = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload contract.NetworkPeerResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got, want := payload.Peer.DisplayName, "Coder"; got != want {
			t.Fatalf("payload.Peer.DisplayName = %q, want %q", got, want)
		}
		if got, want := payload.Peer.Metrics.Sent, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Sent = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.Received, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Received = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.Delivered, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Delivered = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.Rejected, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Rejected = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.SentSizeBytes, int64(11); got != want {
			t.Fatalf("payload.Peer.Metrics.SentSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.ReceivedSizeBytes, int64(22); got != want {
			t.Fatalf("payload.Peer.Metrics.ReceivedSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.DeliveredSizeBytes, int64(33); got != want {
			t.Fatalf("payload.Peer.Metrics.DeliveredSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.RejectedSizeBytes, int64(44); got != want {
			t.Fatalf("payload.Peer.Metrics.RejectedSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.TotalSizeBytes, int64(110); got != want {
			t.Fatalf("payload.Peer.Metrics.TotalSizeBytes = %d, want %d", got, want)
		}
		if payload.Peer.PeerCard.ProfilesSupported == nil {
			t.Fatal("payload.Peer.PeerCard.ProfilesSupported = nil, want slice")
		}
		if got, want := payload.Peer.PeerCard.ProfilesSupported, []string{
			network.ProtocolV0,
		}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("payload.Peer.PeerCard.ProfilesSupported = %#v, want %#v", got, want)
		}
		if payload.Peer.PeerCard.ArtifactsSupported == nil {
			t.Fatal("payload.Peer.PeerCard.ArtifactsSupported = nil, want empty slice")
		}
		if got := len(payload.Peer.PeerCard.ArtifactsSupported); got != 0 {
			t.Fatalf("len(payload.Peer.PeerCard.ArtifactsSupported) = %d, want 0", got)
		}
		if payload.Peer.PeerCard.TrustModesSupported == nil {
			t.Fatal("payload.Peer.PeerCard.TrustModesSupported = nil, want empty slice")
		}
		if got := len(payload.Peer.PeerCard.TrustModesSupported); got != 0 {
			t.Fatalf("len(payload.Peer.PeerCard.TrustModesSupported) = %d, want 0", got)
		}
		if got, want := payload.Peer.PeerCard.Capabilities, []contract.NetworkCapabilityBriefPayload{{
			ID:      "review-pr",
			Summary: "Review pull requests",
		}}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("peer detail capability brief = %#v, want %#v", got, want)
		}
		if _, ok := payload.Peer.PeerCard.Ext["agh.capabilities_brief"]; ok {
			t.Fatalf(
				"peer detail capability brief ext should be stripped: %#v",
				payload.Peer.PeerCard.Ext,
			)
		}
		if payload.Peer.CapabilityCatalog == nil {
			t.Fatal("payload.Peer.CapabilityCatalog = nil, want rich capability catalog")
		}
		if got, want := payload.Peer.CapabilityCatalog.Capabilities, []contract.NetworkCapabilityPayload{{
			ID:                "review-pr",
			Summary:           "Review pull requests",
			Outcome:           "Actionable review findings",
			Version:           "1.0.0",
			Digest:            "sha256:review-pr-v1",
			ContextNeeded:     []string{"pull request link"},
			ArtifactsExpected: []string{"review summary"},
			Requirements:      []string{"workspace-read"},
		}}; !reflect.DeepEqual(
			got,
			want,
		) {
			t.Fatalf("peer detail capability catalog = %#v, want %#v", got, want)
		}
	})

	t.Run("Should derive remote peer metrics from channel audit history", func(t *testing.T) {
		fixture := newHandlerFixture(
			t,
			testutil.StubSessionManager{
				ListAllFn: func(context.Context) ([]*session.Info, error) {
					return nil, nil
				},
			},
			testutil.StubObserver{},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Config.Network.Enabled = true

		remoteDisplayName := "Reviewer"
		fixture.Handlers.Network = testutil.StubNetworkService{
			ListPeersFn: func(_ context.Context, _ string, channel string) ([]network.PeerInfo, error) {
				if channel != "" {
					t.Fatalf("ListPeers() channel = %q, want empty filter", channel)
				}
				return []network.PeerInfo{{
					PeerID:  "reviewer.sess-remote",
					Channel: "builders",
					Local:   false,
					PeerCard: network.PeerCard{
						PeerID:              "reviewer.sess-remote",
						DisplayName:         &remoteDisplayName,
						Capabilities:        []string{"review-pr"},
						ProfilesSupported:   []string{network.ProtocolV0},
						ArtifactsSupported:  []string{},
						TrustModesSupported: []string{},
						Ext: network.ExtensionMap{
							"agh.capabilities_brief": json.RawMessage(
								`[{"id":"review-pr","summary":"Review pull requests"}]`,
							),
						},
					},
				}}, nil
			},
		}
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListNetworkAuditFn: func(_ context.Context, query store.NetworkAuditQuery) ([]store.NetworkAuditEntry, error) {
				if got, want := query.Channel, "builders"; got != want {
					t.Fatalf("ListNetworkAudit() channel = %q, want %q", got, want)
				}
				if query.SessionID != "" {
					t.Fatalf(
						"ListNetworkAudit() session_id = %q, want empty remote lookup",
						query.SessionID,
					)
				}
				return []store.NetworkAuditEntry{
					{
						Direction: network.AuditDirectionReceived,
						Kind:      "say",
						Channel:   "builders",
						PeerFrom:  "reviewer.sess-remote",
						MessageID: "msg-remote-1",
						Size:      77,
					},
					{
						Direction: network.AuditDirectionDelivered,
						Kind:      "direct",
						Channel:   "builders",
						PeerTo:    "reviewer.sess-remote",
						MessageID: "msg-remote-2",
						Size:      88,
					},
					{
						Direction: network.AuditDirectionRejected,
						Kind:      "receipt",
						Channel:   "builders",
						PeerFrom:  "other.sess-peer",
						MessageID: "msg-other",
						Size:      99,
					},
				}, nil
			},
		}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/workspaces/ws-workspace/network/peers/reviewer.sess-remote",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("remote peer detail code = %d, want %d", resp.Code, http.StatusOK)
		}

		var payload contract.NetworkPeerResponse
		testutil.DecodeJSONResponse(t, resp, &payload)
		if got, want := payload.Peer.DisplayName, "Reviewer"; got != want {
			t.Fatalf("payload.Peer.DisplayName = %q, want %q", got, want)
		}
		if got, want := payload.Peer.Metrics.Received, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Received = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.Delivered, int64(1); got != want {
			t.Fatalf("payload.Peer.Metrics.Delivered = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.Rejected, int64(0); got != want {
			t.Fatalf("payload.Peer.Metrics.Rejected = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.ReceivedSizeBytes, int64(77); got != want {
			t.Fatalf("payload.Peer.Metrics.ReceivedSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.DeliveredSizeBytes, int64(88); got != want {
			t.Fatalf("payload.Peer.Metrics.DeliveredSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.RejectedSizeBytes, int64(0); got != want {
			t.Fatalf("payload.Peer.Metrics.RejectedSizeBytes = %d, want %d", got, want)
		}
		if got, want := payload.Peer.Metrics.TotalSizeBytes, int64(165); got != want {
			t.Fatalf("payload.Peer.Metrics.TotalSizeBytes = %d, want %d", got, want)
		}
	})
}

func greetBodyJSON(
	peerID string,
	displayName string,
	capabilitySummary string,
	summary string,
) json.RawMessage {
	return json.RawMessage(`{"peer_card":{"peer_id":"` +
		peerID +
		`","display_name":"` +
		displayName +
		`","profiles_supported":["agh-network/v0"],"capabilities":["review-pr"],"artifacts_supported":["capability"],"trust_modes_supported":[],"ext":{"agh.capabilities_brief":[{"id":"review-pr","summary":"` +
		capabilitySummary +
		`"}]}},"summary":"` +
		summary +
		`"}`)
}
