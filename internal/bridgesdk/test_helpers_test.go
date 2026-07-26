package bridgesdk

import (
	"context"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
	"github.com/compozy/agh/internal/subprocess"
)

func testCheckHandler(
	context.Context,
	*Session,
	bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	return bridgepkg.BridgeCheckResponse{
		Checks: []bridgepkg.BridgeCheckRecord{bridgepkg.PassCheck("provider.identity")},
	}, nil
}

func testBridgeInstance(id string) bridgepkg.BridgeInstance {
	return bridgepkg.BridgeInstance{
		ID:            id,
		Scope:         bridgepkg.ScopeWorkspace,
		WorkspaceID:   "ws-1",
		Platform:      "telegram",
		ExtensionName: "telegram-adapter",
		DisplayName:   "Telegram Adapter",
		Source:        bridgepkg.BridgeInstanceSourceDynamic,
		Enabled:       true,
		Status:        bridgepkg.BridgeStatusReady,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
		CreatedAt:     time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
	}
}

func testManagedRuntime(instanceIDs ...string) *subprocess.InitializeBridgeRuntime {
	managed := make([]subprocess.InitializeBridgeManagedInstance, 0, len(instanceIDs))
	for _, instanceID := range instanceIDs {
		managed = append(managed, subprocess.InitializeBridgeManagedInstance{
			Instance: testBridgeInstance(instanceID),
			BoundSecrets: []subprocess.InitializeBridgeBoundSecret{{
				BindingName: "bot_token",
				Kind:        "token",
				Value:       "secret-" + instanceID,
			}},
		})
	}
	return &subprocess.InitializeBridgeRuntime{
		RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:          subprocess.BridgeRuntimePurposeService,
		Provider:         "telegram-adapter",
		Platform:         "telegram",
		ManagedInstances: managed,
	}
}

func testInitializeRequest() subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "test",
		SessionNonce:             "nonce-test",
		Extension: subprocess.InitializeExtension{
			Name:       "telegram-adapter",
			Version:    "1.0.0",
			SourceTier: "workspace",
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides: []string{extensionprotocol.CapabilityProvideBridgeAdapter},
			GrantedActions: []extensionprotocol.HostAPIMethod{
				extensionprotocol.HostAPIMethodBridgesInstancesList,
				extensionprotocol.HostAPIMethodBridgesInstancesGet,
				extensionprotocol.HostAPIMethodBridgesInstancesReportState,
				extensionprotocol.HostAPIMethodBridgesMessagesIngest,
			},
		},
		Methods: subprocess.InitializeMethods{
			ExtensionServices: []string{string(extensionprotocol.ExtensionServiceMethodBridgesDeliver)},
		},
		Runtime: subprocess.InitializeRuntime{
			HealthCheckIntervalMS: 5000,
			HealthCheckTimeoutMS:  1000,
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  1000,
			Bridge:                testManagedRuntime("brg-1"),
		},
	}
}

func testInboundEnvelope(
	idempotencyKey string,
	platformMessageID string,
	text string,
) bridgepkg.InboundMessageEnvelope {
	return bridgepkg.InboundMessageEnvelope{
		BridgeInstanceID:  "brg-1",
		Scope:             bridgepkg.ScopeWorkspace,
		WorkspaceID:       "ws-1",
		PeerID:            "peer-1",
		ThreadID:          "thread-1",
		PlatformMessageID: platformMessageID,
		ReceivedAt:        time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC),
		Sender: bridgepkg.MessageSender{
			ID: "sender-1",
		},
		Content: bridgepkg.MessageContent{
			Text: text,
		},
		EventFamily:    bridgepkg.InboundEventFamilyMessage,
		IdempotencyKey: idempotencyKey,
	}
}
