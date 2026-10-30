package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

func TestHostedServiceBindNonceLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unbound launch after its deadline", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		if _, err := service.Launch(
			t.Context(),
			HostedLaunchRequest{SessionID: "sess-unbound", WorkspaceID: "ws-1", AgentName: "claude"},
		); err != nil {
			t.Fatal(err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-unbound"); err != nil {
			t.Fatal(err)
		}
		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		if err := service.WaitForBind(canceled, "sess-unbound"); !errors.Is(err, context.Canceled) {
			t.Fatalf("WaitForBind() = %v, want cancellation", err)
		}
		now = now.Add(time.Hour)
		if err := service.WaitForBind(t.Context(), "sess-unbound"); !errors.Is(err, ErrHostedBindTimeout) {
			t.Fatalf("WaitForBind() = %v, want bind timeout", err)
		}
	})

	t.Run("Should wait until the current bind deadline expires", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			executable := hostedTestExecutable(t, "compozy")
			service := newHostedTestService(t, executable, &hostedRegistryStub{}, time.Now)
			if _, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-timeout"}); err != nil {
				t.Fatal(err)
			}
			started := time.Now()
			if err := service.ArmLaunch(t.Context(), "sess-timeout"); err != nil {
				t.Fatal(err)
			}
			if err := service.WaitForBind(t.Context(), "sess-timeout"); !errors.Is(err, ErrHostedBindTimeout) {
				t.Fatalf("WaitForBind() = %v, want timeout", err)
			}
			if elapsed := time.Since(started); elapsed < 2*time.Second {
				t.Fatalf("bind wait elapsed = %v, want the full configured window", elapsed)
			}
		})
	})

	t.Run("Should honor a renewed deadline while awaiting the first projection", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			executable := hostedTestExecutable(t, "compozy")
			release := make(chan struct{})
			registry := &hostedRegistryStub{releaseList: release}
			service := newHostedTestService(t, executable, registry, time.Now)
			launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-renewed"})
			if err != nil {
				t.Fatal(err)
			}
			if err := service.ArmLaunch(t.Context(), "sess-renewed"); err != nil {
				t.Fatal(err)
			}
			request := HostedBindRequest{SessionID: "sess-renewed", Nonce: hostedLaunchNonce(t, launch.Args)}
			bindCtx, cancel := context.WithCancel(t.Context())
			defer cancel()
			bound := make(chan error, 1)
			go func() { _, err := service.Bind(bindCtx, request, hostedTestPeer(executable)); bound <- err }()
			synctest.Wait()
			ready := make(chan error, 1)
			go func() { ready <- service.WaitForBind(t.Context(), request.SessionID) }()
			synctest.Wait()
			time.Sleep(time.Second)
			cancel()
			if err := <-bound; !errors.Is(err, context.Canceled) {
				t.Fatalf("initial bind = %v", err)
			}
			synctest.Wait()
			time.Sleep(1500 * time.Millisecond)
			select {
			case err := <-ready:
				t.Fatalf("wait ended before renewed deadline: %v", err)
			default:
			}
			close(release)
			if _, err := service.Bind(t.Context(), request, hostedTestPeer(executable)); err != nil {
				t.Fatal(err)
			}
			if err := <-ready; err != nil {
				t.Fatalf("replacement bind readiness = %v", err)
			}
		})
	})

	for _, late := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should require projection completion before the deadline late=%t", late),
			func(t *testing.T) {
				t.Parallel()
				now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
				executable := hostedTestExecutable(t, "compozy")
				registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
				if late {
					registry.onListed = func() { now = now.Add(time.Hour) }
				}
				service := newHostedTestService(t, executable, registry, func() time.Time { return now })
				peer := hostedTestPeer(executable)
				hostedTestBind(t, service, "sess-projection-deadline", peer)
				// Negotiation may return after the deadline even when projection was on time.
				now = now.Add(time.Hour)
				err := service.WaitForBind(t.Context(), "sess-projection-deadline")
				if late && !errors.Is(err, ErrHostedBindTimeout) {
					t.Fatalf("WaitForBind(late projection) = %v, want timeout", err)
				}
				if !late && err != nil {
					t.Fatalf("WaitForBind(on-time projection) = %v", err)
				}
			},
		)
	}

	t.Run("Should rearm a released proxy without replacing the provider session", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{
			SessionID: "sess-reconnect", ProfileID: "profile-a", WorkspaceID: "ws-1", AgentName: "agy",
		})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-reconnect"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		request := HostedBindRequest{SessionID: "sess-reconnect", Nonce: hostedLaunchNonce(t, launch.Args)}
		peer := hostedTestPeer(executable)
		first, err := service.Bind(t.Context(), request, peer)
		if err != nil {
			t.Fatalf("Bind(first proxy) error = %v", err)
		}
		if err := service.WaitForBind(t.Context(), request.SessionID); err != nil {
			t.Fatalf("WaitForBind() = %v", err)
		}
		if err := service.BindRun(t.Context(), request.SessionID, "run-reconnect", 7); err != nil {
			t.Fatalf("BindRun() error = %v", err)
		}
		now = now.Add(time.Hour)
		otherPeer := peer
		otherPeer.PID++
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
		if _, err := service.Bind(t.Context(), request, otherPeer); !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(after unauthorized release) error = %v, want ErrHostedNonceInvalid", err)
		}
		if err := service.ReleaseBindForPeer(t.Context(), first.BindID, peer); err != nil {
			t.Fatalf("ReleaseBindForPeer() error = %v", err)
		}
		peer.PID++
		reconnected, err := service.Bind(t.Context(), request, peer)
		if err != nil {
			t.Fatalf("Bind(restarted proxy) error = %v", err)
		}
		if err := service.WaitForBind(t.Context(), request.SessionID); err != nil {
			t.Fatalf("WaitForBind(reconnected) = %v", err)
		}
		if reconnected.BindID == first.BindID || reconnected.Scope.ProfileID != "profile-a" ||
			reconnected.Scope.WorkspaceID != "ws-1" || reconnected.Scope.SessionID != request.SessionID ||
			reconnected.Scope.AgentName != "agy" || reconnected.Scope.RunID != "run-reconnect" ||
			reconnected.Scope.Generation != 7 {
			t.Fatalf("reconnected binding = %#v, want a new bind with the same scope and active run", reconnected)
		}
		if _, err := service.Projection(t.Context(), first.BindID, peer); !errors.Is(err, ErrHostedBindNotFound) {
			t.Fatalf("Projection(released bind) error = %v, want ErrHostedBindNotFound", err)
		}
		service.ReleaseBind(first.BindID)
		if _, err := service.Bind(t.Context(), request, peer); !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("Bind(concurrent proxy after stale release) error = %v, want ErrHostedNonceInvalid", err)
		}
		if _, err := service.Projection(t.Context(), reconnected.BindID, peer); err != nil {
			t.Fatalf("Projection(reconnected bind) error = %v", err)
		}
	})

	t.Run("Should expire the reconnect window after an authenticated release", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-reconnect-expired"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-reconnect-expired"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		request := HostedBindRequest{SessionID: "sess-reconnect-expired", Nonce: hostedLaunchNonce(t, launch.Args)}
		peer := hostedTestPeer(executable)
		bind, err := service.Bind(t.Context(), request, peer)
		if err != nil {
			t.Fatalf("Bind() error = %v", err)
		}
		now = now.Add(time.Hour)
		if err := service.ReleaseBindForPeer(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("ReleaseBindForPeer() error = %v", err)
		}
		now = now.Add(3 * time.Second)
		// A stale release must not reopen the window armed by the authenticated release.
		service.ReleaseBind(bind.BindID)
		if _, err := service.Bind(t.Context(), request, peer); !errors.Is(err, ErrHostedNonceExpired) {
			t.Fatalf("Bind(expired reconnect window) error = %v, want ErrHostedNonceExpired", err)
		}
	})

	t.Run(
		"Should reject concurrent binds and stale or expired launches without leaks",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
			executable := hostedTestExecutable(t, "compozy")
			registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__hosted_echo")}}
			service := newHostedTestService(t, executable, registry, func() time.Time { return now })

			launch, err := service.Launch(t.Context(), HostedLaunchRequest{
				SessionID:   "sess-1",
				ProfileID:   "profile-a",
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
			if bind.Scope.ProfileID != "profile-a" || bind.Scope.SessionID != "sess-1" ||
				bind.Scope.WorkspaceID != "ws-1" || bind.Scope.AgentName != "codex" {
				t.Fatalf("bind scope = %#v, want launch scope", bind.Scope)
			}

			now = now.Add(3 * time.Second)
			restartedPeer := peer
			restartedPeer.PID++
			_, err = service.Bind(
				t.Context(),
				HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
				restartedPeer,
			)
			if !errors.Is(err, ErrHostedNonceInvalid) {
				t.Fatalf("Bind(restarted provider with consumed launch) error = %v, want ErrHostedNonceInvalid", err)
			}
			relaunch, err := service.Launch(t.Context(), HostedLaunchRequest{
				SessionID: "sess-1", ProfileID: "profile-a", WorkspaceID: "ws-1", AgentName: "codex",
			})
			if err != nil {
				t.Fatalf("Launch(restarted provider) error = %v", err)
			}
			restartedNonce := hostedLaunchNonce(t, relaunch.Args)
			if err = service.ArmLaunch(t.Context(), "sess-1"); err != nil {
				t.Fatalf("ArmLaunch(restarted provider) error = %v", err)
			}
			restarted, err := service.Bind(
				t.Context(), HostedBindRequest{SessionID: "sess-1", Nonce: restartedNonce}, restartedPeer,
			)
			if err != nil || restarted.BindID == bind.BindID {
				t.Fatalf("Bind(restarted provider) = %#v error=%v, want distinct bind", restarted, err)
			}

			_, err = service.Bind(
				t.Context(),
				HostedBindRequest{SessionID: "sess-1", Nonce: nonce},
				restartedPeer,
			)
			if !errors.Is(err, ErrHostedNonceInvalid) {
				t.Fatalf("Bind(stale launch nonce) error = %v, want ErrHostedNonceInvalid", err)
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

			service.ReleaseBind(restarted.BindID)
			service.ReleaseSession("sess-1")
			_, err = service.Bind(
				t.Context(),
				HostedBindRequest{SessionID: "sess-1", Nonce: restartedNonce},
				restartedPeer,
			)
			if !errors.Is(err, ErrHostedNonceInvalid) {
				t.Fatalf("Bind(after ReleaseSession) error = %v, want ErrHostedNonceInvalid", err)
			}
		},
	)
}

func TestHostedProjectionResponse(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the registry-owned projection order", func(t *testing.T) {
		t.Parallel()

		views := []tools.ToolView{
			hostedToolView("compozy__zeta"),
			hostedToolView("compozy__alpha"),
		}

		projection := hostedProjectionResponse(views, new(hostedProjectionDigestMemo))
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

	t.Run("Should preserve empty projection encoding and digest", func(t *testing.T) {
		t.Parallel()

		memo := new(hostedProjectionDigestMemo)
		for _, views := range [][]tools.ToolView{nil, {}} {
			projection := hostedProjectionResponse(views, memo)
			if projection.Tools != nil {
				t.Fatalf("empty projection tools = %#v, want nil", projection.Tools)
			}
			requireHostedProjectionDigest(t, projection)
		}
	})

	t.Run("Should isolate digest snapshots from input and output mutations", func(t *testing.T) {
		t.Parallel()

		for _, testCase := range []struct {
			name   string
			output bool
		}{
			{name: "Should isolate backend capabilities supplied by the caller"},
			{name: "Should isolate backend capabilities returned to the caller", output: true},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				memo := new(hostedProjectionDigestMemo)
				views := []tools.ToolView{hostedToolView("compozy__alpha")}
				views[0].Descriptor.Backend.RequiresCapabilities = []string{"files.read"}
				first := hostedProjectionResponse(views, memo)
				if testCase.output {
					views = first.Tools
				}
				views[0].Descriptor.Backend.RequiresCapabilities[0] = "files.write"
				changed := hostedProjectionResponse(views, memo)
				requireHostedProjectionDigest(t, changed)
				if changed.Digest == first.Digest {
					t.Fatal("backend capability mutation retained the previous digest")
				}
			})
		}

		memo := new(hostedProjectionDigestMemo)
		views := []tools.ToolView{hostedToolView("compozy__alpha")}
		first := hostedProjectionResponse(views, memo)
		views[0].Descriptor.DisplayTitle = "Input changed"
		changed := hostedProjectionResponse(views, memo)
		requireHostedProjectionDigest(t, changed)
		if changed.Digest == first.Digest {
			t.Fatal("input mutation retained the previous digest")
		}
		changed.Tools[0].Descriptor.InputSchema = json.RawMessage(`{"type":"array"}`)
		changed.Tools[0].Decision.ReasonCodes = []tools.ReasonCode{tools.ReasonPolicyDenied}
		fromOutput := hostedProjectionResponse(changed.Tools, memo)
		requireHostedProjectionDigest(t, fromOutput)
		if fromOutput.Digest == changed.Digest {
			t.Fatal("output mutation retained the previous digest")
		}
		fromOutput.Tools[0].Descriptor.InputSchema[0] = '!'
		invalid := hostedProjectionResponse(fromOutput.Tools, memo)
		if invalid.Digest != "" {
			t.Fatalf("mutated raw schema digest = %q, want empty", invalid.Digest)
		}
		original := hostedProjectionResponse(first.Tools, memo)
		if original.Digest != first.Digest {
			t.Fatalf("original digest = %q, want %q", original.Digest, first.Digest)
		}
	})

	t.Run("Should return an empty digest for invalid schema JSON and recover after repair", func(t *testing.T) {
		t.Parallel()

		memo := new(hostedProjectionDigestMemo)
		views := []tools.ToolView{hostedToolView("compozy__alpha")}
		valid := hostedProjectionResponse(views, memo)
		for _, schema := range []string{`{"type":`, ``, `{"type":`} {
			views[0].Descriptor.InputSchema = json.RawMessage(schema)
			projection := hostedProjectionResponse(views, memo)
			if schema == "" {
				requireHostedProjectionDigest(t, projection)
			} else if projection.Digest != "" {
				t.Fatalf("invalid schema digest = %q, want empty", projection.Digest)
			}
		}
		repaired := hostedProjectionResponse(valid.Tools, memo)
		if repaired.Digest != valid.Digest {
			t.Fatalf("repaired digest = %q, want %q", repaired.Digest, valid.Digest)
		}
	})

	t.Run("Should keep concurrent digest snapshots isolated", func(t *testing.T) {
		t.Parallel()

		memo := new(hostedProjectionDigestMemo)
		var group sync.WaitGroup
		for _, id := range []tools.ToolID{"compozy__alpha", "compozy__beta"} {
			group.Go(func() {
				views := []tools.ToolView{hostedToolView(id)}
				for range 8 {
					requireHostedProjectionDigest(t, hostedProjectionResponse(views, memo))
				}
			})
		}
		group.Wait()
	})

	cases := []struct {
		name   string
		change func(*tools.ToolView)
	}{
		{name: "Should detect tool identity changes", change: func(v *tools.ToolView) {
			v.Descriptor.ID = "compozy__beta"
		}},
		{name: "Should detect input schema changes with the same declared digest", change: func(v *tools.ToolView) {
			v.Descriptor.InputSchema = json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`)
		}},
		{name: "Should detect output schema changes with the same declared digest", change: func(v *tools.ToolView) {
			v.Descriptor.OutputSchema = json.RawMessage(`{"type":"array"}`)
		}},
		{name: "Should detect source and workspace identity changes", change: func(v *tools.ToolView) {
			v.Descriptor.Source.WorkspaceID = "workspace-2"
			v.Descriptor.Source.ResourceVersion = "version-2"
		}},
		{name: "Should detect backend changes", change: func(v *tools.ToolView) {
			v.Descriptor.Backend.NativeName = "changed_handler"
		}},
		{name: "Should detect presentation changes", change: func(v *tools.ToolView) {
			v.Descriptor.FriendlyVerb = "Reading <details>"
		}},
		{name: "Should detect description changes", change: func(v *tools.ToolView) {
			v.Descriptor.Description = "Changed description"
		}},
		{name: "Should detect toolset and search metadata changes", change: func(v *tools.ToolView) {
			v.Descriptor.Toolsets = []tools.ToolsetID{tools.ToolsetIDCatalog}
			v.Descriptor.Tags = []string{"changed"}
			v.Descriptor.SearchHints = []string{"changed hint"}
		}},
		{name: "Should detect declared schema digest changes", change: func(v *tools.ToolView) {
			v.Descriptor.InputSchemaDigest = "changed-input"
			v.Descriptor.OutputSchemaDigest = "changed-output"
		}},
		{name: "Should detect risk and visibility changes", change: func(v *tools.ToolView) {
			v.Descriptor.Visibility = tools.VisibilityOperator
			v.Descriptor.Risk = tools.RiskMutating
			v.Descriptor.ReadOnly = false
			v.Descriptor.Destructive = true
			v.Descriptor.OpenWorld = true
			v.Descriptor.RequiresInteraction = true
			v.Descriptor.ConcurrencySafe = true
			v.Descriptor.SetMaxResultBytes(2048)
		}},
		{name: "Should detect live availability changes", change: func(v *tools.ToolView) {
			v.Availability.Executable = false
			v.Availability.ReasonCodes = []tools.ReasonCode{tools.ReasonBackendUnhealthy}
		}},
		{name: "Should detect live policy changes", change: func(v *tools.ToolView) {
			v.Decision.Callable = false
			v.Decision.ApprovalRequired = true
			v.Decision.SessionPolicyResult = "denied"
			v.Decision.ReasonCodes = []tools.ReasonCode{tools.ReasonPolicyDenied}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			memo := new(hostedProjectionDigestMemo)
			views := []tools.ToolView{hostedToolView("compozy__alpha")}
			views[0].Descriptor.InputSchemaDigest = "declared-input"
			views[0].Descriptor.OutputSchemaDigest = "declared-output"
			first := hostedProjectionResponse(views, memo)
			requireHostedProjectionDigest(t, first)
			testCase.change(&views[0])
			for range 2 {
				changed := hostedProjectionResponse(views, memo)
				requireHostedProjectionDigest(t, changed)
				if changed.Digest == first.Digest {
					t.Fatal("changed projection retained the previous digest")
				}
			}
		})
	}
}

func requireHostedProjectionDigest(t *testing.T, projection HostedProjectionResponse) {
	t.Helper()

	payload, err := json.Marshal(projection.Tools)
	if err != nil {
		t.Fatalf("json.Marshal(projection.Tools) error = %v", err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(payload))
	if projection.Digest != want {
		t.Fatalf("projection digest = %q, want canonical JSON SHA256 %q", projection.Digest, want)
	}
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
			BindID: bind.BindID, ToolName: tools.ToolIDTerminalOpen.String(),
		}, peer)
		if !errors.Is(err, ErrHostedRunRequired) {
			t.Fatalf("Call(terminal before BindRun) error = %v, want ErrHostedRunRequired", err)
		}
		_, err = service.Call(t.Context(), HostedCallRequest{
			BindID: bind.BindID, ToolName: "compozy__alpha",
		}, peer)
		if err != nil {
			t.Fatalf("Call(session tool before BindRun) error = %v", err)
		}
		preRunScope, preRunCall := registry.lastCall(t)
		if preRunScope.RunID != "" || preRunScope.Generation != 0 ||
			preRunCall.RunID != "" || preRunCall.Generation != 0 {
			t.Fatalf("session call identity = scope:%#v call:%#v, want no active run", preRunScope, preRunCall)
		}
		if err = service.BindRun(t.Context(), "sess-1", "run-1", 17); err != nil {
			t.Fatalf("BindRun() error = %v", err)
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
		if scope.SessionID != "sess-1" || scope.WorkspaceID != "ws-1" || scope.AgentName != "codex" ||
			scope.RunID != "run-1" || scope.Generation != 17 {
			t.Fatalf("registry call scope = %#v, want hosted launch scope", scope)
		}
		if call.ToolID != "compozy__alpha" || call.ToolCallID != "call-1" || call.CorrelationID != "corr-1" ||
			call.RunID != "run-1" || call.Generation != 17 {
			t.Fatalf("registry call identity = %#v, want hosted call identity", call)
		}
		if call.ApprovalToken != "" {
			t.Fatalf("registry call ApprovalToken = %q, want empty", call.ApprovalToken)
		}
		directCalls, bootstrapCalls := registry.callCounts()
		if directCalls != 0 || bootstrapCalls != 2 {
			t.Fatalf(
				"registry call counts = direct:%d bootstrap:%d, want direct:0 bootstrap:2",
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

// Invariant: a hosted bind builds one isolated projection per logical descriptor generation, including after source-state expiry.
// Owning layer: internal/mcp hosted projection cache.
// Canonical suite: internal/mcp/hosted_test.go.
func TestHostedServiceProjectionGenerationCache(t *testing.T) {
	t.Parallel()

	for _, streams := range []int{1, 16} {
		t.Run(fmt.Sprintf("Should build once per epoch for %d concurrent peers", streams), func(t *testing.T) {
			t.Parallel()
			executable := hostedTestExecutable(t, "compozy")
			native, err := tools.NewNativeProvider(tools.BuiltinSource(), hostedRuntimeNativeTool(
				"compozy__alpha", tools.RiskRead, true))
			if err != nil {
				t.Fatal(err)
			}
			provider := &hostedCountingProvider{Provider: native}
			registry, err := tools.NewRegistry(tools.WithProviders(provider))
			if err != nil {
				t.Fatal(err)
			}
			service := newHostedTestServiceWithProjectionGeneration(t, executable, registry, nil)
			epoch := NewProjectionEpoch()
			service.projectionEpoch = epoch.Current
			current := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
			service.now = func() time.Time { return current }
			binds := make([]HostedBindResponse, streams)
			peers := make([]PeerInfo, streams)
			for i := range streams {
				peers[i] = hostedTestPeer(executable)
				peers[i].PID += i
				binds[i] = hostedTestBind(t, service, fmt.Sprintf("sess-shared-%d", i), peers[i])
			}
			baseline := provider.calls.Load()
			for change := range 3 {
				epoch.Advance()
				var wg sync.WaitGroup
				for i := range streams {
					wg.Go(func() {
						response, err := service.StreamProjection(t.Context(), binds[i].BindID, peers[i], "")
						if err != nil || !slices.Equal(hostedToolIDs(response.Tools), []string{"compozy__alpha"}) {
							t.Errorf("peer %d response = %#v, error = %v", i, response, err)
							return
						}
						response.Tools[0].Descriptor.InputSchema[0] = '!'
						unchanged, err := service.StreamProjection(
							t.Context(),
							binds[i].BindID,
							peers[i],
							response.Digest,
						)
						if err != nil || unchanged.Tools != nil || unchanged.Digest != response.Digest {
							t.Errorf("unchanged peer %d response = %#v, error = %v", i, unchanged, err)
						}
					})
				}
				wg.Wait()
				probe := streams - 1
				cached, err := service.StreamProjection(t.Context(), binds[probe].BindID, peers[probe], "")
				if err != nil || len(cached.Tools) != 1 || !json.Valid(cached.Tools[0].Descriptor.InputSchema) {
					t.Fatalf("cached peer response = %#v, error = %v", cached, err)
				}
				if got := provider.calls.Load() - baseline; got != int64(change+1) {
					t.Fatalf("projection builds = %d, want %d independent of stream count", got, change+1)
				}
			}
			current = current.Add(service.StreamRefreshInterval())
			fresh, err := service.StreamProjection(t.Context(), binds[0].BindID, peers[0], "")
			if err != nil || !json.Valid(fresh.Tools[0].Descriptor.InputSchema) {
				t.Fatalf("refresh = %#v, %v", fresh, err)
			}
			if got := provider.calls.Load() - baseline; got != 4 {
				t.Fatalf("refresh builds = %d, want 4", got)
			}
			wrongPeer := peers[0]
			wrongPeer.PID += streams + 1
			if _, err := service.StreamProjection(
				t.Context(),
				binds[0].BindID,
				wrongPeer,
				"",
			); !errors.Is(
				err,
				ErrHostedPeerInvalid,
			) {
				t.Fatalf("foreign peer error = %v", err)
			}
			service.ReleaseBind(binds[0].BindID)
			if _, err := service.StreamProjection(
				t.Context(),
				binds[0].BindID,
				peers[0],
				"",
			); !errors.Is(
				err,
				ErrHostedBindNotFound,
			) {
				t.Fatalf("released peer error = %v", err)
			}
		})
	}

	t.Run("Should isolate peer policies and session dependent availability", func(t *testing.T) {
		t.Parallel()
		executable := hostedTestExecutable(t, "compozy")
		alpha := hostedRuntimeNativeTool("compozy__alpha", tools.RiskRead, true)
		beta := hostedRuntimeNativeTool("compozy__beta", tools.RiskRead, true)
		beta.Availability = func(_ context.Context, scope tools.Scope) tools.Availability {
			if scope.SessionID == "sess-unavailable" {
				return tools.Unavailable(tools.ReasonDependencyMissing)
			}
			return tools.Available()
		}
		native, err := tools.NewNativeProvider(tools.BuiltinSource(), alpha, beta)
		if err != nil {
			t.Fatal(err)
		}
		var restricted atomic.Bool
		resolver := hostedPolicyResolver(func(_ context.Context, scope tools.Scope) (tools.PolicyInputs, error) {
			inputs := tools.DefaultPolicyInputs()
			if scope.SessionID == "sess-restricted" || restricted.Load() {
				inputs.Session = tools.SessionToolPolicy{Enforced: true, Tools: []tools.ToolID{"compozy__alpha"}}
			}
			if scope.SessionID == "sess-agent" {
				patterns, err := tools.ParseToolPatterns([]string{"compozy__alpha"})
				if err != nil {
					return tools.PolicyInputs{}, err
				}
				inputs.Agent.Tools = patterns
			}
			if scope.SessionID == "sess-empty-agent" {
				inputs.Agent.Enforced = true
			}
			return inputs, nil
		})
		registry, err := tools.NewRegistry(
			tools.WithProviders(native),
			tools.WithPolicyInputResolver(resolver, tools.ToolsetCatalog{}),
		)
		if err != nil {
			t.Fatal(err)
		}
		service := newHostedTestServiceWithProjectionGeneration(t, executable, registry, nil)
		peer := hostedTestPeer(executable)
		full := hostedTestBind(t, service, "sess-full", peer)
		limited := hostedTestBind(t, service, "sess-restricted", peer)
		unavailable := hostedTestBind(t, service, "sess-unavailable", peer)
		agent := hostedTestBind(t, service, "sess-agent", peer)
		emptyAgent := hostedTestBind(t, service, "sess-empty-agent", peer)
		for _, tc := range []struct {
			bind string
			want []string
		}{
			{full.BindID, []string{"compozy__alpha", "compozy__beta"}},
			{limited.BindID, []string{"compozy__alpha"}},
			{unavailable.BindID, []string{"compozy__alpha"}},
			{agent.BindID, []string{"compozy__alpha"}},
			{emptyAgent.BindID, nil},
			{full.BindID, []string{"compozy__alpha", "compozy__beta"}},
		} {
			response, err := service.StreamProjection(t.Context(), tc.bind, peer, "")
			if err != nil || !slices.Equal(hostedToolIDs(response.Tools), tc.want) {
				t.Fatalf("projection = %#v, error = %v, want %v", response, err, tc.want)
			}
		}
		restricted.Store(true)
		response, err := service.StreamProjection(t.Context(), full.BindID, peer, "")
		if err != nil || !slices.Equal(hostedToolIDs(response.Tools), []string{"compozy__alpha"}) {
			t.Fatalf("policy refresh = %#v, error = %v", response, err)
		}
	})

	t.Run("Should keep profile and workspace views isolated across shared streams", func(t *testing.T) {
		t.Parallel()
		executable := hostedTestExecutable(t, "compozy")
		native, err := tools.NewNativeProvider(
			tools.BuiltinSource(),
			hostedRuntimeNativeTool("compozy__alpha", tools.RiskRead, true),
		)
		if err != nil {
			t.Fatal(err)
		}
		provider := &hostedCountingProvider{
			Provider: native,
			decorate: func(views []tools.Descriptor, scope tools.Scope) {
				views[0].Description = scope.ProfileID + "/" + scope.WorkspaceID
			},
		}
		registry, err := tools.NewRegistry(tools.WithProviders(provider))
		if err != nil {
			t.Fatal(err)
		}
		service := newHostedTestServiceWithProjectionGeneration(t, executable, registry, nil)
		peer := hostedTestPeer(executable)
		for _, scope := range []struct{ profile, workspace string }{{"a", "a"}, {"b", "a"}, {"a", "b"}} {
			sessionID := "sess-" + scope.profile + "-" + scope.workspace
			launch, err := service.Launch(
				t.Context(),
				HostedLaunchRequest{SessionID: sessionID, ProfileID: scope.profile, WorkspaceID: scope.workspace},
			)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.ArmLaunch(t.Context(), sessionID); err != nil {
				t.Fatal(err)
			}
			bind, err := service.Bind(
				t.Context(),
				HostedBindRequest{SessionID: sessionID, Nonce: hostedLaunchNonce(t, launch.Args)},
				peer,
			)
			if err != nil {
				t.Fatal(err)
			}
			response, err := service.StreamProjection(t.Context(), bind.BindID, peer, "")
			if err != nil || len(response.Tools) != 1 ||
				response.Tools[0].Descriptor.Description != scope.profile+"/"+scope.workspace {
				t.Fatalf("scoped projection = %#v, error = %v", response, err)
			}
		}
	})

	for _, cached := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should revalidate a consuming peer after its policy changes with cached=%t", cached),
			func(t *testing.T) {
				t.Parallel()
				executable := hostedTestExecutable(t, "compozy")
				native, err := tools.NewNativeProvider(tools.BuiltinSource(),
					hostedRuntimeNativeTool("compozy__alpha", tools.RiskRead, true),
					hostedRuntimeNativeTool("compozy__beta", tools.RiskRead, true))
				if err != nil {
					t.Fatal(err)
				}
				classified, resume := make(chan struct{}), make(chan struct{})
				var restricted, classifiedOnce, armed atomic.Bool
				resolver := hostedPolicyResolver(
					func(ctx context.Context, scope tools.Scope) (tools.PolicyInputs, error) {
						inputs := tools.DefaultPolicyInputs()
						if scope.SessionID != "sess-changing" {
							return inputs, nil
						}
						if restricted.Load() {
							inputs.Session = tools.SessionToolPolicy{
								Enforced: true,
								Tools:    []tools.ToolID{"compozy__alpha"},
							}
						}
						if armed.Load() && classifiedOnce.CompareAndSwap(false, true) {
							close(classified)
							select {
							case <-resume:
							case <-ctx.Done():
								return tools.PolicyInputs{}, ctx.Err()
							}
						}
						return inputs, nil
					},
				)
				provider := &hostedCountingProvider{Provider: native}
				registry, err := tools.NewRegistry(tools.WithProviders(provider),
					tools.WithPolicyInputResolver(resolver, tools.ToolsetCatalog{}))
				if err != nil {
					t.Fatal(err)
				}
				service := newHostedTestServiceWithProjectionGeneration(t, executable, registry, nil)
				peer := hostedTestPeer(executable)
				leader := hostedTestBind(t, service, "sess-stable", peer)
				follower := hostedTestBind(t, service, "sess-changing", peer)
				armed.Store(true)
				started := make(chan struct{})
				var first atomic.Bool
				provider.beforeList = func(ctx context.Context) error {
					if cached || !first.CompareAndSwap(false, true) {
						return nil
					}
					close(started)
					select {
					case <-resume:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}
				leaderDone := make(chan error, 1)
				go func() { _, err := service.StreamProjection(t.Context(), leader.BindID, peer, ""); leaderDone <- err }()
				if cached {
					if err := <-leaderDone; err != nil {
						t.Fatal(err)
					}
				} else {
					<-started
				}
				followerDone := make(chan error, 1)
				go func() {
					response, err := service.StreamProjection(t.Context(), follower.BindID, peer, "")
					if err == nil && !slices.Equal(hostedToolIDs(response.Tools), []string{"compozy__alpha"}) {
						err = fmt.Errorf("consumer received stale tools: %v", hostedToolIDs(response.Tools))
					}
					followerDone <- err
				}()
				<-classified
				restricted.Store(true)
				close(resume)
				if err := <-followerDone; err != nil {
					t.Fatal(err)
				}
				if !cached {
					if err := <-leaderDone; err != nil {
						t.Fatal(err)
					}
				}
			},
		)
	}

	for _, action := range []string{"cancel", "release", "advance epoch"} {
		t.Run("Should preserve other peers when a shared builder must "+action, func(t *testing.T) {
			t.Parallel()
			executable := hostedTestExecutable(t, "compozy")
			native, err := tools.NewNativeProvider(
				tools.BuiltinSource(),
				hostedRuntimeNativeTool("compozy__alpha", tools.RiskRead, true),
			)
			if err != nil {
				t.Fatal(err)
			}
			provider := &hostedCountingProvider{Provider: native}
			registry, err := tools.NewRegistry(tools.WithProviders(provider))
			if err != nil {
				t.Fatal(err)
			}
			service := newHostedTestServiceWithProjectionGeneration(t, executable, registry, nil)
			epoch := NewProjectionEpoch()
			service.projectionEpoch = epoch.Current
			peer := hostedTestPeer(executable)
			leader := hostedTestBind(t, service, "sess-flight-leader", peer)
			follower := hostedTestBind(t, service, "sess-flight-follower", peer)
			started, resume := make(chan struct{}), make(chan struct{})
			var first atomic.Bool
			provider.beforeList = func(ctx context.Context) error {
				if !first.CompareAndSwap(false, true) {
					return nil
				}
				close(started)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-resume:
					return nil
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			finished := make(chan error, 1)
			go func() { _, err := service.StreamProjection(ctx, leader.BindID, peer, ""); finished <- err }()
			select {
			case <-started:
			case <-t.Context().Done():
				t.Fatal("builder did not start")
			}
			followerDone := make(chan error, 1)
			go func() {
				response, err := service.StreamProjection(t.Context(), follower.BindID, peer, "")
				if err == nil && !slices.Equal(hostedToolIDs(response.Tools), []string{"compozy__alpha"}) {
					err = fmt.Errorf("unexpected follower tools: %v", hostedToolIDs(response.Tools))
				}
				followerDone <- err
			}()
			switch action {
			case "cancel":
				cancel()
			case "release":
				service.ReleaseBind(leader.BindID)
			case "advance epoch":
				epoch.Advance()
			}
			close(resume)
			leaderErr := <-finished
			if action == "cancel" && !errors.Is(leaderErr, context.Canceled) {
				t.Fatalf("leader error = %v", leaderErr)
			}
			if action == "release" && !errors.Is(leaderErr, ErrHostedBindNotFound) {
				t.Fatalf("leader error = %v", leaderErr)
			}
			if action == "advance epoch" && leaderErr != nil {
				t.Fatal(leaderErr)
			}
			if err := <-followerDone; err != nil {
				t.Fatalf("follower: %v", err)
			}
			before := provider.calls.Load()
			if _, err := service.StreamProjection(t.Context(), follower.BindID, peer, ""); err != nil {
				t.Fatal(err)
			}
			if provider.calls.Load() != before {
				t.Fatal("surviving peer did not reuse the stable build")
			}
		})
	}

	for _, versioned := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should omit unchanged stream tools and deliver changed tools with versioned=%t", versioned),
			func(t *testing.T) {
				t.Parallel()

				executable := hostedTestExecutable(t, "compozy")
				registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
				var generation atomic.Uint64
				generation.Store(1)
				service := newHostedTestServiceWithProjectionGeneration(t, executable, registry,
					func(context.Context, tools.Scope) (string, bool) {
						return fmt.Sprint(generation.Load()), versioned
					},
				)
				service.projectionEpoch = generation.Load
				peer := hostedTestPeer(executable)
				bind := hostedTestBind(t, service, "sess-stream-digest", peer)
				first, err := service.StreamProjection(t.Context(), bind.BindID, peer, "")
				if err != nil || len(first.Tools) != 1 {
					t.Fatalf("initial projection = %#v, error = %v", first, err)
				}
				unchanged, err := service.StreamProjection(t.Context(), bind.BindID, peer, first.Digest)
				if err != nil || unchanged.Digest != first.Digest || unchanged.Tools != nil {
					t.Fatalf("unchanged projection = %#v, error = %v", unchanged, err)
				}
				registry.replaceViews([]tools.ToolView{hostedToolView("compozy__beta")})
				generation.Add(1)
				changed, err := service.StreamProjection(t.Context(), bind.BindID, peer, first.Digest)
				if err != nil || changed.Digest == first.Digest ||
					!slices.Equal(hostedToolIDs(changed.Tools), []string{"compozy__beta"}) {
					t.Fatalf("changed projection = %#v, error = %v", changed, err)
				}
				changed.Tools[0].Descriptor.Description = "caller mutation"
				fresh, err := service.Projection(t.Context(), bind.BindID, peer)
				if err != nil || len(fresh.Tools) != 1 || fresh.Tools[0].Descriptor.Description == "caller mutation" {
					t.Fatalf("isolated one-shot projection = %#v, error = %v", fresh, err)
				}
			},
		)
	}

	t.Run("Should retain the bootstrap projection until its generation advances", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		required := hostedToolView("mcp__local__echo_environment")
		registry := &hostedRegistryStub{
			bootstrapViews: []tools.ToolView{required},
			bootstrapSet:   true,
		}
		var generation atomic.Uint64
		generation.Store(1)
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) {
				return fmt.Sprint(generation.Load()), true
			},
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-bootstrap-generation", peer)
		if got, want := hostedToolIDs(bind.Tools), []string{required.Descriptor.ID.String()}; !slices.Equal(got, want) {
			t.Fatalf("Bind() tools = %#v, want bootstrap tools %#v", got, want)
		}

		projection, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(stable generation) error = %v", err)
		}
		if got, want := hostedToolIDs(
			projection.Tools,
		), []string{
			required.Descriptor.ID.String(),
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("Projection(stable generation) tools = %#v, want bootstrap tools %#v", got, want)
		}
		if got := registry.listCallCount(); got != 0 {
			t.Fatalf("registry List calls = %d, want 0 while bootstrap generation is stable", got)
		}

		generation.Add(1)
		changed, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(changed generation) error = %v", err)
		}
		if got := hostedToolIDs(changed.Tools); len(got) != 0 {
			t.Fatalf("Projection(changed generation) tools = %#v, want authoritative empty projection", got)
		}
		if got, want := registry.listCallCount(), 1; got != want {
			t.Fatalf("registry List calls = %d, want %d after generation advance", got, want)
		}
	})

	t.Run("Should rebuild a hosted projection when descriptors change after source expiry", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		source := tools.SourceRef{
			Kind:          tools.SourceMCP,
			Owner:         "fixture",
			RawServerName: "fixture",
			Scope:         "global",
		}
		alpha := []tools.MCPToolDescriptor{{ID: "mcp__fixture__alpha"}}
		beta := []tools.MCPToolDescriptor{{ID: "mcp__fixture__beta"}}
		executor := &CallExecutor{toolCache: mcpToolListCache{
			projectionStates: make(map[string]mcpToolProjectionState),
		}}
		now := time.Now()
		if err := executor.recordToolProjection(source, "", alpha, 60_000, now); err != nil {
			t.Fatalf("recordToolProjection(alpha) error = %v", err)
		}
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(ctx context.Context, _ tools.Scope) (string, bool) {
				return executor.ProjectionGeneration(ctx, []tools.SourceRef{source})
			},
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-expired-generation", peer)
		baselineCalls := registry.listCallCount()

		if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("Projection(alpha) error = %v", err)
		}
		if err := executor.recordToolProjection(source, "", alpha, 1, now.Add(-time.Second)); err != nil {
			t.Fatalf("recordToolProjection(expire alpha) error = %v", err)
		}
		registry.replaceViews([]tools.ToolView{hostedToolView("compozy__beta")})
		if err := executor.recordToolProjection(source, "", beta, 60_000, time.Now()); err != nil {
			t.Fatalf("recordToolProjection(beta) error = %v", err)
		}

		projection, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(beta) error = %v", err)
		}
		if got, want := hostedToolIDs(projection.Tools), []string{"compozy__beta"}; !slices.Equal(got, want) {
			t.Fatalf("Projection(beta) tools = %#v, want %#v", got, want)
		}
		if got, want := registry.listCallCount(), baselineCalls+1; got != want {
			t.Fatalf("registry List calls = %d, want %d after descriptor generation changed", got, want)
		}
	})

	t.Run("Should build once per bind and authoritative generation", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{
			views: []tools.ToolView{hostedToolView("compozy__alpha")},
		}
		var generation atomic.Uint64
		generation.Store(1)
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) {
				return fmt.Sprint(generation.Load()), true
			},
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-generation-cache", peer)
		baselineCalls := registry.listCallCount()

		first, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(first) error = %v", err)
		}
		second, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(second) error = %v", err)
		}
		if got, want := registry.listCallCount(), baselineCalls; got != want {
			t.Fatalf("registry List calls = %d, want %d for unchanged generation", got, want)
		}

		wantSchema := string(second.Tools[0].Descriptor.InputSchema)
		first.Tools[0].Descriptor.InputSchema[0] = '['
		first.Tools[0].Decision.ReasonCodes = append(
			first.Tools[0].Decision.ReasonCodes,
			tools.ReasonPolicyDenied,
		)
		if string(second.Tools[0].Descriptor.InputSchema) != wantSchema {
			t.Fatalf(
				"cached schema = %s after caller mutation, want isolated %s",
				second.Tools[0].Descriptor.InputSchema,
				wantSchema,
			)
		}
		if slices.Contains(second.Tools[0].Decision.ReasonCodes, tools.ReasonPolicyDenied) {
			t.Fatalf("cached decision reasons leaked caller mutation: %#v", second.Tools[0].Decision.ReasonCodes)
		}

		registry.replaceViews([]tools.ToolView{hostedToolView("compozy__beta")})
		generation.Add(1)
		changed, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(changed generation) error = %v", err)
		}
		if got, want := hostedToolIDs(changed.Tools), []string{"compozy__beta"}; !slices.Equal(got, want) {
			t.Fatalf("changed projection tools = %#v, want %#v", got, want)
		}
		if got, want := registry.listCallCount(), baselineCalls+1; got != want {
			t.Fatalf("registry List calls = %d, want %d after generation advance", got, want)
		}
	})

	t.Run("Should coalesce concurrent cold misses into one registry list", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		ctx := testContext(t)
		generationReached := make(chan struct{}, 2)
		releaseGeneration := make(chan struct{})
		listStarted := make(chan struct{}, 1)
		releaseList := make(chan struct{})
		releaseGenerationOnce := sync.OnceFunc(func() { close(releaseGeneration) })
		releaseListOnce := sync.OnceFunc(func() { close(releaseList) })
		t.Cleanup(func() {
			releaseGenerationOnce()
			releaseListOnce()
		})
		var gateGeneration atomic.Bool
		var generation atomic.Uint64
		generation.Store(1)
		registry := &hostedRegistryStub{
			views: []tools.ToolView{hostedToolView("compozy__alpha")},
		}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(ctx context.Context, _ tools.Scope) (string, bool) {
				if gateGeneration.Load() {
					generationReached <- struct{}{}
					select {
					case <-releaseGeneration:
					case <-ctx.Done():
					}
				}
				return fmt.Sprint(generation.Load()), true
			},
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-concurrent-generation", peer)
		baselineCalls := registry.listCallCount()
		registry.listStarted = listStarted
		registry.releaseList = releaseList
		generation.Add(1)
		gateGeneration.Store(true)

		type projectionResult struct {
			response HostedProjectionResponse
			err      error
		}
		results := make(chan projectionResult, 2)
		for range 2 {
			go func() {
				response, err := service.Projection(ctx, bind.BindID, peer)
				results <- projectionResult{response: response, err: err}
			}()
		}
		for range 2 {
			select {
			case <-generationReached:
			case <-ctx.Done():
				t.Fatalf("concurrent projection did not reach generation lookup: %v", ctx.Err())
			}
		}
		releaseGenerationOnce()
		select {
		case <-listStarted:
		case <-ctx.Done():
			t.Fatalf("projection did not begin the registry list: %v", ctx.Err())
		}
		releaseListOnce()

		for range 2 {
			select {
			case result := <-results:
				if result.err != nil {
					t.Fatalf("Projection() error = %v", result.err)
				}
				got := hostedToolIDs(result.response.Tools)
				want := []string{"compozy__alpha"}
				if !slices.Equal(got, want) {
					t.Fatalf("Projection() tools = %#v, want %#v", got, want)
				}
			case <-ctx.Done():
				t.Fatalf("concurrent projection did not return: %v", ctx.Err())
			}
		}
		if got, want := registry.listCallCount(), baselineCalls+1; got != want {
			t.Fatalf("registry List calls = %d, want %d for one concurrent cold miss", got, want)
		}
	})

	t.Run("Should recompute when the generation is not authoritative", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) { return "", false },
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-unknown-generation", peer)
		baselineCalls := registry.listCallCount()

		for range 2 {
			if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
				t.Fatalf("Projection() error = %v", err)
			}
		}
		if got, want := registry.listCallCount(), baselineCalls+2; got != want {
			t.Fatalf("registry List calls = %d, want %d for unknown generation", got, want)
		}
		changedView := hostedToolView("compozy__alpha")
		changedView.Descriptor.InputSchema = json.RawMessage(`{"type":"object"}`)
		changedView.Availability.Executable = false
		changedView.Decision.Callable = false
		registry.replaceViews([]tools.ToolView{changedView})
		changed, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(changed without generation) error = %v", err)
		}
		requireHostedProjectionDigest(t, changed)
		if changed.Digest == bind.Digest || changed.Tools[0].Availability.Executable ||
			changed.Tools[0].Decision.Callable {
			t.Fatal("projection retained stale schema, availability, or policy without an authoritative generation")
		}
		if got, want := registry.listCallCount(), baselineCalls+3; got != want {
			t.Fatalf("registry List calls = %d, want %d after live changes", got, want)
		}
	})

	t.Run("Should bound unversioned stream rebuilds by refresh interval and daemon epoch", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		var generationLookups atomic.Int64
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) {
				generationLookups.Add(1)
				return "", false
			},
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-unversioned-stream", peer)
		current := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
		var epoch atomic.Uint64
		epoch.Store(1)
		service.now = func() time.Time { return current }
		service.projectionEpoch = epoch.Load
		baselineCalls := registry.listCallCount()
		baselineLookups := generationLookups.Load()
		stream := func(label string) HostedProjectionResponse {
			t.Helper()
			response, err := service.StreamProjection(t.Context(), bind.BindID, peer, "")
			if err != nil {
				t.Fatalf("StreamProjection(%s) error = %v", label, err)
			}
			return response
		}

		stream("cold")
		for range 5 {
			stream("within refresh")
		}
		if got, want := registry.listCallCount(), baselineCalls+1; got != want {
			t.Fatalf("registry List calls = %d, want %d within one refresh interval", got, want)
		}
		if got, want := generationLookups.Load(), baselineLookups+1; got != want {
			t.Fatalf("generation lookups = %d, want %d within one refresh interval", got, want)
		}

		registry.replaceViews([]tools.ToolView{hostedToolView("compozy__beta")})
		epoch.Add(1)
		if got := hostedToolIDs(stream("epoch advance").Tools); !slices.Equal(got, []string{"compozy__beta"}) {
			t.Fatalf("StreamProjection(epoch advance) tools = %#v, want immediate rebuild", got)
		}

		registry.replaceViews([]tools.ToolView{hostedToolView("compozy__gamma")})
		current = current.Add(hostedUnversionedStreamRefresh)
		if got := hostedToolIDs(stream("refresh elapsed").Tools); !slices.Equal(got, []string{"compozy__gamma"}) {
			t.Fatalf("StreamProjection(refresh elapsed) tools = %#v, want live availability", got)
		}
		if got, want := registry.listCallCount(), baselineCalls+3; got != want {
			t.Fatalf("registry List calls = %d, want %d after epoch and refresh rebuilds", got, want)
		}
		if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("Projection() error = %v", err)
		}
		if got, want := registry.listCallCount(), baselineCalls+4; got != want {
			t.Fatalf("registry List calls = %d, want %d: one-shot Projection must stay fresh", got, want)
		}
	})

	t.Run("Should rebuild an unversioned stream projection invalidated during its build", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) { return "", false },
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-unversioned-race", peer)
		var epoch atomic.Uint64
		epoch.Store(1)
		service.projectionEpoch = epoch.Load
		invalidate := sync.OnceFunc(func() {
			registry.replaceViews([]tools.ToolView{hostedToolView("compozy__beta")})
			epoch.Add(1)
		})
		registry.mu.Lock()
		registry.onListed = invalidate
		registry.mu.Unlock()

		for _, label := range []string{"invalidated build", "cached rebuild"} {
			response, err := service.StreamProjection(t.Context(), bind.BindID, peer, "")
			if err != nil {
				t.Fatalf("StreamProjection(%s) error = %v", label, err)
			}
			if got := hostedToolIDs(response.Tools); !slices.Equal(got, []string{"compozy__beta"}) {
				t.Fatalf("StreamProjection(%s) tools = %#v, want views listed after the epoch advance", label, got)
			}
		}
	})

	t.Run("Should hand one-shot readers private copies of shared cached snapshots", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) { return "stable", true },
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-cache-alias", peer)
		first, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(first) error = %v", err)
		}
		baselineCalls := registry.listCallCount()
		first.Tools[0].Descriptor.InputSchema[0] = '!'
		first.Tools[0].Descriptor.Tags = append(first.Tools[0].Descriptor.Tags, "mutated")
		first.Tools[0].Availability.ReasonCodes = append(
			first.Tools[0].Availability.ReasonCodes,
			tools.ReasonPolicyDenied,
		)

		second, err := service.Projection(t.Context(), bind.BindID, peer)
		if err != nil {
			t.Fatalf("Projection(cache hit) error = %v", err)
		}
		streamed, err := service.StreamProjection(t.Context(), bind.BindID, peer, "")
		if err != nil {
			t.Fatalf("StreamProjection(cache hit) error = %v", err)
		}
		if got := registry.listCallCount(); got != baselineCalls {
			t.Fatalf("registry List calls = %d, want %d: reads must hit the generation cache", got, baselineCalls)
		}
		pristine := hostedToolView("compozy__alpha")
		for label, response := range map[string]HostedProjectionResponse{"projection": second, "stream": streamed} {
			if response.Digest != first.Digest || len(response.Tools) != 1 ||
				!json.Valid(response.Tools[0].Descriptor.InputSchema) ||
				!slices.Equal(response.Tools[0].Descriptor.Tags, pristine.Descriptor.Tags) ||
				len(response.Tools[0].Availability.ReasonCodes) != 0 {
				t.Fatalf("%s cache hit = %#v, want the snapshot unaffected by caller mutation", label, response)
			}
		}
	})

	t.Run("Should signal projection waiters on epoch advance and bind release only", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) { return "", false },
		)
		epoch := NewProjectionEpoch()
		service.projectionEpoch = epoch.Current
		service.projectionChanges = epoch.Changes
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-change-signal", peer)

		changes := service.ProjectionChanges()
		for range 3 {
			if _, err := service.StreamProjection(t.Context(), bind.BindID, peer, ""); err != nil {
				t.Fatalf("StreamProjection() error = %v", err)
			}
		}
		requireHostedSignalOpen(t, changes.Epoch, "epoch before advance")
		requireHostedSignalOpen(t, changes.Released, "release before release")

		before := epoch.Current()
		if got := epoch.Advance(); got != before+1 {
			t.Fatalf("Advance() = %d, want %d", got, before+1)
		}
		requireHostedSignalClosed(t, changes.Epoch, "epoch after advance")
		rearmed := service.ProjectionChanges()
		requireHostedSignalOpen(t, rearmed.Epoch, "re-armed epoch")

		service.ReleaseBind(bind.BindID)
		requireHostedSignalClosed(t, rearmed.Released, "release after release")
	})

	t.Run("Should discard a released bind cache entry", func(t *testing.T) {
		t.Parallel()

		executable := hostedTestExecutable(t, "compozy")
		registry := &hostedRegistryStub{views: []tools.ToolView{hostedToolView("compozy__alpha")}}
		service := newHostedTestServiceWithProjectionGeneration(
			t,
			executable,
			registry,
			func(context.Context, tools.Scope) (string, bool) { return "1", true },
		)
		peer := hostedTestPeer(executable)
		bind := hostedTestBind(t, service, "sess-release-cache", peer)
		if _, err := service.Projection(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("Projection() error = %v", err)
		}
		if err := service.ReleaseBindForPeer(t.Context(), bind.BindID, peer); err != nil {
			t.Fatalf("ReleaseBindForPeer() error = %v", err)
		}
		if got := service.projectionCacheSize(); got != 0 {
			t.Fatalf("projection cache entries = %d, want 0 after release", got)
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
		if err = service.BindRun(t.Context(), "sess-approval", "run-approval", 19); err != nil {
			t.Fatalf("BindRun() error = %v", err)
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
		if scope.SessionID != "sess-approval" || scope.WorkspaceID != "ws-approval" || scope.AgentName != "codex" ||
			scope.RunID != "run-approval" || scope.Generation != 19 {
			t.Fatalf("approval scope = %#v, want hosted session scope", scope)
		}
		if call.ToolID != mutateID || call.ApprovalToken != "" ||
			call.RunID != "run-approval" || call.Generation != 19 {
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

		disabled, err := NewHostedService(&HostedConfig{ExpectedBinary: executable})
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
		if _, err := NewHostedService(&HostedConfig{Enabled: true}); err == nil {
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

	t.Run("Should discard an unarmed launch when arming fails", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		if _, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-arm-failed"}); err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := service.ArmLaunch(ctx, "sess-arm-failed"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ArmLaunch(canceled) error = %v, want context.Canceled", err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-arm-failed"); !errors.Is(err, ErrHostedNonceInvalid) {
			t.Fatalf("ArmLaunch(after failed arm) error = %v, want ErrHostedNonceInvalid", err)
		}
	})

	t.Run("Should preserve an armed launch when a repeat arm fails", func(t *testing.T) {
		t.Parallel()

		service := newHostedTestService(t, executable, registry, func() time.Time { return now })
		launch, err := service.Launch(t.Context(), HostedLaunchRequest{SessionID: "sess-arm-preserved"})
		if err != nil {
			t.Fatalf("Launch() error = %v", err)
		}
		if err := service.ArmLaunch(t.Context(), "sess-arm-preserved"); err != nil {
			t.Fatalf("ArmLaunch() error = %v", err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if err := service.ArmLaunch(ctx, "sess-arm-preserved"); !errors.Is(err, context.Canceled) {
			t.Fatalf("ArmLaunch(canceled repeat) error = %v, want context.Canceled", err)
		}
		if _, err := service.Bind(
			t.Context(),
			HostedBindRequest{SessionID: "sess-arm-preserved", Nonce: hostedLaunchNonce(t, launch.Args)},
			peer,
		); err != nil {
			t.Fatalf("Bind(after canceled repeat arm) error = %v", err)
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
		if err := service.BindRun(t.Context(), "sess-empty-input", "run-empty-input", 23); err != nil {
			t.Fatalf("BindRun() error = %v", err)
		}
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
	service, err := NewHostedService(&HostedConfig{
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

func newHostedTestServiceWithProjectionGeneration(
	t *testing.T,
	executable string,
	registry tools.Registry,
	generation HostedProjectionGeneration,
) *HostedService {
	t.Helper()

	service, err := NewHostedService(&HostedConfig{
		Enabled:              true,
		ExpectedBinary:       executable,
		Registry:             func() tools.Registry { return registry },
		ProjectionGeneration: generation,
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

func requireHostedSignalOpen(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
		t.Fatalf("%s signal closed, want open", label)
	default:
	}
}

func requireHostedSignalClosed(t *testing.T, signal <-chan struct{}, label string) {
	t.Helper()
	select {
	case <-signal:
	default:
		t.Fatalf("%s signal open, want closed", label)
	}
}

func hostedToolView(id tools.ToolID) tools.ToolView {
	return tools.ToolView{
		Descriptor: tools.Descriptor{
			ID:               id,
			Backend:          tools.BackendRef{Kind: tools.BackendNativeGo, NativeName: id.String()},
			ToolPresentation: tools.NewToolPresentation("Hosted "+id.String(), "", ""),
			Description:      "Hosted test tool",
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
			ID:               id,
			Backend:          tools.BackendRef{Kind: tools.BackendNativeGo, NativeName: id.String()},
			ToolPresentation: tools.NewToolPresentation("Hosted "+id.String(), "", ""),
			Description:      "Hosted runtime test tool",
			InputSchema:      json.RawMessage(`{"type":"object"}`),
			OutputSchema:     json.RawMessage(`{"type":"object"}`),
			Source:           tools.BuiltinSource(),
			Visibility:       tools.VisibilityModel,
			Risk:             risk,
			ReadOnly:         readOnly,
			Destructive:      risk == tools.RiskDestructive,
			OpenWorld:        risk == tools.RiskOpenWorld,
			ConcurrencySafe:  readOnly,
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
	mu      sync.Mutex
	records []hostedApprovalRecord
}

type hostedApprovalRecord struct {
	scope tools.Scope
	call  *tools.CallRequest
	view  *tools.ToolView
}

var _ tools.ApprovalBridge = (*hostedApprovalBridge)(nil)

func (b *hostedApprovalBridge) RequestToolApproval(
	_ context.Context,
	scope tools.Scope,
	call *tools.CallRequest,
	view *tools.ToolView,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	record := hostedApprovalRecord{scope: scope}
	if call != nil {
		record.call = new(*call)
	}
	if view != nil {
		record.view = new(*view)
	}
	b.records = append(b.records, record)
	return nil
}

func (b *hostedApprovalBridge) last(t *testing.T) (tools.Scope, tools.CallRequest, *tools.ToolView) {
	t.Helper()

	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.records) == 0 {
		t.Fatal("approval bridge was not called")
	}
	record := b.records[len(b.records)-1]
	if record.call == nil {
		t.Fatal("last approval bridge call did not include a tool call")
	}
	return record.scope, *record.call, record.view
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
	listCalls      int
	listStarted    chan<- struct{}
	releaseList    <-chan struct{}
	// onListed runs after List captured its views, simulating an invalidation during a build.
	onListed func()
}

var _ tools.Registry = (*hostedRegistryStub)(nil)

func (r *hostedRegistryStub) List(ctx context.Context, scope tools.Scope) ([]tools.ToolView, error) {
	r.mu.Lock()
	r.listCalls++
	r.scopes = append(r.scopes, scope)
	listStarted := r.listStarted
	releaseList := r.releaseList
	if r.err != nil {
		r.mu.Unlock()
		return nil, r.err
	}
	out := make([]tools.ToolView, len(r.views))
	copy(out, r.views)
	onListed := r.onListed
	r.mu.Unlock()
	if onListed != nil {
		onListed()
	}
	if listStarted != nil {
		select {
		case listStarted <- struct{}{}:
		default:
		}
	}
	if releaseList != nil {
		select {
		case <-releaseList:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return out, nil
}

func (r *hostedRegistryStub) listCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.listCalls
}

func (r *hostedRegistryStub) replaceViews(views []tools.ToolView) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.views = cloneToolViews(views)
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

type hostedCountingProvider struct {
	tools.Provider
	calls      atomic.Int64
	beforeList func(context.Context) error
	decorate   func([]tools.Descriptor, tools.Scope)
}

func (p *hostedCountingProvider) List(ctx context.Context, scope tools.Scope) ([]tools.Descriptor, error) {
	p.calls.Add(1)
	if p.beforeList != nil {
		if err := p.beforeList(ctx); err != nil {
			return nil, err
		}
	}
	views, err := p.Provider.List(ctx, scope)
	if err == nil && p.decorate != nil {
		p.decorate(views, scope)
	}
	return views, err
}

func (p *hostedCountingProvider) ProjectionScope(scope tools.Scope) tools.Scope {
	return tools.Scope{ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID}
}

type hostedPolicyResolver func(context.Context, tools.Scope) (tools.PolicyInputs, error)

func (r hostedPolicyResolver) Resolve(ctx context.Context, scope tools.Scope) (tools.PolicyInputs, error) {
	return r(ctx, scope)
}
