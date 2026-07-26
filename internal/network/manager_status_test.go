// Suite: Network manager status
// Invariant: status is disabled, ready, or active according to availability and Live participation.
// Boundary IN: the real in-process Network manager and participant registry.
// Boundary OUT: HTTP/OpenAPI projection, owned by internal/api/core/network_test.go.
package network

import (
	"context"
	"slices"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
)

func TestManagerStatusReportsAvailabilityAndParticipation(t *testing.T) {
	t.Parallel()

	t.Run("Should report disabled ready and active without transport states", func(t *testing.T) {
		t.Parallel()

		manager, err := NewManager(
			t.Context(),
			aghconfig.DefaultNetworkConfig(),
			"",
			nil,
			WithManagerLogger(discardManagerLogger()),
			WithManagerAuditWriter(managerAuditWriterStub{}),
		)
		if err != nil {
			t.Fatalf("NewManager() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})

		ready, err := manager.Status(t.Context())
		if err != nil {
			t.Fatalf("Status(ready) error = %v", err)
		}
		if ready.Status != StatusReady || ready.LocalPeers != 0 {
			t.Fatalf("Status(ready) = %#v, want ready with zero Live participants", ready)
		}

		joinManagerSendParticipant(t, manager, "sess-live", "coder.sess-live")
		active, err := manager.Status(t.Context())
		if err != nil {
			t.Fatalf("Status(active) error = %v", err)
		}
		if active.Status != StatusActive || active.LocalPeers != 1 {
			t.Fatalf("Status(active) = %#v, want active with one Live participant", active)
		}

		manager.SetEnabled(false)
		disabled, err := manager.Status(t.Context())
		if err != nil {
			t.Fatalf("Status(disabled) error = %v", err)
		}
		if disabled.Status != StatusDisabled || disabled.Enabled {
			t.Fatalf("Status(disabled) = %#v, want disabled availability", disabled)
		}
	})
}

func TestManagerMembershipDispatchesLifecycleHooks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 17, 12, 30, 0, 0, time.UTC)
	hooks := &managerHookDispatcherSpy{}
	manager, err := NewManager(
		t.Context(),
		aghconfig.DefaultNetworkConfig(),
		"",
		nil,
		WithManagerLogger(discardManagerLogger()),
		WithManagerClock(func() time.Time { return now }),
		WithManagerAuditWriter(managerAuditWriterStub{}),
		WithManagerHookDispatcher(hooks),
	)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	t.Cleanup(func() {
		if shutdownErr := manager.Shutdown(context.Background()); shutdownErr != nil {
			t.Fatalf("Shutdown() error = %v", shutdownErr)
		}
	})

	joinManagerSendParticipant(t, manager, "sess-live", "coder.sess-live")
	if err := manager.LeaveChannel(t.Context(), "sess-live"); err != nil {
		t.Fatalf("LeaveChannel() error = %v", err)
	}
	want := []hookspkg.HookEvent{hookspkg.HookNetworkPeerJoined, hookspkg.HookNetworkPeerLeft}
	if got := hooks.events(); !slices.Equal(got, want) {
		t.Fatalf("lifecycle hook events = %#v, want %#v", got, want)
	}
	for _, call := range hooks.callsSnapshot() {
		if call.payload.PeerID != "coder.sess-live" ||
			call.payload.WorkspaceID != testWorkspaceID ||
			call.payload.Channel != "builders" {
			t.Fatalf("lifecycle payload = %#v, want joined peer identity", call.payload)
		}
	}
}

func TestNetworkMetricLabelsExcludeHighCardinalityIdentifiers(t *testing.T) {
	t.Parallel()

	stats := newRuntimeStats()
	stats.recordConversationWrite(
		store.NetworkConversationMessage{
			MessageID: "msg-high", SessionID: "sess-coder", WorkspaceID: testWorkspaceID,
			Channel: "builders", Surface: store.NetworkSurfaceDirect,
			DirectID: "direct_99401d24bee62651d189e5a561785466", Direction: AuditDirectionReceived,
			PeerFrom: "coder.sess-abc", PeerTo: "reviewer.sess-xyz", Kind: string(KindSay),
			WorkID: "work-high", TraceID: "trace-high", CausationID: "cause-high",
			Body: mustRawJSON(t, SayBody{Text: "safe"}), Timestamp: time.Now().UTC(),
		},
		store.NetworkConversationWriteResult{
			MessageID: "msg-high", ConversationOpened: true, WorkOpened: true,
			WorkState: store.NetworkWorkStateWorking,
		},
	)
	snapshot := stats.snapshot()
	if snapshot.OpenDirectRooms != 1 || snapshot.OpenWorkItems != 1 || snapshot.DirectResolves != 1 {
		t.Fatalf("metric counters = %#v, want direct room, work item, and direct resolve", snapshot)
	}
	names := make(map[string]bool, len(snapshot.Metrics))
	for _, sample := range snapshot.Metrics {
		names[sample.Name] = true
		for _, forbidden := range []string{
			"thread_id", "direct_id", "work_id", "message_id", "trace_id", "causation_id",
		} {
			if _, ok := sample.Labels[forbidden]; ok {
				t.Fatalf("%s labels include high-cardinality %q: %#v", sample.Name, forbidden, sample.Labels)
			}
		}
	}
	for _, want := range []string{
		"network_conversation_messages_total",
		"network_direct_rooms_open_total",
		"network_work_open_total",
		"network_open_work_items",
		"network_direct_resolve_total",
	} {
		if !names[want] {
			t.Fatalf("metric names = %#v, want %q", names, want)
		}
	}
}
