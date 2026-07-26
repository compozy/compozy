package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	"github.com/compozy/agh/internal/testutil"
)

type contractDaemonProcess struct {
	pid     int
	done    chan struct{}
	waitErr error
}

func (p *contractDaemonProcess) PID() int {
	return p.pid
}

func (p *contractDaemonProcess) Done() <-chan struct{} {
	return p.done
}

func (p *contractDaemonProcess) Wait() error {
	<-p.done
	return p.waitErr
}

func (p *contractDaemonProcess) complete(err error) {
	p.waitErr = err
	close(p.done)
}

func TestCLIDaemonLifecycleContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should wait for the spawned daemon PID before accepting readiness", func(t *testing.T) {
		t.Parallel()

		child := &contractDaemonProcess{pid: 42, done: make(chan struct{})}
		statusCalls := 0
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				statusCalls++
				if statusCalls == 1 {
					return DaemonStatus{Status: "ready", PID: 84}, nil
				}
				return DaemonStatus{Status: "ready", PID: 42}, nil
			},
		})
		deps.pollInterval = time.Millisecond
		deps.startTimeout = 100 * time.Millisecond
		deps.processAlive = func(pid int) bool { return pid == 42 }

		status, err := waitForDaemonStart(testutil.Context(t), deps, child)
		child.complete(nil)
		if err != nil {
			t.Fatalf("waitForDaemonStart() error = %v", err)
		}
		if status.Status != "ready" || status.PID != 42 {
			t.Fatalf("waitForDaemonStart() status = %#v, want ready pid 42", status)
		}
		if statusCalls < 2 {
			t.Fatalf("DaemonStatus() calls = %d, want readiness retry after pid mismatch", statusCalls)
		}
	})

	t.Run("Should propagate live daemon status failures instead of synthesizing starting", func(t *testing.T) {
		t.Parallel()

		statusErr := errors.New("control-plane rpc failed")
		deps := newTestDeps(t, &stubClient{
			daemonStatusFn: func(context.Context) (DaemonStatus, error) {
				return DaemonStatus{}, statusErr
			},
		})
		deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
			return aghdaemon.Info{PID: 42, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(pid int) bool { return pid == 42 }

		runtime, err := loadRuntimeContext(deps)
		if err != nil {
			t.Fatalf("loadRuntimeContext() error = %v", err)
		}
		status, err := daemonStatusFromDeps(testutil.Context(t), deps, runtime)
		if !errors.Is(err, statusErr) {
			t.Fatalf("daemonStatusFromDeps() error = %v, want %v", err, statusErr)
		}
		if !reflect.DeepEqual(status, DaemonStatus{}) {
			t.Fatalf("daemonStatusFromDeps() status = %#v, want empty status on control-plane failure", status)
		}
	})
}

func TestCLIAuthoredBodyRoutingContracts(t *testing.T) {
	t.Parallel()

	t.Run("Should route agent soul file body when stdin is explicit false", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "SOUL.md")
		if err := os.WriteFile(bodyPath, []byte("# Soul\n\nBe precise.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(SOUL.md) error = %v", err)
		}
		called := false
		client := &stubClient{
			putAgentSoulFn: func(_ context.Context, name string, request AgentSoulPutRequest) (AgentSoulMutationRecord, error) {
				called = true
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("PutAgentSoul() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.WorkspaceID != "checkout-api" || request.ExpectedDigest != "sha256:old" {
					t.Fatalf("PutAgentSoul() request = %#v, want workspace and expected digest", request)
				}
				if request.Body != "# Soul\n\nBe precise.\n" {
					t.Fatalf("PutAgentSoul() body = %q, want file body", request.Body)
				}
				return AgentSoulMutationRecord{
					Soul: AgentSoulRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:new",
					},
					Revision: AgentSoulRevisionRecord{
						ID:        "rev-1",
						AgentName: "coder",
						Action:    contract.AgentSoulRevisionPut,
						NewDigest: "sha256:new",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"soul",
			"write",
			"coder",
			"--stdin=false",
			"--file",
			bodyPath,
			"--expected-digest",
			"sha256:old",
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent soul write error = %v", err)
		}
		if !called {
			t.Fatal("PutAgentSoul() was not called")
		}
	})

	t.Run("Should route agent heartbeat file body when stdin is explicit false", func(t *testing.T) {
		t.Parallel()

		bodyPath := filepath.Join(t.TempDir(), "HEARTBEAT.md")
		if err := os.WriteFile(bodyPath, []byte("# Heartbeat\n\nCheck in.\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(HEARTBEAT.md) error = %v", err)
		}
		called := false
		client := &stubClient{
			putAgentHeartbeatFn: func(
				_ context.Context,
				name string,
				request AgentHeartbeatPutRequest,
			) (AgentHeartbeatMutationRecord, error) {
				called = true
				if name != "coder" || request.AgentName != "coder" {
					t.Fatalf("PutAgentHeartbeat() agent = %q/%q, want coder", name, request.AgentName)
				}
				if request.WorkspaceID != "checkout-api" || request.ExpectedDigest != "sha256:old" {
					t.Fatalf("PutAgentHeartbeat() request = %#v, want workspace and expected digest", request)
				}
				if request.Body != "# Heartbeat\n\nCheck in.\n" {
					t.Fatalf("PutAgentHeartbeat() body = %q, want file body", request.Body)
				}
				return AgentHeartbeatMutationRecord{
					Heartbeat: AgentHeartbeatRecord{
						AgentName:        "coder",
						Valid:            true,
						ValidationStatus: contract.AuthoredValidationValid,
						Digest:           "sha256:new",
					},
					Revision: AgentHeartbeatRevisionRecord{
						ID:        "rev-hb-1",
						AgentName: "coder",
						Operation: contract.HeartbeatRevisionWrite,
						NewDigest: "sha256:new",
						CreatedAt: fixedTestNow,
					},
				}, nil
			},
		}

		_, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"agent",
			"heartbeat",
			"write",
			"coder",
			"--stdin=false",
			"--file",
			bodyPath,
			"--expected-digest",
			"sha256:old",
			"--workspace",
			"checkout-api",
			"--json",
		)
		if err != nil {
			t.Fatalf("agent heartbeat write error = %v", err)
		}
		if !called {
			t.Fatal("PutAgentHeartbeat() was not called")
		}
	})
}
