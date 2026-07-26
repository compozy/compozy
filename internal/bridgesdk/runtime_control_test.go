// Suite: bridge SDK control runtime
// Invariant: control mode exposes one allowed typed method and never runs provider service lifecycle hooks.
// Boundary IN: initialized SDK JSON-RPC handlers.
// Boundary OUT: subprocess launch/reap, owned by extension manager tests.
package bridgesdk

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/subprocess"
)

func TestRuntimeControlModeSkipsServiceLifecycleAndRestrictsMethods(t *testing.T) {
	t.Run("Should expose only the allowed control method without service lifecycle hooks", func(t *testing.T) {
		t.Parallel()

		serviceInitializeCalls := 0
		serviceHealthCalls := 0
		serviceShutdownCalls := 0
		checkCalls := 0
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "telegram-adapter", Version: "1.0.0"},
			Initialize: func(context.Context, *Session) error {
				serviceInitializeCalls++
				return nil
			},
			Deliver: func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, nil
			},
			Check: func(_ context.Context, session *Session, request bridgepkg.BridgeCheckRequest) (
				bridgepkg.BridgeCheckResponse,
				error,
			) {
				checkCalls++
				if session.HostAPI() != nil {
					t.Fatal("control session HostAPI() != nil, want zero Host API surface")
				}
				if _, ok := session.Cache().BoundSecretValue(request.BridgeInstanceID, "bot_token"); !ok {
					t.Fatal("control session cache missing launch-time secret")
				}
				return bridgepkg.BridgeCheckResponse{
					Checks: []bridgepkg.BridgeCheckRecord{bridgepkg.PassCheck("provider.identity")},
				}, nil
			},
			RegisterWebhook: func(
				context.Context,
				*Session,
				bridgepkg.BridgeWebhookRegistrationRequest,
			) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
				t.Fatal("disallowed webhook handler executed")
				return bridgepkg.BridgeWebhookRegistrationResponse{}, nil
			},
			HealthCheck: func(context.Context, *Session) error {
				serviceHealthCalls++
				return nil
			},
			Shutdown: func(context.Context, *Session, subprocess.ShutdownRequest) error {
				serviceShutdownCalls++
				return nil
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}

		request := testControlInitializeRequest(bridgepkg.ControlMethodCheck)
		raw, err := json.Marshal(request)
		if err != nil {
			t.Fatalf("json.Marshal(initialize) error = %v", err)
		}
		result, err := runtime.handleInitialize(context.Background(), raw)
		if err != nil {
			t.Fatalf("handleInitialize() error = %v", err)
		}
		response, ok := result.(subprocess.InitializeResponse)
		if !ok {
			t.Fatalf("initialize response type = %T, want InitializeResponse", result)
		}
		if got, want := response.ImplementedMethods, []string{
			string(bridgepkg.ControlMethodCheck),
			"shutdown",
		}; !slices.Equal(got, want) {
			t.Fatalf("ImplementedMethods = %#v, want %#v", got, want)
		}
		if response.Supports.HealthCheck {
			t.Fatal("Supports.HealthCheck = true, want false in control mode")
		}
		if serviceInitializeCalls != 0 {
			t.Fatalf("service Initialize calls = %d, want zero", serviceInitializeCalls)
		}

		checkRaw, err := json.Marshal(bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-1"})
		if err != nil {
			t.Fatalf("json.Marshal(check) error = %v", err)
		}
		checked, err := runtime.handleCheck(context.Background(), checkRaw)
		if err != nil {
			t.Fatalf("handleCheck() error = %v", err)
		}
		if _, ok := checked.(bridgepkg.BridgeCheckResponse); !ok {
			t.Fatalf("handleCheck() result type = %T, want BridgeCheckResponse", checked)
		}
		if checkCalls != 1 {
			t.Fatalf("check calls = %d, want 1", checkCalls)
		}

		webhookRaw, err := json.Marshal(bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: "brg-1"})
		if err != nil {
			t.Fatalf("json.Marshal(webhook) error = %v", err)
		}
		_, err = runtime.handleRegisterWebhook(context.Background(), webhookRaw)
		if !rpcErrorDataContains(err, "not allowed") {
			t.Fatalf("handleRegisterWebhook() error = %v, want not allowed", err)
		}
		if _, err := runtime.handleHealthCheck(context.Background(), nil); !rpcErrorDataContains(err, "not available") {
			t.Fatalf("handleHealthCheck() error = %v, want unavailable", err)
		}
		if serviceHealthCalls != 0 {
			t.Fatalf("service HealthCheck calls = %d, want zero", serviceHealthCalls)
		}
		if _, err := runtime.handleShutdown(context.Background(), nil); err != nil {
			t.Fatalf("handleShutdown() error = %v", err)
		}
		if serviceShutdownCalls != 0 {
			t.Fatalf("service Shutdown calls = %d, want zero", serviceShutdownCalls)
		}
	})
}

