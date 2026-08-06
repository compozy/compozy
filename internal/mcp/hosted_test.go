package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

func TestHostedServiceBindNonceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should allow established session rebinds and reject expired first binds without leaks", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })

		launch, err := service.Launch(t.Context(), HostedLaunchRequest{
			SessionID:   "sess-1",
			WorkspaceID: "ws-1",
			AgentName:   "codex",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		nonce := hostedLaunchNonce(t, launch.Args)
		peer := hostedTestPeer(executable)
		now = now.Add(time.Minute)
		_, err = service.Bind(t.Context(), HostedBindRequest{SessionID: "sess-1", Nonce: nonce}, peer)
		if !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(before ArmLaunch) error = %v, want ErrHostedNonceInvalid", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}

		bind, err := service.Bind(t.Context(), HostedBindRequest{SessionID: "sess-1", Nonce: nonce}, peer)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if bind.Scope.SessionID != "sess-1" || bind.Scope.WorkspaceID != "ws-1" || bind.Scope.AgentName != "codex" {
			t.Fatalf("bind scope = %#v, want launch scope", bind.Scope)
		}

		now = now.Add(3 * time.Second)
		restartedPeer := peer
		restartedPeer.PID++
		restarted, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			restartedPeer,
		)
		if err != nil {
			t.Fatalf("Bind(restarted provider after launch TTL) error = %v", err)
		}
		if restarted.BindID == bind.BindID {
			t.Fatalf("restarted BindID = %q, want a distinct peer-bound bind", restarted.BindID)
		}

		_, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: "wrong-nonce"},
			restartedPeer,
		)
		if !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(wrong established nonce) error = %v, want ErrHostedNonceInvalid", err)
		}

		expiring, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-expired"})
		if err != nil {
			t.Fatalf("Launch(expired) error = %v", err)
		}
		expiredNonce := hostedLaunchNonce(t, expiring.Args)
		now = now.Add(time.Minute)
		if err = service.ArmLaunch(t.Context(), "sess-expired"); err != nil {
			t.Fatalf("ArmLaunch(expired) error = %v", err)
		}
		now = now.Add(3 * time.Second)

		_, err = service.Bind(t.Context(), HostedBindRequest{SessionID: "sess-expired", Nonce: expiredNonce}, peer)
		if !errors.Is(err, ErrHostedNonceExpired) {
			t.Fatalf("expired Bind() error = %v, want ErrHostedNonceExpired", err)
		}
		if strings.Contains(err.Error(), expiredNonce) {
			t.Fatalf("expired Bind() leaked nonce in error: %q", err.Error())
		}

		service.ReleaseSession("sess-1")
		_, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			restartedPeer,
		)
		if !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(after ReleaseSession) error = %v, want ErrHostedNonceInvalid", err)
		}
	})
}

func TestHostedProjectionResponse(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the registry-owned projection order", func(t *testing.T) {
		t.Parallel()

		views := []tools.ToolView{
			hostedToolView("compozy__zeta"),
			hostedToolView("compozy__alpha"),
		}

		projection := hostedProjectionResponse(views)
		if got, want := hostedToolIDs(
			projection.Tools,
		), []string{
			"compozy__alpha",
			"compozy__zeta",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("projection tools = %#v, want %#v", got, want)
		}
		if got, want := hostedToolIDs(views), []string{"compozy__zeta", "compozy__alpha"}; !slices.Equal(got, want) {
			t.Fatalf("registry-owned views = %#v, want %#v", got, want)
		}
	})
}

