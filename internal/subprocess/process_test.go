package subprocess

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

const (
	testHelperEnvKey       = "AGH_TEST_SUBPROCESS_HELPER"
	testScenarioEnvKey     = "AGH_TEST_SUBPROCESS_SCENARIO"
	testShutdownMarkerEnv  = "AGH_TEST_SUBPROCESS_SHUTDOWN_MARKER"
	defaultProtocolVersion = "1"
)

func TestSubprocessHelperProcess(_ *testing.T) {
	if os.Getenv(testHelperEnvKey) != "1" {
		return
	}

	server := newHelperServer(os.Getenv(testScenarioEnvKey), strings.TrimSpace(os.Getenv(testShutdownMarkerEnv)))
	os.Exit(server.run())
}

func TestLaunchSpawnsProcessAndConnectsPipes(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "raw_echo", LaunchConfig{
		DisableTransport: true,
	})
	defer shutdownProcess(t, process)

	if process.PID() <= 0 {
		t.Fatalf("PID() = %d, want > 0", process.PID())
	}
	if process.Stdin() == nil {
		t.Fatal("Stdin() = nil, want non-nil")
	}
	if process.Stdout() == nil {
		t.Fatal("Stdout() = nil, want non-nil")
	}

	if _, err := io.WriteString(process.Stdin(), "ping\n"); err != nil {
		t.Fatalf("WriteString(stdin) error = %v", err)
	}

	line, err := bufio.NewReader(process.Stdout()).ReadString('\n')
	if err != nil {
		t.Fatalf("ReadString(stdout) error = %v", err)
	}
	if line != "ping\n" {
		t.Fatalf("stdout line = %q, want %q", line, "ping\n")
	}
}

func TestCallSendsRequestAndReceivesResponse(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	var response struct {
		Message string `json:"message"`
	}
	if err := process.Call(testContext(t), "echo", map[string]string{"message": "hello"}, &response); err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	if response.Message != "hello" {
		t.Fatalf("Call(echo) response = %#v, want message hello", response)
	}
}

func TestCallReturnsTypedResponseDecodeError(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the method and underlying materialization failure", func(t *testing.T) {
		t.Parallel()

		process := launchHelperProcess(t, "default", LaunchConfig{})
		defer shutdownProcess(t, process)
		initializeProcess(t, process, InitializeRuntime{
			HealthCheckIntervalMS: 1_000,
			HealthCheckTimeoutMS:  100,
			ShutdownTimeoutMS:     250,
			DefaultHookTimeoutMS:  100,
		})

		var response struct {
			Message int `json:"message"`
		}
		err := process.Call(
			testContext(t),
			"echo",
			map[string]string{"message": "not-an-integer"},
			&response,
		)
		var decodeErr *ResponseDecodeError
		if !errors.As(err, &decodeErr) {
			t.Fatalf("Call(echo) error = %v, want ResponseDecodeError", err)
		}
		if got, want := decodeErr.Method, "echo"; got != want {
			t.Fatalf("ResponseDecodeError.Method = %q, want %q", got, want)
		}
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("Call(echo) error = %v, want wrapped json.UnmarshalTypeError", err)
		}
	})
}

func TestCallWithContextCancellationReturnsPromptly(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	startedAt := time.Now()
	err := process.Call(ctx, "sleep", map[string]any{
		"delay_ms": 200,
		"message":  "late",
	}, nil)
	if err == nil {
		t.Fatal("Call(sleep) error = nil, want context cancellation")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Call(sleep) error = %v, want DeadlineExceeded", err)
	}
	if elapsed := time.Since(startedAt); elapsed >= 150*time.Millisecond {
		t.Fatalf("Call(sleep) elapsed = %v, want cancellation before helper delay", elapsed)
	}
}

