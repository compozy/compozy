package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func TestReconcileDiscordInstanceConfigsZeroManaged(t *testing.T) {
	t.Run("Should close existing batchers when ownership drops to zero managed instances", func(t *testing.T) {
		t.Parallel()

		provider, err := newDiscordProvider(io.Discard)
		if err != nil {
			t.Fatalf("newDiscordProvider() error = %v", err)
		}
		t.Cleanup(provider.lifecycle.Stop)

		batcher, err := bridgesdk.NewInboundBatcher(bridgesdk.InboundBatcherConfig{
			Context: context.Background(),
			Delay:   time.Hour,
			Dispatch: func(context.Context, bridgesdk.InboundBatch) error {
				return nil
			},
			Now: func() time.Time {
				return time.Date(2026, 5, 16, 20, 30, 0, 0, time.UTC)
			},
		})
		if err != nil {
			t.Fatalf("NewInboundBatcher() error = %v", err)
		}
		t.Cleanup(batcher.Close)

		provider.routes.Replace(map[string]resolvedInstanceConfig{
			"brg-discord": {
				instanceID: "brg-discord",
				managed:    testDiscordManagedInstance("brg-discord"),
				batcher:    batcher,
			},
		}, nil)

		configs := provider.reconcileInstanceConfigs(context.Background(), nil, nil)
		if configs != nil {
			t.Fatalf("reconcileInstanceConfigs() = %#v, want nil", configs)
		}

		routeCount := len(provider.routes.Snapshot())
		if routeCount != 0 {
			t.Fatalf("len(provider.routes) = %d, want 0", routeCount)
		}

		err = batcher.Enqueue(testDiscordInboundEnvelope("brg-discord"))
		if err == nil || !strings.Contains(err.Error(), "closed") {
			t.Fatalf("batcher.Enqueue() error = %v, want closed batcher error", err)
		}
	})
}

func testDiscordInboundEnvelope(instanceID string) bridgepkg.InboundMessageEnvelope {
	return bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  instanceID,
		Scope:             bridgepkg.ScopeWorkspace,
		WorkspaceID:       "ws-1",
		GroupID:           "channel-1",
		ThreadID:          "thread-1",
		PlatformMessageID: "msg-1",
		ReceivedAt:        time.Date(2026, 5, 16, 20, 31, 0, 0, time.UTC),
		Sender:            bridgepkg.MessageSender{ID: "user-1"},
		Content:           bridgepkg.MessageContent{Text: "hello"},
		EventFamily:       bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey:    "discord:brg-discord:msg-1",
	}
}