func TestHostedServiceValidatesPeerAndBinaryFailClosed(t *testing.T) {
	t.Parallel()

	t.Run("Should reject invalid peer metadata without consuming the launch nonce", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		otherExecutable := hostedTestExecutable(t, "other-compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })

		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		nonce := hostedLaunchNonce(t, launch.Args)

		_, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			PeerInfo{Supported: false, PID: 10, UID: os.Getuid(), ExecutablePath: executable},
		)
		if !errors.Is(err, ErrHostedPeerInvalid) {
			t.Fatalf("unsupported peer Bind() error = %v, want ErrHostedPeerInvalid", err)
		}

		_, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			hostedTestPeer(otherExecutable),
		)
		if !errors.Is(err, ErrHostedBinaryInvalid) {
			t.Fatalf("wrong binary Bind() error = %v, want ErrHostedBinaryInvalid", err)
		}

		if _, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			hostedTestPeer(executable),
		); err != nil {
			t.Fatalf("Bind(valid after failed validation) error = %v", err)
		}
	})

	t.Run("Should accept the same executable file through an alternate filesystem path", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		alternate := filepath.Join(t.TempDir(), "compozy-hardlink")
		if err := os.Link(executable, alternate); err != nil {
			t.Fatalf("Link(%q, %q) error = %v", executable, alternate, err)
		}
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })

		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-1"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		nonce := hostedLaunchNonce(t, launch.Args)
		peer := hostedTestPeer(alternate)

		bind, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind(peer with alternate executable path) error = %v", err)
		}
		if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("Projection(peer with alternate executable path) error = %v", err)
		}
	})
}

func TestHostedServiceProjectionAndCallUseRegistryScope(t *testing.T) {
	t.Parallel()

	t.Run("Should project sorted tools and call the registry with hosted scope", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{
			views: []tools.ToolView{
				hostedToolView("compozy__zeta"),
				hostedToolView("compozy__alpha"),
			},
			result: tools.ToolResult{Content: []tools.ToolContent{{Type: "text", Text: "ok"}}},
		}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })

		launch, err := service.Launch(t.Context(), HostedLaunchRequest{
			SessionID:   "sess-1",
			WorkspaceID: "ws-1",
			AgentName:   "codex",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		peer := hostedTestPeer(executable)
		bind, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: hostedLaunchNonce(t, launch.Args)},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if got, want := hostedToolIDs(bind.Tools), []string{
			"compozy__alpha",
			"compozy__zeta",
		}; !slices.Equal(got, want) {
			t.Fatalf("bind tools = %#v, want sorted session projection %#v", got, want)
		}

		projection, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection() error = %v", err)
		}
		if projection.Digest == "" || projection.Digest != bind.Digest {
			t.Fatalf(
				"projection digest = %q, bind digest = %q, want stable non-empty digest",
				projection.Digest,
				bind.Digest,
			)
		}

		_, err = service.Call(t.Context(), HostedCallRequest{
			BindID:        bind.BindID,
			ToolName:      "compozy__alpha",
			ToolCallID:    "call-1",
			CorrelationID: "corr-1",
			Input:         json.RawMessage(`{"message":"hello","approval_token":"client-supplied"}`),
		}, peer)
		if err != nil {
			t.Fatalf("Call() error = %v", err)
		}

		scope, call := registry.lastCall(t)
		if scope.SessionID != "sess-1" || scope.WorkspaceID != "ws-1" || scope.AgentName != "codex" {
			t.Fatalf("registry call scope = %#v, want hosted launch scope", scope)
		}
		if call.ToolID != "compozy__alpha" || call.ToolCallID != "call-1" || call.CorrelationID != "corr-1" {
			t.Fatalf("registry call identity = %#v, want hosted call identity", call)
		}
		if call.ApprovalToken != "" {
			t.Fatalf("registry call ApprovalToken = %q, want empty", call.ApprovalToken)
		}
		directCalls, bootstrapCalls := registry.callCounts()
		if directCalls != 0 || bootstrapCalls != 1 {
			t.Fatalf(
				"registry call counts = direct:%d bootstrap:%d, want direct:0 bootstrap:1",
				directCalls,
				bootstrapCalls,
			)
		}
	})

	t.Run("Should bind the bootstrap projection before deferred tools arrive", func(t *testing.T) {
		t.Parallel()

		native := hostedToolView("compozy__task_run_claim_next")
		vendor := hostedToolView("mcp__github__search")
		registry := &hostedRegistryStub{
			views:          []tools.ToolView{vendor, native},
			bootstrapViews: []tools.ToolView{native},
			bootstrapSet:   true,
		}
		executable := hostedTestExecutable(t, "compozy")
		service := newHostedTestService(
			t,
			executable,
			registry,
			func() time.Time { return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC) },
		)
		peer := hostedTestPeer(executable)
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-bootstrap"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-bootstrap"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		bind, err := service.Bind(
			t.Context(),
			HostedBindRequest{
				SessionID: "sess-bootstrap",
				Nonce:     hostedLaunchNonce(t, launch.Args),
			},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if got, want := hostedToolIDs(bind.Tools), []string{native.Descriptor.ID.String()}; !slices.Equal(got, want) {
			t.Fatalf("Bind() tools = %#v, want bootstrap tools %#v", got, want)
		}

		projection, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection() error = %v", err)
		}
		if got, want := hostedToolIDs(projection.Tools), []string{
			native.Descriptor.ID.String(),
			vendor.Descriptor.ID.String(),
		}; !slices.Equal(got, want) {
			t.Fatalf("Projection() tools = %#v, want full tools %#v", got, want)
		}
		if projection.Digest == bind.Digest {
			t.Fatalf("Projection() digest = bootstrap digest %q, want deferred update", projection.Digest)
		}
	})
}

