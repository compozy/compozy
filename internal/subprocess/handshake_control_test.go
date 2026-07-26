// Suite: bridge control initialize handshake
// Invariant: a control subprocess gets one instance, one typed method, and zero Host API grants.
// Boundary IN: AGH-to-extension initialize negotiation.
// Boundary OUT: transient process cleanup, owned by extension manager tests.
package subprocess

import (
	"strings"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

func TestInitializeBridgeRuntimeV2PurposeContract(t *testing.T) {
	t.Parallel()

	managed := controlManagedBridgeInstance(t, "brg-control")

	t.Run("Should accept a service runtime with no control methods", func(t *testing.T) {
		t.Parallel()

		runtime := InitializeBridgeRuntime{
			RuntimeVersion:   InitializeBridgeRuntimeVersion2,
			Purpose:          BridgeRuntimePurposeService,
			Provider:         "telegram-adapter",
			Platform:         "telegram",
			ManagedInstances: []InitializeBridgeManagedInstance{managed},
		}
		if err := runtime.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("Should require the v2 runtime version", func(t *testing.T) {
		t.Parallel()

		runtime := validControlBridgeRuntime(managed, bridgepkg.ControlMethodCheck)
		runtime.RuntimeVersion = "1"
		err := runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "version") {
			t.Fatalf("Validate() error = %v, want v2 error", err)
		}
	})

	t.Run("Should require a closed purpose", func(t *testing.T) {
		t.Parallel()

		runtime := validControlBridgeRuntime(managed, bridgepkg.ControlMethodCheck)
		runtime.Purpose = ""
		err := runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "purpose") {
			t.Fatalf("Validate() error = %v, want purpose error", err)
		}
	})

	t.Run("Should reject a noncanonical control purpose before applying grants", func(t *testing.T) {
		t.Parallel()

		request := controlInitializeRequest(managed, bridgepkg.ControlMethodCheck)
		request.Runtime.Bridge.Purpose = BridgeRuntimePurpose(" control ")
		request.Capabilities.GrantedActions = []extensionprotocol.HostAPIMethod{
			extensionprotocol.HostAPIMethodBridgesInstancesGet,
		}
		err := request.Validate()
		if err == nil || !strings.Contains(err.Error(), "must be canonical") {
			t.Fatalf("Validate() error = %v, want canonical-purpose rejection", err)
		}
	})

	t.Run("Should require exactly one control instance", func(t *testing.T) {
		t.Parallel()

		runtime := validControlBridgeRuntime(managed, bridgepkg.ControlMethodCheck)
		runtime.ManagedInstances = append(runtime.ManagedInstances, controlManagedBridgeInstance(t, "brg-other"))
		err := runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("Validate() error = %v, want singleton error", err)
		}
	})

	t.Run("Should require exactly one known control method", func(t *testing.T) {
		t.Parallel()

		runtime := validControlBridgeRuntime(managed, bridgepkg.ControlMethodCheck)
		runtime.AllowedMethods = []string{
			string(bridgepkg.ControlMethodCheck),
			string(bridgepkg.ControlMethodWebhookRegister),
		}
		err := runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("Validate() error = %v, want one method error", err)
		}

		runtime.AllowedMethods = []string{"bridges/raw-action"}
		err = runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("Validate() error = %v, want unsupported method error", err)
		}
	})

	t.Run("Should reject control methods on a service runtime", func(t *testing.T) {
		t.Parallel()

		runtime := InitializeBridgeRuntime{
			RuntimeVersion:   InitializeBridgeRuntimeVersion2,
			Purpose:          BridgeRuntimePurposeService,
			Provider:         "telegram-adapter",
			Platform:         "telegram",
			AllowedMethods:   []string{string(bridgepkg.ControlMethodCheck)},
			ManagedInstances: []InitializeBridgeManagedInstance{managed},
		}
		err := runtime.Validate()
		if err == nil || !strings.Contains(err.Error(), "service") {
			t.Fatalf("Validate() error = %v, want service method error", err)
		}
	})
}

