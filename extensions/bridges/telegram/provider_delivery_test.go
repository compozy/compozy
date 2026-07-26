package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestTelegramProviderContracts(t *testing.T) {
	t.Run("Should exclude invalid webhook path configs from routing", func(t *testing.T) {
		t.Parallel()

		provider, err := newTelegramProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTelegramProvider() error = %v", err)
		}
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-1": {
				instanceID:  "brg-1",
				webhookPath: "/telegram/shared",
				configError: errors.New("duplicate webhook path"),
			},
		}, nil)

		if cfg, ok := provider.configForPath("/telegram/shared"); ok {
			t.Fatalf("configForPath() = (%#v, true), want invalid config excluded", cfg)
		}
	})

	t.Run("Should evict terminal delivery state after final acknowledgement", func(t *testing.T) {
		t.Parallel()

		provider, err := newTelegramProvider(io.Discard)
		if err != nil {
			t.Fatalf("newTelegramProvider() error = %v", err)
		}
		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-1": {instanceID: "brg-1"},
		}, nil)
		provider.apiFactory = func(*resolvedInstanceConfig) telegramAPI {
			return &fakeTelegramAPI{nextMessageID: 900}
		}

		startReq := testDeliveryRequest(
			"brg-1",
			"delivery-1",
			1,
			bridgepkg.DeliveryEventTypeStart,
			false,
		)
		startAck, err := provider.handleBridgesDeliver(context.Background(), nil, startReq)
		if err != nil {
			t.Fatalf("handleBridgesDeliver(start) error = %v", err)
		}
		if got, want := provider.deliveryState(
			"brg-1",
			"delivery-1",
		).RemoteMessageID, startAck.RemoteMessageID; got != want {
			t.Fatalf("deliveryState(start).RemoteMessageID = %q, want %q", got, want)
		}

		finalReq := testDeliveryRequest(
			"brg-1",
			"delivery-1",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		finalReq.Event.Content.Text = "final message"
		if _, err := provider.handleBridgesDeliver(context.Background(), nil, finalReq); err != nil {
			t.Fatalf("handleBridgesDeliver(final) error = %v", err)
		}
		if got := provider.deliveryState("brg-1", "delivery-1"); got != (deliveryState{}) {
			t.Fatalf("deliveryState(final) = %#v, want empty after terminal ack", got)
		}
	})

	t.Run(
		"Should render a full progress turn and post the final answer separately in the same topic",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
			api := &fakeTelegramAPI{nextMessageID: 950}
			provider := newTelegramProgressTestProvider(t, func() time.Time { return now }, api)
			session := &bridgesdk.Session{}
			phases := []struct {
				phase bridgepkg.ToolProgressPhase
				label string
			}{
				{phase: bridgepkg.ToolProgressPhaseStarted, label: "Inspecting"},
				{phase: bridgepkg.ToolProgressPhaseCompleted, label: "Inspecting"},
				{phase: bridgepkg.ToolProgressPhaseStarted, label: "Summarizing"},
			}
			for index, phase := range phases {
				request := testProgressRequest(
					"brg-progress",
					"delivery-full-turn",
					int64(index+1),
					phase.phase,
					phase.label,
				)
				request.Event.RoutingKey.ThreadID = "42"
				request.Event.DeliveryTarget.ThreadID = "42"
				if phase.phase == bridgepkg.ToolProgressPhaseCompleted {
					request.Event.Progress.DurationMS = 31_000
				}
				if _, err := provider.handleBridgesProgress(context.Background(), session, request); err != nil {
					t.Fatalf("handleBridgesProgress(%d) error = %v", index, err)
				}
				now = now.Add(2 * time.Second)
			}

			final := testDeliveryRequest(
				"brg-progress",
				"delivery-full-turn",
				4,
				bridgepkg.DeliveryEventTypeFinal,
				true,
			)
			final.Event.Content.Text = "Final **answer**"
			final.Event.RoutingKey.ThreadID = "42"
			final.Event.DeliveryTarget.ThreadID = "42"
			if _, err := provider.handleBridgesDeliver(context.Background(), session, final); err != nil {
				t.Fatalf("handleBridgesDeliver(final) error = %v", err)
			}

			if got, want := len(api.sendRequests), 2; got != want {
				t.Fatalf("send calls = %d, want progress bubble plus final answer (%d)", got, want)
			}
			if got, want := len(api.editRequests), 2; got != want {
				t.Fatalf("progress edits = %d, want %d", got, want)
			}
			if got, want := api.sendRequests[0].MessageThreadID, int64(42); got != want {
				t.Fatalf("progress message_thread_id = %d, want %d", got, want)
			}
			if got, want := api.sendRequests[1].MessageThreadID, int64(42); got != want {
				t.Fatalf("final message_thread_id = %d, want %d", got, want)
			}
			if got, want := api.sendRequests[1].Text, "Final *answer*"; got != want {
				t.Fatalf("final text = %q, want %q", got, want)
			}
			if got := api.editRequests[1].Text; !strings.Contains(got, "\u2705 Inspecting \u00b7 31s") ||
				!strings.Contains(got, "\U0001f527 Summarizing") {
				t.Fatalf("final progress bubble = %q, want completed duration and next tool", got)
			}
			if got, want := len(api.chatActions), 2; got != want {
				t.Fatalf("typing calls = %d, want one per started tool (%d)", got, want)
			}
			if state := provider.deliveryState("brg-progress", "delivery-full-turn"); state != (deliveryState{}) {
				t.Fatalf("terminal delivery state = %#v, want evicted state", state)
			}
		},
	)
}