func TestHostedServiceProjectionMatchesRegistrySessionProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should mirror callable session projection and keep operator denial diagnostics", func(t *testing.T) {
		t.Parallel()

		readID := tools.ToolIDToolInfo
		mutateID := tools.ToolIDTaskRunFail
		registry := hostedRuntimeRegistry(t, tools.PolicyInputs{
			SystemPermissionMode: tools.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		}, nil, hostedRuntimeNativeTool(readID, tools.RiskRead, true), hostedRuntimeNativeTool(
			mutateID,
			tools.RiskMutating,
			false,
		))
		executable := hostedTestExecutable(t, "compozy")
		service := newHostedTestService(
			t,
			executable,
			registry,
			func() time.Time { return time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC) },
		)
		peer := hostedTestPeer(executable)
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{
			SessionID:   "sess-1",
			WorkspaceID: "ws-1",
			AgentName:   "codex",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		bind, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-1", Nonce: hostedLaunchNonce(t, launch.Args)},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		scope := tools.Scope{SessionID: "sess-1", WorkspaceID: "ws-1", AgentName: "codex"}
		sessionProjection, err := registry.SessionProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("SessionProjection() error = %v", err)
		}
		if got, want := hostedToolIDs(bind.Tools), hostedToolIDs(sessionProjection); !slices.Equal(got, want) {
			t.Fatalf("hosted bind tools = %#v, want registry session projection %#v", got, want)
		}
		if got := hostedToolIDs(bind.Tools); slices.Contains(got, mutateID.String()) {
			t.Fatalf("hosted projection exposed approval-unreachable tool: %#v", got)
		}

		operatorProjection, err := registry.OperatorProjection(t.Context(), scope)
		if err != nil {
			t.Fatalf("OperatorProjection() error = %v", err)
		}
		mutating := hostedViewByID(operatorProjection, mutateID)
		if mutating == nil ||
			!slices.Contains(mutating.Decision.ReasonCodes, tools.ReasonApprovalRequired) ||
			!slices.Contains(mutating.Decision.ReasonCodes, tools.ReasonApprovalUnreachable) {
			t.Fatalf("operator mutating view = %#v, want denial diagnostics", mutating)
		}
	})
}