func TestHandleMethodRoutesInboundRequests(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	if err := process.HandleMethod("host/add", func(_ context.Context, params json.RawMessage) (any, error) {
		var request struct {
			A int `json:"a"`
			B int `json:"b"`
		}
		if err := json.Unmarshal(params, &request); err != nil {
			return nil, err
		}
		return map[string]int{"sum": request.A + request.B}, nil
	}); err != nil {
		t.Fatalf("HandleMethod(host/add) error = %v", err)
	}

	var response struct {
		Sum int `json:"sum"`
	}
	if err := process.Call(testContext(t), "relay_to_host", map[string]any{
		"method": "host/add",
		"params": map[string]int{"a": 2, "b": 5},
	}, &response); err != nil {
		t.Fatalf("Call(relay_to_host) error = %v", err)
	}
	if response.Sum != 7 {
		t.Fatalf("relay_to_host response = %#v, want sum 7", response)
	}
}

func TestInitializeHandshakeSucceedsWithCompatibleVersions(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)

	response := initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})
	if response.ProtocolVersion != defaultProtocolVersion {
		t.Fatalf("Initialize() protocol_version = %q, want %q", response.ProtocolVersion, defaultProtocolVersion)
	}
}

func TestInitializeHandshakeFailsForUnsupportedProtocolVersion(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "version_mismatch", LaunchConfig{})
	defer shutdownProcess(t, process)

	_, err := process.Initialize(testContext(t), newInitializeRequest(InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	}))
	if err == nil {
		t.Fatal("Initialize() error = nil, want invalid params error")
	}

	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("Initialize() error = %T, want *RPCError", err)
	}
	if rpcErr.Code != codeInvalidParams {
		t.Fatalf("Initialize() rpc error code = %d, want %d", rpcErr.Code, codeInvalidParams)
	}
}

func TestHealthCheckMarksUnhealthyAfterConsecutiveFailures(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "health_timeout", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 20,
		HealthCheckTimeoutMS:  10,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	waitForCondition(t, time.Second, func() bool {
		state := process.HealthState()
		return !state.Healthy && state.ConsecutiveFailures >= 2
	})
}

func TestHealthCheckHealthyFalseMarksUnhealthyImmediately(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "health_false", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 20,
		HealthCheckTimeoutMS:  10,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	waitForCondition(t, time.Second, func() bool {
		state := process.HealthState()
		return !state.Healthy && strings.Contains(state.Message, "unhealthy") &&
			state.ConsecutiveFailures >= 3
	})
}

func TestStopHealthMonitorIsRaceFree(t *testing.T) {
	t.Parallel()

	for range 32 {
		lifecycleCtx, cancel := context.WithCancel(context.Background())
		process := &Process{
			lifecycleCtx:    lifecycleCtx,
			cancelLifecycle: cancel,
			healthThreshold: 1,
		}

		process.maybeStartHealthMonitor(InitializeRuntime{
			HealthCheckIntervalMS: 1,
			HealthCheckTimeoutMS:  10,
		}, InitializeSupports{HealthCheck: true})

		waitForCondition(t, time.Second, func() bool {
			return process.HealthState().LastCheckedAt != (time.Time{})
		})

		process.stopHealthMonitor()
		cancel()
	}
}

func TestStopHealthMonitorCancelsInFlightProbe(t *testing.T) {
	t.Parallel()

	lifecycleCtx, cancel := context.WithCancel(context.Background())
	process := &Process{
		stdin:           discardWriteCloser{Writer: io.Discard},
		lifecycleCtx:    lifecycleCtx,
		cancelLifecycle: cancel,
		done:            make(chan struct{}),
		state:           processStateReady,
		healthThreshold: 1,
	}
	process.transport = newTransport(process, defaultMaxMessageBytes)

	process.maybeStartHealthMonitor(InitializeRuntime{
		HealthCheckIntervalMS: 1,
		HealthCheckTimeoutMS:  1_000,
	}, InitializeSupports{HealthCheck: true})

	waitForCondition(t, time.Second, func() bool {
		process.transport.pendingMu.Lock()
		defer process.transport.pendingMu.Unlock()
		return len(process.transport.pending) == 1
	})

	cancel()

	stopped := make(chan struct{})
	go func() {
		process.stopHealthMonitor()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("stopHealthMonitor() blocked waiting for in-flight probe cancellation")
	}

	process.transport.pendingMu.Lock()
	defer process.transport.pendingMu.Unlock()
	if got := len(process.transport.pending); got != 0 {
		t.Fatalf("len(process.transport.pending) = %d, want 0", got)
	}
}

