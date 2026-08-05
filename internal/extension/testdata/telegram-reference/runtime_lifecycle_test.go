package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/bridgesdk"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
)

func TestTelegramReferenceShutdownCancellation(t *testing.T) {
	// not parallel: setAdapterTestEnv mutates process environment for marker paths.
	t.Run("Should cancel blocked ingest host call before shutdown drain expires", func(t *testing.T) {
		env := setAdapterTestEnv(t)
		runtime, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()

		now := time.Date(2026, 5, 16, 23, 35, 0, 0, time.UTC)
		managed := telegramRuntimeManagedInstance(now, "brg-telegram-reference")
		handleTelegramRuntimeLifecycle(t, hostPeer, managed)

		started := make(chan struct{})
		done := make(chan error, 1)
		release := make(chan struct{})
		var startedOnce sync.Once
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(release) })
		})
		mustHandle(
			t,
			hostPeer,
			string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
			func(ctx context.Context, params json.RawMessage) (any, error) {
				var envelope bridgepkg.InboundMessageEnvelope
				if err := json.Unmarshal(params, &envelope); err != nil {
					return nil, err
				}
				startedOnce.Do(func() { close(started) })
				select {
				case <-ctx.Done():
					done <- ctx.Err()
					return nil, ctx.Err()
				case <-release:
					return telegramRuntimeIngestResult(envelope), nil
				}
			},
		)

		initializeTelegramRuntimeRuntime(t, hostPeer, now, managed)
		if err := appendJSONLineForTest(
			env.updatesPath,
			telegramRuntimeUpdate(now, managed.Instance.ID, 9001, "blocked ingest"),
		); err != nil {
			t.Fatalf("appendJSONLineForTest(update) error = %v", err)
		}
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("ingest host call did not start before timeout")
		}

		shutdownDeadline := 2 * time.Second
		maxCancellationLatency := shutdownDeadline / 2
		startedAt := time.Now()
		if err := runtime.lifecycle.Shutdown(
			context.Background(),
			nil,
			subprocess.ShutdownRequest{DeadlineMS: int64(shutdownDeadline / time.Millisecond)},
		); err != nil {
			t.Fatalf("ProviderLifecycle.Shutdown() error = %v", err)
		}
		if elapsed := time.Since(startedAt); elapsed > maxCancellationLatency {
			releaseOnce.Do(func() { close(release) })
			t.Fatalf(
				"ProviderLifecycle.Shutdown() took %s, want cancellation before %s drain deadline",
				elapsed,
				shutdownDeadline,
			)
		}
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("ingest context error = %v, want context.Canceled", err)
			}
		default:
		}
	})
}

func TestTelegramReferenceAuthStatus(t *testing.T) {
	t.Run("Should classify whitespace-only cached bot token as auth required", func(t *testing.T) {
		now := time.Date(2026, 5, 16, 23, 40, 0, 0, time.UTC)
		managed := telegramRuntimeManagedInstance(now, "brg-telegram-reference")
		managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
			{BindingName: "bot_token", Kind: "token", Value: "   "},
		}
		if got, want := bridgeStatusForManaged(managed), bridgepkg.BridgeStatusAuthRequired; got != want {
			t.Fatalf("bridgeStatusForManaged() = %q, want %q", got, want)
		}
	})
}