func TestHostedServiceCallUsesRegistryApprovalBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should request daemon-mediated approval without accepting hosted approval tokens", func(t *testing.T) {
		t.Parallel()

		mutateID := tools.ToolIDTaskRunFail
		bridge := &hostedApprovalBridge{}
		registry := hostedRuntimeRegistry(t, tools.PolicyInputs{
			SystemPermissionMode: tools.PermissionModeDenyAll,
			ApprovalAvailable:    true,
		}, bridge, hostedRuntimeNativeTool(mutateID, tools.RiskMutating, false))
		executable := hostedTestExecutable(t, "compozy")
		service := newHostedTestService(
			t,
			executable,
			registry,
			func() time.Time { return time.Date(2026, 4, 30, 12, 30, 0, 0, time.UTC) },
		)
		peer := hostedTestPeer(executable)
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{
			SessionID:   "sess-approval",
			WorkspaceID: "ws-approval",
			AgentName:   "codex",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-approval"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		bind, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-approval", Nonce: hostedLaunchNonce(t, launch.Args)},
			peer,
		)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		if !slices.Contains(hostedToolIDs(bind.Tools), mutateID.String()) {
			t.Fatalf("bind tools = %#v, want approval-mediated tool", hostedToolIDs(bind.Tools))
		}

		_, err = service.Call(t.Context(), HostedCallRequest{
			BindID:   bind.BindID,
			ToolName: mutateID.String(),
			Input:    json.RawMessage(`{"approval_token":"client-supplied","message":"run"}`),
		}, peer)
		if err != nil {
			t.Fatalf("Call(approval mediated) error = %v", err)
		}
		scope, call, view := bridge.last(t)
		if scope.SessionID != "sess-approval" || scope.WorkspaceID != "ws-approval" || scope.AgentName != "codex" {
			t.Fatalf("approval scope = %#v, want hosted session scope", scope)
		}
		if call.ToolID != mutateID || call.ApprovalToken != "" {
			t.Fatalf("approval call = %#v, want hosted call without approval token", call)
		}
		if view == nil || view.Descriptor.ID != mutateID || !view.Decision.ApprovalRequired {
			t.Fatalf("approval view = %#v, want approval-required tool view", view)
		}
	})
}

