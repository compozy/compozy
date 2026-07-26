package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRuntimeServeInitializeDeliverHealthShutdownAndSync(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	hostConn, runtimeConn := net.Pipe()
	defer func() {
		_ = hostConn.Close()
	}()
	defer func() {
		_ = runtimeConn.Close()
	}()

	shutdownCalled := false
	runtime, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:       "telegram-adapter",
			Version:    "1.0.0",
			SDKName:    "bridgesdk",
			SDKVersion: "test",
		},
		Check: testCheckHandler,
		Initialize: func(_ context.Context, session *Session) error {
			if session.BridgeRuntime() == nil {
				t.Fatal("session.BridgeRuntime() = nil, want non-nil")
			}
			if session.Cache() == nil {
				t.Fatal("session.Cache() = nil, want non-nil")
			}
			return nil
		},
		Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "remote-1", "")
		},
		HealthCheck: func(context.Context, *Session) error { return nil },
		Shutdown: func(_ context.Context, _ *Session, _ subprocess.ShutdownRequest) error {
			shutdownCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	hostPeer := NewPeer(hostConn, hostConn)
	if err := hostPeer.Handle("bridges/instances/list", func(context.Context, json.RawMessage) (any, error) {
		instance := testBridgeInstance("brg-1")
		instance.Status = bridgepkg.BridgeStatusDegraded
		return []bridgepkg.BridgeInstance{instance}, nil
	}); err != nil {
		t.Fatalf("hostPeer.Handle(list) error = %v", err)
	}
	if err := hostPeer.Handle("bridges/instances/get", func(context.Context, json.RawMessage) (any, error) {
		instance := testBridgeInstance("brg-1")
		return instance, nil
	}); err != nil {
		t.Fatalf("hostPeer.Handle(get) error = %v", err)
	}
	if err := hostPeer.Handle("bridges/messages/ingest", func(_ context.Context, raw json.RawMessage) (any, error) {
		var envelope bridgepkg.InboundMessageEnvelope
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, err
		}
		return bridgepkg.BridgesMessagesIngestResult{
			SessionID:    "sess-1",
			RouteCreated: true,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            envelope.Scope,
				WorkspaceID:      envelope.WorkspaceID,
				BridgeInstanceID: envelope.BridgeInstanceID,
				PeerID:           envelope.PeerID,
				ThreadID:         envelope.ThreadID,
			},
		}, nil
	}); err != nil {
		t.Fatalf("hostPeer.Handle(ingest) error = %v", err)
	}

	go func() { _ = runtime.Serve(ctx, runtimeConn, runtimeConn) }()
	go func() { _ = hostPeer.Serve(ctx) }()

	var response subprocess.InitializeResponse
	if err := hostPeer.Call(ctx, "initialize", testInitializeRequest(), &response); err != nil {
		t.Fatalf("hostPeer.Call(initialize) error = %v", err)
	}

	session := runtime.Session()
	if session == nil {
		t.Fatal("runtime.Session() = nil, want non-nil")
	}

	items, err := session.SyncInstances(ctx)
	if err != nil {
		t.Fatalf("session.SyncInstances() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("len(session.SyncInstances()) = %d, want %d", got, want)
	}
	if got, want := items[0].Instance.Status, bridgepkg.BridgeStatusDegraded; got != want {
		t.Fatalf("items[0].Instance.Status = %q, want %q", got, want)
	}

	gotInstance, err := session.HostAPI().GetBridgeInstance(ctx, "brg-1")
	if err != nil {
		t.Fatalf("session.HostAPI().GetBridgeInstance() error = %v", err)
	}
	if got, want := gotInstance.ID, "brg-1"; got != want {
		t.Fatalf("GetBridgeInstance().ID = %q, want %q", got, want)
	}

	ingestResult, err := session.HostAPI().IngestBridgeMessage(ctx, testInboundEnvelope("idem-1", "msg-1", "hello"))
	if err != nil {
		t.Fatalf("session.HostAPI().IngestBridgeMessage() error = %v", err)
	}
	if got, want := ingestResult.SessionID, "sess-1"; got != want {
		t.Fatalf("IngestBridgeMessage().SessionID = %q, want %q", got, want)
	}

	var health subprocess.HealthCheckResponse
	if err := hostPeer.Call(ctx, "health_check", nil, &health); err != nil {
		t.Fatalf("hostPeer.Call(health_check) error = %v", err)
	}
	if !health.Healthy {
		t.Fatal("health.Healthy = false, want true")
	}

	var ack bridgepkg.DeliveryAck
	if err := hostPeer.Call(ctx, "bridges/deliver", bridgepkg.DeliveryRequest{
		Event: bridgepkg.DeliveryEvent{
			DeliveryID:       "dlv-1",
			BridgeInstanceID: "brg-1",
			Seq:              1,
			EventType:        bridgepkg.DeliveryEventTypeStart,
			RoutingKey: bridgepkg.RoutingKey{
				Scope:            bridgepkg.ScopeWorkspace,
				WorkspaceID:      "ws-1",
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
			},
			DeliveryTarget: bridgepkg.DeliveryTarget{
				BridgeInstanceID: "brg-1",
				PeerID:           "peer-1",
				Mode:             bridgepkg.DeliveryModeDirectSend,
			},
			Content: bridgepkg.MessageContent{Text: "hello"},
		},
	}, &ack); err != nil {
		t.Fatalf("hostPeer.Call(bridges/deliver) error = %v", err)
	}
	if got, want := ack.RemoteMessageID, "remote-1"; got != want {
		t.Fatalf("ack.RemoteMessageID = %q, want %q", got, want)
	}

	var shutdown subprocess.ShutdownResponse
	if err := hostPeer.Call(ctx, "shutdown", subprocess.ShutdownRequest{
		Reason:     "test",
		DeadlineMS: int64(time.Second / time.Millisecond),
	}, &shutdown); err != nil {
		t.Fatalf("hostPeer.Call(shutdown) error = %v", err)
	}
	if !shutdown.Acknowledged {
		t.Fatal("shutdown.Acknowledged = false, want true")
	}
	if !shutdownCalled {
		t.Fatal("shutdownCalled = false, want true")
	}
}