func TestTelegramReferenceMalformedUpdateProgress(t *testing.T) {
	// not parallel: setAdapterTestEnv mutates process environment for marker paths.
	t.Run("Should skip malformed update line and ingest later valid update", func(t *testing.T) {
		env := setAdapterTestEnv(t)
		_, hostPeer, cleanup := newRuntimePeerPair(t)
		defer cleanup()

		now := time.Date(2026, 5, 16, 23, 45, 0, 0, time.UTC)
		managed := telegramRuntimeManagedInstance(now, "brg-telegram-reference")
		handleTelegramRuntimeLifecycle(t, hostPeer, managed)
		ingested := recordTelegramRuntimeIngests(t, hostPeer)

		validLine, err := json.Marshal(telegramRuntimeUpdate(
			now,
			managed.Instance.ID,
			9002,
			"valid after malformed",
		))
		if err != nil {
			t.Fatalf("json.Marshal(valid update) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(env.updatesPath), 0o700); err != nil {
			t.Fatalf("os.MkdirAll(updates dir) error = %v", err)
		}
		payload := append([]byte("{malformed\n"), append(validLine, '\n')...)
		if err := os.WriteFile(env.updatesPath, payload, 0o600); err != nil {
			t.Fatalf("os.WriteFile(updates) error = %v", err)
		}

		initializeTelegramRuntimeRuntime(t, hostPeer, now, managed)
		waitForTelegramRuntimeCondition(t, func() bool {
			items := ingested()
			return len(items) > 0 &&
				items[len(items)-1].Content.Text == "valid after malformed"
		})
	})
}

func telegramRuntimeManagedInstance(
	now time.Time,
	instanceID string,
) subprocess.InitializeBridgeManagedInstance {
	managed := testBridgeRuntime(now, instanceID)
	managed.BoundSecrets = []subprocess.InitializeBridgeBoundSecret{
		{BindingName: "bot_token", Kind: "token", Value: "telegram-bot-token"},
	}
	return managed
}

func handleTelegramRuntimeLifecycle(
	t *testing.T,
	hostPeer *bridgesdk.Peer,
	managed subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesList),
		func(context.Context, json.RawMessage) (any, error) {
			return []bridgepkg.BridgeInstance{managed.Instance}, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesGet),
		func(context.Context, json.RawMessage) (any, error) {
			return managed.Instance, nil
		},
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesInstancesReportState),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var payload bridgepkg.BridgesInstancesReportStateParams
			if err := json.Unmarshal(params, &payload); err != nil {
				return nil, err
			}
			instance := managed.Instance
			instance.Status = payload.Status
			return instance, nil
		},
	)
}

func recordTelegramRuntimeIngests(
	t *testing.T,
	hostPeer *bridgesdk.Peer,
) func() []bridgepkg.InboundMessageEnvelope {
	t.Helper()

	var (
		mu       sync.Mutex
		ingested []bridgepkg.InboundMessageEnvelope
	)
	mustHandle(
		t,
		hostPeer,
		string(extensionprotocol.HostAPIMethodBridgesMessagesIngest),
		func(_ context.Context, params json.RawMessage) (any, error) {
			var envelope bridgepkg.InboundMessageEnvelope
			if err := json.Unmarshal(params, &envelope); err != nil {
				return nil, err
			}
			mu.Lock()
			ingested = append(ingested, envelope)
			mu.Unlock()
			return telegramRuntimeIngestResult(envelope), nil
		},
	)

	return func() []bridgepkg.InboundMessageEnvelope {
		mu.Lock()
		defer mu.Unlock()
		cloned := make([]bridgepkg.InboundMessageEnvelope, len(ingested))
		copy(cloned, ingested)
		return cloned
	}
}

func telegramRuntimeIngestResult(
	envelope bridgepkg.InboundMessageEnvelope,
) bridgepkg.BridgesMessagesIngestResult {
	return bridgepkg.BridgesMessagesIngestResult{
		SessionID:    "sess-" + envelope.BridgeInstanceID,
		RouteCreated: true,
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            envelope.Scope,
			WorkspaceID:      envelope.WorkspaceID,
			BridgeInstanceID: envelope.BridgeInstanceID,
			PeerID:           envelope.PeerID,
			ThreadID:         envelope.ThreadID,
		},
	}
}

func initializeTelegramRuntimeRuntime(
	t *testing.T,
	hostPeer *bridgesdk.Peer,
	now time.Time,
	managed subprocess.InitializeBridgeManagedInstance,
) {
	t.Helper()

	if err := hostPeer.Call(
		context.Background(),
		"initialize",
		testInitializeRequest(now, managed),
		nil,
	); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}
}

func telegramRuntimeUpdate(
	now time.Time,
	instanceID string,
	updateID int64,
	text string,
) telegramUpdate {
	return telegramUpdate{
		BridgeInstanceID: instanceID,
		UpdateID:         updateID,
		Message: &telegramMessage{
			MessageID: updateID + 100,
			Date:      now.Unix(),
			Chat:      telegramChat{ID: 777},
			From:      telegramUser{ID: 888, Username: "alice"},
			Text:      text,
		},
	}
}

func waitForTelegramRuntimeCondition(t *testing.T, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition did not succeed before timeout")
}