func TestRuntimeControlModeValidatesHandlerResponses(t *testing.T) {
	t.Run("Should reject an invalid provider check response", func(t *testing.T) {
		t.Parallel()

		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "telegram-adapter", Version: "1.0.0"},
			Deliver: func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, nil
			},
			Check: func(context.Context, *Session, bridgepkg.BridgeCheckRequest) (
				bridgepkg.BridgeCheckResponse,
				error,
			) {
				return bridgepkg.BridgeCheckResponse{}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		initializeRuntimeForControlTest(t, runtime, bridgepkg.ControlMethodCheck)

		raw, err := json.Marshal(bridgepkg.BridgeCheckRequest{BridgeInstanceID: "brg-1"})
		if err != nil {
			t.Fatalf("json.Marshal(check) error = %v", err)
		}
		if _, err := runtime.handleCheck(context.Background(), raw); !rpcErrorDataContains(err, "checks") {
			t.Fatalf("handleCheck() error = %v, want response validation error", err)
		}
	})
}

func rpcErrorDataContains(err error, want string) bool {
	var rpcErr *subprocess.RPCError
	return errors.As(err, &rpcErr) && strings.Contains(string(rpcErr.Data), want)
}

func TestRuntimeControlModeRegistersWebhookOnlyWhenExplicitlyAllowed(t *testing.T) {
	t.Run("Should invoke webhook registration only for its explicit control grant", func(t *testing.T) {
		t.Parallel()

		registerCalls := 0
		runtime, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "telegram-adapter", Version: "1.0.0"},
			Deliver: func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, nil
			},
			Check: testCheckHandler,
			RegisterWebhook: func(
				_ context.Context,
				_ *Session,
				request bridgepkg.BridgeWebhookRegistrationRequest,
			) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
				registerCalls++
				if request.BridgeInstanceID != "brg-1" {
					t.Fatalf("BridgeInstanceID = %q, want brg-1", request.BridgeInstanceID)
				}
				return bridgepkg.BridgeWebhookRegistrationResponse{
					Status: bridgepkg.BridgeCheckStatusPass,
				}, nil
			},
		})
		if err != nil {
			t.Fatalf("NewRuntime() error = %v", err)
		}
		initializeRuntimeForControlTest(t, runtime, bridgepkg.ControlMethodWebhookRegister)

		raw, err := json.Marshal(bridgepkg.BridgeWebhookRegistrationRequest{BridgeInstanceID: "brg-1"})
		if err != nil {
			t.Fatalf("json.Marshal(webhook) error = %v", err)
		}
		result, err := runtime.handleRegisterWebhook(context.Background(), raw)
		if err != nil {
			t.Fatalf("handleRegisterWebhook() error = %v", err)
		}
		response, ok := result.(bridgepkg.BridgeWebhookRegistrationResponse)
		if !ok || response.Status != bridgepkg.BridgeCheckStatusPass {
			t.Fatalf("handleRegisterWebhook() result = %#v, want pass response", result)
		}
		if registerCalls != 1 {
			t.Fatalf("register calls = %d, want 1", registerCalls)
		}
	})
}

func TestNewRuntimeRequiresCheckHandlerForEveryBridgeAdapter(t *testing.T) {
	t.Run("Should reject a runtime without the required check handler", func(t *testing.T) {
		t.Parallel()

		_, err := NewRuntime(RuntimeConfig{
			ExtensionInfo: subprocess.InitializeExtensionInfo{Name: "telegram-adapter", Version: "1.0.0"},
			Deliver: func(context.Context, *Session, bridgepkg.DeliveryRequest) (bridgepkg.DeliveryAck, error) {
				return bridgepkg.DeliveryAck{}, nil
			},
		})
		if err == nil || !strings.Contains(err.Error(), "check handler") {
			t.Fatalf("NewRuntime() error = %v, want check handler required", err)
		}
	})
}

func testControlInitializeRequest(method bridgepkg.ControlMethod) subprocess.InitializeRequest {
	request := testInitializeRequest()
	request.Capabilities = subprocess.InitializeCapabilities{}
	request.Methods = subprocess.InitializeMethods{
		DaemonRequests:    []string{"shutdown"},
		ExtensionServices: []string{string(method)},
	}
	runtime := testManagedRuntime("brg-1")
	runtime.Purpose = subprocess.BridgeRuntimePurposeControl
	runtime.AllowedMethods = []string{string(method)}
	runtime.ManagedInstances[0].Instance.Enabled = false
	runtime.ManagedInstances[0].Instance.Status = bridgepkg.BridgeStatusDisabled
	request.Runtime.Bridge = runtime
	return request
}

func initializeRuntimeForControlTest(
	t *testing.T,
	runtime *Runtime,
	method bridgepkg.ControlMethod,
) {
	t.Helper()

	raw, err := json.Marshal(testControlInitializeRequest(method))
	if err != nil {
		t.Fatalf("json.Marshal(initialize) error = %v", err)
	}
	if _, err := runtime.handleInitialize(context.Background(), raw); err != nil {
		t.Fatalf("handleInitialize() error = %v", err)
	}
}