func TestInitializeRequestControlRuntimeRejectsHostAPIGrants(t *testing.T) {
	t.Parallel()

	managed := controlManagedBridgeInstance(t, "brg-control")
	request := controlInitializeRequest(managed, bridgepkg.ControlMethodCheck)

	t.Run("Should accept zero grants and the exact allowed method", func(t *testing.T) {
		t.Parallel()

		if err := request.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("Should reject granted actions", func(t *testing.T) {
		t.Parallel()

		invalid := CloneInitializeRequest(request)
		invalid.Capabilities.GrantedActions = []extensionprotocol.HostAPIMethod{
			extensionprotocol.HostAPIMethodBridgesInstancesGet,
		}
		err := invalid.Validate()
		if err == nil || !strings.Contains(err.Error(), "zero Host API") {
			t.Fatalf("Validate() error = %v, want zero grant error", err)
		}
	})

	t.Run("Should reject daemon methods beyond shutdown", func(t *testing.T) {
		t.Parallel()

		invalid := CloneInitializeRequest(request)
		invalid.Methods.DaemonRequests = []string{"shutdown", "health_check"}
		err := invalid.Validate()
		if err == nil || !strings.Contains(err.Error(), "only shutdown") {
			t.Fatalf("Validate() error = %v, want shutdown-only error", err)
		}
	})

	t.Run("Should reject an extension method outside the exact allowlist", func(t *testing.T) {
		t.Parallel()

		invalid := CloneInitializeRequest(request)
		invalid.Methods.ExtensionServices = []string{string(bridgepkg.ControlMethodWebhookRegister)}
		err := invalid.Validate()
		if err == nil || !strings.Contains(err.Error(), "allowed method") {
			t.Fatalf("Validate() error = %v, want exact method error", err)
		}
	})
}

func TestControlInitializeResponseRemainsCapabilityFree(t *testing.T) {
	t.Parallel()

	request := controlInitializeRequest(
		controlManagedBridgeInstance(t, "brg-control-response"),
		bridgepkg.ControlMethodCheck,
	)
	valid := InitializeResponse{
		ProtocolVersion:    "1",
		ImplementedMethods: []string{"shutdown", string(bridgepkg.ControlMethodCheck)},
	}

	t.Run("Should accept only shutdown and the requested control method", func(t *testing.T) {
		t.Parallel()

		if err := validateInitializeResponse(request, valid); err != nil {
			t.Fatalf("validateInitializeResponse() error = %v", err)
		}
		if got := BridgeRuntimePurposeValues(); len(got) != 2 ||
			got[0] != string(BridgeRuntimePurposeService) || got[1] != string(BridgeRuntimePurposeControl) {
			t.Fatalf("BridgeRuntimePurposeValues() = %#v", got)
		}
	})

	t.Run("Should reject accepted capabilities", func(t *testing.T) {
		t.Parallel()

		response := valid
		response.AcceptedCapabilities.Provides = []string{"bridge.adapter"}
		err := validateInitializeResponse(request, response)
		if err == nil || !strings.Contains(err.Error(), "capabilities") {
			t.Fatalf("validateInitializeResponse() error = %v, want capability rejection", err)
		}
	})

	t.Run("Should reject extra methods and service-only affordances", func(t *testing.T) {
		t.Parallel()

		response := valid
		response.ImplementedMethods = append(response.ImplementedMethods, "health_check")
		if err := validateInitializeResponse(request, response); err == nil ||
			!strings.Contains(err.Error(), "exact methods") {
			t.Fatalf("validateInitializeResponse(extra method) error = %v", err)
		}

		response = valid
		response.Supports.HealthCheck = true
		if err := validateInitializeResponse(request, response); err == nil ||
			!strings.Contains(err.Error(), "health checks") {
			t.Fatalf("validateInitializeResponse(health) error = %v", err)
		}

		response = valid
		response.SupportedHookEvents = []string{"session.started"}
		if err := validateInitializeResponse(request, response); err == nil ||
			!strings.Contains(err.Error(), "hooks") {
			t.Fatalf("validateInitializeResponse(hooks) error = %v", err)
		}
	})
}

func validControlBridgeRuntime(
	managed InitializeBridgeManagedInstance,
	method bridgepkg.ControlMethod,
) InitializeBridgeRuntime {
	return InitializeBridgeRuntime{
		RuntimeVersion:   InitializeBridgeRuntimeVersion2,
		Purpose:          BridgeRuntimePurposeControl,
		Provider:         "telegram-adapter",
		Platform:         "telegram",
		AllowedMethods:   []string{string(method)},
		ManagedInstances: []InitializeBridgeManagedInstance{managed},
	}
}

func controlInitializeRequest(
	managed InitializeBridgeManagedInstance,
	method bridgepkg.ControlMethod,
) InitializeRequest {
	return InitializeRequest{
		ProtocolVersion:          "1",
		SupportedProtocolVersion: []string{"1"},
		AGHVersion:               "test",
		SessionNonce:             "nonce-control",
		Extension: InitializeExtension{
			Name:       "telegram-adapter",
			Version:    "1.0.0",
			SourceTier: "bundled",
		},
		Methods: InitializeMethods{
			DaemonRequests:    []string{"shutdown"},
			ExtensionServices: []string{string(method)},
		},
		Runtime: InitializeRuntime{
			HealthCheckIntervalMS: 1000,
			HealthCheckTimeoutMS:  1000,
			ShutdownTimeoutMS:     1000,
			DefaultHookTimeoutMS:  1000,
			Bridge: func() *InitializeBridgeRuntime {
				runtime := validControlBridgeRuntime(managed, method)
				return &runtime
			}(),
		},
	}
}

func controlManagedBridgeInstance(t *testing.T, id string) InitializeBridgeManagedInstance {
	t.Helper()

	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	return InitializeBridgeManagedInstance{
		Instance: bridgepkg.BridgeInstance{
			ID:            id,
			Scope:         bridgepkg.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-adapter",
			DisplayName:   "Telegram",
			Enabled:       false,
			Status:        bridgepkg.BridgeStatusDisabled,
			RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		BoundSecrets: []InitializeBridgeBoundSecret{{
			BindingName: "bot_token",
			Kind:        "token",
			Value:       "test-secret",
		}},
	}
}