func TestHostedServiceReleaseAndFailureBranches(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	executable := hostedTestExecutable(t, "compozy")
	registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
	peer := hostedTestPeer(executable)

	t.Run("Should reject disabled service and invalid construction", func(t *testing.T) {
		t.Parallel()

		disabled, err := NewHostedService(HostedConfig{ExpectedBinary: executable})
		if err != nil {
			t.Fatalf("NewHostedService(disabled) error = %v", err)
		}
		if _, err := disabled.Launch(
			t.Context(),
			HostedLaunchRequest{SessionID: "sess-disabled"},
		); !errors.Is(
			err,
			ErrHostedDisabled,
		) {
			t.Fatalf("Launch(disabled) error = %v, want ErrHostedDisabled", err)
		}
		if _, err := NewHostedService(HostedConfig{Enabled: true}); err == nil {
			t.Fatal("NewHostedService(blank expected binary) error = nil, want error")
		}
	})

	t.Run("Should cancel unbound launch records", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-cancel"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		nonce := hostedLaunchNonce(t, launch.Args)
		service.CancelLaunch("sess-cancel")
		_, err = service.Bind(t.Context(), HostedBindRequest{SessionID: "sess-cancel", Nonce: nonce}, peer)
		if !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(after CancelLaunch) error = %v, want ErrHostedNonceInvalid", err)
		}
	})

	t.Run("Should release bind and session records", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		first := hostedTestBind(t, service, "sess-release-bind", peer)
		otherPeer := peer
		otherPeer.PID = 54321
		if err := service.ReleaseBindForPeer(
			t.Context(),
			first.BindID,
			otherPeer,
		); !errors.Is(
			err,
			ErrHostedPeerInvalid,
		) {
			t.Fatalf("ReleaseBindForPeer(other peer) error = %v, want ErrHostedPeerInvalid", err)
		}
		if _, err := service.Projection(t.Context(), first.BindID, peer); err != nil {
			t.Fatalf("Projection(after rejected ReleaseBindForPeer) error = %v", err)
		}
		if err := service.ReleaseBindForPeer(t.Context(), first.BindID, peer); err != nil {
			t.Fatalf("ReleaseBindForPeer(owner) error = %v", err)
		}
		if _, err := service.Projection(t.Context(), first.BindID, peer); !errors.Is(err, ErrHostedBindNotFound) {
			t.Fatalf("Projection(after ReleaseBind) error = %v, want ErrHostedBindNotFound", err)
		}

		second := hostedTestBind(t, service, "sess-release-session", peer)
		service.ReleaseSession("sess-release-session")
		if _, err := service.Projection(t.Context(), second.BindID, peer); !errors.Is(err, ErrHostedBindNotFound) {
			t.Fatalf("Projection(after ReleaseSession) error = %v, want ErrHostedBindNotFound", err)
		}
	})

	t.Run("Should fail closed when registry is unavailable", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, nil, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-no-registry"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err = service.ArmLaunch(t.Context(), "sess-no-registry"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		_, err = service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-no-registry", Nonce: hostedLaunchNonce(t, launch.Args)},
			peer,
		)
		if !errors.Is(err, ErrHostedRegistryRequired) {
			t.Fatalf("Bind(no registry) error = %v, want ErrHostedRegistryRequired", err)
		}
	})

	t.Run("Should reject missing bind and invalid tool names", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		if _, err := service.Projection(t.Context(), "", peer); !errors.Is(err, ErrHostedBindRequired) {
			t.Fatalf("Projection(blank bind) error = %v, want ErrHostedBindRequired", err)
		}
		bind := hostedTestBind(t, service, "sess-invalid-call", peer)
		if _, err := service.Call(t.Context(), HostedCallRequest{
			BindID:   bind.BindID,
			ToolName: "not valid",
		}, peer); err == nil {
			t.Fatal("Call(invalid tool name) error = nil, want validation error")
		}
	})

	t.Run("Should default empty call input to object", func(t *testing.T) {
		t.Parallel()

		localRegistry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, localRegistry, func() time.Time { return now })
		bind := hostedTestBind(t, service, "sess-empty-input", peer)
		if _, err := service.Call(t.Context(), HostedCallRequest{
			BindID:   bind.BindID,
			ToolName: "compozy__hosted_echo",
		}, peer); err != nil {
			t.Fatalf("Call(empty input) error = %v", err)
		}
		_, call := localRegistry.lastCall(t)
		if string(call.Input) != `{}` {
			t.Fatalf("Call input = %s, want {}", call.Input)
		}
	})
}