func TestShutdownSendsCooperativeRequest(t *testing.T) {
	t.Parallel()

	markerPath := filepath.Join(t.TempDir(), "shutdown.marker")
	process := launchHelperProcess(t, "default", LaunchConfig{
		ShutdownTimeout: 250 * time.Millisecond,
	}, testShutdownMarkerEnv+"="+markerPath)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	if err := process.Shutdown(testContext(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want cooperative shutdown marker", markerPath, err)
	}
}

func TestShutdownKillsAfterTimeout(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("SIGKILL escalation semantics are unix-only")
	}

	process := launchHelperProcess(t, "shutdown_hang", LaunchConfig{
		ShutdownTimeout: 50 * time.Millisecond,
		PostSignalGrace: 25 * time.Millisecond,
	})
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     50,
		DefaultHookTimeoutMS:  100,
	})

	startedAt := time.Now()
	if err := process.Shutdown(testContext(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed < 50*time.Millisecond {
		t.Fatalf("Shutdown() elapsed = %v, want wait through shutdown timeout", elapsed)
	}
}

func TestJSONRPCFramingIgnoresBlankLines(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "blank_lines", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	var response struct {
		Message string `json:"message"`
	}
	if err := process.Call(testContext(t), "echo", map[string]string{"message": "blank-ok"}, &response); err != nil {
		t.Fatalf("Call(echo) error = %v", err)
	}
	if response.Message != "blank-ok" {
		t.Fatalf("Call(echo) response = %#v, want blank-ok", response)
	}
}

func TestMessagesExceedingTenMiBAreRejected(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "oversize", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	err := process.Call(testContext(t), "oversize", struct{}{}, nil)
	if err == nil {
		t.Fatal("Call(oversize) error = nil, want message-size failure")
	}
	if !strings.Contains(err.Error(), "message exceeds") {
		t.Fatalf("Call(oversize) error = %v, want message exceeds", err)
	}
}

func TestCallRejectsBeforeInitialize(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)

	err := process.Call(testContext(t), "echo", map[string]string{"message": "early"}, nil)
	if err == nil {
		t.Fatal("Call(echo) error = nil, want not initialized")
	}
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Call(echo) error = %v, want ErrNotInitialized", err)
	}
}

func TestHandleMethodRegistrationValidation(t *testing.T) {
	t.Parallel()

	if err := (*Process)(
		nil,
	).HandleMethod("host/add", func(context.Context, json.RawMessage) (any, error) { return nil, nil }); err == nil {
		t.Fatal("(*Process)(nil).HandleMethod() error = nil, want non-nil")
	}

	rawProcess := launchHelperProcess(t, "raw_echo", LaunchConfig{DisableTransport: true})
	defer shutdownProcess(t, rawProcess)

	if err := rawProcess.HandleMethod(
		"host/add",
		func(context.Context, json.RawMessage) (any, error) { return nil, nil },
	); !errors.Is(
		err,
		ErrTransportDisabled,
	) {
		t.Fatalf("HandleMethod() error = %v, want ErrTransportDisabled", err)
	}
}

