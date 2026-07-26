package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func TestGitHubRuntimeProgressDeliveryAcknowledgesWithoutPlatformSideEffects(t *testing.T) {
	t.Run("Should ack progress without creating a GitHub API client", func(t *testing.T) {
		t.Parallel()

		provider, hostPeer, cleanup := newGitHubRuntimePeerPair(t)
		defer cleanup()
		var apiFactoryCalls atomic.Int32
		provider.apiFactory = func(resolvedInstanceConfig) githubAPI {
			apiFactoryCalls.Add(1)
			return &fakeGitHubAPI{}
		}
		if err := hostPeer.Handle(
			string(extensionprotocol.HostAPIMethodBridgesInstancesList),
			func(context.Context, json.RawMessage) (any, error) {
				return []bridgepkg.BridgeInstance{}, nil
			},
		); err != nil {
			t.Fatalf("hostPeer.Handle(instances list) error = %v", err)
		}
		if err := hostPeer.Call(
			context.Background(),
			"initialize",
			githubRuntimeInitializeRequest(githubProgressManagedInstance()),
			nil,
		); err != nil {
			t.Fatalf("hostPeer.Call(initialize) error = %v", err)
		}

		request := githubProgressNoopRequest()
		var ack bridgepkg.DeliveryAck
		if err := hostPeer.Call(context.Background(), "bridges/deliver", request, &ack); err != nil {
			t.Fatalf("hostPeer.Call(progress) error = %v", err)
		}
		if got, want := ack.DeliveryID, request.Event.DeliveryID; got != want {
			t.Fatalf("ack delivery id = %q, want %q", got, want)
		}
		if got, want := ack.Seq, request.Event.Seq; got != want {
			t.Fatalf("ack seq = %d, want %d", got, want)
		}
		if ack.RemoteMessageID != "" || ack.ReplaceRemoteMessageID != "" {
			t.Fatalf("progress ack = %#v, want no platform handles", ack)
		}
		if got := apiFactoryCalls.Load(); got != 0 {
			t.Fatalf("apiFactory calls = %d, want 0", got)
		}
	})
}

func newGitHubRuntimePeerPair(t *testing.T) (*githubProvider, *bridgesdk.Peer, func()) {
	t.Helper()

	hostConn, runtimeConn := net.Pipe()
	provider, err := newGitHubProvider(io.Discard)
	if err != nil {
		t.Fatalf("newGitHubProvider() error = %v", err)
	}
	hostPeer := bridgesdk.NewPeer(hostConn, hostConn)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 2)
	go func() { errCh <- provider.serve(runtimeConn, runtimeConn) }()
	go func() { errCh <- hostPeer.Serve(ctx) }()

	cleanup := func() {
		cancel()
		provider.lifecycle.Stop()
		if err := hostConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("hostConn.Close() error = %v", err)
		}
		if err := runtimeConn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("runtimeConn.Close() error = %v", err)
		}
		for range 2 {
			err := <-errCh
			if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, net.ErrClosed) {
				continue
			}
			if strings.Contains(err.Error(), "closed") {
				continue
			}
			t.Errorf("runtime peer serve error = %v", err)
		}
		waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
		defer waitCancel()
		if err := provider.lifecycle.Wait(waitCtx); err != nil {
			t.Errorf("provider lifecycle wait error = %v", err)
		}
	}
	return provider, hostPeer, cleanup
}

func githubRuntimeInitializeRequest(
	managed ...subprocess.InitializeBridgeManagedInstance,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "0.5.0",
		SessionNonce:             "nonce-github-progress-noop",
		Extension: subprocess.InitializeExtension{
			Name:       "github",
			Version:    "0.1.0",
			SourceTier: "user",
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{"bridge.adapter"},
			GrantedActions: []extensionprotocol.HostAPIMethod{
				extensionprotocol.HostAPIMethodBridgesInstancesList,
			},
			GrantedSecurity: []string{"bridge.read", "bridge.write"},
		},
		Methods: subprocess.InitializeMethods{
			ExtensionServices: []string{"bridges/deliver", "health_check", "shutdown"},
		},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 30_000,
			HealthCheckTimeoutMS:  5_000,
			ShutdownTimeoutMS:     5_000,
			DefaultHookTimeoutMS:  5_000,
			Bridge: &subprocess.InitializeBridgeRuntime{
				RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
				Purpose:          subprocess.BridgeRuntimePurposeService,
				Provider:         "github",
				Platform:         "github",
				ManagedInstances: managed,
			},
		},
	}
}

func githubProgressManagedInstance() subprocess.InitializeBridgeManagedInstance {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	return subprocess.InitializeBridgeManagedInstance{Instance: bridgepkg.BridgeInstance{
		ID:            "brg-github-progress-noop",
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   "ws-github",
		Platform:      "github",
		ExtensionName: "github",
		DisplayName:   "GitHub",
		Source:        bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true},
		CreatedAt:     now,
		UpdatedAt:     now,
	}}
}

func githubProgressNoopRequest() bridgepkg.DeliveryRequest {
	return bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
		DeliveryID:       "delivery-github-progress-noop",
		BridgeInstanceID: "brg-github-progress-noop",
		RoutingKey: bridgepkg.RoutingKey{
			Scope:            bridgepkg.ScopeWorkspace,
			WorkspaceID:      "ws-github",
			BridgeInstanceID: "brg-github-progress-noop",
			GroupID:          "acme/app",
			ThreadID:         "github:acme/app:42",
		},
		DeliveryTarget: bridgepkg.DeliveryTarget{
			BridgeInstanceID: "brg-github-progress-noop",
			GroupID:          "acme/app",
			ThreadID:         "github:acme/app:42",
			Mode:             bridgepkg.DeliveryModeReply,
		},
		Seq:       1,
		EventType: bridgepkg.DeliveryEventTypeProgress,
		Operation: bridgepkg.DeliveryOperationPost,
		Progress: &bridgepkg.ToolProgress{
			ToolCallID: "tool-call-github-noop",
			ToolID:     "agh__search",
			Phase:      bridgepkg.ToolProgressPhaseStarted,
			Label:      "Search",
			Index:      1,
		},
	}}
}