func TestRuntimeServeRejectsDeliveryBeforeInitialize(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	hostConn, runtimeConn := net.Pipe()
	defer func() {
		_ = hostConn.Close()
	}()
	defer func() {
		_ = runtimeConn.Close()
	}()

	runtime, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    "telegram-adapter",
			Version: "1.0.0",
		},
		Check: testCheckHandler,
		Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "remote-1", "")
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	hostPeer := NewPeer(hostConn, hostConn)
	go func() { _ = runtime.Serve(ctx, runtimeConn, runtimeConn) }()
	go func() { _ = hostPeer.Serve(ctx) }()

	err = hostPeer.Call(ctx, "bridges/deliver", bridgepkg.DeliveryRequest{}, nil)
	var rpcErr *subprocess.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("hostPeer.Call(bridges/deliver) error = %T, want *subprocess.RPCError", err)
	}
	if got, want := rpcErr.Code, bridgeSDKRPCCodeNotInitialized; got != want {
		t.Fatalf("rpcErr.Code = %d, want %d", got, want)
	}
}

func TestRuntimeServeRejectsInvalidInitializePayload(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	hostConn, runtimeConn := net.Pipe()
	defer func() {
		_ = hostConn.Close()
	}()
	defer func() {
		_ = runtimeConn.Close()
	}()

	runtime, err := NewRuntime(RuntimeConfig{
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    "telegram-adapter",
			Version: "1.0.0",
		},
		Check: testCheckHandler,
		Deliver: func(_ context.Context, session *Session, request bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
			return session.AckDelivery(request, "remote-1", "")
		},
	})
	if err != nil {
		t.Fatalf("NewRuntime() error = %v", err)
	}

	hostPeer := NewPeer(hostConn, hostConn)
	go func() { _ = runtime.Serve(ctx, runtimeConn, runtimeConn) }()
	go func() { _ = hostPeer.Serve(ctx) }()

	badRequest := testInitializeRequest()
	badRequest.Runtime.Bridge = nil

	err = hostPeer.Call(ctx, "initialize", badRequest, nil)
	var rpcErr *subprocess.RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("hostPeer.Call(initialize) error = %T, want *subprocess.RPCError", err)
	}
	if got, want := rpcErr.Code, bridgeSDKRPCCodeInvalidParams; got != want {
		t.Fatalf("rpcErr.Code = %d, want %d", got, want)
	}
}