func TestUnknownInboundMethodReturnsMethodNotFound(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	err := process.Call(testContext(t), "relay_to_host", map[string]any{
		"method": "host/missing",
		"params": map[string]int{"a": 1},
	}, nil)
	if err == nil {
		t.Fatal("Call(relay_to_host missing) error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "Internal error") {
		t.Fatalf("Call(relay_to_host missing) error = %v, want wrapped internal error", err)
	}
}

func TestHealthMonitorRecordsHealthyResponses(t *testing.T) {
	t.Parallel()

	process := launchHelperProcess(t, "default", LaunchConfig{})
	defer shutdownProcess(t, process)
	initializeProcess(t, process, InitializeRuntime{
		HealthCheckIntervalMS: 20,
		HealthCheckTimeoutMS:  10,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	waitForCondition(t, time.Second, func() bool {
		state := process.HealthState()
		return state.Healthy && state.LastCheckedAt != (time.Time{}) && strings.HasPrefix(state.Message, "ok-")
	})
}

func TestInitializeRequestValidateRejectsMissingFields(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		mutate  func(*InitializeRequest)
		wantSub string
	}{
		{
			name: "missing-protocol-version",
			mutate: func(request *InitializeRequest) {
				request.ProtocolVersion = ""
			},
			wantSub: "protocol_version",
		},
		{
			name: "missing-session-nonce",
			mutate: func(request *InitializeRequest) {
				request.SessionNonce = ""
			},
			wantSub: "session_nonce",
		},
		{
			name: "missing-supported-versions",
			mutate: func(request *InitializeRequest) {
				request.SupportedProtocolVersion = nil
			},
			wantSub: "supported_protocol_versions",
		},
		{
			name: "missing-extension-name",
			mutate: func(request *InitializeRequest) {
				request.Extension.Name = ""
			},
			wantSub: "extension.name",
		},
		{
			name: "missing-extension-version",
			mutate: func(request *InitializeRequest) {
				request.Extension.Version = ""
			},
			wantSub: "extension.version",
		},
		{
			name: "missing-health-interval",
			mutate: func(request *InitializeRequest) {
				request.Runtime.HealthCheckIntervalMS = 0
			},
			wantSub: "health_check_interval_ms",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := newInitializeRequest(InitializeRuntime{
				HealthCheckIntervalMS: 1_000,
				HealthCheckTimeoutMS:  100,
				ShutdownTimeoutMS:     250,
				DefaultHookTimeoutMS:  100,
			})
			tc.mutate(&request)

			err := request.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestValidateInitializeResponseRejectsInvalidContracts(t *testing.T) {
	t.Parallel()

	request := newInitializeRequest(InitializeRuntime{
		HealthCheckIntervalMS: 1_000,
		HealthCheckTimeoutMS:  100,
		ShutdownTimeoutMS:     250,
		DefaultHookTimeoutMS:  100,
	})

	testCases := []struct {
		name    string
		setup   func(*InitializeRequest)
		mutate  func(*InitializeResponse)
		wantSub string
	}{
		{
			name: "unsupported-version",
			mutate: func(response *InitializeResponse) {
				response.ProtocolVersion = "2"
			},
			wantSub: "unsupported protocol version",
		},
		{
			name: "action-outside-grant",
			mutate: func(response *InitializeResponse) {
				response.AcceptedCapabilities.Actions = []extensionprotocol.HostAPIMethod{
					extensionprotocol.HostAPIMethodSessionsCreate,
				}
			},
			wantSub: "accepted actions",
		},
		{
			name: "missing-shutdown",
			mutate: func(response *InitializeResponse) {
				response.ImplementedMethods = []string{"health_check"}
			},
			wantSub: "shutdown method",
		},
		{
			name: "missing-health-support",
			mutate: func(response *InitializeResponse) {
				response.Supports.HealthCheck = false
			},
			wantSub: "health_check support",
		},
		{
			name: "missing-bridge-deliver-service",
			setup: func(request *InitializeRequest) {
				request.Capabilities.Provides = []string{extensionprotocol.CapabilityProvideBridgeAdapter}
				request.Methods.ExtensionServices = extensionprotocol.CapabilityServiceMethods(
					request.Capabilities.Provides,
				)
			},
			mutate: func(response *InitializeResponse) {
				response.AcceptedCapabilities.Provides = []string{extensionprotocol.CapabilityProvideBridgeAdapter}
				response.ImplementedMethods = []string{"health_check", "shutdown"}
			},
			wantSub: "bridges/deliver",
		},
		{
			name: "missing-tool-provider-provide-tools-service",
			setup: func(request *InitializeRequest) {
				request.Capabilities.Provides = []string{extensionprotocol.CapabilityToolProvider}
				request.Methods.ExtensionServices = extensionprotocol.CapabilityServiceMethods(
					request.Capabilities.Provides,
				)
			},
			mutate: func(response *InitializeResponse) {
				response.AcceptedCapabilities.Provides = []string{extensionprotocol.CapabilityToolProvider}
				response.ImplementedMethods = []string{
					"health_check",
					"shutdown",
					string(extensionprotocol.ExtensionServiceMethodToolsCall),
				}
			},
			wantSub: "provide_tools",
		},
		{
			name: "missing-tool-provider-call-service",
			setup: func(request *InitializeRequest) {
				request.Capabilities.Provides = []string{extensionprotocol.CapabilityToolProvider}
				request.Methods.ExtensionServices = extensionprotocol.CapabilityServiceMethods(
					request.Capabilities.Provides,
				)
			},
			mutate: func(response *InitializeResponse) {
				response.AcceptedCapabilities.Provides = []string{extensionprotocol.CapabilityToolProvider}
				response.ImplementedMethods = []string{
					"health_check",
					"shutdown",
					string(extensionprotocol.ExtensionServiceMethodProvideTools),
				}
			},
			wantSub: "tools/call",
		},
		{
			name: "missing-watch-source-poll-service",
			setup: func(request *InitializeRequest) {
				request.Capabilities.Provides = []string{extensionprotocol.CapabilityProvideWatchSource}
				request.Methods.ExtensionServices = extensionprotocol.CapabilityServiceMethods(
					request.Capabilities.Provides,
				)
			},
			mutate: func(response *InitializeResponse) {
				response.AcceptedCapabilities.Provides = []string{extensionprotocol.CapabilityProvideWatchSource}
				response.ImplementedMethods = []string{"health_check", "shutdown"}
				response.WatchSourceKinds = []string{"reviews"}
			},
			wantSub: "watch/poll",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := request
			if tc.setup != nil {
				tc.setup(&req)
			}

			response := InitializeResponse{
				ProtocolVersion: defaultProtocolVersion,
				AcceptedCapabilities: AcceptedCapabilities{
					Provides: append([]string(nil), req.Capabilities.Provides...),
					Actions:  append([]extensionprotocol.HostAPIMethod(nil), req.Capabilities.GrantedActions...),
					Security: append([]string(nil), req.Capabilities.GrantedSecurity...),
				},
				ImplementedMethods: []string{"health_check", "shutdown"},
				Supports: InitializeSupports{
					HealthCheck: true,
				},
			}
			tc.mutate(&response)

			err := validateInitializeResponse(req, response)
			if err == nil {
				t.Fatal("validateInitializeResponse() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("validateInitializeResponse() error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestRPCErrorFormattingAndIDParsing(t *testing.T) {
	t.Parallel()

	if got := (&RPCError{Code: 12, Message: "boom"}).Error(); !strings.Contains(got, "boom") {
		t.Fatalf("RPCError.Error() = %q, want message", got)
	}
	if got := (&RPCError{Code: 12}).Error(); !strings.Contains(got, "12") {
		t.Fatalf("RPCError.Error() without message = %q, want code", got)
	}

	stringID, err := parseRPCID(json.RawMessage(`"abc"`))
	if err != nil {
		t.Fatalf("parseRPCID(string) error = %v", err)
	}
	if stringID.key != "s:abc" {
		t.Fatalf("parseRPCID(string) key = %q, want s:abc", stringID.key)
	}

	numericID, err := parseRPCID(json.RawMessage(`42`))
	if err != nil {
		t.Fatalf("parseRPCID(number) error = %v", err)
	}
	if numericID.key != "n:42" {
		t.Fatalf("parseRPCID(number) key = %q, want n:42", numericID.key)
	}

	if _, err := parseRPCID(json.RawMessage(`4.2`)); err == nil {
		t.Fatal("parseRPCID(fractional) error = nil, want non-nil")
	}
}

func TestNilHelpersAndBufferUtilities(t *testing.T) {
	t.Parallel()

	var nilProcess *Process
	if nilProcess.PID() != 0 {
		t.Fatalf("(*Process)(nil).PID() = %d, want 0", nilProcess.PID())
	}
	if nilProcess.Stdin() != nil {
		t.Fatal("(*Process)(nil).Stdin() != nil")
	}
	if nilProcess.Stdout() != nil {
		t.Fatal("(*Process)(nil).Stdout() != nil")
	}
	if nilProcess.Stderr() != "" {
		t.Fatalf("(*Process)(nil).Stderr() = %q, want empty", nilProcess.Stderr())
	}
	<-nilProcess.Done()

	buffer := &boundedBuffer{limit: 4}
	if _, err := buffer.Write([]byte("abcdef")); err != nil {
		t.Fatalf("boundedBuffer.Write() error = %v", err)
	}
	if got := buffer.String(); got != "cdef" {
		t.Fatalf("boundedBuffer.String() = %q, want cdef", got)
	}

	buffer = &boundedBuffer{limit: 4}
	if _, err := buffer.Write([]byte("uvwxyz")); err != nil {
		t.Fatalf("boundedBuffer.Write(large) error = %v", err)
	}
	if got := buffer.String(); got != "wxyz" {
		t.Fatalf("boundedBuffer.String() after large write = %q, want wxyz", got)
	}

	if got := attachStderr(errors.New("base"), "stderr-output").Error(); !strings.Contains(got, "stderr-output") {
		t.Fatalf("attachStderr() = %q, want stderr suffix", got)
	}
	if got := (LaunchConfig{}).defaultShutdownReason(); got != "daemon_shutdown" {
		t.Fatalf("defaultShutdownReason() = %q, want daemon_shutdown", got)
	}
	if got := (LaunchConfig{ShutdownReason: "manual"}).defaultShutdownReason(); got != "manual" {
		t.Fatalf("defaultShutdownReason(custom) = %q, want manual", got)
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func launchHelperProcess(t *testing.T, scenario string, cfg LaunchConfig, extraEnv ...string) *Process {
	t.Helper()

	bin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	env := append([]string(nil), os.Environ()...)
	env = append(env,
		testHelperEnvKey+"=1",
		testScenarioEnvKey+"="+scenario,
	)
	env = append(env, extraEnv...)

	process, err := Launch(testContext(t), LaunchConfig{
		Command:                bin,
		Args:                   []string{"-test.run=TestSubprocessHelperProcess"},
		Env:                    env,
		DisableTransport:       cfg.DisableTransport,
		MaxMessageBytes:        cfg.MaxMessageBytes,
		ShutdownTimeout:        cfg.ShutdownTimeout,
		PostSignalGrace:        cfg.PostSignalGrace,
		ShutdownReason:         cfg.ShutdownReason,
		HealthFailureThreshold: cfg.HealthFailureThreshold,
	})
	if err != nil {
		t.Fatalf("Launch(helper %s) error = %v", scenario, err)
	}
	return process
}

func initializeProcess(t *testing.T, process *Process, runtimeCfg InitializeRuntime) InitializeResponse {
	t.Helper()

	response, err := process.Initialize(testContext(t), newInitializeRequest(runtimeCfg))
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	return response
}

func newInitializeRequest(runtimeCfg InitializeRuntime) InitializeRequest {
	return InitializeRequest{
		ProtocolVersion:          defaultProtocolVersion,
		SupportedProtocolVersion: []string{defaultProtocolVersion},
		AGHVersion:               "dev",
		SessionNonce:             "session-nonce-test",
		Extension: InitializeExtension{
			Name:       "test-extension",
			Version:    "0.1.0",
			SourceTier: "user",
		},
		Capabilities: InitializeCapabilities{
			Provides:        nil,
			GrantedActions:  []extensionprotocol.HostAPIMethod{extensionprotocol.HostAPIMethodSessionsList},
			GrantedSecurity: []string{"memory.read", "memory.write"},
		},
		Methods: InitializeMethods{
			DaemonRequests:    []string{"health_check", "shutdown"},
			ExtensionServices: []string{"echo", "sleep", "relay_to_host"},
		},
		Runtime: runtimeCfg,
	}
}

func shutdownProcess(t *testing.T, process *Process) {
	t.Helper()
	if process == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = process.Shutdown(ctx)
}

func waitForCondition(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for condition")
}

type helperServer struct {
	scenario       string
	shutdownMarker string

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]chan rpcEnvelope

	healthMu    sync.Mutex
	healthCount int

	shutdownHang bool
}

func newHelperServer(scenario string, shutdownMarker string) *helperServer {
	return &helperServer{
		scenario:       scenario,
		shutdownMarker: shutdownMarker,
		pending:        make(map[string]chan rpcEnvelope),
	}
}

func (h *helperServer) run() int {
	if h.scenario == "raw_echo" {
		if _, err := io.Copy(os.Stdout, os.Stdin); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "raw echo: %v\n", err)
			return 1
		}
		return 0
	}

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultMaxMessageBytes+1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var envelope rpcEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "decode envelope: %v\n", err)
			return 1
		}
		if envelope.Method == "" {
			h.deliverResponse(envelope)
			continue
		}
		if len(envelope.ID) == 0 {
			continue
		}
		go h.handleRequest(envelope)
	}
	if err := scanner.Err(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "scan stdin: %v\n", err)
		return 1
	}
	if h.shutdownHang {
		blockForever()
	}
	return 0
}

func (h *helperServer) handleRequest(envelope rpcEnvelope) {
	switch envelope.Method {
	case initializeMethod:
		h.handleInitialize(envelope)
	case "echo":
		h.handleEcho(envelope)
	case "sleep":
		h.handleSleep(envelope)
	case "relay_to_host":
		h.handleRelayToHost(envelope)
	case "health_check":
		h.handleHealthCheck(envelope)
	case shutdownMethod:
		h.handleShutdown(envelope)
	case "oversize":
		h.handleOversize(envelope)
	default:
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeMethodNotFound, "Method not found", map[string]string{"method": envelope.Method}),
		)
	}
}

func (h *helperServer) handleInitialize(envelope rpcEnvelope) {
	if h.scenario == "version_mismatch" {
		_ = h.sendError(envelope.ID, NewRPCError(codeInvalidParams, "Invalid params", map[string]any{
			"reason":                      "unsupported_protocol_version",
			"requested":                   "9",
			"supported_protocol_versions": []string{defaultProtocolVersion},
		}))
		return
	}

	var request InitializeRequest
	if err := json.Unmarshal(envelope.Params, &request); err != nil {
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeInvalidParams, "Invalid params", map[string]string{"error": err.Error()}),
		)
		return
	}

	response := InitializeResponse{
		ProtocolVersion: defaultProtocolVersion,
		ExtensionInfo: InitializeExtensionInfo{
			Name:    request.Extension.Name,
			Version: request.Extension.Version,
			SDKName: "agh-test-helper",
		},
		AcceptedCapabilities: AcceptedCapabilities{
			Provides: append([]string(nil), request.Capabilities.Provides...),
			Actions:  append([]extensionprotocol.HostAPIMethod(nil), request.Capabilities.GrantedActions...),
			Security: append([]string(nil), request.Capabilities.GrantedSecurity...),
		},
		ImplementedMethods: []string{"echo", "sleep", "relay_to_host", "health_check", "shutdown", "oversize"},
		Supports: InitializeSupports{
			HealthCheck: true,
		},
	}
	_ = h.sendResult(envelope.ID, response)

	if h.scenario == "crash_after_init" {
		go func() {
			time.Sleep(50 * time.Millisecond)
			os.Exit(3)
		}()
	}
}

func (h *helperServer) handleEcho(envelope rpcEnvelope) {
	var request struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(envelope.Params, &request); err != nil {
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeInvalidParams, "Invalid params", map[string]string{"error": err.Error()}),
		)
		return
	}
	_ = h.sendResult(envelope.ID, map[string]string{"message": request.Message})
}