func newHostedTestService(
	t *testing.T,
	executable string,
	registry tools.Registry,
	now func() time.Time,
) *HostedService {
	t.Helper()

	counter := byte(1)
	service, err := NewHostedService(HostedConfig{
		Enabled:        true,
		BindNonceTTL:   2 * time.Second,
		ExpectedBinary: executable,
		Registry: func() tools.Registry {
			return registry
		},
		Now: now,
		NonceReader: func(dst []byte) error {
			for i := range dst {
				dst[i] = counter
				counter++
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("NewHostedService() error = %v", err)
	}
	return service
}

func hostedTestBind(
	t *testing.T,
	service *HostedService,
	sessionID string,
	peer PeerInfo,
) HostedBindResponse {
	t.Helper()

	launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: sessionID})
	if err != nil {
		t.Fatalf("Launch(%q) error = %v", sessionID, err)
	}
	if err := service.ArmLaunch(t.Context(), sessionID); err != nil {
		t.Fatalf("ArmLaunch(%q) error = %v", sessionID, err)
	}
	bind, err := service.Bind(
		t.Context(),
		HostedBindRequest{SessionID: sessionID, Nonce: hostedLaunchNonce(t, launch.Args)},
		peer,
	)
	if err != nil {
		t.Fatalf("Bind(%q) error = %v", sessionID, err)
	}
	return bind
}

func hostedTestExecutable(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return resolved
}

func hostedTestPeer(executable string) PeerInfo {
	return PeerInfo{
		Supported:      true,
		PID:            12345,
		UID:            os.Getuid(),
		ExecutablePath: executable,
	}
}

func hostedLaunchNonce(t *testing.T, args []string) string {
	t.Helper()

	for i := range args {
		if args[i] == "--bind-nonce" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("launch args = %#v, want --bind-nonce", args)
	return ""
}

func hostedToolIDs(views []tools.ToolView) []string {
	ids := make([]string, 0, len(views))
	for i := range views {
		ids = append(ids, views[i].Descriptor.ID.String())
	}
	return ids
}

func hostedToolView(id tools.ToolID) tools.ToolView {
	return tools.ToolView{
		Descriptor: tools.Descriptor{
			ID:           id,
			Backend:      tools.BackendRef{Kind: tools.BackendNativeGo, NativeName: id.String()},
			DisplayTitle: "Hosted " + id.String(),
			Description:  "Hosted test tool",
			InputSchema: json.RawMessage(
				`{"type":"object","properties":{"message":{"type":"string"}},"additionalProperties":false}`,
			),
			OutputSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
			Source:       tools.SourceRef{Kind: tools.SourceBuiltin, Owner: "daemon"},
			Visibility:   tools.VisibilityModel,
			Risk:         tools.RiskRead,
			ReadOnly:     true,
		},
		Availability: tools.Availability{
			Registered: true,
			Enabled:    true,
			Available:  true,
			Authorized: true,
			Executable: true,
		},
		Decision: tools.EffectiveToolDecision{
			VisibleToOperator:    true,
			VisibleToSession:     true,
			Callable:             true,
			RegistryPolicyResult: "allowed",
		},
	}
}

func hostedRuntimeRegistry(
	t *testing.T,
	inputs tools.PolicyInputs,
	bridge tools.ApprovalBridge,
	nativeTools ...tools.NativeTool,
) *tools.RuntimeRegistry {
	t.Helper()

	provider, err := tools.NewNativeProvider(tools.BuiltinSource(), nativeTools...)
	if err != nil {
		t.Fatalf("NewNativeProvider() error = %v", err)
	}
	options := []tools.RegistryOption{
		tools.WithProviders(provider),
		tools.WithPolicyInputs(inputs, tools.ToolsetCatalog{}),
	}
	if bridge != nil {
		options = append(options, tools.WithApprovalBridge(bridge))
	}
	registry, err := tools.NewRegistry(options...)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func hostedRuntimeNativeTool(id tools.ToolID, risk tools.RiskClass, readOnly bool) tools.NativeTool {
	return tools.NativeTool{
		Descriptor: tools.Descriptor{
			ID:              id,
			Backend:         tools.BackendRef{Kind: tools.BackendNativeGo, NativeName: id.String()},
			DisplayTitle:    "Hosted " + id.String(),
			Description:     "Hosted runtime test tool",
			InputSchema:     json.RawMessage(`{"type":"object"}`),
			OutputSchema:    json.RawMessage(`{"type":"object"}`),
			Source:          tools.BuiltinSource(),
			Visibility:      tools.VisibilityModel,
			Risk:            risk,
			ReadOnly:        readOnly,
			Destructive:     risk == tools.RiskDestructive,
			OpenWorld:       risk == tools.RiskOpenWorld,
			ConcurrencySafe: readOnly,
		},
		Call: func(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error) {
			return tools.ToolResult{
				Content: []tools.ToolContent{{Type: "text", Text: "ok"}},
				Preview: "ok",
			}, nil
		},
	}
}

func hostedViewByID(views []tools.ToolView, id tools.ToolID) *tools.ToolView {
	for i := range views {
		if views[i].Descriptor.ID == id {
			return &views[i]
		}
	}
	return nil
}

type hostedApprovalBridge struct {
	mu     sync.Mutex
	scopes []tools.Scope
	calls  []tools.CallRequest
	views  []tools.ToolView
}

var _ tools.ApprovalBridge = (*hostedApprovalBridge)(nil)

func (b *hostedApprovalBridge) RequestToolApproval(
	_ context.Context,
	scope tools.Scope,
	call tools.CallRequest,
	view *tools.ToolView,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.scopes = append(b.scopes, scope)
	b.calls = append(b.calls, call)
	if view != nil {
		b.views = append(b.views, *view)
	}
	return nil
}

func (b *hostedApprovalBridge) last(t *testing.T) (tools.Scope, tools.CallRequest, *tools.ToolView) {
	t.Helper()

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.calls) == 0 {
		t.Fatal("approval bridge was not called")
	}
	view := b.views[len(b.views)-1]
	return b.scopes[len(b.scopes)-1], b.calls[len(b.calls)-1], &view
}

type hostedRegistryStub struct {
	mu             sync.Mutex
	views          []tools.ToolView
	bootstrapViews []tools.ToolView
	bootstrapSet   bool
	scopes         []tools.Scope
	calls          []tools.CallRequest
	result         tools.ToolResult
	err            error
	directCalls    int
	bootstrapCalls int
}

var _ tools.Registry = (*hostedRegistryStub)(nil)

func (r *hostedRegistryStub) List(_ context.Context, scope tools.Scope) ([]tools.ToolView, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scopes = append(r.scopes, scope)
	if r.err != nil {
		return nil, r.err
	}
	out := make([]tools.ToolView, len(r.views))
	copy(out, r.views)
	return out, nil
}

func (r *hostedRegistryStub) BootstrapSessionProjection(
	ctx context.Context,
	scope tools.Scope,
) ([]tools.ToolView, error) {
	if !r.bootstrapSet {
		return r.List(ctx, scope)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scopes = append(r.scopes, scope)
	if r.err != nil {
		return nil, r.err
	}
	return append([]tools.ToolView(nil), r.bootstrapViews...), nil
}

func (r *hostedRegistryStub) Search(
	ctx context.Context,
	scope tools.Scope,
	_ tools.SearchQuery,
) ([]tools.ToolView, error) {
	return r.List(ctx, scope)
}

func (r *hostedRegistryStub) Get(ctx context.Context, scope tools.Scope, id tools.ToolID) (tools.ToolView, error) {
	views, err := r.List(ctx, scope)
	if err != nil {
		return tools.ToolView{}, err
	}
	for i := range views {
		if views[i].Descriptor.ID == id {
			return views[i], nil
		}
	}
	return tools.ToolView{}, tools.ErrToolNotFound
}

func (r *hostedRegistryStub) Call(
	_ context.Context,
	scope tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.directCalls++
	return r.callLocked(scope, req)
}

func (r *hostedRegistryStub) BootstrapSessionCall(
	_ context.Context,
	scope tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bootstrapCalls++
	return r.callLocked(scope, req)
}

func (r *hostedRegistryStub) callLocked(
	scope tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	r.scopes = append(r.scopes, scope)
	r.calls = append(r.calls, req)
	if r.err != nil {
		return tools.ToolResult{}, r.err
	}
	if len(r.result.Content) > 0 || len(r.result.Structured) > 0 || r.result.Preview != "" {
		return r.result, nil
	}
	return tools.ToolResult{Content: []tools.ToolContent{{Type: "text", Text: "ok"}}}, nil
}

func (r *hostedRegistryStub) callCounts() (direct int, bootstrap int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.directCalls, r.bootstrapCalls
}

func (r *hostedRegistryStub) lastCall(t *testing.T) (tools.Scope, tools.CallRequest) {
	t.Helper()

	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		t.Fatal("registry Call was not invoked")
	}
	return r.scopes[len(r.scopes)-1], r.calls[len(r.calls)-1]
}
