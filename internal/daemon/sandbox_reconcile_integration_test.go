//go:build integration

package daemon

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestDaemonSandboxReconcileIntegrationBootFinalizeReattachesBeforeObserverReconcile(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-crashed-active",
		state:  session.StateActive,
		env:    remoteMeta("env-crashed-active", sandbox.BackendDaytona, "daytona", "sandbox-crashed-active"),
		agent:  "coder",
		worker: "ws-crashed-active",
	})

	resourceReconcile := &fakeResourceReconcileDriver{
		onRunBoot: func() {
			if got := len(provider.prepareRequests); got != 0 {
				t.Fatalf("Prepare calls during resource RunBoot = %d, want 0", got)
			}
		},
	}
	observer := &fakeObserver{
		onReconcile: func() {
			if got := len(provider.prepareRequests); got != 1 {
				t.Fatalf("Prepare calls before observer Reconcile = %d, want 1", got)
			}
		},
	}
	state.resourceReconcile = resourceReconcile
	state.observer = observer

	if err := daemon.bootFinalize(testutil.Context(t), state); err != nil {
		t.Fatalf("bootFinalize() error = %v", err)
	}

	if resourceReconcile.runBootCalls != 1 {
		t.Fatalf("resource RunBoot calls = %d, want 1", resourceReconcile.runBootCalls)
	}
	if !observer.reconciled {
		t.Fatal("observer Reconcile was not called")
	}
	req := provider.prepareRequests[0]
	if req.SandboxID != "env-crashed-active" ||
		req.InstanceID != "sandbox-crashed-active" ||
		string(req.ProviderState) == "" {
		t.Fatalf("PrepareRequest = %#v, want persisted sandbox identity and state", req)
	}
}

func TestDaemonSandboxReconcileIntegrationPartialCreateFoundBySandboxID(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	provider.findState = sandbox.SessionState{
		SandboxID:     "env-timeout",
		Backend:       sandbox.BackendDaytona,
		Profile:       "daytona",
		State:         "found",
		InstanceID:    "sandbox-timeout",
		ProviderState: json.RawMessage(`{"sandbox":"timeout"}`),
	}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-timeout",
		state:  session.StateActive,
		env:    remoteMeta("env-timeout", sandbox.BackendDaytona, "daytona", ""),
		agent:  "coder",
		worker: "ws-timeout",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := provider.findRequests[0].Labels["agh_sandbox_id"]; got != "env-timeout" {
		t.Fatalf("agh_sandbox_id lookup = %q, want env-timeout", got)
	}
	if got := provider.prepareRequests[0].InstanceID; got != "sandbox-timeout" {
		t.Fatalf("PrepareRequest.InstanceID = %q, want sandbox-timeout", got)
	}
}

func TestDaemonSandboxReconcileIntegrationUnrecoverableSandboxDestroyLogged(t *testing.T) {
	daemon, state, provider, logs := newSandboxReconcileHarness(t)
	provider.prepareErr = errIntegrationReconcileProviderFailure{}
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-unrecoverable",
		state:  session.StateActive,
		env:    remoteMeta("env-unrecoverable", sandbox.BackendDaytona, "daytona", "sandbox-unrecoverable"),
		agent:  "coder",
		worker: "ws-unrecoverable",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.destroyStates); got != 1 {
		t.Fatalf("Destroy calls = %d, want 1", got)
	}
	if !strings.Contains(logs.String(), "daemon: sandbox reattach failed") ||
		!strings.Contains(logs.String(), "daemon: sandbox destroy complete") {
		t.Fatalf("logs missing reattach failure and cleanup: %s", logs.String())
	}
}

func TestDaemonSandboxReconcileIntegrationStoppedRemoteSessionDoesNotReattach(t *testing.T) {
	daemon, state, provider, _ := newSandboxReconcileHarness(t)
	writeSandboxReconcileMeta(t, daemon, sandboxReconcileMeta{
		id:     "sess-stopped-integration",
		state:  session.StateStopped,
		env:    remoteMeta("env-stopped-integration", sandbox.BackendDaytona, "daytona", "sandbox-stopped-integration"),
		agent:  "coder",
		worker: "ws-stopped-integration",
	})

	daemon.reconcileDaemonSandboxes(testutil.Context(t), state)

	if got := len(provider.prepareRequests); got != 0 {
		t.Fatalf("Prepare calls = %d, want 0", got)
	}
	if got := len(provider.destroyStates); got != 1 {
		t.Fatalf("Destroy calls = %d, want 1 cleanup attempt", got)
	}
}

type errIntegrationReconcileProviderFailure struct{}

func (errIntegrationReconcileProviderFailure) Error() string {
	return "reattach failed"
}