func (h *helperServer) handleSleep(envelope rpcEnvelope) {
	var request struct {
		DelayMS int64  `json:"delay_ms"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(envelope.Params, &request); err != nil {
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeInvalidParams, "Invalid params", map[string]string{"error": err.Error()}),
		)
		return
	}
	time.Sleep(time.Duration(request.DelayMS) * time.Millisecond)
	_ = h.sendResult(envelope.ID, map[string]string{"message": request.Message})
}

func (h *helperServer) handleRelayToHost(envelope rpcEnvelope) {
	var request struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(envelope.Params, &request); err != nil {
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeInvalidParams, "Invalid params", map[string]string{"error": err.Error()}),
		)
		return
	}

	result, err := h.callHost(request.Method, request.Params)
	if err != nil {
		_ = h.sendError(
			envelope.ID,
			NewRPCError(codeInternalError, "Internal error", map[string]string{"error": err.Error()}),
		)
		return
	}
	_ = h.sendResult(envelope.ID, result)
}

func (h *helperServer) handleHealthCheck(envelope rpcEnvelope) {
	switch h.scenario {
	case "health_timeout":
		time.Sleep(200 * time.Millisecond)
		_ = h.sendResult(envelope.ID, HealthCheckResponse{Healthy: true})
	case "health_false":
		_ = h.sendResult(envelope.ID, HealthCheckResponse{Healthy: false, Message: "helper unhealthy"})
	default:
		h.healthMu.Lock()
		h.healthCount++
		count := h.healthCount
		h.healthMu.Unlock()
		_ = h.sendResult(envelope.ID, HealthCheckResponse{
			Healthy: true,
			Message: "ok-" + strconv.Itoa(count),
		})
	}
}

func (h *helperServer) handleShutdown(envelope rpcEnvelope) {
	if h.shutdownMarker != "" {
		_ = os.WriteFile(h.shutdownMarker, []byte("shutdown"), 0o644)
	}
	if h.scenario == "shutdown_delayed_ack" {
		time.Sleep(100 * time.Millisecond)
	}
	if h.scenario == "shutdown_hang" {
		h.shutdownHang = true
		configureIgnoreTermination()
	}
	_ = h.sendResult(envelope.ID, ShutdownResponse{Acknowledged: true})
}

func (h *helperServer) handleOversize(envelope rpcEnvelope) {
	payload := strings.Repeat("x", defaultMaxMessageBytes+1024)
	_ = h.sendResult(envelope.ID, map[string]string{"message": payload})
}

func (h *helperServer) callHost(method string, params json.RawMessage) (json.RawMessage, error) {
	requestID := fmt.Sprintf("\"ext-%d\"", time.Now().UnixNano())
	envelopeCh := make(chan rpcEnvelope, 1)

	h.pendingMu.Lock()
	h.pending["s:"+strings.Trim(requestID, "\"")] = envelopeCh
	h.pendingMu.Unlock()

	if err := h.writeEnvelope(rpcRequest{
		JSONRPC: jsonRPCVersion,
		ID:      json.RawMessage(requestID),
		Method:  method,
		Params:  params,
	}); err != nil {
		return nil, err
	}

	select {
	case response := <-envelopeCh:
		if response.Error != nil {
			return nil, response.Error
		}
		return response.Result, nil
	case <-time.After(2 * time.Second):
		return nil, errors.New("timed out waiting for host response")
	}
}

func (h *helperServer) deliverResponse(envelope rpcEnvelope) {
	id, err := parseRPCID(envelope.ID)
	if err != nil {
		return
	}
	h.pendingMu.Lock()
	responseCh, ok := h.pending[id.key]
	if ok {
		delete(h.pending, id.key)
	}
	h.pendingMu.Unlock()
	if ok {
		responseCh <- envelope
		close(responseCh)
	}
}

func (h *helperServer) sendResult(id json.RawMessage, result any) error {
	if h.scenario == "blank_lines" {
		if _, err := os.Stdout.WriteString("\n\n"); err != nil {
			return err
		}
	}
	return h.writeEnvelope(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
}

func (h *helperServer) sendError(id json.RawMessage, err *RPCError) error {
	return h.writeEnvelope(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error:   err,
	})
}

func (h *helperServer) writeEnvelope(envelope any) error {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return err
	}

	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	if _, err := os.Stdout.Write(encoded); err != nil {
		return err
	}
	_, err = os.Stdout.WriteString("\n")
	return err
}

func blockForever() {
	select {}
}
